package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	PhoneVerificationSendIntervalSeconds = 60
	PhoneVerificationMaxSends            = 5
	PhoneVerificationMaxAttempts         = 5
)

var (
	ErrPhoneNumberEmpty                 = errors.New("phone number is empty")
	ErrPhoneNumberInvalid               = errors.New("phone number is invalid")
	ErrPhoneNumberAlreadyRegistered     = errors.New("phone number already registered")
	ErrPhoneNumberNotRegistered         = errors.New("phone number is not registered")
	ErrPhoneVerificationPurposeInvalid  = errors.New("phone verification purpose is invalid")
	ErrPhoneVerificationSendTooFrequent = errors.New("phone verification code sent too frequently")
	ErrPhoneVerificationSendTooMany     = errors.New("phone verification code sent too many times")
	ErrPhoneVerificationSendFailed      = errors.New("phone verification code send failed")
	ErrPhoneVerificationCodeInvalid     = errors.New("phone verification code is invalid")
	ErrPhoneVerificationCodeExpired     = errors.New("phone verification code has expired")
	ErrPhoneVerificationTooManyAttempts = errors.New("phone verification code has too many attempts")
)

var e164PhonePattern = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)

var phoneVerificationSendLocks sync.Map

// NormalizePhoneNumber 将手机号规范化为 E.164 风格，裸 11 位大陆手机号默认补 +86。
func NormalizePhoneNumber(phoneNumber string) (string, error) {
	phoneNumber = strings.TrimSpace(phoneNumber)
	if phoneNumber == "" {
		return "", ErrPhoneNumberEmpty
	}
	if strings.HasPrefix(phoneNumber, "+") {
		if !e164PhonePattern.MatchString(phoneNumber) {
			return "", ErrPhoneNumberInvalid
		}
		return phoneNumber, nil
	}
	if len(phoneNumber) == 11 && strings.HasPrefix(phoneNumber, "1") && onlyDigits(phoneNumber) {
		return "+86" + phoneNumber, nil
	}
	return "", ErrPhoneNumberInvalid
}

func onlyDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// SendPhoneVerificationCode 发送并持久化手机号验证码。
func SendPhoneVerificationCode(phoneNumber string, purpose string, clientIP string) error {
	phoneNumber, err := NormalizePhoneNumber(phoneNumber)
	if err != nil {
		return err
	}
	if err := validatePhoneVerificationPurpose(purpose); err != nil {
		return err
	}
	if err := checkPhonePurposeAccountState(phoneNumber, purpose); err != nil {
		return err
	}

	unlock, err := acquirePhoneVerificationSendLock(phoneNumber, purpose)
	if err != nil {
		return err
	}
	defer unlock()

	code, err := common.GenerateNumericVerificationCode(common.VerificationCodeLength)
	if err != nil {
		return err
	}
	codeHash, err := common.Password2Hash(code)
	if err != nil {
		return err
	}

	var recordId int
	now := common.GetTimestamp()
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := checkPhoneVerificationSendLimitWithTx(tx, phoneNumber, purpose); err != nil {
			return err
		}
		sendCount, err := model.CountPhoneVerificationCodesSinceWithTx(tx, phoneNumber, purpose, now-int64(common.VerificationValidMinutes*60))
		if err != nil {
			return err
		}
		if err := model.ExpirePendingPhoneVerificationCodesWithTx(tx, phoneNumber, purpose); err != nil {
			return err
		}
		record := &model.PhoneVerificationCode{
			PhoneNumber:  phoneNumber,
			CodeHash:     codeHash,
			Purpose:      purpose,
			ExpiresAt:    now + int64(common.VerificationValidMinutes*60),
			SendCount:    int(sendCount) + 1,
			AttemptCount: 0,
			Status:       model.PhoneVerificationStatusPending,
			ClientIp:     clientIP,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		recordId = record.Id
		return nil
	}); err != nil {
		return err
	}

	if err := getSmsSender().Send(context.Background(), SmsSendRequest{
		PhoneNumber: phoneNumber,
		Code:        code,
		Purpose:     purpose,
	}); err != nil {
		if recordId > 0 {
			_ = model.ExpirePhoneVerificationCode(recordId)
		}
		common.SysLog(fmt.Sprintf("send phone verification failed phone=%s purpose=%s error=%s", common.MaskPhone(phoneNumber), purpose, err.Error()))
		return fmt.Errorf("%w: %v", ErrPhoneVerificationSendFailed, err)
	}
	return nil
}

func acquirePhoneVerificationSendLock(phoneNumber string, purpose string) (func(), error) {
	key := "phone_verification_send:" + phoneNumber + ":" + purpose
	if common.RedisEnabled && common.RDB != nil {
		ctx := context.Background()
		token := common.GetUUID()
		ok, err := common.RDB.SetNX(ctx, key, token, 15*time.Second).Result()
		if err == nil {
			if !ok {
				return nil, ErrPhoneVerificationSendTooFrequent
			}
			return func() {
				_ = common.RDB.Eval(ctx, `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`, []string{key}, token).Err()
			}, nil
		}
	}

	value, _ := phoneVerificationSendLocks.LoadOrStore(key, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	if !lock.TryLock() {
		return nil, ErrPhoneVerificationSendTooFrequent
	}
	return lock.Unlock, nil
}

func checkPhonePurposeAccountState(phoneNumber string, purpose string) error {
	switch purpose {
	case model.PhoneVerificationPurposeRegister:
		taken, err := IsPhoneAuthAccountAlreadyTaken(phoneNumber)
		if err != nil {
			return err
		}
		if taken {
			return ErrPhoneNumberAlreadyRegistered
		}
	case model.PhoneVerificationPurposeLogin, model.PhoneVerificationPurposeResetPassword:
		taken := model.IsPhoneNumberAlreadyTaken(phoneNumber)
		if !taken {
			return ErrPhoneNumberNotRegistered
		}
	}
	return nil
}

// IsPhoneAuthAccountAlreadyTaken checks both the phone profile and username
// values that would be shadowed by phone password-login normalization.
func IsPhoneAuthAccountAlreadyTaken(phoneNumber string) (bool, error) {
	phoneNumber, err := NormalizePhoneNumber(phoneNumber)
	if err != nil {
		return false, err
	}
	if model.IsPhoneNumberAlreadyTaken(phoneNumber) {
		return true, nil
	}
	for _, username := range phoneAuthUsernameCandidates(phoneNumber) {
		exist, err := model.CheckUserExistOrDeleted(username, "")
		if err != nil {
			return false, err
		}
		if exist {
			return true, nil
		}
	}
	return false, nil
}

func phoneAuthUsernameCandidates(normalizedPhone string) []string {
	candidates := []string{normalizedPhone}
	if strings.HasPrefix(normalizedPhone, "+86") {
		localNumber := strings.TrimPrefix(normalizedPhone, "+86")
		if len(localNumber) == 11 && strings.HasPrefix(localNumber, "1") && onlyDigits(localNumber) {
			candidates = append(candidates, localNumber)
		}
	}
	return candidates
}

func checkPhoneVerificationSendLimit(phoneNumber string, purpose string) error {
	return checkPhoneVerificationSendLimitWithTx(model.DB, phoneNumber, purpose)
}

func checkPhoneVerificationSendLimitWithTx(tx *gorm.DB, phoneNumber string, purpose string) error {
	now := common.GetTimestamp()
	latest, err := model.GetLatestPhoneVerificationCodeWithTx(tx, phoneNumber, purpose)
	if err == nil && now-latest.CreatedAt < PhoneVerificationSendIntervalSeconds {
		return ErrPhoneVerificationSendTooFrequent
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	count, err := model.CountPhoneVerificationCodesSinceWithTx(tx, phoneNumber, purpose, now-int64(common.VerificationValidMinutes*60))
	if err != nil {
		return err
	}
	if count >= PhoneVerificationMaxSends {
		return ErrPhoneVerificationSendTooMany
	}
	return nil
}

// ConsumePhoneVerificationCode 校验并消费手机号验证码。
func ConsumePhoneVerificationCode(phoneNumber string, purpose string, code string) error {
	return ConsumePhoneVerificationCodeWithTx(model.DB, phoneNumber, purpose, code)
}

// ConsumePhoneVerificationCodeWithTx 校验并在指定事务内消费手机号验证码。
func ConsumePhoneVerificationCodeWithTx(tx *gorm.DB, phoneNumber string, purpose string, code string) error {
	if tx == nil {
		tx = model.DB
	}
	phoneNumber, err := NormalizePhoneNumber(phoneNumber)
	if err != nil {
		return err
	}
	if err := validatePhoneVerificationPurpose(purpose); err != nil {
		return err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrPhoneVerificationCodeInvalid
	}

	record, err := model.GetLatestPendingPhoneVerificationCodeWithTx(tx, phoneNumber, purpose)
	if err != nil {
		return ErrPhoneVerificationCodeInvalid
	}
	now := common.GetTimestamp()
	if record.ExpiresAt <= now {
		_ = model.ExpirePhoneVerificationCode(record.Id)
		return ErrPhoneVerificationCodeExpired
	}
	if record.AttemptCount >= PhoneVerificationMaxAttempts {
		_ = model.ExpirePhoneVerificationCode(record.Id)
		return ErrPhoneVerificationTooManyAttempts
	}

	if !common.ValidatePasswordAndHash(code, record.CodeHash) {
		expire := record.AttemptCount+1 >= PhoneVerificationMaxAttempts
		if err := model.IncrementPhoneVerificationAttempt(record.Id, expire); err != nil {
			if errors.Is(err, model.ErrPhoneVerificationCodeNotPending) {
				return ErrPhoneVerificationCodeInvalid
			}
			return err
		}
		return ErrPhoneVerificationCodeInvalid
	}
	if err := model.MarkPhoneVerificationCodeUsedWithTx(tx, record.Id); err != nil {
		if errors.Is(err, model.ErrPhoneVerificationCodeNotPending) {
			return ErrPhoneVerificationCodeInvalid
		}
		return err
	}
	return nil
}

func validatePhoneVerificationPurpose(purpose string) error {
	switch purpose {
	case model.PhoneVerificationPurposeRegister, model.PhoneVerificationPurposeLogin, model.PhoneVerificationPurposeResetPassword:
		return nil
	default:
		return ErrPhoneVerificationPurposeInvalid
	}
}

// PhoneVerificationDuration 返回验证码有效期，供控制器或前端状态扩展使用。
func PhoneVerificationDuration() time.Duration {
	return time.Duration(common.VerificationValidMinutes) * time.Minute
}
