package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type QiniuOfficialLedgerApplyResult struct {
	ProcessedCount int
	AppliedCount   int
	SkippedCount   int
	FailedCount    int
	Errors         []string
}

const qiniuOfficialLedgerAutoApplyDisabledMessage = "旧 official ledger 自动调账已禁用；本 change 仅保留观测数据，后续补账模块处理"

func ApplyPendingQiniuOfficialLedgerRecords(ctx context.Context, limit int) (*QiniuOfficialLedgerApplyResult, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	result := &QiniuOfficialLedgerApplyResult{Errors: make([]string, 0)}
	return result, nil
}

func ApplyQiniuOfficialLedgerRecord(ctx context.Context, recordId int) (applied bool, skipped bool, err error) {
	if recordId <= 0 {
		return false, false, errors.New("官方账单记录 ID 无效")
	}
	if ctx != nil && ctx.Err() != nil {
		return false, false, ctx.Err()
	}
	return false, true, nil
}

func applyQiniuOfficialLedgerRecordLegacy(ctx context.Context, recordId int) (applied bool, skipped bool, err error) {
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		record, err := lockQiniuOfficialUsageRecordTx(tx, recordId)
		if err != nil {
			return err
		}
		if record.RecordType != model.QiniuOfficialRecordTypeBill {
			skipped = true
			return nil
		}
		if record.Status == model.QiniuOfficialRecordStatusApplied && record.OfficialQuota == record.AppliedQuota {
			skipped = true
			return nil
		}
		if record.Status == model.QiniuOfficialRecordStatusSkipped || record.Status == model.QiniuOfficialRecordStatusUnmapped {
			skipped = true
			return nil
		}
		if record.UserId <= 0 || record.TokenId <= 0 {
			return updateQiniuOfficialRecordStatusTx(tx, record.Id, model.QiniuOfficialRecordStatusUnmapped, "official_key_unmapped")
		}
		deltaQuota := record.OfficialQuota - record.AppliedQuota
		applyVersion := record.ApplyVersion + 1
		idempotencyKey := model.QiniuOfficialLedgerIdempotencyKey(record.Id, applyVersion)
		exists, err := qiniuOfficialLedgerApplicationExistsTx(tx, idempotencyKey)
		if err != nil || exists {
			skipped = exists
			return err
		}
		walletFlowId := 0
		if deltaQuota > 0 {
			if err := debitBalanceByQuotaTx(tx, record.UserId, deltaQuota, idempotencyKey, model.WalletFlowTypeBalanceConsume, qiniuOfficialLedgerRemark(record, deltaQuota), idempotencyKey); err != nil {
				return err
			}
			walletFlowId, err = getWalletFlowIdByIdempotencyTx(tx, idempotencyKey)
			if err != nil {
				return err
			}
			if err := applyUserUsedQuotaDeltaTx(tx, record.UserId, deltaQuota); err != nil {
				return err
			}
			if err := applyTokenUsedQuotaDeltaTx(tx, record.TokenId, deltaQuota); err != nil {
				return err
			}
		} else if deltaQuota < 0 {
			refundQuota := -deltaQuota
			if err := creditBalanceByQuotaTx(tx, record.UserId, refundQuota, idempotencyKey, model.WalletFlowTypeBalanceRefund, qiniuOfficialLedgerRemark(record, deltaQuota), idempotencyKey); err != nil {
				return err
			}
			walletFlowId, err = getWalletFlowIdByIdempotencyTx(tx, idempotencyKey)
			if err != nil {
				return err
			}
			if err := applyUserUsedQuotaDeltaTx(tx, record.UserId, deltaQuota); err != nil {
				return err
			}
			if err := applyTokenUsedQuotaDeltaTx(tx, record.TokenId, deltaQuota); err != nil {
				return err
			}
		}
		if deltaQuota != 0 {
			if err := ensureQiniuQuotaSyncTaskForOfficialLedgerTx(tx, record); err != nil {
				return err
			}
		}
		application := &model.QiniuOfficialLedgerApplication{
			UsageRecordId:  record.Id,
			ApplyVersion:   applyVersion,
			UserId:         record.UserId,
			TokenId:        record.TokenId,
			DeltaQuota:     deltaQuota,
			DeltaAmount:    signedWalletAmount(deltaQuota),
			WalletFlowId:   walletFlowId,
			IdempotencyKey: idempotencyKey,
			Status:         model.QiniuOfficialLedgerStatusSuccess,
		}
		if err := model.CreateQiniuOfficialLedgerApplication(tx, application); err != nil {
			return err
		}
		if err := tx.Model(&model.QiniuOfficialUsageRecord{}).
			Where("id = ?", record.Id).
			Updates(map[string]interface{}{
				"applied_quota": record.OfficialQuota,
				"apply_version": applyVersion,
				"status":        model.QiniuOfficialRecordStatusApplied,
				"last_error":    "",
				"updated_time":  common.GetTimestamp(),
			}).Error; err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil {
		_ = markQiniuOfficialLedgerRecordFailed(recordId, err)
	} else if applied {
		if logErr := ensureQiniuOfficialLedgerLog(recordId); logErr != nil {
			common.SysLog(fmt.Sprintf("failed to create qiniu official ledger log record_id=%d error=%s", recordId, sanitizeQiniuTaskError(logErr)))
		}
	}
	return applied, skipped, err
}

func ensureQiniuQuotaSyncTaskForOfficialLedgerTx(tx *gorm.DB, record *model.QiniuOfficialUsageRecord) error {
	if tx == nil {
		tx = model.DB
	}
	if record == nil || record.UserId <= 0 || record.TokenId <= 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&model.QiniuKeySyncTask{}).
		Where("user_id = ? AND token_id = ? AND task_type = ? AND status IN ?", record.UserId, record.TokenId, model.QiniuKeyTaskTypeQuotaSync, []string{
			model.QiniuKeyTaskStatusPending,
			model.QiniuKeyTaskStatusRunning,
			model.QiniuKeyTaskStatusFailed,
		}).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return tx.Create(&model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeQuotaSync,
		UserId:   record.UserId,
		TokenId:  record.TokenId,
		QiniuKey: strings.TrimPrefix(fullQiniuAPIKey(record.QiniuKey), "sk-"),
		Status:   model.QiniuKeyTaskStatusPending,
		Payload:  fmt.Sprintf(`{"source":"qiniu_official_ledger","usage_record_id":%d}`, record.Id),
	}).Error
}

func lockQiniuOfficialUsageRecordTx(tx *gorm.DB, recordId int) (*model.QiniuOfficialUsageRecord, error) {
	query := tx
	if !common.UsingSQLite {
		query = tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var record model.QiniuOfficialUsageRecord
	if err := query.Where("id = ?", recordId).First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func updateQiniuOfficialRecordStatusTx(tx *gorm.DB, recordId int, status string, lastError string) error {
	return tx.Model(&model.QiniuOfficialUsageRecord{}).
		Where("id = ?", recordId).
		Updates(map[string]interface{}{
			"status":       status,
			"last_error":   lastError,
			"updated_time": common.GetTimestamp(),
		}).Error
}

func markQiniuOfficialLedgerRecordFailed(recordId int, err error) error {
	return updateQiniuOfficialRecordStatusTx(model.DB, recordId, model.QiniuOfficialRecordStatusFailed, sanitizeQiniuTaskError(err))
}

func qiniuOfficialLedgerApplicationExistsTx(tx *gorm.DB, idempotencyKey string) (bool, error) {
	var count int64
	err := tx.Model(&model.QiniuOfficialLedgerApplication{}).Where("idempotency_key = ?", idempotencyKey).Count(&count).Error
	return count > 0, err
}

func getWalletFlowIdByIdempotencyTx(tx *gorm.DB, idempotencyKey string) (int, error) {
	if idempotencyKey == "" {
		return 0, nil
	}
	var flow model.WalletFlow
	if err := tx.Select("id").Where("idempotency_key = ?", idempotencyKey).First(&flow).Error; err != nil {
		return 0, err
	}
	return flow.Id, nil
}

func applyUserUsedQuotaDeltaTx(tx *gorm.DB, userId int, deltaQuota int) error {
	if deltaQuota == 0 {
		return nil
	}
	var user model.User
	query := tx
	if !common.UsingSQLite {
		query = tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Select("id", "used_quota").Where("id = ?", userId).First(&user).Error; err != nil {
		return err
	}
	nextUsedQuota := user.UsedQuota + deltaQuota
	if nextUsedQuota < 0 {
		nextUsedQuota = 0
	}
	return tx.Model(&model.User{}).Where("id = ?", userId).Update("used_quota", nextUsedQuota).Error
}

func applyTokenUsedQuotaDeltaTx(tx *gorm.DB, tokenId int, deltaQuota int) error {
	if deltaQuota == 0 {
		return nil
	}
	var token model.Token
	// 官方账单可能在 Key 删除后才到达，软删除 Token 仍要按官方 ledger 修正用量。
	query := tx.Unscoped()
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Select("id", "used_quota").Where("id = ?", tokenId).First(&token).Error; err != nil {
		return err
	}
	nextUsedQuota := token.UsedQuota + deltaQuota
	if nextUsedQuota < 0 {
		nextUsedQuota = 0
	}
	return tx.Unscoped().Model(&model.Token{}).Where("id = ?", tokenId).Updates(map[string]interface{}{
		"used_quota":    nextUsedQuota,
		"accessed_time": common.GetTimestamp(),
	}).Error
}

func qiniuOfficialLedgerFailedRetryBefore() int64 {
	interval := operation_setting.GetQiniuKeySetting().OfficialLedgerRetryIntervalSeconds
	if interval <= 0 {
		interval = operation_setting.QiniuOfficialLedgerDefaultRetryInterval
	}
	return common.GetTimestamp() - int64(interval)
}

func signedWalletAmount(deltaQuota int) float64 {
	if deltaQuota == 0 {
		return 0
	}
	amount := quotaToWalletAmount(int(math.Abs(float64(deltaQuota))))
	if deltaQuota < 0 {
		return -amount
	}
	return amount
}

func qiniuOfficialLedgerRemark(record *model.QiniuOfficialUsageRecord, deltaQuota int) string {
	action := "消费"
	if deltaQuota < 0 {
		action = "退款"
	}
	return fmt.Sprintf("官方用量同步%s：%s/%s", action, record.ModelName, record.BillingItem)
}
