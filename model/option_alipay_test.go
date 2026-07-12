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

func TestUpdateAlipayOptionsDoesNotMutateEpaySettings(t *testing.T) {
	originalDB := DB
	originalOptionMap := common.OptionMap
	originalAlipayEnabled := setting.AlipayEnabled
	originalAlipaySandbox := setting.AlipaySandbox
	originalAlipayAppID := setting.AlipayAppId
	originalAlipayPrivateKey := setting.AlipayPrivateKey
	originalAlipayPublicKey := setting.AlipayPublicKey
	originalAlipayUnitPrice := setting.AlipayUnitPrice
	originalAlipayMinTopUp := setting.AlipayMinTopUp
	originalAlipayReturnURL := setting.AlipayReturnUrl
	originalAlipayNotifyURL := setting.AlipayNotifyUrl
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		DB = originalDB
		common.OptionMap = originalOptionMap
		setting.AlipayEnabled = originalAlipayEnabled
		setting.AlipaySandbox = originalAlipaySandbox
		setting.AlipayAppId = originalAlipayAppID
		setting.AlipayPrivateKey = originalAlipayPrivateKey
		setting.AlipayPublicKey = originalAlipayPublicKey
		setting.AlipayUnitPrice = originalAlipayUnitPrice
		setting.AlipayMinTopUp = originalAlipayMinTopUp
		setting.AlipayReturnUrl = originalAlipayReturnURL
		setting.AlipayNotifyUrl = originalAlipayNotifyURL
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
	})

	db, err := gorm.Open(sqlite.Open("file:option_alipay?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))

	DB = db
	common.OptionMap = map[string]string{}
	operation_setting.PayAddress = "https://epay.example.com"
	operation_setting.EpayId = "epay-id"
	operation_setting.EpayKey = "epay-key"
	operation_setting.PayMethods = []map[string]string{{"name": "支付宝", "type": "alipay"}}

	require.NoError(t, UpdateOption("AlipayEnabled", "true"))
	require.NoError(t, UpdateOption("AlipaySandbox", "true"))
	require.NoError(t, UpdateOption("AlipayAppId", "2021000000000000"))
	require.NoError(t, UpdateOption("AlipayPrivateKey", "merchant-private-key"))
	require.NoError(t, UpdateOption("AlipayPublicKey", "alipay-public-key"))
	require.NoError(t, UpdateOption("AlipayUnitPrice", "7.3"))
	require.NoError(t, UpdateOption("AlipayMinTopUp", "5"))
	require.NoError(t, UpdateOption("AlipayReturnUrl", "https://app.example.com/wallet"))
	require.NoError(t, UpdateOption("AlipayNotifyUrl", "https://app.example.com/api/alipay/notify"))

	require.True(t, setting.AlipayEnabled)
	require.True(t, setting.AlipaySandbox)
	require.Equal(t, "2021000000000000", setting.AlipayAppId)
	require.Equal(t, "merchant-private-key", setting.AlipayPrivateKey)
	require.Equal(t, "alipay-public-key", setting.AlipayPublicKey)
	require.InDelta(t, 7.3, setting.AlipayUnitPrice, 0.000001)
	require.Equal(t, 5, setting.AlipayMinTopUp)
	require.Equal(t, "https://app.example.com/wallet", setting.AlipayReturnUrl)
	require.Equal(t, "https://app.example.com/api/alipay/notify", setting.AlipayNotifyUrl)

	require.Equal(t, "https://epay.example.com", operation_setting.PayAddress)
	require.Equal(t, "epay-id", operation_setting.EpayId)
	require.Equal(t, "epay-key", operation_setting.EpayKey)
	require.Equal(t, []map[string]string{{"name": "支付宝", "type": "alipay"}}, operation_setting.PayMethods)
	require.Equal(t, "true", common.OptionMap["AlipayEnabled"])
	require.Equal(t, "7.3", common.OptionMap["AlipayUnitPrice"])
	require.Equal(t, "5", common.OptionMap["AlipayMinTopUp"])
}
