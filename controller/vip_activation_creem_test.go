package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVipActivationCreemProductsUseRoundedPaymentAmount(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	oldVipActivationPrice := paymentSetting.VipActivationPrice
	oldCreemProducts := setting.CreemProducts
	paymentSetting.VipActivationPrice = 19.999
	setting.CreemProducts = `[
		{"productId":"prod_rounded","name":"Rounded","price":20,"currency":"USD","quota":20},
		{"productId":"prod_raw","name":"Raw","price":19.999,"currency":"USD","quota":20}
	]`
	t.Cleanup(func() {
		paymentSetting.VipActivationPrice = oldVipActivationPrice
		setting.CreemProducts = oldCreemProducts
	})

	products := getVipActivationCreemProducts()

	require.Len(t, products, 1)
	assert.Equal(t, "prod_rounded", products[0].ProductId)
}
