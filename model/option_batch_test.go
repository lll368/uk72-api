package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateOptionsSavesBatchAndUpdatesRuntimeConfig(t *testing.T) {
	originalDB := DB
	originalOptionMap := common.OptionMap
	piggySetting := operation_setting.GetPiggyWithdrawSetting()
	originalPiggySetting := *piggySetting
	t.Cleanup(func() {
		DB = originalDB
		common.OptionMap = originalOptionMap
		*piggySetting = originalPiggySetting
	})

	db, err := gorm.Open(sqlite.Open("file:option_batch?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))

	DB = db
	common.OptionMap = map[string]string{}
	*piggySetting = operation_setting.PiggyWithdrawSetting{}

	err = UpdateOptions([]Option{
		{Key: "piggy_withdraw_setting.domain", Value: "https://piggy.example.com"},
		{Key: "piggy_withdraw_setting.app_key", Value: "app-key"},
		{Key: "piggy_withdraw_setting.request_timeout", Value: "10"},
		{Key: "piggy_withdraw_setting.platform_fee_rate", Value: "8.1256"},
	})

	require.NoError(t, err)

	var savedOptions []Option
	require.NoError(t, DB.Order("key").Find(&savedOptions).Error)
	require.Len(t, savedOptions, 4)
	require.Equal(t, "https://piggy.example.com", common.OptionMap["piggy_withdraw_setting.domain"])
	require.Equal(t, "app-key", operation_setting.GetPiggyWithdrawSetting().AppKey)
	require.Equal(t, 10, operation_setting.GetPiggyWithdrawSetting().RequestTimeout)
	require.Equal(t, 8.1256, operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate)
}
