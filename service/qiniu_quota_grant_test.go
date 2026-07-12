package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestCreateQiniuQuotaGrantForRechargeAndCommissionIsIdempotent(t *testing.T) {
	truncate(t)
	setting := operation_setting.GetQiniuKeySetting()
	oldSetting := *setting
	setting.Enabled = true
	t.Cleanup(func() { *setting = oldSetting })

	token := seedQiniuBucketApplicationUserToken(t, 5501, 5501, 0, 0, 0)
	requireNoError(t, CreateQiniuQuotaGrantForRecharge(token.UserId, "trade-5501", amountToQuota(1.25)))
	requireNoError(t, CreateQiniuQuotaGrantForRecharge(token.UserId, "trade-5501", amountToQuota(1.25)))
	requireNoError(t, CreateQiniuQuotaGrantForCommissionTransfer(token.UserId, 9001, 0.75))
	requireNoError(t, CreateQiniuQuotaGrantForCommissionTransfer(token.UserId, 9001, 0.75))

	var grants []model.QiniuQuotaGrant
	requireNoError(t, model.DB.Order("business_key asc").Find(&grants).Error)
	if len(grants) != 2 {
		t.Fatalf("expected two idempotent quota grants, got %#v", grants)
	}
	if grants[0].BusinessKey != "commission_transfer:9001" || grants[1].BusinessKey != "recharge:trade-5501" {
		t.Fatalf("unexpected grant business keys: %#v", grants)
	}
	var taskCount int64
	requireNoError(t, model.DB.Model(&model.QiniuKeySyncTask{}).Count(&taskCount).Error)
	if taskCount != 0 {
		t.Fatalf("quota grant creation must not enqueue legacy quota sync task, got %d", taskCount)
	}
}

func TestApplyQiniuQuotaGrantUsesCumulativeGrantLimitWithoutUsedQuery(t *testing.T) {
	truncate(t)

	token := seedQiniuBucketApplicationUserToken(t, 5511, 5511, 0, 0, 0)
	requireNoError(t, model.DB.Create(&model.QiniuQuotaGrant{
		UserId:            token.UserId,
		TokenId:           token.Id,
		BusinessKey:       "recharge:already-applied",
		GrantAmount:       1.25,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusApplied,
	}).Error)
	pending := &model.QiniuQuotaGrant{
		UserId:            token.UserId,
		TokenId:           token.Id,
		BusinessKey:       "recharge:pending",
		GrantAmount:       0.75,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusPending,
	}
	requireNoError(t, model.DB.Create(pending).Error)

	var observedLimit float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v2/stat/usage") {
			t.Fatalf("quota grant must not query per-key used amount: %s", r.URL.String())
		}
		if r.Method != http.MethodPut || !strings.HasPrefix(r.URL.Path, "/v1/apikey/quota/") {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		var body qiniuQuotaLimitRequest
		if err := common.DecodeJson(r.Body, &body); err != nil {
			t.Fatalf("failed to decode qiniu quota request: %v", err)
		}
		observedLimit = body.TotalQuota.Limit
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	operation_setting.GetQiniuKeySetting().Enabled = true

	applied, skipped, err := ApplyQiniuQuotaGrant(context.Background(), pending.Id)
	requireNoError(t, err)
	if !applied || skipped {
		t.Fatalf("expected quota grant applied, applied=%t skipped=%t", applied, skipped)
	}
	if observedLimit != 2.0 {
		t.Fatalf("expected cumulative grant limit 2.0, got %.6f", observedLimit)
	}
	var updated model.QiniuQuotaGrant
	requireNoError(t, model.DB.First(&updated, "id = ?", pending.Id).Error)
	if updated.RemoteApplyStatus != model.QiniuQuotaGrantStatusApplied || updated.RemoteApplyTime == 0 {
		t.Fatalf("expected applied grant status, got %#v", updated)
	}
}

func TestApplyQiniuQuotaGrantUsesTokenChildAccountIdentity(t *testing.T) {
	truncate(t)

	token := seedQiniuBucketApplicationUserToken(t, 5513, 5513, 0, 0, 0)
	account := seedQiniuIdentityClientAccount(t, 811, model.QiniuChildAccountStatusEnabled, "child-grant-ak", "child-grant-sk")
	requireNoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Update("qiniu_child_account_id", account.Id).Error)
	pending := &model.QiniuQuotaGrant{
		UserId:            token.UserId,
		TokenId:           token.Id,
		BusinessKey:       "recharge:child-account-grant",
		GrantAmount:       0.75,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusPending,
	}
	requireNoError(t, model.DB.Create(pending).Error)

	var observedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || !strings.HasPrefix(r.URL.Path, "/v1/apikey/quota/") {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		observedAuth = r.Header.Get("Authorization")
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	operation_setting.GetQiniuKeySetting().Enabled = true

	applied, skipped, err := ApplyQiniuQuotaGrant(context.Background(), pending.Id)
	requireNoError(t, err)
	if !applied || skipped {
		t.Fatalf("expected quota grant applied, applied=%t skipped=%t", applied, skipped)
	}
	if !strings.HasPrefix(observedAuth, "Qiniu child-grant-ak:") {
		t.Fatalf("expected child account authorization, got %q", observedAuth)
	}
}

func TestApplyQiniuQuotaGrantIncludesInitialBaseline(t *testing.T) {
	truncate(t)

	token := seedQiniuBucketApplicationUserToken(t, 5512, 5512, 0, 0, 0)
	requireNoError(t, model.DB.Create(&model.QiniuQuotaGrant{
		UserId:            token.UserId,
		TokenId:           token.Id,
		BusinessKey:       qiniuInitialQuotaBaselineBusinessKey(token.Id),
		GrantAmount:       5.00,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusApplied,
		RemoteApplyTime:   common.GetTimestamp(),
	}).Error)
	requireNoError(t, model.DB.Create(&model.QiniuQuotaGrant{
		UserId:            token.UserId,
		TokenId:           token.Id,
		BusinessKey:       "recharge:already-applied-with-baseline",
		GrantAmount:       1.25,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusApplied,
	}).Error)
	pending := &model.QiniuQuotaGrant{
		UserId:            token.UserId,
		TokenId:           token.Id,
		BusinessKey:       "commission_transfer:pending-with-baseline",
		GrantAmount:       0.75,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusPending,
	}
	requireNoError(t, model.DB.Create(pending).Error)

	var observedLimit float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v2/stat/usage") {
			t.Fatalf("quota grant must not query per-key used amount: %s", r.URL.String())
		}
		if r.Method != http.MethodPut || !strings.HasPrefix(r.URL.Path, "/v1/apikey/quota/") {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		var body qiniuQuotaLimitRequest
		requireNoError(t, common.DecodeJson(r.Body, &body))
		observedLimit = body.TotalQuota.Limit
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	operation_setting.GetQiniuKeySetting().Enabled = true

	applied, skipped, err := ApplyQiniuQuotaGrant(context.Background(), pending.Id)
	requireNoError(t, err)
	if !applied || skipped {
		t.Fatalf("expected quota grant applied, applied=%t skipped=%t", applied, skipped)
	}
	if observedLimit != 7.0 {
		t.Fatalf("expected baseline plus grants limit 7.0, got %.6f", observedLimit)
	}
}

func TestScanPendingQiniuQuotaGrantsAppliesPendingAndDeferredGrants(t *testing.T) {
	truncate(t)

	token := seedQiniuBucketApplicationUserToken(t, 5521, 5521, 0, 0, 0)
	requireNoError(t, model.DB.Create(&model.QiniuQuotaGrant{
		UserId:            token.UserId,
		TokenId:           token.Id,
		BusinessKey:       "recharge:already-applied",
		GrantAmount:       1.00,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusApplied,
	}).Error)
	requireNoError(t, model.DB.Create(&model.QiniuQuotaGrant{
		UserId:            token.UserId,
		TokenId:           0,
		BusinessKey:       "recharge:deferred-token",
		GrantAmount:       0.50,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusPending,
	}).Error)

	var observedLimit float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v2/stat/usage") {
			t.Fatalf("quota grant scan must not query per-key used amount: %s", r.URL.String())
		}
		if r.Method != http.MethodPut || !strings.HasPrefix(r.URL.Path, "/v1/apikey/quota/") {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		var body qiniuQuotaLimitRequest
		requireNoError(t, common.DecodeJson(r.Body, &body))
		observedLimit = body.TotalQuota.Limit
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	operation_setting.GetQiniuKeySetting().Enabled = true

	result, err := ScanPendingQiniuQuotaGrants(context.Background(), 10)
	requireNoError(t, err)
	if result.ProcessedCount != 1 || result.AppliedCount != 1 || result.FailedCount != 0 {
		t.Fatalf("expected one deferred grant applied, got %#v", result)
	}
	if observedLimit != 1.50 {
		t.Fatalf("expected cumulative grant limit 1.50, got %.6f", observedLimit)
	}
	var grant model.QiniuQuotaGrant
	requireNoError(t, model.DB.First(&grant, "business_key = ?", "recharge:deferred-token").Error)
	if grant.TokenId != token.Id || grant.RemoteApplyStatus != model.QiniuQuotaGrantStatusApplied {
		t.Fatalf("expected deferred grant to bind token and apply, got %#v", grant)
	}
}

func TestScanPendingQiniuQuotaGrantsSkipsRecentFailedGrants(t *testing.T) {
	truncate(t)

	token := seedQiniuBucketApplicationUserToken(t, 5531, 5531, 0, 0, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("recent failed quota grant must wait for retry interval before calling qiniu: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	setting := operation_setting.GetQiniuKeySetting()
	setting.Enabled = true
	setting.OfficialLedgerRetryIntervalSeconds = int((10 * time.Minute).Seconds())

	requireNoError(t, model.DB.Create(&model.QiniuQuotaGrant{
		UserId:            token.UserId,
		TokenId:           token.Id,
		BusinessKey:       "recharge:recent-failed",
		GrantAmount:       0.75,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusFailed,
		LastError:         "temporary failure",
		UpdatedTime:       common.GetTimestamp(),
	}).Error)

	result, err := ScanPendingQiniuQuotaGrants(context.Background(), 10)
	requireNoError(t, err)
	if result.ProcessedCount != 0 || result.AppliedCount != 0 || result.FailedCount != 0 {
		t.Fatalf("expected recent failed grant to stay deferred, got %#v", result)
	}
}
