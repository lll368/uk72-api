package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// ReverseTopUpOrder 对普通充值订单执行幂等冲正，保留原始订单和历史流水。
func ReverseTopUpOrder(tradeNo string, provider string, reason string) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}
	reason = normalizeReverseReason(reason)

	return model.DB.Transaction(func(tx *gorm.DB) error {
		var topUp model.TopUp
		if err := walletLockQuery(tx).Where("trade_no = ?", tradeNo).First(&topUp).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrTopUpNotFound
			}
			return err
		}
		if strings.TrimSpace(provider) != "" && topUp.PaymentProvider != provider {
			return model.ErrPaymentMethodMismatch
		}
		if topUp.Status == common.TopUpStatusReversed {
			return nil
		}
		if topUp.Status != common.TopUpStatusSuccess {
			return model.ErrTopUpStatusInvalid
		}

		quotaToReverse := calculateTopUpQuota(&topUp)
		if quotaToReverse <= 0 {
			return errors.New("无效的冲正额度")
		}
		if err := debitBalanceByQuotaTx(tx, topUp.UserId, quotaToReverse, topUp.TradeNo, model.WalletFlowTypeRefundReverse, reason, "wallet:topup-reverse:"+topUp.TradeNo); err != nil {
			return err
		}
		if err := reverseCommissionsTx(tx, model.CommissionSourceTypeTopUp, topUp.TradeNo, reason); err != nil {
			return err
		}
		now := common.GetTimestamp()
		return tx.Model(&model.TopUp{}).Where("id = ? AND status = ?", topUp.Id, common.TopUpStatusSuccess).
			Updates(map[string]interface{}{
				"status":      common.TopUpStatusReversed,
				"reversed_at": now,
			}).Error
	})
}

// ReverseVipActivationOrder 对 VVIP 开通订单执行幂等冲正，复用 disabled 状态表达撤销。
func ReverseVipActivationOrder(tradeNo string, provider string, reason string) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return model.ErrVipActivationOrderNotFound
	}
	reason = normalizeReverseReason(reason)

	return model.DB.Transaction(func(tx *gorm.DB) error {
		var order model.VipActivationRecord
		if err := vipActivationLockQuery(tx).Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrVipActivationOrderNotFound
			}
			return err
		}
		if strings.TrimSpace(provider) != "" && order.PaymentProvider != provider {
			return model.ErrPaymentMethodMismatch
		}
		if order.Status == model.VipActivationStatusDisabled {
			now := common.GetTimestamp()
			if err := tx.Model(&model.VipActivationRecord{}).
				Where("id = ?", order.Id).
				Updates(map[string]interface{}{
					"disable_reason": mergeDisableReason(order.DisableReason, reason),
					"updated_at":     now,
				}).Error; err != nil {
				return err
			}
			if err := createVipActivationReverseWalletFlowTx(tx, &order, reason); err != nil {
				return err
			}
			return reverseCommissionsTx(tx, model.CommissionSourceTypeVipActivation, order.TradeNo, reason)
		}
		if order.Status != model.VipActivationStatusSuccess {
			return model.ErrVipActivationOrderStatusInvalid
		}

		now := common.GetTimestamp()
		if err := tx.Model(&model.VipActivationRecord{}).
			Where("id = ? AND status = ?", order.Id, model.VipActivationStatusSuccess).
			Updates(map[string]interface{}{
				"status":         model.VipActivationStatusDisabled,
				"disabled_at":    now,
				"disable_reason": mergeDisableReason(order.DisableReason, reason),
				"updated_at":     now,
			}).Error; err != nil {
			return err
		}
		if err := upsertVvipProfileTx(tx, order.UserId, order.ActivatedAt, false, now); err != nil {
			return err
		}
		if err := createVipActivationReverseWalletFlowTx(tx, &order, reason); err != nil {
			return err
		}
		return reverseCommissionsTx(tx, model.CommissionSourceTypeVipActivation, order.TradeNo, reason)
	})
}

func normalizeReverseReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "支付退款或冲正"
	}
	return reason
}

func mergeDisableReason(existing string, reason string) string {
	existing = strings.TrimSpace(existing)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return existing
	}
	if existing == "" {
		return reason
	}
	if strings.Contains(existing, reason) {
		return existing
	}
	return existing + "；" + reason
}
