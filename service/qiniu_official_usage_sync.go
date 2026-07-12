package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

const qiniuOfficialUsageSyncTickInterval = time.Minute

var (
	qiniuOfficialUsageSyncOnce    sync.Once
	qiniuOfficialUsageSyncRunning atomic.Bool
)

type QiniuOfficialUsageSyncResult struct {
	WindowCount            int
	TokenCount             int
	UsageRecordCount       int
	BillRecordCount        int
	InsertedCount          int
	UnmappedCount          int
	CutoverSkippedCount    int
	LedgerAppliedCount     int
	LedgerFailedCount      int
	LedgerLogRepairedCount int
	Errors                 []string
}

type qiniuOfficialSyncWindow struct {
	Start       time.Time
	End         time.Time
	Granularity string
	CostGrain   string
	QueryCost   bool
}

func IsQiniuOfficialLedgerEnabled() bool {
	return operation_setting.GetQiniuKeySetting().OfficialLedgerEnabled
}

func StartQiniuOfficialUsageSyncTask() {
	qiniuOfficialUsageSyncOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog(fmt.Sprintf("qiniu official usage sync task started: tick=%s", qiniuOfficialUsageSyncTickInterval))
			ticker := time.NewTicker(qiniuOfficialUsageSyncTickInterval)
			defer ticker.Stop()
			runQiniuOfficialUsageSyncOnce()
			for range ticker.C {
				runQiniuOfficialUsageSyncOnce()
			}
		})
	})
}

func runQiniuOfficialUsageSyncOnce() {
	if !IsQiniuOfficialLedgerEnabled() {
		return
	}
	if !qiniuOfficialUsageSyncRunning.CompareAndSwap(false, true) {
		return
	}
	defer qiniuOfficialUsageSyncRunning.Store(false)
	result, err := SyncQiniuOfficialUsageOnce(context.Background())
	if err != nil {
		common.SysLog("qiniu official usage sync failed: " + sanitizeQiniuTaskError(err))
		return
	}
	if result.UsageRecordCount > 0 || result.BillRecordCount > 0 || result.LedgerLogRepairedCount > 0 || len(result.Errors) > 0 {
		common.SysLog(fmt.Sprintf(
			"qiniu official usage sync finished windows=%d tokens=%d usage_records=%d bill_records=%d inserted=%d unmapped=%d cutover_skipped=%d ledger_applied=%d ledger_failed=%d ledger_log_repaired=%d errors=%d",
			result.WindowCount,
			result.TokenCount,
			result.UsageRecordCount,
			result.BillRecordCount,
			result.InsertedCount,
			result.UnmappedCount,
			result.CutoverSkippedCount,
			result.LedgerAppliedCount,
			result.LedgerFailedCount,
			result.LedgerLogRepairedCount,
			len(result.Errors),
		))
	}
}

func SyncQiniuOfficialUsageOnce(ctx context.Context) (*QiniuOfficialUsageSyncResult, error) {
	return syncQiniuOfficialUsageAt(ctx, time.Now())
}

func syncQiniuOfficialUsageAt(ctx context.Context, now time.Time) (*QiniuOfficialUsageSyncResult, error) {
	setting := operation_setting.GetQiniuKeySetting()
	if !setting.OfficialLedgerEnabled {
		return &QiniuOfficialUsageSyncResult{Errors: make([]string, 0)}, nil
	}
	if _, err := newQiniuKeyClient(setting); err != nil {
		return nil, err
	}
	batchSize := setting.OfficialLedgerBatchSize
	if batchSize <= 0 {
		batchSize = operation_setting.QiniuOfficialLedgerDefaultBatchSize
	}
	windows := buildQiniuOfficialSyncWindows(now, setting)
	result := &QiniuOfficialUsageSyncResult{
		WindowCount: len(windows),
		Errors:      make([]string, 0),
	}
	throttle := newQiniuOfficialSyncThrottle(setting.OfficialLedgerRateLimitPerSecond)
	deletedTokenSyncAfter := qiniuOfficialDeletedTokenSyncAfter(windows, setting.OfficialLedgerCutoverTime)
	lastTokenId := 0
	for {
		tokens, err := model.ListQiniuManagedTokensForOfficialLedgerSyncAfterId(lastTokenId, batchSize, deletedTokenSyncAfter)
		if err != nil {
			if result.TokenCount == 0 {
				return nil, err
			}
			result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
			return result, err
		}
		if len(tokens) == 0 {
			break
		}
		result.TokenCount += len(tokens)
		lastTokenId = tokens[len(tokens)-1].Id
		for _, token := range tokens {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			client, err := NewQiniuAccountIdentityClient(token.QiniuChildAccountId, QiniuAccountOperationOfficialUsage)
			if err != nil {
				result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
				continue
			}
			for _, window := range windows {
				if err := throttle.wait(ctx); err != nil {
					return result, err
				}
				usageItems, err := client.QueryOfficialTokenUsage(ctx, qiniuOfficialUsageQuery{
					Granularity: window.Granularity,
					Start:       window.Start,
					End:         window.End,
					APIKey:      token.Key,
				})
				if err != nil {
					result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
				} else {
					summary, err := persistQiniuOfficialUsageItems(&token, usageItems, window, setting.OfficialLedgerCutoverTime)
					if err != nil {
						result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
					} else {
						result.UsageRecordCount += summary.recordCount
						result.InsertedCount += summary.insertedCount
						result.UnmappedCount += summary.unmappedCount
						result.CutoverSkippedCount += summary.cutoverSkippedCount
					}
				}
				if !window.QueryCost {
					continue
				}
				if err := throttle.wait(ctx); err != nil {
					return result, err
				}
				billItems, err := client.QueryOfficialCostDetails(ctx, qiniuOfficialCostDetailQuery{
					StartDate: window.Start,
					EndDate:   window.End,
					Grain:     window.CostGrain,
					APIKey:    token.Key,
				})
				if err != nil {
					result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
					continue
				}
				summary, err := persistQiniuOfficialBillItems(&token, billItems, window, setting.OfficialLedgerCutoverTime)
				if err != nil {
					result.Errors = append(result.Errors, sanitizeQiniuTaskError(err))
					continue
				}
				result.BillRecordCount += summary.recordCount
				result.InsertedCount += summary.insertedCount
				result.UnmappedCount += summary.unmappedCount
				result.CutoverSkippedCount += summary.cutoverSkippedCount
			}
		}
	}
	// 本 change 中 official usage/cost-detail 仅保留为管理员观测数据。
	// 不再自动应用 ledger 或修复合成日志，避免与七牛 market 实时扣费重复入账。
	return result, nil
}

func qiniuOfficialDeletedTokenSyncAfter(windows []qiniuOfficialSyncWindow, cutoverTime int64) int64 {
	deletedAfter := int64(0)
	for _, window := range windows {
		start := window.Start.Unix()
		if deletedAfter == 0 || start < deletedAfter {
			deletedAfter = start
		}
	}
	if cutoverTime > 0 && (deletedAfter == 0 || cutoverTime > deletedAfter) {
		deletedAfter = cutoverTime
	}
	return deletedAfter
}

type qiniuOfficialPersistSummary struct {
	recordCount         int
	insertedCount       int
	unmappedCount       int
	cutoverSkippedCount int
}

func persistQiniuOfficialUsageItems(token *model.Token, items []qiniuOfficialTokenUsageItem, window qiniuOfficialSyncWindow, cutoverTime int64) (*qiniuOfficialPersistSummary, error) {
	summary := &qiniuOfficialPersistSummary{}
	for _, item := range items {
		record := qiniuOfficialUsageRecordFromUsageItem(token, item, window, cutoverTime)
		upserted, inserted, err := model.UpsertQiniuOfficialUsageRecord(nil, record)
		if err != nil {
			return summary, err
		}
		summary.recordCount++
		if inserted {
			summary.insertedCount++
		}
		if upserted.Status == model.QiniuOfficialRecordStatusUnmapped {
			summary.unmappedCount++
		}
		if upserted.Status == model.QiniuOfficialRecordStatusSkipped && cutoverTime > 0 && upserted.PeriodEnd <= cutoverTime {
			summary.cutoverSkippedCount++
		}
	}
	return summary, nil
}

func persistQiniuOfficialBillItems(token *model.Token, items []qiniuOfficialCostDetailItem, window qiniuOfficialSyncWindow, cutoverTime int64) (*qiniuOfficialPersistSummary, error) {
	summary := &qiniuOfficialPersistSummary{}
	for _, item := range items {
		record := qiniuOfficialUsageRecordFromBillItem(token, item, window, cutoverTime)
		upserted, inserted, err := model.UpsertQiniuOfficialUsageRecord(nil, record)
		if err != nil {
			return summary, err
		}
		summary.recordCount++
		if inserted {
			summary.insertedCount++
		}
		if upserted.Status == model.QiniuOfficialRecordStatusUnmapped {
			summary.unmappedCount++
		}
		if upserted.Status == model.QiniuOfficialRecordStatusSkipped && cutoverTime > 0 && upserted.PeriodEnd <= cutoverTime {
			summary.cutoverSkippedCount++
		}
	}
	return summary, nil
}

func qiniuOfficialUsageRecordFromUsageItem(token *model.Token, item qiniuOfficialTokenUsageItem, window qiniuOfficialSyncWindow, cutoverTime int64) *model.QiniuOfficialUsageRecord {
	record := &model.QiniuOfficialUsageRecord{
		RecordType:       model.QiniuOfficialRecordTypeUsage,
		SourceAPI:        qiniuOfficialUsagePath,
		QiniuKey:         officialItemKey(item.APIKey, token),
		PeriodStart:      item.PeriodStart,
		PeriodEnd:        item.PeriodEnd,
		Granularity:      window.Granularity,
		ModelName:        item.ModelName,
		BillingItem:      item.BillingItem,
		PromptTokens:     item.PromptTokens,
		CompletionTokens: item.CompletionTokens,
		TotalTokens:      item.TotalTokens,
		Currency:         "CNY",
		Status:           model.QiniuOfficialRecordStatusSkipped,
		LastError:        "usage_record_observation_only",
		RawResponse:      item.RawResponse,
		FetchedAt:        common.GetTimestamp(),
	}
	fillQiniuOfficialRecordLocalRefs(record, token)
	fillQiniuOfficialRecordIdentity(record)
	if cutoverTime > 0 && record.PeriodEnd <= cutoverTime {
		record.LastError = "before_official_ledger_cutover"
	}
	return record
}

func qiniuOfficialUsageRecordFromBillItem(token *model.Token, item qiniuOfficialCostDetailItem, window qiniuOfficialSyncWindow, cutoverTime int64) *model.QiniuOfficialUsageRecord {
	record := &model.QiniuOfficialUsageRecord{
		RecordType:    model.QiniuOfficialRecordTypeBill,
		SourceAPI:     qiniuAPIKeyUsagePath,
		QiniuKey:      officialItemKey(item.APIKey, token),
		PeriodStart:   item.PeriodStart,
		PeriodEnd:     item.PeriodEnd,
		Granularity:   window.CostGrain,
		ModelName:     item.ModelName,
		BillingItem:   item.BillingItem,
		FeeAmount:     item.FeeAmount,
		Currency:      item.Currency,
		OfficialQuota: amountToQuota(item.FeeAmount),
		Status:        model.QiniuOfficialRecordStatusPending,
		RawResponse:   item.RawResponse,
		FetchedAt:     common.GetTimestamp(),
	}
	fillQiniuOfficialRecordLocalRefs(record, token)
	if record.UserId == 0 || record.TokenId == 0 {
		record.Status = model.QiniuOfficialRecordStatusUnmapped
		record.LastError = "official_key_unmapped"
	}
	if cutoverTime > 0 && record.PeriodEnd <= cutoverTime {
		record.Status = model.QiniuOfficialRecordStatusSkipped
		record.LastError = "before_official_ledger_cutover"
	}
	fillQiniuOfficialRecordIdentity(record)
	return record
}

func fillQiniuOfficialRecordLocalRefs(record *model.QiniuOfficialUsageRecord, token *model.Token) {
	if record == nil || token == nil || !token.IsQiniuManaged() {
		return
	}
	record.UserId = token.UserId
	record.TokenId = token.Id
	record.QiniuChildAccountId = token.QiniuChildAccountId
	if record.QiniuKey == "" {
		record.QiniuKey = fullQiniuAPIKey(token.Key)
	}
}

func fillQiniuOfficialRecordIdentity(record *model.QiniuOfficialUsageRecord) {
	if record == nil {
		return
	}
	record.RecordHash = qiniuOfficialHash(record.RawResponse)
	record.RecordKey = qiniuOfficialRecordKey(
		record.RecordType,
		record.QiniuKey,
		record.PeriodStart,
		record.PeriodEnd,
		record.Granularity,
		record.ModelName,
		record.BillingItem,
	)
}

func officialItemKey(itemKey string, token *model.Token) string {
	itemKey = strings.TrimSpace(itemKey)
	if itemKey != "" {
		return itemKey
	}
	if token == nil {
		return ""
	}
	return fullQiniuAPIKey(token.Key)
}

func qiniuOfficialRecordKey(parts ...any) string {
	raw := make([]string, 0, len(parts))
	for _, part := range parts {
		raw = append(raw, fmt.Sprint(part))
	}
	return "qiniu:official:" + qiniuOfficialHash(strings.Join(raw, "|"))
}

func qiniuOfficialHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func buildQiniuOfficialSyncWindows(now time.Time, setting *operation_setting.QiniuKeySetting) []qiniuOfficialSyncWindow {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(qiniuCSTLocation)
	windowHours := operation_setting.QiniuOfficialLedgerDefaultWindowHours
	windowDays := operation_setting.QiniuOfficialLedgerDefaultWindowDays
	if setting != nil {
		if setting.OfficialLedgerWindowHours > 0 {
			windowHours = setting.OfficialLedgerWindowHours
		}
		if setting.OfficialLedgerWindowDays > 0 {
			windowDays = setting.OfficialLedgerWindowDays
		}
	}
	windows := make([]qiniuOfficialSyncWindow, 0, windowHours+windowDays+1)
	seen := make(map[string]bool)
	addWindow := func(start time.Time, end time.Time, granularity string, costGrain string, queryCost bool) {
		if !end.After(start) {
			return
		}
		key := fmt.Sprintf("%s:%d:%d", granularity, start.Unix(), end.Unix())
		if seen[key] {
			return
		}
		seen[key] = true
		windows = append(windows, qiniuOfficialSyncWindow{
			Start:       start,
			End:         end,
			Granularity: granularity,
			CostGrain:   costGrain,
			QueryCost:   queryCost,
		})
	}
	currentHourStart := now.Truncate(time.Hour)
	addWindow(currentHourStart, now, "hour", "day", false)
	for i := 1; i < windowHours; i++ {
		start := currentHourStart.Add(-time.Duration(i) * time.Hour)
		addWindow(start, start.Add(time.Hour), "hour", "day", false)
	}
	todayStart := dateOnly(now)
	for i := 0; i < windowDays; i++ {
		start := todayStart.AddDate(0, 0, -i)
		end := start.AddDate(0, 0, 1)
		if i == 0 && now.Before(end) {
			end = now
		}
		addWindow(start, end, "day", "day", true)
	}
	if setting != nil && setting.OfficialLedgerCutoverTime > 0 {
		cutover := time.Unix(setting.OfficialLedgerCutoverTime, 0).In(qiniuCSTLocation)
		cutoverDay := dateOnly(cutover)
		addWindow(cutoverDay, cutoverDay.AddDate(0, 0, 1), "day", "day", true)
	}
	return windows
}

type qiniuOfficialSyncThrottle struct {
	interval time.Duration
	lastCall time.Time
}

func newQiniuOfficialSyncThrottle(rateLimitPerSecond int) *qiniuOfficialSyncThrottle {
	if rateLimitPerSecond <= 0 {
		rateLimitPerSecond = operation_setting.QiniuOfficialLedgerDefaultRateLimit
	}
	return &qiniuOfficialSyncThrottle{interval: time.Second / time.Duration(rateLimitPerSecond)}
}

func (throttle *qiniuOfficialSyncThrottle) wait(ctx context.Context) error {
	if throttle == nil || throttle.interval <= 0 || throttle.lastCall.IsZero() {
		if throttle != nil {
			throttle.lastCall = time.Now()
		}
		return ctx.Err()
	}
	waitFor := throttle.interval - time.Since(throttle.lastCall)
	if waitFor > 0 {
		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	throttle.lastCall = time.Now()
	return ctx.Err()
}
