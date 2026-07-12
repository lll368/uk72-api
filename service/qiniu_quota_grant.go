package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	qiniuQuotaGrantRetryTickInterval = 1 * time.Minute
	qiniuQuotaGrantRetryBatchSize    = 100
)

var (
	qiniuQuotaGrantApplyOnce    sync.Once
	qiniuQuotaGrantApplyRunning atomic.Bool
)

type QiniuQuotaGrantApplyResult struct {
	ProcessedCount int
	AppliedCount   int
	SkippedCount   int
	FailedCount    int
	Errors         []string
}

func CreateQiniuQuotaGrantForRecharge(userId int, tradeNo string, grantQuota int) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if userId <= 0 || tradeNo == "" || grantQuota <= 0 {
		return nil
	}
	return createQiniuQuotaGrantTx(nil, userId, "recharge:"+tradeNo, quotaToWalletAmount(grantQuota))
}

func CreateQiniuQuotaGrantForCommissionTransfer(userId int, walletFlowId int, amount float64) error {
	if userId <= 0 || walletFlowId <= 0 || amount <= 0 {
		return nil
	}
	return createQiniuQuotaGrantTx(nil, userId, fmt.Sprintf("commission_transfer:%d", walletFlowId), amount)
}

func qiniuInitialQuotaBaselineBusinessKey(tokenId int) string {
	return fmt.Sprintf("key_initial_balance:%d", tokenId)
}

func createQiniuInitialQuotaBaselineTx(tx *gorm.DB, userId int, tokenId int, amount float64) error {
	if userId <= 0 || tokenId <= 0 {
		return errors.New("七牛初始额度 baseline 缺少用户或 token")
	}
	if amount < 0 {
		amount = 0
	}
	if tx == nil {
		tx = model.DB
	}
	businessKey := qiniuInitialQuotaBaselineBusinessKey(tokenId)
	var existing model.QiniuQuotaGrant
	err := tx.Where("business_key = ?", businessKey).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	now := common.GetTimestamp()
	return tx.Create(&model.QiniuQuotaGrant{
		UserId:            userId,
		TokenId:           tokenId,
		BusinessKey:       businessKey,
		GrantAmount:       amount,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusApplied,
		RemoteApplyTime:   now,
	}).Error
}

func createQiniuQuotaGrantForRechargeTx(tx *gorm.DB, userId int, tradeNo string, grantQuota int) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if userId <= 0 || tradeNo == "" || grantQuota <= 0 {
		return nil
	}
	return createQiniuQuotaGrantTx(tx, userId, "recharge:"+tradeNo, quotaToWalletAmount(grantQuota))
}

func createQiniuQuotaGrantForCommissionTransferTx(tx *gorm.DB, userId int, walletFlowId int, amount float64) error {
	if userId <= 0 || walletFlowId <= 0 || amount <= 0 {
		return nil
	}
	return createQiniuQuotaGrantTx(tx, userId, fmt.Sprintf("commission_transfer:%d", walletFlowId), amount)
}

func createQiniuQuotaGrantTx(tx *gorm.DB, userId int, businessKey string, grantAmount float64) error {
	if grantAmount <= 0 || !IsQiniuKeyLifecycleEnabled() {
		return nil
	}
	if tx == nil {
		tx = model.DB
	}
	tokenId := 0
	var token model.Token
	err := tx.Where("user_id = ? AND status = ?", userId, common.TokenStatusEnabled).Order("id asc").First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysLog(fmt.Sprintf("qiniu quota grant deferred user_id=%d business_key=%s reason=no_enabled_token", userId, businessKey))
		} else {
			return err
		}
	} else if !IsQiniuManagedToken(&token) {
		common.SysLog(fmt.Sprintf("qiniu quota grant skipped user_id=%d token_id=%d reason=legacy_local_key", userId, token.Id))
		return nil
	} else {
		tokenId = token.Id
	}
	grant := &model.QiniuQuotaGrant{
		UserId:            userId,
		TokenId:           tokenId,
		BusinessKey:       businessKey,
		GrantAmount:       grantAmount,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusPending,
	}
	var existing model.QiniuQuotaGrant
	err = tx.Where("business_key = ?", businessKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(grant).Error
	}
	return err
}

func StartQiniuQuotaGrantApplyTask() {
	qiniuQuotaGrantApplyOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog(fmt.Sprintf("qiniu quota grant apply task started: tick=%s", qiniuQuotaGrantRetryTickInterval))
			ticker := time.NewTicker(qiniuQuotaGrantRetryTickInterval)
			defer ticker.Stop()
			runQiniuQuotaGrantApplyOnce()
			for range ticker.C {
				runQiniuQuotaGrantApplyOnce()
			}
		})
	})
}

func runQiniuQuotaGrantApplyOnce() {
	if !IsQiniuKeyLifecycleEnabled() {
		return
	}
	if !qiniuQuotaGrantApplyRunning.CompareAndSwap(false, true) {
		return
	}
	defer qiniuQuotaGrantApplyRunning.Store(false)
	result, err := ScanPendingQiniuQuotaGrants(context.Background(), qiniuQuotaGrantRetryBatchSize)
	if err != nil {
		common.SysLog("qiniu quota grant apply scan failed: " + sanitizeQiniuTaskError(err))
		return
	}
	if result.ProcessedCount > 0 || len(result.Errors) > 0 {
		common.SysLog(fmt.Sprintf(
			"qiniu quota grant apply scan finished processed=%d applied=%d skipped=%d failed=%d errors=%d",
			result.ProcessedCount,
			result.AppliedCount,
			result.SkippedCount,
			result.FailedCount,
			len(result.Errors),
		))
	}
}

func ScanPendingQiniuQuotaGrants(ctx context.Context, limit int) (*QiniuQuotaGrantApplyResult, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if limit <= 0 || limit > qiniuQuotaGrantRetryBatchSize {
		limit = qiniuQuotaGrantRetryBatchSize
	}
	result := &QiniuQuotaGrantApplyResult{Errors: make([]string, 0)}
	var grants []model.QiniuQuotaGrant
	if err := model.DB.Where(
		"remote_apply_status = ? OR (remote_apply_status = ? AND updated_time <= ?)",
		model.QiniuQuotaGrantStatusPending,
		model.QiniuQuotaGrantStatusFailed,
		qiniuQuotaGrantFailedRetryBefore(),
	).Order("id asc").Limit(limit).Find(&grants).Error; err != nil {
		return nil, err
	}
	for _, grant := range grants {
		if ctx != nil && ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.ProcessedCount++
		applied, skipped, err := ApplyQiniuQuotaGrant(ctx, grant.Id)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
			continue
		}
		if applied {
			result.AppliedCount++
		}
		if skipped {
			result.SkippedCount++
		}
	}
	return result, nil
}

func ApplyQiniuQuotaGrant(ctx context.Context, grantId int) (applied bool, skipped bool, err error) {
	if grantId <= 0 {
		return false, false, errors.New("quota grant ID 无效")
	}
	if ctx != nil && ctx.Err() != nil {
		return false, false, ctx.Err()
	}
	var grant model.QiniuQuotaGrant
	if err := model.DB.First(&grant, "id = ?", grantId).Error; err != nil {
		return false, false, err
	}
	if grant.RemoteApplyStatus == model.QiniuQuotaGrantStatusApplied {
		return false, true, nil
	}
	token, err := qiniuQuotaGrantToken(&grant)
	if err != nil {
		_ = markQiniuQuotaGrantFailed(grant.Id, err)
		return false, false, err
	}
	targetLimit, err := qiniuQuotaGrantTargetLimit(token.Id, grant.Id, grant.GrantAmount)
	if err != nil {
		_ = markQiniuQuotaGrantFailed(grant.Id, err)
		return false, false, err
	}
	client, err := NewQiniuAccountIdentityClient(token.QiniuChildAccountId, QiniuAccountOperationQuota)
	if err != nil {
		_ = markQiniuQuotaGrantFailed(grant.Id, err)
		return false, false, err
	}
	if err := client.SetAPIKeyTotalQuota(ctx, token.Key, targetLimit); err != nil {
		_ = markQiniuQuotaGrantFailed(grant.Id, err)
		return false, false, err
	}
	if err := model.DB.Model(&model.QiniuQuotaGrant{}).Where("id = ?", grant.Id).Updates(map[string]interface{}{
		"token_id":            token.Id,
		"remote_apply_status": model.QiniuQuotaGrantStatusApplied,
		"remote_apply_time":   common.GetTimestamp(),
		"last_error":          "",
		"updated_time":        common.GetTimestamp(),
	}).Error; err != nil {
		return false, false, err
	}
	return true, false, nil
}

func qiniuQuotaGrantFailedRetryBefore() int64 {
	interval := operation_setting.GetQiniuKeySetting().OfficialLedgerRetryIntervalSeconds
	if interval <= 0 {
		interval = operation_setting.QiniuOfficialLedgerDefaultRetryInterval
	}
	return common.GetTimestamp() - int64(interval)
}

func qiniuQuotaGrantToken(grant *model.QiniuQuotaGrant) (*model.Token, error) {
	if grant == nil {
		return nil, errors.New("quota grant 不能为空")
	}
	var token *model.Token
	var err error
	if grant.TokenId > 0 {
		var loaded model.Token
		err = model.DB.First(&loaded, "id = ?", grant.TokenId).Error
		if err == nil && loaded.IsQiniuManaged() && loaded.Status == common.TokenStatusEnabled {
			return &loaded, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	token, err = model.GetFirstEnabledUserToken(grant.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("quota grant 暂无可用 token")
		}
		return nil, err
	}
	if !token.IsQiniuManaged() || token.Status != common.TokenStatusEnabled {
		return nil, errors.New("quota grant 对应 token 不可用")
	}
	return token, nil
}

func qiniuQuotaGrantTargetLimit(tokenId int, currentGrantId int, currentGrantAmount float64) (float64, error) {
	var appliedAmount float64
	if err := model.DB.Model(&model.QiniuQuotaGrant{}).
		Select("COALESCE(SUM(grant_amount), 0)").
		Where("token_id = ? AND remote_apply_status = ? AND id <> ?", tokenId, model.QiniuQuotaGrantStatusApplied, currentGrantId).
		Scan(&appliedAmount).Error; err != nil {
		return 0, err
	}
	targetLimit := appliedAmount + currentGrantAmount
	if targetLimit < 0 {
		return 0, nil
	}
	return targetLimit, nil
}

func markQiniuQuotaGrantFailed(grantId int, err error) error {
	if err == nil {
		return nil
	}
	return model.DB.Model(&model.QiniuQuotaGrant{}).Where("id = ?", grantId).Updates(map[string]interface{}{
		"remote_apply_status": model.QiniuQuotaGrantStatusFailed,
		"last_error":          sanitizeQiniuTaskError(err),
		"updated_time":        common.GetTimestamp(),
	}).Error
}
