package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateVipActivationCommissionOptionRejectsNaN(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	oldVipActivationPrice := paymentSetting.VipActivationPrice
	oldLevel1Amount := paymentSetting.VipActivationCommissionLevel1Amount
	oldLevel2Amount := paymentSetting.VipActivationCommissionLevel2Amount
	paymentSetting.VipActivationPrice = 1680
	paymentSetting.VipActivationCommissionLevel1Amount = 10
	paymentSetting.VipActivationCommissionLevel2Amount = 5
	t.Cleanup(func() {
		paymentSetting.VipActivationPrice = oldVipActivationPrice
		paymentSetting.VipActivationCommissionLevel1Amount = oldLevel1Amount
		paymentSetting.VipActivationCommissionLevel2Amount = oldLevel2Amount
	})

	err := validateVipActivationOption("payment_setting.vip_activation_commission_level1_amount", "NaN")
	var priceErr error
	require.NotPanics(t, func() {
		priceErr = validateVipActivationOption("payment_setting.vip_activation_price", "NaN")
	})

	assert.Error(t, err)
	assert.Error(t, priceErr)
}

func TestValidateVipActivationCommissionOptionRejectsLegacyRateKey(t *testing.T) {
	err := validateVipActivationOption("payment_setting.vip_activation_commission_level1_rate", "0.2")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "固定金额")
}

func TestValidateVipActivationCommissionOptionRejectsTotalGreaterThanPrice(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	oldVipActivationPrice := paymentSetting.VipActivationPrice
	oldLevel1Amount := paymentSetting.VipActivationCommissionLevel1Amount
	oldLevel2Amount := paymentSetting.VipActivationCommissionLevel2Amount
	paymentSetting.VipActivationPrice = 1680
	paymentSetting.VipActivationCommissionLevel1Amount = 1000
	paymentSetting.VipActivationCommissionLevel2Amount = 400
	t.Cleanup(func() {
		paymentSetting.VipActivationPrice = oldVipActivationPrice
		paymentSetting.VipActivationCommissionLevel1Amount = oldLevel1Amount
		paymentSetting.VipActivationCommissionLevel2Amount = oldLevel2Amount
	})

	err := validateVipActivationOption("payment_setting.vip_activation_commission_level2_amount", "800")

	assert.Error(t, err)
}

func TestValidateVipActivationOptionRejectsMoreThanTwoDecimalPlaces(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	oldVipActivationPrice := paymentSetting.VipActivationPrice
	oldLevel1Amount := paymentSetting.VipActivationCommissionLevel1Amount
	oldLevel2Amount := paymentSetting.VipActivationCommissionLevel2Amount
	paymentSetting.VipActivationPrice = 1680
	paymentSetting.VipActivationCommissionLevel1Amount = 1000
	paymentSetting.VipActivationCommissionLevel2Amount = 400
	t.Cleanup(func() {
		paymentSetting.VipActivationPrice = oldVipActivationPrice
		paymentSetting.VipActivationCommissionLevel1Amount = oldLevel1Amount
		paymentSetting.VipActivationCommissionLevel2Amount = oldLevel2Amount
	})

	priceErr := validateVipActivationOption("payment_setting.vip_activation_price", "19.999")
	level1Err := validateVipActivationOption("payment_setting.vip_activation_commission_level1_amount", "1000.001")

	require.Error(t, priceErr)
	require.Error(t, level1Err)
	assert.Contains(t, priceErr.Error(), "2 位小数")
	assert.Contains(t, level1Err.Error(), "2 位小数")
}

func TestValidateAlipayOptionRejectsInvalidMoneyAndURL(t *testing.T) {
	unitPriceErr := validateAlipayOption("AlipayUnitPrice", "0")
	minTopUpErr := validateAlipayOption("AlipayMinTopUp", "-1")
	returnURLErr := validateAlipayOption("AlipayReturnUrl", "ftp://example.com/return")
	notifyURLErr := validateAlipayOption("AlipayNotifyUrl", "callback")

	require.Error(t, unitPriceErr)
	require.Error(t, minTopUpErr)
	require.Error(t, returnURLErr)
	require.Error(t, notifyURLErr)
	assert.Contains(t, unitPriceErr.Error(), "大于 0")
	assert.Contains(t, minTopUpErr.Error(), "大于等于 0")
	assert.Contains(t, returnURLErr.Error(), "http:// 或 https://")
	assert.Contains(t, notifyURLErr.Error(), "http:// 或 https://")
}

func TestVisiblePublicKeyOptionIncludesAlipayPublicKey(t *testing.T) {
	assert.True(t, isVisiblePublicKeyOption("AlipayPublicKey"))
	assert.False(t, isVisiblePublicKeyOption("AlipayPrivateKey"))
}

func TestGetOptionsIncludesPiggyAppKeyButHidesAppSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)

	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		"piggy_withdraw_setting.app_key":    "piggy-app-key",
		"piggy_withdraw_setting.app_secret": "piggy-app-secret",
		"AlipayPublicKey":                   "alipay-public-key",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/option/", nil)

	GetOptions(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool           `json:"success"`
		Data    []model.Option `json:"data"`
	}
	require.NoError(t, common.UnmarshalJsonStr(recorder.Body.String(), &response))
	require.True(t, response.Success)

	optionValues := make(map[string]string, len(response.Data))
	for _, item := range response.Data {
		optionValues[item.Key] = item.Value
	}

	assert.Equal(t, "piggy-app-key", optionValues["piggy_withdraw_setting.app_key"])
	assert.Equal(t, "alipay-public-key", optionValues["AlipayPublicKey"])
	_, exists := optionValues["piggy_withdraw_setting.app_secret"]
	assert.False(t, exists)
}

func TestValidateAlipayOptionAllowsSupportedKeys(t *testing.T) {
	originalUnitPrice := setting.AlipayUnitPrice
	originalMinTopUp := setting.AlipayMinTopUp
	t.Cleanup(func() {
		setting.AlipayUnitPrice = originalUnitPrice
		setting.AlipayMinTopUp = originalMinTopUp
	})

	assert.NoError(t, validateAlipayOption("AlipayEnabled", "true"))
	assert.NoError(t, validateAlipayOption("AlipaySandbox", "false"))
	assert.NoError(t, validateAlipayOption("AlipayAppId", "2021000000000000"))
	assert.NoError(t, validateAlipayOption("AlipayPrivateKey", "private-key"))
	assert.NoError(t, validateAlipayOption("AlipayPublicKey", "public-key"))
	assert.NoError(t, validateAlipayOption("AlipayUnitPrice", "7.3"))
	assert.NoError(t, validateAlipayOption("AlipayMinTopUp", "1"))
	assert.NoError(t, validateAlipayOption("AlipayReturnUrl", ""))
	assert.NoError(t, validateAlipayOption("AlipayNotifyUrl", "https://example.com/api/alipay/notify"))
	assert.NoError(t, validateAlipayOption("UnrelatedKey", "bad-url"))
}

func TestValidateWechatPayOptionRejectsInvalidMoneyKeyAndURL(t *testing.T) {
	unitPriceErr := validateWechatPayOption("WechatPayUnitPrice", "0")
	minTopUpErr := validateWechatPayOption("WechatPayMinTopUp", "-1")
	apiKeyErr := validateWechatPayOption("WechatPayAPIv3Key", "short")
	notifyURLErr := validateWechatPayOption("WechatPayNotifyUrl", "callback")
	sandboxErr := validateWechatPayOption("WechatPaySandbox", "true")

	require.Error(t, unitPriceErr)
	require.Error(t, minTopUpErr)
	require.Error(t, apiKeyErr)
	require.Error(t, notifyURLErr)
	require.Error(t, sandboxErr)
	assert.Contains(t, unitPriceErr.Error(), "大于 0")
	assert.Contains(t, minTopUpErr.Error(), "大于等于 0")
	assert.Contains(t, apiKeyErr.Error(), "32")
	assert.Contains(t, notifyURLErr.Error(), "http:// 或 https://")
	assert.Contains(t, sandboxErr.Error(), "不支持")
}

func TestValidateWechatPayOptionRejectsEnableWhenRequiredFieldsMissing(t *testing.T) {
	originalEnabled := setting.WechatPayEnabled
	originalAppID := setting.WechatPayAppId
	originalMchID := setting.WechatPayMchId
	originalSerialNo := setting.WechatPayMerchantSerialNo
	originalPrivateKey := setting.WechatPayMerchantPrivateKey
	originalAPIv3Key := setting.WechatPayAPIv3Key
	originalPlatformKey := setting.WechatPayPlatformPublicKey
	t.Cleanup(func() {
		setting.WechatPayEnabled = originalEnabled
		setting.WechatPayAppId = originalAppID
		setting.WechatPayMchId = originalMchID
		setting.WechatPayMerchantSerialNo = originalSerialNo
		setting.WechatPayMerchantPrivateKey = originalPrivateKey
		setting.WechatPayAPIv3Key = originalAPIv3Key
		setting.WechatPayPlatformPublicKey = originalPlatformKey
	})

	setting.WechatPayAppId = "wx1234567890abcdef"
	setting.WechatPayMchId = "1900000001"
	setting.WechatPayMerchantSerialNo = "7777777777777777777777777777777777777777"
	setting.WechatPayMerchantPrivateKey = ""
	setting.WechatPayAPIv3Key = "0123456789abcdef0123456789abcdef"
	setting.WechatPayPlatformPublicKey = "platform-public-key"

	err := validateWechatPayOption("WechatPayEnabled", "true")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "未完整配置")
}

func TestValidateWechatPayOptionAllowsSupportedKeys(t *testing.T) {
	originalAppID := setting.WechatPayAppId
	originalMchID := setting.WechatPayMchId
	originalSerialNo := setting.WechatPayMerchantSerialNo
	originalPrivateKey := setting.WechatPayMerchantPrivateKey
	originalAPIv3Key := setting.WechatPayAPIv3Key
	originalPlatformKey := setting.WechatPayPlatformPublicKey
	originalUnitPrice := setting.WechatPayUnitPrice
	originalMinTopUp := setting.WechatPayMinTopUp
	t.Cleanup(func() {
		setting.WechatPayAppId = originalAppID
		setting.WechatPayMchId = originalMchID
		setting.WechatPayMerchantSerialNo = originalSerialNo
		setting.WechatPayMerchantPrivateKey = originalPrivateKey
		setting.WechatPayAPIv3Key = originalAPIv3Key
		setting.WechatPayPlatformPublicKey = originalPlatformKey
		setting.WechatPayUnitPrice = originalUnitPrice
		setting.WechatPayMinTopUp = originalMinTopUp
	})

	setting.WechatPayAppId = "wx1234567890abcdef"
	setting.WechatPayMchId = "1900000001"
	setting.WechatPayMerchantSerialNo = "7777777777777777777777777777777777777777"
	setting.WechatPayMerchantPrivateKey = "merchant-private-key"
	setting.WechatPayAPIv3Key = "0123456789abcdef0123456789abcdef"
	setting.WechatPayPlatformPublicKey = "platform-public-key"

	assert.NoError(t, validateWechatPayOption("WechatPayEnabled", "true"))
	assert.NoError(t, validateWechatPayOption("WechatPaySandbox", "false"))
	assert.NoError(t, validateWechatPayOption("WechatPayAppId", "wx1234567890abcdef"))
	assert.NoError(t, validateWechatPayOption("WechatPayMchId", "1900000001"))
	assert.NoError(t, validateWechatPayOption("WechatPayMerchantSerialNo", "7777777777777777777777777777777777777777"))
	assert.NoError(t, validateWechatPayOption("WechatPayMerchantPrivateKey", "merchant-private-key"))
	assert.NoError(t, validateWechatPayOption("WechatPayAPIv3Key", "0123456789abcdef0123456789abcdef"))
	assert.NoError(t, validateWechatPayOption("WechatPayPlatformSerialNo", "8888888888888888888888888888888888888888"))
	assert.NoError(t, validateWechatPayOption("WechatPayPlatformPublicKey", "platform-public-key"))
	assert.NoError(t, validateWechatPayOption("WechatPayUnitPrice", "7.3"))
	assert.NoError(t, validateWechatPayOption("WechatPayMinTopUp", "1"))
	assert.NoError(t, validateWechatPayOption("WechatPayNotifyUrl", "https://example.com/api/wechat/notify"))
	assert.NoError(t, validateWechatPayOption("UnrelatedKey", "bad-url"))
}

func TestApplyQiniuKeyOptionSupportsMarketCatalogAndOfficialLedgerFields(t *testing.T) {
	next := operation_setting.QiniuKeySetting{}

	cases := []model.Option{
		{Key: "qiniu_key_setting.market_catalog_enabled", Value: "true"},
		{Key: "qiniu_key_setting.market_catalog_base_url", Value: "https://openai.qiniu.com"},
		{Key: "qiniu_key_setting.market_catalog_ttl_seconds", Value: "120"},
		{Key: "qiniu_key_setting.market_catalog_overseas", Value: "false"},
		{Key: "qiniu_key_setting.market_catalog_fallback_enabled", Value: "true"},
		{Key: "qiniu_key_setting.official_ledger_enabled", Value: "true"},
		{Key: "qiniu_key_setting.official_ledger_cutover_time", Value: "1710000000"},
		{Key: "qiniu_key_setting.official_ledger_sync_interval_seconds", Value: "90"},
		{Key: "qiniu_key_setting.official_ledger_window_hours", Value: "8"},
		{Key: "qiniu_key_setting.official_ledger_window_days", Value: "3"},
		{Key: "qiniu_key_setting.official_ledger_batch_size", Value: "50"},
		{Key: "qiniu_key_setting.official_ledger_rate_limit_per_second", Value: "2"},
		{Key: "qiniu_key_setting.official_ledger_retry_interval_seconds", Value: "600"},
		{Key: "qiniu_key_setting.cost_detail_cutover_time", Value: "1710100000"},
		{Key: "qiniu_key_setting.cost_detail_lookback_days", Value: "4"},
		{Key: "qiniu_key_setting.cost_detail_auto_apply_enabled", Value: "false"},
		{Key: "qiniu_key_setting.child_account_base_url", Value: "https://api.qiniu.com"},
		{Key: "qiniu_key_setting.child_account_binding_enabled", Value: "true"},
		{Key: "qiniu_key_setting.child_account_assignment_mode", Value: operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild},
		{Key: "qiniu_key_setting.child_account_binding_cutover_time", Value: "1710200000"},
	}
	for _, item := range cases {
		applied, err := applyQiniuKeyOption(&next, item.Key, item.Value)
		require.NoError(t, err)
		require.True(t, applied, item.Key)
	}

	assert.True(t, next.MarketCatalogEnabled)
	assert.Equal(t, "https://openai.qiniu.com", next.MarketCatalogBaseURL)
	assert.Equal(t, 120, next.MarketCatalogTTLSeconds)
	assert.False(t, next.MarketCatalogOverseas)
	assert.True(t, next.MarketCatalogFallbackEnabled)
	assert.True(t, next.OfficialLedgerEnabled)
	assert.Equal(t, int64(1710000000), next.OfficialLedgerCutoverTime)
	assert.Equal(t, 90, next.OfficialLedgerSyncIntervalSeconds)
	assert.Equal(t, 8, next.OfficialLedgerWindowHours)
	assert.Equal(t, 3, next.OfficialLedgerWindowDays)
	assert.Equal(t, 50, next.OfficialLedgerBatchSize)
	assert.Equal(t, 2, next.OfficialLedgerRateLimitPerSecond)
	assert.Equal(t, 600, next.OfficialLedgerRetryIntervalSeconds)
	costDetailCutover := reflect.ValueOf(next).FieldByName("CostDetailCutoverTime")
	require.True(t, costDetailCutover.IsValid(), "CostDetailCutoverTime field must exist")
	assert.Equal(t, int64(1710100000), costDetailCutover.Int())
	assert.Equal(t, 4, next.CostDetailLookbackDays)
	assert.False(t, next.CostDetailAutoApplyEnabled)
	assert.Equal(t, "https://api.qiniu.com", next.ChildAccountBaseURL)
	assert.True(t, next.ChildAccountBindingEnabled)
	assert.Equal(t, operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild, next.ChildAccountAssignmentMode)
	assert.Equal(t, int64(1710200000), next.ChildAccountBindingCutoverTime)
}

func TestPrepareQiniuKeyOptionsForPersistenceSetsInitialChildBindingCutoverOnce(t *testing.T) {
	setting := operation_setting.GetQiniuKeySetting()
	original := *setting
	t.Cleanup(func() {
		*setting = original
	})
	*setting = operation_setting.QiniuKeySetting{
		BaseURL:                    operation_setting.QiniuKeyDefaultBaseURL,
		ChildAccountAssignmentMode: operation_setting.QiniuChildAccountAssignmentModeParentOnly,
	}

	prepared := prepareQiniuKeyOptionsForPersistence([]model.Option{
		{Key: "qiniu_key_setting.child_account_binding_enabled", Value: "true"},
		{Key: "qiniu_key_setting.child_account_assignment_mode", Value: operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild},
	}, 1710200300)
	assert.Equal(t, "1710200300", qiniuOptionValue(prepared, "qiniu_key_setting.child_account_binding_cutover_time"))

	*setting = operation_setting.QiniuKeySetting{
		BaseURL:                        operation_setting.QiniuKeyDefaultBaseURL,
		ChildAccountBindingEnabled:     false,
		ChildAccountAssignmentMode:     operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild,
		ChildAccountBindingCutoverTime: 1710200000,
	}
	prepared = prepareQiniuKeyOptionsForPersistence([]model.Option{
		{Key: "qiniu_key_setting.child_account_binding_enabled", Value: "true"},
	}, 1710200400)
	assert.Empty(t, qiniuOptionValue(prepared, "qiniu_key_setting.child_account_binding_cutover_time"))
}

func TestValidateQiniuKeyOptionRejectsInvalidMarketAndOfficialLedgerValues(t *testing.T) {
	setting := operation_setting.GetQiniuKeySetting()
	original := *setting
	t.Cleanup(func() {
		*setting = original
	})
	*setting = operation_setting.QiniuKeySetting{
		BaseURL:                      operation_setting.QiniuKeyDefaultBaseURL,
		MarketCatalogEnabled:         true,
		MarketCatalogBaseURL:         operation_setting.QiniuMarketDefaultBaseURL,
		MarketCatalogTTLSeconds:      operation_setting.QiniuMarketCatalogDefaultTTLSeconds,
		MarketCatalogOverseas:        true,
		MarketCatalogFallbackEnabled: true,
	}

	assert.NoError(t, validateQiniuKeyOption("qiniu_key_setting.market_catalog_enabled", "true"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.market_catalog_base_url", "ftp://qiniu.example.com"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.market_catalog_ttl_seconds", "0"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.official_ledger_cutover_time", "-1"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.official_ledger_sync_interval_seconds", "0"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.official_ledger_window_hours", "0"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.official_ledger_window_days", "0"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.official_ledger_batch_size", "0"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.official_ledger_rate_limit_per_second", "0"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.official_ledger_retry_interval_seconds", "0"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.cost_detail_cutover_time", "-1"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.cost_detail_lookback_days", "0"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.cost_detail_lookback_days", fmt.Sprint(operation_setting.QiniuCostDetailMaxLookbackDays+1)))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.child_account_base_url", "ftp://api.qiniu.com"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.child_account_assignment_mode", "shared_child"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.child_account_binding_cutover_time", "-1"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.child_account_email_prefix", "bad prefix"))
	assert.Error(t, validateQiniuKeyOption("qiniu_key_setting.child_account_email_prefix", "bad@prefix"))
}

func qiniuOptionValue(options []model.Option, key string) string {
	for _, option := range options {
		if option.Key == key {
			return option.Value
		}
	}
	return ""
}

func TestValidatePiggyWithdrawOptionRejectsClearingEnabledAppSecret(t *testing.T) {
	piggySetting := operation_setting.GetPiggyWithdrawSetting()
	original := *piggySetting
	t.Cleanup(func() {
		*piggySetting = original
	})
	*piggySetting = operation_setting.PiggyWithdrawSetting{
		Enabled:         true,
		Domain:          "https://piggy.example.com",
		AppKey:          "app-key",
		AppSecret:       "1234567890abcdef",
		AESIV:           "0000000000000000",
		TaxFundId:       "tax-fund",
		PositionName:    "技术服务",
		Position:        "tech",
		SignNotifyUrl:   "https://app.example.com/api/withdraw/piggy/contract/notify",
		PayNotifyUrl:    "https://app.example.com/api/withdraw/piggy/payment/notify",
		RequestTimeout:  5,
		CallbackLockTTL: 60,
		CooldownMinutes: 30,
		CalcType:        "C",
	}

	err := validatePiggyWithdrawOption("piggy_withdraw_setting.app_secret", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "app_secret")
}

func TestValidatePiggyWithdrawOptionAllowsClearingDisabledAppSecret(t *testing.T) {
	piggySetting := operation_setting.GetPiggyWithdrawSetting()
	original := *piggySetting
	t.Cleanup(func() {
		*piggySetting = original
	})
	*piggySetting = operation_setting.PiggyWithdrawSetting{
		Enabled:         false,
		Domain:          "https://piggy.example.com",
		AppKey:          "app-key",
		AppSecret:       "1234567890abcdef",
		AESIV:           "0000000000000000",
		TaxFundId:       "tax-fund",
		PositionName:    "技术服务",
		Position:        "tech",
		SignNotifyUrl:   "https://app.example.com/api/withdraw/piggy/contract/notify",
		PayNotifyUrl:    "https://app.example.com/api/withdraw/piggy/payment/notify",
		RequestTimeout:  5,
		CallbackLockTTL: 60,
		CooldownMinutes: 30,
		CalcType:        "C",
	}

	assert.NoError(t, validatePiggyWithdrawOption("piggy_withdraw_setting.app_secret", ""))
}

func TestValidatePiggyWithdrawOptionRejectsClearingEnabledAESIV(t *testing.T) {
	piggySetting := operation_setting.GetPiggyWithdrawSetting()
	original := *piggySetting
	t.Cleanup(func() {
		*piggySetting = original
	})
	*piggySetting = operation_setting.PiggyWithdrawSetting{
		Enabled:         true,
		Domain:          "https://piggy.example.com",
		AppKey:          "app-key",
		AppSecret:       "1234567890abcdef",
		AESIV:           "0000000000000000",
		TaxFundId:       "tax-fund",
		PositionName:    "技术服务",
		Position:        "tech",
		SignNotifyUrl:   "https://app.example.com/api/withdraw/piggy/contract/notify",
		PayNotifyUrl:    "https://app.example.com/api/withdraw/piggy/payment/notify",
		RequestTimeout:  5,
		CallbackLockTTL: 60,
		CooldownMinutes: 30,
		CalcType:        "C",
	}

	err := validatePiggyWithdrawOption("piggy_withdraw_setting.aes_iv", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "aes_iv")
}

func TestValidatePiggyWithdrawOptionAllowsClearingDisabledAESIV(t *testing.T) {
	piggySetting := operation_setting.GetPiggyWithdrawSetting()
	original := *piggySetting
	t.Cleanup(func() {
		*piggySetting = original
	})
	*piggySetting = operation_setting.PiggyWithdrawSetting{
		Enabled:         false,
		Domain:          "https://piggy.example.com",
		AppKey:          "app-key",
		AppSecret:       "1234567890abcdef",
		AESIV:           "0000000000000000",
		TaxFundId:       "tax-fund",
		PositionName:    "技术服务",
		Position:        "tech",
		SignNotifyUrl:   "https://app.example.com/api/withdraw/piggy/contract/notify",
		PayNotifyUrl:    "https://app.example.com/api/withdraw/piggy/payment/notify",
		RequestTimeout:  5,
		CallbackLockTTL: 60,
		CooldownMinutes: 30,
		CalcType:        "C",
	}

	assert.NoError(t, validatePiggyWithdrawOption("piggy_withdraw_setting.aes_iv", ""))
}

func TestValidatePiggyWithdrawOptionRejectsEnabledPayNotifyURLWithQuery(t *testing.T) {
	piggySetting := operation_setting.GetPiggyWithdrawSetting()
	original := *piggySetting
	t.Cleanup(func() {
		*piggySetting = original
	})
	*piggySetting = operation_setting.PiggyWithdrawSetting{
		Enabled:         true,
		Domain:          "https://piggy.example.com",
		AppKey:          "app-key",
		AppSecret:       "1234567890abcdef",
		AESIV:           "0000000000000000",
		TaxFundId:       "tax-fund",
		PositionName:    "技术服务",
		Position:        "tech",
		SignNotifyUrl:   "https://app.example.com/api/withdraw/piggy/contract/notify",
		PayNotifyUrl:    "https://app.example.com/api/withdraw/piggy/payment/notify",
		RequestTimeout:  5,
		CallbackLockTTL: 60,
		CooldownMinutes: 30,
		CalcType:        "C",
	}

	err := validatePiggyWithdrawOption("piggy_withdraw_setting.pay_notify_url", "https://app.example.com/api/withdraw/piggy/payment/notify?token=abc")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "查询参数")
}

func TestApplyPiggyWithdrawOptionPlatformFeeRate(t *testing.T) {
	next := operation_setting.PiggyWithdrawSetting{}

	applied, err := applyPiggyWithdrawOption(&next, "piggy_withdraw_setting.platform_fee_rate", "8.1256")
	require.NoError(t, err)
	require.True(t, applied)
	assert.Equal(t, 8.1256, next.PlatformFeeRate)

	applied, err = applyPiggyWithdrawOption(&next, "piggy_withdraw_setting.platform_fee_rate", "0")
	require.NoError(t, err)
	require.True(t, applied)
	assert.Equal(t, float64(0), next.PlatformFeeRate)
}

func TestValidatePiggyWithdrawOptionRejectsInvalidPlatformFeeRate(t *testing.T) {
	piggySetting := operation_setting.GetPiggyWithdrawSetting()
	original := *piggySetting
	t.Cleanup(func() {
		*piggySetting = original
	})
	*piggySetting = operation_setting.PiggyWithdrawSetting{
		Enabled:         false,
		Domain:          "https://piggy.example.com",
		RequestTimeout:  5,
		CallbackLockTTL: 60,
		CooldownMinutes: 30,
		CalcType:        "C",
	}

	tests := []string{"-0.01", "NaN", "100", "100.01", "8.12345", "abc"}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			err := validatePiggyWithdrawOption("piggy_withdraw_setting.platform_fee_rate", value)
			assert.Error(t, err)
		})
	}
}

func TestValidateBatchOptionUpdatesRejectsEmptyAndDuplicateKeys(t *testing.T) {
	emptyErr := validateBatchOptionUpdates(nil)
	duplicateErr := validateBatchOptionUpdates([]model.Option{
		{Key: "piggy_withdraw_setting.domain", Value: "https://piggy.example.com"},
		{Key: "piggy_withdraw_setting.domain", Value: "https://piggy2.example.com"},
	})

	require.Error(t, emptyErr)
	require.Error(t, duplicateErr)
	assert.Contains(t, emptyErr.Error(), "不能为空")
	assert.Contains(t, duplicateErr.Error(), "重复")
}

func TestValidateBatchPiggyWithdrawOptionsUsesMergedConfiguration(t *testing.T) {
	piggySetting := operation_setting.GetPiggyWithdrawSetting()
	original := *piggySetting
	t.Cleanup(func() {
		*piggySetting = original
	})
	*piggySetting = operation_setting.PiggyWithdrawSetting{
		Enabled:         false,
		Domain:          "https://old-piggy.example.com",
		RequestTimeout:  5,
		CallbackLockTTL: 60,
		CooldownMinutes: 30,
		CalcType:        "C",
	}

	err := validateBatchOptionUpdates([]model.Option{
		{Key: "piggy_withdraw_setting.enabled", Value: "true"},
		{Key: "piggy_withdraw_setting.domain", Value: "https://piggy.example.com"},
		{Key: "piggy_withdraw_setting.app_key", Value: "app-key"},
		{Key: "piggy_withdraw_setting.app_secret", Value: "1234567890abcdef"},
		{Key: "piggy_withdraw_setting.aes_iv", Value: "0000000000000000"},
		{Key: "piggy_withdraw_setting.tax_fund_id", Value: "tax-fund"},
		{Key: "piggy_withdraw_setting.position_name", Value: "技术服务"},
		{Key: "piggy_withdraw_setting.position", Value: "tech"},
		{Key: "piggy_withdraw_setting.sign_notify_url", Value: "https://app.example.com/api/withdraw/piggy/contract/notify"},
		{Key: "piggy_withdraw_setting.pay_notify_url", Value: "https://app.example.com/api/withdraw/piggy/payment/notify"},
		{Key: "piggy_withdraw_setting.request_timeout", Value: "10"},
		{Key: "piggy_withdraw_setting.callback_lock_ttl", Value: "120"},
		{Key: "piggy_withdraw_setting.cooldown_minutes", Value: "0"},
		{Key: "piggy_withdraw_setting.calc_type", Value: "C"},
		{Key: "piggy_withdraw_setting.platform_fee_rate", Value: "8.1256"},
	})

	assert.NoError(t, err)
}

func TestValidateBatchWechatPayOptionsUsesMergedConfiguration(t *testing.T) {
	originalEnabled := setting.WechatPayEnabled
	originalAppID := setting.WechatPayAppId
	originalMchID := setting.WechatPayMchId
	originalSerialNo := setting.WechatPayMerchantSerialNo
	originalPrivateKey := setting.WechatPayMerchantPrivateKey
	originalAPIv3Key := setting.WechatPayAPIv3Key
	originalPlatformKey := setting.WechatPayPlatformPublicKey
	originalUnitPrice := setting.WechatPayUnitPrice
	originalMinTopUp := setting.WechatPayMinTopUp
	t.Cleanup(func() {
		setting.WechatPayEnabled = originalEnabled
		setting.WechatPayAppId = originalAppID
		setting.WechatPayMchId = originalMchID
		setting.WechatPayMerchantSerialNo = originalSerialNo
		setting.WechatPayMerchantPrivateKey = originalPrivateKey
		setting.WechatPayAPIv3Key = originalAPIv3Key
		setting.WechatPayPlatformPublicKey = originalPlatformKey
		setting.WechatPayUnitPrice = originalUnitPrice
		setting.WechatPayMinTopUp = originalMinTopUp
	})
	setting.WechatPayEnabled = false
	setting.WechatPayAppId = ""
	setting.WechatPayMchId = ""
	setting.WechatPayMerchantSerialNo = ""
	setting.WechatPayMerchantPrivateKey = ""
	setting.WechatPayAPIv3Key = ""
	setting.WechatPayPlatformPublicKey = ""

	err := validateBatchOptionUpdates([]model.Option{
		{Key: "WechatPayAppId", Value: "wx1234567890abcdef"},
		{Key: "WechatPayMchId", Value: "1900000001"},
		{Key: "WechatPayMerchantSerialNo", Value: "7777777777777777777777777777777777777777"},
		{Key: "WechatPayMerchantPrivateKey", Value: "merchant-private-key"},
		{Key: "WechatPayAPIv3Key", Value: "0123456789abcdef0123456789abcdef"},
		{Key: "WechatPayPlatformPublicKey", Value: "platform-public-key"},
		{Key: "WechatPayUnitPrice", Value: "7.3"},
		{Key: "WechatPayMinTopUp", Value: "1"},
		{Key: "WechatPayEnabled", Value: "true"},
	})

	assert.NoError(t, err)
}

func TestValidateBatchWechatPayOptionsRejectsEnableWhenMergedConfigurationIncomplete(t *testing.T) {
	originalEnabled := setting.WechatPayEnabled
	originalAppID := setting.WechatPayAppId
	originalMchID := setting.WechatPayMchId
	originalSerialNo := setting.WechatPayMerchantSerialNo
	originalPrivateKey := setting.WechatPayMerchantPrivateKey
	originalAPIv3Key := setting.WechatPayAPIv3Key
	originalPlatformKey := setting.WechatPayPlatformPublicKey
	t.Cleanup(func() {
		setting.WechatPayEnabled = originalEnabled
		setting.WechatPayAppId = originalAppID
		setting.WechatPayMchId = originalMchID
		setting.WechatPayMerchantSerialNo = originalSerialNo
		setting.WechatPayMerchantPrivateKey = originalPrivateKey
		setting.WechatPayAPIv3Key = originalAPIv3Key
		setting.WechatPayPlatformPublicKey = originalPlatformKey
	})
	setting.WechatPayEnabled = false
	setting.WechatPayAppId = ""
	setting.WechatPayMchId = ""
	setting.WechatPayMerchantSerialNo = ""
	setting.WechatPayMerchantPrivateKey = ""
	setting.WechatPayAPIv3Key = ""
	setting.WechatPayPlatformPublicKey = ""

	err := validateBatchOptionUpdates([]model.Option{
		{Key: "WechatPayAppId", Value: "wx1234567890abcdef"},
		{Key: "WechatPayMchId", Value: "1900000001"},
		{Key: "WechatPayMerchantSerialNo", Value: "7777777777777777777777777777777777777777"},
		{Key: "WechatPayAPIv3Key", Value: "0123456789abcdef0123456789abcdef"},
		{Key: "WechatPayPlatformPublicKey", Value: "platform-public-key"},
		{Key: "WechatPayEnabled", Value: "true"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "未完整配置")
}

func TestValidateBatchPiggyWithdrawOptionsRejectsInvalidAESIVWithoutMutatingRuntime(t *testing.T) {
	piggySetting := operation_setting.GetPiggyWithdrawSetting()
	original := *piggySetting
	t.Cleanup(func() {
		*piggySetting = original
	})
	*piggySetting = operation_setting.PiggyWithdrawSetting{
		Enabled:         false,
		Domain:          "https://old-piggy.example.com",
		AppKey:          "old-key",
		AppSecret:       "1234567890abcdef",
		AESIV:           "0000000000000000",
		TaxFundId:       "old-tax",
		PositionName:    "旧岗位",
		Position:        "old-position",
		SignNotifyUrl:   "https://app.example.com/api/withdraw/piggy/contract/notify",
		PayNotifyUrl:    "https://app.example.com/api/withdraw/piggy/payment/notify",
		RequestTimeout:  5,
		CallbackLockTTL: 60,
		CooldownMinutes: 30,
		CalcType:        "C",
	}

	err := validateBatchOptionUpdates([]model.Option{
		{Key: "piggy_withdraw_setting.enabled", Value: "true"},
		{Key: "piggy_withdraw_setting.domain", Value: "https://piggy.example.com"},
		{Key: "piggy_withdraw_setting.aes_iv", Value: "short"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AES IV")
	assert.Equal(t, "https://old-piggy.example.com", piggySetting.Domain)
	assert.Equal(t, "0000000000000000", piggySetting.AESIV)
	assert.False(t, piggySetting.Enabled)
}
