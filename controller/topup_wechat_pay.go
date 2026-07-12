package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func getWechatPayMinTopup() int64 {
	minTopup := setting.WechatPayMinTopUp
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		minTopup = minTopup * int(common.QuotaPerUnit)
	}
	return int64(minTopup)
}

func getWechatPayMoneyForUser(amount int64, userId int, group string) float64 {
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
		Mul(decimal.NewFromFloat(setting.WechatPayUnitPrice)).
		Mul(decimal.NewFromFloat(topupGroupRatio)).
		Mul(decimal.NewFromFloat(discount)).
		InexactFloat64()
}

func RequestWechatPayAmount(c *gin.Context) {
	var req AmountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if !isWechatPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "微信支付未启用"})
		return
	}
	if req.Amount < getWechatPayMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getWechatPayMinTopup())})
		return
	}

	userId := c.GetInt("id")
	group, err := model.GetUserGroup(userId, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getWechatPayMoneyForUser(req.Amount, userId, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": strconv.FormatFloat(payMoney, 'f', 2, 64)})
}

func RequestWechatPayPay(c *gin.Context) {
	var req EpayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if req.PaymentMethod != model.PaymentMethodWechatDirect {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付渠道"})
		return
	}
	if !isWechatPayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "微信支付未启用"})
		return
	}
	if req.Amount < getWechatPayMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getWechatPayMinTopup())})
		return
	}

	userId := c.GetInt("id")
	group, err := model.GetUserGroup(userId, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getWechatPayMoneyForUser(req.Amount, userId, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	tradeNo := createTopUpTradeNo(userId)
	topUp := createTopUpOrderSnapshot(
		userId,
		req.Amount,
		payMoney,
		model.PaymentMethodWechatDirect,
		model.PaymentProviderWechat,
		tradeNo,
	)
	if err = topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", userId, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	notifyURL := strings.TrimSpace(setting.WechatPayNotifyUrl)
	if notifyURL == "" {
		notifyURL = service.GetCallbackAddress() + "/api/wechat/notify"
	}
	result, err := service.CreateWechatPayNativeOrder(c.Request.Context(), service.WechatPayNativeOrderRequest{
		OutTradeNo:  tradeNo,
		Description: fmt.Sprintf("TUC%d", req.Amount),
		PaidAmount:  payMoney,
		NotifyURL:   notifyURL,
		Attach: map[string]string{
			"biz_type": service.PaymentBizTypeTopUp,
		},
	})
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderWechat, common.TopUpStatusFailed)
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付拉起充值支付失败 user_id=%d trade_no=%s amount=%d error=%q", userId, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("微信支付充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f", userId, tradeNo, req.Amount, payMoney))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"trade_no":   result.TradeNo,
			"code_url":   result.CodeURL,
			"expires_at": result.ExpiresAt,
		},
	})
}
