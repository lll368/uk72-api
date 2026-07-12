package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const QiniuCostDetailBucketBillingSource = "qiniu_cost_detail_bucket"
const qiniuCostDetailBucketUserFacingModelName = "billing-settlement"

type qiniuBillingBucketApplyOptions struct {
	ExpectedApplicationId int
	OperationSource       string
}

type qiniuBillingBucketApplyValidationError struct {
	message string
	status  string
	reason  string
}

func (e *qiniuBillingBucketApplyValidationError) Error() string {
	return e.message
}

func newQiniuBillingBucketApplyValidationError(message string) error {
	return &qiniuBillingBucketApplyValidationError{message: message}
}

func newQiniuBillingBucketApplyStatusValidationError(message string, status string, reason string) error {
	return &qiniuBillingBucketApplyValidationError{
		message: message,
		status:  status,
		reason:  reason,
	}
}

func ApplyQiniuBillingBucket(ctx context.Context, bucketId int) (applied bool, skipped bool, err error) {
	return applyQiniuBillingBucket(ctx, bucketId, qiniuBillingBucketApplyOptions{
		OperationSource: model.QiniuBillingOperationSourceSystem,
	})
}

func applyQiniuBillingBucket(ctx context.Context, bucketId int, options qiniuBillingBucketApplyOptions) (applied bool, skipped bool, err error) {
	if bucketId <= 0 {
		return false, false, errors.New("账单 bucket ID 无效")
	}
	if ctx != nil && ctx.Err() != nil {
		return false, false, ctx.Err()
	}
	operationSource := strings.TrimSpace(options.OperationSource)
	if operationSource == "" {
		operationSource = model.QiniuBillingOperationSourceSystem
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		bucket, err := lockQiniuBillingBucketTx(tx, bucketId)
		if err != nil {
			return err
		}
		if bucket.PendingDeltaQuota == 0 && (bucket.Status == model.QiniuBillingBucketStatusApplied || bucket.Status == model.QiniuBillingBucketStatusReconciled) {
			skipped = true
			return nil
		}
		if bucket.OwnerStatus != model.QiniuBillingOwnerStatusResolved && bucket.OwnerStatus != model.QiniuBillingOwnerStatusManualResolved {
			if err := updateQiniuBillingBucketStatusTx(tx, bucket.Id, model.QiniuBillingBucketStatusFailed, "bucket_owner_unresolved"); err != nil {
				return err
			}
			return errors.New("账单 bucket 归属未解析，不能应用账务")
		}
		status, reason := qiniuBillingBucketStatusForPendingDelta(bucket.BillingDate, bucket.PendingDeltaQuota)
		if status != model.QiniuBillingBucketStatusPending && status != model.QiniuBillingBucketStatusReconciled {
			return newQiniuBillingBucketApplyStatusValidationError("账单 bucket 未通过自动落账守卫："+reason, status, reason)
		}
		deltaQuota := bucket.PendingDeltaQuota
		applyVersion := bucket.ApplyVersion + 1
		idempotencyKey := model.QiniuBillingBucketIdempotencyKey(bucket.Id, applyVersion)
		existingApplication, exists, err := qiniuBillingBucketApplicationByIdempotencyTx(tx, idempotencyKey)
		if err != nil {
			return err
		}
		if options.ExpectedApplicationId > 0 {
			if !exists || existingApplication.Id != options.ExpectedApplicationId {
				return newQiniuBillingBucketApplyValidationError("账单 bucket application 不是当前待应用版本")
			}
			if existingApplication.Status != model.QiniuBillingApplicationStatusFailed {
				return newQiniuBillingBucketApplyValidationError("只能重试失败的账单 bucket application")
			}
		}
		if exists && existingApplication.Status == model.QiniuBillingApplicationStatusSuccess {
			skipped = true
			return nil
		}
		if exists && existingApplication.Status != model.QiniuBillingApplicationStatusFailed {
			skipped = true
			return nil
		}
		if deltaQuota != 0 {
			flowExists, err := walletFlowExistsTx(tx, idempotencyKey)
			if err != nil {
				return err
			}
			if flowExists {
				return errors.New("账单 bucket 钱包流水已存在但 application 未成功")
			}
		}

		walletFlowId := 0
		consumeLogId := 0
		balanceBeforeQuota := 0
		balanceAfterQuota := 0
		debtQuota := 0
		if deltaQuota != 0 {
			walletFlowId, balanceBeforeQuota, balanceAfterQuota, debtQuota, err = applyQiniuBucketWalletDeltaTx(tx, bucket, deltaQuota, idempotencyKey)
			if err != nil {
				return err
			}
			if err := applyUserUsedQuotaDeltaTx(tx, bucket.UserId, deltaQuota); err != nil {
				return err
			}
			if err := applyTokenUsedQuotaDeltaTx(tx, bucket.TokenId, deltaQuota); err != nil {
				return err
			}
			consumeLogId, err = createQiniuBucketSettlementLogTx(tx, bucket, applyVersion, deltaQuota, balanceAfterQuota, debtQuota)
			if err != nil {
				return err
			}
		} else {
			balanceBeforeQuota, balanceAfterQuota, err = currentUserBalanceQuotaTx(tx, bucket.UserId)
			if err != nil {
				return err
			}
		}

		application := &model.QiniuBillingBucketApplication{
			BucketId:           bucket.Id,
			ApplyVersion:       applyVersion,
			DeltaQuota:         deltaQuota,
			DeltaAmount:        signedWalletAmount(deltaQuota),
			WalletFlowId:       walletFlowId,
			ConsumeLogId:       consumeLogId,
			IdempotencyKey:     idempotencyKey,
			BalanceBeforeQuota: balanceBeforeQuota,
			BalanceAfterQuota:  balanceAfterQuota,
			DebtQuota:          debtQuota,
			Status:             model.QiniuBillingApplicationStatusSuccess,
			OperationSource:    operationSource,
		}
		if exists {
			if err := updateQiniuBillingBucketApplicationSuccessTx(tx, existingApplication.Id, application); err != nil {
				return err
			}
		} else if err := model.CreateQiniuBillingBucketApplication(tx, application); err != nil {
			return err
		}

		finalStatus := model.QiniuBillingBucketStatusApplied
		if deltaQuota == 0 {
			finalStatus = model.QiniuBillingBucketStatusReconciled
		}
		if err := tx.Model(&model.QiniuBillingBucket{}).Where("id = ?", bucket.Id).Updates(map[string]interface{}{
			"applied_delta_quota": bucket.AppliedDeltaQuota + deltaQuota,
			"pending_delta_quota": 0,
			"apply_version":       applyVersion,
			"status":              finalStatus,
			"last_error":          "",
			"updated_time":        common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil {
		var validationErr *qiniuBillingBucketApplyValidationError
		if !errors.As(err, &validationErr) {
			_ = markQiniuBillingBucketFailed(bucketId, err)
			_ = recordQiniuBillingBucketFailedApplication(bucketId, err)
		} else if validationErr.status != "" {
			_ = updateQiniuBillingBucketStatusTx(model.DB, bucketId, validationErr.status, validationErr.reason)
		}
	}
	return applied, skipped, err
}

func lockQiniuBillingBucketTx(tx *gorm.DB, bucketId int) (*model.QiniuBillingBucket, error) {
	query := tx
	if !common.UsingSQLite {
		query = tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var bucket model.QiniuBillingBucket
	if err := query.Where("id = ?", bucketId).First(&bucket).Error; err != nil {
		return nil, err
	}
	return &bucket, nil
}

func updateQiniuBillingBucketStatusTx(tx *gorm.DB, bucketId int, status string, lastError string) error {
	return tx.Model(&model.QiniuBillingBucket{}).Where("id = ?", bucketId).Updates(map[string]interface{}{
		"status":       status,
		"last_error":   lastError,
		"updated_time": common.GetTimestamp(),
	}).Error
}

func markQiniuBillingBucketFailed(bucketId int, err error) error {
	if err == nil {
		return nil
	}
	return updateQiniuBillingBucketStatusTx(model.DB, bucketId, model.QiniuBillingBucketStatusFailed, sanitizeQiniuTaskError(err))
}

func qiniuBillingBucketApplicationByIdempotencyTx(tx *gorm.DB, idempotencyKey string) (*model.QiniuBillingBucketApplication, bool, error) {
	var application model.QiniuBillingBucketApplication
	err := tx.Where("idempotency_key = ?", idempotencyKey).First(&application).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &application, true, nil
}

func updateQiniuBillingBucketApplicationSuccessTx(tx *gorm.DB, applicationId int, application *model.QiniuBillingBucketApplication) error {
	if applicationId <= 0 || application == nil {
		return errors.New("账单 bucket application 无效")
	}
	return tx.Model(&model.QiniuBillingBucketApplication{}).Where("id = ?", applicationId).Updates(map[string]interface{}{
		"delta_quota":          application.DeltaQuota,
		"delta_amount":         application.DeltaAmount,
		"wallet_flow_id":       application.WalletFlowId,
		"consume_log_id":       application.ConsumeLogId,
		"balance_before_quota": application.BalanceBeforeQuota,
		"balance_after_quota":  application.BalanceAfterQuota,
		"debt_quota":           application.DebtQuota,
		"status":               model.QiniuBillingApplicationStatusSuccess,
		"last_error":           "",
		"next_retry_time":      0,
		"operation_source":     application.OperationSource,
		"updated_time":         common.GetTimestamp(),
	}).Error
}

func recordQiniuBillingBucketFailedApplication(bucketId int, cause error) error {
	if bucketId <= 0 || cause == nil {
		return nil
	}
	var bucket model.QiniuBillingBucket
	if err := model.DB.First(&bucket, "id = ?", bucketId).Error; err != nil {
		return err
	}
	applyVersion := bucket.ApplyVersion + 1
	idempotencyKey := model.QiniuBillingBucketIdempotencyKey(bucket.Id, applyVersion)
	lastError := sanitizeQiniuTaskError(cause)
	now := common.GetTimestamp()
	nextRetryTime := qiniuCostDetailNextRetryTime(now)
	existing, exists, err := qiniuBillingBucketApplicationByIdempotencyTx(model.DB, idempotencyKey)
	if err != nil {
		return err
	}
	if exists {
		if existing.Status == model.QiniuBillingApplicationStatusSuccess {
			return nil
		}
		return model.DB.Model(&model.QiniuBillingBucketApplication{}).Where("id = ?", existing.Id).Updates(map[string]interface{}{
			"delta_quota":      bucket.PendingDeltaQuota,
			"delta_amount":     signedWalletAmount(bucket.PendingDeltaQuota),
			"status":           model.QiniuBillingApplicationStatusFailed,
			"last_error":       lastError,
			"retry_count":      gorm.Expr("retry_count + ?", 1),
			"last_retry_time":  now,
			"next_retry_time":  nextRetryTime,
			"operation_source": model.QiniuBillingOperationSourceSystem,
			"updated_time":     now,
		}).Error
	}
	return model.CreateQiniuBillingBucketApplication(model.DB, &model.QiniuBillingBucketApplication{
		BucketId:        bucket.Id,
		ApplyVersion:    applyVersion,
		DeltaQuota:      bucket.PendingDeltaQuota,
		DeltaAmount:     signedWalletAmount(bucket.PendingDeltaQuota),
		IdempotencyKey:  idempotencyKey,
		Status:          model.QiniuBillingApplicationStatusFailed,
		LastError:       lastError,
		RetryCount:      1,
		LastRetryTime:   now,
		NextRetryTime:   nextRetryTime,
		OperationSource: model.QiniuBillingOperationSourceSystem,
	})
}

func applyQiniuBucketWalletDeltaTx(tx *gorm.DB, bucket *model.QiniuBillingBucket, deltaQuota int, idempotencyKey string) (int, int, int, int, error) {
	if bucket == nil {
		return 0, 0, 0, 0, errors.New("账单 bucket 不能为空")
	}
	exists, err := walletFlowExistsTx(tx, idempotencyKey)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if exists {
		return 0, 0, 0, 0, errors.New("账单 bucket 钱包流水已存在")
	}
	account, err := ensureLegacyWalletBalanceTx(tx, bucket.UserId)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	balanceBeforeQuota := walletAmountToQuota(account.BalanceAmount)
	absQuota := int(math.Abs(float64(deltaQuota)))
	amount := quotaToWalletAmount(absQuota)
	direction := model.WalletFlowDirectionOut
	flowType := model.WalletFlowTypeBalanceConsume
	if deltaQuota > 0 {
		// cost-detail 是七牛侧已发生账单，补扣必须落账；余额不足时允许形成 delayed settlement debt。
		account.BalanceAmount -= amount
		if err := tx.Model(&model.User{}).Where("id = ?", bucket.UserId).Update("quota", gorm.Expr("quota - ?", deltaQuota)).Error; err != nil {
			return 0, 0, 0, 0, err
		}
	} else {
		direction = model.WalletFlowDirectionIn
		flowType = model.WalletFlowTypeBalanceRefund
		account.BalanceAmount += amount
		if err := tx.Model(&model.User{}).Where("id = ?", bucket.UserId).Update("quota", gorm.Expr("quota + ?", absQuota)).Error; err != nil {
			return 0, 0, 0, 0, err
		}
	}
	if err := tx.Save(account).Error; err != nil {
		return 0, 0, 0, 0, err
	}
	balanceAfterQuota := walletAmountToQuota(account.BalanceAmount)
	debtQuota := 0
	if balanceAfterQuota < 0 {
		debtQuota = -balanceAfterQuota
	}
	flow := &model.WalletFlow{
		UserId:                bucket.UserId,
		BizNo:                 idempotencyKey,
		IdempotencyKey:        walletIdempotencyKey(idempotencyKey),
		FlowType:              flowType,
		WalletType:            model.WalletTypeBalance,
		Direction:             direction,
		Amount:                amount,
		BalanceAfter:          account.BalanceAmount,
		CommissionAfter:       account.CommissionAmount,
		FrozenCommissionAfter: account.FrozenCommissionAmount,
		Remark:                qiniuBillingBucketWalletRemark(bucket, deltaQuota, debtQuota),
	}
	if err := createWalletFlowTx(tx, flow); err != nil {
		return 0, 0, 0, 0, err
	}
	flowId, err := getWalletFlowIdByIdempotencyTx(tx, idempotencyKey)
	return flowId, balanceBeforeQuota, balanceAfterQuota, debtQuota, err
}

func currentUserBalanceQuotaTx(tx *gorm.DB, userId int) (int, int, error) {
	var user model.User
	if err := tx.Select("id", "quota").Where("id = ?", userId).First(&user).Error; err != nil {
		return 0, 0, err
	}
	return user.Quota, user.Quota, nil
}

func createQiniuBucketSettlementLogTx(tx *gorm.DB, bucket *model.QiniuBillingBucket, applyVersion int, deltaQuota int, balanceAfterQuota int, debtQuota int) (int, error) {
	if bucket == nil || deltaQuota == 0 {
		return 0, nil
	}
	logType := model.LogTypeConsume
	content := "账单延迟对账补扣"
	if deltaQuota < 0 {
		logType = model.LogTypeRefund
		content = "账单延迟对账退款"
	}
	other := map[string]interface{}{
		"billing_source":        QiniuCostDetailBucketBillingSource,
		"bucket_id":             bucket.Id,
		"billing_date":          bucket.BillingDate,
		"qiniu_masked_key":      bucket.QiniuMaskedKey,
		"official_amount":       bucket.OfficialAmount,
		"official_quota":        bucket.OfficialQuota,
		"local_realtime_amount": quotaToWalletAmount(bucket.LocalRealtimeQuota),
		"local_realtime_quota":  bucket.LocalRealtimeQuota,
		"delta_quota":           deltaQuota,
		"delta_amount":          signedWalletAmount(deltaQuota),
		"apply_version":         applyVersion,
		"balance_after_quota":   balanceAfterQuota,
		"debt_quota":            debtQuota,
		"debt":                  debtQuota > 0,
	}
	log := &model.Log{
		UserId:    bucket.UserId,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
		ModelName: qiniuCostDetailBucketUserFacingModelName,
		Quota:     int(math.Abs(float64(deltaQuota))),
		TokenId:   bucket.TokenId,
		Other:     common.MapToJsonStr(other),
	}
	if err := tx.Create(log).Error; err != nil {
		return 0, err
	}
	return log.Id, nil
}

func qiniuBillingBucketWalletRemark(bucket *model.QiniuBillingBucket, deltaQuota int, debtQuota int) string {
	action := "调整"
	if deltaQuota > 0 {
		action = "补扣"
	} else if deltaQuota < 0 {
		action = "退款"
	}
	prefix := "账单延迟对账" + action
	if bucket.LocalRealtimeStatus == model.QiniuBillingLocalRealtimeStatusMissing {
		// 普通用户侧只说明这笔调整来自官方同步账单，不暴露供应商和内部修复标记。
		prefix = "官方同步" + prefix
	}
	parts := []string{
		prefix,
		fmt.Sprintf("date=%s", bucket.BillingDate),
		fmt.Sprintf("delta=%s", loggerSafeQuota(deltaQuota)),
	}
	if debtQuota > 0 {
		parts = append(parts, fmt.Sprintf("debt=%s", loggerSafeQuota(debtQuota)))
	}
	return strings.Join(parts, " ")
}

func walletAmountToQuota(amount float64) int {
	return int(decimal.NewFromFloat(amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
}

func loggerSafeQuota(quota int) string {
	return fmt.Sprintf("%d", quota)
}
