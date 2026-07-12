package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateWechatPayOptionsDoesNotMutateEpaySettings(t *testing.T) {
	originalDB := DB
	originalOptionMap := common.OptionMap
	originalWechatPayEnabled := setting.WechatPayEnabled
	originalWechatPaySandbox := setting.WechatPaySandbox
	originalWechatPayAppID := setting.WechatPayAppId
	originalWechatPayMchID := setting.WechatPayMchId
	originalWechatPayMerchantSerialNo := setting.WechatPayMerchantSerialNo
	originalWechatPayMerchantPrivateKey := setting.WechatPayMerchantPrivateKey
	originalWechatPayAPIv3Key := setting.WechatPayAPIv3Key
	originalWechatPayPlatformSerialNo := setting.WechatPayPlatformSerialNo
	originalWechatPayPlatformPublicKey := setting.WechatPayPlatformPublicKey
	originalWechatPayUnitPrice := setting.WechatPayUnitPrice
	originalWechatPayMinTopUp := setting.WechatPayMinTopUp
	originalWechatPayNotifyURL := setting.WechatPayNotifyUrl
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		DB = originalDB
		common.OptionMap = originalOptionMap
		setting.WechatPayEnabled = originalWechatPayEnabled
		setting.WechatPaySandbox = originalWechatPaySandbox
		setting.WechatPayAppId = originalWechatPayAppID
		setting.WechatPayMchId = originalWechatPayMchID
		setting.WechatPayMerchantSerialNo = originalWechatPayMerchantSerialNo
		setting.WechatPayMerchantPrivateKey = originalWechatPayMerchantPrivateKey
		setting.WechatPayAPIv3Key = originalWechatPayAPIv3Key
		setting.WechatPayPlatformSerialNo = originalWechatPayPlatformSerialNo
		setting.WechatPayPlatformPublicKey = originalWechatPayPlatformPublicKey
		setting.WechatPayUnitPrice = originalWechatPayUnitPrice
		setting.WechatPayMinTopUp = originalWechatPayMinTopUp
		setting.WechatPayNotifyUrl = originalWechatPayNotifyURL
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
	})

	db, err := gorm.Open(sqlite.Open("file:option_wechat_pay?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))

	DB = db
	common.OptionMap = map[string]string{}
	operation_setting.PayAddress = "https://epay.example.com"
	operation_setting.EpayId = "epay-id"
	operation_setting.EpayKey = "epay-key"
	operation_setting.PayMethods = []map[string]string{{"name": "微信", "type": "wxpay"}}

	require.NoError(t, UpdateOption("WechatPaySandbox", "false"))
	require.NoError(t, UpdateOption("WechatPayAppId", "wx1234567890abcdef"))
	require.NoError(t, UpdateOption("WechatPayMchId", "1900000001"))
	require.NoError(t, UpdateOption("WechatPayMerchantSerialNo", "7777777777777777777777777777777777777777"))
	require.NoError(t, UpdateOption("WechatPayMerchantPrivateKey", "merchant-private-key"))
	require.NoError(t, UpdateOption("WechatPayAPIv3Key", "0123456789abcdef0123456789abcdef"))
	require.NoError(t, UpdateOption("WechatPayPlatformSerialNo", "8888888888888888888888888888888888888888"))
	require.NoError(t, UpdateOption("WechatPayPlatformPublicKey", "platform-public-key"))
	require.NoError(t, UpdateOption("WechatPayUnitPrice", "7.3"))
	require.NoError(t, UpdateOption("WechatPayMinTopUp", "5"))
	require.NoError(t, UpdateOption("WechatPayNotifyUrl", "https://app.example.com/api/wechat/notify"))
	require.NoError(t, UpdateOption("WechatPayEnabled", "true"))

	require.True(t, setting.WechatPayEnabled)
	require.False(t, setting.WechatPaySandbox)
	require.Equal(t, "wx1234567890abcdef", setting.WechatPayAppId)
	require.Equal(t, "1900000001", setting.WechatPayMchId)
	require.Equal(t, "7777777777777777777777777777777777777777", setting.WechatPayMerchantSerialNo)
	require.Equal(t, "merchant-private-key", setting.WechatPayMerchantPrivateKey)
	require.Equal(t, "0123456789abcdef0123456789abcdef", setting.WechatPayAPIv3Key)
	require.Equal(t, "8888888888888888888888888888888888888888", setting.WechatPayPlatformSerialNo)
	require.Equal(t, "platform-public-key", setting.WechatPayPlatformPublicKey)
	require.InDelta(t, 7.3, setting.WechatPayUnitPrice, 0.000001)
	require.Equal(t, 5, setting.WechatPayMinTopUp)
	require.Equal(t, "https://app.example.com/api/wechat/notify", setting.WechatPayNotifyUrl)

	require.Equal(t, "https://epay.example.com", operation_setting.PayAddress)
	require.Equal(t, "epay-id", operation_setting.EpayId)
	require.Equal(t, "epay-key", operation_setting.EpayKey)
	require.Equal(t, []map[string]string{{"name": "微信", "type": "wxpay"}}, operation_setting.PayMethods)
	require.Equal(t, "true", common.OptionMap["WechatPayEnabled"])
	require.Equal(t, "7.3", common.OptionMap["WechatPayUnitPrice"])
	require.Equal(t, "5", common.OptionMap["WechatPayMinTopUp"])
}
