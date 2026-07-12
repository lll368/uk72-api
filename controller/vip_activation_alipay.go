package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

// VipActivationRequestAlipay 创建支付宝直连 VVIP 一次性开通订单。
func VipActivationRequestAlipay(c *gin.Context) {
	if !ensureVipActivationPayAllowed(c) {
		return
	}
	if !requirePaymentCompliance(c) {
		return
	}
	if !isAlipayTopUpEnabled() {
		common.ApiErrorMsg(c, "支付宝支付未启用")
		return
	}

	userId := c.GetInt("id")
	vipOrder, err := service.CreateVipActivationOrder(userId, model.PaymentProviderAlipay, model.PaymentMethodAlipayDirect)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	notifyURL := strings.TrimSpace(setting.AlipayNotifyUrl)
	if notifyURL == "" {
		notifyURL = service.GetCallbackAddress() + "/api/alipay/notify"
	}
	returnURL := vipActivationReturnPath()
	result, err := service.BuildAlipayPagePayParams(service.AlipayPagePayRequest{
		OutTradeNo:  vipOrder.TradeNo,
		Subject:     "VVIP Activation",
		TotalAmount: strconv.FormatFloat(vipOrder.PaidAmount, 'f', 2, 64),
		ReturnURL:   returnURL,
		NotifyURL:   notifyURL,
		PassbackParams: map[string]string{
			"biz_type": service.PaymentBizTypeVipActivation,
		},
	})
	if err != nil {
		markVipActivationOrderFailed(vipOrder)
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}

	common.ApiSuccess(c, gin.H{
		"url":      result.GatewayURL,
		"params":   result.Params,
		"order_id": vipOrder.TradeNo,
	})
}
