package service

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

var (
	ErrWithdrawAmountInvalid = errors.New("提现金额无效")
	ErrWithdrawStatusInvalid = errors.New("提现订单状态不允许当前操作")
	ErrWithdrawOrderNotFound = errors.New("提现订单不存在")
)

type SubmitWithdrawOrderRequest struct {
	UserId         int
	Amount         float64
	FeeAmount      float64
	ReceiveType    string
	ReceiveAccount string
	Remark         string
}

// SubmitWithdrawOrder 创建提现单并立即冻结可提现佣金。
func SubmitWithdrawOrder(req SubmitWithdrawOrderRequest) (*model.WithdrawOrder, error) {
	if req.UserId <= 0 {
		return nil, errors.New("用户不存在")
	}
	if req.Amount <= 0 {
		return nil, ErrWithdrawAmountInvalid
	}
	minAmount := operation_setting.GetPaymentSetting().CommissionMinWithdrawAmount
	if minAmount > 0 && req.Amount+walletAmountEpsilon < minAmount {
		return nil, fmt.Errorf("提现金额不能小于 %.2f", minAmount)
	}
	if strings.TrimSpace(req.ReceiveAccount) == "" {
		return nil, errors.New("收款账户不能为空")
	}
	if req.FeeAmount < 0 || req.FeeAmount > req.Amount {
		return nil, errors.New("提现手续费无效")
	}

	var order *model.WithdrawOrder
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		account, err := getOrCreateWalletAccountTx(tx, req.UserId, true)
		if err != nil {
			return err
		}
		if account.CommissionAmount+walletAmountEpsilon < req.Amount {
			return ErrCommissionInsufficient
		}
		account.CommissionAmount -= req.Amount
		account.FrozenCommissionAmount += req.Amount
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		order = &model.WithdrawOrder{
			UserId:         req.UserId,
			WithdrawNo:     fmt.Sprintf("WDR%d%s", req.UserId, common.GetTimeString()),
			Amount:         req.Amount,
			FeeAmount:      req.FeeAmount,
			ActualAmount:   req.Amount - req.FeeAmount,
			Status:         model.WithdrawStatusPending,
			ReceiveType:    strings.TrimSpace(req.ReceiveType),
			ReceiveAccount: strings.TrimSpace(req.ReceiveAccount),
			Remark:         strings.TrimSpace(req.Remark),
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		return createWalletFlowTx(tx, &model.WalletFlow{
			UserId:                req.UserId,
			BizNo:                 order.WithdrawNo,
			IdempotencyKey:        walletIdempotencyKey("wallet:withdraw-freeze:" + order.WithdrawNo),
			FlowType:              model.WalletFlowTypeWithdrawFreeze,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionOut,
			Amount:                req.Amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                "提现申请冻结佣金",
		})
	})
	return order, err
}

func lockWithdrawOrderTx(tx *gorm.DB, id int) (*model.WithdrawOrder, error) {
	if id <= 0 {
		return nil, ErrWithdrawOrderNotFound
	}
	var order model.WithdrawOrder
	if err := walletLockQuery(tx).Where("id = ?", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWithdrawOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

// ApproveWithdrawOrder 将待审核提现单标记为审核通过，不改变冻结金额。
func ApproveWithdrawOrder(id int, reviewerId int, remark string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderTx(tx, id)
		if err != nil {
			return err
		}
		if order.Status != model.WithdrawStatusPending {
			return ErrWithdrawStatusInvalid
		}
		if order.Provider != "" && order.Provider != model.WithdrawProviderManual {
			return ErrWithdrawStatusInvalid
		}
		now := common.GetTimestamp()
		return tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
			"status":      model.WithdrawStatusApproved,
			"reviewer_id": reviewerId,
			"reviewed_at": now,
			"remark":      strings.TrimSpace(remark),
		}).Error
	})
}

// RejectWithdrawOrder 驳回提现并把冻结佣金退回可提现佣金。
func RejectWithdrawOrder(id int, reviewerId int, reason string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderTx(tx, id)
		if err != nil {
			return err
		}
		if order.Status == model.WithdrawStatusRejected || order.Status == model.WithdrawStatusPaid {
			return ErrWithdrawStatusInvalid
		}
		if order.Provider != "" && order.Provider != model.WithdrawProviderManual {
			return ErrWithdrawStatusInvalid
		}
		account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
		if err != nil {
			return err
		}
		account.FrozenCommissionAmount = math.Max(0, account.FrozenCommissionAmount-order.Amount)
		account.CommissionAmount += order.Amount
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
			"status":      model.WithdrawStatusRejected,
			"reviewer_id": reviewerId,
			"reviewed_at": now,
			"fail_reason": strings.TrimSpace(reason),
			"remark":      strings.TrimSpace(reason),
		}).Error; err != nil {
			return err
		}
		return createWalletFlowTx(tx, &model.WalletFlow{
			UserId:                order.UserId,
			BizNo:                 order.WithdrawNo,
			IdempotencyKey:        walletIdempotencyKey("wallet:withdraw-reject:" + order.WithdrawNo),
			FlowType:              model.WalletFlowTypeWithdrawReject,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionIn,
			Amount:                order.Amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                strings.TrimSpace(reason),
		})
	})
}

// MarkWithdrawOrderPaid 登记打款成功，并从冻结佣金中扣减提现金额。
func MarkWithdrawOrderPaid(id int, reviewerId int, paymentVoucher string, remark string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderTx(tx, id)
		if err != nil {
			return err
		}
		if order.Status != model.WithdrawStatusApproved && order.Status != model.WithdrawStatusFailed {
			return ErrWithdrawStatusInvalid
		}
		if order.Provider != "" && order.Provider != model.WithdrawProviderManual {
			return ErrWithdrawStatusInvalid
		}
		account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
		if err != nil {
			return err
		}
		if account.FrozenCommissionAmount+walletAmountEpsilon < order.Amount {
			return ErrCommissionInsufficient
		}
		account.FrozenCommissionAmount = math.Max(0, account.FrozenCommissionAmount-order.Amount)
		account.TotalWithdrawAmount += order.Amount
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		if err := tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
			"status":          model.WithdrawStatusPaid,
			"reviewer_id":     reviewerId,
			"reviewed_at":     now,
			"paid_at":         now,
			"payment_voucher": strings.TrimSpace(paymentVoucher),
			"remark":          strings.TrimSpace(remark),
		}).Error; err != nil {
			return err
		}
		return createWalletFlowTx(tx, &model.WalletFlow{
			UserId:                order.UserId,
			BizNo:                 order.WithdrawNo,
			IdempotencyKey:        walletIdempotencyKey("wallet:withdraw-success:" + order.WithdrawNo),
			FlowType:              model.WalletFlowTypeWithdrawSuccess,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionOut,
			Amount:                order.Amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                strings.TrimSpace(remark),
		})
	})
}

// MarkWithdrawOrderFailed 标记打款失败，保留冻结佣金以便重试或后续驳回。
func MarkWithdrawOrderFailed(id int, reviewerId int, reason string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockWithdrawOrderTx(tx, id)
		if err != nil {
			return err
		}
		if order.Status != model.WithdrawStatusApproved {
			return ErrWithdrawStatusInvalid
		}
		if order.Provider != "" && order.Provider != model.WithdrawProviderManual {
			return ErrWithdrawStatusInvalid
		}
		now := common.GetTimestamp()
		return tx.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
			"status":      model.WithdrawStatusFailed,
			"reviewer_id": reviewerId,
			"reviewed_at": now,
			"fail_reason": strings.TrimSpace(reason),
			"remark":      strings.TrimSpace(reason),
		}).Error
	})
}
