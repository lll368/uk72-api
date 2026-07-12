package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

// VipActivationRequestWechatPay 创建微信支付直连 VVIP 一次性开通订单。
func VipActivationRequestWechatPay(c *gin.Context) {
	if !ensureVipActivationPayAllowed(c) {
		return
	}
	if !requirePaymentCompliance(c) {
		return
	}
	if !isWechatPayTopUpEnabled() {
		common.ApiErrorMsg(c, "微信支付未启用")
		return
	}

	userId := c.GetInt("id")
	vipOrder, err := service.CreateVipActivationOrder(userId, model.PaymentProviderWechat, model.PaymentMethodWechatDirect)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	notifyURL := strings.TrimSpace(setting.WechatPayNotifyUrl)
	if notifyURL == "" {
		notifyURL = service.GetCallbackAddress() + "/api/wechat/notify"
	}
	result, err := service.CreateWechatPayNativeOrder(c.Request.Context(), service.WechatPayNativeOrderRequest{
		OutTradeNo:  vipOrder.TradeNo,
		Description: "VVIP Activation",
		PaidAmount:  vipOrder.PaidAmount,
		NotifyURL:   notifyURL,
		Attach: map[string]string{
			"biz_type": service.PaymentBizTypeVipActivation,
		},
	})
	if err != nil {
		markVipActivationOrderFailed(vipOrder)
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}

	common.ApiSuccess(c, gin.H{
		"code_url":   result.CodeURL,
		"order_id":   vipOrder.TradeNo,
		"expires_at": result.ExpiresAt,
	})
}
