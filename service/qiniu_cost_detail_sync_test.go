package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func configureQiniuCostDetailCutoverForTest(t *testing.T, billingDate string) {
	t.Helper()
	setting := operation_setting.GetQiniuKeySetting()
	oldSetting := *setting
	cutoverDay, err := time.ParseInLocation("2006-01-02", billingDate, qiniuCSTLocation)
	requireNoError(t, err)
	setting.CostDetailCutoverTime = cutoverDay.Unix()
	setting.CostDetailAutoApplyEnabled = true
	t.Cleanup(func() {
		*setting = oldSetting
	})
}

func TestSyncQiniuCostDetailUsesTPlusOneLookbackAndUpsertsRawRecords(t *testing.T) {
	truncate(t)

	now := time.Date(2026, 6, 5, 10, 30, 0, 0, qiniuCSTLocation)
	requestedDates := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == qiniuOfficialUsagePath {
			t.Fatalf("cost-detail bucket sync must not call usage API")
		}
		if r.Method != http.MethodGet || r.URL.Path != qiniuAPIKeyUsagePath {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") == "" || r.Header.Get("Authorization")[:6] != "Qiniu " {
			t.Fatalf("expected AK/SK authorization for account-level cost-detail, got %q", r.Header.Get("Authorization"))
		}
		startDate := r.URL.Query().Get("start_date")
		requestedDates = append(requestedDates, startDate)
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data": map[string]any{
				"api_keys": []map[string]any{
					{
						"api_key": "sk-abc***123456",
						"bills": []map[string]any{
							{
								"date": startDate,
								"models": []map[string]any{
									{
										"model_id": "deepseek-v3",
										"items": []map[string]any{
											{
												"name":  "deepseek-v3输入",
												"key":   "input",
												"usage": map[string]any{"count": 1.5, "unit": "k/tokens"},
												"fee":   0.25,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)

	result, err := syncQiniuCostDetailAt(context.Background(), now)
	requireNoError(t, err)
	if result.WindowCount != 7 || result.RawRecordCount != 7 || result.InsertedCount != 7 || len(result.Errors) != 0 {
		t.Fatalf("unexpected first cost-detail sync result: %#v", result)
	}
	if result.AlertSummary.UnmappedCount != 7 || result.AlertSummary.LatestSuccessfulSyncTime == 0 || result.AlertSummary.LatestRetryResult == "" {
		t.Fatalf("expected sync alert summary for unresolved records, got %#v", result.AlertSummary)
	}
	sort.Strings(requestedDates)
	expectedDates := []string{"2026-05-29", "2026-05-30", "2026-05-31", "2026-06-01", "2026-06-02", "2026-06-03", "2026-06-04"}
	if !equalStringSlices(requestedDates, expectedDates) {
		t.Fatalf("expected T+1 seven-day lookback %v, got %v", expectedDates, requestedDates)
	}

	var records []model.QiniuCostDetailRecord
	requireNoError(t, model.DB.Order("billing_date asc").Find(&records).Error)
	if len(records) != 7 {
		t.Fatalf("expected 7 raw records, got %d", len(records))
	}
	for _, record := range records {
		if record.QiniuMaskedKey != "sk-abc***123456" || record.KeyPrefix != "abc" || record.KeySuffix != "123456" {
			t.Fatalf("unexpected masked key identity: %#v", record)
		}
		if record.OwnerStatus != model.QiniuBillingOwnerStatusUnmapped || record.Currency != "CNY" {
			t.Fatalf("expected unmapped CNY raw record, got %#v", record)
		}
	}

	result, err = syncQiniuCostDetailAt(context.Background(), now)
	requireNoError(t, err)
	if result.InsertedCount != 0 || result.RawRecordCount != 7 {
		t.Fatalf("expected repeated cost-detail sync to upsert existing raw records, got %#v", result)
	}
	var count int64
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Count(&count).Error)
	if count != 7 {
		t.Fatalf("expected raw record count to remain 7, got %d", count)
	}
}

func TestSyncQiniuCostDetailRecordsErrorsWithoutBlockingOtherDays(t *testing.T) {
	truncate(t)

	now := time.Date(2026, 6, 5, 10, 30, 0, 0, qiniuCSTLocation)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != qiniuAPIKeyUsagePath {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		startDate := r.URL.Query().Get("start_date")
		if startDate == "2026-06-03" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"temporary failure"}`))
			return
		}
		item := map[string]any{
			"name":  "deepseek-v3输入",
			"key":   "input",
			"usage": map[string]any{"count": 1.0, "unit": "k/tokens"},
			"fee":   0.01,
		}
		if startDate == "2026-06-02" {
			delete(item, "fee")
		}
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data": map[string]any{
				"api_keys": []map[string]any{
					{
						"api_key": "sk-def***654321",
						"bills": []map[string]any{
							{
								"date": startDate,
								"models": []map[string]any{
									{
										"model_id": "deepseek-v3",
										"items":    []map[string]any{item},
									},
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)

	result, err := syncQiniuCostDetailAt(context.Background(), now)
	requireNoError(t, err)
	if result.WindowCount != 7 || result.RawRecordCount != 5 || result.InsertedCount != 5 || len(result.Errors) != 2 {
		t.Fatalf("expected one successful day and two observable errors, got %#v", result)
	}
	if result.AlertSummary.UnmappedCount != 5 || result.AlertSummary.LatestError == "" {
		t.Fatalf("expected alert summary to include unmapped count and latest error, got %#v", result.AlertSummary)
	}
	var count int64
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Count(&count).Error)
	if count != 5 {
		t.Fatalf("expected five persisted raw records after partial failure, got %d", count)
	}
}

func TestSyncQiniuCostDetailUsesConfiguredLookbackDays(t *testing.T) {
	truncate(t)

	now := time.Date(2026, 6, 5, 10, 30, 0, 0, qiniuCSTLocation)
	requestedDates := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != qiniuAPIKeyUsagePath {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		requestedDates = append(requestedDates, r.URL.Query().Get("start_date"))
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data":   map[string]any{"api_keys": []map[string]any{}},
		})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	operation_setting.GetQiniuKeySetting().CostDetailLookbackDays = 4

	result, err := syncQiniuCostDetailAt(context.Background(), now)
	requireNoError(t, err)
	if result.WindowCount != 4 || len(result.Errors) != 0 {
		t.Fatalf("expected configured four-day lookback, got %#v", result)
	}
	sort.Strings(requestedDates)
	expectedDates := []string{"2026-06-01", "2026-06-02", "2026-06-03", "2026-06-04"}
	if !equalStringSlices(requestedDates, expectedDates) {
		t.Fatalf("expected configured lookback dates %v, got %v", expectedDates, requestedDates)
	}
}

func TestSyncQiniuCostDetailQueriesChildAccountTokenAndPersistsOwnership(t *testing.T) {
	truncate(t)

	keyBody := "kid" + strings.Repeat("2", 55) + "654321"
	fullKey := "sk-" + keyBody
	token := seedQiniuBucketApplicationUserToken(t, 5612, 5612, int(1*common.QuotaPerUnit), 0, 0)
	account := seedQiniuIdentityClientAccount(t, 814, model.QiniuChildAccountStatusDisabled, "child-cost-ak", "child-cost-sk")
	requireNoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]interface{}{
		"key":                    keyBody,
		"qiniu_child_account_id": account.Id,
	}).Error)

	now := time.Date(2026, 6, 5, 10, 30, 0, 0, qiniuCSTLocation)
	var parentAccountCalls int
	var childTokenCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != qiniuAPIKeyUsagePath {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		switch auth := r.Header.Get("Authorization"); {
		case strings.HasPrefix(auth, "Qiniu ak:"):
			parentAccountCalls++
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data":   map[string]any{"api_keys": []map[string]any{}},
			})
		case auth == "Bearer "+fullKey:
			childTokenCalls++
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"api_key": fullKey,
					"bills": []map[string]any{
						{
							"date": "2026-06-04",
							"models": []map[string]any{
								{
									"model_id": "deepseek-v3",
									"items": []map[string]any{
										{
											"name":  "deepseek-v3输入",
											"key":   "input",
											"usage": map[string]any{"count": 1.0, "unit": "k/tokens"},
											"fee":   0.01,
										},
									},
								},
							},
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected authorization %q", auth)
		}
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	setting := operation_setting.GetQiniuKeySetting()
	setting.CostDetailLookbackDays = 1
	setting.CostDetailAutoApplyEnabled = false

	result, err := syncQiniuCostDetailAt(context.Background(), now)
	requireNoError(t, err)
	if result.RawRecordCount != 1 || result.ResolvedCount != 1 || len(result.Errors) != 0 {
		t.Fatalf("unexpected cost-detail sync result: %#v", result)
	}
	if parentAccountCalls != 1 || childTokenCalls != 1 {
		t.Fatalf("expected one parent account call and one child token call, parent=%d child=%d", parentAccountCalls, childTokenCalls)
	}
	var record model.QiniuCostDetailRecord
	requireNoError(t, model.DB.First(&record, "token_id = ?", token.Id).Error)
	if record.QiniuChildAccountId != account.Id {
		t.Fatalf("expected raw record child account %d, got %#v", account.Id, record)
	}
	var bucket model.QiniuBillingBucket
	requireNoError(t, model.DB.First(&bucket, "token_id = ? AND billing_date = ?", token.Id, "2026-06-04").Error)
	if bucket.QiniuChildAccountId != account.Id {
		t.Fatalf("expected bucket child account %d, got %#v", account.Id, bucket)
	}
}

func TestSyncQiniuCostDetailDeduplicatesParentMaskedAndChildFullKeyBill(t *testing.T) {
	truncate(t)

	keyBody := "kid" + strings.Repeat("3", 55) + "654321"
	fullKey := "sk-" + keyBody
	maskedKey := "sk-kid********654321"
	token := seedQiniuBucketApplicationUserToken(t, 5613, 5613, int(1*common.QuotaPerUnit), 0, 0)
	account := seedQiniuIdentityClientAccount(t, 815, model.QiniuChildAccountStatusDisabled, "child-dedupe-ak", "child-dedupe-sk")
	requireNoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]interface{}{
		"key":                    keyBody,
		"qiniu_child_account_id": account.Id,
	}).Error)

	now := time.Date(2026, 6, 5, 10, 30, 0, 0, qiniuCSTLocation)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != qiniuAPIKeyUsagePath {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		apiKey := maskedKey
		if auth := r.Header.Get("Authorization"); auth == "Bearer "+fullKey {
			apiKey = fullKey
		} else if !strings.HasPrefix(auth, "Qiniu ak:") {
			t.Fatalf("unexpected authorization %q", auth)
		}
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data": map[string]any{
				"api_keys": []map[string]any{
					{
						"api_key": apiKey,
						"bills": []map[string]any{
							{
								"date": "2026-06-04",
								"models": []map[string]any{
									{
										"model_id": "deepseek-v3",
										"items": []map[string]any{
											{
												"name":  "deepseek-v3输入",
												"key":   "input",
												"usage": map[string]any{"count": 1.0, "unit": "k/tokens"},
												"fee":   0.01,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	setting := operation_setting.GetQiniuKeySetting()
	setting.CostDetailLookbackDays = 1
	setting.CostDetailAutoApplyEnabled = false

	result, err := syncQiniuCostDetailAt(context.Background(), now)
	requireNoError(t, err)
	if len(result.Errors) != 0 {
		t.Fatalf("expected no sync errors, got %#v", result)
	}
	var rawCount int64
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Count(&rawCount).Error)
	if rawCount != 1 {
		t.Fatalf("expected parent masked bill and child full-key bill to share one raw record, got %d", rawCount)
	}
	var bucket model.QiniuBillingBucket
	requireNoError(t, model.DB.First(&bucket, "token_id = ? AND billing_date = ?", token.Id, "2026-06-04").Error)
	if bucket.OfficialAmount != 0.01 {
		t.Fatalf("expected bucket amount to stay 0.01, got %#v", bucket)
	}
	var item model.QiniuBillingBucketItem
	requireNoError(t, model.DB.First(&item, "bucket_id = ?", bucket.Id).Error)
	if item.RawRecordIds == "" || strings.Contains(item.RawRecordIds, ",") {
		t.Fatalf("expected bucket item to reference exactly one raw record, got %#v", item)
	}
}

func TestSyncQiniuCostDetailOfficialRevisionUpsertsStableRawRecord(t *testing.T) {
	truncate(t)

	setting := operation_setting.GetQiniuKeySetting()
	oldSetting := *setting
	t.Cleanup(func() { *setting = oldSetting })

	keyBody := "rev" + strings.Repeat("1", 55) + "123456"
	token := seedQiniuBucketApplicationUserToken(t, 5611, 5611, int(1*common.QuotaPerUnit), 0, 0)
	requireNoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Update("key", keyBody).Error)

	now := time.Date(2026, 6, 5, 10, 30, 0, 0, qiniuCSTLocation)
	feeAmount := 0.25
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != qiniuAPIKeyUsagePath {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		apiKeys := []map[string]any{}
		if r.URL.Query().Get("start_date") == "2026-06-04" {
			apiKeys = append(apiKeys, map[string]any{
				"api_key": "sk-rev***123456",
				"bills": []map[string]any{
					{
						"date": "2026-06-04",
						"models": []map[string]any{
							{
								"model_id": "deepseek-v3",
								"items": []map[string]any{
									{
										"name":  "deepseek-v3输入",
										"key":   "input",
										"usage": map[string]any{"count": 1.0, "unit": "k/tokens"},
										"fee":   feeAmount,
									},
								},
							},
						},
					},
				},
			})
		}
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data":   map[string]any{"api_keys": apiKeys},
		})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	operation_setting.GetQiniuKeySetting().CostDetailAutoApplyEnabled = false

	result, err := syncQiniuCostDetailAt(context.Background(), now)
	requireNoError(t, err)
	if result.InsertedCount != 1 || result.RawRecordCount != 1 || len(result.Errors) != 0 {
		t.Fatalf("unexpected initial cost-detail sync result: %#v", result)
	}

	feeAmount = 0.30
	result, err = syncQiniuCostDetailAt(context.Background(), now)
	requireNoError(t, err)
	if result.InsertedCount != 0 || result.RawRecordCount != 1 || len(result.Errors) != 0 {
		t.Fatalf("expected official revision to upsert existing raw record, got %#v", result)
	}
	var recordCount int64
	requireNoError(t, model.DB.Model(&model.QiniuCostDetailRecord{}).Where("billing_date = ?", "2026-06-04").Count(&recordCount).Error)
	if recordCount != 1 {
		t.Fatalf("expected one stable raw record after official revision, got %d", recordCount)
	}
	var record model.QiniuCostDetailRecord
	requireNoError(t, model.DB.First(&record, "billing_date = ?", "2026-06-04").Error)
	if record.FeeAmount != 0.30 {
		t.Fatalf("expected revised raw fee amount, got %#v", record)
	}
	var bucket model.QiniuBillingBucket
	requireNoError(t, model.DB.First(&bucket, "token_id = ? AND billing_date = ?", token.Id, "2026-06-04").Error)
	if bucket.PreviousOfficialQuota != amountToQuota(0.25) || bucket.OfficialQuota != amountToQuota(0.30) || bucket.PendingDeltaQuota != amountToQuota(0.30) {
		t.Fatalf("expected bucket to preserve official revision without duplicate amount, got %#v", bucket)
	}
}

func TestBuildQiniuCostDetailSyncWindowsCapsLookbackDays(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 30, 0, 0, qiniuCSTLocation)
	windows := buildQiniuCostDetailSyncWindows(now, operation_setting.QiniuCostDetailMaxLookbackDays+1)
	if len(windows) != operation_setting.QiniuCostDetailMaxLookbackDays {
		t.Fatalf("expected lookback capped to %d days, got %d", operation_setting.QiniuCostDetailMaxLookbackDays, len(windows))
	}
}

func TestSyncQiniuCostDetailResolvesBucketsAndAutoApplies(t *testing.T) {
	truncate(t)

	setting := operation_setting.GetQiniuKeySetting()
	oldSetting := *setting
	t.Cleanup(func() { *setting = oldSetting })

	keyBody := "abc" + strings.Repeat("1", 58) + "123456"
	token := seedQiniuBucketApplicationUserToken(t, 5601, 5601, int(1*common.QuotaPerUnit), 0, 0)
	requireNoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Update("key", keyBody).Error)

	now := time.Date(2026, 6, 5, 10, 30, 0, 0, qiniuCSTLocation)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != qiniuAPIKeyUsagePath {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		startDate := r.URL.Query().Get("start_date")
		apiKeys := []map[string]any{}
		if startDate == "2026-06-04" {
			apiKeys = append(apiKeys, map[string]any{
				"api_key": "sk-abc***123456",
				"bills": []map[string]any{
					{
						"date": startDate,
						"models": []map[string]any{
							{
								"model_id": "deepseek-v3",
								"items": []map[string]any{
									{
										"name":  "deepseek-v3输入",
										"key":   "input",
										"usage": map[string]any{"count": 1.0, "unit": "k/tokens"},
										"fee":   0.01,
									},
								},
							},
						},
					},
				},
			})
		}
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data":   map[string]any{"api_keys": apiKeys},
		})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	operation_setting.GetQiniuKeySetting().CostDetailAutoApplyEnabled = true
	operation_setting.GetQiniuKeySetting().CostDetailCutoverTime = time.Date(2026, 6, 4, 0, 0, 0, 0, qiniuCSTLocation).Unix()

	result, err := syncQiniuCostDetailAt(context.Background(), now)
	requireNoError(t, err)
	if result.RawRecordCount != 1 || result.InsertedCount != 1 || len(result.Errors) != 0 {
		t.Fatalf("unexpected cost-detail sync result: %#v", result)
	}

	var record model.QiniuCostDetailRecord
	requireNoError(t, model.DB.First(&record, "billing_date = ?", "2026-06-04").Error)
	if record.OwnerStatus != model.QiniuBillingOwnerStatusResolved || record.TokenId != token.Id {
		t.Fatalf("expected sync to resolve raw record ownership, got %#v", record)
	}
	var bucket model.QiniuBillingBucket
	requireNoError(t, model.DB.First(&bucket, "token_id = ? AND billing_date = ?", token.Id, "2026-06-04").Error)
	if bucket.Status != model.QiniuBillingBucketStatusApplied || bucket.PendingDeltaQuota != 0 || bucket.AppliedDeltaQuota != amountToQuota(0.01) {
		t.Fatalf("expected sync to auto apply resolved bucket, got %#v", bucket)
	}
	var app model.QiniuBillingBucketApplication
	requireNoError(t, model.DB.First(&app, "bucket_id = ?", bucket.Id).Error)
	if app.Status != model.QiniuBillingApplicationStatusSuccess || app.WalletFlowId == 0 || app.ConsumeLogId == 0 {
		t.Fatalf("expected successful bucket application from sync, got %#v", app)
	}
}

func equalStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
