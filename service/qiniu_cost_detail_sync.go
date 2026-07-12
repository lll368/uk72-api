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
)

const (
	qiniuCostDetailSyncTickInterval   = 24 * time.Hour
	qiniuCostDetailDefaultLookbackDay = operation_setting.QiniuCostDetailDefaultLookbackDays
)

var (
	qiniuCostDetailSyncOnce    sync.Once
	qiniuCostDetailSyncRunning atomic.Bool
)

type QiniuCostDetailSyncResult struct {
	WindowCount    int
	RawRecordCount int
	InsertedCount  int
	ResolvedCount  int
	BucketCount    int
	AlertSummary   QiniuCostDetailSyncAlertSummary
	Errors         []string
}

type QiniuCostDetailSyncAlertSummary struct {
	UnmappedCount            int
	AmbiguousCount           int
	FailedApplicationCount   int
	AffectedQuota            int
	AffectedAmount           float64
	LatestError              string
	LatestSuccessfulSyncTime int64
	LatestRetryResult        string
}

type qiniuCostDetailSyncWindow struct {
	BillingDate string
	Start       time.Time
	End         time.Time
}

func IsQiniuCostDetailSyncEnabled() bool {
	return operation_setting.GetQiniuKeySetting().OfficialLedgerEnabled
}

func StartQiniuCostDetailSyncTask() {
	qiniuCostDetailSyncOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog(fmt.Sprintf("qiniu cost-detail bucket sync task started: tick=%s", qiniuCostDetailSyncTickInterval))
			ticker := time.NewTicker(qiniuCostDetailSyncTickInterval)
			defer ticker.Stop()
			runQiniuCostDetailSyncOnce()
			for range ticker.C {
				runQiniuCostDetailSyncOnce()
			}
		})
	})
}

func runQiniuCostDetailSyncOnce() {
	if !IsQiniuCostDetailSyncEnabled() {
		return
	}
	if !qiniuCostDetailSyncRunning.CompareAndSwap(false, true) {
		return
	}
	defer qiniuCostDetailSyncRunning.Store(false)
	result, err := SyncQiniuCostDetailOnce(context.Background())
	if err != nil {
		common.SysLog("qiniu cost-detail bucket sync failed: " + sanitizeQiniuTaskError(err))
		return
	}
	if result.RawRecordCount > 0 || len(result.Errors) > 0 {
		common.SysLog(fmt.Sprintf(
			"qiniu cost-detail bucket sync finished windows=%d raw_records=%d inserted=%d resolved=%d buckets=%d errors=%d unmapped=%d ambiguous=%d failed_applications=%d affected_quota=%d",
			result.WindowCount,
			result.RawRecordCount,
			result.InsertedCount,
			result.ResolvedCount,
			result.BucketCount,
			len(result.Errors),
			result.AlertSummary.UnmappedCount,
			result.AlertSummary.AmbiguousCount,
			result.AlertSummary.FailedApplicationCount,
			result.AlertSummary.AffectedQuota,
		))
	}
}

func SyncQiniuCostDetailOnce(ctx context.Context) (*QiniuCostDetailSyncResult, error) {
	return syncQiniuCostDetailAt(ctx, time.Now())
}

func GetQiniuCostDetailAlertSummary() QiniuCostDetailSyncAlertSummary {
	return buildQiniuCostDetailAlertSummary(time.Time{}, nil)
}

func syncQiniuCostDetailAt(ctx context.Context, now time.Time) (*QiniuCostDetailSyncResult, error) {
	setting := operation_setting.GetQiniuKeySetting()
	if !setting.OfficialLedgerEnabled {
		return &QiniuCostDetailSyncResult{Errors: make([]string, 0)}, nil
	}
	client, err := newQiniuKeyClient(setting)
	if err != nil {
		return nil, err
	}
	windows := buildQiniuCostDetailSyncWindows(now, setting.CostDetailLookbackDays)
	result := &QiniuCostDetailSyncResult{
		WindowCount: len(windows),
		Errors:      make([]string, 0),
	}
	throttle := newQiniuOfficialSyncThrottle(setting.OfficialLedgerRateLimitPerSecond)
	for _, window := range windows {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if err := throttle.wait(ctx); err != nil {
			return result, err
		}
		items, err := client.QueryOfficialCostDetails(ctx, qiniuOfficialCostDetailQuery{
			StartDate: window.Start,
			EndDate:   window.Start,
			Grain:     "day",
		})
		if err != nil {
			result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
		} else {
			summary, err := persistQiniuCostDetailItems(ctx, items, window)
			if err != nil {
				result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
			} else {
				mergeQiniuCostDetailPersistSummary(result, summary)
			}
		}
		childSummary, err := syncQiniuCostDetailChildTokenWindow(ctx, window, setting, throttle)
		if err != nil {
			result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
			continue
		}
		mergeQiniuCostDetailPersistSummary(result, childSummary)
	}
	result.AlertSummary = buildQiniuCostDetailAlertSummary(now, result.Errors)
	return result, nil
}

func syncQiniuCostDetailChildTokenWindow(ctx context.Context, window qiniuCostDetailSyncWindow, setting *operation_setting.QiniuKeySetting, throttle *qiniuOfficialSyncThrottle) (*qiniuCostDetailPersistSummary, error) {
	summary := &qiniuCostDetailPersistSummary{errors: make([]string, 0)}
	batchSize := operation_setting.QiniuOfficialLedgerDefaultBatchSize
	if setting != nil && setting.OfficialLedgerBatchSize > 0 {
		batchSize = setting.OfficialLedgerBatchSize
	}
	lastTokenId := 0
	for {
		tokens, err := model.ListQiniuManagedTokensForOfficialLedgerSyncAfterId(lastTokenId, batchSize, window.Start.Unix())
		if err != nil {
			return summary, err
		}
		if len(tokens) == 0 {
			return summary, nil
		}
		lastTokenId = tokens[len(tokens)-1].Id
		for _, token := range tokens {
			if ctx != nil && ctx.Err() != nil {
				return summary, ctx.Err()
			}
			if token.QiniuChildAccountId <= 0 {
				continue
			}
			client, err := NewQiniuAccountIdentityClient(token.QiniuChildAccountId, QiniuAccountOperationCostDetail)
			if err != nil {
				summary.errors = append(summary.errors, sanitizeQiniuTaskError(err))
				continue
			}
			if err := throttle.wait(ctx); err != nil {
				return summary, err
			}
			items, err := client.QueryOfficialCostDetails(ctx, qiniuOfficialCostDetailQuery{
				StartDate: window.Start,
				EndDate:   window.Start,
				Grain:     "day",
				APIKey:    token.Key,
			})
			if err != nil {
				summary.errors = append(summary.errors, sanitizeQiniuTaskError(err))
				continue
			}
			tokenSummary, err := persistQiniuCostDetailItemsForToken(ctx, &token, items, window)
			if err != nil {
				summary.errors = append(summary.errors, sanitizeQiniuTaskError(err))
				continue
			}
			summary.recordCount += tokenSummary.recordCount
			summary.insertedCount += tokenSummary.insertedCount
			summary.resolvedCount += tokenSummary.resolvedCount
			summary.bucketCount += tokenSummary.bucketCount
			summary.errors = append(summary.errors, tokenSummary.errors...)
		}
	}
}

func mergeQiniuCostDetailPersistSummary(result *QiniuCostDetailSyncResult, summary *qiniuCostDetailPersistSummary) {
	if result == nil || summary == nil {
		return
	}
	result.RawRecordCount += summary.recordCount
	result.InsertedCount += summary.insertedCount
	result.ResolvedCount += summary.resolvedCount
	result.BucketCount += summary.bucketCount
	result.Errors = append(result.Errors, summary.errors...)
}

func buildQiniuCostDetailAlertSummary(now time.Time, syncErrors []string) QiniuCostDetailSyncAlertSummary {
	summary := QiniuCostDetailSyncAlertSummary{}
	if !now.IsZero() {
		summary.LatestSuccessfulSyncTime = now.Unix()
	} else {
		var latest struct {
			UpdatedTime int64
		}
		_ = model.DB.Model(&model.QiniuCostDetailRecord{}).
			Select("COALESCE(MAX(updated_time), 0) AS updated_time").
			Scan(&latest).Error
		summary.LatestSuccessfulSyncTime = latest.UpdatedTime
	}
	var unmappedCount int64
	var ambiguousCount int64
	var failedApplicationCount int64
	_ = model.DB.Model(&model.QiniuCostDetailRecord{}).
		Where("owner_status = ?", model.QiniuBillingOwnerStatusUnmapped).
		Count(&unmappedCount).Error
	_ = model.DB.Model(&model.QiniuCostDetailRecord{}).
		Where("owner_status = ?", model.QiniuBillingOwnerStatusAmbiguous).
		Count(&ambiguousCount).Error
	_ = model.DB.Model(&model.QiniuBillingBucketApplication{}).
		Where("status = ?", model.QiniuBillingApplicationStatusFailed).
		Count(&failedApplicationCount).Error
	summary.UnmappedCount = int(unmappedCount)
	summary.AmbiguousCount = int(ambiguousCount)
	summary.FailedApplicationCount = int(failedApplicationCount)

	var affected struct {
		Total int
	}
	_ = model.DB.Model(&model.QiniuBillingBucket{}).
		Select("COALESCE(SUM(CASE WHEN pending_delta_quota < 0 THEN -pending_delta_quota ELSE pending_delta_quota END), 0) AS total").
		Where("status IN ?", []string{
			model.QiniuBillingBucketStatusFailed,
			model.QiniuBillingBucketStatusSkipped,
			model.QiniuBillingBucketStatusNeedsReview,
		}).
		Scan(&affected).Error
	summary.AffectedQuota = affected.Total
	summary.AffectedAmount = float64(summary.AffectedQuota) / float64(common.QuotaPerUnit)

	if len(syncErrors) > 0 {
		summary.LatestError = syncErrors[len(syncErrors)-1]
	} else {
		summary.LatestError = latestQiniuCostDetailAlertError()
	}
	summary.LatestRetryResult = fmt.Sprintf(
		"unmapped=%d ambiguous=%d failed_applications=%d affected_quota=%d",
		summary.UnmappedCount,
		summary.AmbiguousCount,
		summary.FailedApplicationCount,
		summary.AffectedQuota,
	)
	return summary
}

func latestQiniuCostDetailAlertError() string {
	var latestError string
	var latestUpdated int64
	var raw model.QiniuCostDetailRecord
	if err := model.DB.Where("last_error <> ''").Order("updated_time desc, id desc").First(&raw).Error; err == nil {
		latestError = raw.LastError
		latestUpdated = raw.UpdatedTime
	}
	var bucket model.QiniuBillingBucket
	if err := model.DB.Where("last_error <> ''").Order("updated_time desc, id desc").First(&bucket).Error; err == nil && bucket.UpdatedTime >= latestUpdated {
		latestError = bucket.LastError
		latestUpdated = bucket.UpdatedTime
	}
	var application model.QiniuBillingBucketApplication
	if err := model.DB.Where("last_error <> ''").Order("updated_time desc, id desc").First(&application).Error; err == nil && application.UpdatedTime >= latestUpdated {
		latestError = application.LastError
	}
	if strings.TrimSpace(latestError) == "" {
		return ""
	}
	return sanitizeQiniuTaskError(errors.New(latestError))
}

func buildQiniuCostDetailSyncWindows(now time.Time, lookbackDays int) []qiniuCostDetailSyncWindow {
	if now.IsZero() {
		now = time.Now()
	}
	if lookbackDays <= 0 {
		lookbackDays = qiniuCostDetailDefaultLookbackDay
	} else if lookbackDays > operation_setting.QiniuCostDetailMaxLookbackDays {
		lookbackDays = operation_setting.QiniuCostDetailMaxLookbackDays
	}
	todayStart := dateOnly(now.In(qiniuCSTLocation))
	windows := make([]qiniuCostDetailSyncWindow, 0, lookbackDays)
	for i := 1; i <= lookbackDays; i++ {
		start := todayStart.AddDate(0, 0, -i)
		windows = append(windows, qiniuCostDetailSyncWindow{
			BillingDate: start.Format("2006-01-02"),
			Start:       start,
			End:         start.AddDate(0, 0, 1),
		})
	}
	return windows
}

type qiniuCostDetailPersistSummary struct {
	recordCount   int
	insertedCount int
	resolvedCount int
	bucketCount   int
	errors        []string
}

func persistQiniuCostDetailItems(ctx context.Context, items []qiniuOfficialCostDetailItem, window qiniuCostDetailSyncWindow) (*qiniuCostDetailPersistSummary, error) {
	summary := &qiniuCostDetailPersistSummary{errors: make([]string, 0)}
	for _, item := range items {
		if ctx != nil && ctx.Err() != nil {
			return summary, ctx.Err()
		}
		record := qiniuCostDetailRecordFromItem(item, window)
		persisted, inserted, err := model.UpsertQiniuCostDetailRecord(nil, record)
		if err != nil {
			return summary, err
		}
		summary.recordCount++
		if inserted {
			summary.insertedCount++
		}
		ownership, err := ResolveQiniuCostDetailRecordOwnership(ctx, persisted.Id)
		if err != nil {
			summary.errors = append(summary.errors, sanitizeQiniuTaskError(err))
			continue
		}
		if ownership.OwnerStatus == model.QiniuBillingOwnerStatusResolved || ownership.OwnerStatus == model.QiniuBillingOwnerStatusManualResolved {
			summary.resolvedCount++
		}
		if ownership.BucketId > 0 {
			summary.bucketCount++
		}
	}
	return summary, nil
}

func persistQiniuCostDetailItemsForToken(ctx context.Context, token *model.Token, items []qiniuOfficialCostDetailItem, window qiniuCostDetailSyncWindow) (*qiniuCostDetailPersistSummary, error) {
	fallbackKey := qiniuCostDetailMaskedKeyFromToken(token)
	if fallbackKey == "" {
		return persistQiniuCostDetailItems(ctx, items, window)
	}
	for index := range items {
		maskedKey, err := qiniuCostDetailExistingMaskedKeyForTokenItem(token, items[index], window)
		if err != nil {
			return nil, err
		}
		if maskedKey == "" {
			maskedKey = fallbackKey
		}
		// 子账号按 APIKey 查询时七牛会返回 full key；落库前统一成本地可匹配的脱敏口径，
		// 避免父账号 masked 账单和子账号 full-key 账单生成不同 record_hash 后重复入账。
		items[index].APIKey = maskedKey
	}
	return persistQiniuCostDetailItems(ctx, items, window)
}

func qiniuCostDetailExistingMaskedKeyForTokenItem(token *model.Token, item qiniuOfficialCostDetailItem, window qiniuCostDetailSyncWindow) (string, error) {
	if token == nil || token.Id <= 0 {
		return "", nil
	}
	billingDate := qiniuCostDetailBillingDateForItem(item, window)
	modelName := strings.TrimSpace(item.ModelName)
	billingItem := strings.TrimSpace(item.BillingItem)
	if billingDate == "" || modelName == "" || billingItem == "" {
		return "", nil
	}
	var records []model.QiniuCostDetailRecord
	err := model.DB.Select("qiniu_masked_key").
		Where("token_id = ? AND billing_date = ? AND model_name = ? AND billing_item = ? AND owner_status IN ?", token.Id, billingDate, modelName, billingItem, []string{
			model.QiniuBillingOwnerStatusResolved,
			model.QiniuBillingOwnerStatusManualResolved,
		}).
		Order("id asc").
		Limit(1).
		Find(&records).Error
	if err != nil || len(records) == 0 {
		return "", err
	}
	return strings.TrimSpace(records[0].QiniuMaskedKey), nil
}

func qiniuCostDetailMaskedKeyFromToken(token *model.Token) string {
	if token == nil || strings.TrimSpace(token.Key) == "" {
		return ""
	}
	identity := model.BuildQiniuTokenKeyIdentity(token.Key)
	if identity.KeyPrefix == "" || identity.KeySuffix == "" {
		return ""
	}
	return "sk-" + identity.KeyPrefix + "***" + identity.KeySuffix
}

func qiniuCostDetailBillingDateForItem(item qiniuOfficialCostDetailItem, window qiniuCostDetailSyncWindow) string {
	if strings.TrimSpace(window.BillingDate) != "" {
		return strings.TrimSpace(window.BillingDate)
	}
	if item.PeriodStart > 0 {
		return time.Unix(item.PeriodStart, 0).In(qiniuCSTLocation).Format("2006-01-02")
	}
	return ""
}

func qiniuCostDetailRecordFromItem(item qiniuOfficialCostDetailItem, window qiniuCostDetailSyncWindow) *model.QiniuCostDetailRecord {
	maskedKey := strings.TrimSpace(item.APIKey)
	identity := model.BuildQiniuMaskedKeyIdentity(maskedKey)
	billingDate := qiniuCostDetailBillingDateForItem(item, window)
	currency := strings.TrimSpace(item.Currency)
	if currency == "" {
		currency = "CNY"
	}
	return &model.QiniuCostDetailRecord{
		QiniuMaskedKey: maskedKey,
		KeyPrefix:      identity.KeyPrefix,
		KeySuffix:      identity.KeySuffix,
		BillingDate:    billingDate,
		ModelName:      strings.TrimSpace(item.ModelName),
		BillingItem:    strings.TrimSpace(item.BillingItem),
		UsageCount:     item.UsageCount,
		UsageUnit:      strings.TrimSpace(item.UsageUnit),
		FeeAmount:      item.FeeAmount,
		Currency:       currency,
		RecordHash:     qiniuCostDetailRecordHash(item, billingDate),
		RawResponse:    item.RawResponse,
		OwnerStatus:    model.QiniuBillingOwnerStatusUnmapped,
	}
}

func qiniuCostDetailRecordHash(item qiniuOfficialCostDetailItem, billingDate string) string {
	return "qiniu:cost_detail:" + qiniuOfficialHash(strings.Join([]string{
		strings.TrimSpace(item.APIKey),
		strings.TrimSpace(billingDate),
		strings.TrimSpace(item.ModelName),
		strings.TrimSpace(item.BillingItem),
	}, "|"))
}
