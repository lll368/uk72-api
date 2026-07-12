package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestRecalculateQiniuBillingBucketAggregatesRawItemsAndLocalRealtimeLogs(t *testing.T) {
	truncate(t)

	token := seedQiniuOwnershipToken(t, 5301, 5301, "bucket-owner-123456")
	billingDate := "2026-06-04"
	inputRecord := seedQiniuCostDetailRawRecord(t, "bucket-input", "sk-buc***123456", "buc", "123456", billingDate, 0.25)
	outputRecord := seedQiniuCostDetailRawRecord(t, "bucket-output", "sk-buc***123456", "buc", "123456", billingDate, 0.15)
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id IN ?", []int{inputRecord.Id, outputRecord.Id}).Updates(map[string]interface{}{
		"owner_status": model.QiniuBillingOwnerStatusResolved,
		"user_id":      token.UserId,
		"token_id":     token.Id,
	}).Error)
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", outputRecord.Id).Updates(map[string]interface{}{
		"billing_item": "output",
		"fee_amount":   0.15,
	}).Error)

	localQuota := amountToQuota(0.10)
	requireNoError(t, model.LOG_DB.Create(&model.Log{
		UserId:    token.UserId,
		TokenId:   token.Id,
		Type:      model.LogTypeConsume,
		Quota:     localQuota,
		CreatedAt: time.Date(2026, 6, 4, 12, 0, 0, 0, qiniuCSTLocation).Unix(),
		Other:     common.MapToJsonStr(map[string]interface{}{"billing_source": QiniuMarketRealtimeBillingSource}),
	}).Error)

	bucket, err := RecalculateQiniuBillingBucket(token.Id, billingDate)
	requireNoError(t, err)
	expectedOfficialQuota := amountToQuota(0.40)
	if bucket.OfficialQuota != expectedOfficialQuota || bucket.LocalRealtimeQuota != localQuota || bucket.PendingDeltaQuota != expectedOfficialQuota-localQuota {
		t.Fatalf("unexpected bucket quota summary: %#v", bucket)
	}
	if bucket.LocalRealtimeStatus != QiniuBillingLocalRealtimeStatusFound {
		t.Fatalf("expected local realtime found status, got %#v", bucket)
	}

	var items []model.QiniuBillingBucketItem
	requireNoError(t, model.DB.Where("bucket_id = ?", bucket.Id).Order("billing_item asc").Find(&items).Error)
	if len(items) != 2 {
		t.Fatalf("expected two bucket items, got %#v", items)
	}
}

func TestRecalculateQiniuBillingBucketUsesRealtimeApplicationsWhenLogsMissing(t *testing.T) {
	truncate(t)

	token := seedQiniuOwnershipToken(t, 5331, 5331, "bucket-app-123456")
	billingDate := "2026-06-04"
	record := seedQiniuCostDetailRawRecord(t, "bucket-app", "sk-app***123456", "app", "123456", billingDate, 0.25)
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", record.Id).Updates(map[string]interface{}{
		"owner_status": model.QiniuBillingOwnerStatusResolved,
		"user_id":      token.UserId,
		"token_id":     token.Id,
	}).Error)

	localQuota := amountToQuota(0.10)
	requireNoError(t, model.DB.Create(&model.QiniuRealtimeWalletApplication{
		UserId:            token.UserId,
		TokenId:           token.Id,
		RequestId:         "req-bucket-app",
		IdempotencyKey:    "qiniu:realtime:request:req-bucket-app",
		Quota:             localQuota,
		Amount:            quotaToWalletAmount(localQuota),
		SettlementApplied: true,
		Status:            model.QiniuRealtimeWalletApplicationStatusApplied,
		CreatedTime:       time.Date(2026, 6, 4, 12, 0, 0, 0, qiniuCSTLocation).Unix(),
	}).Error)

	bucket, err := RecalculateQiniuBillingBucket(token.Id, billingDate)
	requireNoError(t, err)
	if bucket.LocalRealtimeQuota != localQuota || bucket.PendingDeltaQuota != amountToQuota(0.25)-localQuota {
		t.Fatalf("expected realtime application to count as local realtime quota, got %#v", bucket)
	}
	if bucket.LocalRealtimeStatus != QiniuBillingLocalRealtimeStatusFound {
		t.Fatalf("expected local realtime found status, got %#v", bucket)
	}
}

func TestRecalculateQiniuBillingBucketDoesNotDoubleCountRealtimeApplicationAndLog(t *testing.T) {
	truncate(t)

	token := seedQiniuOwnershipToken(t, 5332, 5332, "bucket-app-log-123456")
	billingDate := "2026-06-04"
	record := seedQiniuCostDetailRawRecord(t, "bucket-app-log", "sk-apl***123456", "apl", "123456", billingDate, 0.50)
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", record.Id).Updates(map[string]interface{}{
		"owner_status": model.QiniuBillingOwnerStatusResolved,
		"user_id":      token.UserId,
		"token_id":     token.Id,
	}).Error)

	localQuota := amountToQuota(0.20)
	coveredLog := &model.Log{
		UserId:    token.UserId,
		TokenId:   token.Id,
		Type:      model.LogTypeConsume,
		Quota:     amountToQuota(0.30),
		CreatedAt: time.Date(2026, 6, 4, 9, 0, 0, 0, qiniuCSTLocation).Unix(),
		RequestId: "req-bucket-app-log",
		Other:     common.MapToJsonStr(map[string]interface{}{"billing_source": QiniuMarketRealtimeBillingSource}),
	}
	requireNoError(t, model.LOG_DB.Create(coveredLog).Error)
	requireNoError(t, model.DB.Create(&model.QiniuRealtimeWalletApplication{
		UserId:            token.UserId,
		TokenId:           token.Id,
		RequestId:         "req-bucket-app-log",
		ConsumeLogId:      coveredLog.Id,
		ConsumeLogIds:     strconv.Itoa(coveredLog.Id),
		CoveredLogCount:   1,
		IdempotencyKey:    "qiniu:realtime:request:req-bucket-app-log",
		Quota:             localQuota,
		Amount:            quotaToWalletAmount(localQuota),
		SettlementApplied: true,
		Status:            model.QiniuRealtimeWalletApplicationStatusApplied,
		CreatedTime:       time.Date(2026, 6, 4, 9, 0, 0, 0, qiniuCSTLocation).Unix(),
	}).Error)

	bucket, err := RecalculateQiniuBillingBucket(token.Id, billingDate)
	requireNoError(t, err)
	if bucket.LocalRealtimeQuota != localQuota {
		t.Fatalf("expected application quota to be the realtime fact source without log double count, got %#v", bucket)
	}
}

func TestRecalculateQiniuBillingBucketCountsSameSecondLegacyLogWithoutReliableApplicationLink(t *testing.T) {
	truncate(t)

	token := seedQiniuOwnershipToken(t, 5334, 5334, "bucket-same-second-legacy-123456")
	billingDate := "2026-06-04"
	record := seedQiniuCostDetailRawRecord(t, "bucket-same-second-legacy", "sk-ssl***123456", "ssl", "123456", billingDate, 0.70)
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", record.Id).Updates(map[string]interface{}{
		"owner_status": model.QiniuBillingOwnerStatusResolved,
		"user_id":      token.UserId,
		"token_id":     token.Id,
	}).Error)

	createdAt := time.Date(2026, 6, 4, 9, 0, 0, 0, qiniuCSTLocation).Unix()
	applicationQuota := amountToQuota(0.20)
	legacyLogQuota := amountToQuota(0.15)
	requireNoError(t, model.DB.Create(&model.QiniuRealtimeWalletApplication{
		UserId:            token.UserId,
		TokenId:           token.Id,
		RequestId:         "req-bucket-same-second-application",
		IdempotencyKey:    "qiniu:realtime:request:req-bucket-same-second-application",
		Quota:             applicationQuota,
		Amount:            quotaToWalletAmount(applicationQuota),
		SettlementApplied: true,
		Status:            model.QiniuRealtimeWalletApplicationStatusApplied,
		CreatedTime:       createdAt,
	}).Error)
	requireNoError(t, model.LOG_DB.Create(&model.Log{
		UserId:    token.UserId,
		TokenId:   token.Id,
		Type:      model.LogTypeConsume,
		Quota:     legacyLogQuota,
		CreatedAt: createdAt,
		Other:     common.MapToJsonStr(map[string]interface{}{"billing_source": QiniuMarketRealtimeBillingSource}),
	}).Error)

	bucket, err := RecalculateQiniuBillingBucket(token.Id, billingDate)
	requireNoError(t, err)
	expectedLocalQuota := applicationQuota + legacyLogQuota
	if bucket.LocalRealtimeQuota != expectedLocalQuota {
		t.Fatalf("expected same-second legacy log without reliable link to count, got %#v", bucket)
	}
}

func TestRecalculateQiniuBillingBucketIncludesLegacyLogsWhenApplicationsExist(t *testing.T) {
	truncate(t)

	token := seedQiniuOwnershipToken(t, 5333, 5333, "bucket-mixed-app-log-123456")
	billingDate := "2026-06-04"
	record := seedQiniuCostDetailRawRecord(t, "bucket-mixed-app-log", "sk-mix***123456", "mix", "123456", billingDate, 0.70)
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", record.Id).Updates(map[string]interface{}{
		"owner_status": model.QiniuBillingOwnerStatusResolved,
		"user_id":      token.UserId,
		"token_id":     token.Id,
	}).Error)

	applicationQuota := amountToQuota(0.20)
	legacyLogQuota := amountToQuota(0.15)
	requireNoError(t, model.DB.Create(&model.QiniuRealtimeWalletApplication{
		UserId:            token.UserId,
		TokenId:           token.Id,
		RequestId:         "req-bucket-mixed-application",
		IdempotencyKey:    "qiniu:realtime:request:req-bucket-mixed-application",
		Quota:             applicationQuota,
		Amount:            quotaToWalletAmount(applicationQuota),
		SettlementApplied: true,
		Status:            model.QiniuRealtimeWalletApplicationStatusApplied,
		CreatedTime:       time.Date(2026, 6, 4, 9, 0, 0, 0, qiniuCSTLocation).Unix(),
	}).Error)
	requireNoError(t, model.LOG_DB.Create(&model.Log{
		UserId:    token.UserId,
		TokenId:   token.Id,
		Type:      model.LogTypeConsume,
		Quota:     applicationQuota,
		CreatedAt: time.Date(2026, 6, 4, 9, 0, 0, 0, qiniuCSTLocation).Unix(),
		RequestId: "req-bucket-mixed-application",
		Other:     common.MapToJsonStr(map[string]interface{}{"billing_source": QiniuMarketRealtimeBillingSource}),
	}).Error)
	requireNoError(t, model.LOG_DB.Create(&model.Log{
		UserId:    token.UserId,
		TokenId:   token.Id,
		Type:      model.LogTypeConsume,
		Quota:     legacyLogQuota,
		CreatedAt: time.Date(2026, 6, 4, 10, 0, 0, 0, qiniuCSTLocation).Unix(),
		RequestId: "req-bucket-mixed-legacy-log",
		Other:     common.MapToJsonStr(map[string]interface{}{"billing_source": QiniuMarketRealtimeBillingSource}),
	}).Error)

	bucket, err := RecalculateQiniuBillingBucket(token.Id, billingDate)
	requireNoError(t, err)
	expectedLocalQuota := applicationQuota + legacyLogQuota
	if bucket.LocalRealtimeQuota != expectedLocalQuota {
		t.Fatalf("expected application quota plus unmatched legacy log quota without double count, got %#v", bucket)
	}
	if bucket.PendingDeltaQuota != amountToQuota(0.70)-expectedLocalQuota {
		t.Fatalf("unexpected pending delta for mixed application/log bucket: %#v", bucket)
	}
}

func TestRecalculateQiniuBillingBucketRecordsMissingLogsRevisionAndAutoApplyPolicy(t *testing.T) {
	truncate(t)

	setting := operation_setting.GetQiniuKeySetting()
	oldSetting := *setting
	setting.CostDetailAutoApplyEnabled = false
	t.Cleanup(func() { *setting = oldSetting })

	token := seedQiniuOwnershipToken(t, 5311, 5311, "bucket-revision-123456")
	record := seedQiniuCostDetailRawRecord(t, "bucket-revision", "sk-rev***123456", "rev", "123456", "2026-06-04", 0.25)
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", record.Id).Updates(map[string]interface{}{
		"owner_status": model.QiniuBillingOwnerStatusResolved,
		"user_id":      token.UserId,
		"token_id":     token.Id,
	}).Error)

	bucket, err := RecalculateQiniuBillingBucket(token.Id, record.BillingDate)
	requireNoError(t, err)
	if bucket.LocalRealtimeStatus != QiniuBillingLocalRealtimeStatusMissing || bucket.Status != model.QiniuBillingBucketStatusNeedsReview {
		t.Fatalf("expected missing local logs and needs_review when auto apply disabled, got %#v", bucket)
	}

	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", record.Id).Update("fee_amount", 0.30).Error)
	bucket, err = RecalculateQiniuBillingBucket(token.Id, record.BillingDate)
	requireNoError(t, err)
	if bucket.PreviousOfficialQuota != amountToQuota(0.25) || bucket.OfficialQuota != amountToQuota(0.30) {
		t.Fatalf("expected official revision audit fields, got %#v", bucket)
	}
	if bucket.Status != model.QiniuBillingBucketStatusNeedsReview {
		t.Fatalf("expected auto apply disabled to keep needs_review, got %#v", bucket)
	}

	setting.CostDetailCutoverTime = time.Date(2026, 6, 4, 0, 0, 0, 0, qiniuCSTLocation).Unix()
	setting.CostDetailAutoApplyEnabled = true
	bucket, err = RecalculateQiniuBillingBucket(token.Id, record.BillingDate)
	requireNoError(t, err)
	if bucket.Status != model.QiniuBillingBucketStatusApplied || bucket.PendingDeltaQuota != 0 {
		t.Fatalf("expected auto apply enabled to apply pending bucket delta, got %#v", bucket)
	}
	var app model.QiniuBillingBucketApplication
	requireNoError(t, model.DB.First(&app, "bucket_id = ?", bucket.Id).Error)
	if app.Status != model.QiniuBillingApplicationStatusSuccess || app.DeltaQuota != amountToQuota(0.30) {
		t.Fatalf("expected auto apply to create successful application, got %#v", app)
	}
}

func TestRecalculateQiniuBillingBucketRequiresCutoverAndNormalizesBillingDate(t *testing.T) {
	truncate(t)

	setting := operation_setting.GetQiniuKeySetting()
	oldSetting := *setting
	setting.CostDetailAutoApplyEnabled = true
	setting.CostDetailCutoverTime = 0
	t.Cleanup(func() { *setting = oldSetting })

	token := seedQiniuOwnershipToken(t, 5341, 5341, "bucket-cutover-123456")
	record := seedQiniuCostDetailRawRecord(t, "bucket-cutover", "sk-cut***123456", "cut", "123456", "2026-06-05", 0.25)
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", record.Id).Updates(map[string]interface{}{
		"owner_status": model.QiniuBillingOwnerStatusResolved,
		"user_id":      token.UserId,
		"token_id":     token.Id,
	}).Error)

	bucket, err := RecalculateQiniuBillingBucket(token.Id, record.BillingDate)
	requireNoError(t, err)
	if bucket.Status != model.QiniuBillingBucketStatusSkipped || bucket.LastError != "cutover_not_configured" {
		t.Fatalf("expected missing cutover to keep bucket observation-only, got %#v", bucket)
	}
	var appCount int64
	requireNoError(t, model.DB.Model(&model.QiniuBillingBucketApplication{}).Where("bucket_id = ?", bucket.Id).Count(&appCount).Error)
	if appCount != 0 {
		t.Fatalf("missing cutover must not create application, got %d", appCount)
	}

	zeroStatus, zeroReason := qiniuBillingBucketStatusForPendingDelta(record.BillingDate, 0)
	if zeroStatus != model.QiniuBillingBucketStatusSkipped || zeroReason != qiniuCostDetailCutoverNotConfiguredReason {
		t.Fatalf("zero delta must still respect missing cutover guard, status=%s reason=%s", zeroStatus, zeroReason)
	}

	setting.CostDetailCutoverTime = time.Date(2026, 6, 6, 0, 0, 0, 0, qiniuCSTLocation).Unix()
	zeroStatus, zeroReason = qiniuBillingBucketStatusForPendingDelta(record.BillingDate, 0)
	if zeroStatus != model.QiniuBillingBucketStatusSkipped || zeroReason != qiniuCostDetailBeforeCutoverReason {
		t.Fatalf("zero delta must still respect before-cutover guard, status=%s reason=%s", zeroStatus, zeroReason)
	}

	// 2026-06-04 16:30 UTC 在平台账务时区为 2026-06-05，必须允许 6 月 5 日账单自动落账。
	setting.CostDetailCutoverTime = time.Date(2026, 6, 4, 16, 30, 0, 0, time.UTC).Unix()
	bucket, err = RecalculateQiniuBillingBucket(token.Id, record.BillingDate)
	requireNoError(t, err)
	if bucket.Status != model.QiniuBillingBucketStatusApplied || bucket.PendingDeltaQuota != 0 {
		t.Fatalf("expected cutover billing date to allow same-day automatic application, got %#v", bucket)
	}
	requireNoError(t, model.DB.Model(&model.QiniuBillingBucketApplication{}).Where("bucket_id = ?", bucket.Id).Count(&appCount).Error)
	if appCount != 1 {
		t.Fatalf("expected one automatic bucket application after cutover, got %d", appCount)
	}
}

func TestRecalculateQiniuBillingBucketPreservesNegativeOfficialAmount(t *testing.T) {
	truncate(t)

	setting := operation_setting.GetQiniuKeySetting()
	oldSetting := *setting
	setting.CostDetailAutoApplyEnabled = false
	t.Cleanup(func() { *setting = oldSetting })

	token := seedQiniuOwnershipToken(t, 5321, 5321, "bucket-negative-123456")
	record := seedQiniuCostDetailRawRecord(t, "bucket-negative", "sk-neg***123456", "neg", "123456", "2026-06-04", -0.20)
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("id = ?", record.Id).Updates(map[string]interface{}{
		"owner_status": model.QiniuBillingOwnerStatusResolved,
		"user_id":      token.UserId,
		"token_id":     token.Id,
	}).Error)

	bucket, err := RecalculateQiniuBillingBucket(token.Id, record.BillingDate)
	requireNoError(t, err)
	expectedQuota := -amountToQuota(0.20)
	if bucket.OfficialQuota != expectedQuota || bucket.PendingDeltaQuota != expectedQuota {
		t.Fatalf("expected negative official amount to remain negative, got %#v", bucket)
	}
}
