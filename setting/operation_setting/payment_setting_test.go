package operation_setting

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaymentSettingVipActivationDefaults(t *testing.T) {
	assert.InDelta(t, 1680.0, paymentSetting.VipActivationPrice, 0.000001)
	assert.InDelta(t, 1000.0, paymentSetting.VipActivationCommissionLevel1Amount, 0.000001)
	assert.InDelta(t, 400.0, paymentSetting.VipActivationCommissionLevel2Amount, 0.000001)
}

func TestValidateVipActivationCommissionAmounts(t *testing.T) {
	assert.NoError(t, ValidateVipActivationCommissionAmounts(1680, 1000, 400))
	assert.Error(t, ValidateVipActivationCommissionAmounts(math.NaN(), 1000, 400))
	assert.Error(t, ValidateVipActivationCommissionAmounts(1680, -1, 400))
	assert.Error(t, ValidateVipActivationCommissionAmounts(1680, 1000, -1))
	assert.Error(t, ValidateVipActivationCommissionAmounts(1680, 1000, 800))
}
