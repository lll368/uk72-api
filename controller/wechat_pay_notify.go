package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type wechatPayNotifyOrder struct {
	TradeNo       string
	BizType       string
	ExpectedCents int64
}

const wechatPayNotifyMaxBodyBytes int64 = 1024 * 1024

// WechatPayNotify 处理微信支付 API v3 异步通知。资金变更只信任验签和解密通过的服务端回调。
func WechatPayNotify(c *gin.Context) {
	body, err := readWechatPayNotifyBody(c)
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付 webhook 读取请求体失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		writeWechatPayNotifyError(c, http.StatusBadRequest, err.Error())
		return
	}

	payload, parseErr := service.ParseWechatPayNotifyPayload(body)
	eventType := "notify"
	if parseErr == nil && payload != nil && strings.TrimSpace(payload.EventType) != "" {
		eventType = payload.EventType
	}
	auditLog, auditErr := service.CreatePaymentCallbackAudit(service.PaymentCallbackAuditInput{
		Provider:  model.PaymentProviderWechat,
		EventType: eventType,
		BizType:   service.PaymentBizTypeUnknown,
		Payload:   body,
	})
	if auditErr != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 webhook 审计日志创建失败 client_ip=%s error=%q", c.ClientIP(), auditErr.Error()))
	}

	if !isWechatPayWebhookEnabled() {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, "webhook disabled")
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付 webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		writeWechatPayNotifyError(c, http.StatusBadRequest, "webhook disabled")
		return
	}
	if parseErr != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, parseErr.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付 webhook payload 解析失败 client_ip=%s error=%q", c.ClientIP(), parseErr.Error()))
		writeWechatPayNotifyError(c, http.StatusBadRequest, "invalid payload")
		return
	}
	if err = validateWechatPayNotifySignature(c, body); err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付 webhook 验签失败 client_ip=%s error=%q", c.ClientIP(), err.Error()))
		writeWechatPayNotifyError(c, http.StatusBadRequest, "invalid signature")
		return
	}

	transactionBody, err := service.DecryptConfiguredWechatPayResource(payload.Resource)
	if err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付 webhook 解密失败 event_type=%s client_ip=%s error=%q", payload.EventType, c.ClientIP(), err.Error()))
		writeWechatPayNotifyError(c, http.StatusBadRequest, "decrypt failed")
		return
	}
	transaction, err := service.ParseWechatPayTransaction(transactionBody)
	if err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付 webhook 交易解析失败 event_type=%s client_ip=%s error=%q", payload.EventType, c.ClientIP(), err.Error()))
		writeWechatPayNotifyError(c, http.StatusBadRequest, "invalid transaction")
		return
	}
	tradeNo := strings.TrimSpace(transaction.OutTradeNo)
	_ = service.MarkPaymentCallbackAuditVerified(auditLog, tradeNo, payload.EventType, service.PaymentBizTypeUnknown)

	if err = validateWechatPayTransactionConfig(transaction); err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付 webhook 应用或商户校验失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		writeWechatPayNotifyError(c, http.StatusBadRequest, err.Error())
		return
	}
	order, err := resolveWechatPayNotifyOrder(tradeNo)
	if err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付 webhook 订单校验失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		writeWechatPayNotifyError(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = service.MarkPaymentCallbackAuditVerified(auditLog, tradeNo, payload.EventType, order.BizType)
	if transaction.Amount.PaidCents() != order.ExpectedCents {
		err = fmt.Errorf("微信支付回调金额不匹配: expected=%d actual=%d", order.ExpectedCents, transaction.Amount.PaidCents())
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付 webhook 金额不匹配 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		writeWechatPayNotifyError(c, http.StatusBadRequest, "amount mismatch")
		return
	}

	if !service.IsWechatPaySuccessNotification(payload.EventType, transaction.TradeState) {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusSuccess, "")
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("微信支付 webhook 忽略非成功状态 trade_no=%s event_type=%s trade_state=%s biz_type=%s client_ip=%s", tradeNo, payload.EventType, transaction.TradeState, order.BizType, c.ClientIP()))
		writeWechatPayNotifySuccess(c)
		return
	}

	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	payloadText := string(transactionBody)
	if order.BizType == service.PaymentBizTypeVipActivation {
		err = service.CompleteVipActivationOrder(tradeNo, payloadText, model.PaymentProviderWechat, model.PaymentMethodWechatDirect)
	} else {
		actualPaidAmount := float64(transaction.Amount.PaidCents()) / 100
		err = service.CompleteTopUpOrder(service.CompleteTopUpOrderRequest{
			TradeNo:                 tradeNo,
			ExpectedPaymentProvider: model.PaymentProviderWechat,
			ActualPaymentMethod:     model.PaymentMethodWechatDirect,
			ActualPaidAmount:        actualPaidAmount,
			ProviderPayload:         payloadText,
			CallerIP:                c.ClientIP(),
		})
	}
	if err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付 webhook 结算失败 trade_no=%s biz_type=%s client_ip=%s error=%q", tradeNo, order.BizType, c.ClientIP(), err.Error()))
		writeWechatPayNotifyError(c, http.StatusBadRequest, "settlement failed")
		return
	}

	_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusSuccess, "")
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("微信支付 webhook 处理成功 trade_no=%s event_type=%s trade_state=%s biz_type=%s paid_cents=%d client_ip=%s", tradeNo, payload.EventType, transaction.TradeState, order.BizType, transaction.Amount.PaidCents(), c.ClientIP()))
	writeWechatPayNotifySuccess(c)
}

func readWechatPayNotifyBody(c *gin.Context) ([]byte, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, errors.New("微信支付回调请求为空")
	}
	if c.Request.Method != http.MethodPost {
		return nil, errors.New("微信支付回调仅支持 POST")
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, wechatPayNotifyMaxBodyBytes))
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return nil, errors.New("微信支付回调请求体过大")
		}
		return nil, err
	}
	if len(body) == 0 {
		return nil, errors.New("微信支付回调请求体为空")
	}
	return body, nil
}

func validateWechatPayNotifySignature(c *gin.Context, body []byte) error {
	return service.VerifyConfiguredWechatPayNotifySignature(
		c.GetHeader("Wechatpay-Timestamp"),
		c.GetHeader("Wechatpay-Nonce"),
		c.GetHeader("Wechatpay-Serial"),
		body,
		c.GetHeader("Wechatpay-Signature"),
	)
}

func validateWechatPayTransactionConfig(transaction *service.WechatPayTransaction) error {
	if transaction == nil {
		return errors.New("微信支付交易为空")
	}
	if strings.TrimSpace(transaction.OutTradeNo) == "" {
		return errors.New("微信支付回调缺少商户订单号")
	}
	if strings.TrimSpace(transaction.AppID) != strings.TrimSpace(setting.WechatPayAppId) {
		return errors.New("微信支付回调应用 ID 不匹配")
	}
	if strings.TrimSpace(transaction.MchID) != strings.TrimSpace(setting.WechatPayMchId) {
		return errors.New("微信支付回调商户号不匹配")
	}
	if transaction.Amount.PaidCents() <= 0 {
		return errors.New("微信支付回调支付金额必须大于 0")
	}
	return nil
}

func resolveWechatPayNotifyOrder(tradeNo string) (*wechatPayNotifyOrder, error) {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, errors.New("微信支付回调缺少商户订单号")
	}
	if record, err := model.GetVipActivationRecordByTradeNo(tradeNo); err == nil {
		if record.PaymentProvider != model.PaymentProviderWechat {
			return nil, model.ErrPaymentMethodMismatch
		}
		expectedCents, err := service.WechatPayAmountToCents(record.PaidAmount)
		if err != nil {
			return nil, err
		}
		return &wechatPayNotifyOrder{
			TradeNo:       tradeNo,
			BizType:       service.PaymentBizTypeVipActivation,
			ExpectedCents: expectedCents,
		}, nil
	} else if !errors.Is(err, model.ErrVipActivationOrderNotFound) {
		return nil, err
	}

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil {
		return nil, model.ErrTopUpNotFound
	}
	if topUp.PaymentProvider != model.PaymentProviderWechat {
		return nil, model.ErrPaymentMethodMismatch
	}
	expectedAmount := topUp.PaidAmount
	if expectedAmount <= 0 {
		expectedAmount = topUp.Money
	}
	expectedCents, err := service.WechatPayAmountToCents(expectedAmount)
	if err != nil {
		return nil, err
	}
	return &wechatPayNotifyOrder{
		TradeNo:       tradeNo,
		BizType:       service.PaymentBizTypeTopUp,
		ExpectedCents: expectedCents,
	}, nil
}

func writeWechatPayNotifySuccess(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

func writeWechatPayNotifyError(c *gin.Context, status int, message string) {
	if status <= 0 {
		status = http.StatusBadRequest
	}
	c.JSON(status, gin.H{"code": "FAIL", "message": message})
}
