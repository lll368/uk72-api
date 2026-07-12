package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

var (
	ErrWithdrawalProfileIncomplete = errors.New("提现资料不完整")
	ErrWithdrawalPhoneInvalid      = errors.New("提现手机号格式无效")
	ErrWithdrawPayoutSnapshotEmpty = errors.New("提现订单打款资料快照不完整，请重新提交提现申请")
	ErrPiggyWithdrawDisabled       = errors.New("小猪提现未启用")
	ErrPiggyContractUnsigned       = errors.New("小猪电子合同未签约")
	ErrPiggyCallbackDuplicate      = errors.New("小猪回调已处理")
	ErrPiggyQueryStatusUnknown     = errors.New("小猪订单查询结果无法判定远端业务状态")
)

const (
	piggyCompensationStatusPending          = "pending_compensation"
	piggyCompensationStatusManualProcessed  = "manual_processed"
	piggyCompensationStatusSubmitRecovering = "submit_recovering"
	piggyManualResultPaid                   = "manual_paid"
	piggyManualResultFailed                 = "manual_failed"
	piggyWithdrawCompensationScanLimit      = 50
	piggyWithdrawCompensationTickInterval   = time.Minute
)

var (
	piggyDuplicateOuterTradeNoMarkers = []string{
		"outertradeno",
		"outer_trade_no",
		"out_trade_no",
		"外部订单号",
		"商户订单号",
	}
	piggyDuplicateFailureMarkers = []string{
		"duplicate",
		"already exists",
		"重复",
		"已存在",
	}
	piggyOrderNotFoundMarkers = []string{
		"劳务订单不存在",
		"订单不存在",
		"order not found",
		"labor order not found",
	}
	piggyWithdrawCompensationTaskOnce    sync.Once
	piggyWithdrawCompensationTaskRunning atomic.Bool
)

type WithdrawalProfileInput struct {
	AccountType string `json:"account_type"`
	RealName    string `json:"real_name"`
	IdCardNo    string `json:"id_card_no"`
	Mobile      string `json:"mobile"`
	BankCardNo  string `json:"bank_card_no"`
	BankName    string `json:"bank_name"`
}

type WithdrawalEligibility struct {
	Enabled                     bool                     `json:"enabled"`
	CanWithdraw                 bool                     `json:"can_withdraw"`
	NeedProfile                 bool                     `json:"need_profile"`
	NeedSign                    bool                     `json:"need_sign"`
	Profile                     *model.WithdrawalProfile `json:"profile"`
	WithdrawableCommission      float64                  `json:"withdrawable_commission"`
	FrozenCommission            float64                  `json:"frozen_commission"`
	CommissionMinWithdrawAmount float64                  `json:"commission_min_withdraw_amount"`
	CooldownRemainingSeconds    int64                    `json:"cooldown_remaining_seconds"`
	DisabledReason              string                   `json:"disabled_reason"`
	BlockingReasons             []string                 `json:"blocking_reasons"`
}

type PiggyWithdrawSubmitRequest struct {
	UserId int
	Amount float64
	Remark string
}

type PiggyTaxTrialRequest struct {
	UserId int
	Amount float64
}

type PiggyContractPreviewResult struct {
	DocumentID string `json:"document_id"`
	PreviewURL string `json:"preview_url"`
}

// WithdrawApprovalResult 描述管理员审核后外部打款提交的实际结果。
type WithdrawApprovalResult struct {
	Submitted   bool   `json:"submitted"`
	Recoverable bool   `json:"recoverable"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

type PiggyPaymentCallbackEnvelope struct {
	Code         string          `json:"code"`
	Msg          string          `json:"msg"`
	IsSuccess    json.RawMessage `json:"isSuccess"`
	ErrorCode    string          `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage"`
	Data         struct {
		BizAESContent string `json:"bizAESContent"`
	} `json:"data"`
	Sign string `json:"sign"`
}

type PiggyPaymentCallbackContent struct {
	OuterTradeNo        string `json:"outerTradeNo"`
	NotifyType          string `json:"notifyType"`
	TradeStatus         string `json:"tradeStatus"`
	TradeTime           string `json:"tradeTime"`
	FrontLogNo          string `json:"frontLogNo"`
	LaborOrderNo        string `json:"laborOrderNo"`
	EmpName             string `json:"empName"`
	EmpPhone            string `json:"empPhone"`
	LicenseType         string `json:"licenseType"`
	LicenseId           string `json:"licenseId"`
	SettleType          string `json:"settleType"`
	PayAccount          string `json:"payAccount"`
	BankName            string `json:"bankName"`
	PositionName        string `json:"positionName"`
	TradeFailCode       string `json:"tradeFailCode"`
	TradeResult         string `json:"tradeResult"`
	TradeResultDescribe string `json:"tradeResultDescribe"`
	PretaxAmount        string `json:"pretaxAmount"`
	IndividualTaxAmount string `json:"individualTaxAmount"`
	AddedTaxAmount      string `json:"addedTaxAmount"`
	AfterTaxAmount      string `json:"afterTaxAmount"`
	FeeAmount           string `json:"feeAmount"`
	CalcType            string `json:"calcType"`
}

func (c *PiggyPaymentCallbackContent) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = PiggyPaymentCallbackContent{}
	assign := func(key string, dest *string) error {
		value, ok := raw[key]
		if !ok {
			return nil
		}
		text, err := piggyJSONScalarToString(value)
		if err != nil {
			return fmt.Errorf("%s 格式错误: %w", key, err)
		}
		*dest = text
		return nil
	}
	fields := []struct {
		key  string
		dest *string
	}{
		{"outerTradeNo", &c.OuterTradeNo},
		{"notifyType", &c.NotifyType},
		{"tradeStatus", &c.TradeStatus},
		{"tradeTime", &c.TradeTime},
		{"frontLogNo", &c.FrontLogNo},
		{"laborOrderNo", &c.LaborOrderNo},
		{"empName", &c.EmpName},
		{"empPhone", &c.EmpPhone},
		{"licenseType", &c.LicenseType},
		{"licenseId", &c.LicenseId},
		{"settleType", &c.SettleType},
		{"payAccount", &c.PayAccount},
		{"bankName", &c.BankName},
		{"positionName", &c.PositionName},
		{"tradeFailCode", &c.TradeFailCode},
		{"tradeResult", &c.TradeResult},
		{"tradeResultDescribe", &c.TradeResultDescribe},
		{"pretaxAmount", &c.PretaxAmount},
		{"individualTaxAmount", &c.IndividualTaxAmount},
		{"addedTaxAmount", &c.AddedTaxAmount},
		{"afterTaxAmount", &c.AfterTaxAmount},
		{"feeAmount", &c.FeeAmount},
		{"calcType", &c.CalcType},
	}
	for _, field := range fields {
		if err := assign(field.key, field.dest); err != nil {
			return err
		}
	}
	return nil
}

func piggyJSONScalarToString(value json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "", errors.New("仅支持字符串或数字")
	}
	if strings.HasPrefix(trimmed, "\"") {
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return "", err
		}
		return text, nil
	}
	if first := trimmed[0]; first == '-' || (first >= '0' && first <= '9') {
		return trimmed, nil
	}
	return "", errors.New("仅支持字符串或数字")
}

type PiggyContractCallbackPayload struct {
	Code           string                    `json:"code"`
	Msg            string                    `json:"msg"`
	Data           PiggyContractCallbackData `json:"data"`
	UserName       string                    `json:"userName"`
	Name           string                    `json:"name"`
	IdCardNo       string                    `json:"idCardNo"`
	Mobile         string                    `json:"mobile"`
	BankAccount    string                    `json:"bankAccount"`
	ContractURL    string                    `json:"contract_url"`
	ContractUrl    string                    `json:"contractUrl"`
	DocumentID     string                    `json:"document_id"`
	DocumentId     string                    `json:"documentId"`
	SubsidiaryName string                    `json:"subsidiary_name"`
	SignStatus     string                    `json:"signStatus"`
	Status         string                    `json:"status"`
	CustomParams   any                       `json:"customParams"`
	Sign           string                    `json:"sign"`
}

type PiggyContractCallbackData struct {
	UserName            string `json:"userName"`
	Name                string `json:"name"`
	IdCardNo            string `json:"idCardNo"`
	Mobile              string `json:"mobile"`
	BankAccount         string `json:"bankAccount"`
	ContractURL         string `json:"contract_url"`
	ContractUrl         string `json:"contractUrl"`
	DocumentID          string `json:"document_id"`
	DocumentId          string `json:"documentId"`
	SubsidiaryName      string `json:"subsidiary_name"`
	SubsidiaryNameCamel string `json:"subsidiaryName"`
	SignStatus          string `json:"signStatus"`
	Status              string `json:"status"`
	CustomParams        any    `json:"customParams"`
}

type normalizedPiggyContractCallback struct {
	Code           string
	Msg            string
	UserName       string
	IdCardNo       string
	Mobile         string
	BankAccount    string
	ContractURL    string
	DocumentID     string
	SubsidiaryName string
	SignStatus     string
	Status         string
	CustomParams   any
	Sign           string
}

type piggySignedContractResult struct {
	CompanyName         string `json:"company_name"`
	CompanyNameCamel    string `json:"companyName"`
	UserName            string `json:"userName"`
	Name                string `json:"name"`
	IdCardNo            string `json:"idCardNo"`
	Mobile              string `json:"mobile"`
	BankAccount         string `json:"bankAccount"`
	ContractURL         string `json:"contract_url"`
	ContractUrl         string `json:"contractUrl"`
	DocumentID          string `json:"document_id"`
	DocumentId          string `json:"documentId"`
	SubsidiaryName      string `json:"subsidiary_name"`
	SubsidiaryNameCamel string `json:"subsidiaryName"`
	SignTime            string `json:"sign_time"`
	SignTimeCamel       string `json:"signTime"`
	Position            string `json:"position"`
}

func GetWithdrawalProfile(userId int) (*model.WithdrawalProfile, error) {
	if userId <= 0 {
		return nil, errors.New("用户不存在")
	}
	var profile model.WithdrawalProfile
	if err := model.DB.Where("user_id = ?", userId).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return maskWithdrawalProfile(&profile), nil
}

func SaveWithdrawalProfile(userId int, input WithdrawalProfileInput) (*model.WithdrawalProfile, error) {
	if userId <= 0 {
		return nil, errors.New("用户不存在")
	}
	accountType := strings.TrimSpace(input.AccountType)
	if accountType == "" {
		accountType = model.WithdrawAccountTypeBankcard
	}
	if accountType != model.WithdrawAccountTypeBankcard {
		return nil, fmt.Errorf("暂不支持 %s 提现账户", accountType)
	}
	input.RealName = strings.TrimSpace(input.RealName)
	input.IdCardNo = strings.TrimSpace(input.IdCardNo)
	input.Mobile = strings.TrimSpace(input.Mobile)
	input.BankCardNo = strings.TrimSpace(input.BankCardNo)
	input.BankName = strings.TrimSpace(input.BankName)
	if input.RealName == "" || input.IdCardNo == "" || input.Mobile == "" || input.BankCardNo == "" || input.BankName == "" {
		return nil, ErrWithdrawalProfileIncomplete
	}
	piggyMobile, err := normalizeWithdrawalMobile(input.Mobile)
	if err != nil {
		return nil, err
	}
	input.Mobile = piggyMobile
	var profile model.WithdrawalProfile
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("user_id = ?", userId).First(&profile).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			profile = model.WithdrawalProfile{UserId: userId}
		}
		changedContractIdentity := profile.RealName != input.RealName ||
			profile.IdCardNo != input.IdCardNo
		profile.AccountType = accountType
		profile.RealName = input.RealName
		profile.IdCardNo = input.IdCardNo
		profile.Mobile = input.Mobile
		profile.BankCardNo = input.BankCardNo
		profile.BankName = input.BankName
		if changedContractIdentity {
			// 小猪电子签约按姓名、身份证、服务类型、结算主体、税源地确定合同范围；
			// 手机号、银行卡号、银行名称仅影响收付款资料，不应让已签合同失效。
			profile.PiggySignStatus = model.PiggySignStatusUnsigned
			profile.PiggySignedAt = 0
			profile.PiggyContractURL = ""
			profile.PiggyContractDocumentID = ""
			profile.PiggyContractSubsidiaryName = ""
			profile.PiggyContractPosition = ""
			profile.PiggyContractPositionName = ""
			profile.PiggyContractTaxFundID = ""
		}
		return tx.Save(&profile).Error
	})
	if err != nil {
		return nil, err
	}
	return maskWithdrawalProfile(&profile), nil
}

func GetPiggyWithdrawalEligibility(userId int) (*WithdrawalEligibility, error) {
	account, err := GetOrCreateWalletAccount(userId)
	if err != nil {
		return nil, err
	}
	profile, err := GetWithdrawalProfile(userId)
	if err != nil {
		return nil, err
	}
	setting := operation_setting.GetPiggyWithdrawSetting()
	result := &WithdrawalEligibility{
		Enabled:                     setting.Enabled,
		Profile:                     profile,
		WithdrawableCommission:      account.CommissionAmount,
		FrozenCommission:            account.FrozenCommissionAmount,
		CommissionMinWithdrawAmount: operation_setting.GetPaymentSetting().CommissionMinWithdrawAmount,
	}
	if !setting.Enabled {
		result.BlockingReasons = append(result.BlockingReasons, "小猪提现未启用")
	}
	if !isWithdrawalProfileComplete(profile) {
		result.NeedProfile = true
		result.BlockingReasons = append(result.BlockingReasons, "请先完善银行卡提现资料")
	}
	if !isPiggyContractSignedForCurrentScope(profile, setting) {
		result.NeedSign = true
		if profile != nil && profile.PiggySignStatus == model.PiggySignStatusSigned {
			result.BlockingReasons = append(result.BlockingReasons, "小猪电子合同签约范围已变更，请重新签约")
		} else {
			result.BlockingReasons = append(result.BlockingReasons, "请先完成小猪电子合同签约")
		}
	}
	if account.CommissionAmount+walletAmountEpsilon < result.CommissionMinWithdrawAmount {
		result.BlockingReasons = append(result.BlockingReasons, "可提现佣金不足")
	}
	cooldown, err := getPiggyWithdrawCooldownRemaining(userId, setting.CooldownMinutes)
	if err != nil {
		return nil, err
	}
	result.CooldownRemainingSeconds = cooldown
	if cooldown > 0 {
		result.BlockingReasons = append(result.BlockingReasons, "提现冷却中")
	}
	if isForbiddenWithdrawTime(setting.ForbiddenWithdrawTime, time.Now()) {
		result.BlockingReasons = append(result.BlockingReasons, "当前时间禁止提现")
	}
	result.CanWithdraw = len(result.BlockingReasons) == 0
	if len(result.BlockingReasons) > 0 {
		result.DisabledReason = result.BlockingReasons[0]
	}
	return result, nil
}

func GetPiggyContractSignURL(ctx context.Context, userId int) (map[string]any, error) {
	setting := operation_setting.GetPiggyWithdrawSetting()
	if !setting.Enabled {
		return nil, ErrPiggyWithdrawDisabled
	}
	if err := operation_setting.ValidatePiggyWithdrawSettingForEnable(setting); err != nil {
		return nil, err
	}
	profile, err := getRawWithdrawalProfile(userId)
	if err != nil {
		return nil, err
	}
	if !isWithdrawalProfileComplete(profile) {
		return nil, ErrWithdrawalProfileIncomplete
	}
	if isPiggyContractSignedForCurrentScope(profile, setting) {
		return map[string]any{"signed": true}, nil
	}
	client, err := newConfiguredPiggyClient(setting)
	if err != nil {
		return nil, err
	}
	customParams, _ := common.Marshal(map[string]any{"userId": userId})
	result, reqDigest, err := client.GetContractSignURL(ctx, PiggyContractSignURLRequest{
		UserName:     profile.RealName,
		IdCardNo:     profile.IdCardNo,
		Mobile:       profile.Mobile,
		BankAccount:  profile.BankCardNo,
		Position:     piggyContractServiceType(setting),
		NotifyUrl:    setting.SignNotifyUrl,
		JumpPage:     setting.SignJumpPage,
		CustomParams: string(customParams),
	})
	if err != nil {
		return nil, err
	}
	signURL := ""
	if result != nil {
		signURL = strings.TrimSpace(result.SignURL)
	}
	if signURL == "" {
		return nil, errors.New("小猪签约地址为空")
	}
	update := map[string]interface{}{"piggy_sign_url_digest": reqDigest}
	if result != nil && result.Raw != nil {
		update["last_callback_digest"] = digestPayload([]byte(result.Raw.RawBody))
	}
	_ = model.DB.Model(&model.WithdrawalProfile{}).Where("id = ?", profile.Id).Updates(update).Error
	return map[string]any{
		"signed":   false,
		"sign_url": signURL,
	}, nil
}

func RefreshPiggyContractStatus(ctx context.Context, userId int) (*model.WithdrawalProfile, error) {
	if userId <= 0 {
		return nil, errors.New("用户不存在")
	}
	setting := operation_setting.GetPiggyWithdrawSetting()
	if !setting.Enabled {
		return nil, ErrPiggyWithdrawDisabled
	}
	if err := operation_setting.ValidatePiggyWithdrawSettingForEnable(setting); err != nil {
		return nil, err
	}
	profile, err := getRawWithdrawalProfile(userId)
	if err != nil {
		return nil, err
	}
	if !isWithdrawalProfileComplete(profile) {
		return nil, ErrWithdrawalProfileIncomplete
	}
	if isPiggyContractSignedForCurrentScope(profile, setting) {
		return maskWithdrawalProfile(profile), nil
	}
	client, err := newConfiguredPiggyClient(setting)
	if err != nil {
		return nil, err
	}
	return refreshPiggyContractStatusFromProvider(ctx, client, profile, setting)
}

func GetPiggyContractPreviewURL(ctx context.Context, userId int) (*PiggyContractPreviewResult, error) {
	if userId <= 0 {
		return nil, errors.New("用户不存在")
	}
	setting := operation_setting.GetPiggyWithdrawSetting()
	if err := validatePiggyContractPreviewSetting(setting); err != nil {
		return nil, err
	}
	profile, err := getRawWithdrawalProfile(userId)
	if err != nil {
		return nil, err
	}
	if !isWithdrawalProfileComplete(profile) {
		return nil, ErrWithdrawalProfileIncomplete
	}
	if !isPiggyContractSignedForCurrentScope(profile, setting) {
		return nil, ErrPiggyContractUnsigned
	}

	var client *PiggyClient
	getClient := func() (*PiggyClient, error) {
		if client != nil {
			return client, nil
		}
		next, err := newPiggyPreviewClient(setting)
		if err != nil {
			return nil, err
		}
		client = next
		return client, nil
	}

	documentID := strings.TrimSpace(profile.PiggyContractDocumentID)
	if documentID == "" {
		if err := validatePiggyContractResultQuerySetting(setting); err != nil {
			return nil, err
		}
		client, err := getClient()
		if err != nil {
			return nil, err
		}
		refreshed, err := refreshPiggyContractStatusFromProvider(ctx, client, profile, setting)
		if err != nil {
			return nil, err
		}
		if refreshed != nil {
			documentID = strings.TrimSpace(refreshed.PiggyContractDocumentID)
		}
	}
	if documentID == "" {
		return nil, errors.New("小猪合同编号为空")
	}

	client, err = getClient()
	if err != nil {
		return nil, err
	}
	previewURL, _, err := client.PreviewContract(ctx, documentID)
	if err != nil {
		return nil, err
	}
	previewURL = strings.TrimSpace(previewURL)
	if previewURL == "" {
		return nil, errors.New("小猪合同预览地址为空")
	}
	return &PiggyContractPreviewResult{
		DocumentID: documentID,
		PreviewURL: previewURL,
	}, nil
}

func refreshPiggyContractStatusFromProvider(ctx context.Context, client *PiggyClient, profile *model.WithdrawalProfile, setting *operation_setting.PiggyWithdrawSetting) (*model.WithdrawalProfile, error) {
	if profile == nil {
		return nil, ErrWithdrawalProfileIncomplete
	}
	if client == nil {
		return nil, errors.New("小猪 client 未初始化")
	}
	response, _, err := client.QueryContractResult(ctx, PiggyContractStatusRequest{
		UserName: profile.RealName,
		IdCardNo: profile.IdCardNo,
	})
	if err != nil {
		return nil, err
	}
	contract, err := findMatchingPiggySignedContractResult(response, profile, setting)
	if err != nil {
		return nil, err
	}
	signedAt := parsePiggyContractSignTime(firstNonEmpty(contract.SignTime, contract.SignTimeCamel))
	if signedAt == 0 {
		signedAt = common.GetTimestamp()
	}
	updates := map[string]interface{}{
		"piggy_sign_status":              model.PiggySignStatusSigned,
		"piggy_signed_at":                signedAt,
		"piggy_contract_url":             firstNonEmpty(contract.ContractURL, contract.ContractUrl),
		"piggy_contract_document_id":     firstNonEmpty(contract.DocumentID, contract.DocumentId),
		"piggy_contract_subsidiary_name": firstNonEmpty(contract.SubsidiaryName, contract.SubsidiaryNameCamel),
		"piggy_contract_position":        piggyContractServiceType(setting),
		"piggy_contract_position_name":   strings.TrimSpace(setting.PositionName),
		"piggy_contract_tax_fund_id":     strings.TrimSpace(setting.TaxFundId),
	}
	if response != nil && strings.TrimSpace(response.RawBody) != "" {
		updates["last_callback_digest"] = digestPayload([]byte(response.RawBody))
	}
	if err := model.DB.Model(&model.WithdrawalProfile{}).Where("id = ?", profile.Id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetWithdrawalProfile(profile.UserId)
}

func validatePiggyContractPreviewSetting(setting *operation_setting.PiggyWithdrawSetting) error {
	if setting == nil {
		return errors.New("小猪合同预览配置不能为空")
	}
	if strings.TrimSpace(setting.Domain) == "" {
		return errors.New("小猪合同预览域名未配置")
	}
	return nil
}

func validatePiggyContractResultQuerySetting(setting *operation_setting.PiggyWithdrawSetting) error {
	if err := validatePiggyContractPreviewSetting(setting); err != nil {
		return err
	}
	required := map[string]string{
		"app_key":       setting.AppKey,
		"app_secret":    setting.AppSecret,
		"position_name": setting.PositionName,
		"tax_fund_id":   setting.TaxFundId,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("小猪合同签署结果查询配置缺失: %s", key)
		}
	}
	return nil
}

func SubmitPiggyWithdrawOrder(ctx context.Context, req PiggyWithdrawSubmitRequest) (*model.WithdrawOrder, error) {
	if req.UserId <= 0 {
		return nil, errors.New("用户不存在")
	}
	if req.Amount <= 0 {
		return nil, ErrWithdrawAmountInvalid
	}
	amountCents := floatYuanToCents(req.Amount)
	if amountCents <= 0 {
		return nil, ErrWithdrawAmountInvalid
	}
	setting := operation_setting.GetPiggyWithdrawSetting()
	if !setting.Enabled {
		return nil, ErrPiggyWithdrawDisabled
	}
	if err := operation_setting.ValidatePiggyWithdrawSettingForEnable(setting); err != nil {
		return nil, err
	}
	if err := validatePiggyWithdrawRequestAvailability(req.UserId, amountCents, setting); err != nil {
		return nil, err
	}
	feeCalc, err := calculatePiggyPlatformFee(amountCents, setting.PlatformFeeRate)
	if err != nil {
		return nil, err
	}
	profile, err := getRawWithdrawalProfile(req.UserId)
	if err != nil {
		return nil, err
	}
	if !isWithdrawalProfileComplete(profile) {
		return nil, ErrWithdrawalProfileIncomplete
	}
	if !isPiggyContractSignedForCurrentScope(profile, setting) {
		return nil, ErrPiggyContractUnsigned
	}

	var order *model.WithdrawOrder
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		account, err := getOrCreateWalletAccountTx(tx, req.UserId, true)
		if err != nil {
			return err
		}
		if account.CommissionAmount+walletAmountEpsilon < centsToFloat(amountCents) {
			return ErrCommissionInsufficient
		}
		account.CommissionAmount -= centsToFloat(amountCents)
		account.FrozenCommissionAmount += centsToFloat(amountCents)
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		order = &model.WithdrawOrder{
			UserId:                 req.UserId,
			WithdrawNo:             fmt.Sprintf("PWDR%d%s", req.UserId, common.GetTimeString()),
			Amount:                 centsToFloat(feeCalc.RequestedAmountCents),
			PlatformFeeRate:        feeCalc.PlatformFeeRate,
			PlatformFeeAmountCents: feeCalc.PlatformFeeAmountCents,
			ActualAmount:           0,
			Status:                 model.WithdrawStatusPending,
			Provider:               model.WithdrawProviderPiggyLaborV3,
			PiggyStatus:            model.WithdrawStatusPending,
			ReceiveType:            model.WithdrawAccountTypeBankcard,
			ReceiveAccount:         maskBankCard(profile.BankCardNo),
			WithdrawalProfileId:    profile.Id,
			AccountName:            profile.RealName,
			BankName:               profile.BankName,
			PayoutMobile:           profile.Mobile,
			PayoutIdCardNo:         profile.IdCardNo,
			PayoutBankCardNo:       profile.BankCardNo,
			TaxBeforeAmountCents:   feeCalc.PiggyPayAmountCents,
			FrozenAmountCents:      feeCalc.RequestedAmountCents,
			PiggyPayAmountCents:    feeCalc.PiggyPayAmountCents,
			PiggyPayAmount:         feeCalc.PiggyTaxBeforeAmount,
			ExternalTradeNo:        "",
			TaxFundId:              setting.TaxFundId,
			PositionName:           setting.PositionName,
			Position:               setting.Position,
			CalcType:               setting.CalcType,
			BankRemark:             setting.BankRemark,
			Remark:                 strings.TrimSpace(req.Remark),
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		return createWalletFlowTx(tx, &model.WalletFlow{
			UserId:                req.UserId,
			BizNo:                 order.WithdrawNo,
			IdempotencyKey:        walletIdempotencyKey("wallet:piggy-withdraw-freeze:" + order.WithdrawNo),
			FlowType:              model.WalletFlowTypeWithdrawFreeze,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionOut,
			Amount:                order.Amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                "小猪提现申请冻结税前佣金",
		})
	})
	if err != nil {
		return nil, err
	}
	return order, nil
}

func validatePiggyWithdrawRequestAvailability(userId int, amountCents int64, setting *operation_setting.PiggyWithdrawSetting) error {
	minAmount := operation_setting.GetPaymentSetting().CommissionMinWithdrawAmount
	if minAmount > 0 && centsToFloat(amountCents)+walletAmountEpsilon < minAmount {
		return fmt.Errorf("提现金额不能小于 %.2f", minAmount)
	}
	if isForbiddenWithdrawTime(setting.ForbiddenWithdrawTime, time.Now()) {
		return errors.New("当前时间禁止提现")
	}
	if cooldown, err := getPiggyWithdrawCooldownRemaining(userId, setting.CooldownMinutes); err != nil {
		return err
	} else if cooldown > 0 {
		return fmt.Errorf("提现冷却中，请 %d 秒后再试", cooldown)
	}
	return nil
}

func ensurePiggyWithdrawCommissionAvailable(userId int, amountCents int64) error {
	account, err := GetOrCreateWalletAccount(userId)
	if err != nil {
		return err
	}
	if account.CommissionAmount+walletAmountEpsilon < centsToFloat(amountCents) {
		return ErrCommissionInsufficient
	}
	return nil
}

func TrialPiggyWithdrawTax(ctx context.Context, req PiggyTaxTrialRequest) (*PiggyTaxTrialCalcResult, error) {
	if req.UserId <= 0 {
		return nil, errors.New("用户不存在")
	}
	amountCents := floatYuanToCents(req.Amount)
	if amountCents <= 0 {
		return nil, ErrWithdrawAmountInvalid
	}
	setting := operation_setting.GetPiggyWithdrawSetting()
	if !setting.Enabled {
		return nil, ErrPiggyWithdrawDisabled
	}
	if err := operation_setting.ValidatePiggyWithdrawSettingForEnable(setting); err != nil {
		return nil, err
	}
	if err := validatePiggyWithdrawRequestAvailability(req.UserId, amountCents, setting); err != nil {
		return nil, err
	}
	feeCalc, err := calculatePiggyPlatformFee(amountCents, setting.PlatformFeeRate)
	if err != nil {
		return nil, err
	}
	profile, err := getRawWithdrawalProfile(req.UserId)
	if err != nil {
		return nil, err
	}
	if !isWithdrawalProfileComplete(profile) {
		return nil, ErrWithdrawalProfileIncomplete
	}
	if !isPiggyContractSignedForCurrentScope(profile, setting) {
		return nil, ErrPiggyContractUnsigned
	}
	client, err := newConfiguredPiggyClient(setting)
	if err != nil {
		return nil, err
	}
	result, _, err := client.SingleTaxTrialCalc(ctx, PiggyTaxTrialCalcRequest{
		OuterTradeNo: buildPiggyTaxTrialNo(req.UserId),
		TaxFundId:    setting.TaxFundId,
		LicenseId:    profile.IdCardNo,
		CalcAmount:   feeCalc.PiggyTaxBeforeAmount,
		CalcType:     setting.CalcType,
	})
	if err != nil {
		return nil, err
	}
	result.RequestedAmount = feeCalc.RequestedAmount
	result.RequestedAmountCents = feeCalc.RequestedAmountCents
	result.PlatformFeeRate = feeCalc.PlatformFeeRate
	result.PlatformFeeAmount = feeCalc.PlatformFeeAmount
	result.PlatformFeeAmountCents = feeCalc.PlatformFeeAmountCents
	result.PiggyTaxBeforeAmount = feeCalc.PiggyTaxBeforeAmount
	result.PiggyTaxBeforeAmountCents = feeCalc.PiggyPayAmountCents
	return result, nil
}

func buildPiggyTaxTrialNo(userId int) string {
	return fmt.Sprintf("PTRIAL%d%d", userId, time.Now().UnixNano()%1_000_000_000_000)
}

func AdminApproveWithdrawOrder(ctx context.Context, id int, reviewerId int, remark string) error {
	_, err := AdminApproveWithdrawOrderWithResult(ctx, id, reviewerId, remark)
	return err
}

// AdminApproveWithdrawOrderWithResult 审核提现单，并返回外部通道提交语义，供管理端区分成功和可恢复未知状态。
func AdminApproveWithdrawOrderWithResult(ctx context.Context, id int, reviewerId int, remark string) (*WithdrawApprovalResult, error) {
	provider, err := getWithdrawOrderProvider(id)
	if err != nil {
		return nil, err
	}
	if provider == model.WithdrawProviderPiggyLaborV3 {
		return approvePiggyWithdrawOrder(ctx, id, reviewerId, remark)
	}
	if err := ApproveWithdrawOrder(id, reviewerId, remark); err != nil {
		return nil, err
	}
	return &WithdrawApprovalResult{
		Submitted:   false,
		Recoverable: false,
		Status:      model.WithdrawStatusApproved,
		Message:     "提现审核已通过",
	}, nil
}

func AdminRejectWithdrawOrder(ctx context.Context, id int, reviewerId int, reason string) error {
	provider, err := getWithdrawOrderProvider(id)
	if err != nil {
		return err
	}
	if provider == model.WithdrawProviderPiggyLaborV3 {
		return rejectPiggyWithdrawOrder(id, reviewerId, reason)
	}
	return RejectWithdrawOrder(id, reviewerId, reason)
}

func getWithdrawOrderProvider(id int) (string, error) {
	if id <= 0 {
		return "", ErrWithdrawOrderNotFound
	}
	var order model.WithdrawOrder
	if err := model.DB.Select("id", "provider").Where("id = ?", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrWithdrawOrderNotFound
		}
		return "", err
	}
	return strings.TrimSpace(order.Provider), nil
}

func approvePiggyWithdrawOrder(ctx context.Context, id int, reviewerId int, remark string) (*WithdrawApprovalResult, error) {
	setting := operation_setting.GetPiggyWithdrawSetting()
	if !setting.Enabled {
		return nil, ErrPiggyWithdrawDisabled
	}
	if err := operation_setting.ValidatePiggyWithdrawSettingForEnable(setting); err != nil {
		return nil, err
	}
	client, err := newConfiguredPiggyClient(setting)
	if err != nil {
		return nil, err
	}
	var order *model.WithdrawOrder
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		locked, err := lockWithdrawOrderTx(tx, id)
		if err != nil {
			return err
		}
		if locked.Provider != model.WithdrawProviderPiggyLaborV3 {
			return ErrWithdrawStatusInvalid
		}
		if locked.Status != model.WithdrawStatusPending {
			return ErrWithdrawStatusInvalid
		}
		if !isPiggyOrderPayoutSnapshotComplete(locked) {
			return ErrWithdrawPayoutSnapshotEmpty
		}
		now := common.GetTimestamp()
		if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", locked.Id).Updates(map[string]interface{}{
			"status":       model.WithdrawStatusApproved,
			"piggy_status": model.WithdrawStatusApproved,
			"reviewer_id":  reviewerId,
			"reviewed_at":  now,
			"remark":       strings.TrimSpace(remark),
		}).Error; err != nil {
			return err
		}
		locked.Status = model.WithdrawStatusApproved
		locked.PiggyStatus = model.WithdrawStatusApproved
		locked.ReviewerId = reviewerId
		locked.ReviewedAt = now
		locked.Remark = strings.TrimSpace(remark)
		order = locked
		return nil
	}); err != nil {
		return nil, err
	}

	return submitPiggyOrderToProvider(ctx, client, order, setting)
}

func submitPiggyOrderToProvider(ctx context.Context, client *PiggyClient, order *model.WithdrawOrder, setting *operation_setting.PiggyWithdrawSetting) (*WithdrawApprovalResult, error) {
	if order == nil || setting == nil {
		return nil, ErrWithdrawOrderNotFound
	}
	if !isPiggyOrderPayoutSnapshotComplete(order) {
		return nil, ErrWithdrawPayoutSnapshotEmpty
	}
	payAmount := strings.TrimSpace(order.PiggyPayAmount)
	if payAmount == "" {
		payAmount = centsToYuanString(order.PiggyPayAmountCents)
	}
	resp, reqDigest, err := client.SingleOrderSubmit(ctx, PiggySubmitOrderRequest{
		NotifyUrl:    setting.PayNotifyUrl,
		TaxFundId:    setting.TaxFundId,
		OuterTradeNo: order.WithdrawNo,
		EmpName:      strings.TrimSpace(order.AccountName),
		EmpPhone:     strings.TrimSpace(order.PayoutMobile),
		LicenseType:  "ID_CARD",
		LicenseId:    strings.TrimSpace(order.PayoutIdCardNo),
		SettleType:   model.WithdrawAccountTypeBankcard,
		PayAccount:   strings.TrimSpace(order.PayoutBankCardNo),
		BankName:     strings.TrimSpace(order.BankName),
		PositionName: setting.PositionName,
		PayAmount:    payAmount,
		BankRemo:     setting.BankRemark,
		CalcType:     setting.CalcType,
	})
	respDigest := ""
	if resp != nil {
		respDigest = digestPayload([]byte(resp.RawBody))
	}
	if err != nil {
		if resp != nil && !resp.Success() {
			failReason := piggySubmitFailureReason(resp, err)
			if isPiggyDuplicateOuterTradeNoFailure(resp, err) {
				return handlePiggyDuplicateOuterTradeNoSubmission(ctx, order, failReason, reqDigest, respDigest)
			}
			released, releaseErr := failPiggySubmittedOrderAndRelease(order.Id, failReason, reqDigest, respDigest)
			if releaseErr != nil {
				return nil, releaseErr
			}
			if !released {
				current, currentErr := piggyWithdrawApprovalResultFromCurrentOrder(order.Id, nil)
				if currentErr != nil {
					return nil, currentErr
				}
				if current != nil {
					return current, nil
				}
				return nil, ErrWithdrawStatusInvalid
			}
			return nil, err
		}
		reason := piggySubmitUnknownRecoverableReason(err)
		marked, markErr := markPiggyOrderSubmitRecoverableByIdIfActive(order.Id, reason, reqDigest, respDigest, model.WithdrawStatusApproved)
		if markErr != nil {
			return nil, markErr
		}
		result := &WithdrawApprovalResult{
			Submitted:   false,
			Recoverable: true,
			Status:      model.WithdrawStatusApproved,
			Message:     "小猪提交结果未知，请稍后恢复提交或检查网络",
		}
		if !marked {
			return piggyWithdrawApprovalResultFromCurrentOrder(order.Id, result)
		}
		return result, nil
	}
	submitUpdates := map[string]interface{}{
		"request_payload_digest":  reqDigest,
		"response_payload_digest": respDigest,
		"external_trade_no":       order.WithdrawNo,
		"status":                  model.WithdrawStatusSubmitted,
		"piggy_status":            model.WithdrawStatusSubmitted,
		"manual_review_reason":    "",
		"fail_reason":             "",
		"compensation_status":     "",
		"submitted_at":            common.GetTimestamp(),
	}
	result := model.DB.Model(&model.WithdrawOrder{}).
		Where("id = ? AND provider = ? AND status = ?", order.Id, model.WithdrawProviderPiggyLaborV3, model.WithdrawStatusApproved).
		Updates(submitUpdates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if err := model.DB.Model(&model.WithdrawOrder{}).
			Where("id = ? AND provider = ?", order.Id, model.WithdrawProviderPiggyLaborV3).
			UpdateColumns(map[string]interface{}{
				"request_payload_digest":  reqDigest,
				"response_payload_digest": respDigest,
			}).Error; err != nil {
			return nil, err
		}
		return piggyWithdrawApprovalResultFromCurrentOrder(order.Id, &WithdrawApprovalResult{
			Submitted:   true,
			Recoverable: false,
			Status:      model.WithdrawStatusSubmitted,
			Message:     "小猪提现已提交",
		})
	}
	return &WithdrawApprovalResult{
		Submitted:   true,
		Recoverable: false,
		Status:      model.WithdrawStatusSubmitted,
		Message:     "小猪提现已提交",
	}, nil
}

func handlePiggyDuplicateOuterTradeNoSubmission(ctx context.Context, order *model.WithdrawOrder, failReason string, reqDigest string, respDigest string) (*WithdrawApprovalResult, error) {
	if order == nil {
		return nil, ErrWithdrawOrderNotFound
	}
	statusProved, queryReqDigest, queryRespDigest, queryErr := queryPiggyOrderStatusWithEvidence(ctx, order.WithdrawNo)
	current, currentErr := getPiggyOrderById(order.Id)
	if currentErr != nil {
		return nil, currentErr
	}
	if current.Status != model.WithdrawStatusApproved {
		result := piggyWithdrawApprovalResultFromOrder(current)
		if result != nil {
			return result, nil
		}
		return nil, ErrWithdrawStatusInvalid
	}
	manualReason := "小猪订单提交单号重复，已用原流水号查询但未确认远端订单状态，请人工核对: " + strings.TrimSpace(failReason)
	if queryErr != nil {
		manualReason = "小猪订单提交单号重复，原流水号查询失败，请人工核对远端订单状态: " + queryErr.Error() + "; " + strings.TrimSpace(failReason)
	} else if !statusProved {
		manualReason = "小猪订单提交单号重复，原流水号查询结果无法判定远端业务状态，请人工核对: " + strings.TrimSpace(failReason)
	}
	reqDigest, respDigest = piggyPreferQueryEvidence(reqDigest, respDigest, queryReqDigest, queryRespDigest, queryErr)
	marked, markErr := markPiggyOrderManualReviewByIdIfActiveWithResult(order.Id, manualReason, reqDigest, respDigest, model.WithdrawStatusApproved)
	if markErr != nil {
		return nil, markErr
	}
	if !marked {
		return piggyWithdrawApprovalResultFromCurrentOrder(order.Id, nil)
	}
	return &WithdrawApprovalResult{
		Submitted:   false,
		Recoverable: false,
		Status:      model.WithdrawStatusManualReview,
		Message:     manualReason,
	}, nil
}

func rejectPiggyWithdrawOrder(id int, reviewerId int, reason string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderTx(tx, id)
		if err != nil {
			return err
		}
		if order.Provider != model.WithdrawProviderPiggyLaborV3 {
			return ErrWithdrawStatusInvalid
		}
		if order.Status != model.WithdrawStatusPending {
			return ErrWithdrawStatusInvalid
		}
		account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
		if err != nil {
			return err
		}
		amount := piggyWithdrawOrderAmount(order)
		account.FrozenCommissionAmount = maxFloat(0, account.FrozenCommissionAmount-amount)
		account.CommissionAmount += amount
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		cleanReason := strings.TrimSpace(reason)
		if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
			"status":       model.WithdrawStatusRejected,
			"piggy_status": model.WithdrawStatusRejected,
			"reviewer_id":  reviewerId,
			"reviewed_at":  now,
			"terminal_at":  now,
			"fail_reason":  cleanReason,
			"remark":       cleanReason,
		}).Error; err != nil {
			return err
		}
		return createWalletFlowTx(tx, &model.WalletFlow{
			UserId:                order.UserId,
			BizNo:                 order.WithdrawNo,
			IdempotencyKey:        walletIdempotencyKey("wallet:piggy-withdraw-reject:" + order.WithdrawNo),
			FlowType:              model.WalletFlowTypeWithdrawReject,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionIn,
			Amount:                amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                cleanReason,
		})
	})
}

func HandlePiggyContractCallback(body []byte, signature string) error {
	setting := operation_setting.GetPiggyWithdrawSetting()
	log := &model.PiggyWithdrawCallbackLog{
		CallbackType:  model.PiggyCallbackTypeContract,
		PayloadDigest: digestPayload(body),
		ProcessStatus: model.PaymentProcessStatusPending,
	}
	if err := model.DB.Create(log).Error; err != nil {
		return err
	}
	if err := validatePiggyCallbackSecret(setting.AppSecret); err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), nil)
		return err
	}
	var payload PiggyContractCallbackPayload
	if err := common.Unmarshal(body, &payload); err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), nil)
		return err
	}
	callback := normalizePiggyContractCallbackPayload(payload)
	hasSignature := strings.TrimSpace(signature) != "" || strings.TrimSpace(callback.Sign) != ""
	if hasSignature {
		if err := verifyPiggyCallbackSignature(body, signature, callback.Sign, setting.AppSecret); err != nil {
			finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), nil)
			return err
		}
	}
	userId := piggyCallbackUserId(callback.CustomParams)
	profile, err := findPiggyContractCallbackProfile(callback, hasSignature)
	if err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), map[string]interface{}{"user_id": userId})
		return err
	}
	status := firstNonEmpty(callback.SignStatus, callback.Status, callback.Code)
	if !hasSignature {
		// 无签名兼容只针对小猪官方回调结构，成功标识以顶层 code=0 为准；
		// 不使用 data.signStatus/status 兜底，避免非官方结构绕过无签名安全边界。
		status = callback.Code
	}
	signStatus, err := normalizePiggyContractSignStatus(status, hasSignature)
	if err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), map[string]interface{}{"user_id": profile.UserId})
		return err
	}
	updates := map[string]interface{}{
		"piggy_sign_status":    signStatus,
		"last_callback_digest": digestPayload(body),
	}
	if signStatus == model.PiggySignStatusSigned {
		updates["piggy_signed_at"] = common.GetTimestamp()
		updates["piggy_contract_url"] = strings.TrimSpace(callback.ContractURL)
		updates["piggy_contract_document_id"] = strings.TrimSpace(callback.DocumentID)
		updates["piggy_contract_subsidiary_name"] = strings.TrimSpace(callback.SubsidiaryName)
		updates["piggy_contract_position"] = piggyContractServiceType(setting)
		updates["piggy_contract_position_name"] = strings.TrimSpace(setting.PositionName)
		updates["piggy_contract_tax_fund_id"] = strings.TrimSpace(setting.TaxFundId)
	}
	if err := model.DB.Model(&model.WithdrawalProfile{}).Where("id = ?", profile.Id).Updates(updates).Error; err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), map[string]interface{}{"user_id": profile.UserId})
		return err
	}
	finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusSuccess, "", map[string]interface{}{"user_id": profile.UserId})
	return nil
}

func HandlePiggyPaymentCallback(ctx context.Context, body []byte, signature string) error {
	setting := operation_setting.GetPiggyWithdrawSetting()
	log := &model.PiggyWithdrawCallbackLog{
		CallbackType:  model.PiggyCallbackTypePayment,
		PayloadDigest: digestPayload(body),
		ProcessStatus: model.PaymentProcessStatusPending,
	}
	if err := model.DB.Create(log).Error; err != nil {
		return err
	}
	if err := validatePiggyPaymentCallbackCrypto(setting); err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), nil)
		return err
	}
	var envelope PiggyPaymentCallbackEnvelope
	if err := common.Unmarshal(body, &envelope); err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), nil)
		return err
	}
	hasSignature := strings.TrimSpace(signature) != "" || strings.TrimSpace(envelope.Sign) != ""
	if hasSignature {
		if err := verifyPiggyCallbackSignature(body, signature, envelope.Sign, setting.AppSecret); err != nil {
			finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), nil)
			return err
		}
	}
	plain, err := piggyDecryptAES(envelope.Data.BizAESContent, setting.AppSecret, setting.AESIV)
	if err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), nil)
		return err
	}
	var content PiggyPaymentCallbackContent
	if err := common.Unmarshal(plain, &content); err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), map[string]interface{}{"decrypted_digest": digestPayload(plain)})
		return err
	}
	if !hasSignature {
		if err := validateUnsignedPiggyPaymentCallbackContent(content); err != nil {
			finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), map[string]interface{}{
				"order_no":         strings.TrimSpace(content.OuterTradeNo),
				"notify_type":      strings.TrimSpace(content.NotifyType),
				"trade_status":     strings.TrimSpace(content.TradeStatus),
				"front_log_no":     strings.TrimSpace(content.FrontLogNo),
				"labor_order_no":   strings.TrimSpace(content.LaborOrderNo),
				"decrypted_digest": digestPayload(plain),
			})
			return err
		}
	}
	idempotencyKey := buildPiggyPaymentCallbackIdempotencyKey(content)
	updates := map[string]interface{}{
		"order_no":         strings.TrimSpace(content.OuterTradeNo),
		"notify_type":      strings.TrimSpace(content.NotifyType),
		"trade_status":     strings.TrimSpace(content.TradeStatus),
		"front_log_no":     strings.TrimSpace(content.FrontLogNo),
		"labor_order_no":   strings.TrimSpace(content.LaborOrderNo),
		"idempotency_key":  idempotencyKey,
		"decrypted_digest": digestPayload(plain),
	}
	if duplicate, err := hasProcessedPiggyCallback(idempotencyKey); err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), updates)
		return err
	} else if duplicate {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusSuccess, "", updates)
		return nil
	}
	if err := processPiggyPaymentCallbackContent(ctx, content); err != nil {
		finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusFailed, err.Error(), updates)
		return err
	}
	finishPiggyCallbackLog(log.Id, model.PaymentProcessStatusSuccess, "", updates)
	return nil
}

func AdminListPiggyCallbackLogs(callbackType string, orderNo string, processStatus string, pageInfo *common.PageInfo) ([]model.PiggyWithdrawCallbackLog, int64, error) {
	query := model.DB.Model(&model.PiggyWithdrawCallbackLog{})
	if callbackType = strings.TrimSpace(callbackType); callbackType != "" {
		query = query.Where("callback_type = ?", callbackType)
	}
	if orderNo = strings.TrimSpace(orderNo); orderNo != "" {
		query = query.Where("order_no = ?", orderNo)
	}
	if processStatus = strings.TrimSpace(processStatus); processStatus != "" {
		query = query.Where("process_status = ?", processStatus)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []model.PiggyWithdrawCallbackLog
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func AdminRecordPiggyManualResult(orderId int, adminId int, result string, compensationStatus string) error {
	manualResult := strings.TrimSpace(result)
	manualStatus := strings.TrimSpace(compensationStatus)
	return model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderTx(tx, orderId)
		if err != nil {
			return err
		}
		if order.Provider != model.WithdrawProviderPiggyLaborV3 {
			return ErrWithdrawStatusInvalid
		}
		if order.Status != model.WithdrawStatusManualReview || isPiggyTerminalStatus(order.Status) {
			return ErrWithdrawStatusInvalid
		}
		switch manualStatus {
		case "", piggyCompensationStatusPending:
			return recordPiggyManualReviewNoteTx(tx, order, adminId, manualResult)
		case piggyManualResultPaid, model.WithdrawStatusPaid:
			return markPiggyManualResultPaidTx(tx, order, adminId, manualResult)
		case piggyManualResultFailed, model.WithdrawStatusFailed:
			return failPiggyManualResultAndReleaseTx(tx, order, adminId, manualResult)
		default:
			return ErrWithdrawStatusInvalid
		}
	})
}

func recordPiggyManualReviewNoteTx(tx *gorm.DB, order *model.WithdrawOrder, adminId int, result string) error {
	now := common.GetTimestamp()
	return tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"manual_handled_by":    adminId,
		"manual_handled_at":    now,
		"manual_handle_result": strings.TrimSpace(result),
		"compensation_status":  piggyCompensationStatusPending,
		"manual_review_reason": strings.TrimSpace(result),
	}).Error
}

func markPiggyManualResultPaidTx(tx *gorm.DB, order *model.WithdrawOrder, adminId int, result string) error {
	account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
	if err != nil {
		return err
	}
	amount := piggyWithdrawOrderAmount(order)
	if account.FrozenCommissionAmount+walletAmountEpsilon < amount {
		return ErrCommissionInsufficient
	}
	account.FrozenCommissionAmount = maxFloat(0, account.FrozenCommissionAmount-amount)
	account.TotalWithdrawAmount += amount
	if err := tx.Save(account).Error; err != nil {
		return err
	}
	now := common.GetTimestamp()
	remark := firstNonEmpty(result, "人工确认小猪提现已付款")
	if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":                model.WithdrawStatusPaid,
		"piggy_status":          model.WithdrawStatusPaid,
		"reviewer_id":           adminId,
		"reviewed_at":           now,
		"paid_at":               now,
		"terminal_at":           now,
		"manual_handled_by":     adminId,
		"manual_handled_at":     now,
		"manual_handle_result":  remark,
		"manual_review_reason":  remark,
		"compensation_status":   piggyCompensationStatusManualProcessed,
		"payment_voucher":       remark,
		"trade_result_describe": remark,
	}).Error; err != nil {
		return err
	}
	return createWalletFlowTx(tx, &model.WalletFlow{
		UserId:                order.UserId,
		BizNo:                 order.WithdrawNo,
		IdempotencyKey:        walletIdempotencyKey("wallet:piggy-withdraw-success:" + order.WithdrawNo),
		FlowType:              model.WalletFlowTypeWithdrawSuccess,
		WalletType:            model.WalletTypeCommission,
		Direction:             model.WalletFlowDirectionOut,
		Amount:                amount,
		BalanceAfter:          account.BalanceAmount,
		CommissionAfter:       account.CommissionAmount,
		FrozenCommissionAfter: account.FrozenCommissionAmount,
		Remark:                remark,
	})
}

func failPiggyManualResultAndReleaseTx(tx *gorm.DB, order *model.WithdrawOrder, adminId int, result string) error {
	account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
	if err != nil {
		return err
	}
	amount := piggyWithdrawOrderAmount(order)
	account.FrozenCommissionAmount = maxFloat(0, account.FrozenCommissionAmount-amount)
	account.CommissionAmount += amount
	if err := tx.Save(account).Error; err != nil {
		return err
	}
	now := common.GetTimestamp()
	reason := firstNonEmpty(result, "人工确认小猪提现失败")
	if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":                model.WithdrawStatusFailed,
		"piggy_status":          model.WithdrawStatusFailed,
		"reviewer_id":           adminId,
		"reviewed_at":           now,
		"terminal_at":           now,
		"manual_handled_by":     adminId,
		"manual_handled_at":     now,
		"manual_handle_result":  reason,
		"manual_review_reason":  reason,
		"compensation_status":   piggyCompensationStatusManualProcessed,
		"fail_reason":           reason,
		"trade_result_describe": reason,
	}).Error; err != nil {
		return err
	}
	return createWalletFlowTx(tx, &model.WalletFlow{
		UserId:                order.UserId,
		BizNo:                 order.WithdrawNo,
		IdempotencyKey:        walletIdempotencyKey("wallet:piggy-withdraw-failed:" + order.WithdrawNo),
		FlowType:              model.WalletFlowTypeWithdrawReject,
		WalletType:            model.WalletTypeCommission,
		Direction:             model.WalletFlowDirectionIn,
		Amount:                amount,
		BalanceAfter:          account.BalanceAmount,
		CommissionAfter:       account.CommissionAmount,
		FrozenCommissionAfter: account.FrozenCommissionAmount,
		Remark:                reason,
	})
}

func AdminRetryPiggyConfirm(ctx context.Context, orderId int) error {
	order, err := getPiggyOrderById(orderId)
	if err != nil {
		return err
	}
	if order.Status != model.WithdrawStatusAwaitConfirm {
		return ErrWithdrawStatusInvalid
	}
	return confirmPiggyOrder(ctx, order.WithdrawNo)
}

func AdminRecoverPiggyApprovedSubmission(ctx context.Context, orderId int) error {
	_, err := AdminRecoverPiggyApprovedSubmissionWithResult(ctx, orderId)
	return err
}

// AdminRecoverPiggyApprovedSubmissionWithResult 恢复提交已审核但未拿到小猪外部单号的提现单。
func AdminRecoverPiggyApprovedSubmissionWithResult(ctx context.Context, orderId int) (*WithdrawApprovalResult, error) {
	if orderId <= 0 {
		return nil, ErrWithdrawOrderNotFound
	}
	setting := operation_setting.GetPiggyWithdrawSetting()
	if !setting.Enabled {
		return nil, ErrPiggyWithdrawDisabled
	}
	if err := operation_setting.ValidatePiggyWithdrawSettingForEnable(setting); err != nil {
		return nil, err
	}
	client, err := newConfiguredPiggyClient(setting)
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	deadline := now - piggyApprovedSubmissionRecoveryDelay(setting)
	result := model.DB.Model(&model.WithdrawOrder{}).
		Where("id = ? AND provider = ? AND status = ?", orderId, model.WithdrawProviderPiggyLaborV3, model.WithdrawStatusApproved).
		Where("reviewed_at > 0 AND reviewed_at <= ?", deadline).
		Where("(updated_at = 0 OR updated_at <= ?)", deadline).
		Where(`(
			compensation_status = ?
			OR compensation_status = ?
			OR compensation_status IS NULL
			OR (compensation_status = ? AND (updated_at = 0 OR updated_at <= ?))
		)`, "", piggyCompensationStatusPending, piggyCompensationStatusSubmitRecovering, deadline).
		Updates(map[string]interface{}{
			"compensation_status": piggyCompensationStatusSubmitRecovering,
			"updated_at":          now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		current, err := getPiggyOrderById(orderId)
		if err != nil {
			return nil, err
		}
		if current.Status != model.WithdrawStatusApproved {
			// 回调可能已先于恢复操作推进订单状态，此时返回当前状态，避免误提示恢复失败。
			result := piggyWithdrawApprovalResultFromOrder(current)
			if result != nil {
				return result, nil
			}
			return nil, ErrWithdrawStatusInvalid
		}
		return nil, ErrWithdrawStatusInvalid
	}
	order, err := getPiggyOrderById(orderId)
	if err != nil {
		return nil, err
	}
	statusProved, queryReqDigest, queryRespDigest, queryErr := queryPiggyOrderStatusWithEvidence(ctx, order.WithdrawNo)
	if queryErr != nil || !statusProved {
		reasonErr := queryErr
		if reasonErr == nil {
			reasonErr = ErrPiggyQueryStatusUnknown
		}
		if detail := strings.TrimSpace(reasonErr.Error()); detail != "" {
			reasonErr = fmt.Errorf("查询失败: %s", detail)
		}
		reason := piggySubmitUnknownRecoverableReason(reasonErr)
		if piggyIsOrderNotFoundQueryError(queryErr) {
			reason = "小猪订单查询失败，远端返回劳务订单不存在，请人工核对并收口: " + strings.TrimSpace(queryErr.Error())
			if _, markErr := markPiggyOrderManualReviewByIdIfActiveWithResult(orderId, reason, queryReqDigest, queryRespDigest, model.WithdrawStatusApproved); markErr != nil {
				return nil, markErr
			}
			current, currentErr := getPiggyOrderById(orderId)
			if currentErr != nil {
				return nil, currentErr
			}
			result := piggyWithdrawApprovalResultFromOrder(current)
			if result != nil {
				return result, nil
			}
			return nil, ErrWithdrawStatusInvalid
		}
		if _, markErr := markPiggyOrderSubmitRecoverableByIdIfActive(orderId, reason, queryReqDigest, queryRespDigest, model.WithdrawStatusApproved); markErr != nil {
			return nil, markErr
		}
		current, currentErr := getPiggyOrderById(orderId)
		if currentErr != nil {
			return nil, currentErr
		}
		if current.Status != model.WithdrawStatusApproved {
			result := piggyWithdrawApprovalResultFromOrder(current)
			if result != nil {
				return result, nil
			}
			return nil, ErrWithdrawStatusInvalid
		}
		return &WithdrawApprovalResult{
			Submitted:   false,
			Recoverable: true,
			Status:      model.WithdrawStatusApproved,
			Message:     "小猪提交结果未知，请稍后恢复提交或检查网络",
		}, nil
	}
	current, currentErr := getPiggyOrderById(orderId)
	if currentErr != nil {
		return nil, currentErr
	}
	if current.Status != model.WithdrawStatusApproved || strings.TrimSpace(current.ExternalTradeNo) != "" {
		result := piggyWithdrawApprovalResultFromOrder(current)
		if result != nil {
			return result, nil
		}
		return nil, ErrWithdrawStatusInvalid
	}
	order = current
	return submitPiggyOrderToProvider(ctx, client, order, setting)
}

func AdminCancelPiggyOrder(ctx context.Context, orderId int, adminId int, reason string) error {
	client, err := newConfiguredPiggyClient(operation_setting.GetPiggyWithdrawSetting())
	if err != nil {
		return err
	}
	order, err := claimPiggyOrderForCancel(orderId)
	if err != nil {
		return err
	}
	resp, reqDigest, err := client.SingleOrderCancel(ctx, order.WithdrawNo)
	respDigest := ""
	if resp != nil {
		respDigest = digestPayload([]byte(resp.RawBody))
	}
	if err != nil {
		markPiggyCancellingOrderManualReview(order.Id, "取消小猪订单失败: "+err.Error(), reqDigest, respDigest)
		return err
	}
	return cancelPiggyOrderAndRelease(order.WithdrawNo, adminId, strings.TrimSpace(reason), reqDigest, respDigest)
}

func ScanPiggyWithdrawCompensations(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	setting := operation_setting.GetPiggyWithdrawSetting()
	deadline := common.GetTimestamp() - piggyApprovedSubmissionRecoveryDelay(setting)
	var orders []model.WithdrawOrder
	err := model.DB.Where("provider = ?", model.WithdrawProviderPiggyLaborV3).
		Where(model.DB.
			Where(`status = ? AND reviewed_at > 0 AND reviewed_at <= ? AND (updated_at = 0 OR updated_at <= ?) AND (
				compensation_status = ?
				OR compensation_status = ?
				OR compensation_status IS NULL
				OR (compensation_status = ? AND (updated_at = 0 OR updated_at <= ?))
			)`, model.WithdrawStatusApproved, deadline, deadline, "", piggyCompensationStatusPending, piggyCompensationStatusSubmitRecovering, deadline).
			Or("status IN ?", []string{
				model.WithdrawStatusSubmitted,
				model.WithdrawStatusAwaitConfirm,
				model.WithdrawStatusConfirming,
				model.WithdrawStatusCancelling,
				model.WithdrawStatusConfirmed,
			}).
			Or("(status = ? AND (compensation_status = ? OR compensation_status = ? OR compensation_status IS NULL))", model.WithdrawStatusManualReview, "", piggyCompensationStatusPending),
		).
		Order("id asc").
		Limit(limit).
		Find(&orders).Error
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, order := range orders {
		if order.Status == model.WithdrawStatusApproved {
			if !shouldScanPiggyApprovedOrder(&order) {
				continue
			}
			if result, err := AdminRecoverPiggyApprovedSubmissionWithResult(ctx, order.Id); err == nil &&
				result != nil &&
				result.Status != model.WithdrawStatusApproved {
				processed++
			}
			continue
		}
		if order.Status == model.WithdrawStatusAwaitConfirm {
			if err := confirmPiggyOrder(ctx, order.WithdrawNo); err == nil {
				processed++
			}
			continue
		}
		if order.Status == model.WithdrawStatusConfirming {
			if !isPiggyConfirmingExpired(&order) {
				continue
			}
			if err := queryPiggyOrderStatus(ctx, order.WithdrawNo); err == nil {
				processed++
			}
			continue
		}
		if order.Status == model.WithdrawStatusCancelling {
			if !isPiggyCancellingExpired(&order) {
				continue
			}
			if err := queryPiggyOrderStatus(ctx, order.WithdrawNo); err == nil {
				_ = markExpiredPiggyCancellingManualReview(order.WithdrawNo)
				processed++
			}
			continue
		}
		if order.Status == model.WithdrawStatusSubmitted ||
			order.Status == model.WithdrawStatusConfirmed {
			if err := queryPiggyOrderStatus(ctx, order.WithdrawNo); err == nil {
				processed++
			}
			continue
		}
		if order.Status == model.WithdrawStatusManualReview && shouldScanPiggyManualReviewOrder(&order) {
			if err := queryPiggyOrderStatus(ctx, order.WithdrawNo); err == nil {
				processed++
			}
		}
	}
	return processed, nil
}

func StartPiggyWithdrawCompensationTask() {
	piggyWithdrawCompensationTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		go func() {
			common.SysLog(fmt.Sprintf("piggy withdraw compensation task started: tick=%s", piggyWithdrawCompensationTickInterval))
			ticker := time.NewTicker(piggyWithdrawCompensationTickInterval)
			defer ticker.Stop()
			runPiggyWithdrawCompensationOnce()
			for range ticker.C {
				runPiggyWithdrawCompensationOnce()
			}
		}()
	})
}

func runPiggyWithdrawCompensationOnce() {
	if !operation_setting.GetPiggyWithdrawSetting().Enabled {
		return
	}
	if !piggyWithdrawCompensationTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer piggyWithdrawCompensationTaskRunning.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), 2*piggyWithdrawCompensationTickInterval)
	defer cancel()
	processed, err := ScanPiggyWithdrawCompensations(ctx, piggyWithdrawCompensationScanLimit)
	if err != nil {
		common.SysLog("piggy withdraw compensation scan failed: " + err.Error())
		return
	}
	if processed > 0 {
		common.SysLog(fmt.Sprintf("piggy withdraw compensation scan processed=%d", processed))
	}
}

func processPiggyPaymentCallbackContent(ctx context.Context, content PiggyPaymentCallbackContent) error {
	notifyType := strings.TrimSpace(content.NotifyType)
	tradeStatus := strings.TrimSpace(content.TradeStatus)
	if notifyType == "" {
		reason := "小猪回调缺少 notifyType"
		if tradeStatus == "" {
			reason = "小猪回调缺少 notifyType/tradeStatus"
		}
		if markErr := markPiggyOrderManualReviewByNo(content.OuterTradeNo, reason); markErr != nil {
			return markErr
		}
		return errors.New(reason)
	}
	switch notifyType {
	case "submitResult":
		switch tradeStatus {
		case "await":
			shouldConfirm, err := markPiggyOrderAwaitConfirm(content)
			if err != nil {
				return err
			}
			if !shouldConfirm {
				return nil
			}
			return confirmPiggyOrder(ctx, content.OuterTradeNo)
		case "failure":
			return failPiggyOrderAndRelease(content, "小猪订单提交失败")
		default:
			return markPiggyOrderManualReviewByNo(content.OuterTradeNo, "未知 submitResult 状态: "+tradeStatus)
		}
	case "tradeResult":
		switch tradeStatus {
		case "success":
			return markPiggyOrderPaid(content)
		case "failure":
			return failPiggyOrderAndRelease(content, "小猪最终打款失败")
		default:
			return markPiggyOrderManualReviewByNo(content.OuterTradeNo, "未知 tradeResult 状态: "+tradeStatus)
		}
	default:
		return markPiggyOrderManualReviewByNo(content.OuterTradeNo, "未知小猪回调类型: "+notifyType)
	}
}

func markPiggyOrderAwaitConfirm(content PiggyPaymentCallbackContent) (bool, error) {
	shouldConfirm := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderByNoTx(tx, content.OuterTradeNo)
		if err != nil {
			return err
		}
		if isPiggyTerminalStatus(order.Status) || order.Status == model.WithdrawStatusConfirmed {
			return nil
		}
		if order.Status == model.WithdrawStatusCancelling {
			return nil
		}
		if order.Status == model.WithdrawStatusConfirming && !isPiggyConfirmingExpired(order) {
			return nil
		}
		if order.Status != model.WithdrawStatusSubmitted &&
			order.Status != model.WithdrawStatusAwaitConfirm &&
			order.Status != model.WithdrawStatusApproved &&
			order.Status != model.WithdrawStatusManualReview &&
			order.Status != model.WithdrawStatusConfirming {
			if order.Status == model.WithdrawStatusPending {
				return markPiggyOrderManualReviewFromCallbackTx(tx, order, "小猪回调早于管理员审核提交，请人工核对远端订单状态", content)
			}
			return ErrWithdrawStatusInvalid
		}
		if err := validatePiggyCallbackPretaxAmount(order, content); err != nil {
			return markPiggyOrderManualReviewFromCallbackTx(tx, order, err.Error(), content)
		}
		updates := piggyOrderCallbackAmountUpdates(content)
		updates["status"] = model.WithdrawStatusAwaitConfirm
		updates["piggy_status"] = model.WithdrawStatusAwaitConfirm
		updates["notify_type"] = content.NotifyType
		updates["trade_status"] = content.TradeStatus
		updates["front_log_no"] = strings.TrimSpace(content.FrontLogNo)
		updates["labor_order_no"] = strings.TrimSpace(content.LaborOrderNo)
		addPiggyApprovedSubmissionMarkers(order, updates)
		if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(updates).Error; err != nil {
			return err
		}
		shouldConfirm = true
		return nil
	})
	return shouldConfirm, err
}

func confirmPiggyOrder(ctx context.Context, withdrawNo string) error {
	client, err := newConfiguredPiggyClient(operation_setting.GetPiggyWithdrawSetting())
	if err != nil {
		return err
	}
	order, claimed, err := claimPiggyOrderForConfirm(withdrawNo)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	resp, reqDigest, err := client.SingleOrderConfirmPay(ctx, withdrawNo)
	respDigest := ""
	if resp != nil {
		respDigest = digestPayload([]byte(resp.RawBody))
	}
	if err != nil {
		markPiggyConfirmingOrderManualReview(order.Id, "小猪确认打款失败: "+err.Error(), reqDigest, respDigest)
		return err
	}
	return model.DB.Model(&model.WithdrawOrder{}).Where("id = ? AND status = ?", order.Id, model.WithdrawStatusConfirming).Updates(map[string]interface{}{
		"status":                  model.WithdrawStatusConfirmed,
		"piggy_status":            model.WithdrawStatusConfirmed,
		"confirmed_at":            common.GetTimestamp(),
		"request_payload_digest":  reqDigest,
		"response_payload_digest": respDigest,
	}).Error
}

func claimPiggyOrderForConfirm(withdrawNo string) (*model.WithdrawOrder, bool, error) {
	withdrawNo = strings.TrimSpace(withdrawNo)
	if withdrawNo == "" {
		return nil, false, ErrWithdrawOrderNotFound
	}
	now := common.GetTimestamp()
	result := model.DB.Model(&model.WithdrawOrder{}).
		Where("withdraw_no = ? AND provider = ? AND status = ?", withdrawNo, model.WithdrawProviderPiggyLaborV3, model.WithdrawStatusAwaitConfirm).
		Updates(map[string]interface{}{
			"status":       model.WithdrawStatusConfirming,
			"piggy_status": model.WithdrawStatusConfirming,
			"updated_at":   now,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	order, err := getPiggyOrderByNo(withdrawNo)
	if err != nil {
		return nil, false, err
	}
	if result.RowsAffected == 0 {
		switch order.Status {
		case model.WithdrawStatusConfirming, model.WithdrawStatusCancelling, model.WithdrawStatusConfirmed, model.WithdrawStatusManualReview:
			return order, false, nil
		default:
			if isPiggyTerminalStatus(order.Status) {
				return order, false, nil
			}
			return nil, false, ErrWithdrawStatusInvalid
		}
	}
	return order, true, nil
}

func claimPiggyOrderForCancel(orderId int) (*model.WithdrawOrder, error) {
	if orderId <= 0 {
		return nil, ErrWithdrawOrderNotFound
	}
	now := common.GetTimestamp()
	result := model.DB.Model(&model.WithdrawOrder{}).
		Where("id = ? AND provider = ? AND status IN ?", orderId, model.WithdrawProviderPiggyLaborV3, []string{
			model.WithdrawStatusSubmitted,
			model.WithdrawStatusAwaitConfirm,
			model.WithdrawStatusManualReview,
		}).
		Updates(map[string]interface{}{
			"status":       model.WithdrawStatusCancelling,
			"piggy_status": model.WithdrawStatusCancelling,
			"updated_at":   now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if _, err := getPiggyOrderById(orderId); err != nil {
			return nil, err
		}
		return nil, ErrWithdrawStatusInvalid
	}
	return getPiggyOrderById(orderId)
}

func markPiggyOrderPaid(content PiggyPaymentCallbackContent) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderByNoTx(tx, content.OuterTradeNo)
		if err != nil {
			return err
		}
		if isPiggyTerminalStatus(order.Status) {
			return nil
		}
		if order.Status == model.WithdrawStatusPending {
			return markPiggyOrderManualReviewFromCallbackTx(tx, order, "小猪回调早于管理员审核提交，请人工核对远端订单状态", content)
		}
		if err := validatePiggyCallbackPretaxAmount(order, content); err != nil {
			return markPiggyOrderManualReviewFromCallbackTx(tx, order, err.Error(), content)
		}
		account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
		if err != nil {
			return err
		}
		amount := order.Amount
		if amount <= 0 {
			amount = centsToFloat(order.FrozenAmountCents)
		}
		if account.FrozenCommissionAmount+walletAmountEpsilon < amount {
			return ErrCommissionInsufficient
		}
		account.FrozenCommissionAmount = maxFloat(0, account.FrozenCommissionAmount-amount)
		account.TotalWithdrawAmount += amount
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		updates := piggyOrderCallbackAmountUpdates(content)
		updates["status"] = model.WithdrawStatusPaid
		updates["piggy_status"] = model.WithdrawStatusPaid
		updates["actual_amount"] = centsToFloat(toCentsIgnoreError(content.AfterTaxAmount))
		updates["fee_amount"] = centsToFloat(toCentsIgnoreError(content.FeeAmount))
		updates["notify_type"] = content.NotifyType
		updates["trade_status"] = content.TradeStatus
		updates["front_log_no"] = strings.TrimSpace(content.FrontLogNo)
		updates["labor_order_no"] = strings.TrimSpace(content.LaborOrderNo)
		updates["trade_result"] = strings.TrimSpace(content.TradeResult)
		updates["trade_result_describe"] = strings.TrimSpace(content.TradeResultDescribe)
		updates["paid_at"] = common.GetTimestamp()
		updates["terminal_at"] = common.GetTimestamp()
		addPiggyApprovedSubmissionMarkers(order, updates)
		if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(updates).Error; err != nil {
			return err
		}
		return createWalletFlowTx(tx, &model.WalletFlow{
			UserId:                order.UserId,
			BizNo:                 order.WithdrawNo,
			IdempotencyKey:        walletIdempotencyKey("wallet:piggy-withdraw-success:" + order.WithdrawNo),
			FlowType:              model.WalletFlowTypeWithdrawSuccess,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionOut,
			Amount:                amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                "小猪提现最终打款成功",
		})
	})
}

func failPiggyOrderAndRelease(content PiggyPaymentCallbackContent, defaultReason string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderByNoTx(tx, content.OuterTradeNo)
		if err != nil {
			return err
		}
		if isPiggyTerminalStatus(order.Status) {
			return nil
		}
		if order.Status == model.WithdrawStatusPending {
			return markPiggyOrderManualReviewFromCallbackTx(tx, order, "小猪回调早于管理员审核提交，请人工核对远端订单状态", content)
		}
		account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
		if err != nil {
			return err
		}
		amount := order.Amount
		if amount <= 0 {
			amount = centsToFloat(order.FrozenAmountCents)
		}
		account.FrozenCommissionAmount = maxFloat(0, account.FrozenCommissionAmount-amount)
		account.CommissionAmount += amount
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		failReason := firstNonEmpty(content.TradeResultDescribe, content.TradeFailCode, defaultReason)
		updates := piggyOrderCallbackAmountUpdates(content)
		updates["status"] = model.WithdrawStatusFailed
		updates["piggy_status"] = model.WithdrawStatusFailed
		updates["notify_type"] = content.NotifyType
		updates["trade_status"] = content.TradeStatus
		updates["front_log_no"] = strings.TrimSpace(content.FrontLogNo)
		updates["labor_order_no"] = strings.TrimSpace(content.LaborOrderNo)
		updates["trade_fail_code"] = strings.TrimSpace(content.TradeFailCode)
		updates["trade_result"] = strings.TrimSpace(content.TradeResult)
		updates["trade_result_describe"] = strings.TrimSpace(content.TradeResultDescribe)
		updates["fail_reason"] = strings.TrimSpace(failReason)
		updates["terminal_at"] = common.GetTimestamp()
		addPiggyApprovedSubmissionMarkers(order, updates)
		if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(updates).Error; err != nil {
			return err
		}
		return createWalletFlowTx(tx, &model.WalletFlow{
			UserId:                order.UserId,
			BizNo:                 order.WithdrawNo,
			IdempotencyKey:        walletIdempotencyKey("wallet:piggy-withdraw-failed:" + order.WithdrawNo),
			FlowType:              model.WalletFlowTypeWithdrawReject,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionIn,
			Amount:                amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                failReason,
		})
	})
}

func failPiggySubmittedOrderAndRelease(orderId int, reason string, reqDigest string, respDigest string) (bool, error) {
	released := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderTx(tx, orderId)
		if err != nil {
			return err
		}
		if order.Provider != model.WithdrawProviderPiggyLaborV3 {
			return ErrWithdrawStatusInvalid
		}
		if order.Status != model.WithdrawStatusApproved {
			return nil
		}
		account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
		if err != nil {
			return err
		}
		amount := order.Amount
		if amount <= 0 {
			amount = centsToFloat(order.FrozenAmountCents)
		}
		account.FrozenCommissionAmount = maxFloat(0, account.FrozenCommissionAmount-amount)
		account.CommissionAmount += amount
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
			"status":                  model.WithdrawStatusFailed,
			"piggy_status":            model.WithdrawStatusFailed,
			"fail_reason":             strings.TrimSpace(reason),
			"terminal_at":             now,
			"request_payload_digest":  reqDigest,
			"response_payload_digest": respDigest,
			"compensation_status":     "",
		}).Error; err != nil {
			return err
		}
		if err := createWalletFlowTx(tx, &model.WalletFlow{
			UserId:                order.UserId,
			BizNo:                 order.WithdrawNo,
			IdempotencyKey:        walletIdempotencyKey("wallet:piggy-withdraw-failed:" + order.WithdrawNo),
			FlowType:              model.WalletFlowTypeWithdrawReject,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionIn,
			Amount:                amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                strings.TrimSpace(reason),
		}); err != nil {
			return err
		}
		released = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return released, nil
}

func cancelPiggyOrderAndRelease(withdrawNo string, adminId int, reason string, reqDigest string, respDigest string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderByNoTx(tx, withdrawNo)
		if err != nil {
			return err
		}
		if isPiggyTerminalStatus(order.Status) {
			return nil
		}
		if order.Status != model.WithdrawStatusCancelling {
			return ErrWithdrawStatusInvalid
		}
		account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
		if err != nil {
			return err
		}
		amount := order.Amount
		if amount <= 0 {
			amount = centsToFloat(order.FrozenAmountCents)
		}
		account.FrozenCommissionAmount = maxFloat(0, account.FrozenCommissionAmount-amount)
		account.CommissionAmount += amount
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
			"status":                  model.WithdrawStatusCancelled,
			"piggy_status":            model.WithdrawStatusCancelled,
			"reviewer_id":             adminId,
			"reviewed_at":             now,
			"terminal_at":             now,
			"fail_reason":             reason,
			"request_payload_digest":  reqDigest,
			"response_payload_digest": respDigest,
		}).Error; err != nil {
			return err
		}
		return createWalletFlowTx(tx, &model.WalletFlow{
			UserId:                order.UserId,
			BizNo:                 order.WithdrawNo,
			IdempotencyKey:        walletIdempotencyKey("wallet:piggy-withdraw-cancel:" + order.WithdrawNo),
			FlowType:              model.WalletFlowTypeWithdrawReject,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionIn,
			Amount:                amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                reason,
		})
	})
}

func queryPiggyOrderStatus(ctx context.Context, withdrawNo string) error {
	_, _, _, err := queryPiggyOrderStatusWithEvidence(ctx, withdrawNo)
	return err
}

func queryPiggyOrderStatusWithResult(ctx context.Context, withdrawNo string) (bool, error) {
	statusProved, _, _, err := queryPiggyOrderStatusWithEvidence(ctx, withdrawNo)
	return statusProved, err
}

func piggyPreferQueryEvidence(reqDigest string, respDigest string, queryReqDigest string, queryRespDigest string, queryErr error) (string, string) {
	if strings.TrimSpace(queryReqDigest) == "" {
		return reqDigest, respDigest
	}
	reqDigest = queryReqDigest
	return reqDigest, queryRespDigest
}

func piggyApplyDigestUpdates(updates map[string]interface{}, reqDigest string, respDigest string) {
	if updates == nil {
		return
	}
	reqDigest = strings.TrimSpace(reqDigest)
	respDigest = strings.TrimSpace(respDigest)
	if reqDigest != "" {
		updates["request_payload_digest"] = reqDigest
		updates["response_payload_digest"] = respDigest
		return
	}
	if respDigest != "" {
		updates["response_payload_digest"] = respDigest
	}
}

func queryPiggyOrderStatusWithEvidence(ctx context.Context, withdrawNo string) (bool, string, string, error) {
	client, err := newConfiguredPiggyClient(operation_setting.GetPiggyWithdrawSetting())
	if err != nil {
		return false, "", "", err
	}
	resp, reqDigest, err := client.SingleOrderQuery(ctx, withdrawNo)
	respDigest := ""
	if resp != nil {
		respDigest = digestPayload([]byte(resp.RawBody))
	}
	updates := map[string]interface{}{
		"request_payload_digest":  reqDigest,
		"response_payload_digest": respDigest,
	}
	if err != nil {
		if markErr := markPiggyOrderManualReviewByNoIfActive(withdrawNo, "小猪订单查询失败: "+err.Error(), reqDigest, respDigest); markErr != nil {
			return false, reqDigest, respDigest, markErr
		}
		current, currentErr := getPiggyOrderByNo(withdrawNo)
		if currentErr == nil && isPiggyTerminalStatus(current.Status) {
			return false, reqDigest, respDigest, nil
		}
		return false, reqDigest, respDigest, err
	}
	if err := model.DB.Model(&model.WithdrawOrder{}).Where("withdraw_no = ? AND provider = ?", withdrawNo, model.WithdrawProviderPiggyLaborV3).UpdateColumns(updates).Error; err != nil {
		return false, reqDigest, respDigest, err
	}
	statusProved, err := applyPiggyQueryStatus(ctx, withdrawNo, resp)
	return statusProved, reqDigest, respDigest, err
}

func lockWithdrawOrderByNoTx(tx *gorm.DB, withdrawNo string) (*model.WithdrawOrder, error) {
	var order model.WithdrawOrder
	if err := walletLockQuery(tx).Where("withdraw_no = ?", strings.TrimSpace(withdrawNo)).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWithdrawOrderNotFound
		}
		return nil, err
	}
	if order.Provider != model.WithdrawProviderPiggyLaborV3 {
		return nil, ErrWithdrawStatusInvalid
	}
	return &order, nil
}

func getPiggyOrderByNo(withdrawNo string) (*model.WithdrawOrder, error) {
	var order model.WithdrawOrder
	if err := model.DB.Where("withdraw_no = ? AND provider = ?", strings.TrimSpace(withdrawNo), model.WithdrawProviderPiggyLaborV3).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWithdrawOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

func getPiggyOrderById(orderId int) (*model.WithdrawOrder, error) {
	var order model.WithdrawOrder
	if err := model.DB.Where("id = ? AND provider = ?", orderId, model.WithdrawProviderPiggyLaborV3).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWithdrawOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

func getRawWithdrawalProfile(userId int) (*model.WithdrawalProfile, error) {
	var profile model.WithdrawalProfile
	if err := model.DB.Where("user_id = ?", userId).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &profile, nil
}

func maskWithdrawalProfile(profile *model.WithdrawalProfile) *model.WithdrawalProfile {
	if profile == nil {
		return nil
	}
	copyValue := *profile
	copyValue.MaskedIdCardNo = maskChineseIDCard(profile.IdCardNo)
	copyValue.MaskedMobile = maskMobile(profile.Mobile)
	copyValue.MaskedBankCardNo = maskBankCard(profile.BankCardNo)
	return &copyValue
}

func isWithdrawalProfileComplete(profile *model.WithdrawalProfile) bool {
	return profile != nil &&
		profile.UserId > 0 &&
		profile.AccountType == model.WithdrawAccountTypeBankcard &&
		strings.TrimSpace(profile.RealName) != "" &&
		strings.TrimSpace(profile.IdCardNo) != "" &&
		strings.TrimSpace(profile.Mobile) != "" &&
		strings.TrimSpace(profile.BankCardNo) != "" &&
		strings.TrimSpace(profile.BankName) != ""
}

func isPiggyOrderPayoutSnapshotComplete(order *model.WithdrawOrder) bool {
	return order != nil &&
		strings.TrimSpace(order.AccountName) != "" &&
		strings.TrimSpace(order.PayoutMobile) != "" &&
		strings.TrimSpace(order.PayoutIdCardNo) != "" &&
		strings.TrimSpace(order.PayoutBankCardNo) != "" &&
		strings.TrimSpace(order.BankName) != ""
}

func isPiggyContractSignedForCurrentScope(profile *model.WithdrawalProfile, setting *operation_setting.PiggyWithdrawSetting) bool {
	if profile == nil || profile.PiggySignStatus != model.PiggySignStatusSigned {
		return false
	}
	if setting == nil {
		setting = operation_setting.GetPiggyWithdrawSetting()
	}
	hasScopeSnapshot := strings.TrimSpace(profile.PiggyContractPosition) != "" ||
		strings.TrimSpace(profile.PiggyContractPositionName) != "" ||
		strings.TrimSpace(profile.PiggyContractTaxFundID) != ""
	if !hasScopeSnapshot {
		return true
	}
	return piggyContractPositionMatches(strings.TrimSpace(profile.PiggyContractPosition), setting) &&
		strings.TrimSpace(profile.PiggyContractPositionName) == strings.TrimSpace(setting.PositionName) &&
		strings.TrimSpace(profile.PiggyContractTaxFundID) == strings.TrimSpace(setting.TaxFundId)
}

func findMatchingPiggySignedContractResult(response *PiggyAPIResponse, profile *model.WithdrawalProfile, setting *operation_setting.PiggyWithdrawSetting) (*piggySignedContractResult, error) {
	if response == nil || len(response.RawData) == 0 {
		return nil, errors.New("未查询到小猪已签合同")
	}
	if common.GetJsonType(response.RawData) != "array" {
		return nil, errors.New("小猪签署结果格式异常")
	}
	var results []piggySignedContractResult
	if err := common.Unmarshal(response.RawData, &results); err != nil {
		return nil, err
	}
	for i := range results {
		if matchesPiggySignedContractResult(results[i], profile, setting) {
			return &results[i], nil
		}
	}
	return nil, errors.New("未查询到小猪已签合同")
}

func matchesPiggySignedContractResult(result piggySignedContractResult, profile *model.WithdrawalProfile, setting *operation_setting.PiggyWithdrawSetting) bool {
	if profile == nil {
		return false
	}
	resultName := firstNonEmpty(result.Name, result.UserName)
	if resultName != strings.TrimSpace(profile.RealName) ||
		strings.TrimSpace(result.IdCardNo) != strings.TrimSpace(profile.IdCardNo) {
		return false
	}
	if strings.TrimSpace(result.Mobile) != "" &&
		normalizePiggyContractCallbackMobile(result.Mobile) != strings.TrimSpace(profile.Mobile) {
		return false
	}
	position := strings.TrimSpace(result.Position)
	if position == "" || setting == nil {
		return true
	}
	// 小猪签署结果的 position 是服务类型；历史数据可能保存过旧配置 position，刷新时兼容旧值。
	return piggyContractPositionMatches(position, setting)
}

func piggyContractServiceType(setting *operation_setting.PiggyWithdrawSetting) string {
	if setting == nil {
		setting = operation_setting.GetPiggyWithdrawSetting()
	}
	// 小猪电子合同 position 参数要求传“服务类型”名称；本系统配置中 position_name 与提现上报的服务类型一致。
	return strings.TrimSpace(firstNonEmpty(setting.PositionName, setting.Position))
}

func piggyContractPositionMatches(position string, setting *operation_setting.PiggyWithdrawSetting) bool {
	position = strings.TrimSpace(position)
	if position == "" || setting == nil {
		return position == ""
	}
	return position == piggyContractServiceType(setting) ||
		position == strings.TrimSpace(setting.Position)
}

func parsePiggyContractSignTime(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed.Unix()
		}
	}
	return 0
}

func normalizeWithdrawalMobile(mobile string) (string, error) {
	normalized, err := NormalizePhoneNumber(mobile)
	if err != nil {
		return "", ErrWithdrawalPhoneInvalid
	}
	localMobile := strings.TrimPrefix(normalized, "+86")
	if !strings.HasPrefix(normalized, "+86") ||
		len(localMobile) != 11 ||
		!strings.HasPrefix(localMobile, "1") ||
		!onlyDigits(localMobile) {
		return "", ErrWithdrawalPhoneInvalid
	}
	return localMobile, nil
}

func getPiggyWithdrawCooldownRemaining(userId int, cooldownMinutes int) (int64, error) {
	if cooldownMinutes <= 0 {
		return 0, nil
	}
	threshold := common.GetTimestamp() - int64(cooldownMinutes*60)
	var order model.WithdrawOrder
	err := model.DB.Where("user_id = ? AND provider = ? AND created_at > ?", userId, model.WithdrawProviderPiggyLaborV3, threshold).
		Order("created_at desc").
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	remaining := int64(cooldownMinutes*60) - (common.GetTimestamp() - order.CreatedAt)
	if remaining < 0 {
		return 0, nil
	}
	return remaining, nil
}

func isForbiddenWithdrawTime(rule string, now time.Time) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return false
	}
	currentMinutes := now.Hour()*60 + now.Minute()
	for _, part := range strings.Split(rule, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			start, ok1 := parseForbiddenTimePoint(rangeParts[0])
			end, ok2 := parseForbiddenTimePoint(rangeParts[1])
			if !ok1 || !ok2 {
				continue
			}
			if start <= end && currentMinutes >= start && currentMinutes <= end {
				return true
			}
			if start > end && (currentMinutes >= start || currentMinutes <= end) {
				return true
			}
			continue
		}
		hour, err := strconv.Atoi(part)
		if err == nil && now.Hour() == hour {
			return true
		}
	}
	return false
}

func parseForbiddenTimePoint(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if !strings.Contains(value, ":") {
		hour, err := strconv.Atoi(value)
		if err != nil || hour < 0 || hour > 23 {
			return 0, false
		}
		return hour * 60, true
	}
	parts := strings.SplitN(value, ":", 2)
	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

func validatePiggyCallbackSecret(appSecret string) error {
	secretLen := len([]byte(strings.TrimSpace(appSecret)))
	if secretLen != 16 && secretLen != 24 && secretLen != 32 {
		return errors.New("小猪回调密钥未配置或长度无效")
	}
	return nil
}

func validatePiggyPaymentCallbackCrypto(setting *operation_setting.PiggyWithdrawSetting) error {
	if setting == nil {
		return errors.New("小猪回调配置不能为空")
	}
	if err := validatePiggyCallbackSecret(setting.AppSecret); err != nil {
		return err
	}
	if len([]byte(strings.TrimSpace(setting.AESIV))) != 16 {
		return errors.New("小猪回调 AES IV 未配置或长度无效")
	}
	return nil
}

func validateUnsignedPiggyPaymentCallbackContent(content PiggyPaymentCallbackContent) error {
	withdrawNo := strings.TrimSpace(content.OuterTradeNo)
	if withdrawNo == "" {
		return errors.New("小猪无签名支付回调缺少 outerTradeNo")
	}
	order, err := getPiggyOrderByNo(withdrawNo)
	if err != nil {
		return err
	}
	if err := validatePiggyCallbackPretaxAmount(order, content); err != nil {
		return err
	}
	if err := requirePiggyUnsignedCallbackField("empName", content.EmpName, order.AccountName); err != nil {
		return err
	}
	if err := requirePiggyUnsignedCallbackField("empPhone", normalizePiggyContractCallbackMobile(content.EmpPhone), normalizePiggyContractCallbackMobile(order.PayoutMobile)); err != nil {
		return err
	}
	if err := requirePiggyUnsignedCallbackField("licenseType", content.LicenseType, "ID_CARD"); err != nil {
		return err
	}
	if err := requirePiggyUnsignedCallbackField("licenseId", content.LicenseId, order.PayoutIdCardNo); err != nil {
		return err
	}
	if err := requirePiggyUnsignedCallbackField("settleType", content.SettleType, model.WithdrawAccountTypeBankcard); err != nil {
		return err
	}
	if err := requirePiggyUnsignedCallbackField("payAccount", content.PayAccount, order.PayoutBankCardNo); err != nil {
		return err
	}
	if err := requirePiggyUnsignedCallbackField("positionName", content.PositionName, firstNonEmpty(order.PositionName, operation_setting.GetPiggyWithdrawSetting().PositionName)); err != nil {
		return err
	}
	if strings.TrimSpace(content.BankName) != "" && strings.TrimSpace(order.BankName) != "" &&
		strings.TrimSpace(content.BankName) != strings.TrimSpace(order.BankName) {
		return errors.New("小猪无签名支付回调 bankName 与订单快照不匹配")
	}
	return nil
}

func validatePiggyQueryCallbackContent(expectedWithdrawNo string, content PiggyPaymentCallbackContent) error {
	withdrawNo := strings.TrimSpace(content.OuterTradeNo)
	if withdrawNo == "" {
		return errors.New("小猪订单查询结果缺少 outerTradeNo")
	}
	if strings.TrimSpace(expectedWithdrawNo) != "" && withdrawNo != strings.TrimSpace(expectedWithdrawNo) {
		return fmt.Errorf("小猪订单查询结果 outerTradeNo 与请求单号不匹配: actual=%s expected=%s", withdrawNo, strings.TrimSpace(expectedWithdrawNo))
	}
	if strings.TrimSpace(content.PretaxAmount) == "" {
		return errors.New("小猪订单查询结果缺少 pretaxAmount")
	}
	order, err := getPiggyOrderByNo(withdrawNo)
	if err != nil {
		return err
	}
	if err := validatePiggyCallbackPretaxAmount(order, content); err != nil {
		return err
	}
	return nil
}

func requirePiggyUnsignedCallbackField(name string, actual string, expected string) error {
	actual = strings.TrimSpace(actual)
	expected = strings.TrimSpace(expected)
	if actual == "" {
		return fmt.Errorf("小猪无签名支付回调缺少 %s", name)
	}
	if expected == "" || actual != expected {
		return fmt.Errorf("小猪无签名支付回调 %s 与订单快照不匹配", name)
	}
	return nil
}

func verifyPiggyCallbackSignature(body []byte, headerSignature string, payloadSignature string, appSecret string) error {
	if err := validatePiggyCallbackSecret(appSecret); err != nil {
		return err
	}
	signature := strings.TrimSpace(payloadSignature)
	if signature == "" {
		signature = strings.TrimSpace(headerSignature)
	}
	if signature == "" {
		return errors.New("小猪回调缺少签名")
	}
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return err
	}
	if _, ok := payload["sign"]; ok {
		expected, err := piggySignJSON(appSecret, payload)
		if err != nil {
			return err
		}
		if !strings.EqualFold(expected, signature) {
			return errors.New("小猪回调验签失败")
		}
		return nil
	}
	form := make(map[string]string)
	for key, value := range payload {
		form[key] = fmt.Sprintf("%v", value)
	}
	if !strings.EqualFold(piggySignForm(appSecret, form), signature) {
		return errors.New("小猪回调验签失败")
	}
	return nil
}

func piggyCallbackUserId(customParams any) int {
	if customParams == nil {
		return 0
	}
	switch typed := customParams.(type) {
	case map[string]any:
		return piggyCallbackUserIdFromMap(typed)
	case string:
		var parsed map[string]any
		if err := common.UnmarshalJsonStr(strings.TrimSpace(typed), &parsed); err != nil {
			return 0
		}
		return piggyCallbackUserIdFromMap(parsed)
	default:
		return 0
	}
}

func piggyCallbackUserIdFromMap(customParams map[string]any) int {
	value, ok := customParams["userId"]
	if !ok {
		value = customParams["user_id"]
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		id, _ := strconv.Atoi(strings.TrimSpace(typed))
		return id
	default:
		return 0
	}
}

func normalizePiggyContractCallbackPayload(payload PiggyContractCallbackPayload) normalizedPiggyContractCallback {
	return normalizedPiggyContractCallback{
		Code:           strings.TrimSpace(payload.Code),
		Msg:            strings.TrimSpace(payload.Msg),
		UserName:       firstNonEmpty(payload.Data.Name, payload.Data.UserName, payload.Name, payload.UserName),
		IdCardNo:       firstNonEmpty(payload.Data.IdCardNo, payload.IdCardNo),
		Mobile:         firstNonEmpty(payload.Data.Mobile, payload.Mobile),
		BankAccount:    firstNonEmpty(payload.Data.BankAccount, payload.BankAccount),
		ContractURL:    firstNonEmpty(payload.Data.ContractURL, payload.Data.ContractUrl, payload.ContractURL, payload.ContractUrl),
		DocumentID:     firstNonEmpty(payload.Data.DocumentID, payload.Data.DocumentId, payload.DocumentID, payload.DocumentId),
		SubsidiaryName: firstNonEmpty(payload.Data.SubsidiaryName, payload.Data.SubsidiaryNameCamel, payload.SubsidiaryName),
		SignStatus:     firstNonEmpty(payload.Data.SignStatus, payload.SignStatus),
		Status:         firstNonEmpty(payload.Data.Status, payload.Status),
		CustomParams:   firstNonNil(payload.Data.CustomParams, payload.CustomParams),
		Sign:           strings.TrimSpace(payload.Sign),
	}
}

func normalizePiggyContractSignStatus(status string, hasSignature bool) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		if hasSignature {
			return model.PiggySignStatusSigned, nil
		}
		// 无签名回调没有验签保护，必须显式携带成功状态，避免仅靠身份匹配就被伪造为签约成功。
		return "", errors.New("小猪无签名签约回调缺少签约状态")
	}
	switch normalized {
	case "0", "success", "signed", "finish", "completed":
		return model.PiggySignStatusSigned, nil
	default:
		if !hasSignature {
			// 官方无签名回调只应通知签署成功；非成功状态不能写入用户签约资料，避免公开回调被用于降级已签合同。
			return "", errors.New("小猪无签名签约回调签约状态不是成功")
		}
		return model.PiggySignStatusFailed, nil
	}
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func findPiggyContractCallbackProfile(callback normalizedPiggyContractCallback, hasSignature bool) (*model.WithdrawalProfile, error) {
	if !hasSignature {
		return findUnsignedPiggyContractCallbackProfile(callback)
	}
	userId := piggyCallbackUserId(callback.CustomParams)
	if userId > 0 {
		var profile model.WithdrawalProfile
		if err := model.DB.Where("user_id = ?", userId).First(&profile).Error; err != nil {
			return nil, err
		}
		return &profile, nil
	}
	idCardNo := strings.TrimSpace(callback.IdCardNo)
	realName := strings.TrimSpace(callback.UserName)
	mobile := normalizePiggyContractCallbackMobile(callback.Mobile)
	if idCardNo == "" || realName == "" || mobile == "" {
		return nil, errors.New("小猪签约回调身份信息不完整")
	}
	var profiles []model.WithdrawalProfile
	if err := model.DB.
		Where("id_card_no = ? AND real_name = ? AND mobile = ?", idCardNo, realName, mobile).
		Limit(2).
		Find(&profiles).Error; err != nil {
		return nil, err
	}
	if len(profiles) != 1 {
		return nil, errors.New("小猪签约回调身份信息未匹配到唯一提现资料")
	}
	return &profiles[0], nil
}

func findUnsignedPiggyContractCallbackProfile(callback normalizedPiggyContractCallback) (*model.WithdrawalProfile, error) {
	userId := piggyCallbackUserId(callback.CustomParams)
	if userId <= 0 {
		return nil, errors.New("小猪无签名签约回调缺少 customParams.userId")
	}
	var profile model.WithdrawalProfile
	if err := model.DB.Where("user_id = ?", userId).First(&profile).Error; err != nil {
		return nil, err
	}
	if err := validateUnsignedPiggyContractCallbackProfile(&profile, callback); err != nil {
		return nil, err
	}
	return &profile, nil
}

func validateUnsignedPiggyContractCallbackProfile(profile *model.WithdrawalProfile, callback normalizedPiggyContractCallback) error {
	if profile == nil {
		return ErrWithdrawalProfileIncomplete
	}
	idCardNo := strings.TrimSpace(callback.IdCardNo)
	realName := strings.TrimSpace(callback.UserName)
	mobile := normalizePiggyContractCallbackMobile(callback.Mobile)
	if idCardNo == "" || realName == "" || mobile == "" {
		return errors.New("小猪无签名签约回调身份信息不完整")
	}
	// 无签名回调缺少密码学来源证明，必须同时绑定 customParams.userId 和三项身份字段；
	// 不能使用公共身份唯一匹配兜底，否则公开回调地址可能被伪造签约成功。
	if strings.TrimSpace(profile.RealName) != realName ||
		strings.TrimSpace(profile.IdCardNo) != idCardNo ||
		normalizePiggyContractCallbackMobile(profile.Mobile) != mobile {
		return errors.New("小猪无签名签约回调身份信息与提现资料不匹配")
	}
	return nil
}

func normalizePiggyContractCallbackMobile(mobile string) string {
	mobile = strings.TrimSpace(mobile)
	if mobile == "" {
		return ""
	}
	normalized, err := NormalizePhoneNumber(mobile)
	if err != nil {
		return mobile
	}
	return strings.TrimPrefix(normalized, "+86")
}

func buildPiggyPaymentCallbackIdempotencyKey(content PiggyPaymentCallbackContent) string {
	flowNo := firstNonEmpty(content.FrontLogNo, content.LaborOrderNo)
	return strings.Join([]string{
		strings.TrimSpace(content.OuterTradeNo),
		strings.TrimSpace(content.NotifyType),
		strings.TrimSpace(content.TradeStatus),
		strings.TrimSpace(flowNo),
	}, "|")
}

func applyPiggyQueryStatus(ctx context.Context, withdrawNo string, resp *PiggyAPIResponse) (bool, error) {
	content, ok, requireSnapshotValidation, err := piggyPaymentContentFromQueryResponse(withdrawNo, resp)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if requireSnapshotValidation {
		if err := validatePiggyQueryCallbackContent(withdrawNo, content); err != nil {
			return false, err
		}
	}
	content = inferPiggyQueryNotifyType(content)
	if err := processPiggyPaymentCallbackContent(ctx, content); err != nil {
		return false, err
	}
	return true, nil
}

func inferPiggyQueryNotifyType(content PiggyPaymentCallbackContent) PiggyPaymentCallbackContent {
	if strings.TrimSpace(content.NotifyType) != "" {
		return content
	}
	switch strings.TrimSpace(content.TradeStatus) {
	case "await":
		content.NotifyType = "submitResult"
	case "success", "failure":
		content.NotifyType = "tradeResult"
	}
	return content
}

func piggyPaymentContentFromQueryResponse(withdrawNo string, resp *PiggyAPIResponse) (PiggyPaymentCallbackContent, bool, bool, error) {
	if resp == nil || len(resp.Data) == 0 {
		return PiggyPaymentCallbackContent{}, false, false, nil
	}
	if encrypted := pickPiggyString(resp.Data, "bizAESContent", "biz_aes_content"); encrypted != "" {
		setting := operation_setting.GetPiggyWithdrawSetting()
		plain, err := piggyDecryptAES(encrypted, setting.AppSecret, setting.AESIV)
		if err != nil {
			return PiggyPaymentCallbackContent{}, false, false, err
		}
		var content PiggyPaymentCallbackContent
		if err := common.Unmarshal(plain, &content); err != nil {
			return PiggyPaymentCallbackContent{}, false, false, err
		}
		if strings.TrimSpace(content.NotifyType) == "" && strings.TrimSpace(content.TradeStatus) == "" {
			return PiggyPaymentCallbackContent{}, false, false, nil
		}
		return content, true, true, nil
	}
	content := PiggyPaymentCallbackContent{
		OuterTradeNo:        firstNonEmpty(pickPiggyString(resp.Data, "outerTradeNo", "outer_trade_no"), withdrawNo),
		NotifyType:          pickPiggyString(resp.Data, "notifyType", "notify_type"),
		TradeStatus:         pickPiggyString(resp.Data, "tradeStatus", "trade_status"),
		TradeTime:           pickPiggyString(resp.Data, "tradeTime", "trade_time"),
		FrontLogNo:          pickPiggyString(resp.Data, "frontLogNo", "front_log_no"),
		LaborOrderNo:        pickPiggyString(resp.Data, "laborOrderNo", "labor_order_no"),
		EmpName:             pickPiggyString(resp.Data, "empName", "emp_name"),
		EmpPhone:            pickPiggyString(resp.Data, "empPhone", "emp_phone"),
		LicenseType:         pickPiggyString(resp.Data, "licenseType", "license_type"),
		LicenseId:           pickPiggyString(resp.Data, "licenseId", "license_id"),
		SettleType:          pickPiggyString(resp.Data, "settleType", "settle_type"),
		PayAccount:          pickPiggyString(resp.Data, "payAccount", "pay_account"),
		BankName:            pickPiggyString(resp.Data, "bankName", "bank_name"),
		PositionName:        pickPiggyString(resp.Data, "positionName", "position_name"),
		TradeFailCode:       pickPiggyString(resp.Data, "tradeFailCode", "trade_fail_code"),
		TradeResult:         pickPiggyString(resp.Data, "tradeResult", "trade_result"),
		TradeResultDescribe: pickPiggyString(resp.Data, "tradeResultDescribe", "trade_result_describe"),
		PretaxAmount:        pickPiggyString(resp.Data, "pretaxAmount", "pretax_amount"),
		IndividualTaxAmount: pickPiggyString(resp.Data, "individualTaxAmount", "individual_tax_amount"),
		AddedTaxAmount:      pickPiggyString(resp.Data, "addedTaxAmount", "added_tax_amount"),
		AfterTaxAmount:      pickPiggyString(resp.Data, "afterTaxAmount", "after_tax_amount"),
		FeeAmount:           pickPiggyString(resp.Data, "feeAmount", "fee_amount"),
		CalcType:            pickPiggyString(resp.Data, "calcType", "calc_type"),
	}
	if strings.TrimSpace(content.NotifyType) == "" && strings.TrimSpace(content.TradeStatus) == "" {
		return PiggyPaymentCallbackContent{}, false, false, nil
	}
	return content, true, false, nil
}

func hasProcessedPiggyCallback(idempotencyKey string) (bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return false, nil
	}
	var count int64
	err := model.DB.Model(&model.PiggyWithdrawCallbackLog{}).
		Where("idempotency_key = ? AND process_status = ?", idempotencyKey, model.PaymentProcessStatusSuccess).
		Count(&count).Error
	return count > 0, err
}

func finishPiggyCallbackLog(id int, status string, errorMessage string, extra map[string]interface{}) {
	updates := map[string]interface{}{
		"process_status": status,
		"error_message":  strings.TrimSpace(errorMessage),
	}
	for key, value := range extra {
		updates[key] = value
	}
	_ = model.DB.Model(&model.PiggyWithdrawCallbackLog{}).Where("id = ?", id).Updates(updates).Error
}

func piggyOrderCallbackAmountUpdates(content PiggyPaymentCallbackContent) map[string]interface{} {
	return map[string]interface{}{
		"piggy_pretax_amount_cents":    toCentsIgnoreError(content.PretaxAmount),
		"piggy_individual_tax_cents":   toCentsIgnoreError(content.IndividualTaxAmount),
		"piggy_added_tax_cents":        toCentsIgnoreError(content.AddedTaxAmount),
		"piggy_after_tax_amount_cents": toCentsIgnoreError(content.AfterTaxAmount),
		"piggy_fee_amount_cents":       toCentsIgnoreError(content.FeeAmount),
		"calc_type":                    firstNonEmpty(content.CalcType, operation_setting.GetPiggyWithdrawSetting().CalcType),
	}
}

func validatePiggyCallbackPretaxAmount(order *model.WithdrawOrder, content PiggyPaymentCallbackContent) error {
	if order == nil {
		return ErrWithdrawOrderNotFound
	}
	pretaxAmount := strings.TrimSpace(content.PretaxAmount)
	if pretaxAmount == "" {
		return nil
	}
	callbackCents, err := yuanToCents(pretaxAmount)
	if err != nil {
		return fmt.Errorf("小猪回调税前金额格式错误: %w", err)
	}
	expectedCents := expectedPiggyPretaxAmountCents(order)
	if expectedCents <= 0 {
		return errors.New("小猪回调税前金额无法与本地订单金额核对")
	}
	if callbackCents != expectedCents {
		return fmt.Errorf("小猪回调税前金额与小猪打款金额不一致: callback=%s expected=%s", centsToYuanString(callbackCents), centsToYuanString(expectedCents))
	}
	return nil
}

func expectedPiggyPretaxAmountCents(order *model.WithdrawOrder) int64 {
	if order == nil {
		return 0
	}
	// 新平台费订单优先按提交给小猪的 post-fee 金额核对；旧订单没有平台费快照时继续按历史全额核对。
	if order.PlatformFeeRate > 0 || order.PlatformFeeAmountCents > 0 {
		if order.PiggyPayAmountCents > 0 {
			return order.PiggyPayAmountCents
		}
		if order.TaxBeforeAmountCents > 0 {
			return order.TaxBeforeAmountCents
		}
	}
	if order.FrozenAmountCents > 0 {
		return order.FrozenAmountCents
	}
	if order.PiggyPayAmountCents > 0 {
		return order.PiggyPayAmountCents
	}
	return floatYuanToCents(order.Amount)
}

func markPiggyOrderManualReviewFromCallbackTx(tx *gorm.DB, order *model.WithdrawOrder, reason string, content PiggyPaymentCallbackContent) error {
	updates := piggyOrderCallbackAmountUpdates(content)
	updates["status"] = model.WithdrawStatusManualReview
	updates["piggy_status"] = model.WithdrawStatusManualReview
	updates["manual_review_reason"] = strings.TrimSpace(reason)
	updates["notify_type"] = strings.TrimSpace(content.NotifyType)
	updates["trade_status"] = strings.TrimSpace(content.TradeStatus)
	updates["front_log_no"] = strings.TrimSpace(content.FrontLogNo)
	updates["labor_order_no"] = strings.TrimSpace(content.LaborOrderNo)
	updates["trade_fail_code"] = strings.TrimSpace(content.TradeFailCode)
	updates["trade_result"] = strings.TrimSpace(content.TradeResult)
	updates["trade_result_describe"] = strings.TrimSpace(content.TradeResultDescribe)
	addPiggyApprovedSubmissionMarkers(order, updates)
	return tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(updates).Error
}

func addPiggyApprovedSubmissionMarkers(order *model.WithdrawOrder, updates map[string]interface{}) {
	if order == nil || updates == nil || order.Status != model.WithdrawStatusApproved {
		return
	}
	if strings.TrimSpace(order.ExternalTradeNo) == "" {
		updates["external_trade_no"] = order.WithdrawNo
	}
	if order.SubmittedAt <= 0 {
		updates["submitted_at"] = common.GetTimestamp()
	}
	if strings.TrimSpace(order.CompensationStatus) == piggyCompensationStatusSubmitRecovering {
		updates["compensation_status"] = ""
	}
}

func piggySubmitFailureReason(resp *PiggyAPIResponse, err error) string {
	if resp == nil {
		if err != nil {
			return err.Error()
		}
		return "小猪订单提交失败"
	}
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	return firstNonEmpty(resp.ErrorMessage, resp.Msg, resp.ErrorCode, errText, "小猪订单提交失败")
}

func piggySubmitUnknownRecoverableReason(err error) string {
	if err == nil {
		return "小猪订单提交结果未知，可使用原流水号恢复提交"
	}
	return "小猪订单提交结果未知，可使用原流水号恢复提交: " + err.Error()
}

func piggyWithdrawApprovalResultFromCurrentOrder(orderId int, fallback *WithdrawApprovalResult) (*WithdrawApprovalResult, error) {
	var order model.WithdrawOrder
	if err := model.DB.Select("id", "status", "external_trade_no", "manual_review_reason", "fail_reason").
		Where("id = ? AND provider = ?", orderId, model.WithdrawProviderPiggyLaborV3).
		First(&order).Error; err != nil {
		return nil, err
	}
	result := piggyWithdrawApprovalResultFromOrder(&order)
	if result != nil {
		return result, nil
	}
	return fallback, nil
}

func piggyWithdrawApprovalResultFromOrder(order *model.WithdrawOrder) *WithdrawApprovalResult {
	if order == nil {
		return nil
	}
	status := strings.TrimSpace(order.Status)
	switch status {
	case model.WithdrawStatusApproved:
		return &WithdrawApprovalResult{
			Submitted:   false,
			Recoverable: strings.TrimSpace(order.ExternalTradeNo) == "",
			Status:      status,
			Message:     firstNonEmpty(order.ManualReviewReason, "小猪提交结果未知，请稍后恢复提交或检查网络"),
		}
	case model.WithdrawStatusSubmitted,
		model.WithdrawStatusAwaitConfirm,
		model.WithdrawStatusConfirming,
		model.WithdrawStatusConfirmed:
		return &WithdrawApprovalResult{
			Submitted:   true,
			Recoverable: false,
			Status:      status,
			Message:     "小猪提现已提交",
		}
	case model.WithdrawStatusPaid:
		return &WithdrawApprovalResult{
			Submitted:   true,
			Recoverable: false,
			Status:      status,
			Message:     "小猪提现已支付",
		}
	case model.WithdrawStatusFailed,
		model.WithdrawStatusCancelled,
		model.WithdrawStatusRejected:
		return &WithdrawApprovalResult{
			Submitted:   false,
			Recoverable: false,
			Status:      status,
			Message:     firstNonEmpty(order.FailReason, order.ManualReviewReason, "小猪提现已进入终态"),
		}
	case model.WithdrawStatusManualReview:
		return &WithdrawApprovalResult{
			Submitted:   false,
			Recoverable: false,
			Status:      status,
			Message:     firstNonEmpty(order.ManualReviewReason, "小猪提现需要人工核对"),
		}
	default:
		return nil
	}
}

func isPiggyDuplicateOuterTradeNoFailure(resp *PiggyAPIResponse, err error) bool {
	parts := make([]string, 0, 4)
	if resp != nil {
		parts = append(parts, resp.ErrorMessage, resp.Msg, resp.ErrorCode)
	}
	if err != nil {
		parts = append(parts, err.Error())
	}
	text := strings.ToLower(strings.Join(parts, " "))

	return containsAnyPiggyFailureMarker(text, piggyDuplicateOuterTradeNoMarkers) &&
		containsAnyPiggyFailureMarker(text, piggyDuplicateFailureMarkers)
}

func containsAnyPiggyFailureMarker(text string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func piggyIsOrderNotFoundQueryError(err error) bool {
	if err == nil {
		return false
	}
	return containsAnyPiggyFailureMarker(strings.ToLower(err.Error()), piggyOrderNotFoundMarkers)
}

func toCentsIgnoreError(value string) int64 {
	cents, _ := yuanToCents(value)
	return cents
}

func piggyWithdrawOrderAmount(order *model.WithdrawOrder) float64 {
	if order == nil {
		return 0
	}
	if order.Amount > 0 {
		return order.Amount
	}
	return centsToFloat(order.FrozenAmountCents)
}

func isPiggyTerminalStatus(status string) bool {
	switch status {
	case model.WithdrawStatusPaid, model.WithdrawStatusFailed, model.WithdrawStatusCancelled, model.WithdrawStatusRejected:
		return true
	default:
		return false
	}
}

func isPiggyConfirmingExpired(order *model.WithdrawOrder) bool {
	if order == nil || order.Status != model.WithdrawStatusConfirming {
		return false
	}
	ttl := operation_setting.GetPiggyWithdrawSetting().CallbackLockTTL
	if ttl <= 0 {
		ttl = operation_setting.PiggyWithdrawDefaultCallbackLockTTL
	}
	if order.UpdatedAt <= 0 {
		return true
	}
	return common.GetTimestamp()-order.UpdatedAt >= int64(ttl)
}

func isPiggyCancellingExpired(order *model.WithdrawOrder) bool {
	if order == nil || order.Status != model.WithdrawStatusCancelling {
		return false
	}
	ttl := operation_setting.GetPiggyWithdrawSetting().CallbackLockTTL
	if ttl <= 0 {
		ttl = operation_setting.PiggyWithdrawDefaultCallbackLockTTL
	}
	if order.UpdatedAt <= 0 {
		return true
	}
	return common.GetTimestamp()-order.UpdatedAt >= int64(ttl)
}

func piggyApprovedSubmissionRecoveryDelay(setting *operation_setting.PiggyWithdrawSetting) int64 {
	timeout := operation_setting.PiggyWithdrawDefaultRequestTimeout
	if setting != nil && setting.RequestTimeout > 0 {
		timeout = setting.RequestTimeout
	}
	delay := int64(timeout * 2)
	if delay < 60 {
		return 60
	}
	return delay
}

func shouldScanPiggyManualReviewOrder(order *model.WithdrawOrder) bool {
	if order == nil || order.Status != model.WithdrawStatusManualReview {
		return false
	}
	status := strings.TrimSpace(order.CompensationStatus)
	// 已人工关闭的订单不能再由补偿扫描推进，避免覆盖运营结论或重复资金变更。
	if status == piggyCompensationStatusManualProcessed {
		return false
	}
	return status == "" || status == piggyCompensationStatusPending
}

func shouldScanPiggyApprovedOrder(order *model.WithdrawOrder) bool {
	if order == nil || order.Status != model.WithdrawStatusApproved {
		return false
	}
	setting := operation_setting.GetPiggyWithdrawSetting()
	deadline := common.GetTimestamp() - piggyApprovedSubmissionRecoveryDelay(setting)
	if order.ReviewedAt <= 0 || order.ReviewedAt > deadline {
		return false
	}
	if order.UpdatedAt > 0 && order.UpdatedAt > deadline {
		return false
	}
	switch strings.TrimSpace(order.CompensationStatus) {
	case "", piggyCompensationStatusPending:
		return true
	case piggyCompensationStatusSubmitRecovering:
		return order.UpdatedAt <= 0 || order.UpdatedAt <= deadline
	default:
		return false
	}
}

func markPiggyOrderManualReviewByIdIfActive(orderId int, reason string, reqDigest string, respDigest string, allowedStatuses ...string) error {
	_, err := markPiggyOrderManualReviewByIdIfActiveWithResult(orderId, reason, reqDigest, respDigest, allowedStatuses...)
	return err
}

func markPiggyOrderManualReviewByIdIfActiveWithResult(orderId int, reason string, reqDigest string, respDigest string, allowedStatuses ...string) (bool, error) {
	return markPiggyOrderManualReviewIfActiveWithResult(
		model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", orderId),
		reason,
		reqDigest,
		respDigest,
		allowedStatuses...,
	)
}

func markPiggyOrderManualReviewByNoIfActive(withdrawNo string, reason string, reqDigest string, respDigest string, allowedStatuses ...string) error {
	_, err := markPiggyOrderManualReviewIfActiveWithResult(
		model.DB.Model(&model.WithdrawOrder{}).Where("withdraw_no = ?", strings.TrimSpace(withdrawNo)),
		reason,
		reqDigest,
		respDigest,
		allowedStatuses...,
	)
	return err
}

func markPiggyOrderSubmitRecoverableByIdIfActive(orderId int, reason string, reqDigest string, respDigest string, allowedStatuses ...string) (bool, error) {
	updates := map[string]interface{}{
		"status":               model.WithdrawStatusApproved,
		"piggy_status":         model.WithdrawStatusApproved,
		"manual_review_reason": strings.TrimSpace(reason),
		"compensation_status":  piggyCompensationStatusPending,
		"updated_at":           common.GetTimestamp(),
	}
	piggyApplyDigestUpdates(updates, reqDigest, respDigest)
	statuses := allowedStatuses
	if len(statuses) == 0 {
		statuses = []string{model.WithdrawStatusApproved}
	}
	result := model.DB.Model(&model.WithdrawOrder{}).
		Where("id = ? AND provider = ?", orderId, model.WithdrawProviderPiggyLaborV3).
		Where("status IN ?", statuses).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func markPiggyOrderManualReviewIfActive(query *gorm.DB, reason string, reqDigest string, respDigest string, allowedStatuses ...string) error {
	_, err := markPiggyOrderManualReviewIfActiveWithResult(query, reason, reqDigest, respDigest, allowedStatuses...)
	return err
}

func markPiggyOrderManualReviewIfActiveWithResult(query *gorm.DB, reason string, reqDigest string, respDigest string, allowedStatuses ...string) (bool, error) {
	if query == nil {
		return false, nil
	}
	updates := map[string]interface{}{
		"status":               model.WithdrawStatusManualReview,
		"piggy_status":         model.WithdrawStatusManualReview,
		"manual_review_reason": strings.TrimSpace(reason),
		"compensation_status":  piggyCompensationStatusPending,
	}
	piggyApplyDigestUpdates(updates, reqDigest, respDigest)
	statuses := allowedStatuses
	if len(statuses) == 0 {
		statuses = []string{
			model.WithdrawStatusSubmitted,
			model.WithdrawStatusAwaitConfirm,
			model.WithdrawStatusConfirming,
			model.WithdrawStatusCancelling,
			model.WithdrawStatusManualReview,
		}
	}
	result := query.
		Where("provider = ?", model.WithdrawProviderPiggyLaborV3).
		Where("status IN ?", statuses).
		Where("(status <> ? OR compensation_status = ? OR compensation_status = ? OR compensation_status IS NULL)", model.WithdrawStatusManualReview, "", piggyCompensationStatusPending).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func markPiggyConfirmingOrderManualReview(orderId int, reason string, reqDigest string, respDigest string) {
	_ = markPiggyOrderManualReviewByIdIfActive(orderId, reason, reqDigest, respDigest, model.WithdrawStatusConfirming)
}

func markPiggyCancellingOrderManualReview(orderId int, reason string, reqDigest string, respDigest string) {
	_ = markPiggyOrderManualReviewByIdIfActive(orderId, reason, reqDigest, respDigest, model.WithdrawStatusCancelling)
}

func markExpiredPiggyCancellingManualReview(withdrawNo string) error {
	return markPiggyOrderManualReviewByNoIfActive(withdrawNo, "小猪取消结果超时未确认，请人工核对取消结果", "", "", model.WithdrawStatusCancelling)
}

func markPiggyOrderManualReviewByNo(withdrawNo string, reason string) error {
	return markPiggyOrderManualReviewByNoIfActive(withdrawNo, reason, "", "")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func maxFloat(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func IsPiggyClientTimeout(err error) bool {
	if err == nil {
		return false
	}
	type timeout interface {
		Timeout() bool
	}
	var netErr timeout
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, http.ErrHandlerTimeout)
}
