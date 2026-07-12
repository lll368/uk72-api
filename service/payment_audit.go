package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

const (
	PaymentBizTypeTopUp         = "topup"
	PaymentBizTypeVipActivation = "vip_activation"
	PaymentBizTypeSubscription  = "subscription"
	PaymentBizTypeUnknown       = "unknown"
)

// PaymentCallbackAuditInput 描述一次支付回调审计日志的初始信息。
type PaymentCallbackAuditInput struct {
	Provider  string
	EventType string
	TradeNo   string
	BizType   string
	Payload   []byte
}

// CreatePaymentCallbackAudit 创建支付回调审计日志。审计必须独立写入，避免业务事务失败时丢失回调轨迹。
func CreatePaymentCallbackAudit(input PaymentCallbackAuditInput) (*model.PaymentCallbackLog, error) {
	provider := strings.TrimSpace(input.Provider)
	if provider == "" {
		provider = PaymentBizTypeUnknown
	}
	bizType := strings.TrimSpace(input.BizType)
	if bizType == "" {
		bizType = PaymentBizTypeUnknown
	}
	log := &model.PaymentCallbackLog{
		Provider:      provider,
		EventType:     strings.TrimSpace(input.EventType),
		TradeNo:       strings.TrimSpace(input.TradeNo),
		BizType:       bizType,
		VerifyStatus:  false,
		ProcessStatus: model.PaymentProcessStatusPending,
		PayloadDigest: digestPaymentPayload(input.Payload),
	}
	if err := model.DB.Create(log).Error; err != nil {
		return nil, err
	}
	return log, nil
}

// MarkPaymentCallbackAuditVerified 标记回调已通过验签，并补齐业务单号和业务类型。
func MarkPaymentCallbackAuditVerified(log *model.PaymentCallbackLog, tradeNo string, eventType string, bizType string) error {
	if log == nil || log.Id <= 0 {
		return nil
	}
	updates := map[string]interface{}{"verify_status": true}
	if tradeNo = strings.TrimSpace(tradeNo); tradeNo != "" {
		updates["trade_no"] = tradeNo
		log.TradeNo = tradeNo
	}
	if eventType = strings.TrimSpace(eventType); eventType != "" {
		updates["event_type"] = eventType
		log.EventType = eventType
	}
	if bizType = strings.TrimSpace(bizType); bizType != "" {
		updates["biz_type"] = bizType
		log.BizType = bizType
	}
	log.VerifyStatus = true
	return model.DB.Model(&model.PaymentCallbackLog{}).Where("id = ?", log.Id).Updates(updates).Error
}

// FinishPaymentCallbackAudit 标记回调业务处理结果。
func FinishPaymentCallbackAudit(log *model.PaymentCallbackLog, processStatus string, errorMessage string) error {
	if log == nil || log.Id <= 0 {
		return nil
	}
	processStatus = strings.TrimSpace(processStatus)
	if processStatus == "" {
		processStatus = model.PaymentProcessStatusSuccess
	}
	updates := map[string]interface{}{
		"process_status": processStatus,
		"error_message":  strings.TrimSpace(errorMessage),
	}
	log.ProcessStatus = processStatus
	log.ErrorMessage = strings.TrimSpace(errorMessage)
	return model.DB.Model(&model.PaymentCallbackLog{}).Where("id = ?", log.Id).Updates(updates).Error
}

func digestPaymentPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
