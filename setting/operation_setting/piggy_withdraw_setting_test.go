package operation_setting

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPiggyWithdrawSettingPlatformFeeDefaultAndExplicitZero(t *testing.T) {
	original := piggyWithdrawSetting
	t.Cleanup(func() {
		piggyWithdrawSetting = original
	})

	piggyWithdrawSetting = PiggyWithdrawSetting{
		PlatformFeeRate: PiggyWithdrawDefaultPlatformFeeRate,
	}
	assert.Equal(t, float64(8), GetPiggyWithdrawSetting().PlatformFeeRate)

	piggyWithdrawSetting = PiggyWithdrawSetting{
		PlatformFeeRate: 0,
	}
	require.NoError(t, ValidatePiggyWithdrawSettingForEnable(GetPiggyWithdrawSetting()))
	assert.Equal(t, float64(0), GetPiggyWithdrawSetting().PlatformFeeRate)
}

func TestValidatePiggyWithdrawPlatformFeeRate(t *testing.T) {
	for _, rate := range []float64{0, 8, 8.1256, 99.9999} {
		assert.NoError(t, ValidatePiggyWithdrawPlatformFeeRate(rate), rate)
	}

	tests := []struct {
		name string
		rate float64
	}{
		{name: "negative", rate: -0.0001},
		{name: "one hundred", rate: 100},
		{name: "above one hundred", rate: 100.01},
		{name: "nan", rate: math.NaN()},
		{name: "infinite", rate: math.Inf(1)},
		{name: "unsupported precision", rate: 8.12345},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, ValidatePiggyWithdrawPlatformFeeRate(tt.rate))
		})
	}
}
