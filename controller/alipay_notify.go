package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type alipayNotifyOrder struct {
	TradeNo        string
	BizType        string
	ExpectedAmount float64
}

// AlipayNotify 处理支付宝官方直连异步通知。资金变更只信任验签通过的服务端回调。
func AlipayNotify(c *gin.Context) {
	params, err := parseAlipayNotifyRequest(c)
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 参数解析失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		writeAlipayNotifyResponse(c, false)
		return
	}

	tradeNo := strings.TrimSpace(params["out_trade_no"])
	tradeStatus := strings.TrimSpace(params["trade_status"])
	payload := common.GetJsonString(params)
	auditLog, auditErr := service.CreatePaymentCallbackAudit(service.PaymentCallbackAuditInput{
		Provider:  model.PaymentProviderAlipay,
		EventType: tradeStatus,
		TradeNo:   tradeNo,
		BizType:   service.PaymentBizTypeUnknown,
		Payload:   []byte(payload),
	})
	if auditErr != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 审计日志创建失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), auditErr.Error()))
	}

	if !isAlipayWebhookEnabled() {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, "webhook disabled")
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 被拒绝 reason=webhook_disabled trade_no=%s path=%q client_ip=%s", tradeNo, c.Request.RequestURI, c.ClientIP()))
		writeAlipayNotifyResponse(c, false)
		return
	}
	if err = validateAlipayNotifyParams(params); err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 校验失败 trade_no=%s client_ip=%s error=%q params=%q", tradeNo, c.ClientIP(), err.Error(), payload))
		writeAlipayNotifyResponse(c, false)
		return
	}

	order, err := resolveAlipayNotifyOrder(tradeNo)
	if err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 订单校验失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		writeAlipayNotifyResponse(c, false)
		return
	}
	actualPaidAmount, err := parseAlipayNotifyPaidAmount(params)
	if err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 金额解析失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		writeAlipayNotifyResponse(c, false)
		return
	}
	if !matchAlipayNotifyAmount(order.ExpectedAmount, actualPaidAmount) {
		err = fmt.Errorf("支付宝回调金额不匹配: expected=%.2f actual=%s", order.ExpectedAmount, actualPaidAmount.StringFixed(2))
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 金额不匹配 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		writeAlipayNotifyResponse(c, false)
		return
	}

	_ = service.MarkPaymentCallbackAuditVerified(auditLog, tradeNo, tradeStatus, order.BizType)
	if service.IsAlipayClosedTradeStatus(tradeStatus) {
		err = func() error {
			LockOrder(tradeNo)
			defer UnlockOrder(tradeNo)
			return failAlipayClosedNotifyOrder(tradeNo, order.BizType, payload)
		}()
		if err != nil {
			_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
			logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 关闭订单处理失败 trade_no=%s biz_type=%s client_ip=%s error=%q", tradeNo, order.BizType, c.ClientIP(), err.Error()))
			writeAlipayNotifyResponse(c, false)
			return
		}
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusSuccess, "")
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 关闭订单处理成功 trade_no=%s biz_type=%s client_ip=%s", tradeNo, order.BizType, c.ClientIP()))
		writeAlipayNotifyResponse(c, true)
		return
	}
	if !service.IsAlipaySuccessTradeStatus(tradeStatus) {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusSuccess, "")
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 忽略非成功状态 trade_no=%s trade_status=%s biz_type=%s client_ip=%s", tradeNo, tradeStatus, order.BizType, c.ClientIP()))
		writeAlipayNotifyResponse(c, true)
		return
	}

	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	if order.BizType == service.PaymentBizTypeVipActivation {
		err = service.CompleteVipActivationOrder(tradeNo, payload, model.PaymentProviderAlipay, model.PaymentMethodAlipayDirect)
	} else {
		err = service.CompleteTopUpOrder(service.CompleteTopUpOrderRequest{
			TradeNo:                 tradeNo,
			ExpectedPaymentProvider: model.PaymentProviderAlipay,
			ActualPaymentMethod:     model.PaymentMethodAlipayDirect,
			ActualPaidAmount:        actualPaidAmount.InexactFloat64(),
			ProviderPayload:         payload,
			CallerIP:                c.ClientIP(),
		})
	}
	if err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 结算失败 trade_no=%s biz_type=%s client_ip=%s error=%q", tradeNo, order.BizType, c.ClientIP(), err.Error()))
		writeAlipayNotifyResponse(c, false)
		return
	}

	_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusSuccess, "")
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝直连 webhook 处理成功 trade_no=%s trade_status=%s biz_type=%s paid_amount=%s client_ip=%s", tradeNo, tradeStatus, order.BizType, actualPaidAmount.StringFixed(2), c.ClientIP()))
	writeAlipayNotifyResponse(c, true)
}

func parseAlipayNotifyRequest(c *gin.Context) (map[string]string, error) {
	if c == nil || c.Request == nil {
		return nil, errors.New("支付宝回调请求为空")
	}
	if c.Request.Method != http.MethodPost {
		return nil, errors.New("支付宝回调仅支持 POST")
	}
	if err := c.Request.ParseForm(); err != nil {
		return nil, err
	}
	params := service.NormalizeAlipayNotifyValues(c.Request.PostForm)
	if len(params) == 0 {
		return nil, errors.New("支付宝回调参数为空")
	}
	return params, nil
}

func validateAlipayNotifyParams(params map[string]string) error {
	if strings.TrimSpace(params["out_trade_no"]) == "" {
		return errors.New("支付宝回调缺少商户订单号")
	}
	if strings.TrimSpace(params["trade_status"]) == "" {
		return errors.New("支付宝回调缺少交易状态")
	}
	if err := service.VerifyConfiguredAlipayNotify(params); err != nil {
		return err
	}
	if strings.TrimSpace(params["app_id"]) != strings.TrimSpace(setting.AlipayAppId) {
		return errors.New("支付宝回调应用 ID 不匹配")
	}
	return nil
}

func resolveAlipayNotifyOrder(tradeNo string) (*alipayNotifyOrder, error) {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, errors.New("支付宝回调缺少商户订单号")
	}
	if record, err := model.GetVipActivationRecordByTradeNo(tradeNo); err == nil {
		if record.PaymentProvider != model.PaymentProviderAlipay {
			return nil, model.ErrPaymentMethodMismatch
		}
		return &alipayNotifyOrder{
			TradeNo:        tradeNo,
			BizType:        service.PaymentBizTypeVipActivation,
			ExpectedAmount: record.PaidAmount,
		}, nil
	} else if !errors.Is(err, model.ErrVipActivationOrderNotFound) {
		return nil, err
	}

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil {
		return nil, model.ErrTopUpNotFound
	}
	if topUp.PaymentProvider != model.PaymentProviderAlipay {
		return nil, model.ErrPaymentMethodMismatch
	}
	expectedAmount := topUp.PaidAmount
	if expectedAmount <= 0 {
		expectedAmount = topUp.Money
	}
	return &alipayNotifyOrder{
		TradeNo:        tradeNo,
		BizType:        service.PaymentBizTypeTopUp,
		ExpectedAmount: expectedAmount,
	}, nil
}

func parseAlipayNotifyPaidAmount(params map[string]string) (decimal.Decimal, error) {
	value := strings.TrimSpace(params["receipt_amount"])
	if value == "" {
		value = strings.TrimSpace(params["total_amount"])
	}
	if value == "" {
		return decimal.Zero, errors.New("支付宝回调缺少支付金额")
	}
	amount, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero, fmt.Errorf("支付宝回调支付金额格式错误: %w", err)
	}
	if !amount.GreaterThan(decimal.Zero) {
		return decimal.Zero, errors.New("支付宝回调支付金额必须大于 0")
	}
	return amount.Round(2), nil
}

func matchAlipayNotifyAmount(expected float64, actual decimal.Decimal) bool {
	if expected <= 0 {
		return false
	}
	expectedDecimal := decimal.NewFromFloat(expected).Round(2)
	return expectedDecimal.Equal(actual.Round(2))
}

func failAlipayClosedNotifyOrder(tradeNo string, bizType string, payload string) error {
	if bizType == service.PaymentBizTypeVipActivation {
		return service.FailVipActivationOrder(tradeNo, payload, model.PaymentProviderAlipay)
	}
	err := model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderAlipay, common.TopUpStatusFailed)
	if !errors.Is(err, model.ErrTopUpStatusInvalid) {
		return err
	}
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp != nil && topUp.Status == common.TopUpStatusFailed {
		// 支付宝可能重复投递 TRADE_CLOSED，已失败订单按幂等成功处理。
		return nil
	}
	return err
}

func writeAlipayNotifyResponse(c *gin.Context, success bool) {
	if success {
		c.String(http.StatusOK, "success")
		return
	}
	c.String(http.StatusOK, "fail")
}
