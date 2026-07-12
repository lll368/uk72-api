package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateVipActivationOrder 创建固定金额的 VVIP 一次性开通订单。
func CreateVipActivationOrder(userId int, paymentProvider string, paymentMethod string) (*model.VipActivationRecord, error) {
	if userId <= 0 {
		return nil, errors.New("用户不存在")
	}
	if strings.TrimSpace(paymentProvider) == "" || strings.TrimSpace(paymentMethod) == "" {
		return nil, errors.New("支付渠道不能为空")
	}
	isActive, err := model.IsUserActiveVvip(userId)
	if err != nil {
		return nil, err
	}
	if isActive {
		return nil, model.ErrVipActivationAlreadyActive
	}

	activationPrice := operation_setting.GetVipActivationPaymentAmount()
	if activationPrice <= 0 {
		return nil, errors.New("VVIP 开通费用必须大于等于 0.01")
	}
	tradeNo := fmt.Sprintf("VIPUSR%dNO%s%d", userId, common.GetRandomString(6), time.Now().Unix())
	order := &model.VipActivationRecord{
		UserId:           userId,
		TradeNo:          tradeNo,
		ActivationAmount: activationPrice,
		PaidAmount:       activationPrice,
		Discount:         model.DefaultVipActivationDiscount,
		PaymentProvider:  paymentProvider,
		PaymentMethod:    paymentMethod,
		Status:           model.VipActivationStatusPending,
	}
	if err := model.DB.Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

// CompleteVipActivationOrder 幂等完成 VVIP 开通订单，并同步用户 VVIP 查询快照。
func CompleteVipActivationOrder(tradeNo string, providerPayload string, expectedPaymentProvider string, actualPaymentMethod string) error {
	if strings.TrimSpace(tradeNo) == "" {
		return model.ErrVipActivationOrderNotFound
	}

	var (
		logUserId                    int
		logPaidAmount                float64
		logPaymentMethod             string
		invalidatedByPreviousDisable bool
	)
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var order model.VipActivationRecord
		if err := vipActivationLockQuery(tx).Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrVipActivationOrderNotFound
			}
			return err
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return model.ErrPaymentMethodMismatch
		}
		if order.Status == model.VipActivationStatusSuccess {
			return applyVipActivationSuccessEffectsTx(tx, &order, "", "", order.ActivatedAt, false)
		}
		if order.Status == model.VipActivationStatusDisabled {
			return nil
		}
		if order.Status != model.VipActivationStatusPending {
			return model.ErrVipActivationOrderStatusInvalid
		}
		disabledAfterOrder, err := hasVipActivationDisabledAfterOrderTx(tx, order.UserId, order.CreatedAt)
		if err != nil {
			return err
		}
		if disabledAfterOrder {
			updates := map[string]interface{}{
				"status":     model.VipActivationStatusFailed,
				"updated_at": common.GetTimestamp(),
			}
			if providerPayload != "" {
				updates["provider_payload"] = providerPayload
			}
			if err := tx.Model(&model.VipActivationRecord{}).
				Where("id = ? AND status = ?", order.Id, model.VipActivationStatusPending).
				Updates(updates).Error; err != nil {
				return err
			}
			invalidatedByPreviousDisable = true
			return nil
		}

		now := common.GetTimestamp()
		order.Status = model.VipActivationStatusSuccess
		order.ActivatedAt = now
		if err := applyVipActivationSuccessEffectsTx(tx, &order, providerPayload, actualPaymentMethod, now, true); err != nil {
			return err
		}

		logUserId = order.UserId
		logPaidAmount = order.PaidAmount
		logPaymentMethod = order.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}
	if invalidatedByPreviousDisable {
		return model.ErrVipActivationOrderStatusInvalid
	}
	if logUserId > 0 {
		model.RecordLog(logUserId, model.LogTypeTopup, fmt.Sprintf("VVIP 开通成功，支付金额: %.2f，支付方式: %s", logPaidAmount, logPaymentMethod))
	}
	return nil
}

// RepairVipActivationSettlement 幂等补齐已成功 VVIP 开通订单的资金流水和分佣副作用。
func RepairVipActivationSettlement(tradeNo string, provider string) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return model.ErrVipActivationOrderNotFound
	}
	provider = strings.TrimSpace(provider)
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var order model.VipActivationRecord
		if err := vipActivationLockQuery(tx).Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrVipActivationOrderNotFound
			}
			return err
		}
		if provider != "" && order.PaymentProvider != provider {
			return model.ErrPaymentMethodMismatch
		}
		if order.Status != model.VipActivationStatusSuccess {
			return model.ErrVipActivationOrderStatusInvalid
		}
		return applyVipActivationSuccessEffectsTx(tx, &order, "", "", order.ActivatedAt, false)
	})
}

func applyVipActivationSuccessEffectsTx(tx *gorm.DB, order *model.VipActivationRecord, providerPayload string, actualPaymentMethod string, activatedAt int64, applyDefaultVvipDiscount bool) error {
	if tx == nil {
		tx = model.DB
	}
	if order == nil || order.UserId <= 0 || strings.TrimSpace(order.TradeNo) == "" {
		return nil
	}
	if activatedAt <= 0 {
		activatedAt = common.GetTimestamp()
	}
	order.Status = model.VipActivationStatusSuccess
	order.ActivatedAt = activatedAt
	applyVipActivationPaymentSnapshot(order)
	if providerPayload != "" {
		order.ProviderPayload = providerPayload
	}
	if actualPaymentMethod != "" && order.PaymentMethod != actualPaymentMethod {
		order.PaymentMethod = actualPaymentMethod
	}
	if err := tx.Save(order).Error; err != nil {
		return err
	}
	if err := upsertVvipProfileTx(tx, order.UserId, order.ActivatedAt, true, 0); err != nil {
		return err
	}
	// 默认 VVIP 折扣只在订单首次支付成功时写入，重复回调和 repair 不能覆盖后续人工调整。
	if applyDefaultVvipDiscount {
		if discount, ok := model.GetDefaultVvipTopupDiscount(); ok {
			if err := model.UpdateUserTopupDiscountTx(tx, order.UserId, &discount); err != nil {
				return err
			}
		}
	}
	if err := createVipActivationWalletFlowTx(tx, order); err != nil {
		return err
	}
	return settleVipActivationCommissionsTx(tx, order)
}

func applyVipActivationPaymentSnapshot(order *model.VipActivationRecord) {
	if order == nil {
		return
	}
	if order.ActivationAmount <= 0 {
		order.ActivationAmount = operation_setting.GetVipActivationPaymentAmount()
	}
	if order.PaidAmount <= 0 {
		order.PaidAmount = operation_setting.GetVipActivationPaymentAmount()
	}
	if order.ActivationAmount > 0 && order.PaidAmount > 0 {
		order.Discount = decimal.NewFromFloat(order.PaidAmount).Div(decimal.NewFromFloat(order.ActivationAmount)).InexactFloat64()
	}
	if order.Discount <= 0 {
		order.Discount = model.DefaultVipActivationDiscount
	}
}

func createVipActivationWalletFlowTx(tx *gorm.DB, order *model.VipActivationRecord) error {
	if tx == nil {
		tx = model.DB
	}
	if order == nil || order.UserId <= 0 || strings.TrimSpace(order.TradeNo) == "" {
		return nil
	}
	amount := order.PaidAmount
	if amount <= 0 {
		amount = operation_setting.GetVipActivationPaymentAmount()
	}
	idempotencyKey := fmt.Sprintf("wallet:vip-activation:%s", strings.TrimSpace(order.TradeNo))
	exists, err := walletFlowExistsTx(tx, idempotencyKey)
	if err != nil || exists {
		return err
	}
	account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
	if err != nil {
		return err
	}
	return createWalletFlowTx(tx, &model.WalletFlow{
		UserId:                order.UserId,
		BizNo:                 strings.TrimSpace(order.TradeNo),
		IdempotencyKey:        walletIdempotencyKey(idempotencyKey),
		FlowType:              model.WalletFlowTypeVipActivation,
		WalletType:            model.WalletTypeBalance,
		Direction:             model.WalletFlowDirectionOut,
		Amount:                amount,
		BalanceAfter:          account.BalanceAmount,
		CommissionAfter:       account.CommissionAmount,
		FrozenCommissionAfter: account.FrozenCommissionAmount,
		Remark:                "VVIP 开通支付",
	})
}

func createVipActivationReverseWalletFlowTx(tx *gorm.DB, order *model.VipActivationRecord, reason string) error {
	if tx == nil {
		tx = model.DB
	}
	if order == nil || order.UserId <= 0 || strings.TrimSpace(order.TradeNo) == "" {
		return nil
	}
	amount := order.PaidAmount
	if amount <= 0 {
		amount = operation_setting.GetVipActivationPaymentAmount()
	}
	idempotencyKey := fmt.Sprintf("wallet:vip-activation-reverse:%s", strings.TrimSpace(order.TradeNo))
	exists, err := walletFlowExistsTx(tx, idempotencyKey)
	if err != nil || exists {
		return err
	}
	account, err := getOrCreateWalletAccountTx(tx, order.UserId, true)
	if err != nil {
		return err
	}
	return createWalletFlowTx(tx, &model.WalletFlow{
		UserId:                order.UserId,
		BizNo:                 strings.TrimSpace(order.TradeNo),
		IdempotencyKey:        walletIdempotencyKey(idempotencyKey),
		FlowType:              model.WalletFlowTypeRefundReverse,
		WalletType:            model.WalletTypeBalance,
		Direction:             model.WalletFlowDirectionIn,
		Amount:                amount,
		BalanceAfter:          account.BalanceAmount,
		CommissionAfter:       account.CommissionAmount,
		FrozenCommissionAfter: account.FrozenCommissionAmount,
		Remark:                strings.TrimSpace(reason),
	})
}

// FailVipActivationOrder 将待支付 VVIP 开通订单标记为失败，供支付失败、过期回调使用。
func FailVipActivationOrder(tradeNo string, providerPayload string, expectedPaymentProvider string) error {
	if strings.TrimSpace(tradeNo) == "" {
		return model.ErrVipActivationOrderNotFound
	}

	return model.DB.Transaction(func(tx *gorm.DB) error {
		var order model.VipActivationRecord
		if err := vipActivationLockQuery(tx).Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrVipActivationOrderNotFound
			}
			return err
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return model.ErrPaymentMethodMismatch
		}
		if order.Status == model.VipActivationStatusFailed {
			return nil
		}
		if order.Status != model.VipActivationStatusPending {
			return model.ErrVipActivationOrderStatusInvalid
		}
		updates := map[string]interface{}{
			"status":     model.VipActivationStatusFailed,
			"updated_at": common.GetTimestamp(),
		}
		if providerPayload != "" {
			updates["provider_payload"] = providerPayload
		}
		return tx.Model(&model.VipActivationRecord{}).
			Where("id = ?", order.Id).
			Updates(updates).Error
	})
}

// DisableVipActivation 禁用用户当前有效 VVIP 身份，并保留开通记录审计信息。
func DisableVipActivation(userId int, adminUserId int, reason string, callerIp string) error {
	if userId <= 0 {
		return errors.New("用户不存在")
	}
	now := common.GetTimestamp()
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var records []model.VipActivationRecord
		if err := vipActivationLockQuery(tx).
			Where("user_id = ? AND status IN ?", userId, []string{model.VipActivationStatusPending, model.VipActivationStatusSuccess}).
			Find(&records).Error; err != nil {
			return err
		}
		latestActivatedAt := int64(0)
		for _, record := range records {
			if record.Status == model.VipActivationStatusSuccess && record.ActivatedAt > latestActivatedAt {
				latestActivatedAt = record.ActivatedAt
			}
		}
		if latestActivatedAt == 0 {
			return model.ErrVipActivationOrderNotFound
		}

		updates := map[string]interface{}{
			"status":         model.VipActivationStatusDisabled,
			"disabled_at":    now,
			"disabled_by":    adminUserId,
			"disable_reason": strings.TrimSpace(reason),
			"updated_at":     now,
		}
		if err := tx.Model(&model.VipActivationRecord{}).
			Where("user_id = ? AND status = ? AND activated_at > ?", userId, model.VipActivationStatusSuccess, 0).
			Updates(updates).Error; err != nil {
			return err
		}
		// 禁用 VVIP 时必须同时让旧待支付订单失效，避免旧支付回调重新激活身份。
		if err := tx.Model(&model.VipActivationRecord{}).
			Where("user_id = ? AND status = ?", userId, model.VipActivationStatusPending).
			Updates(map[string]interface{}{
				"status":           model.VipActivationStatusFailed,
				"provider_payload": "管理员禁用 VVIP 权限，待支付订单失效",
				"updated_at":       now,
			}).Error; err != nil {
			return err
		}
		return upsertVvipProfileTx(tx, userId, latestActivatedAt, false, now)
	})
	if err != nil {
		return err
	}
	model.RecordLogWithAdminInfo(userId, model.LogTypeManage, "管理员禁用 VVIP 权限", map[string]interface{}{
		"admin_user_id": adminUserId,
		"caller_ip":     callerIp,
		"reason":        reason,
	})
	return nil
}

// BindInvitationRelationAfterRegistrationTx 在注册事务中按 VVIP 资格兜底绑定上下级关系。
func BindInvitationRelationAfterRegistrationTx(tx *gorm.DB, inviterId int, childUserId int, source string) error {
	if tx == nil {
		tx = model.DB
	}
	if inviterId <= 0 || childUserId <= 0 {
		return nil
	}
	isActiveVvip, err := model.IsUserActiveVvipTx(tx, inviterId)
	if err != nil {
		return err
	}
	if !isActiveVvip {
		return nil
	}
	if source == "" {
		source = model.UserRelationSourceRegister
	}
	_, err = model.CreateActiveUserRelationTx(tx, inviterId, childUserId, source, "")
	if err == nil {
		return nil
	}
	if errors.Is(err, model.ErrUserRelationSelfBinding) ||
		errors.Is(err, model.ErrUserRelationAlreadyBound) ||
		errors.Is(err, model.ErrUserRelationCycle) {
		return nil
	}
	return err
}

func upsertVvipProfileTx(tx *gorm.DB, userId int, activatedAt int64, active bool, disabledAt int64) error {
	if tx == nil {
		tx = model.DB
	}
	var profile model.UserProfile
	err := tx.Where("user_id = ?", userId).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		status := model.VvipStatusDisabled
		if active {
			status = model.VvipStatusActive
		}
		profile = model.UserProfile{
			UserId:          userId,
			IsVvip:          active,
			VvipActivatedAt: activatedAt,
			VvipDisabledAt:  disabledAt,
			VvipStatus:      status,
		}
		return tx.Create(&profile).Error
	}
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"is_vvip":           active,
		"vvip_activated_at": activatedAt,
		"vvip_disabled_at":  disabledAt,
	}
	if active {
		updates["vvip_status"] = model.VvipStatusActive
		updates["vvip_disabled_at"] = int64(0)
	} else {
		updates["vvip_status"] = model.VvipStatusDisabled
	}
	return tx.Model(&model.UserProfile{}).Where("user_id = ?", userId).Updates(updates).Error
}

func hasVipActivationDisabledAfterOrderTx(tx *gorm.DB, userId int, orderCreatedAt int64) (bool, error) {
	if tx == nil {
		tx = model.DB
	}
	var count int64
	err := tx.Model(&model.VipActivationRecord{}).
		Where("user_id = ? AND status = ? AND disabled_at > ? AND disabled_at > ?", userId, model.VipActivationStatusDisabled, 0, orderCreatedAt).
		Count(&count).Error
	return count > 0, err
}

func vipActivationLockQuery(tx *gorm.DB) *gorm.DB {
	if common.UsingSQLite {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

// ManualVvipActivationInput 管理员手动激活 VVIP 的输入参数。
type ManualVvipActivationInput struct {
	UserId      int
	AdminUserId int
	Remark      string
	CallerIP    string
}

// AdminManualActivateVvip 管理员手动将用户设置为算力伙伴（VVIP）。
// 不产生资金流水、不产生上级分佣，但需填写备注以便审计。
func AdminManualActivateVvip(input ManualVvipActivationInput) error {
	if input.UserId <= 0 {
		return errors.New("用户不存在")
	}
	if strings.TrimSpace(input.Remark) == "" {
		return model.ErrVipActivationManualRemarkRequired
	}
	if input.AdminUserId <= 0 {
		return errors.New("管理员信息无效")
	}

	user, err := model.GetUserById(input.UserId, false)
	if err != nil || user == nil {
		return errors.New("用户不存在")
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Check user VVIP status — only allow if never been VVIP
		var profile model.UserProfile
		err := tx.Where("user_id = ?", input.UserId).First(&profile).Error
		if err == nil && (profile.VvipStatus == model.VvipStatusActive || profile.VvipStatus == model.VvipStatusDisabled) {
			return model.ErrVipActivationUserAlreadyVvip
		}
		// gorm.ErrRecordNotFound means user has no profile yet → VVIP status is "none", allowed

		// 2. Also check there's no active VvipActivationRecord
		isActive, err := model.IsUserActiveVvipTx(tx, input.UserId)
		if err != nil {
			return err
		}
		if isActive {
			return model.ErrVipActivationUserAlreadyVvip
		}

		// 3. Create VipActivationRecord with zero amount, skip BeforeCreate hooks
		now := common.GetTimestamp()
		tradeNo := fmt.Sprintf("VIPMANUAL%dNO%s%d", input.UserId, common.GetRandomString(6), now)
		record := &model.VipActivationRecord{
			UserId:           input.UserId,
			TradeNo:          tradeNo,
			ActivationAmount: 0,
			PaidAmount:       0,
			Discount:         0,
			PaymentProvider:  "manual",
			PaymentMethod:    "manual",
			Status:           model.VipActivationStatusSuccess,
			ActivatedAt:      now,
			ActivatedBy:      input.AdminUserId,
			ActivationRemark: strings.TrimSpace(input.Remark),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := tx.Session(&gorm.Session{SkipHooks: true}).Create(record).Error; err != nil {
			return err
		}

		// 4. Upsert VVIP profile
		if err := upsertVvipProfileTx(tx, input.UserId, now, true, 0); err != nil {
			return err
		}

		// 5. Apply default VVIP topup discount
		if discount, ok := model.GetDefaultVvipTopupDiscount(); ok {
			if err := model.UpdateUserTopupDiscountTx(tx, input.UserId, &discount); err != nil {
				return err
			}
		}

		// 6. Generate AffCode if missing
		if strings.TrimSpace(user.AffCode) == "" {
			user.AffCode = common.GetRandomString(4)
			if err := tx.Model(&model.User{}).Where("id = ?", input.UserId).Update("aff_code", user.AffCode).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}
	model.RecordLogWithAdminInfo(input.UserId, model.LogTypeManage, "管理员手动设置算力伙伴", map[string]interface{}{
		"admin_user_id": input.AdminUserId,
		"caller_ip":     input.CallerIP,
		"remark":        input.Remark,
	})
	return nil
}
