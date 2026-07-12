package service

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func seedQiniuLedgerUserToken(t *testing.T, userId int, tokenId int, quota int, usedQuota int, tokenUsedQuota int) {
	t.Helper()
	requireNoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: fmt.Sprintf("qiniu_ledger_user_%d", userId),
		AffCode:  fmt.Sprintf("qiniu_ledger_aff_%d", userId),
		Quota:    quota,
		Status:   common.UserStatusEnabled,
	}).Error)
	requireNoError(t, model.DB.Model(&model.User{}).Where("id = ?", userId).Updates(map[string]interface{}{"used_quota": usedQuota}).Error)
	requireNoError(t, model.DB.Create(&model.Token{
		Id:             tokenId,
		UserId:         userId,
		Name:           "qiniu-token",
		Key:            fmt.Sprintf("qiniu-ledger-token-%d", tokenId),
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		UsedQuota:      tokenUsedQuota,
	}).Error)
	requireNoError(t, model.DB.Create(&model.WalletAccount{
		UserId:        userId,
		BalanceAmount: quotaToWalletAmount(quota),
	}).Error)
}

func seedQiniuOfficialBillRecord(t *testing.T, userId int, tokenId int, officialQuota int, appliedQuota int, applyVersion int) *model.QiniuOfficialUsageRecord {
	return seedQiniuOfficialBillRecordWithItem(t, userId, tokenId, officialQuota, appliedQuota, applyVersion, "input")
}

func seedQiniuOfficialBillRecordWithItem(t *testing.T, userId int, tokenId int, officialQuota int, appliedQuota int, applyVersion int, billingItem string) *model.QiniuOfficialUsageRecord {
	t.Helper()
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, qiniuCSTLocation)
	record := &model.QiniuOfficialUsageRecord{
		RecordKey:     qiniuOfficialRecordKey("test", userId, tokenId, officialQuota, appliedQuota, applyVersion, common.GetTimeString()),
		RecordType:    model.QiniuOfficialRecordTypeBill,
		SourceAPI:     qiniuAPIKeyUsagePath,
		QiniuKey:      fmt.Sprintf("sk-ledger-test-%d", tokenId),
		UserId:        userId,
		TokenId:       tokenId,
		PeriodStart:   now.Unix(),
		PeriodEnd:     now.AddDate(0, 0, 1).Unix(),
		Granularity:   "day",
		ModelName:     "deepseek-v3",
		BillingItem:   billingItem,
		FeeAmount:     quotaToWalletAmount(officialQuota),
		Currency:      "CNY",
		OfficialQuota: officialQuota,
		AppliedQuota:  appliedQuota,
		ApplyVersion:  applyVersion,
		Status:        model.QiniuOfficialRecordStatusPending,
		RawResponse:   `{"fee":1}`,
	}
	fillQiniuOfficialRecordIdentity(record)
	requireNoError(t, model.DB.Create(record).Error)
	return record
}

func TestApplyQiniuOfficialLedgerPositiveDeltaObservationOnly(t *testing.T) {
	truncate(t)
	seedQiniuLedgerUserToken(t, 4301, 4301, 10000, 0, 0)
	record := seedQiniuOfficialBillRecord(t, 4301, 4301, 2000, 0, 0)

	applied, skipped, err := ApplyQiniuOfficialLedgerRecord(context.Background(), record.Id)
	requireNoError(t, err)
	if applied || !skipped {
		t.Fatalf("expected official ledger auto apply skipped, applied=%v skipped=%v", applied, skipped)
	}
	assertQiniuOfficialLedgerNoSideEffects(t, 4301, 4301, 10000, 0, 0, record.Id, model.QiniuOfficialRecordStatusPending, 0)
}

func TestApplyQiniuOfficialLedgerNegativeDeltaObservationOnly(t *testing.T) {
	truncate(t)
	seedQiniuLedgerUserToken(t, 4302, 4302, 5000, 3000, 3000)
	record := seedQiniuOfficialBillRecord(t, 4302, 4302, 1000, 3000, 1)

	applied, skipped, err := ApplyQiniuOfficialLedgerRecord(context.Background(), record.Id)
	requireNoError(t, err)
	if applied || !skipped {
		t.Fatalf("expected official ledger refund apply skipped, applied=%v skipped=%v", applied, skipped)
	}
	assertQiniuOfficialLedgerNoSideEffects(t, 4302, 4302, 5000, 3000, 3000, record.Id, model.QiniuOfficialRecordStatusPending, 3000)
}

func TestApplyPendingQiniuOfficialLedgerRecordsObservationOnly(t *testing.T) {
	truncate(t)
	seedQiniuLedgerUserToken(t, 4310, 4310, 0, 0, 0)
	failedRecord := seedQiniuOfficialBillRecord(t, 4310, 4310, 2000, 0, 0)
	requireNoError(t, model.DB.Model(&model.QiniuOfficialUsageRecord{}).
		Where("id = ?", failedRecord.Id).
		Updates(map[string]interface{}{
			"status":       model.QiniuOfficialRecordStatusFailed,
			"last_error":   "insufficient balance",
			"updated_time": common.GetTimestamp(),
		}).Error)

	seedQiniuLedgerUserToken(t, 4311, 4311, 10000, 0, 0)
	pendingRecord := seedQiniuOfficialBillRecord(t, 4311, 4311, 1000, 0, 0)

	result, err := ApplyPendingQiniuOfficialLedgerRecords(context.Background(), 10)
	requireNoError(t, err)
	if result.ProcessedCount != 0 || result.AppliedCount != 0 || result.FailedCount != 0 {
		t.Fatalf("expected pending official ledger apply disabled, got %#v", result)
	}
	var reloadedPending model.QiniuOfficialUsageRecord
	requireNoError(t, model.DB.First(&reloadedPending, "id = ?", pendingRecord.Id).Error)
	if reloadedPending.Status != model.QiniuOfficialRecordStatusPending {
		t.Fatalf("expected pending record unchanged, got %#v", reloadedPending)
	}
	var reloadedFailed model.QiniuOfficialUsageRecord
	requireNoError(t, model.DB.First(&reloadedFailed, "id = ?", failedRecord.Id).Error)
	if reloadedFailed.Status != model.QiniuOfficialRecordStatusFailed {
		t.Fatalf("expected failed record unchanged, got %#v", reloadedFailed)
	}
	assertQiniuQuotaSyncTaskCount(t, 4310, 4310, 0)
	assertQiniuQuotaSyncTaskCount(t, 4311, 4311, 0)
}

func TestRepairQiniuOfficialLedgerLogsDisabled(t *testing.T) {
	truncate(t)
	seedQiniuLedgerUserToken(t, 4305, 4305, 10000, 0, 0)
	record := seedQiniuOfficialBillRecord(t, 4305, 4305, 2000, 0, 0)
	app := &model.QiniuOfficialLedgerApplication{
		UsageRecordId:  record.Id,
		ApplyVersion:   1,
		UserId:         4305,
		TokenId:        4305,
		DeltaQuota:     2000,
		DeltaAmount:    2,
		IdempotencyKey: "legacy-disabled-repair",
		Status:         model.QiniuOfficialLedgerStatusSuccess,
	}
	requireNoError(t, model.DB.Create(app).Error)

	repaired, err := RepairQiniuOfficialLedgerLogs(10)
	requireNoError(t, err)
	if repaired != 0 {
		t.Fatalf("expected official ledger log repair disabled, got %d", repaired)
	}
	exists, err := qiniuOfficialLedgerLogExists(record.Id)
	requireNoError(t, err)
	if exists {
		t.Fatalf("expected no synthetic official ledger log when repair is disabled")
	}
}

func TestRetryQiniuOfficialLedgerApplicationDisabled(t *testing.T) {
	truncate(t)
	seedQiniuLedgerUserToken(t, 4309, 4309, 10000, 0, 0)
	record := seedQiniuOfficialBillRecord(t, 4309, 4309, 2000, 0, 0)
	app := &model.QiniuOfficialLedgerApplication{
		UsageRecordId:  record.Id,
		ApplyVersion:   1,
		UserId:         4309,
		TokenId:        4309,
		DeltaQuota:     2000,
		DeltaAmount:    2,
		IdempotencyKey: "legacy-disabled-retry",
		Status:         model.QiniuOfficialLedgerStatusSuccess,
	}
	requireNoError(t, model.DB.Create(app).Error)

	result, err := RetryQiniuOfficialLedgerApplication(context.Background(), app.Id)
	requireNoError(t, err)
	if result.Applied || !result.Skipped || result.RepairedLog || result.Message == "" {
		t.Fatalf("expected retry application disabled, got %#v", result)
	}
	if got := getUserQuota(t, 4309); got != 10000 {
		t.Fatalf("expected user quota unchanged, got %d", got)
	}
	if got := getUserUsedQuota(t, 4309); got != 0 {
		t.Fatalf("expected user used quota unchanged, got %d", got)
	}
	assertQiniuQuotaSyncTaskCount(t, 4309, 4309, 0)
}

func assertQiniuOfficialLedgerNoSideEffects(t *testing.T, userId int, tokenId int, expectedQuota int, expectedUserUsed int, expectedTokenUsed int, recordId int, expectedStatus string, expectedAppliedQuota int) {
	t.Helper()
	if got := getUserQuota(t, userId); got != expectedQuota {
		t.Fatalf("expected user quota unchanged %d, got %d", expectedQuota, got)
	}
	if got := getUserUsedQuota(t, userId); got != expectedUserUsed {
		t.Fatalf("expected user used quota unchanged %d, got %d", expectedUserUsed, got)
	}
	if got := getTokenUsedQuota(t, tokenId); got != expectedTokenUsed {
		t.Fatalf("expected token used quota unchanged %d, got %d", expectedTokenUsed, got)
	}
	var reloaded model.QiniuOfficialUsageRecord
	requireNoError(t, model.DB.First(&reloaded, "id = ?", recordId).Error)
	if reloaded.Status != expectedStatus || reloaded.AppliedQuota != expectedAppliedQuota {
		t.Fatalf("expected official record unchanged status=%s applied=%d, got %#v", expectedStatus, expectedAppliedQuota, reloaded)
	}
	var appCount int64
	requireNoError(t, model.DB.Model(&model.QiniuOfficialLedgerApplication{}).Where("usage_record_id = ?", recordId).Count(&appCount).Error)
	if appCount != 0 {
		t.Fatalf("expected no ledger application, got %d", appCount)
	}
	var flowCount int64
	requireNoError(t, model.DB.Model(&model.WalletFlow{}).Count(&flowCount).Error)
	if flowCount != 0 {
		t.Fatalf("expected no wallet flow, got %d", flowCount)
	}
	var logCount int64
	requireNoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logCount).Error)
	if logCount != 0 {
		t.Fatalf("expected no synthetic log, got %d", logCount)
	}
	assertQiniuQuotaSyncTaskCount(t, userId, tokenId, 0)
}

func getUserUsedQuota(t *testing.T, userId int) int {
	t.Helper()
	var user model.User
	requireNoError(t, model.DB.Select("used_quota").Where("id = ?", userId).First(&user).Error)
	return user.UsedQuota
}

func assertQiniuQuotaSyncTaskCount(t *testing.T, userId int, tokenId int, expected int64) {
	t.Helper()
	var count int64
	requireNoError(t, model.DB.Model(&model.QiniuKeySyncTask{}).
		Where("user_id = ? AND token_id = ? AND task_type = ?", userId, tokenId, model.QiniuKeyTaskTypeQuotaSync).
		Count(&count).Error)
	if count != expected {
		t.Fatalf("expected %d qiniu quota sync task(s), got %d", expected, count)
	}
	if expected == 0 {
		return
	}
	var task model.QiniuKeySyncTask
	requireNoError(t, model.DB.Where("user_id = ? AND token_id = ? AND task_type = ?", userId, tokenId, model.QiniuKeyTaskTypeQuotaSync).First(&task).Error)
	if task.Status != model.QiniuKeyTaskStatusPending || task.QiniuKey == "" {
		t.Fatalf("unexpected quota sync task: %#v", task)
	}
}

func assertQiniuQuotaGrant(t *testing.T, userId int, tokenId int, businessKey string, grantAmount float64, status string) {
	t.Helper()
	var grant model.QiniuQuotaGrant
	requireNoError(t, model.DB.Where("business_key = ?", businessKey).First(&grant).Error)
	if grant.UserId != userId || grant.TokenId != tokenId {
		t.Fatalf("unexpected qiniu quota grant owner: %#v", grant)
	}
	if math.Abs(grant.GrantAmount-grantAmount) > 0.000001 {
		t.Fatalf("expected qiniu quota grant amount %.6f, got %#v", grantAmount, grant)
	}
	if grant.RemoteApplyStatus != status {
		t.Fatalf("expected qiniu quota grant status %s, got %#v", status, grant)
	}
}

func assertQiniuLedgerApplication(t *testing.T, recordId int, applyVersion int, deltaQuota int, flowType string) {
	t.Helper()
	var app model.QiniuOfficialLedgerApplication
	requireNoError(t, model.DB.Where("usage_record_id = ? AND apply_version = ?", recordId, applyVersion).First(&app).Error)
	if app.DeltaQuota != deltaQuota {
		t.Fatalf("expected delta %d, got %#v", deltaQuota, app)
	}
	if app.IdempotencyKey != model.QiniuOfficialLedgerIdempotencyKey(recordId, applyVersion) {
		t.Fatalf("unexpected ledger idempotency key: %#v", app)
	}
	if deltaQuota == 0 {
		return
	}
	if app.ConsumeLogId == 0 {
		t.Fatalf("expected synthetic consume/refund log id, got %#v", app)
	}
	if app.WalletFlowId == 0 {
		t.Fatalf("expected wallet flow id, got %#v", app)
	}
	var flow model.WalletFlow
	requireNoError(t, model.DB.First(&flow, "id = ?", app.WalletFlowId).Error)
	if flow.FlowType != flowType || flow.IdempotencyKey == nil || *flow.IdempotencyKey != app.IdempotencyKey {
		t.Fatalf("unexpected wallet flow: %#v app=%#v", flow, app)
	}
	var log model.Log
	requireNoError(t, model.LOG_DB.First(&log, "id = ?", app.ConsumeLogId).Error)
	expectedLogType := model.LogTypeConsume
	if deltaQuota < 0 {
		expectedLogType = model.LogTypeRefund
	}
	if log.Type != expectedLogType || log.Quota != int(math.Abs(float64(deltaQuota))) {
		t.Fatalf("unexpected synthetic ledger log: %#v", log)
	}
	other, err := common.StrToMap(log.Other)
	requireNoError(t, err)
	if other["billing_source"] != qiniuOfficialLedgerBillingSource || other["qiniu_official_ledger_log"] != true {
		t.Fatalf("expected official ledger markers in synthetic log, got %#v", other)
	}
}
