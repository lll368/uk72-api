package operation_setting

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	QiniuKeyDefaultBaseURL                  = "https://api.qnaigc.com"
	QiniuChildAccountDefaultBaseURL         = "https://api.qiniu.com"
	QiniuMarketDefaultBaseURL               = "https://openai.qiniu.com"
	QiniuKeyDefaultRequestTimeout           = 15
	QiniuKeyDefaultRetryInterval            = 300
	QiniuMarketCatalogDefaultTTLSeconds     = 3600
	QiniuOfficialLedgerDefaultSyncInterval  = 60
	QiniuOfficialLedgerDefaultWindowHours   = 6
	QiniuOfficialLedgerDefaultWindowDays    = 2
	QiniuOfficialLedgerDefaultBatchSize     = 100
	QiniuOfficialLedgerDefaultRateLimit     = 4
	QiniuOfficialLedgerDefaultRetryInterval = 300
	QiniuCostDetailDefaultLookbackDays      = 7
	QiniuCostDetailMaxLookbackDays          = 30
	QiniuChildAccountDefaultEmailDomain     = "uk72.cn"
	QiniuChildAccountDefaultEmailPrefix     = "child"
	QiniuChildAccountDefaultPasswordLength  = 18
	QiniuChildAccountDefaultRequestTimeout  = 15
	QiniuChildAccountDefaultRetryInterval   = 300
)

const (
	QiniuChildAccountAssignmentModeParentOnly     = "parent_only"
	QiniuChildAccountAssignmentModeOneKeyOneChild = "one_key_one_child"
)

// QiniuKeySetting 保存七牛 AI Token Key 生命周期配置。
type QiniuKeySetting struct {
	Enabled                            bool   `json:"enabled"`
	BaseURL                            string `json:"base_url"`
	AccessKey                          string `json:"access_key"`
	SecretKey                          string `json:"secret_key"`
	RequestTimeout                     int    `json:"request_timeout"`
	RetryIntervalSeconds               int    `json:"retry_interval_seconds"`
	OfficialLedgerEnabled              bool   `json:"official_ledger_enabled"`
	OfficialLedgerCutoverTime          int64  `json:"official_ledger_cutover_time"`
	OfficialLedgerSyncIntervalSeconds  int    `json:"official_ledger_sync_interval_seconds"`
	OfficialLedgerWindowHours          int    `json:"official_ledger_window_hours"`
	OfficialLedgerWindowDays           int    `json:"official_ledger_window_days"`
	OfficialLedgerBatchSize            int    `json:"official_ledger_batch_size"`
	OfficialLedgerRateLimitPerSecond   int    `json:"official_ledger_rate_limit_per_second"`
	OfficialLedgerRetryIntervalSeconds int    `json:"official_ledger_retry_interval_seconds"`
	CostDetailCutoverTime              int64  `json:"cost_detail_cutover_time"`
	CostDetailLookbackDays             int    `json:"cost_detail_lookback_days"`
	CostDetailAutoApplyEnabled         bool   `json:"cost_detail_auto_apply_enabled"`
	MarketCatalogEnabled               bool   `json:"market_catalog_enabled"`
	MarketCatalogBaseURL               string `json:"market_catalog_base_url"`
	MarketCatalogTTLSeconds            int    `json:"market_catalog_ttl_seconds"`
	MarketCatalogOverseas              bool   `json:"market_catalog_overseas"`
	MarketCatalogFallbackEnabled       bool   `json:"market_catalog_fallback_enabled"`
	ChildAccountBaseURL                string `json:"child_account_base_url"`
	ChildAccountEmailDomain            string `json:"child_account_email_domain"`
	ChildAccountEmailPrefix            string `json:"child_account_email_prefix"`
	ChildAccountPasswordLength         int    `json:"child_account_password_length"`
	ChildAccountRequestTimeout         int    `json:"child_account_request_timeout"`
	ChildAccountRetryIntervalSeconds   int    `json:"child_account_retry_interval_seconds"`
	ChildAccountBindingEnabled         bool   `json:"child_account_binding_enabled"`
	ChildAccountAssignmentMode         string `json:"child_account_assignment_mode"`
	ChildAccountBindingCutoverTime     int64  `json:"child_account_binding_cutover_time"`
}

var qiniuKeySetting = QiniuKeySetting{
	Enabled:                            false,
	BaseURL:                            QiniuKeyDefaultBaseURL,
	RequestTimeout:                     QiniuKeyDefaultRequestTimeout,
	RetryIntervalSeconds:               QiniuKeyDefaultRetryInterval,
	OfficialLedgerEnabled:              false,
	OfficialLedgerSyncIntervalSeconds:  QiniuOfficialLedgerDefaultSyncInterval,
	OfficialLedgerWindowHours:          QiniuOfficialLedgerDefaultWindowHours,
	OfficialLedgerWindowDays:           QiniuOfficialLedgerDefaultWindowDays,
	OfficialLedgerBatchSize:            QiniuOfficialLedgerDefaultBatchSize,
	OfficialLedgerRateLimitPerSecond:   QiniuOfficialLedgerDefaultRateLimit,
	OfficialLedgerRetryIntervalSeconds: QiniuOfficialLedgerDefaultRetryInterval,
	CostDetailAutoApplyEnabled:         true,
	CostDetailLookbackDays:             QiniuCostDetailDefaultLookbackDays,
	MarketCatalogBaseURL:               QiniuMarketDefaultBaseURL,
	MarketCatalogTTLSeconds:            QiniuMarketCatalogDefaultTTLSeconds,
	MarketCatalogOverseas:              true,
	MarketCatalogFallbackEnabled:       true,
	ChildAccountBaseURL:                QiniuChildAccountDefaultBaseURL,
	ChildAccountEmailDomain:            QiniuChildAccountDefaultEmailDomain,
	ChildAccountEmailPrefix:            QiniuChildAccountDefaultEmailPrefix,
	ChildAccountPasswordLength:         QiniuChildAccountDefaultPasswordLength,
	ChildAccountRequestTimeout:         QiniuChildAccountDefaultRequestTimeout,
	ChildAccountRetryIntervalSeconds:   QiniuChildAccountDefaultRetryInterval,
	ChildAccountBindingEnabled:         false,
	ChildAccountAssignmentMode:         QiniuChildAccountAssignmentModeParentOnly,
	ChildAccountBindingCutoverTime:     0,
}

var qiniuChildAccountDomainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
var qiniuChildAccountPrefixPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func init() {
	config.GlobalConfig.Register("qiniu_key_setting", &qiniuKeySetting)
}

func GetQiniuKeySetting() *QiniuKeySetting {
	normalizeQiniuKeySetting(&qiniuKeySetting)
	return &qiniuKeySetting
}

func normalizeQiniuKeySetting(setting *QiniuKeySetting) {
	if setting == nil {
		return
	}
	setting.BaseURL = strings.TrimRight(strings.TrimSpace(setting.BaseURL), "/")
	if setting.BaseURL == "" {
		setting.BaseURL = QiniuKeyDefaultBaseURL
	}
	setting.AccessKey = strings.TrimSpace(setting.AccessKey)
	setting.SecretKey = strings.TrimSpace(setting.SecretKey)
	if setting.RequestTimeout <= 0 {
		setting.RequestTimeout = QiniuKeyDefaultRequestTimeout
	}
	if setting.RetryIntervalSeconds <= 0 {
		setting.RetryIntervalSeconds = QiniuKeyDefaultRetryInterval
	}
	setting.MarketCatalogBaseURL = strings.TrimRight(strings.TrimSpace(setting.MarketCatalogBaseURL), "/")
	if setting.MarketCatalogBaseURL == "" {
		setting.MarketCatalogBaseURL = QiniuMarketDefaultBaseURL
	}
	if setting.MarketCatalogTTLSeconds <= 0 {
		setting.MarketCatalogTTLSeconds = QiniuMarketCatalogDefaultTTLSeconds
	}
	setting.ChildAccountBaseURL = strings.TrimRight(strings.TrimSpace(setting.ChildAccountBaseURL), "/")
	if setting.ChildAccountBaseURL == "" {
		setting.ChildAccountBaseURL = QiniuChildAccountDefaultBaseURL
	}
	if setting.OfficialLedgerSyncIntervalSeconds <= 0 {
		setting.OfficialLedgerSyncIntervalSeconds = QiniuOfficialLedgerDefaultSyncInterval
	}
	if setting.OfficialLedgerWindowHours <= 0 {
		setting.OfficialLedgerWindowHours = QiniuOfficialLedgerDefaultWindowHours
	}
	if setting.OfficialLedgerWindowDays <= 0 {
		setting.OfficialLedgerWindowDays = QiniuOfficialLedgerDefaultWindowDays
	}
	if setting.OfficialLedgerBatchSize <= 0 {
		setting.OfficialLedgerBatchSize = QiniuOfficialLedgerDefaultBatchSize
	}
	if setting.OfficialLedgerRateLimitPerSecond <= 0 {
		setting.OfficialLedgerRateLimitPerSecond = QiniuOfficialLedgerDefaultRateLimit
	}
	if setting.OfficialLedgerRetryIntervalSeconds <= 0 {
		setting.OfficialLedgerRetryIntervalSeconds = QiniuOfficialLedgerDefaultRetryInterval
	}
	if setting.CostDetailLookbackDays <= 0 {
		setting.CostDetailLookbackDays = QiniuCostDetailDefaultLookbackDays
	} else if setting.CostDetailLookbackDays > QiniuCostDetailMaxLookbackDays {
		setting.CostDetailLookbackDays = QiniuCostDetailMaxLookbackDays
	}
	setting.ChildAccountEmailDomain = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(setting.ChildAccountEmailDomain)), "@")
	if setting.ChildAccountEmailDomain == "" {
		setting.ChildAccountEmailDomain = QiniuChildAccountDefaultEmailDomain
	}
	setting.ChildAccountEmailPrefix = strings.TrimSpace(setting.ChildAccountEmailPrefix)
	if setting.ChildAccountEmailPrefix == "" {
		setting.ChildAccountEmailPrefix = QiniuChildAccountDefaultEmailPrefix
	}
	if setting.ChildAccountPasswordLength <= 0 {
		setting.ChildAccountPasswordLength = QiniuChildAccountDefaultPasswordLength
	}
	if setting.ChildAccountRequestTimeout <= 0 {
		setting.ChildAccountRequestTimeout = QiniuChildAccountDefaultRequestTimeout
	}
	if setting.ChildAccountRetryIntervalSeconds <= 0 {
		setting.ChildAccountRetryIntervalSeconds = QiniuChildAccountDefaultRetryInterval
	}
	setting.ChildAccountAssignmentMode = strings.TrimSpace(strings.ToLower(setting.ChildAccountAssignmentMode))
	if setting.ChildAccountAssignmentMode == "" {
		setting.ChildAccountAssignmentMode = QiniuChildAccountAssignmentModeParentOnly
	}
}

// ValidateQiniuKeySettingForEnable 校验启用七牛 Key 托管前必须具备的配置。
func ValidateQiniuKeySettingForEnable(next *QiniuKeySetting) error {
	if next == nil {
		return fmt.Errorf("七牛 Key 配置不能为空")
	}
	normalizeQiniuKeySetting(next)
	if !next.Enabled && !next.OfficialLedgerEnabled && !next.MarketCatalogEnabled {
		return nil
	}
	if next.Enabled || next.OfficialLedgerEnabled {
		if strings.TrimSpace(next.AccessKey) == "" {
			return fmt.Errorf("七牛 Key 配置缺少 access_key，不能启用")
		}
		if strings.TrimSpace(next.SecretKey) == "" {
			return fmt.Errorf("七牛 Key 配置缺少 secret_key，不能启用")
		}
		if err := validateQiniuHTTPURL(next.BaseURL, "七牛 Key 接口域名"); err != nil {
			return err
		}
	}
	if next.MarketCatalogEnabled {
		if err := validateQiniuHTTPURL(next.MarketCatalogBaseURL, "七牛模型市场接口域名"); err != nil {
			return err
		}
	}
	if next.ChildAccountBindingEnabled {
		if err := validateQiniuHTTPURL(next.ChildAccountBaseURL, "七牛子账户接口域名"); err != nil {
			return err
		}
	}
	if !IsValidQiniuChildAccountAssignmentMode(next.ChildAccountAssignmentMode) {
		return fmt.Errorf("七牛子账号分配模式无效")
	}
	return nil
}

func IsValidQiniuChildAccountAssignmentMode(mode string) bool {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case QiniuChildAccountAssignmentModeParentOnly, QiniuChildAccountAssignmentModeOneKeyOneChild:
		return true
	default:
		return false
	}
}

func IsQiniuChildAccountBindingEligible(setting *QiniuKeySetting, userCreatedAt int64) bool {
	if setting == nil {
		return false
	}
	normalizeQiniuKeySetting(setting)
	return setting.ChildAccountBindingEnabled &&
		setting.ChildAccountAssignmentMode == QiniuChildAccountAssignmentModeOneKeyOneChild &&
		setting.ChildAccountBindingCutoverTime > 0 &&
		userCreatedAt > setting.ChildAccountBindingCutoverTime
}

func ValidateQiniuChildAccountSettingForCreate(next *QiniuKeySetting) error {
	if next == nil {
		return fmt.Errorf("七牛子账户配置不能为空")
	}
	normalizeQiniuKeySetting(next)
	if strings.TrimSpace(next.AccessKey) == "" {
		return fmt.Errorf("七牛 Key 配置缺少 access_key，不能创建子账户")
	}
	if strings.TrimSpace(next.SecretKey) == "" {
		return fmt.Errorf("七牛 Key 配置缺少 secret_key，不能创建子账户")
	}
	if err := validateQiniuHTTPURL(next.ChildAccountBaseURL, "七牛子账户接口域名"); err != nil {
		return err
	}
	if err := ValidateQiniuChildAccountEmailDomain(next.ChildAccountEmailDomain); err != nil {
		return err
	}
	if err := ValidateQiniuChildAccountEmailPrefix(next.ChildAccountEmailPrefix); err != nil {
		return err
	}
	if next.ChildAccountPasswordLength < 12 || next.ChildAccountPasswordLength > 64 {
		return fmt.Errorf("七牛子账户密码长度必须在 12 到 64 之间")
	}
	return nil
}

func ValidateQiniuChildAccountEmailDomain(value string) error {
	domain := strings.TrimPrefix(strings.TrimSpace(strings.ToLower(value)), "@")
	if domain == "" {
		return fmt.Errorf("七牛子账户邮箱域名不能为空")
	}
	if strings.Contains(domain, "://") || !qiniuChildAccountDomainPattern.MatchString(domain) {
		return fmt.Errorf("七牛子账户邮箱域名无效")
	}
	return nil
}

func ValidateQiniuChildAccountEmailPrefix(value string) error {
	prefix := strings.TrimSpace(value)
	if prefix == "" {
		return fmt.Errorf("七牛子账户邮箱前缀不能为空")
	}
	if !qiniuChildAccountPrefixPattern.MatchString(prefix) {
		return fmt.Errorf("七牛子账户邮箱前缀只能包含字母、数字、点、下划线和短横线")
	}
	return nil
}

func validateQiniuHTTPURL(value string, label string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s 必须以 http:// 或 https:// 开头", label)
	}
	return nil
}
