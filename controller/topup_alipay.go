package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func getAlipayMinTopup() int64 {
	minTopup := setting.AlipayMinTopUp
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		minTopup = minTopup * int(common.QuotaPerUnit)
	}
	return int64(minTopup)
}

func getAlipayPayMoneyForUser(amount int64, userId int, group string) float64 {
	originalAmount := amount
	dAmount := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount = dAmount.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	}

	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	discount := getTopupPaymentDiscount(userId, originalAmount)
	return dAmount.
		Mul(decimal.NewFromFloat(setting.AlipayUnitPrice)).
		Mul(decimal.NewFromFloat(topupGroupRatio)).
		Mul(decimal.NewFromFloat(discount)).
		InexactFloat64()
}

func RequestAlipayAmount(c *gin.Context) {
	var req AmountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if !isAlipayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付宝支付未启用"})
		return
	}
	if req.Amount < getAlipayMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getAlipayMinTopup())})
		return
	}

	userId := c.GetInt("id")
	group, err := model.GetUserGroup(userId, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getAlipayPayMoneyForUser(req.Amount, userId, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": strconv.FormatFloat(payMoney, 'f', 2, 64)})
}

func RequestAlipayPay(c *gin.Context) {
	var req EpayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if req.PaymentMethod != model.PaymentMethodAlipayDirect {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付渠道"})
		return
	}
	if !isAlipayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付宝支付未启用"})
		return
	}
	if req.Amount < getAlipayMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getAlipayMinTopup())})
		return
	}

	userId := c.GetInt("id")
	group, err := model.GetUserGroup(userId, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getAlipayPayMoneyForUser(req.Amount, userId, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	tradeNo := createTopUpTradeNo(userId)
	topUp := createTopUpOrderSnapshot(
		userId,
		req.Amount,
		payMoney,
		model.PaymentMethodAlipayDirect,
		model.PaymentProviderAlipay,
		tradeNo,
	)
	if err = topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", userId, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	notifyURL := strings.TrimSpace(setting.AlipayNotifyUrl)
	if notifyURL == "" {
		notifyURL = service.GetCallbackAddress() + "/api/alipay/notify"
	}
	returnURL := strings.TrimSpace(setting.AlipayReturnUrl)
	if returnURL == "" {
		returnURL = paymentReturnPath("/console/topup?show_history=true")
	}
	result, err := service.BuildAlipayPagePayParams(service.AlipayPagePayRequest{
		OutTradeNo:  tradeNo,
		Subject:     fmt.Sprintf("TUC%d", req.Amount),
		TotalAmount: strconv.FormatFloat(payMoney, 'f', 2, 64),
		ReturnURL:   returnURL,
		NotifyURL:   notifyURL,
		PassbackParams: map[string]string{
			"biz_type": service.PaymentBizTypeTopUp,
		},
		Now: time.Now(),
	})
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderAlipay, common.TopUpStatusFailed)
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝拉起充值支付失败 user_id=%d trade_no=%s amount=%d error=%q", userId, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f", userId, tradeNo, req.Amount, payMoney))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"url":    result.GatewayURL,
			"params": result.Params,
		},
	})
}
