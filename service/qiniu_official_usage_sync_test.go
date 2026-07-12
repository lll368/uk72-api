package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func configureQiniuOfficialLedgerSettingForTest(t *testing.T, baseURL string) {
	t.Helper()
	setting := operation_setting.GetQiniuKeySetting()
	old := *setting
	setting.Enabled = false
	setting.OfficialLedgerEnabled = true
	setting.BaseURL = baseURL
	setting.AccessKey = "ak"
	setting.SecretKey = "sk"
	setting.RequestTimeout = 5
	setting.OfficialLedgerWindowHours = 1
	setting.OfficialLedgerWindowDays = 1
	setting.OfficialLedgerBatchSize = 10
	setting.OfficialLedgerRateLimitPerSecond = 10000
	setting.OfficialLedgerRetryIntervalSeconds = 1
	t.Cleanup(func() {
		*setting = old
	})
}

func seedQiniuManagedToken(t *testing.T, userId int, tokenId int, keyBody string) {
	t.Helper()
	requireNoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: fmt.Sprintf("qiniu_user_%d", userId),
		AffCode:  fmt.Sprintf("qiniu_aff_%d", userId),
		Quota:    int(100 * common.QuotaPerUnit),
		Status:   common.UserStatusEnabled,
	}).Error)
	requireNoError(t, model.DB.Create(&model.Token{
		Id:             tokenId,
		UserId:         userId,
		Name:           "qiniu-token",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
}

func TestQiniuOfficialBillCorrectionRequeuesAppliedRecord(t *testing.T) {
	truncate(t)

	const userID = 4110
	const tokenID = 4110
	seedQiniuLedgerUserToken(t, userID, tokenID, amountToQuota(1), 0, 0)

	var token model.Token
	requireNoError(t, model.DB.First(&token, "id = ?", tokenID).Error)
	start := time.Date(2026, 1, 2, 0, 0, 0, 0, qiniuCSTLocation)
	window := qiniuOfficialSyncWindow{Start: start, End: start.AddDate(0, 0, 1), Granularity: "day", CostGrain: "day", QueryCost: true}
	fullKey := "sk-correction-test"
	makeItem := func(fee float64, raw string) qiniuOfficialCostDetailItem {
		return qiniuOfficialCostDetailItem{
			APIKey:      fullKey,
			ModelName:   "deepseek-v3",
			BillingItem: "input",
			FeeAmount:   fee,
			Currency:    "CNY",
			PeriodStart: start.Unix(),
			PeriodEnd:   start.AddDate(0, 0, 1).Unix(),
			RawResponse: raw,
		}
	}

	_, err := persistQiniuOfficialBillItems(&token, []qiniuOfficialCostDetailItem{makeItem(0.10, `{"fee":0.10}`)}, window, 0)
	requireNoError(t, err)
	ledgerResult, err := ApplyPendingQiniuOfficialLedgerRecords(context.Background(), 10)
	requireNoError(t, err)
	if ledgerResult.AppliedCount != 0 {
		t.Fatalf("expected official ledger auto apply disabled, got %#v", ledgerResult)
	}

	var record model.QiniuOfficialUsageRecord
	requireNoError(t, model.DB.Where("record_type = ?", model.QiniuOfficialRecordTypeBill).First(&record).Error)
	firstQuota := amountToQuota(0.10)
	if record.Status != model.QiniuOfficialRecordStatusPending || record.OfficialQuota != firstQuota || record.AppliedQuota != 0 || record.ApplyVersion != 0 {
		t.Fatalf("expected first bill to remain observation-only, got %#v", record)
	}

	_, err = persistQiniuOfficialBillItems(&token, []qiniuOfficialCostDetailItem{makeItem(0.08, `{"fee":0.08,"revision":1}`)}, window, 0)
	requireNoError(t, err)
	requireNoError(t, model.DB.First(&record, "id = ?", record.Id).Error)
	refundQuota := amountToQuota(0.08)
	if record.Status != model.QiniuOfficialRecordStatusPending || record.OfficialQuota != refundQuota || record.AppliedQuota != 0 || record.ApplyVersion != 0 {
		t.Fatalf("expected corrected lower fee to stay observation-only, got %#v", record)
	}
	ledgerResult, err = ApplyPendingQiniuOfficialLedgerRecords(context.Background(), 10)
	requireNoError(t, err)
	if ledgerResult.AppliedCount != 0 {
		t.Fatalf("expected refund delta apply disabled, got %#v", ledgerResult)
	}

	_, err = persistQiniuOfficialBillItems(&token, []qiniuOfficialCostDetailItem{makeItem(0.08, `{"fee":0.08,"revision":1}`)}, window, 0)
	requireNoError(t, err)
	ledgerResult, err = ApplyPendingQiniuOfficialLedgerRecords(context.Background(), 10)
	requireNoError(t, err)
	if ledgerResult.AppliedCount != 0 {
		t.Fatalf("expected unchanged official fee to avoid duplicate apply, got %#v", ledgerResult)
	}

	_, err = persistQiniuOfficialBillItems(&token, []qiniuOfficialCostDetailItem{makeItem(0.12, `{"fee":0.12,"revision":2}`)}, window, 0)
	requireNoError(t, err)
	requireNoError(t, model.DB.First(&record, "id = ?", record.Id).Error)
	rechargeQuota := amountToQuota(0.12)
	if record.Status != model.QiniuOfficialRecordStatusPending || record.OfficialQuota != rechargeQuota || record.AppliedQuota != 0 || record.ApplyVersion != 0 {
		t.Fatalf("expected corrected higher fee to stay observation-only, got %#v", record)
	}
	ledgerResult, err = ApplyPendingQiniuOfficialLedgerRecords(context.Background(), 10)
	requireNoError(t, err)
	if ledgerResult.AppliedCount != 0 {
		t.Fatalf("expected consume delta apply disabled, got %#v", ledgerResult)
	}
	assertQiniuQuotaSyncTaskCount(t, userID, tokenID, 0)
}

func TestSyncQiniuOfficialUsageUpsertsUsageAndBillRecords(t *testing.T) {
	truncate(t)

	now := time.Date(2026, 1, 2, 10, 30, 0, 0, qiniuCSTLocation)
	keyBody := strings.Repeat("5", 64)
	fullKey := "sk-" + keyBody
	seedQiniuManagedToken(t, 4101, 4101, keyBody)
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch {
		case r.Method == http.MethodGet && r.URL.Path == qiniuOfficialUsagePath:
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:") {
				t.Fatalf("expected official usage AK/SK authorization, got %q", r.Header.Get("Authorization"))
			}
			if r.URL.Query().Get("api_key") != fullKey {
				t.Fatalf("expected api_key query %q, got %q", fullKey, r.URL.Query().Get("api_key"))
			}
			pointTime := r.URL.Query().Get("start")
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": []map[string]any{
					{
						"id": "deepseek-v3",
						"items": []map[string]any{
							{
								"name": "输入 Token",
								"unit": "kToken",
								"categories": []map[string]any{
									{
										"name": "输入 Token",
										"values": []map[string]any{
											{"time": pointTime, "value": 1.5},
										},
									},
								},
							},
						},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == qiniuAPIKeyUsagePath:
			if r.Header.Get("Authorization") != "Bearer "+fullKey {
				t.Fatalf("expected bill Bearer authorization, got %q", r.Header.Get("Authorization"))
			}
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"api_key": fullKey,
					"bills": []map[string]any{
						{
							"date": r.URL.Query().Get("start_date"),
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
			})
		default:
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)

	result, err := syncQiniuOfficialUsageAt(context.Background(), now)
	requireNoError(t, err)
	if result.UsageRecordCount != 2 || result.BillRecordCount != 1 || result.InsertedCount != 3 || result.LedgerAppliedCount != 0 {
		t.Fatalf("unexpected first sync result: %#v", result)
	}
	var count int64
	requireNoError(t, model.DB.Model(&model.QiniuOfficialUsageRecord{}).Count(&count).Error)
	if count != 3 {
		t.Fatalf("expected 3 official records, got %d", count)
	}
	var bill model.QiniuOfficialUsageRecord
	requireNoError(t, model.DB.Where("record_type = ?", model.QiniuOfficialRecordTypeBill).First(&bill).Error)
	if bill.Status != model.QiniuOfficialRecordStatusPending || bill.UserId != 4101 || bill.TokenId != 4101 || bill.OfficialQuota != amountToQuota(0.25) || bill.AppliedQuota != 0 {
		t.Fatalf("unexpected bill record: %#v", bill)
	}
	requireNoError(t, model.DB.Model(&model.WalletFlow{}).Count(&count).Error)
	if count != 0 {
		t.Fatalf("expected official sync not to write wallet flow, got %d", count)
	}

	result, err = syncQiniuOfficialUsageAt(context.Background(), now)
	requireNoError(t, err)
	if result.InsertedCount != 0 {
		t.Fatalf("expected repeated sync to upsert existing records, got inserted=%d", result.InsertedCount)
	}
	requireNoError(t, model.DB.Model(&model.QiniuOfficialUsageRecord{}).Count(&count).Error)
	if count != 3 {
		t.Fatalf("expected official record count to remain 3, got %d", count)
	}
	if callCount != 6 {
		t.Fatalf("expected 6 qiniu calls after two syncs, got %d", callCount)
	}
}

func TestSyncQiniuOfficialUsagePaginatesQiniuManagedTokens(t *testing.T) {
	truncate(t)

	now := time.Date(2026, 1, 2, 10, 30, 0, 0, qiniuCSTLocation)
	keyBodies := []string{
		strings.Repeat("1", 64),
		strings.Repeat("2", 64),
		strings.Repeat("3", 64),
	}
	fullKeys := make(map[string]bool)
	for idx, keyBody := range keyBodies {
		seedQiniuManagedToken(t, 4121+idx, 4121+idx, keyBody)
		fullKeys["sk-"+keyBody] = true
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == qiniuOfficialUsagePath:
			if !fullKeys[r.URL.Query().Get("api_key")] {
				t.Fatalf("unexpected usage api_key %q", r.URL.Query().Get("api_key"))
			}
			writeQiniuJSON(t, w, map[string]any{"status": true, "data": []map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == qiniuAPIKeyUsagePath:
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || !fullKeys[strings.TrimPrefix(auth, "Bearer ")] {
				t.Fatalf("unexpected bill authorization %q", auth)
			}
			fullKey := strings.TrimPrefix(auth, "Bearer ")
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"api_key": fullKey,
					"bills": []map[string]any{
						{
							"date": r.URL.Query().Get("start_date"),
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
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	setting := operation_setting.GetQiniuKeySetting()
	setting.OfficialLedgerBatchSize = 2

	result, err := syncQiniuOfficialUsageAt(context.Background(), now)
	requireNoError(t, err)
	if result.TokenCount != 3 || result.BillRecordCount != 3 {
		t.Fatalf("expected one sync to scan all 3 qiniu tokens, got %#v", result)
	}
	var billCount int64
	requireNoError(t, model.DB.Model(&model.QiniuOfficialUsageRecord{}).Where("record_type = ?", model.QiniuOfficialRecordTypeBill).Count(&billCount).Error)
	if billCount != 3 {
		t.Fatalf("expected 3 persisted official bill records, got %d", billCount)
	}
}

func TestSyncQiniuOfficialUsageScansInactiveManagedTokens(t *testing.T) {
	truncate(t)

	now := time.Date(2026, 1, 2, 10, 30, 0, 0, qiniuCSTLocation)
	keyBodies := []string{
		strings.Repeat("4", 64),
		strings.Repeat("7", 64),
		strings.Repeat("8", 64),
	}
	for idx, keyBody := range keyBodies {
		seedQiniuManagedToken(t, 4131+idx, 4131+idx, keyBody)
	}
	requireNoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 4132).Update("status", common.TokenStatusDisabled).Error)
	requireNoError(t, model.DB.Delete(&model.Token{}, 4133).Error)

	fullKeys := make(map[string]bool)
	for _, keyBody := range keyBodies {
		fullKeys["sk-"+keyBody] = true
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == qiniuOfficialUsagePath:
			if !fullKeys[r.URL.Query().Get("api_key")] {
				t.Fatalf("unexpected usage api_key %q", r.URL.Query().Get("api_key"))
			}
			writeQiniuJSON(t, w, map[string]any{"status": true, "data": []map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == qiniuAPIKeyUsagePath:
			fullKey := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !fullKeys[fullKey] {
				t.Fatalf("unexpected bill authorization %q", r.Header.Get("Authorization"))
			}
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"api_key": fullKey,
					"bills": []map[string]any{
						{
							"date": r.URL.Query().Get("start_date"),
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
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	setting := operation_setting.GetQiniuKeySetting()
	setting.OfficialLedgerBatchSize = 10

	result, err := syncQiniuOfficialUsageAt(context.Background(), now)
	requireNoError(t, err)
	if result.TokenCount != 3 || result.BillRecordCount != 3 || result.LedgerAppliedCount != 0 || result.LedgerFailedCount != 0 {
		t.Fatalf("expected sync to include enabled, disabled and recently deleted qiniu tokens, got %#v", result)
	}
}

func TestSyncQiniuOfficialUsageUsesTokenChildAccountIdentityForHistory(t *testing.T) {
	truncate(t)

	now := time.Date(2026, 1, 2, 10, 30, 0, 0, qiniuCSTLocation)
	keyBody := strings.Repeat("9", 64)
	fullKey := "sk-" + keyBody
	seedQiniuManagedToken(t, 4134, 4134, keyBody)
	account := seedQiniuIdentityClientAccount(t, 813, model.QiniuChildAccountStatusDisabled, "child-usage-ak", "child-usage-sk")
	requireNoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 4134).Update("qiniu_child_account_id", account.Id).Error)

	var usageAuth string
	var billAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == qiniuOfficialUsagePath:
			usageAuth = r.Header.Get("Authorization")
			if r.URL.Query().Get("api_key") != fullKey {
				t.Fatalf("expected api_key query %q, got %q", fullKey, r.URL.Query().Get("api_key"))
			}
			pointTime := r.URL.Query().Get("start")
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": []map[string]any{
					{
						"id": "deepseek-v3",
						"items": []map[string]any{
							{
								"name": "输入 Token",
								"unit": "kToken",
								"categories": []map[string]any{
									{
										"name": "输入 Token",
										"values": []map[string]any{
											{"time": pointTime, "value": 1.5},
										},
									},
								},
							},
						},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == qiniuAPIKeyUsagePath:
			billAuth = r.Header.Get("Authorization")
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"api_key": fullKey,
					"bills": []map[string]any{
						{
							"date": r.URL.Query().Get("start_date"),
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
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)

	result, err := syncQiniuOfficialUsageAt(context.Background(), now)
	requireNoError(t, err)
	if len(result.Errors) != 0 || result.TokenCount != 1 {
		t.Fatalf("expected child account official usage sync without errors, got %#v", result)
	}
	if !strings.HasPrefix(usageAuth, "Qiniu child-usage-ak:") {
		t.Fatalf("expected child account usage authorization, got %q", usageAuth)
	}
	if billAuth != "Bearer "+fullKey {
		t.Fatalf("expected bill Bearer authorization, got %q", billAuth)
	}
	var records []model.QiniuOfficialUsageRecord
	requireNoError(t, model.DB.Order("id asc").Find(&records).Error)
	if len(records) != 3 {
		t.Fatalf("expected three official child-account records, got %d", len(records))
	}
	for _, record := range records {
		if record.UserId != 4134 || record.TokenId != 4134 || record.QiniuChildAccountId != account.Id {
			t.Fatalf("expected official record to keep token child account ownership, got %#v", record)
		}
	}
}

func TestSyncQiniuOfficialUsageRepairsMissingLedgerLogs(t *testing.T) {
	truncate(t)

	now := time.Date(2026, 1, 2, 10, 30, 0, 0, qiniuCSTLocation)
	seedQiniuLedgerUserToken(t, 4135, 4135, 10000, 0, 0)
	record := seedQiniuOfficialBillRecord(t, 4135, 4135, 2000, 0, 0)
	requireNoError(t, model.DB.Create(&model.QiniuOfficialLedgerApplication{
		UsageRecordId:  record.Id,
		ApplyVersion:   1,
		UserId:         4135,
		TokenId:        4135,
		DeltaQuota:     2000,
		DeltaAmount:    2,
		IdempotencyKey: "legacy-sync-repair-disabled",
		Status:         model.QiniuOfficialLedgerStatusSuccess,
	}).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == qiniuOfficialUsagePath:
			writeQiniuJSON(t, w, map[string]any{"status": true, "data": []map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == qiniuAPIKeyUsagePath:
			writeQiniuJSON(t, w, map[string]any{"status": true, "data": map[string]any{"api_key": r.URL.Query().Get("api_key"), "bills": []map[string]any{}}})
		default:
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)

	result, err := syncQiniuOfficialUsageAt(context.Background(), now)
	requireNoError(t, err)
	if result.LedgerLogRepairedCount != 0 || result.LedgerAppliedCount != 0 || result.LedgerFailedCount != 0 {
		t.Fatalf("expected sync to keep official ledger observation-only, got %#v", result)
	}
	exists, err := qiniuOfficialLedgerLogExists(record.Id)
	requireNoError(t, err)
	if exists {
		t.Fatalf("expected sync not to repair synthetic official ledger log")
	}
}

func TestSyncQiniuOfficialUsageMarksBeforeCutoverSkipped(t *testing.T) {
	truncate(t)

	now := time.Date(2026, 1, 2, 10, 30, 0, 0, qiniuCSTLocation)
	keyBody := strings.Repeat("6", 64)
	fullKey := "sk-" + keyBody
	seedQiniuManagedToken(t, 4102, 4102, keyBody)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == qiniuOfficialUsagePath:
			writeQiniuJSON(t, w, map[string]any{"status": true, "data": []map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == qiniuAPIKeyUsagePath:
			if r.Header.Get("Authorization") != "Bearer "+fullKey {
				t.Fatalf("expected bill Bearer authorization, got %q", r.Header.Get("Authorization"))
			}
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"api_key": fullKey,
					"bills": []map[string]any{
						{
							"date": "2026-01-02",
							"models": []map[string]any{
								{
									"model_id": "deepseek-v3",
									"items": []map[string]any{
										{"name": "deepseek-v3输入", "key": "input", "usage": map[string]any{"count": 1.0, "unit": "k/tokens"}, "fee": 0.1},
									},
								},
							},
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)
	setting := operation_setting.GetQiniuKeySetting()
	setting.OfficialLedgerCutoverTime = now.Add(48 * time.Hour).Unix()

	result, err := syncQiniuOfficialUsageAt(context.Background(), now)
	requireNoError(t, err)
	if result.CutoverSkippedCount == 0 {
		t.Fatalf("expected cutover skipped records, got result: %#v", result)
	}
	var bill model.QiniuOfficialUsageRecord
	requireNoError(t, model.DB.Where("record_type = ?", model.QiniuOfficialRecordTypeBill).First(&bill).Error)
	if bill.Status != model.QiniuOfficialRecordStatusSkipped || bill.LastError != "before_official_ledger_cutover" {
		t.Fatalf("expected bill before cutover to be skipped, got %#v", bill)
	}
}

func TestPersistQiniuOfficialBillItemsKeepsUnmappedRecordObservable(t *testing.T) {
	truncate(t)

	start := time.Date(2026, 1, 2, 0, 0, 0, 0, qiniuCSTLocation)
	window := qiniuOfficialSyncWindow{Start: start, End: start.AddDate(0, 0, 1), Granularity: "day", CostGrain: "day", QueryCost: true}
	summary, err := persistQiniuOfficialBillItems(nil, []qiniuOfficialCostDetailItem{
		{
			APIKey:      "sk-unknown***123456",
			ModelName:   "deepseek-v3",
			BillingItem: "input",
			FeeAmount:   0.2,
			Currency:    "CNY",
			PeriodStart: start.Unix(),
			PeriodEnd:   start.AddDate(0, 0, 1).Unix(),
			RawResponse: `{"fee":0.2}`,
		},
	}, window, 0)
	requireNoError(t, err)
	if summary.unmappedCount != 1 || summary.recordCount != 1 {
		t.Fatalf("expected one unmapped record, got %#v", summary)
	}
	var record model.QiniuOfficialUsageRecord
	requireNoError(t, model.DB.First(&record).Error)
	if record.Status != model.QiniuOfficialRecordStatusUnmapped || record.UserId != 0 || record.TokenId != 0 {
		t.Fatalf("expected unmapped observable record, got %#v", record)
	}
	var flowCount int64
	requireNoError(t, model.DB.Model(&model.WalletFlow{}).Count(&flowCount).Error)
	if flowCount != 0 {
		t.Fatalf("unmapped records must not apply wallet balance, got flows=%d", flowCount)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
