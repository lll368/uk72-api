package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type CompleteTopUpOrderRequest struct {
	TradeNo                 string
	ExpectedPaymentProvider string
	ActualPaymentMethod     string
	ActualPaidAmount        float64
	ProviderPayload         string
	CallerIP                string
	StripeCustomerID        string
	CustomerEmail           string
}

// CompleteTopUpOrder 统一处理普通充值成功入账，保证订单、quota、钱包余额、佣金和流水在同一事务内完成。
func CompleteTopUpOrder(req CompleteTopUpOrderRequest) error {
	if strings.TrimSpace(req.TradeNo) == "" {
		return errors.New("未提供支付单号")
	}

	var (
		completedTopUp       *model.TopUp
		quotaToAdd           int
		shouldRecordTopUpLog bool
	)
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var topUp model.TopUp
		if err := walletLockQuery(tx).Where("trade_no = ?", req.TradeNo).First(&topUp).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrTopUpNotFound
			}
			return err
		}
		if req.ExpectedPaymentProvider != "" && topUp.PaymentProvider != req.ExpectedPaymentProvider {
			return model.ErrPaymentMethodMismatch
		}
		if topUp.Status == common.TopUpStatusSuccess {
			snapshotQuota := calculateTopUpQuota(&topUp)
			applyTopUpSnapshotDefaults(&topUp, snapshotQuota)
			if err := tx.Save(&topUp).Error; err != nil {
				return err
			}
			if err := settleTopUpCommissionsTx(tx, &topUp); err != nil {
				return err
			}
			completedTopUp = &topUp
			return nil
		}
		if topUp.Status != common.TopUpStatusPending {
			return model.ErrTopUpStatusInvalid
		}

		quotaToAdd = calculateTopUpQuota(&topUp)
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}
		applyTopUpSnapshotDefaults(&topUp, quotaToAdd)
		applyTopUpActualPaidAmount(&topUp, req.ActualPaidAmount)
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if req.ActualPaymentMethod != "" && topUp.PaymentMethod != req.ActualPaymentMethod {
			topUp.PaymentMethod = req.ActualPaymentMethod
		}
		if err := tx.Save(&topUp).Error; err != nil {
			return err
		}
		if err := updateTopUpUserPaymentSnapshotTx(tx, topUp.UserId, req); err != nil {
			return err
		}
		if err := creditBalanceByQuotaTx(
			tx,
			topUp.UserId,
			quotaToAdd,
			topUp.TradeNo,
			model.WalletFlowTypeRechargeBalance,
			"充值入消费余额",
			"wallet:recharge:"+topUp.TradeNo,
		); err != nil {
			return err
		}
		if err := settleTopUpCommissionsTx(tx, &topUp); err != nil {
			return err
		}
		if err := createQiniuQuotaGrantForRechargeTx(tx, topUp.UserId, topUp.TradeNo, quotaToAdd); err != nil {
			return err
		}
		completedTopUp = &topUp
		shouldRecordTopUpLog = true
		return nil
	})
	if err != nil {
		return err
	}
	if shouldRecordTopUpLog && completedTopUp != nil && quotaToAdd > 0 {
		model.RecordTopupLog(
			completedTopUp.UserId,
			fmt.Sprintf("充值成功，充值额度: %v，支付金额：%.2f", logger.FormatQuota(quotaToAdd), completedTopUp.Money),
			req.CallerIP,
			completedTopUp.PaymentMethod,
			completedTopUp.PaymentProvider,
		)
	}
	return nil
}

func calculateTopUpQuota(topUp *model.TopUp) int {
	if topUp == nil {
		return 0
	}
	switch topUp.PaymentProvider {
	case model.PaymentProviderStripe:
		return int(decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
	case model.PaymentProviderCreem:
		return int(topUp.Amount)
	default:
		return int(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
	}
}

// ApplyTopUpSnapshot 在订单创建阶段固化充值金额、实际支付金额和折扣快照。
func ApplyTopUpSnapshot(topUp *model.TopUp) {
	if topUp == nil {
		return
	}
	applyTopUpSnapshotDefaults(topUp, calculateTopUpQuota(topUp))
}

func applyTopUpSnapshotDefaults(topUp *model.TopUp, quotaToAdd int) {
	if topUp == nil {
		return
	}
	if topUp.RechargeAmount <= 0 {
		topUp.RechargeAmount = quotaToWalletAmount(quotaToAdd)
	}
	if topUp.PaidAmount <= 0 {
		topUp.PaidAmount = topUp.Money
	}
	if topUp.Discount <= 0 {
		if topUp.RechargeAmount > 0 && topUp.PaidAmount > 0 {
			topUp.Discount = decimal.NewFromFloat(topUp.PaidAmount).Div(decimal.NewFromFloat(topUp.RechargeAmount)).InexactFloat64()
		}
		if topUp.Discount <= 0 {
			topUp.Discount = 1
		}
	}
}

func applyTopUpActualPaidAmount(topUp *model.TopUp, actualPaidAmount float64) {
	if topUp == nil || actualPaidAmount <= 0 {
		return
	}
	topUp.PaidAmount = actualPaidAmount
	if topUp.RechargeAmount > 0 {
		topUp.Discount = decimal.NewFromFloat(topUp.PaidAmount).Div(decimal.NewFromFloat(topUp.RechargeAmount)).InexactFloat64()
	}
	if topUp.Discount <= 0 {
		topUp.Discount = 1
	}
}

func updateTopUpUserPaymentSnapshotTx(tx *gorm.DB, userId int, req CompleteTopUpOrderRequest) error {
	updates := map[string]interface{}{}
	if strings.TrimSpace(req.StripeCustomerID) != "" {
		updates["stripe_customer"] = strings.TrimSpace(req.StripeCustomerID)
	}
	if strings.TrimSpace(req.CustomerEmail) != "" {
		var user model.User
		if err := tx.Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if strings.TrimSpace(user.Email) == "" {
			updates["email"] = strings.TrimSpace(req.CustomerEmail)
		}
	}
	if len(updates) == 0 {
		return nil
	}
	return tx.Model(&model.User{}).Where("id = ?", userId).Updates(updates).Error
}
