package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const qiniuOwnershipConfirmationMaxCandidates = 5

const (
	qiniuCostDetailCutoverNotConfiguredReason = "cutover_not_configured"
	qiniuCostDetailBeforeCutoverReason        = "before_cutover"
	qiniuCostDetailAutoApplyDisabledReason    = "auto_apply_disabled"
	qiniuCostDetailInvalidBillingDateReason   = "invalid_billing_date"
)

const (
	QiniuBillingLocalRealtimeStatusFound   = model.QiniuBillingLocalRealtimeStatusFound
	QiniuBillingLocalRealtimeStatusMissing = model.QiniuBillingLocalRealtimeStatusMissing
	QiniuBillingLocalRealtimeStatusError   = model.QiniuBillingLocalRealtimeStatusError
)

type QiniuCostDetailOwnershipResult struct {
	RecordId            int
	OwnerStatus         string
	UserId              int
	TokenId             int
	QiniuChildAccountId int
	CandidateCount      int
	ConfirmedCount      int
	BucketId            int
	LastError           string
}

type QiniuManualOwnershipInput struct {
	RawRecordId int
	TokenId     int
	AdminUserId int
	Reason      string
}

type QiniuManualBucketOwnershipInput struct {
	BucketId    int
	TokenId     int
	AdminUserId int
	Reason      string
}

func ResolveQiniuCostDetailRecordOwnership(ctx context.Context, recordId int) (*QiniuCostDetailOwnershipResult, error) {
	if recordId <= 0 {
		return nil, errors.New("cost-detail 原始记录 ID 无效")
	}
	var record model.QiniuCostDetailRecord
	if err := model.DB.First(&record, "id = ?", recordId).Error; err != nil {
		return nil, err
	}
	result := &QiniuCostDetailOwnershipResult{
		RecordId:            record.Id,
		OwnerStatus:         record.OwnerStatus,
		UserId:              record.UserId,
		TokenId:             record.TokenId,
		QiniuChildAccountId: record.QiniuChildAccountId,
	}
	if record.OwnerStatus == model.QiniuBillingOwnerStatusManualResolved && record.TokenId > 0 {
		bucket, err := RecalculateQiniuBillingBucket(record.TokenId, record.BillingDate)
		if err != nil {
			return nil, err
		}
		if bucket != nil {
			result.BucketId = bucket.Id
		}
		return result, nil
	}
	candidates, err := model.ListQiniuManagedTokensByKeyAffixes(record.KeyPrefix, record.KeySuffix, qiniuOwnershipConfirmationMaxCandidates+1)
	if err != nil {
		return nil, err
	}
	result.CandidateCount = len(candidates)
	switch len(candidates) {
	case 0:
		return updateQiniuCostDetailOwnership(&record, nil, model.QiniuBillingOwnerStatusUnmapped, result)
	case 1:
		return updateQiniuCostDetailOwnership(&record, &candidates[0], model.QiniuBillingOwnerStatusResolved, result)
	default:
		confirmed, confirmErr := confirmQiniuCostDetailOwnership(ctx, &record, candidates)
		result.ConfirmedCount = len(confirmed)
		if confirmErr != nil {
			result.LastError = sanitizeQiniuTaskError(confirmErr)
		}
		if len(confirmed) == 1 {
			return updateQiniuCostDetailOwnership(&record, &confirmed[0], model.QiniuBillingOwnerStatusResolved, result)
		}
		return updateQiniuCostDetailOwnership(&record, nil, model.QiniuBillingOwnerStatusAmbiguous, result)
	}
}

func ManuallyResolveQiniuCostDetailRawRecordOwnership(input QiniuManualOwnershipInput) (*model.QiniuBillingBucket, error) {
	if input.RawRecordId <= 0 {
		return nil, errors.New("cost-detail 原始记录 ID 无效")
	}
	if input.TokenId <= 0 {
		return nil, errors.New("token ID 无效")
	}
	reason := strings.TrimSpace(input.Reason)
	var bucket *model.QiniuBillingBucket
	var token model.Token
	var record model.QiniuCostDetailRecord
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().First(&token, "id = ?", input.TokenId).Error; err != nil {
			return err
		}
		if !token.IsQiniuManaged() {
			return errors.New("只能人工归属到托管 token")
		}
		if err := tx.First(&record, "id = ?", input.RawRecordId).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{
			"owner_status":           model.QiniuBillingOwnerStatusManualResolved,
			"user_id":                token.UserId,
			"token_id":               token.Id,
			"qiniu_child_account_id": token.QiniuChildAccountId,
			"updated_time":           common.GetTimestamp(),
		}
		if err := tx.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", record.Id).Updates(updates).Error; err != nil {
			return err
		}
		record.OwnerStatus = model.QiniuBillingOwnerStatusManualResolved
		record.UserId = token.UserId
		record.TokenId = token.Id
		record.QiniuChildAccountId = token.QiniuChildAccountId
		return nil
	})
	if err != nil {
		return nil, err
	}
	bucket, err = RecalculateQiniuBillingBucket(token.Id, record.BillingDate)
	if err != nil {
		return nil, err
	}
	model.RecordLogWithAdminInfo(token.UserId, model.LogTypeManage, "管理员人工归属 cost-detail 明细", map[string]interface{}{
		"admin_user_id": input.AdminUserId,
		"raw_record_id": input.RawRecordId,
		"token_id":      input.TokenId,
		"bucket_id":     bucketIdForLog(bucket),
		"reason":        reason,
	})
	return bucket, nil
}

func ManuallyResolveQiniuBillingBucketOwnership(input QiniuManualBucketOwnershipInput) (*model.QiniuBillingBucket, error) {
	if input.BucketId <= 0 {
		return nil, errors.New("账单 bucket ID 无效")
	}
	if input.TokenId <= 0 {
		return nil, errors.New("token ID 无效")
	}
	reason := strings.TrimSpace(input.Reason)
	var token model.Token
	var sourceBucket model.QiniuBillingBucket
	var reuseSourceBucket bool
	var rawRecordIds []int
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().First(&token, "id = ?", input.TokenId).Error; err != nil {
			return err
		}
		if !token.IsQiniuManaged() {
			return errors.New("只能人工归属到托管 token")
		}
		if err := tx.First(&sourceBucket, "id = ?", input.BucketId).Error; err != nil {
			return err
		}
		ids, err := qiniuBillingBucketRawRecordIdsTx(tx, &sourceBucket)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return errors.New("账单 bucket 缺少可人工归属的 cost-detail 明细")
		}
		rawRecordIds = ids
		updates := map[string]interface{}{
			"owner_status":           model.QiniuBillingOwnerStatusManualResolved,
			"user_id":                token.UserId,
			"token_id":               token.Id,
			"qiniu_child_account_id": token.QiniuChildAccountId,
			"updated_time":           common.GetTimestamp(),
		}
		result := tx.Model(&model.QiniuCostDetailRecord{}).
			Where("id IN ?", ids).
			Where("owner_status IN ?", []string{
				model.QiniuBillingOwnerStatusUnmapped,
				model.QiniuBillingOwnerStatusAmbiguous,
			}).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("账单 bucket 没有未归属或冲突归属的 cost-detail 明细")
		}
		var target model.QiniuBillingBucket
		err = tx.Select("id").Where("token_id = ? AND billing_date = ?", token.Id, sourceBucket.BillingDate).First(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			reuseSourceBucket = true
			return tx.Model(&model.QiniuBillingBucket{}).Where("id = ?", sourceBucket.Id).Updates(map[string]interface{}{
				"user_id":                token.UserId,
				"token_id":               token.Id,
				"qiniu_child_account_id": token.QiniuChildAccountId,
				"owner_status":           model.QiniuBillingOwnerStatusManualResolved,
				"status":                 model.QiniuBillingBucketStatusPending,
				"last_error":             "",
				"key_fingerprint":        model.QiniuTokenKeyFingerprint(token.Key),
				"updated_time":           common.GetTimestamp(),
			}).Error
		}
		if err != nil {
			return err
		}
		reuseSourceBucket = target.Id == sourceBucket.Id
		return nil
	})
	if err != nil {
		return nil, err
	}
	bucket, err := RecalculateQiniuBillingBucket(token.Id, sourceBucket.BillingDate)
	if err != nil {
		return nil, err
	}
	if !reuseSourceBucket && bucket != nil && bucket.Id != sourceBucket.Id {
		_ = model.DB.Model(&model.QiniuBillingBucket{}).Where("id = ?", sourceBucket.Id).Updates(map[string]interface{}{
			"status":       model.QiniuBillingBucketStatusSkipped,
			"last_error":   fmt.Sprintf("manual_resolved_to_bucket:%d", bucket.Id),
			"updated_time": common.GetTimestamp(),
		}).Error
	}
	model.RecordLogWithAdminInfo(token.UserId, model.LogTypeManage, "管理员人工归属 cost-detail bucket", map[string]interface{}{
		"admin_user_id":  input.AdminUserId,
		"source_bucket":  input.BucketId,
		"bucket_id":      bucketIdForLog(bucket),
		"raw_record_ids": rawRecordIds,
		"token_id":       input.TokenId,
		"reason":         reason,
	})
	return bucket, nil
}

func updateQiniuCostDetailOwnership(record *model.QiniuCostDetailRecord, token *model.Token, ownerStatus string, result *QiniuCostDetailOwnershipResult) (*QiniuCostDetailOwnershipResult, error) {
	if record == nil {
		return nil, errors.New("cost-detail 原始记录不能为空")
	}
	userId := 0
	tokenId := 0
	qiniuChildAccountId := 0
	if token != nil {
		userId = token.UserId
		tokenId = token.Id
		qiniuChildAccountId = token.QiniuChildAccountId
	}
	updates := map[string]interface{}{
		"owner_status":           ownerStatus,
		"user_id":                userId,
		"token_id":               tokenId,
		"qiniu_child_account_id": qiniuChildAccountId,
		"updated_time":           common.GetTimestamp(),
	}
	if ownerStatus == model.QiniuBillingOwnerStatusUnmapped || ownerStatus == model.QiniuBillingOwnerStatusAmbiguous {
		now := common.GetTimestamp()
		updates["retry_count"] = gorm.Expr("retry_count + ?", 1)
		updates["last_retry_time"] = now
		updates["next_retry_time"] = qiniuCostDetailNextRetryTime(now)
		updates["last_error"] = ownerStatus
		if result != nil && strings.TrimSpace(result.LastError) != "" {
			updates["last_error"] = result.LastError
		}
	} else {
		updates["next_retry_time"] = 0
		updates["last_error"] = ""
	}
	if err := model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", record.Id).Updates(updates).Error; err != nil {
		return nil, err
	}
	result.OwnerStatus = ownerStatus
	result.UserId = userId
	result.TokenId = tokenId
	result.QiniuChildAccountId = qiniuChildAccountId
	if token != nil {
		bucket, err := RecalculateQiniuBillingBucket(token.Id, record.BillingDate)
		if err != nil {
			return nil, err
		}
		if bucket != nil {
			result.BucketId = bucket.Id
		}
	}
	return result, nil
}

func confirmQiniuCostDetailOwnership(ctx context.Context, record *model.QiniuCostDetailRecord, candidates []model.Token) ([]model.Token, error) {
	if record == nil || len(candidates) == 0 {
		return nil, nil
	}
	if len(candidates) > qiniuOwnershipConfirmationMaxCandidates || !IsQiniuCostDetailSyncEnabled() {
		return nil, nil
	}
	client, err := newQiniuKeyClient(operation_setting.GetQiniuKeySetting())
	if err != nil {
		return nil, err
	}
	billingDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(record.BillingDate), qiniuCSTLocation)
	if err != nil {
		return nil, err
	}
	confirmed := make([]model.Token, 0, 1)
	var lastErr error
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return confirmed, ctx.Err()
		}
		items, err := client.QueryOfficialCostDetails(ctx, qiniuOfficialCostDetailQuery{
			StartDate: billingDate,
			EndDate:   billingDate,
			Grain:     "day",
			APIKey:    candidate.Key,
		})
		if err != nil {
			lastErr = err
			continue
		}
		if qiniuCostDetailItemsContainRecord(items, record) {
			confirmed = append(confirmed, candidate)
		}
	}
	return confirmed, lastErr
}

func qiniuCostDetailItemsContainRecord(items []qiniuOfficialCostDetailItem, record *model.QiniuCostDetailRecord) bool {
	for _, item := range items {
		if !qiniuCostDetailItemMatchesRecord(item, record) {
			continue
		}
		return true
	}
	return false
}

func qiniuCostDetailItemMatchesRecord(item qiniuOfficialCostDetailItem, record *model.QiniuCostDetailRecord) bool {
	if record == nil {
		return false
	}
	if item.PeriodStart > 0 {
		itemDate := time.Unix(item.PeriodStart, 0).In(qiniuCSTLocation).Format("2006-01-02")
		if itemDate != strings.TrimSpace(record.BillingDate) {
			return false
		}
	}
	if strings.TrimSpace(item.ModelName) != strings.TrimSpace(record.ModelName) {
		return false
	}
	if strings.TrimSpace(item.BillingItem) != strings.TrimSpace(record.BillingItem) {
		return false
	}
	return math.Abs(item.FeeAmount-record.FeeAmount) <= walletAmountEpsilon
}

func RecalculateQiniuBillingBucket(tokenId int, billingDate string) (*model.QiniuBillingBucket, error) {
	if tokenId <= 0 {
		return nil, errors.New("token ID 无效")
	}
	if strings.TrimSpace(billingDate) == "" {
		return nil, errors.New("账单日期不能为空")
	}
	localRealtimeQuota, localRealtimeStatus, err := summarizeQiniuLocalRealtimeQuota(tokenId, billingDate)
	if err != nil {
		localRealtimeStatus = model.QiniuBillingLocalRealtimeStatusError
	}
	var bucket *model.QiniuBillingBucket
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		recalculated, err := recalculateQiniuBillingBucketTx(tx, tokenId, billingDate, localRealtimeQuota, localRealtimeStatus)
		if err != nil {
			return err
		}
		bucket = recalculated
		return nil
	})
	if err != nil || bucket == nil {
		return bucket, err
	}
	if operation_setting.GetQiniuKeySetting().CostDetailAutoApplyEnabled && bucket.Status == model.QiniuBillingBucketStatusPending {
		_, _, applyErr := ApplyQiniuBillingBucket(context.Background(), bucket.Id)
		if applyErr != nil {
			return bucket, applyErr
		}
		if reloadErr := model.DB.First(bucket, "id = ?", bucket.Id).Error; reloadErr != nil {
			return bucket, reloadErr
		}
	}
	return bucket, nil
}

func recalculateQiniuBillingBucketTx(tx *gorm.DB, tokenId int, billingDate string, localRealtimeQuota int, localRealtimeStatus string) (*model.QiniuBillingBucket, error) {
	if tx == nil {
		tx = model.DB
	}
	var token model.Token
	if err := tx.Unscoped().First(&token, "id = ?", tokenId).Error; err != nil {
		return nil, err
	}
	if !token.IsQiniuManaged() {
		return nil, errors.New("只能为托管 token 重算 bucket")
	}
	var records []model.QiniuCostDetailRecord
	err := tx.Where("token_id = ? AND billing_date = ? AND owner_status IN ?", tokenId, billingDate, []string{
		model.QiniuBillingOwnerStatusResolved,
		model.QiniuBillingOwnerStatusManualResolved,
	}).Order("id asc").Find(&records).Error
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("bucket 重算缺少已归属 cost-detail 明细")
	}
	officialAmount := 0.0
	rawRecordIds := make(map[string][]string)
	itemUsage := make(map[string]float64)
	itemFee := make(map[string]float64)
	itemCurrency := make(map[string]string)
	maskedKey := records[0].QiniuMaskedKey
	ownerStatus := model.QiniuBillingOwnerStatusResolved
	for _, record := range records {
		officialAmount += record.FeeAmount
		if record.OwnerStatus == model.QiniuBillingOwnerStatusManualResolved {
			ownerStatus = model.QiniuBillingOwnerStatusManualResolved
		}
		itemKey := qiniuBucketItemKey(record.ModelName, record.BillingItem)
		itemUsage[itemKey] += record.UsageCount
		itemFee[itemKey] += record.FeeAmount
		itemCurrency[itemKey] = record.Currency
		rawRecordIds[itemKey] = append(rawRecordIds[itemKey], strconv.Itoa(record.Id))
		if maskedKey == "" {
			maskedKey = record.QiniuMaskedKey
		}
	}
	officialQuota := qiniuCostDetailAmountToQuota(officialAmount)
	identity := model.BuildQiniuTokenKeyIdentity(token.Key)

	var bucket model.QiniuBillingBucket
	err = tx.Where("token_id = ? AND billing_date = ?", tokenId, billingDate).First(&bucket).Error
	insert := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !insert {
		return nil, err
	}
	if insert {
		bucket = model.QiniuBillingBucket{
			UserId:              token.UserId,
			TokenId:             token.Id,
			QiniuChildAccountId: token.QiniuChildAccountId,
			BillingDate:         billingDate,
			QiniuMaskedKey:      maskedKey,
			KeyFingerprint:      identity.KeyFingerprint,
			OwnerStatus:         model.QiniuBillingOwnerStatusResolved,
		}
	} else {
		bucket.PreviousOfficialAmount = bucket.OfficialAmount
		bucket.PreviousOfficialQuota = bucket.OfficialQuota
	}
	bucket.UserId = token.UserId
	bucket.QiniuChildAccountId = token.QiniuChildAccountId
	bucket.QiniuMaskedKey = maskedKey
	bucket.KeyFingerprint = identity.KeyFingerprint
	bucket.OwnerStatus = ownerStatus
	bucket.OfficialAmount = officialAmount
	bucket.OfficialQuota = officialQuota
	bucket.LocalRealtimeQuota = localRealtimeQuota
	bucket.LocalRealtimeStatus = localRealtimeStatus
	bucket.PendingDeltaQuota = officialQuota - localRealtimeQuota - bucket.AppliedDeltaQuota
	bucket.Status, bucket.LastError = qiniuBillingBucketStatusForPendingDelta(bucket.BillingDate, bucket.PendingDeltaQuota)
	if bucket.LastError != "" {
		bucket.RetryCount++
		bucket.LastRetryTime = common.GetTimestamp()
		bucket.NextRetryTime = qiniuCostDetailNextRetryTime(bucket.LastRetryTime)
	} else {
		bucket.NextRetryTime = 0
	}
	if insert {
		if err := tx.Create(&bucket).Error; err != nil {
			return nil, err
		}
	} else if err := tx.Save(&bucket).Error; err != nil {
		return nil, err
	}
	if err := upsertQiniuBillingBucketItemsTx(tx, bucket.Id, itemUsage, itemFee, itemCurrency, rawRecordIds); err != nil {
		return nil, err
	}
	return &bucket, nil
}

func qiniuCostDetailAmountToQuota(amount float64) int {
	return int(decimal.NewFromFloat(amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
}

func qiniuBillingBucketRawRecordIdsTx(tx *gorm.DB, bucket *model.QiniuBillingBucket) ([]int, error) {
	if tx == nil {
		tx = model.DB
	}
	if bucket == nil {
		return nil, errors.New("账单 bucket 不能为空")
	}
	ids := make([]int, 0)
	var items []model.QiniuBillingBucketItem
	if err := tx.Select("raw_record_ids").Where("bucket_id = ?", bucket.Id).Find(&items).Error; err != nil {
		return nil, err
	}
	seen := make(map[int]bool)
	for _, item := range items {
		for _, part := range strings.Split(item.RawRecordIds, ",") {
			id, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || id <= 0 || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		return ids, nil
	}
	if strings.TrimSpace(bucket.QiniuMaskedKey) == "" {
		return nil, errors.New("账单 bucket 缺少脱敏 Key，不能自动匹配 raw record")
	}
	query := tx.Select("id").Where("billing_date = ?", bucket.BillingDate)
	query = query.Where("qiniu_masked_key = ?", bucket.QiniuMaskedKey)
	query = query.Where("owner_status IN ?", []string{
		model.QiniuBillingOwnerStatusUnmapped,
		model.QiniuBillingOwnerStatusAmbiguous,
	})
	var records []model.QiniuCostDetailRecord
	if err := query.Order("id asc").Find(&records).Error; err != nil {
		return nil, err
	}
	for _, record := range records {
		ids = append(ids, record.Id)
	}
	return ids, nil
}

func summarizeQiniuLocalRealtimeQuota(tokenId int, billingDate string) (int, string, error) {
	billingDay, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(billingDate), qiniuCSTLocation)
	if err != nil {
		return 0, model.QiniuBillingLocalRealtimeStatusError, err
	}
	logDB := model.LOG_DB
	if logDB == nil {
		logDB = model.DB
	}
	type summaryRow struct {
		Quota int
		Count int64
	}
	type logSummaryRow struct {
		Id        int
		Quota     int
		RequestId string
	}
	var applications []model.QiniuRealtimeWalletApplication
	err = model.DB.Model(&model.QiniuRealtimeWalletApplication{}).
		Select("quota, request_id, consume_log_id, consume_log_ids").
		Where("token_id = ? AND status = ? AND settlement_applied = ? AND created_time >= ? AND created_time < ?",
			tokenId,
			model.QiniuRealtimeWalletApplicationStatusApplied,
			true,
			billingDay.Unix(),
			billingDay.AddDate(0, 0, 1).Unix(),
		).
		Find(&applications).Error
	if err != nil {
		return 0, model.QiniuBillingLocalRealtimeStatusError, err
	}
	applicationQuota := 0
	applicationRequestIds := make(map[string]struct{}, len(applications))
	applicationLogIds := make(map[int]struct{})
	for _, application := range applications {
		applicationQuota += application.Quota
		if requestId := strings.TrimSpace(application.RequestId); requestId != "" {
			applicationRequestIds[requestId] = struct{}{}
		}
		for _, logId := range application.ConsumeLogIdList() {
			applicationLogIds[logId] = struct{}{}
		}
	}

	var row summaryRow
	var logRows []logSummaryRow
	err = logDB.Model(&model.Log{}).
		Select("id, quota, request_id").
		Where("token_id = ? AND type = ? AND created_at >= ? AND created_at < ?", tokenId, model.LogTypeConsume, billingDay.Unix(), billingDay.AddDate(0, 0, 1).Unix()).
		Where("other LIKE ? ESCAPE '!'", `%"billing!_source":"qiniu!_market!_realtime"%`).
		Scan(&logRows).Error
	if err != nil {
		return 0, model.QiniuBillingLocalRealtimeStatusError, err
	}
	for _, logRow := range logRows {
		if _, ok := applicationLogIds[logRow.Id]; ok {
			continue
		}
		if requestId := strings.TrimSpace(logRow.RequestId); requestId != "" {
			if _, ok := applicationRequestIds[requestId]; ok {
				continue
			}
		}
		row.Quota += logRow.Quota
		row.Count++
	}
	if len(applications) == 0 && row.Count == 0 {
		return 0, model.QiniuBillingLocalRealtimeStatusMissing, nil
	}
	return applicationQuota + row.Quota, model.QiniuBillingLocalRealtimeStatusFound, nil
}

func qiniuBillingBucketStatusForPendingDelta(billingDate string, pendingDeltaQuota int) (string, string) {
	setting := operation_setting.GetQiniuKeySetting()
	if !setting.CostDetailAutoApplyEnabled {
		return model.QiniuBillingBucketStatusNeedsReview, qiniuCostDetailAutoApplyDisabledReason
	}
	cutoverBillingDate, ok := qiniuCostDetailCutoverBillingDate(setting)
	if !ok {
		return model.QiniuBillingBucketStatusSkipped, qiniuCostDetailCutoverNotConfiguredReason
	}
	normalizedBillingDate := strings.TrimSpace(billingDate)
	if _, err := time.ParseInLocation("2006-01-02", normalizedBillingDate, qiniuCSTLocation); err != nil {
		return model.QiniuBillingBucketStatusSkipped, qiniuCostDetailInvalidBillingDateReason
	}
	if normalizedBillingDate < cutoverBillingDate {
		return model.QiniuBillingBucketStatusSkipped, qiniuCostDetailBeforeCutoverReason
	}
	if pendingDeltaQuota == 0 {
		return model.QiniuBillingBucketStatusReconciled, ""
	}
	return model.QiniuBillingBucketStatusPending, ""
}

func qiniuCostDetailCutoverBillingDate(setting *operation_setting.QiniuKeySetting) (string, bool) {
	if setting == nil || setting.CostDetailCutoverTime <= 0 {
		return "", false
	}
	cutover := time.Unix(setting.CostDetailCutoverTime, 0).In(qiniuCSTLocation)
	return dateOnly(cutover).Format("2006-01-02"), true
}

func qiniuCostDetailNextRetryTime(now int64) int64 {
	interval := operation_setting.GetQiniuKeySetting().OfficialLedgerRetryIntervalSeconds
	if interval <= 0 {
		interval = operation_setting.QiniuOfficialLedgerDefaultRetryInterval
	}
	return now + int64(interval)
}

func upsertQiniuBillingBucketItemsTx(tx *gorm.DB, bucketId int, itemUsage map[string]float64, itemFee map[string]float64, itemCurrency map[string]string, rawRecordIds map[string][]string) error {
	for itemKey, usage := range itemUsage {
		modelName, billingItem := splitQiniuBucketItemKey(itemKey)
		updates := model.QiniuBillingBucketItem{
			BucketId:     bucketId,
			ModelName:    modelName,
			BillingItem:  billingItem,
			UsageCount:   usage,
			FeeAmount:    itemFee[itemKey],
			Currency:     itemCurrency[itemKey],
			RawRecordIds: strings.Join(rawRecordIds[itemKey], ","),
		}
		var existing model.QiniuBillingBucketItem
		err := tx.Where("bucket_id = ? AND model_name = ? AND billing_item = ?", bucketId, modelName, billingItem).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&updates).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := tx.Model(&model.QiniuBillingBucketItem{}).Where("id = ?", existing.Id).Updates(map[string]interface{}{
			"usage_count":    updates.UsageCount,
			"fee_amount":     updates.FeeAmount,
			"currency":       updates.Currency,
			"raw_record_ids": updates.RawRecordIds,
			"updated_time":   common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func qiniuBucketItemKey(modelName string, billingItem string) string {
	return strings.TrimSpace(modelName) + "\x00" + strings.TrimSpace(billingItem)
}

func splitQiniuBucketItemKey(key string) (string, string) {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func bucketIdForLog(bucket *model.QiniuBillingBucket) int {
	if bucket == nil {
		return 0
	}
	return bucket.Id
}

func qiniuOwnershipResultString(result *QiniuCostDetailOwnershipResult) string {
	if result == nil {
		return ""
	}
	return fmt.Sprintf("record_id=%d status=%s user_id=%d token_id=%d candidates=%d confirmed=%d bucket_id=%d",
		result.RecordId,
		result.OwnerStatus,
		result.UserId,
		result.TokenId,
		result.CandidateCount,
		result.ConfirmedCount,
		result.BucketId,
	)
}
