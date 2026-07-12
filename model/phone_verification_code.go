package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	PhoneVerificationPurposeRegister      = "register"
	PhoneVerificationPurposeLogin         = "login"
	PhoneVerificationPurposeResetPassword = "reset_password"
)

const (
	PhoneVerificationStatusPending = "pending"
	PhoneVerificationStatusUsed    = "used"
	PhoneVerificationStatusExpired = "expired"
)

// ErrPhoneVerificationCodeNotPending 表示验证码已被其他请求消费或已失效。
var ErrPhoneVerificationCodeNotPending = errors.New("phone verification code is not pending")

// PhoneVerificationCode 对应 phone_verification_codes 表，记录手机号验证码发送、校验和风控状态。
type PhoneVerificationCode struct {
	Id           int    `json:"id" gorm:"comment:主键ID"`
	PhoneNumber  string `json:"phone_number" gorm:"type:varchar(32);index:idx_phone_verification_lookup,priority:1;comment:手机号"`
	CodeHash     string `json:"code_hash" gorm:"type:varchar(128);not null;comment:验证码哈希值"`
	Purpose      string `json:"purpose" gorm:"type:varchar(32);index:idx_phone_verification_lookup,priority:2;comment:验证码用途"`
	ExpiresAt    int64  `json:"expires_at" gorm:"bigint;index;comment:过期时间戳"`
	SendCount    int    `json:"send_count" gorm:"type:int;default:0;comment:发送次数"`
	AttemptCount int    `json:"attempt_count" gorm:"type:int;default:0;comment:校验尝试次数"`
	Status       string `json:"status" gorm:"type:varchar(32);default:'pending';index;comment:验证码状态"`
	ClientIp     string `json:"client_ip" gorm:"type:varchar(64);default:'';comment:发送客户端IP"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index;comment:创建时间戳"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint;comment:更新时间戳"`
}

// BeforeCreate 初始化验证码记录的时间戳和默认状态。
func (c *PhoneVerificationCode) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if c.CreatedAt == 0 {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if c.Status == "" {
		c.Status = PhoneVerificationStatusPending
	}
	return nil
}

// BeforeUpdate 在验证码记录更新时刷新更新时间戳。
func (c *PhoneVerificationCode) BeforeUpdate(tx *gorm.DB) error {
	c.UpdatedAt = common.GetTimestamp()
	return nil
}

// GetLatestPendingPhoneVerificationCode 查询指定手机号和场景下最新的待验证验证码。
func GetLatestPendingPhoneVerificationCode(phoneNumber string, purpose string) (*PhoneVerificationCode, error) {
	return GetLatestPendingPhoneVerificationCodeWithTx(DB, phoneNumber, purpose)
}

// GetLatestPendingPhoneVerificationCodeWithTx 在指定事务内查询最新待验证验证码。
func GetLatestPendingPhoneVerificationCodeWithTx(tx *gorm.DB, phoneNumber string, purpose string) (*PhoneVerificationCode, error) {
	if phoneNumber == "" || purpose == "" {
		return nil, errors.New("phone number or purpose is empty")
	}
	if tx == nil {
		tx = DB
	}
	var code PhoneVerificationCode
	if err := tx.Where("phone_number = ? AND purpose = ? AND status = ?", phoneNumber, purpose, PhoneVerificationStatusPending).
		Order("created_at DESC, id DESC").
		First(&code).Error; err != nil {
		return nil, err
	}
	return &code, nil
}

// GetLatestPhoneVerificationCode 查询指定手机号和场景下最新验证码记录，包含已使用和已过期记录。
func GetLatestPhoneVerificationCode(phoneNumber string, purpose string) (*PhoneVerificationCode, error) {
	return GetLatestPhoneVerificationCodeWithTx(DB, phoneNumber, purpose)
}

// GetLatestPhoneVerificationCodeWithTx 在指定事务内查询最新验证码记录。
func GetLatestPhoneVerificationCodeWithTx(tx *gorm.DB, phoneNumber string, purpose string) (*PhoneVerificationCode, error) {
	if phoneNumber == "" || purpose == "" {
		return nil, errors.New("phone number or purpose is empty")
	}
	if tx == nil {
		tx = DB
	}
	var code PhoneVerificationCode
	if err := tx.Where("phone_number = ? AND purpose = ?", phoneNumber, purpose).
		Order("created_at DESC, id DESC").
		First(&code).Error; err != nil {
		return nil, err
	}
	return &code, nil
}

// CountPhoneVerificationCodesSince 统计指定时间之后同一手机号同一场景的发送次数。
func CountPhoneVerificationCodesSince(phoneNumber string, purpose string, since int64) (int64, error) {
	return CountPhoneVerificationCodesSinceWithTx(DB, phoneNumber, purpose, since)
}

// CountPhoneVerificationCodesSinceWithTx 在指定事务内统计指定时间之后的验证码发送记录。
func CountPhoneVerificationCodesSinceWithTx(tx *gorm.DB, phoneNumber string, purpose string, since int64) (int64, error) {
	if tx == nil {
		tx = DB
	}
	var count int64
	err := tx.Model(&PhoneVerificationCode{}).
		Where("phone_number = ? AND purpose = ? AND created_at >= ?", phoneNumber, purpose, since).
		Count(&count).Error
	return count, err
}

// ExpirePendingPhoneVerificationCodes 将指定手机号和场景的待验证验证码置为过期。
func ExpirePendingPhoneVerificationCodes(phoneNumber string, purpose string) error {
	return ExpirePendingPhoneVerificationCodesWithTx(DB, phoneNumber, purpose)
}

// ExpirePendingPhoneVerificationCodesWithTx 在指定事务内过期同手机号同场景的待验证验证码。
func ExpirePendingPhoneVerificationCodesWithTx(tx *gorm.DB, phoneNumber string, purpose string) error {
	if phoneNumber == "" || purpose == "" {
		return errors.New("phone number or purpose is empty")
	}
	if tx == nil {
		tx = DB
	}
	return tx.Model(&PhoneVerificationCode{}).
		Where("phone_number = ? AND purpose = ? AND status = ?", phoneNumber, purpose, PhoneVerificationStatusPending).
		Updates(map[string]interface{}{
			"status":     PhoneVerificationStatusExpired,
			"updated_at": common.GetTimestamp(),
		}).Error
}

// ExpirePhoneVerificationCode 将单条验证码记录置为过期。
func ExpirePhoneVerificationCode(id int) error {
	return ExpirePhoneVerificationCodeWithTx(DB, id)
}

// ExpirePhoneVerificationCodeWithTx 在指定事务内过期单条验证码。
func ExpirePhoneVerificationCodeWithTx(tx *gorm.DB, id int) error {
	if id <= 0 {
		return errors.New("invalid phone verification code id")
	}
	if tx == nil {
		tx = DB
	}
	return tx.Model(&PhoneVerificationCode{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     PhoneVerificationStatusExpired,
		"updated_at": common.GetTimestamp(),
	}).Error
}

// MarkPhoneVerificationCodeUsed 将验证码标记为已使用，避免重复消费。
func MarkPhoneVerificationCodeUsed(id int) error {
	return MarkPhoneVerificationCodeUsedWithTx(DB, id)
}

// MarkPhoneVerificationCodeUsedWithTx 使用 CAS 条件在指定事务内消费验证码。
func MarkPhoneVerificationCodeUsedWithTx(tx *gorm.DB, id int) error {
	if id <= 0 {
		return errors.New("invalid phone verification code id")
	}
	if tx == nil {
		tx = DB
	}
	result := tx.Model(&PhoneVerificationCode{}).Where("id = ? AND status = ?", id, PhoneVerificationStatusPending).
		Updates(map[string]interface{}{
			"status":     PhoneVerificationStatusUsed,
			"updated_at": common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPhoneVerificationCodeNotPending
	}
	return nil
}

// IncrementPhoneVerificationAttempt 增加验证码错误尝试次数，并可同步标记为过期。
func IncrementPhoneVerificationAttempt(id int, expire bool) error {
	return IncrementPhoneVerificationAttemptWithTx(DB, id, expire)
}

// IncrementPhoneVerificationAttemptWithTx 在指定事务内递增验证码错误次数。
func IncrementPhoneVerificationAttemptWithTx(tx *gorm.DB, id int, expire bool) error {
	if id <= 0 {
		return errors.New("invalid phone verification code id")
	}
	if tx == nil {
		tx = DB
	}
	updates := map[string]interface{}{
		"attempt_count": gorm.Expr("attempt_count + ?", 1),
		"updated_at":    common.GetTimestamp(),
	}
	if expire {
		updates["status"] = PhoneVerificationStatusExpired
	}
	result := tx.Model(&PhoneVerificationCode{}).Where("id = ? AND status = ?", id, PhoneVerificationStatusPending).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPhoneVerificationCodeNotPending
	}
	return nil
}
