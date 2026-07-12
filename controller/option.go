package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

func isPaymentComplianceOptionKey(key string) bool {
	return strings.HasPrefix(key, "payment_setting.compliance_")
}

func validateVipActivationOption(key string, value string) error {
	switch key {
	case "payment_setting.vip_activation_commission_level1_rate",
		"payment_setting.vip_activation_commission_level2_rate":
		return fmt.Errorf("算力伙伴 开通分佣已改为固定金额，请使用金额配置项")
	case "payment_setting.vip_activation_price",
		"payment_setting.vip_activation_commission_level1_amount",
		"payment_setting.vip_activation_commission_level2_amount":
	default:
		return nil
	}

	amount, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return fmt.Errorf("算力伙伴 开通金额配置必须是数字")
	}
	if !operation_setting.IsVipActivationMoneyAtCentPrecision(amount) {
		return fmt.Errorf("算力伙伴 开通金额配置最多支持 2 位小数")
	}
	paymentSetting := operation_setting.GetPaymentSetting()
	activationPrice := paymentSetting.VipActivationPrice
	level1Amount := paymentSetting.VipActivationCommissionLevel1Amount
	level2Amount := paymentSetting.VipActivationCommissionLevel2Amount
	switch key {
	case "payment_setting.vip_activation_price":
		activationPrice = amount
	case "payment_setting.vip_activation_commission_level1_amount":
		level1Amount = amount
	case "payment_setting.vip_activation_commission_level2_amount":
		level2Amount = amount
	}
	return operation_setting.ValidateVipActivationCommissionAmounts(
		operation_setting.NormalizeVipActivationMoneyToCents(activationPrice),
		level1Amount,
		level2Amount,
	)
}

func validateAlipayOption(key string, value string) error {
	value = strings.TrimSpace(value)
	switch key {
	case "AlipayUnitPrice":
		unitPrice, err := strconv.ParseFloat(value, 64)
		if err != nil || unitPrice <= 0 {
			return fmt.Errorf("支付宝单位价格必须是大于 0 的数字")
		}
	case "AlipayMinTopUp":
		minTopUp, err := strconv.Atoi(value)
		if err != nil || minTopUp < 0 {
			return fmt.Errorf("支付宝最小充值金额必须是大于等于 0 的整数")
		}
	case "AlipayReturnUrl", "AlipayNotifyUrl":
		if value == "" {
			return nil
		}
		parsed, err := url.ParseRequestURI(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("支付宝回调地址必须以 http:// 或 https:// 开头")
		}
	default:
		return nil
	}
	return nil
}

func validateWechatPayOption(key string, value string) error {
	value = strings.TrimSpace(value)
	switch key {
	case "WechatPayUnitPrice":
		unitPrice, err := strconv.ParseFloat(value, 64)
		if err != nil || unitPrice <= 0 {
			return fmt.Errorf("微信支付单位价格必须是大于 0 的数字")
		}
	case "WechatPayMinTopUp":
		minTopUp, err := strconv.Atoi(value)
		if err != nil || minTopUp < 0 {
			return fmt.Errorf("微信支付最小充值金额必须是大于等于 0 的整数")
		}
	case "WechatPayAPIv3Key":
		if value != "" && len([]byte(value)) != 32 {
			return fmt.Errorf("微信支付 API v3 密钥必须是 32 字节")
		}
	case "WechatPayNotifyUrl":
		if value == "" {
			return nil
		}
		parsed, err := url.ParseRequestURI(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("微信支付回调地址必须以 http:// 或 https:// 开头")
		}
	case "WechatPaySandbox":
		if value == "true" {
			return fmt.Errorf("微信支付直连暂不支持沙箱模式")
		}
	case "WechatPayEnabled":
		if value == "true" && !isWechatPaySettingCompleteForEnable() {
			return fmt.Errorf("微信支付直连未完整配置，不能启用")
		}
	default:
		return nil
	}
	return nil
}

func applyPiggyWithdrawOption(next *operation_setting.PiggyWithdrawSetting, key string, value string) (bool, error) {
	if !strings.HasPrefix(key, "piggy_withdraw_setting.") {
		return false, nil
	}
	configKey := strings.TrimPrefix(key, "piggy_withdraw_setting.")
	switch configKey {
	case "enabled":
		next.Enabled = value == "true"
	case "domain":
		next.Domain = strings.TrimSpace(value)
	case "app_key":
		next.AppKey = strings.TrimSpace(value)
	case "app_secret":
		next.AppSecret = strings.TrimSpace(value)
	case "aes_iv":
		next.AESIV = strings.TrimSpace(value)
	case "tax_fund_id":
		next.TaxFundId = strings.TrimSpace(value)
	case "position_name":
		next.PositionName = strings.TrimSpace(value)
	case "position":
		next.Position = strings.TrimSpace(value)
	case "sign_jump_page":
		next.SignJumpPage = strings.TrimSpace(value)
	case "sign_notify_url":
		next.SignNotifyUrl = strings.TrimSpace(value)
	case "pay_notify_url":
		next.PayNotifyUrl = strings.TrimSpace(value)
	case "request_timeout":
		timeout, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || timeout <= 0 {
			return true, fmt.Errorf("小猪请求超时时间必须是大于 0 的整数秒")
		}
		next.RequestTimeout = timeout
	case "callback_lock_ttl":
		ttl, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || ttl <= 0 {
			return true, fmt.Errorf("小猪回调锁 TTL 必须是大于 0 的整数秒")
		}
		next.CallbackLockTTL = ttl
	case "cooldown_minutes":
		minutes, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || minutes < 0 {
			return true, fmt.Errorf("小猪提现冷却时间必须是大于等于 0 的整数分钟")
		}
		next.CooldownMinutes = minutes
	case "forbidden_withdraw_time":
		next.ForbiddenWithdrawTime = strings.TrimSpace(value)
	case "calc_type":
		next.CalcType = strings.TrimSpace(value)
	case "platform_fee_rate":
		rate, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return true, fmt.Errorf("小猪平台服务费率必须是数字")
		}
		if err := operation_setting.ValidatePiggyWithdrawPlatformFeeRate(rate); err != nil {
			return true, err
		}
		next.PlatformFeeRate = rate
	case "bank_remark":
		next.BankRemark = strings.TrimSpace(value)
	default:
		return false, nil
	}
	return true, nil
}

func validatePiggyWithdrawOption(key string, value string) error {
	current := *operation_setting.GetPiggyWithdrawSetting()
	applied, err := applyPiggyWithdrawOption(&current, key, value)
	if err != nil || !applied {
		return err
	}
	return operation_setting.ValidatePiggyWithdrawSettingForEnable(&current)
}

func validatePiggyWithdrawOptions(options []model.Option) error {
	current := *operation_setting.GetPiggyWithdrawSetting()
	appliedAny := false
	for _, option := range options {
		applied, err := applyPiggyWithdrawOption(&current, option.Key, option.Value)
		if err != nil {
			return err
		}
		appliedAny = appliedAny || applied
	}
	if !appliedAny {
		return nil
	}
	return operation_setting.ValidatePiggyWithdrawSettingForEnable(&current)
}

func applyQiniuKeyOption(next *operation_setting.QiniuKeySetting, key string, value string) (bool, error) {
	if !strings.HasPrefix(key, "qiniu_key_setting.") {
		return false, nil
	}
	configKey := strings.TrimPrefix(key, "qiniu_key_setting.")
	switch configKey {
	case "enabled":
		next.Enabled = value == "true"
	case "base_url":
		next.BaseURL = strings.TrimSpace(value)
	case "child_account_base_url":
		baseURL := strings.TrimSpace(value)
		if baseURL != "" {
			parsed, err := url.ParseRequestURI(baseURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				return true, fmt.Errorf("七牛子账户接口域名必须以 http:// 或 https:// 开头")
			}
		}
		next.ChildAccountBaseURL = baseURL
	case "access_key":
		next.AccessKey = strings.TrimSpace(value)
	case "secret_key":
		next.SecretKey = strings.TrimSpace(value)
	case "request_timeout":
		timeout, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || timeout <= 0 {
			return true, fmt.Errorf("Key 请求超时时间必须是大于 0 的整数秒")
		}
		next.RequestTimeout = timeout
	case "retry_interval_seconds":
		interval, err := parsePositiveIntOption(value, "Key 重试间隔")
		if err != nil {
			return true, err
		}
		next.RetryIntervalSeconds = interval
	case "official_ledger_enabled":
		next.OfficialLedgerEnabled = value == "true"
	case "official_ledger_cutover_time":
		cutoverTime, err := parseNonNegativeInt64Option(value, "官方 ledger 切换时间")
		if err != nil {
			return true, err
		}
		next.OfficialLedgerCutoverTime = cutoverTime
	case "official_ledger_sync_interval_seconds":
		interval, err := parsePositiveIntOption(value, "官方 ledger 同步间隔")
		if err != nil {
			return true, err
		}
		next.OfficialLedgerSyncIntervalSeconds = interval
	case "official_ledger_window_hours":
		windowHours, err := parsePositiveIntOption(value, "官方 ledger 小时窗口")
		if err != nil {
			return true, err
		}
		next.OfficialLedgerWindowHours = windowHours
	case "official_ledger_window_days":
		windowDays, err := parsePositiveIntOption(value, "官方 ledger 天级窗口")
		if err != nil {
			return true, err
		}
		next.OfficialLedgerWindowDays = windowDays
	case "official_ledger_batch_size":
		batchSize, err := parsePositiveIntOption(value, "官方 ledger 批量大小")
		if err != nil {
			return true, err
		}
		next.OfficialLedgerBatchSize = batchSize
	case "official_ledger_rate_limit_per_second":
		rateLimit, err := parsePositiveIntOption(value, "官方 ledger 每秒请求限制")
		if err != nil {
			return true, err
		}
		next.OfficialLedgerRateLimitPerSecond = rateLimit
	case "official_ledger_retry_interval_seconds":
		interval, err := parsePositiveIntOption(value, "官方 ledger 重试间隔")
		if err != nil {
			return true, err
		}
		next.OfficialLedgerRetryIntervalSeconds = interval
	case "cost_detail_cutover_time":
		cutoverTime, err := parseNonNegativeInt64Option(value, "cost-detail 自动落账 cutover 时间")
		if err != nil {
			return true, err
		}
		next.CostDetailCutoverTime = cutoverTime
	case "cost_detail_lookback_days":
		lookbackDays, err := parsePositiveIntOption(value, "cost-detail 回扫天数")
		if err != nil {
			return true, err
		}
		if lookbackDays > operation_setting.QiniuCostDetailMaxLookbackDays {
			return true, fmt.Errorf("cost-detail 回扫天数不能超过 %d 天", operation_setting.QiniuCostDetailMaxLookbackDays)
		}
		next.CostDetailLookbackDays = lookbackDays
	case "cost_detail_auto_apply_enabled":
		next.CostDetailAutoApplyEnabled = value == "true"
	case "child_account_binding_enabled":
		next.ChildAccountBindingEnabled = value == "true"
	case "child_account_assignment_mode":
		mode := strings.TrimSpace(strings.ToLower(value))
		if !operation_setting.IsValidQiniuChildAccountAssignmentMode(mode) {
			return true, fmt.Errorf("七牛子账号分配模式无效")
		}
		next.ChildAccountAssignmentMode = mode
	case "child_account_binding_cutover_time":
		cutoverTime, err := parseNonNegativeInt64Option(value, "七牛子账号绑定 cutover 时间")
		if err != nil {
			return true, err
		}
		next.ChildAccountBindingCutoverTime = cutoverTime
	case "market_catalog_enabled":
		next.MarketCatalogEnabled = value == "true"
	case "market_catalog_base_url":
		next.MarketCatalogBaseURL = strings.TrimSpace(value)
	case "market_catalog_ttl_seconds":
		ttl, err := parsePositiveIntOption(value, "模型市场缓存 TTL")
		if err != nil {
			return true, err
		}
		next.MarketCatalogTTLSeconds = ttl
	case "market_catalog_overseas":
		next.MarketCatalogOverseas = value == "true"
	case "market_catalog_fallback_enabled":
		next.MarketCatalogFallbackEnabled = value == "true"
	case "child_account_email_domain":
		domain := strings.TrimSpace(value)
		if err := operation_setting.ValidateQiniuChildAccountEmailDomain(domain); err != nil {
			return true, err
		}
		next.ChildAccountEmailDomain = strings.TrimPrefix(strings.ToLower(domain), "@")
	case "child_account_email_prefix":
		prefix := strings.TrimSpace(value)
		if err := operation_setting.ValidateQiniuChildAccountEmailPrefix(prefix); err != nil {
			return true, err
		}
		next.ChildAccountEmailPrefix = prefix
	case "child_account_password_length":
		length, err := parsePositiveIntOption(value, "七牛子账户密码长度")
		if err != nil {
			return true, err
		}
		if length < 12 || length > 64 {
			return true, fmt.Errorf("七牛子账户密码长度必须在 12 到 64 之间")
		}
		next.ChildAccountPasswordLength = length
	case "child_account_request_timeout":
		timeout, err := parsePositiveIntOption(value, "七牛子账户请求超时时间")
		if err != nil {
			return true, err
		}
		next.ChildAccountRequestTimeout = timeout
	case "child_account_retry_interval_seconds":
		interval, err := parsePositiveIntOption(value, "七牛子账户重试间隔")
		if err != nil {
			return true, err
		}
		next.ChildAccountRetryIntervalSeconds = interval
	default:
		return false, nil
	}
	return true, nil
}

func parsePositiveIntOption(value string, label string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s 必须是大于 0 的整数", label)
	}
	return parsed, nil
}

func parseNonNegativeInt64Option(value string, label string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s 必须是大于等于 0 的整数", label)
	}
	return parsed, nil
}

func validateQiniuKeyOption(key string, value string) error {
	current := *operation_setting.GetQiniuKeySetting()
	applied, err := applyQiniuKeyOption(&current, key, value)
	if err != nil || !applied {
		return err
	}
	return operation_setting.ValidateQiniuKeySettingForEnable(&current)
}

func validateQiniuKeyOptions(options []model.Option) error {
	current := *operation_setting.GetQiniuKeySetting()
	appliedAny := false
	for _, option := range options {
		applied, err := applyQiniuKeyOption(&current, option.Key, option.Value)
		if err != nil {
			return err
		}
		appliedAny = appliedAny || applied
	}
	if !appliedAny {
		return nil
	}
	return operation_setting.ValidateQiniuKeySettingForEnable(&current)
}

func prepareQiniuKeyOptionsForPersistence(options []model.Option, now int64) []model.Option {
	current := *operation_setting.GetQiniuKeySetting()
	next := current
	appliedAny := false
	for _, option := range options {
		applied, err := applyQiniuKeyOption(&next, option.Key, option.Value)
		if err != nil {
			return options
		}
		appliedAny = appliedAny || applied
	}
	if !appliedAny || !shouldSetInitialQiniuChildAccountBindingCutover(current, next) {
		return options
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	return upsertOptionValue(options, "qiniu_key_setting.child_account_binding_cutover_time", strconv.FormatInt(now, 10))
}

func shouldSetInitialQiniuChildAccountBindingCutover(current operation_setting.QiniuKeySetting, next operation_setting.QiniuKeySetting) bool {
	return current.ChildAccountBindingCutoverTime == 0 &&
		next.ChildAccountBindingCutoverTime == 0 &&
		next.ChildAccountBindingEnabled &&
		next.ChildAccountAssignmentMode == operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild
}

func upsertOptionValue(options []model.Option, key string, value string) []model.Option {
	prepared := make([]model.Option, len(options))
	copy(prepared, options)
	for idx := range prepared {
		if prepared[idx].Key == key {
			prepared[idx].Value = value
			return prepared
		}
	}
	return append(prepared, model.Option{Key: key, Value: value})
}

func isWechatPaySettingCompleteForEnable() bool {
	return strings.TrimSpace(setting.WechatPayAppId) != "" &&
		strings.TrimSpace(setting.WechatPayMchId) != "" &&
		strings.TrimSpace(setting.WechatPayMerchantSerialNo) != "" &&
		strings.TrimSpace(setting.WechatPayMerchantPrivateKey) != "" &&
		len([]byte(strings.TrimSpace(setting.WechatPayAPIv3Key))) == 32 &&
		strings.TrimSpace(setting.WechatPayPlatformPublicKey) != ""
}

type wechatPayOptionSnapshot struct {
	Enabled            bool
	AppID              string
	MchID              string
	MerchantSerialNo   string
	MerchantPrivateKey string
	APIv3Key           string
	PlatformPublicKey  string
}

func currentWechatPayOptionSnapshot() wechatPayOptionSnapshot {
	return wechatPayOptionSnapshot{
		Enabled:            setting.WechatPayEnabled,
		AppID:              setting.WechatPayAppId,
		MchID:              setting.WechatPayMchId,
		MerchantSerialNo:   setting.WechatPayMerchantSerialNo,
		MerchantPrivateKey: setting.WechatPayMerchantPrivateKey,
		APIv3Key:           setting.WechatPayAPIv3Key,
		PlatformPublicKey:  setting.WechatPayPlatformPublicKey,
	}
}

func (snapshot *wechatPayOptionSnapshot) apply(option model.Option) bool {
	if snapshot == nil {
		return false
	}
	switch option.Key {
	case "WechatPayEnabled":
		snapshot.Enabled = option.Value == "true"
	case "WechatPayAppId":
		snapshot.AppID = option.Value
	case "WechatPayMchId":
		snapshot.MchID = option.Value
	case "WechatPayMerchantSerialNo":
		snapshot.MerchantSerialNo = option.Value
	case "WechatPayMerchantPrivateKey":
		snapshot.MerchantPrivateKey = option.Value
	case "WechatPayAPIv3Key":
		snapshot.APIv3Key = option.Value
	case "WechatPayPlatformPublicKey":
		snapshot.PlatformPublicKey = option.Value
	default:
		return false
	}
	return true
}

func (snapshot wechatPayOptionSnapshot) completeForEnable() bool {
	return strings.TrimSpace(snapshot.AppID) != "" &&
		strings.TrimSpace(snapshot.MchID) != "" &&
		strings.TrimSpace(snapshot.MerchantSerialNo) != "" &&
		strings.TrimSpace(snapshot.MerchantPrivateKey) != "" &&
		len([]byte(strings.TrimSpace(snapshot.APIv3Key))) == 32 &&
		strings.TrimSpace(snapshot.PlatformPublicKey) != ""
}

func isPositiveOptionValue(value string) bool {
	intValue, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		return intValue > 0
	}
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && floatValue > 0
}

func isVisiblePublicKeyOption(key string) bool {
	switch key {
	case "WaffoPancakeWebhookPublicKey",
		"WaffoPancakeWebhookTestKey",
		"AlipayPublicKey",
		"WechatPayPlatformPublicKey",
		"piggy_withdraw_setting.app_key":
		return true
	default:
		return false
	}
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return
	}

	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]ratio_setting.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = ratio_setting.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := common.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetOptions(c *gin.Context) {
	var options []*model.Option
	optionValues := make(map[string]string)
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		value := common.Interface2String(v)
		isSensitiveKey := strings.HasSuffix(k, "Token") ||
			strings.HasSuffix(k, "Secret") ||
			strings.HasSuffix(k, "Key") ||
			strings.HasSuffix(k, "secret") ||
			strings.HasSuffix(k, "api_key") ||
			strings.HasSuffix(k, "access_key") ||
			strings.HasSuffix(k, "secret_key") ||
			strings.HasSuffix(k, "_key")
		if isSensitiveKey && !isVisiblePublicKeyOption(k) {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == k {
				optionValues[k] = value
				break
			}
		}
	}
	common.OptionMapRWMutex.Unlock()
	options = append(options, &model.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}

type OptionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type OptionBatchUpdateRequest struct {
	Options []OptionUpdateRequest `json:"options"`
}

func normalizeOptionValue(value any) string {
	switch typedValue := value.(type) {
	case bool:
		return common.Interface2String(typedValue)
	case float64:
		return common.Interface2String(typedValue)
	case int:
		return common.Interface2String(typedValue)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func normalizeOptionUpdateRequest(option OptionUpdateRequest) model.Option {
	return model.Option{
		Key:   strings.TrimSpace(option.Key),
		Value: normalizeOptionValue(option.Value),
	}
}

func normalizeOptionUpdateRequests(options []OptionUpdateRequest) []model.Option {
	updates := make([]model.Option, 0, len(options))
	for _, option := range options {
		updates = append(updates, normalizeOptionUpdateRequest(option))
	}
	return updates
}

func validateOptionUpdate(option model.Option, validatePiggy bool, validateQiniu bool) error {
	switch option.Key {
	case "QuotaForInviter", "QuotaForInvitee":
		if isPositiveOptionValue(option.Value) && !operation_setting.IsPaymentComplianceConfirmed() {
			return fmt.Errorf("请先确认支付合规声明")
		}
	default:
		if isPaymentComplianceOptionKey(option.Key) {
			return fmt.Errorf("合规确认字段不允许通过通用设置接口修改")
		}
		if err := validateVipActivationOption(option.Key, option.Value); err != nil {
			return err
		}
		if err := validateAlipayOption(option.Key, option.Value); err != nil {
			return err
		}
		if err := validateWechatPayOption(option.Key, option.Value); err != nil {
			return err
		}
		if validatePiggy {
			if err := validatePiggyWithdrawOption(option.Key, option.Value); err != nil {
				return err
			}
		}
		if validateQiniu {
			if err := validateQiniuKeyOption(option.Key, option.Value); err != nil {
				return err
			}
		}
	}
	switch option.Key {
	case "GitHubOAuthEnabled":
		if option.Value == "true" && common.GitHubClientId == "" {
			return fmt.Errorf("无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！")
		}
	case "discord.enabled":
		if option.Value == "true" && system_setting.GetDiscordSettings().ClientId == "" {
			return fmt.Errorf("无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！")
		}
	case "oidc.enabled":
		if option.Value == "true" && system_setting.GetOIDCSettings().ClientId == "" {
			return fmt.Errorf("无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！")
		}
	case "LinuxDOOAuthEnabled":
		if option.Value == "true" && common.LinuxDOClientId == "" {
			return fmt.Errorf("无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！")
		}
	case "EmailDomainRestrictionEnabled":
		if option.Value == "true" && len(common.EmailDomainWhitelist) == 0 {
			return fmt.Errorf("无法启用邮箱域名限制，请先填入限制的邮箱域名！")
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			return fmt.Errorf("无法启用微信登录，请先填入微信登录相关配置信息！")
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			return fmt.Errorf("无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！")
		}
	case "TelegramOAuthEnabled":
		if option.Value == "true" && common.TelegramBotToken == "" {
			return fmt.Errorf("无法启用 Telegram OAuth，请先填入 Telegram Bot Token！")
		}
	case "theme.frontend":
		if option.Value != "default" && option.Value != "classic" {
			return fmt.Errorf("无效的主题值，可选值：default（新版前端）、classic（经典前端）")
		}
	case "GroupRatio":
		if err := ratio_setting.CheckGroupRatio(option.Value); err != nil {
			return err
		}
	case "ImageRatio":
		if err := ratio_setting.UpdateImageRatioByJSONString(option.Value); err != nil {
			return fmt.Errorf("图片倍率设置失败: %w", err)
		}
	case "AudioRatio":
		if err := ratio_setting.UpdateAudioRatioByJSONString(option.Value); err != nil {
			return fmt.Errorf("音频倍率设置失败: %w", err)
		}
	case "AudioCompletionRatio":
		if err := ratio_setting.UpdateAudioCompletionRatioByJSONString(option.Value); err != nil {
			return fmt.Errorf("音频补全倍率设置失败: %w", err)
		}
	case "CreateCacheRatio":
		if err := ratio_setting.UpdateCreateCacheRatioByJSONString(option.Value); err != nil {
			return fmt.Errorf("缓存创建倍率设置失败: %w", err)
		}
	case "ModelRequestRateLimitGroup":
		if err := setting.CheckModelRequestRateLimitGroup(option.Value); err != nil {
			return err
		}
	case "AutomaticDisableStatusCodes":
		if _, err := operation_setting.ParseHTTPStatusCodeRanges(option.Value); err != nil {
			return err
		}
	case "AutomaticRetryStatusCodes":
		if _, err := operation_setting.ParseHTTPStatusCodeRanges(option.Value); err != nil {
			return err
		}
	case "console_setting.api_info":
		if err := console_setting.ValidateConsoleSettings(option.Value, "ApiInfo"); err != nil {
			return err
		}
	case "console_setting.announcements":
		if err := console_setting.ValidateConsoleSettings(option.Value, "Announcements"); err != nil {
			return err
		}
	case "console_setting.faq":
		if err := console_setting.ValidateConsoleSettings(option.Value, "FAQ"); err != nil {
			return err
		}
	case "console_setting.uptime_kuma_groups":
		if err := console_setting.ValidateConsoleSettings(option.Value, "UptimeKumaGroups"); err != nil {
			return err
		}
	}
	return nil
}

func validateBatchOptionUpdates(options []model.Option) error {
	if len(options) == 0 {
		return fmt.Errorf("设置项不能为空")
	}
	seen := make(map[string]struct{}, len(options))
	piggyOptions := make([]model.Option, 0)
	qiniuOptions := make([]model.Option, 0)
	wechatSnapshot := currentWechatPayOptionSnapshot()
	hasWechatSnapshotUpdate := false
	for _, option := range options {
		if strings.TrimSpace(option.Key) == "" {
			return fmt.Errorf("设置项 key 不能为空")
		}
		if _, ok := seen[option.Key]; ok {
			return fmt.Errorf("设置项 key 重复: %s", option.Key)
		}
		seen[option.Key] = struct{}{}
		isPiggyOption := strings.HasPrefix(option.Key, "piggy_withdraw_setting.")
		if isPiggyOption {
			piggyOptions = append(piggyOptions, option)
		}
		isQiniuOption := strings.HasPrefix(option.Key, "qiniu_key_setting.")
		if isQiniuOption {
			qiniuOptions = append(qiniuOptions, option)
		}
		if wechatSnapshot.apply(option) {
			hasWechatSnapshotUpdate = true
		}
		if option.Key == "WechatPayEnabled" {
			continue
		}
		if err := validateOptionUpdate(option, !isPiggyOption, !isQiniuOption); err != nil {
			return err
		}
	}
	if err := validatePiggyWithdrawOptions(piggyOptions); err != nil {
		return err
	}
	if err := validateQiniuKeyOptions(qiniuOptions); err != nil {
		return err
	}
	if hasWechatSnapshotUpdate && wechatSnapshot.Enabled && !wechatSnapshot.completeForEnable() {
		return fmt.Errorf("微信支付直连未完整配置，不能启用")
	}
	return nil
}

func UpdateOption(c *gin.Context) {
	var request OptionUpdateRequest
	err := common.DecodeJson(c.Request.Body, &request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	option := normalizeOptionUpdateRequest(request)
	if err := validateOptionUpdate(option, true, true); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	options := prepareQiniuKeyOptionsForPersistence([]model.Option{option}, common.GetTimestamp())
	err = model.UpdateOptions(options)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateOptions(c *gin.Context) {
	var request OptionBatchUpdateRequest
	err := common.DecodeJson(c.Request.Body, &request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	options := normalizeOptionUpdateRequests(request.Options)
	if err := validateBatchOptionUpdates(options); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	options = prepareQiniuKeyOptionsForPersistence(options, common.GetTimestamp())
	err = model.UpdateOptions(options)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
