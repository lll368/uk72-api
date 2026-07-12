package operation_setting

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/shopspring/decimal"
)

type PaymentSetting struct {
	AmountOptions  []int           `json:"amount_options"`
	AmountDiscount map[int]float64 `json:"amount_discount"` // 充值金额对应的折扣，例如 100 元 0.9 表示 100 元充值享受 9 折优惠

	DefaultUserTopupDiscount            float64 `json:"default_user_topup_discount"`
	DefaultVvipTopupDiscount            float64 `json:"default_vvip_topup_discount"`
	TopupCommissionLevel1Rate           float64 `json:"topup_commission_level1_rate"`
	TopupCommissionLevel2Rate           float64 `json:"topup_commission_level2_rate"`
	VipActivationPrice                  float64 `json:"vip_activation_price"`
	VipActivationCommissionLevel1Amount float64 `json:"vip_activation_commission_level1_amount"`
	VipActivationCommissionLevel2Amount float64 `json:"vip_activation_commission_level2_amount"`
	VipActivationCommissionLevel1Rate   float64 `json:"vip_activation_commission_level1_rate"`
	VipActivationCommissionLevel2Rate   float64 `json:"vip_activation_commission_level2_rate"`
	CommissionMinWithdrawAmount         float64 `json:"commission_min_withdraw_amount"`

	ComplianceConfirmed    bool   `json:"compliance_confirmed"`
	ComplianceTermsVersion string `json:"compliance_terms_version"`
	ComplianceConfirmedAt  int64  `json:"compliance_confirmed_at"`
	ComplianceConfirmedBy  int    `json:"compliance_confirmed_by"`
	ComplianceConfirmedIP  string `json:"compliance_confirmed_ip"`
}

const CurrentComplianceTermsVersion = "v1"

// 默认配置
var paymentSetting = PaymentSetting{
	AmountOptions:                       []int{10, 20, 50, 100, 200, 500},
	AmountDiscount:                      map[int]float64{},
	DefaultUserTopupDiscount:            1,
	DefaultVvipTopupDiscount:            1,
	VipActivationPrice:                  1680,
	VipActivationCommissionLevel1Amount: 1000,
	VipActivationCommissionLevel2Amount: 400,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("payment_setting", &paymentSetting)
}

func GetPaymentSetting() *PaymentSetting {
	return &paymentSetting
}

func ValidateVipActivationCommissionRates(level1Rate float64, level2Rate float64) error {
	if !isFiniteVipActivationCommissionRate(level1Rate) || level1Rate < 0 || level1Rate > 1 {
		return fmt.Errorf("VVIP 开通上级分佣比例必须在 0 到 1 之间")
	}
	if !isFiniteVipActivationCommissionRate(level2Rate) || level2Rate < 0 || level2Rate > 1 {
		return fmt.Errorf("VVIP 开通上上级分佣比例必须在 0 到 1 之间")
	}
	if level1Rate+level2Rate > 1 {
		return errors.New("VVIP 开通两级分佣比例合计不能大于 1")
	}
	return nil
}

func isFiniteVipActivationCommissionRate(rate float64) bool {
	return !math.IsNaN(rate) && !math.IsInf(rate, 0)
}

func GetVipActivationPrice() float64 {
	if isFiniteVipActivationMoney(paymentSetting.VipActivationPrice) && paymentSetting.VipActivationPrice > 0 {
		return paymentSetting.VipActivationPrice
	}
	return 1680
}

func NormalizeVipActivationMoneyToCents(amount float64) float64 {
	if !isFiniteVipActivationMoney(amount) {
		return amount
	}
	return decimal.NewFromFloat(amount).Round(2).InexactFloat64()
}

func IsVipActivationMoneyAtCentPrecision(amount float64) bool {
	if !isFiniteVipActivationMoney(amount) {
		return true
	}
	value := decimal.NewFromFloat(amount)
	return value.Equal(value.Round(2))
}

func GetVipActivationPaymentAmount() float64 {
	return NormalizeVipActivationMoneyToCents(GetVipActivationPrice())
}

func ValidateVipActivationCommissionAmounts(activationPrice float64, level1Amount float64, level2Amount float64) error {
	if !isFiniteVipActivationMoney(activationPrice) || activationPrice <= 0 {
		return fmt.Errorf("VVIP 开通费用必须是大于 0 的数字")
	}
	if !isFiniteVipActivationMoney(level1Amount) || level1Amount < 0 {
		return fmt.Errorf("VVIP 开通上级分佣金额必须是大于等于 0 的数字")
	}
	if !isFiniteVipActivationMoney(level2Amount) || level2Amount < 0 {
		return fmt.Errorf("VVIP 开通上上级分佣金额必须是大于等于 0 的数字")
	}
	if level1Amount+level2Amount > activationPrice {
		return errors.New("VVIP 开通两级分佣金额合计不能大于开通费用")
	}
	return nil
}

func isFiniteVipActivationMoney(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func IsPaymentComplianceConfirmed() bool {
	return paymentSetting.ComplianceConfirmed &&
		paymentSetting.ComplianceTermsVersion == CurrentComplianceTermsVersion
}
