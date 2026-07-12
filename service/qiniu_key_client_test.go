package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestNormalizeQiniuAPIKey(t *testing.T) {
	keyBody := strings.Repeat("a", 64)
	normalized, err := normalizeQiniuAPIKey("sk-" + keyBody)
	if err != nil {
		t.Fatalf("normalize qiniu api key failed: %v", err)
	}
	if normalized != keyBody {
		t.Fatalf("expected key body %q, got %q", keyBody, normalized)
	}
}

func TestQiniuKeyClientCreateQuotaAndUsage(t *testing.T) {
	keyBody := strings.Repeat("b", 64)
	fullKey := "sk-" + keyBody
	var createCalled bool
	var quotaCalled bool
	var usageCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:") && !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("missing qiniu authorization header: %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			createCalled = true
			var payload map[string]any
			if err := common.DecodeJson(r.Body, &payload); err != nil {
				t.Fatalf("decode create payload failed: %v", err)
			}
			names, ok := payload["names"].([]any)
			if !ok || len(names) != 1 || names[0] != "default-key" {
				t.Fatalf("unexpected create names payload: %#v", payload["names"])
			}
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey, "name": "default-key"}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			quotaCalled = true
			var payload map[string]any
			if err := common.DecodeJson(r.Body, &payload); err != nil {
				t.Fatalf("decode quota payload failed: %v", err)
			}
			totalQuota, ok := payload["total_quota"].(map[string]any)
			if !ok || totalQuota["enabled"] != true || totalQuota["limit"] != float64(12) {
				t.Fatalf("unexpected total quota payload: %#v", payload["total_quota"])
			}
			if _, ok := payload["daily_quota"]; ok {
				t.Fatalf("total quota update must not change daily_quota: %#v", payload)
			}
			if _, ok := payload["monthly_quota"]; ok {
				t.Fatalf("total quota update must not change monthly_quota: %#v", payload)
			}
			writeQiniuJSON(t, w, map[string]any{"status": true, "data": payload})
		case r.Method == http.MethodGet && r.URL.Path == "/v2/stat/usage/apikey/cost-detail":
			usageCalled = true
			if r.Header.Get("Authorization") != "Bearer "+fullKey {
				t.Fatalf("unexpected usage authorization: %q", r.Header.Get("Authorization"))
			}
			if r.URL.Query().Get("grain") != "month" || r.URL.Query().Get("start_date") == "" || r.URL.Query().Get("end_date") == "" {
				t.Fatalf("unexpected usage query: %s", r.URL.RawQuery)
			}
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"api_key":   "sk-bbb***bbbbbb",
					"total_fee": 7.5,
				},
			})
		default:
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:        true,
		BaseURL:        server.URL,
		AccessKey:      "ak",
		SecretKey:      "sk",
		RequestTimeout: 5,
	})
	if err != nil {
		t.Fatalf("new qiniu client failed: %v", err)
	}
	createdKeyBody, err := client.CreateAPIKey(context.Background(), "default-key")
	if err != nil {
		t.Fatalf("create api key failed: %v", err)
	}
	if createdKeyBody != keyBody {
		t.Fatalf("expected created key body %q, got %q", keyBody, createdKeyBody)
	}
	if err := client.SetAPIKeyTotalQuota(context.Background(), createdKeyBody, 12); err != nil {
		t.Fatalf("set api key quota failed: %v", err)
	}
	usedAmount, err := client.GetAPIKeyUsedAmount(context.Background(), createdKeyBody, time.Now().Unix())
	if err != nil {
		t.Fatalf("get api key used amount failed: %v", err)
	}
	if usedAmount != 7.5 {
		t.Fatalf("expected used amount 7.5, got %f", usedAmount)
	}
	if !createCalled || !quotaCalled || !usageCalled {
		t.Fatalf("expected all qiniu endpoints to be called, create=%v quota=%v usage=%v", createCalled, quotaCalled, usageCalled)
	}
}

func TestQiniuKeyClientSetAPIKeyTotalQuotaZeroPayload(t *testing.T) {
	keyBody := strings.Repeat("0", 64)
	fullKey := "sk-" + keyBody
	var observed qiniuQuotaLimitPatchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/apikey/quota/"+fullKey {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		if err := common.DecodeJson(r.Body, &observed); err != nil {
			t.Fatalf("decode quota payload failed: %v", err)
		}
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:        true,
		BaseURL:        server.URL,
		AccessKey:      "ak",
		SecretKey:      "sk",
		RequestTimeout: 5,
	})
	if err != nil {
		t.Fatalf("new qiniu client failed: %v", err)
	}
	if err := client.SetAPIKeyTotalQuota(context.Background(), keyBody, 0); err != nil {
		t.Fatalf("set total quota zero failed: %v", err)
	}
	if observed.TotalQuota == nil || !observed.TotalQuota.Enabled || observed.TotalQuota.Limit != 0 {
		t.Fatalf("expected total_quota enabled with zero limit, got %#v", observed.TotalQuota)
	}
	if observed.DailyQuota != nil || observed.MonthlyQuota != nil {
		t.Fatalf("total-zero request must not change daily/monthly quota, got %#v", observed)
	}
}

func TestQiniuKeyClientSetAPIKeyDailyQuotaZeroPayload(t *testing.T) {
	keyBody := strings.Repeat("1", 64)
	fullKey := "sk-" + keyBody
	var observed qiniuQuotaLimitPatchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/apikey/quota/"+fullKey {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		if err := common.DecodeJson(r.Body, &observed); err != nil {
			t.Fatalf("decode quota payload failed: %v", err)
		}
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:        true,
		BaseURL:        server.URL,
		AccessKey:      "ak",
		SecretKey:      "sk",
		RequestTimeout: 5,
	})
	if err != nil {
		t.Fatalf("new qiniu client failed: %v", err)
	}
	if err := client.SetAPIKeyDailyQuotaZero(context.Background(), keyBody); err != nil {
		t.Fatalf("set daily quota zero failed: %v", err)
	}
	if observed.DailyQuota == nil || !observed.DailyQuota.Enabled || observed.DailyQuota.Limit != 0 {
		t.Fatalf("expected daily_quota enabled with zero limit, got %#v", observed.DailyQuota)
	}
	if observed.TotalQuota != nil || observed.MonthlyQuota != nil {
		t.Fatalf("daily-zero request must not change total/monthly quota, got %#v", observed)
	}
}

func TestQiniuKeyClientSetAPIKeyEnabledPayload(t *testing.T) {
	keyBody := strings.Repeat("2", 64)
	fullKey := "sk-" + keyBody
	var observed map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/ai/inapi/v2/apikey/enabled" {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:") {
			t.Fatalf("missing qiniu authorization header: %q", r.Header.Get("Authorization"))
		}
		if err := common.DecodeJson(r.Body, &observed); err != nil {
			t.Fatalf("decode enabled payload failed: %v", err)
		}
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()
	restoreQiniuEnabledBaseURLForTest(t, server.URL)

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:        true,
		BaseURL:        server.URL,
		AccessKey:      "ak",
		SecretKey:      "sk",
		RequestTimeout: 5,
	})
	if err != nil {
		t.Fatalf("new qiniu client failed: %v", err)
	}
	if err := client.SetAPIKeyEnabled(context.Background(), keyBody, false); err != nil {
		t.Fatalf("set api key disabled failed: %v", err)
	}
	if observed["key"] != fullKey || observed["enabled"] != false {
		t.Fatalf("unexpected enabled payload: %#v", observed)
	}
}

func TestQiniuKeyClientSetAPIKeyEnabledAlreadyDisabledIsIdempotent(t *testing.T) {
	keyBody := strings.Repeat("3", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/ai/inapi/v2/apikey/enabled" {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		writeQiniuJSON(t, w, map[string]any{
			"status": false,
			"code":   "api_key_already_disabled",
			"error":  "api key already disabled",
		})
	}))
	defer server.Close()
	restoreQiniuEnabledBaseURLForTest(t, server.URL)

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:        true,
		BaseURL:        server.URL,
		AccessKey:      "ak",
		SecretKey:      "sk",
		RequestTimeout: 5,
	})
	if err != nil {
		t.Fatalf("new qiniu client failed: %v", err)
	}
	if err := client.SetAPIKeyEnabled(context.Background(), keyBody, false); err != nil {
		t.Fatalf("already-disabled response should be idempotent success: %v", err)
	}
}

func restoreQiniuEnabledBaseURLForTest(t *testing.T, baseURL string) {
	t.Helper()
	oldBaseURL := qiniuAPIKeyEnabledBaseURL
	qiniuAPIKeyEnabledBaseURL = baseURL
	t.Cleanup(func() {
		qiniuAPIKeyEnabledBaseURL = oldBaseURL
	})
}

func TestQiniuTotalQuotaBelowUsedErrorClassification(t *testing.T) {
	trueCases := []error{
		errors.New("接口返回失败: total quota cannot be less than used amount"),
		errors.New("接口返回失败: 总额度不能低于已用金额"),
		errors.New("Key 接口返回异常状态 400: total quota cannot be lower than used amount"),
		qiniuBusinessStatusError(map[string]any{
			"status": false,
			"code":   400,
			"error":  "total quota cannot be lower than used amount",
		}),
	}
	for _, err := range trueCases {
		if !isQiniuTotalQuotaBelowUsedError(err) {
			t.Fatalf("expected deterministic lower-than-used error to be classified: %v", err)
		}
	}

	falseCases := []error{
		context.DeadlineExceeded,
		errors.New("Key 接口返回异常状态 500: total quota cannot be lower than used amount"),
		errors.New("Key 接口返回异常状态 429: total quota cannot be lower than used amount"),
		errors.New("Key 接口返回异常状态 401: total quota cannot be lower than used amount"),
		qiniuBusinessStatusError(map[string]any{
			"status": false,
			"code":   500,
			"error":  "total quota cannot be lower than used amount",
		}),
		qiniuBusinessStatusError(map[string]any{
			"status": false,
			"code":   429,
			"error":  "total quota cannot be lower than used amount",
		}),
		qiniuBusinessStatusError(map[string]any{
			"status": false,
			"code":   401,
			"error":  "total quota cannot be lower than used amount",
		}),
		errors.New("接口返回失败: quota rejected"),
		nil,
	}
	for _, err := range falseCases {
		if isQiniuTotalQuotaBelowUsedError(err) {
			t.Fatalf("expected non-fallback error to stay unclassified: %v", err)
		}
	}
}

func TestQiniuAuthorizationKeepsUrlSafeBase64Padding(t *testing.T) {
	client := &qiniuKeyClient{
		setting: operation_setting.QiniuKeySetting{
			AccessKey: "ak",
			SecretKey: "sk",
		},
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.qnaigc.com/v1/apikeys?x=1", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	got := client.authorization(req, []byte(`{"count":1}`))
	want := "Qiniu ak:RsrHToan4NRFtIHCkG3v_31gZ0I="
	if got != want {
		t.Fatalf("expected authorization %q, got %q", want, got)
	}
}

func TestExtractQiniuUsedAmountRequiresDataTotalFee(t *testing.T) {
	_, ok := extractQiniuUsedAmount(map[string]any{
		"status": true,
		"data": map[string]any{
			"bills": []any{
				map[string]any{
					"total_fee": 99,
				},
			},
		},
	})
	if ok {
		t.Fatalf("expected missing data.total_fee to be rejected")
	}
}

func TestQiniuKeyClientQueryOfficialTokenUsage(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, qiniuCSTLocation)
	end := start.Add(time.Hour)
	keyBody := strings.Repeat("d", 64)
	fullKey := "sk-" + keyBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/stat/usage" {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:") {
			t.Fatalf("expected AK/SK authorization, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "" {
			t.Fatalf("GET usage request should not send content type, got %q", r.Header.Get("Content-Type"))
		}
		if r.URL.Query().Get("granularity") != "hour" || r.URL.Query().Get("api_key") != fullKey {
			t.Fatalf("unexpected usage query: %s", r.URL.RawQuery)
		}
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id":   "deepseek-v3",
					"name": "DeepSeek V3",
					"items": []map[string]any{
						{
							"name": "输入 Token",
							"unit": "kToken",
							"categories": []map[string]any{
								{
									"name": "输入 Token",
									"values": []map[string]any{
										{"time": start.Format(time.RFC3339), "value": 12.5},
									},
								},
							},
						},
						{
							"name": "输出 Token",
							"unit": "kToken",
							"categories": []map[string]any{
								{
									"name": "输出 Token",
									"values": []map[string]any{
										{"time": start.Format(time.RFC3339), "value": 3},
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

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:        true,
		BaseURL:        server.URL,
		AccessKey:      "ak",
		SecretKey:      "sk",
		RequestTimeout: 5,
	})
	if err != nil {
		t.Fatalf("new qiniu client failed: %v", err)
	}
	items, err := client.QueryOfficialTokenUsage(context.Background(), qiniuOfficialUsageQuery{
		Granularity: "hour",
		Start:       start,
		End:         end,
		APIKey:      keyBody,
	})
	if err != nil {
		t.Fatalf("query official token usage failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 usage items, got %d", len(items))
	}
	if items[0].ModelName != "deepseek-v3" || items[0].PromptTokens != 12500 || items[0].TotalTokens != 12500 {
		t.Fatalf("unexpected prompt token item: %#v", items[0])
	}
	if items[1].CompletionTokens != 3000 || items[1].PeriodEnd != end.Unix() || items[1].RawResponse == "" {
		t.Fatalf("unexpected completion token item: %#v", items[1])
	}
}

func TestParseQiniuOfficialTokenUsageAcceptsEmptyDataObject(t *testing.T) {
	items, err := parseQiniuOfficialTokenUsage(map[string]any{
		"status": true,
		"data":   map[string]any{},
	}, "hour")
	if err != nil {
		t.Fatalf("expected empty data object to be accepted, got: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no usage items, got %d", len(items))
	}
}

func TestQiniuKeyClientQueryOfficialCostDetails(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, qiniuCSTLocation)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, qiniuCSTLocation)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/stat/usage/apikey/cost-detail" {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:") {
			t.Fatalf("expected AK/SK authorization, got %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("grain") != "day" || r.URL.Query().Get("start_date") != "2026-01-01" || r.URL.Query().Get("end_date") != "2026-01-31" {
			t.Fatalf("unexpected cost detail query: %s", r.URL.RawQuery)
		}
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data": map[string]any{
				"currency": "CNY",
				"api_keys": []map[string]any{
					{
						"api_key": "sk-abc***def123",
						"bills": []map[string]any{
							{
								"date": "2026-01-01",
								"models": []map[string]any{
									{
										"model_id": "deepseek-v3",
										"items": []map[string]any{
											{
												"name":  "deepseek-v3输入",
												"key":   "input",
												"usage": map[string]any{"count": 100.0, "unit": "k/tokens"},
												"fee":   1.25,
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

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:        true,
		BaseURL:        server.URL,
		AccessKey:      "ak",
		SecretKey:      "sk",
		RequestTimeout: 5,
	})
	if err != nil {
		t.Fatalf("new qiniu client failed: %v", err)
	}
	items, err := client.QueryOfficialCostDetails(context.Background(), qiniuOfficialCostDetailQuery{
		StartDate: start,
		EndDate:   end,
		Grain:     "day",
	})
	if err != nil {
		t.Fatalf("query official cost details failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 cost detail item, got %d", len(items))
	}
	item := items[0]
	if item.APIKey != "sk-abc***def123" || item.ModelName != "deepseek-v3" || item.BillingItem != "input" || item.FeeAmount != 1.25 || item.Currency != "CNY" {
		t.Fatalf("unexpected cost detail item: %#v", item)
	}
	if item.PeriodStart != start.Unix() || item.PeriodEnd != start.AddDate(0, 0, 1).Unix() || item.RawResponse == "" {
		t.Fatalf("unexpected cost period or raw payload: %#v", item)
	}
}

func TestParseQiniuOfficialCostDetailsRequiresFee(t *testing.T) {
	_, err := parseQiniuOfficialCostDetails(map[string]any{
		"status": true,
		"data": map[string]any{
			"api_key": "sk-abc***def123",
			"bills": []any{
				map[string]any{
					"date": "2026-01-01",
					"models": []any{
						map[string]any{
							"model_id": "deepseek-v3",
							"items": []any{
								map[string]any{
									"name":  "deepseek-v3输入",
									"key":   "input",
									"usage": map[string]any{"count": 100.0, "unit": "k/tokens"},
								},
							},
						},
					},
				},
			},
		},
	}, "day")
	if err == nil {
		t.Fatalf("expected missing fee to be rejected")
	}
	if !strings.Contains(err.Error(), "fee") || strings.Contains(err.Error(), "sk-abc***def123") {
		t.Fatalf("expected sanitized observable fee error, got: %v", err)
	}
}

func TestQiniuKeyClientRejectsFalseStatusResponse(t *testing.T) {
	keyBody := strings.Repeat("c", 64)
	fullKey := "sk-" + keyBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/apikey/quota/"+fullKey {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		writeQiniuJSON(t, w, map[string]any{
			"status": false,
			"error":  "quota rejected",
		})
	}))
	defer server.Close()

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:        true,
		BaseURL:        server.URL,
		AccessKey:      "ak",
		SecretKey:      "sk",
		RequestTimeout: 5,
	})
	if err != nil {
		t.Fatalf("new qiniu client failed: %v", err)
	}
	err = client.SetAPIKeyTotalQuota(context.Background(), keyBody, 12)
	if err == nil {
		t.Fatalf("expected false status response to be rejected")
	}
	if !strings.Contains(err.Error(), "quota rejected") {
		t.Fatalf("expected qiniu business error to be preserved, got: %v", err)
	}
}

func writeQiniuJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	data, err := common.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal qiniu response failed: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write qiniu response failed: %v", err)
	}
}
