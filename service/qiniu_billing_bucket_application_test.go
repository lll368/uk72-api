package service

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var qiniuBucketApplicationSeedDay int64

func TestApplyQiniuBillingBucketPositiveDeltaCanCreateDebtAndIsIdempotent(t *testing.T) {
	truncate(t)
	configureQiniuCostDetailCutoverForTest(t, "2026-06-01")

	token := seedQiniuBucketApplicationUserToken(t, 5401, 5401, 500, 0, 0)
	bucket := seedQiniuBillingBucketForApplication(t, token, 1000)
	requireNoError(t, model.DB.Model(&model.QiniuBillingBucket{}).Where("id = ?", bucket.Id).Updates(map[string]interface{}{
		"official_amount":       quotaToWalletAmount(1500),
		"official_quota":        1500,
		"local_realtime_quota":  500,
		"pending_delta_quota":   1000,
		"local_realtime_status": model.QiniuBillingLocalRealtimeStatusFound,
	}).Error)

	applied, skipped, err := ApplyQiniuBillingBucket(context.Background(), bucket.Id)
	requireNoError(t, err)
	if !applied || skipped {
		t.Fatalf("expected bucket applied, applied=%t skipped=%t", applied, skipped)
	}
	var user model.User
	requireNoError(t, model.DB.First(&user, "id = ?", token.UserId).Error)
	if user.Quota != -500 || user.UsedQuota != 1000 {
		t.Fatalf("expected debt quota and used quota update, got %#v", user)
	}
	var updatedToken model.Token
	requireNoError(t, model.DB.First(&updatedToken, "id = ?", token.Id).Error)
	if updatedToken.UsedQuota != 1000 {
		t.Fatalf("expected token used quota 1000, got %#v", updatedToken)
	}
	var account model.WalletAccount
	requireNoError(t, model.DB.First(&account, "user_id = ?", token.UserId).Error)
	if account.BalanceAmount >= 0 {
		t.Fatalf("expected negative wallet balance, got %#v", account)
	}
	var app model.QiniuBillingBucketApplication
	requireNoError(t, model.DB.First(&app, "bucket_id = ?", bucket.Id).Error)
	if app.DebtQuota != 500 || app.IdempotencyKey != model.QiniuBillingBucketIdempotencyKey(bucket.Id, 1) || app.WalletFlowId == 0 || app.ConsumeLogId == 0 {
		t.Fatalf("unexpected application audit: %#v", app)
	}
	var log model.Log
	requireNoError(t, model.LOG_DB.First(&log, "id = ?", app.ConsumeLogId).Error)
	assertNoSupplierBrandInUserText(t, "bucket settlement log content", log.Content)
	assertNoSupplierBrandInUserText(t, "bucket settlement log model_name", log.ModelName)
	var other map[string]interface{}
	requireNoError(t, common.UnmarshalJsonStr(log.Other, &other))
	if other["local_realtime_amount"] != quotaToWalletAmount(500) {
		t.Fatalf("expected synthetic log to include local realtime amount, got %#v", other)
	}
	if other["billing_source"] != QiniuCostDetailBucketBillingSource || other["debt"] != true || int(other["debt_quota"].(float64)) != 500 {
		t.Fatalf("expected synthetic log to include cost-detail debt context, got %#v", other)
	}
	var flowCount int64
	requireNoError(t, model.DB.Model(&model.WalletFlow{}).Where("flow_type = ? AND biz_no = ?", model.WalletFlowTypeBalanceConsume, app.IdempotencyKey).Count(&flowCount).Error)
	if flowCount != 1 {
		t.Fatalf("expected one bucket consume wallet flow, got %d", flowCount)
	}
	var flow model.WalletFlow
	requireNoError(t, model.DB.First(&flow, "flow_type = ? AND biz_no = ?", model.WalletFlowTypeBalanceConsume, app.IdempotencyKey).Error)
	assertNoSupplierBrandInUserText(t, "bucket settlement wallet remark", flow.Remark)
	if !strings.Contains(flow.Remark, "debt=500") {
		t.Fatalf("expected wallet remark to expose debt context, got %q", flow.Remark)
	}
	var taskCount int64
	requireNoError(t, model.DB.Model(&model.QiniuKeySyncTask{}).Count(&taskCount).Error)
	if taskCount != 0 {
		t.Fatalf("bucket settlement must not create qiniu quota sync task, got %d", taskCount)
	}
	var grantCount int64
	requireNoError(t, model.DB.Model(&model.QiniuQuotaGrant{}).Count(&grantCount).Error)
	if grantCount != 0 {
		t.Fatalf("bucket settlement must not create qiniu quota grant, got %d", grantCount)
	}

	applied, skipped, err = ApplyQiniuBillingBucket(context.Background(), bucket.Id)
	requireNoError(t, err)
	if applied || !skipped {
		t.Fatalf("expected repeated application to skip, applied=%t skipped=%t", applied, skipped)
	}
	requireNoError(t, model.DB.Model(&model.QiniuBillingBucketApplication{}).Where("bucket_id = ?", bucket.Id).Count(&flowCount).Error)
	if flowCount != 1 {
		t.Fatalf("expected one application after retry, got %d", flowCount)
	}
}

func TestQiniuBillingBucketWalletRemarkMarksDirectSyncWhenLocalRealtimeMissing(t *testing.T) {
	bucket := &model.QiniuBillingBucket{
		Id:                  5441,
		BillingDate:         "2026-06-05",
		LocalRealtimeStatus: model.QiniuBillingLocalRealtimeStatusMissing,
	}

	remark := qiniuBillingBucketWalletRemark(bucket, 1000, 0)

	assertNoSupplierBrandInUserText(t, "direct-sync bucket wallet remark", remark)
	if !strings.Contains(remark, "官方同步") || strings.Contains(remark, "local_realtime_status") || strings.Contains(remark, "source=official_sync") {
		t.Fatalf("expected user-facing official sync remark without internal markers, got %q", remark)
	}
}

func TestApplyQiniuBillingBucketNegativeAndZeroDelta(t *testing.T) {
	truncate(t)
	configureQiniuCostDetailCutoverForTest(t, "2026-06-01")

	token := seedQiniuBucketApplicationUserToken(t, 5411, 5411, 100, 500, 300)
	refundBucket := seedQiniuBillingBucketForApplication(t, token, -400)
	applied, skipped, err := ApplyQiniuBillingBucket(context.Background(), refundBucket.Id)
	requireNoError(t, err)
	if !applied || skipped {
		t.Fatalf("expected refund bucket applied, applied=%t skipped=%t", applied, skipped)
	}
	var user model.User
	requireNoError(t, model.DB.First(&user, "id = ?", token.UserId).Error)
	if user.Quota != 500 || user.UsedQuota != 100 {
		t.Fatalf("expected refunded quota and user used quota floor, got %#v", user)
	}
	var updatedToken model.Token
	requireNoError(t, model.DB.First(&updatedToken, "id = ?", token.Id).Error)
	if updatedToken.UsedQuota != 0 {
		t.Fatalf("expected token used quota floor, got %#v", updatedToken)
	}
	var refundFlowCount int64
	requireNoError(t, model.DB.Model(&model.WalletFlow{}).Where("flow_type = ?", model.WalletFlowTypeBalanceRefund).Count(&refundFlowCount).Error)
	if refundFlowCount != 1 {
		t.Fatalf("expected one refund wallet flow, got %d", refundFlowCount)
	}

	zeroBucket := seedQiniuBillingBucketForApplication(t, token, 0)
	applied, skipped, err = ApplyQiniuBillingBucket(context.Background(), zeroBucket.Id)
	requireNoError(t, err)
	if !applied || skipped {
		t.Fatalf("expected zero bucket reconciled, applied=%t skipped=%t", applied, skipped)
	}
	var zeroApp model.QiniuBillingBucketApplication
	requireNoError(t, model.DB.First(&zeroApp, "bucket_id = ?", zeroBucket.Id).Error)
	if zeroApp.WalletFlowId != 0 || zeroApp.DeltaQuota != 0 {
		t.Fatalf("zero delta must not write wallet flow, got %#v", zeroApp)
	}
	requireNoError(t, model.DB.First(zeroBucket, "id = ?", zeroBucket.Id).Error)
	if zeroBucket.Status != model.QiniuBillingBucketStatusReconciled {
		t.Fatalf("expected zero bucket reconciled, got %#v", zeroBucket)
	}
}

func TestApplyQiniuBillingBucketZeroDeltaRequiresCutover(t *testing.T) {
	truncate(t)

	setting := operation_setting.GetQiniuKeySetting()
	oldSetting := *setting
	setting.CostDetailAutoApplyEnabled = true
	setting.CostDetailCutoverTime = 0
	t.Cleanup(func() { *setting = oldSetting })

	token := seedQiniuBucketApplicationUserToken(t, 5412, 5412, 100, 0, 0)
	zeroBucket := seedQiniuBillingBucketForApplication(t, token, 0)
	applied, skipped, err := ApplyQiniuBillingBucket(context.Background(), zeroBucket.Id)
	if err == nil || !strings.Contains(err.Error(), qiniuCostDetailCutoverNotConfiguredReason) || applied || skipped {
		t.Fatalf("expected zero delta to require cutover guard, applied=%t skipped=%t err=%v", applied, skipped, err)
	}
	var appCount int64
	requireNoError(t, model.DB.Model(&model.QiniuBillingBucketApplication{}).Where("bucket_id = ?", zeroBucket.Id).Count(&appCount).Error)
	if appCount != 0 {
		t.Fatalf("zero delta before cutover must not create application, got %d", appCount)
	}
	requireNoError(t, model.DB.First(zeroBucket, "id = ?", zeroBucket.Id).Error)
	if zeroBucket.Status != model.QiniuBillingBucketStatusSkipped || zeroBucket.LastError != qiniuCostDetailCutoverNotConfiguredReason {
		t.Fatalf("expected zero delta before cutover to stay observation-only, got %#v", zeroBucket)
	}
}

func TestApplyQiniuBillingBucketRecordsFailedApplicationAndRetriesSameVersion(t *testing.T) {
	truncate(t)
	configureQiniuCostDetailCutoverForTest(t, "2026-06-01")

	token := seedQiniuBucketApplicationUserToken(t, 5421, 5421, 500, 0, 0)
	missingTokenId := 5422
	bucket := seedQiniuBillingBucketForApplication(t, token, 1000)
	requireNoError(t, model.DB.Model(&model.QiniuBillingBucket{}).Where("id = ?", bucket.Id).Update("token_id", missingTokenId).Error)

	applied, skipped, err := ApplyQiniuBillingBucket(context.Background(), bucket.Id)
	if err == nil || applied || skipped {
		t.Fatalf("expected first application to fail without token, applied=%t skipped=%t err=%v", applied, skipped, err)
	}
	var failedApp model.QiniuBillingBucketApplication
	requireNoError(t, model.DB.First(&failedApp, "bucket_id = ?", bucket.Id).Error)
	if failedApp.Status != model.QiniuBillingApplicationStatusFailed || failedApp.ApplyVersion != 1 || failedApp.LastError == "" {
		t.Fatalf("expected failed application audit for v1, got %#v", failedApp)
	}
	if failedApp.RetryCount != 1 || failedApp.LastRetryTime == 0 || failedApp.NextRetryTime == 0 {
		t.Fatalf("expected failed application retry context, got %#v", failedApp)
	}

	requireNoError(t, model.DB.Create(&model.Token{
		Id:             missingTokenId,
		UserId:         token.UserId,
		Name:           "qiniu-bucket-apply-token-restored",
		Key:            "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	applied, skipped, err = ApplyQiniuBillingBucket(context.Background(), bucket.Id)
	requireNoError(t, err)
	if !applied || skipped {
		t.Fatalf("expected retry to apply failed v1 application, applied=%t skipped=%t", applied, skipped)
	}
	var retriedApp model.QiniuBillingBucketApplication
	requireNoError(t, model.DB.First(&retriedApp, "id = ?", failedApp.Id).Error)
	if retriedApp.Status != model.QiniuBillingApplicationStatusSuccess || retriedApp.WalletFlowId == 0 || retriedApp.ConsumeLogId == 0 {
		t.Fatalf("expected failed application row to become success, got %#v", retriedApp)
	}
	if retriedApp.LastError != "" || retriedApp.NextRetryTime != 0 {
		t.Fatalf("expected successful retry to clear failed retry markers, got %#v", retriedApp)
	}
	var appCount int64
	requireNoError(t, model.DB.Model(&model.QiniuBillingBucketApplication{}).Where("bucket_id = ?", bucket.Id).Count(&appCount).Error)
	if appCount != 1 {
		t.Fatalf("expected retry to reuse failed application row, got %d", appCount)
	}
}

func TestAdminRetryQiniuBillingBucketApplicationRequiresFailedCurrentApplication(t *testing.T) {
	truncate(t)
	configureQiniuCostDetailCutoverForTest(t, "2026-06-01")

	token := seedQiniuBucketApplicationUserToken(t, 5431, 5431, 500, 0, 0)
	successBucket := seedQiniuBillingBucketForApplication(t, token, 1000)
	successApp := &model.QiniuBillingBucketApplication{
		BucketId:       successBucket.Id,
		ApplyVersion:   1,
		DeltaQuota:     1000,
		IdempotencyKey: model.QiniuBillingBucketIdempotencyKey(successBucket.Id, 1),
		Status:         model.QiniuBillingApplicationStatusSuccess,
	}
	requireNoError(t, model.DB.Create(successApp).Error)
	_, err := AdminRetryQiniuBillingBucketApplication(context.Background(), successApp.Id, 1, "not failed")
	if err == nil || !strings.Contains(err.Error(), "只能重试失败") {
		t.Fatalf("expected admin retry to reject successful application, err=%v", err)
	}

	staleBucket := seedQiniuBillingBucketForApplication(t, token, 1000)
	requireNoError(t, model.DB.Model(&model.QiniuBillingBucket{}).Where("id = ?", staleBucket.Id).Update("apply_version", 1).Error)
	staleApp := &model.QiniuBillingBucketApplication{
		BucketId:       staleBucket.Id,
		ApplyVersion:   1,
		DeltaQuota:     1000,
		IdempotencyKey: model.QiniuBillingBucketIdempotencyKey(staleBucket.Id, 1),
		Status:         model.QiniuBillingApplicationStatusFailed,
		LastError:      "old failure",
	}
	requireNoError(t, model.DB.Create(staleApp).Error)
	_, err = AdminRetryQiniuBillingBucketApplication(context.Background(), staleApp.Id, 1, "stale failed")
	if err == nil || !strings.Contains(err.Error(), "不是当前待应用版本") {
		t.Fatalf("expected admin retry to reject stale failed application, err=%v", err)
	}

	currentBucket := seedQiniuBillingBucketForApplication(t, token, 1000)
	currentApp := &model.QiniuBillingBucketApplication{
		BucketId:        currentBucket.Id,
		ApplyVersion:    1,
		DeltaQuota:      1000,
		DeltaAmount:     signedWalletAmount(1000),
		IdempotencyKey:  model.QiniuBillingBucketIdempotencyKey(currentBucket.Id, 1),
		Status:          model.QiniuBillingApplicationStatusFailed,
		LastError:       "retryable failure",
		OperationSource: model.QiniuBillingOperationSourceAdmin,
	}
	requireNoError(t, model.DB.Create(currentApp).Error)
	result, err := AdminRetryQiniuBillingBucketApplication(context.Background(), currentApp.Id, 1, "retry current")
	requireNoError(t, err)
	if !result.Applied || result.ApplicationId != currentApp.Id {
		t.Fatalf("expected admin retry to apply selected application, got %#v", result)
	}
	var retriedApp model.QiniuBillingBucketApplication
	requireNoError(t, model.DB.First(&retriedApp, "id = ?", currentApp.Id).Error)
	if retriedApp.Status != model.QiniuBillingApplicationStatusSuccess || retriedApp.OperationSource != model.QiniuBillingOperationSourceAdmin {
		t.Fatalf("expected admin retry to preserve admin source, got %#v", retriedApp)
	}
}

func seedQiniuBucketApplicationUserToken(t *testing.T, userId int, tokenId int, quota int, usedQuota int, tokenUsedQuota int) *model.Token {
	t.Helper()
	requireNoError(t, model.DB.Create(&model.User{
		Id:        userId,
		Username:  "qiniu_bucket_apply_user_" + common.GetRandomString(8),
		AffCode:   "qba_" + common.GetRandomString(8),
		Quota:     quota,
		UsedQuota: usedQuota,
		Status:    common.UserStatusEnabled,
	}).Error)
	token := &model.Token{
		Id:             tokenId,
		UserId:         userId,
		Name:           "qiniu-bucket-apply-token",
		Key:            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		UsedQuota:      tokenUsedQuota,
	}
	requireNoError(t, model.DB.Create(token).Error)
	return token
}

func assertNoSupplierBrandInUserText(t *testing.T, name string, value string) {
	t.Helper()
	if strings.Contains(value, "七牛") || strings.Contains(strings.ToLower(value), "qiniu") {
		t.Fatalf("%s must not expose supplier brand, got %q", name, value)
	}
}

func seedQiniuBillingBucketForApplication(t *testing.T, token *model.Token, pendingDeltaQuota int) *model.QiniuBillingBucket {
	t.Helper()
	day := int(atomic.AddInt64(&qiniuBucketApplicationSeedDay, 1)%20) + 1
	bucket := &model.QiniuBillingBucket{
		UserId:            token.UserId,
		TokenId:           token.Id,
		BillingDate:       fmt.Sprintf("2026-06-%02d", day),
		QiniuMaskedKey:    "sk-app***123456",
		KeyFingerprint:    "fingerprint",
		OwnerStatus:       model.QiniuBillingOwnerStatusResolved,
		OfficialQuota:     pendingDeltaQuota,
		PendingDeltaQuota: pendingDeltaQuota,
		Status:            model.QiniuBillingBucketStatusPending,
	}
	requireNoError(t, model.DB.Create(bucket).Error)
	return bucket
}
