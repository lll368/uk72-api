package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type commissionLevelRule struct {
	level       int
	amount      float64
	rate        float64
	baseAmount  float64
	beneficiary int
}

var errCommissionAlreadyRecorded = errors.New("commission already recorded")

func settleTopUpCommissionsTx(tx *gorm.DB, topUp *model.TopUp) error {
	if tx == nil {
		tx = model.DB
	}
	if topUp == nil || topUp.UserId <= 0 || strings.TrimSpace(topUp.TradeNo) == "" {
		return nil
	}
	rules, err := buildTopUpCommissionRulesTx(tx, topUp)
	if err != nil {
		logger.LogError(context.Background(), fmt.Sprintf("普通充值分佣规则构建失败 source_order_no=%s source_user_id=%d error=%q", topUp.TradeNo, topUp.UserId, err.Error()))
		return nil
	}
	sourceUserLabel := buildCommissionSourceUserLabelTx(tx, topUp.UserId)
	for _, rule := range rules {
		if rule.amount <= 0 {
			continue
		}
		settleCommissionRuleTx(tx, createCommissionParams{
			beneficiaryUserId: rule.beneficiary,
			sourceUserId:      topUp.UserId,
			sourceOrderNo:     topUp.TradeNo,
			sourceType:        model.CommissionSourceTypeTopUp,
			level:             rule.level,
			baseAmount:        rule.baseAmount,
			commissionRate:    rule.rate,
			amount:            rule.amount,
			sourceUserLabel:   sourceUserLabel,
			remark:            buildCommissionIncomeRemark(model.CommissionSourceTypeTopUp, sourceUserLabel),
		})
	}
	return nil
}

func topUpCommissionBaseAmount(topUp *model.TopUp) float64 {
	if topUp == nil {
		return 0
	}
	if topUp.PaidAmount > 0 {
		return topUp.PaidAmount
	}
	if topUp.Money > 0 {
		return topUp.Money
	}
	if topUp.RechargeAmount > 0 {
		return topUp.RechargeAmount
	}
	return 0
}

func topUpCommissionRechargeAmount(topUp *model.TopUp) float64 {
	if topUp == nil {
		return 0
	}
	if topUp.RechargeAmount > 0 {
		return topUp.RechargeAmount
	}
	return quotaToWalletAmount(calculateTopUpQuota(topUp))
}

func normalizeCommissionDiscount(discount float64) float64 {
	if discount <= 0 || discount > 1 {
		return 1
	}
	return discount
}

func topUpCommissionSourceDiscountTx(tx *gorm.DB, topUp *model.TopUp) (float64, error) {
	if topUp == nil {
		return 1, nil
	}
	if topUp.PaidAmount > 0 && topUp.RechargeAmount > 0 {
		return normalizeCommissionDiscount(decimal.NewFromFloat(topUp.PaidAmount).Div(decimal.NewFromFloat(topUp.RechargeAmount)).InexactFloat64()), nil
	}
	if topUp.Discount > 0 && topUp.Discount <= 1 {
		return normalizeCommissionDiscount(topUp.Discount), nil
	}
	if topUp.UserId <= 0 {
		return 1, nil
	}
	discount, err := model.GetEffectiveUserTopupDiscountTx(tx, topUp.UserId)
	if err != nil {
		return 1, err
	}
	return normalizeCommissionDiscount(discount), nil
}

func buildTopUpCommissionRulesTx(tx *gorm.DB, topUp *model.TopUp) ([]commissionLevelRule, error) {
	if topUp == nil || topUp.UserId <= 0 {
		return nil, nil
	}
	paidBaseAmount := topUpCommissionBaseAmount(topUp)
	rechargeAmount := topUpCommissionRechargeAmount(topUp)
	if paidBaseAmount <= 0 && rechargeAmount <= 0 {
		return nil, nil
	}
	var level1 model.UserRelation
	if err := tx.Where("child_user_id = ? AND status = ?", topUp.UserId, model.UserRelationStatusActive).First(&level1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	rules := make([]commissionLevelRule, 0, 2)
	if level1.ParentUserId <= 0 {
		return rules, nil
	}
	level1Active, err := model.IsUserActiveVvipTx(tx, level1.ParentUserId)
	if err != nil {
		return nil, err
	}
	if !level1Active {
		return rules, nil
	}
	sourceDiscount, err := topUpCommissionSourceDiscountTx(tx, topUp)
	if err != nil {
		return nil, err
	}
	level1Discount, err := model.GetEffectiveUserTopupDiscountTx(tx, level1.ParentUserId)
	if err != nil {
		return nil, err
	}
	if rule, ok := buildTopUpDiscountSpreadRule(1, paidBaseAmount, sourceDiscount, level1Discount, level1.ParentUserId); ok {
		rules = append(rules, rule)
	}
	var level2 model.UserRelation
	if err := tx.Where("child_user_id = ? AND status = ?", level1.ParentUserId, model.UserRelationStatusActive).First(&level2).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rules, nil
		}
		return nil, err
	}
	if level2.ParentUserId > 0 {
		active, err := model.IsUserActiveVvipTx(tx, level2.ParentUserId)
		if err != nil {
			return nil, err
		}
		if active {
			level2Discount, err := model.GetEffectiveUserTopupDiscountTx(tx, level2.ParentUserId)
			if err != nil {
				return nil, err
			}
			if rule, ok := buildTopUpDiscountSpreadRule(2, paidBaseAmount, level1Discount, level2Discount, level2.ParentUserId); ok {
				rules = append(rules, rule)
			}
		}
	}
	return rules, nil
}

func buildTopUpDiscountSpreadRule(level int, baseAmount float64, downstreamDiscount float64, upstreamDiscount float64, beneficiaryUserId int) (commissionLevelRule, bool) {
	downstreamDiscount = normalizeCommissionDiscount(downstreamDiscount)
	upstreamDiscount = normalizeCommissionDiscount(upstreamDiscount)
	if level <= 0 || beneficiaryUserId <= 0 || baseAmount <= 0 || downstreamDiscount <= upstreamDiscount {
		return commissionLevelRule{}, false
	}
	// 普通充值佣金按用户实际支付金额结算折扣差额，避免到账金额与实付金额不一致时多发佣金。
	rate := decimal.NewFromFloat(downstreamDiscount).Sub(decimal.NewFromFloat(upstreamDiscount)).InexactFloat64()
	amount := decimal.NewFromFloat(baseAmount).Mul(decimal.NewFromFloat(rate)).InexactFloat64()
	if amount <= 0 {
		return commissionLevelRule{}, false
	}
	return commissionLevelRule{
		level:       level,
		amount:      amount,
		rate:        rate,
		baseAmount:  baseAmount,
		beneficiary: beneficiaryUserId,
	}, true
}

func settleVipActivationCommissionsTx(tx *gorm.DB, order *model.VipActivationRecord) error {
	if tx == nil {
		tx = model.DB
	}
	if order == nil || order.UserId <= 0 || strings.TrimSpace(order.TradeNo) == "" {
		return nil
	}
	sourceUserId := order.UserId
	sourceOrderNo := strings.TrimSpace(order.TradeNo)
	baseAmount := order.PaidAmount
	if baseAmount <= 0 {
		baseAmount = operation_setting.GetVipActivationPaymentAmount()
	}
	setting := operation_setting.GetPaymentSetting()
	if err := operation_setting.ValidateVipActivationCommissionAmounts(
		operation_setting.GetVipActivationPaymentAmount(),
		setting.VipActivationCommissionLevel1Amount,
		setting.VipActivationCommissionLevel2Amount,
	); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("VVIP 开通分佣配置无效 source_order_no=%s source_user_id=%d error=%q", sourceOrderNo, sourceUserId, err.Error()))
		return nil
	}
	var level1 model.UserRelation
	if err := tx.Where("child_user_id = ? AND status = ?", sourceUserId, model.UserRelationStatusActive).First(&level1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		logger.LogError(context.Background(), fmt.Sprintf("VVIP 开通分佣查询上级失败 source_order_no=%s source_user_id=%d error=%q", sourceOrderNo, sourceUserId, err.Error()))
		return nil
	}
	if level1.ParentUserId > 0 {
		if err := createVipActivationCommissionForBeneficiaryTx(tx, level1.ParentUserId, sourceUserId, sourceOrderNo, 1, baseAmount, setting.VipActivationCommissionLevel1Amount); err != nil {
			logger.LogError(context.Background(), fmt.Sprintf("VVIP 开通上级分佣失败 source_order_no=%s source_user_id=%d beneficiary_user_id=%d error=%q", sourceOrderNo, sourceUserId, level1.ParentUserId, err.Error()))
		}
	}
	var level2 model.UserRelation
	if err := tx.Where("child_user_id = ? AND status = ?", level1.ParentUserId, model.UserRelationStatusActive).First(&level2).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		logger.LogError(context.Background(), fmt.Sprintf("VVIP 开通分佣查询上上级失败 source_order_no=%s source_user_id=%d level1_user_id=%d error=%q", sourceOrderNo, sourceUserId, level1.ParentUserId, err.Error()))
		return nil
	}
	if level2.ParentUserId > 0 {
		if err := createVipActivationCommissionForBeneficiaryTx(tx, level2.ParentUserId, sourceUserId, sourceOrderNo, 2, baseAmount, setting.VipActivationCommissionLevel2Amount); err != nil {
			logger.LogError(context.Background(), fmt.Sprintf("VVIP 开通上上级分佣失败 source_order_no=%s source_user_id=%d beneficiary_user_id=%d error=%q", sourceOrderNo, sourceUserId, level2.ParentUserId, err.Error()))
		}
	}
	return nil
}

func createVipActivationCommissionForBeneficiaryTx(tx *gorm.DB, beneficiaryUserId int, sourceUserId int, sourceOrderNo string, level int, baseAmount float64, amount float64) error {
	if baseAmount <= 0 || amount <= 0 || !isFiniteCommissionAmount(baseAmount) || !isFiniteCommissionAmount(amount) {
		return nil
	}
	commissionRate := decimal.NewFromFloat(amount).Div(decimal.NewFromFloat(baseAmount)).InexactFloat64()
	if !isFiniteCommissionAmount(commissionRate) {
		return nil
	}
	sourceUserLabel := buildCommissionSourceUserLabelTx(tx, sourceUserId)
	params := createCommissionParams{
		beneficiaryUserId: beneficiaryUserId,
		sourceUserId:      sourceUserId,
		sourceOrderNo:     sourceOrderNo,
		sourceType:        model.CommissionSourceTypeVipActivation,
		level:             level,
		baseAmount:        baseAmount,
		commissionRate:    commissionRate,
		amount:            amount,
		sourceUserLabel:   sourceUserLabel,
		remark:            buildCommissionIncomeRemark(model.CommissionSourceTypeVipActivation, sourceUserLabel),
	}
	active, err := model.IsUserActiveVvipTx(tx, beneficiaryUserId)
	if err != nil {
		logCommissionSettlement("分佣入账失败", params, err.Error())
		if failErr := createFailedCommissionTx(tx, params, err.Error()); failErr != nil {
			return failErr
		}
		return nil
	}
	if !active {
		return nil
	}
	settleCommissionRuleTx(tx, params)
	return nil
}

func isFiniteCommissionAmount(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

type createCommissionParams struct {
	beneficiaryUserId int
	sourceUserId      int
	sourceOrderNo     string
	sourceType        string
	level             int
	baseAmount        float64
	commissionRate    float64
	amount            float64
	sourceUserLabel   string
	remark            string
}

func buildCommissionSourceUserLabelTx(tx *gorm.DB, userId int) string {
	if userId <= 0 {
		return ""
	}
	if tx == nil {
		tx = model.DB
	}
	var user model.User
	if err := tx.Select("id", "username", "display_name").Where("id = ?", userId).First(&user).Error; err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("分佣来源用户展示名查询失败 source_user_id=%d error=%q", userId, err.Error()))
		return fmt.Sprintf("User #%d", userId)
	}
	displayName := strings.TrimSpace(user.DisplayName)
	username := strings.TrimSpace(user.Username)
	switch {
	case displayName != "" && username != "" && displayName != username:
		return fmt.Sprintf("%s/%s (#%d)", displayName, username, user.Id)
	case displayName != "":
		return fmt.Sprintf("%s (#%d)", displayName, user.Id)
	case username != "":
		return fmt.Sprintf("%s (#%d)", username, user.Id)
	default:
		return fmt.Sprintf("User #%d", user.Id)
	}
}

func buildCommissionIncomeRemark(sourceType string, sourceUserLabel string) string {
	sourceUserLabel = strings.TrimSpace(sourceUserLabel)
	if sourceUserLabel == "" {
		sourceUserLabel = "未知用户"
	}
	switch sourceType {
	case model.CommissionSourceTypeVipActivation:
		return fmt.Sprintf("算力伙伴 开通分佣：下级用户 %s", sourceUserLabel)
	default:
		return fmt.Sprintf("充值分佣：下级用户 %s", sourceUserLabel)
	}
}

func settleCommissionRuleTx(tx *gorm.DB, params createCommissionParams) {
	err := tx.Transaction(func(commissionTx *gorm.DB) error {
		return createSettledCommissionTx(commissionTx, params)
	})
	if errors.Is(err, errCommissionAlreadyRecorded) {
		logCommissionSettlement("分佣记录已存在，跳过重复入账", params, "")
		return
	}
	if err == nil {
		logCommissionSettlement("分佣入账成功", params, "")
		return
	}
	logCommissionSettlement("分佣入账失败", params, err.Error())
	if failErr := createFailedCommissionTx(tx, params, err.Error()); failErr != nil {
		logger.LogError(context.Background(), fmt.Sprintf("分佣失败记录写入失败 source_type=%s source_order_no=%s source_user_id=%d beneficiary_user_id=%d level=%d error=%q",
			params.sourceType, params.sourceOrderNo, params.sourceUserId, params.beneficiaryUserId, params.level, failErr.Error()))
	}
}

func createSettledCommissionTx(tx *gorm.DB, params createCommissionParams) error {
	if params.beneficiaryUserId <= 0 || params.sourceUserId <= 0 || params.sourceOrderNo == "" || params.amount <= 0 || params.level <= 0 {
		return nil
	}
	var existing model.CommissionRecord
	err := tx.Where("source_type = ? AND source_order_no = ? AND level = ? AND beneficiary_user_id = ?",
		params.sourceType, params.sourceOrderNo, params.level, params.beneficiaryUserId).
		First(&existing).Error
	if err == nil {
		return errCommissionAlreadyRecorded
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	now := common.GetTimestamp()
	record := &model.CommissionRecord{
		BeneficiaryUserId:   params.beneficiaryUserId,
		SourceUserId:        params.sourceUserId,
		SourceUserLabel:     strings.TrimSpace(params.sourceUserLabel),
		SourceOrderNo:       params.sourceOrderNo,
		SourceType:          params.sourceType,
		Level:               params.level,
		BaseAmount:          params.baseAmount,
		CommissionRate:      params.commissionRate,
		Amount:              params.amount,
		QualificationStatus: model.CommissionQualificationQualified,
		Status:              model.CommissionStatusSettled,
		SettledAt:           now,
	}
	if err := tx.Create(record).Error; err != nil {
		return err
	}
	idempotencyKey := fmt.Sprintf("wallet:commission:%s:%s:%d:%d", params.sourceType, params.sourceOrderNo, params.level, params.beneficiaryUserId)
	return creditCommissionTx(tx, params.beneficiaryUserId, params.sourceOrderNo, params.amount, model.WalletFlowTypeCommissionIncome, params.remark, idempotencyKey)
}

func createFailedCommissionTx(tx *gorm.DB, params createCommissionParams, errorMessage string) error {
	if params.beneficiaryUserId <= 0 || params.sourceUserId <= 0 || params.sourceOrderNo == "" || params.level <= 0 {
		return nil
	}
	var existing model.CommissionRecord
	err := tx.Where("source_type = ? AND source_order_no = ? AND level = ? AND beneficiary_user_id = ?",
		params.sourceType, params.sourceOrderNo, params.level, params.beneficiaryUserId).
		First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	record := &model.CommissionRecord{
		BeneficiaryUserId:   params.beneficiaryUserId,
		SourceUserId:        params.sourceUserId,
		SourceUserLabel:     strings.TrimSpace(params.sourceUserLabel),
		SourceOrderNo:       params.sourceOrderNo,
		SourceType:          params.sourceType,
		Level:               params.level,
		BaseAmount:          params.baseAmount,
		CommissionRate:      params.commissionRate,
		Amount:              params.amount,
		QualificationStatus: model.CommissionQualificationQualified,
		Status:              model.CommissionStatusFailed,
		ErrorMessage:        truncateCommissionErrorMessage(errorMessage),
	}
	return tx.Create(record).Error
}

func truncateCommissionErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	runes := []rune(message)
	if len(runes) <= 512 {
		return message
	}
	return string(runes[:512])
}

func logCommissionSettlement(message string, params createCommissionParams, errorMessage string) {
	logMessage := fmt.Sprintf("%s source_type=%s source_order_no=%s source_user_id=%d beneficiary_user_id=%d level=%d base_amount=%.6f commission_rate=%.6f amount=%.6f",
		message, params.sourceType, params.sourceOrderNo, params.sourceUserId, params.beneficiaryUserId, params.level, params.baseAmount, params.commissionRate, params.amount)
	if strings.TrimSpace(errorMessage) != "" {
		logger.LogError(context.Background(), logMessage+fmt.Sprintf(" error=%q", errorMessage))
		return
	}
	logger.LogInfo(context.Background(), logMessage)
}

// ReverseCommissions 按来源订单幂等冲正已结算佣金，不删除历史佣金记录。
func ReverseCommissions(sourceType string, sourceOrderNo string, reason string) error {
	if strings.TrimSpace(sourceType) == "" || strings.TrimSpace(sourceOrderNo) == "" {
		return errors.New("佣金来源不能为空")
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		return reverseCommissionsTx(tx, sourceType, sourceOrderNo, reason)
	})
}

func reverseCommissionsTx(tx *gorm.DB, sourceType string, sourceOrderNo string, reason string) error {
	if tx == nil {
		tx = model.DB
	}
	if strings.TrimSpace(sourceType) == "" || strings.TrimSpace(sourceOrderNo) == "" {
		return errors.New("佣金来源不能为空")
	}
	var records []model.CommissionRecord
	if err := tx.Where("source_type = ? AND source_order_no = ? AND status = ?", sourceType, sourceOrderNo, model.CommissionStatusSettled).
		Find(&records).Error; err != nil {
		return err
	}
	now := common.GetTimestamp()
	for i := range records {
		record := records[i]
		idempotencyKey := fmt.Sprintf("wallet:commission-reverse:%s:%s:%d:%d", sourceType, sourceOrderNo, record.Level, record.BeneficiaryUserId)
		exists, err := walletFlowExistsTx(tx, idempotencyKey)
		if err != nil || exists {
			if err != nil {
				return err
			}
			continue
		}
		account, err := getOrCreateWalletAccountTx(tx, record.BeneficiaryUserId, true)
		if err != nil {
			return err
		}
		// 冲正允许佣金余额为负，后续佣金入账可自然抵扣，但提现会被余额校验拦截。
		account.CommissionAmount -= record.Amount
		if err := tx.Save(account).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CommissionRecord{}).Where("id = ?", record.Id).Updates(map[string]interface{}{
			"status":         model.CommissionStatusReversed,
			"reversed_at":    now,
			"reverse_reason": strings.TrimSpace(reason),
		}).Error; err != nil {
			return err
		}
		if err := createWalletFlowTx(tx, &model.WalletFlow{
			UserId:                record.BeneficiaryUserId,
			BizNo:                 sourceOrderNo,
			IdempotencyKey:        walletIdempotencyKey(idempotencyKey),
			FlowType:              model.WalletFlowTypeRefundReverse,
			WalletType:            model.WalletTypeCommission,
			Direction:             model.WalletFlowDirectionOut,
			Amount:                record.Amount,
			BalanceAfter:          account.BalanceAmount,
			CommissionAfter:       account.CommissionAmount,
			FrozenCommissionAfter: account.FrozenCommissionAmount,
			Remark:                strings.TrimSpace(reason),
		}); err != nil {
			return err
		}
	}
	return nil
}
