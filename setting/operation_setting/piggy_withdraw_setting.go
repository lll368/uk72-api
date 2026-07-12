package operation_setting

import (
	"fmt"
	"math"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/shopspring/decimal"
)

const (
	PiggyWithdrawDefaultDomain          = "https://saas.xzsz.ltd"
	PiggyWithdrawDefaultAESIV           = "0000000000000000"
	PiggyWithdrawDefaultRequestTimeout  = 15
	PiggyWithdrawDefaultCallbackLockTTL = 300
	PiggyWithdrawDefaultCooldownMinutes = 30
	PiggyWithdrawDefaultCalcType        = "C"
	PiggyWithdrawDefaultPlatformFeeRate = 8
	PiggyWithdrawPlatformFeeRateScale   = 4
)

// PiggyWithdrawSetting 保存小猪连续劳务 V3 银行卡提现配置。
type PiggyWithdrawSetting struct {
	Enabled               bool    `json:"enabled"`
	Domain                string  `json:"domain"`
	AppKey                string  `json:"app_key"`
	AppSecret             string  `json:"app_secret"`
	AESIV                 string  `json:"aes_iv"`
	TaxFundId             string  `json:"tax_fund_id"`
	PositionName          string  `json:"position_name"`
	Position              string  `json:"position"`
	SignJumpPage          string  `json:"sign_jump_page"`
	SignNotifyUrl         string  `json:"sign_notify_url"`
	PayNotifyUrl          string  `json:"pay_notify_url"`
	RequestTimeout        int     `json:"request_timeout"`
	CallbackLockTTL       int     `json:"callback_lock_ttl"`
	CooldownMinutes       int     `json:"cooldown_minutes"`
	ForbiddenWithdrawTime string  `json:"forbidden_withdraw_time"`
	CalcType              string  `json:"calc_type"`
	PlatformFeeRate       float64 `json:"platform_fee_rate"`
	BankRemark            string  `json:"bank_remark"`
}

var piggyWithdrawSetting = PiggyWithdrawSetting{
	Enabled:               false,
	Domain:                PiggyWithdrawDefaultDomain,
	AESIV:                 PiggyWithdrawDefaultAESIV,
	RequestTimeout:        PiggyWithdrawDefaultRequestTimeout,
	CallbackLockTTL:       PiggyWithdrawDefaultCallbackLockTTL,
	CooldownMinutes:       PiggyWithdrawDefaultCooldownMinutes,
	ForbiddenWithdrawTime: "",
	CalcType:              PiggyWithdrawDefaultCalcType,
	PlatformFeeRate:       PiggyWithdrawDefaultPlatformFeeRate,
}

func init() {
	config.GlobalConfig.Register("piggy_withdraw_setting", &piggyWithdrawSetting)
}

func GetPiggyWithdrawSetting() *PiggyWithdrawSetting {
	normalizePiggyWithdrawSetting(&piggyWithdrawSetting)
	return &piggyWithdrawSetting
}

func normalizePiggyWithdrawSetting(setting *PiggyWithdrawSetting) {
	if setting == nil {
		return
	}
	setting.Domain = strings.TrimRight(strings.TrimSpace(setting.Domain), "/")
	if setting.Domain == "" {
		setting.Domain = PiggyWithdrawDefaultDomain
	}
	if setting.RequestTimeout <= 0 {
		setting.RequestTimeout = PiggyWithdrawDefaultRequestTimeout
	}
	if setting.CallbackLockTTL <= 0 {
		setting.CallbackLockTTL = PiggyWithdrawDefaultCallbackLockTTL
	}
	if setting.CooldownMinutes < 0 {
		setting.CooldownMinutes = 0
	}
	setting.CalcType = strings.ToUpper(strings.TrimSpace(setting.CalcType))
	if setting.CalcType == "" {
		setting.CalcType = PiggyWithdrawDefaultCalcType
	}
}

// ValidatePiggyWithdrawSettingForEnable 校验启用小猪提现前必须具备的配置。
func ValidatePiggyWithdrawSettingForEnable(next *PiggyWithdrawSetting) error {
	if next == nil {
		return fmt.Errorf("小猪提现配置不能为空")
	}
	normalizePiggyWithdrawSetting(next)
	if err := ValidatePiggyWithdrawPlatformFeeRate(next.PlatformFeeRate); err != nil {
		return err
	}
	if !next.Enabled {
		return nil
	}
	required := map[string]string{
		"domain":          next.Domain,
		"app_key":         next.AppKey,
		"app_secret":      next.AppSecret,
		"aes_iv":          next.AESIV,
		"tax_fund_id":     next.TaxFundId,
		"position_name":   next.PositionName,
		"position":        next.Position,
		"sign_notify_url": next.SignNotifyUrl,
		"pay_notify_url":  next.PayNotifyUrl,
		"calc_type":       next.CalcType,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("小猪提现配置缺少 %s，不能启用", key)
		}
	}
	if len([]byte(next.AppSecret)) != 16 && len([]byte(next.AppSecret)) != 24 && len([]byte(next.AppSecret)) != 32 {
		return fmt.Errorf("小猪 appSecret 必须是 16、24 或 32 字节，用于 AES 密钥")
	}
	if len([]byte(next.AESIV)) != 16 {
		return fmt.Errorf("小猪 AES IV 必须是 16 字节")
	}
	if next.CalcType != "C" && next.CalcType != "E" {
		return fmt.Errorf("小猪 calcType 仅支持 C 或 E")
	}
	if err := validatePiggyHTTPURL(next.Domain, "小猪接口域名"); err != nil {
		return err
	}
	if err := validatePiggyHTTPURL(next.SignNotifyUrl, "小猪签约回调地址"); err != nil {
		return err
	}
	if err := validatePiggyHTTPURLWithoutQuery(next.PayNotifyUrl, "小猪支付回调地址"); err != nil {
		return err
	}
	if strings.TrimSpace(next.SignJumpPage) != "" {
		if err := validatePiggyHTTPURL(next.SignJumpPage, "小猪签约跳转地址"); err != nil {
			return err
		}
	}
	return nil
}

// ValidatePiggyWithdrawPlatformFeeRate 校验小猪提现平台服务费率，禁用时也必须保证配置值合法。
func ValidatePiggyWithdrawPlatformFeeRate(rate float64) error {
	if math.IsNaN(rate) || math.IsInf(rate, 0) {
		return fmt.Errorf("小猪平台服务费率必须是数字")
	}
	if rate < 0 || rate >= 100 {
		return fmt.Errorf("小猪平台服务费率必须大于等于 0 且小于 100")
	}
	if !IsPiggyWithdrawPlatformFeeRateSupportedPrecision(rate) {
		return fmt.Errorf("小猪平台服务费率最多支持 %d 位小数", PiggyWithdrawPlatformFeeRateScale)
	}
	return nil
}

// IsPiggyWithdrawPlatformFeeRateSupportedPrecision 判断平台服务费率是否在支持的小数精度内。
func IsPiggyWithdrawPlatformFeeRateSupportedPrecision(rate float64) bool {
	value := decimal.NewFromFloat(rate)
	return value.Equal(value.Round(PiggyWithdrawPlatformFeeRateScale))
}

func validatePiggyHTTPURL(value string, label string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s 必须以 http:// 或 https:// 开头", label)
	}
	return nil
}

func validatePiggyHTTPURLWithoutQuery(value string, label string) error {
	if err := validatePiggyHTTPURL(value, label); err != nil {
		return err
	}
	parsed, _ := url.ParseRequestURI(strings.TrimSpace(value))
	if parsed.ForceQuery || strings.TrimSpace(parsed.RawQuery) != "" {
		return fmt.Errorf("%s 不能包含查询参数", label)
	}
	return nil
}
