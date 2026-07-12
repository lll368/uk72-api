package service

import (
	"errors"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	PaymentReconcileDiffLocalMissing      = "local_missing"
	PaymentReconcileDiffProviderMissing   = "provider_missing"
	PaymentReconcileDiffAmountMismatch    = "amount_mismatch"
	PaymentReconcileDiffStatusMismatch    = "status_mismatch"
	PaymentReconcileDiffDuplicateCallback = "duplicate_callback"
)

const paymentReconcileAmountEpsilon = 0.000001

// ProviderPaymentOrder 是第三方渠道账单摘要，v1 由管理端导入或传入。
type ProviderPaymentOrder struct {
	TradeNo    string  `json:"trade_no"`
	BizType    string  `json:"biz_type"`
	PaidAmount float64 `json:"paid_amount"`
	Status     string  `json:"status"`
}

// PaymentReconciliationDiff 表示一次本地账单和渠道账单的差异。
type PaymentReconciliationDiff struct {
	TradeNo            string  `json:"trade_no"`
	BizType            string  `json:"biz_type"`
	DiffType           string  `json:"diff_type"`
	LocalStatus        string  `json:"local_status"`
	ProviderStatus     string  `json:"provider_status"`
	LocalPaidAmount    float64 `json:"local_paid_amount"`
	ProviderPaidAmount float64 `json:"provider_paid_amount"`
}

// ReconcilePaymentOrdersRequest 描述本地支付对账任务的输入。
type ReconcilePaymentOrdersRequest struct {
	Provider string                 `json:"provider"`
	DateFrom int64                  `json:"date_from"`
	DateTo   int64                  `json:"date_to"`
	Orders   []ProviderPaymentOrder `json:"orders"`
}

type localPaymentOrder struct {
	tradeNo    string
	bizType    string
	status     string
	paidAmount float64
}

// ReconcilePaymentOrders 创建对账任务，并使用传入渠道账单摘要与本地订单做差异比对。
func ReconcilePaymentOrders(req ReconcilePaymentOrdersRequest) (*model.PaymentReconciliationTask, []PaymentReconciliationDiff, error) {
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		return nil, nil, errors.New("支付渠道不能为空")
	}
	if req.DateFrom <= 0 || req.DateTo <= 0 || req.DateFrom > req.DateTo {
		return nil, nil, errors.New("对账时间范围无效")
	}

	var (
		task  *model.PaymentReconciliationTask
		diffs []PaymentReconciliationDiff
	)
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		localOrders, err := loadLocalPaymentOrdersTx(tx, provider, req.DateFrom, req.DateTo)
		if err != nil {
			return err
		}
		diffs, err = comparePaymentOrdersTx(tx, provider, localOrders, req.Orders)
		if err != nil {
			return err
		}
		task = &model.PaymentReconciliationTask{
			Provider:   provider,
			DateFrom:   req.DateFrom,
			DateTo:     req.DateTo,
			Status:     model.PaymentProcessStatusSuccess,
			TotalCount: len(req.Orders),
			DiffCount:  len(diffs),
		}
		return tx.Create(task).Error
	})
	return task, diffs, err
}

func loadLocalPaymentOrdersTx(tx *gorm.DB, provider string, dateFrom int64, dateTo int64) (map[string]localPaymentOrder, error) {
	result := make(map[string]localPaymentOrder)
	topUpSuccessStatus := common.TopUpStatusSuccess
	topUpReversedStatus := common.TopUpStatusReversed
	var topUps []model.TopUp
	if err := tx.Where(
		"payment_provider = ? AND ("+
			"(status = ? AND ((complete_time > ? AND complete_time >= ? AND complete_time <= ?) OR (complete_time = ? AND create_time >= ? AND create_time <= ?))) OR "+
			"(status = ? AND ((reversed_at > ? AND reversed_at >= ? AND reversed_at <= ?) OR (reversed_at = ? AND complete_time > ? AND complete_time >= ? AND complete_time <= ?) OR (reversed_at = ? AND complete_time = ? AND create_time >= ? AND create_time <= ?))) OR "+
			"(status NOT IN ? AND create_time >= ? AND create_time <= ?))",
		provider,
		topUpSuccessStatus,
		0,
		dateFrom,
		dateTo,
		0,
		dateFrom,
		dateTo,
		topUpReversedStatus,
		0,
		dateFrom,
		dateTo,
		0,
		0,
		dateFrom,
		dateTo,
		0,
		0,
		dateFrom,
		dateTo,
		[]string{topUpSuccessStatus, topUpReversedStatus},
		dateFrom,
		dateTo,
	).
		Find(&topUps).Error; err != nil {
		return nil, err
	}
	for _, topUp := range topUps {
		result[paymentOrderKey(PaymentBizTypeTopUp, topUp.TradeNo)] = localPaymentOrder{
			tradeNo:    topUp.TradeNo,
			bizType:    PaymentBizTypeTopUp,
			status:     topUp.Status,
			paidAmount: topUp.PaidAmount,
		}
	}

	vipSuccessStatus := model.VipActivationStatusSuccess
	vipDisabledStatus := model.VipActivationStatusDisabled
	var vipOrders []model.VipActivationRecord
	if err := tx.Where(
		"payment_provider = ? AND ("+
			"(status = ? AND ((activated_at > ? AND activated_at >= ? AND activated_at <= ?) OR (activated_at = ? AND created_at >= ? AND created_at <= ?))) OR "+
			"(status = ? AND ((disabled_at > ? AND disabled_at >= ? AND disabled_at <= ?) OR (disabled_at = ? AND activated_at > ? AND activated_at >= ? AND activated_at <= ?) OR (disabled_at = ? AND activated_at = ? AND created_at >= ? AND created_at <= ?))) OR "+
			"(status NOT IN ? AND created_at >= ? AND created_at <= ?))",
		provider,
		vipSuccessStatus,
		0,
		dateFrom,
		dateTo,
		0,
		dateFrom,
		dateTo,
		vipDisabledStatus,
		0,
		dateFrom,
		dateTo,
		0,
		0,
		dateFrom,
		dateTo,
		0,
		0,
		dateFrom,
		dateTo,
		[]string{vipSuccessStatus, vipDisabledStatus},
		dateFrom,
		dateTo,
	).
		Find(&vipOrders).Error; err != nil {
		return nil, err
	}
	for _, order := range vipOrders {
		result[paymentOrderKey(PaymentBizTypeVipActivation, order.TradeNo)] = localPaymentOrder{
			tradeNo:    order.TradeNo,
			bizType:    PaymentBizTypeVipActivation,
			status:     order.Status,
			paidAmount: order.PaidAmount,
		}
	}

	var subscriptionOrders []model.SubscriptionOrder
	if err := tx.Where(
		"payment_provider = ? AND ((status IN ? AND ((complete_time > ? AND complete_time >= ? AND complete_time <= ?) OR (complete_time = ? AND create_time >= ? AND create_time <= ?))) OR (status NOT IN ? AND create_time >= ? AND create_time <= ?))",
		provider,
		[]string{common.TopUpStatusSuccess},
		0,
		dateFrom,
		dateTo,
		0,
		dateFrom,
		dateTo,
		[]string{common.TopUpStatusSuccess},
		dateFrom,
		dateTo,
	).
		Find(&subscriptionOrders).Error; err != nil {
		return nil, err
	}
	for _, order := range subscriptionOrders {
		result[paymentOrderKey(PaymentBizTypeSubscription, order.TradeNo)] = localPaymentOrder{
			tradeNo:    order.TradeNo,
			bizType:    PaymentBizTypeSubscription,
			status:     order.Status,
			paidAmount: order.Money,
		}
	}
	return result, nil
}

func comparePaymentOrdersTx(tx *gorm.DB, provider string, localOrders map[string]localPaymentOrder, providerOrders []ProviderPaymentOrder) ([]PaymentReconciliationDiff, error) {
	diffs := make([]PaymentReconciliationDiff, 0)
	seenProvider := make(map[string]bool, len(providerOrders))
	for _, providerOrder := range providerOrders {
		providerOrder.TradeNo = strings.TrimSpace(providerOrder.TradeNo)
		providerOrder.BizType = normalizePaymentBizType(providerOrder.BizType)
		key := paymentOrderKey(providerOrder.BizType, providerOrder.TradeNo)
		seenProvider[key] = true
		localOrder, ok := localOrders[key]
		if !ok {
			diffs = append(diffs, PaymentReconciliationDiff{
				TradeNo:            providerOrder.TradeNo,
				BizType:            providerOrder.BizType,
				DiffType:           PaymentReconcileDiffLocalMissing,
				ProviderStatus:     providerOrder.Status,
				ProviderPaidAmount: providerOrder.PaidAmount,
			})
			continue
		}
		if math.Abs(localOrder.paidAmount-providerOrder.PaidAmount) > paymentReconcileAmountEpsilon {
			diffs = append(diffs, PaymentReconciliationDiff{
				TradeNo:            localOrder.tradeNo,
				BizType:            localOrder.bizType,
				DiffType:           PaymentReconcileDiffAmountMismatch,
				LocalStatus:        localOrder.status,
				ProviderStatus:     providerOrder.Status,
				LocalPaidAmount:    localOrder.paidAmount,
				ProviderPaidAmount: providerOrder.PaidAmount,
			})
		}
		if strings.TrimSpace(providerOrder.Status) != "" && localOrder.status != providerOrder.Status {
			diffs = append(diffs, PaymentReconciliationDiff{
				TradeNo:            localOrder.tradeNo,
				BizType:            localOrder.bizType,
				DiffType:           PaymentReconcileDiffStatusMismatch,
				LocalStatus:        localOrder.status,
				ProviderStatus:     providerOrder.Status,
				LocalPaidAmount:    localOrder.paidAmount,
				ProviderPaidAmount: providerOrder.PaidAmount,
			})
		}
		duplicate, err := hasDuplicateSuccessfulCallbackTx(tx, provider, localOrder.tradeNo, localOrder.bizType)
		if err != nil {
			return nil, err
		}
		if duplicate {
			diffs = append(diffs, PaymentReconciliationDiff{
				TradeNo:         localOrder.tradeNo,
				BizType:         localOrder.bizType,
				DiffType:        PaymentReconcileDiffDuplicateCallback,
				LocalStatus:     localOrder.status,
				ProviderStatus:  providerOrder.Status,
				LocalPaidAmount: localOrder.paidAmount,
			})
		}
	}

	for key, localOrder := range localOrders {
		if seenProvider[key] {
			continue
		}
		duplicate, err := hasDuplicateSuccessfulCallbackTx(tx, provider, localOrder.tradeNo, localOrder.bizType)
		if err != nil {
			return nil, err
		}
		if duplicate {
			diffs = append(diffs, PaymentReconciliationDiff{
				TradeNo:         localOrder.tradeNo,
				BizType:         localOrder.bizType,
				DiffType:        PaymentReconcileDiffDuplicateCallback,
				LocalStatus:     localOrder.status,
				LocalPaidAmount: localOrder.paidAmount,
			})
		}
		diffs = append(diffs, PaymentReconciliationDiff{
			TradeNo:         localOrder.tradeNo,
			BizType:         localOrder.bizType,
			DiffType:        PaymentReconcileDiffProviderMissing,
			LocalStatus:     localOrder.status,
			LocalPaidAmount: localOrder.paidAmount,
		})
	}
	return diffs, nil
}

func hasDuplicateSuccessfulCallbackTx(tx *gorm.DB, provider string, tradeNo string, bizType string) (bool, error) {
	var count int64
	err := tx.Model(&model.PaymentCallbackLog{}).
		Where("provider = ? AND trade_no = ? AND biz_type = ? AND verify_status = ? AND process_status = ?", provider, tradeNo, bizType, true, model.PaymentProcessStatusSuccess).
		Count(&count).Error
	return count > 1, err
}

func normalizePaymentBizType(bizType string) string {
	bizType = strings.TrimSpace(bizType)
	if bizType == "" {
		return PaymentBizTypeTopUp
	}
	return bizType
}

func paymentOrderKey(bizType string, tradeNo string) string {
	return normalizePaymentBizType(bizType) + ":" + strings.TrimSpace(tradeNo)
}

// PaymentReconciliationTaskFailed 创建失败对账任务，供 controller 记录参数以外的执行失败。
func PaymentReconciliationTaskFailed(provider string, dateFrom int64, dateTo int64, err error) *model.PaymentReconciliationTask {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &model.PaymentReconciliationTask{
		Provider:     strings.TrimSpace(provider),
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		Status:       model.PaymentProcessStatusFailed,
		ErrorMessage: msg,
		CreatedAt:    common.GetTimestamp(),
		UpdatedAt:    common.GetTimestamp(),
	}
}
