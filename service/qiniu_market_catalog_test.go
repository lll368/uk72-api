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
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func withQiniuMarketSetting(t *testing.T, mutate func(*operation_setting.QiniuKeySetting)) {
	t.Helper()
	setting := operation_setting.GetQiniuKeySetting()
	original := *setting
	mutate(setting)
	t.Cleanup(func() {
		*setting = original
		resetQiniuMarketCatalogCacheForTest()
	})
}

func TestQiniuKeyClientQueryMarketModelsParsesPricingRules(t *testing.T) {
	var marketCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/market/models", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("overseas"))
		require.Empty(t, r.Header.Get("Authorization"))
		marketCalled = true
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id":          "kimi-k2",
					"name":        "Kimi K2",
					"description": "Kimi K2 description",
					"avatar":      "https://static.qiniu.com/kimi.png",
					"hot_tags":    []string{"上新"},
					"features":    []string{"工具调用"},
					"model_constraints": map[string]any{
						"context_length":                 128000,
						"max_completion_tokens":          8192,
						"max_default_completion_tokens":  4096,
						"max_chain_of_thought_length":    4096,
						"max_tokens":                     128000,
						"unknown_constraint_should_skip": "ignored",
					},
					"architecture": map[string]any{
						"input_modalities":  []string{"text"},
						"output_modalities": []string{"text"},
						"function_calling":  map[string]any{"supported": true},
						"reasoning":         map[string]any{"supported": false},
					},
					"pricing_rules_v2": []map[string]any{
						{
							"input_range":  []int{0, 99999999},
							"output_range": []int{0, 99999999},
							"details_v2": map[string]any{
								"input": map[string]any{
									"unit_name":      "token",
									"unit_size":      1000,
									"unit_price":     0.004,
									"unit_price_usd": 0.00056,
									"name":           "输入",
								},
								"output": map[string]any{
									"unit_name":      "token",
									"unit_size":      1000,
									"unit_price":     0.016,
									"unit_price_usd": 0.00222,
									"name":           "输出",
								},
							},
						},
					},
					"support_api_protocols": []string{"openai", "anthropic"},
					"retirement_at":         "",
					"release_at":            "2025-08-05",
					"unexpected_field":      "kept out of bounded dto",
				},
			},
		})
	}))
	defer server.Close()

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		MarketCatalogEnabled:  true,
		MarketCatalogBaseURL:  server.URL,
		MarketCatalogOverseas: true,
		RequestTimeout:        5,
	})
	require.NoError(t, err)

	models, err := client.QueryMarketModels(context.Background())
	require.NoError(t, err)
	require.True(t, marketCalled)
	require.Len(t, models, 1)
	require.Equal(t, "kimi-k2", models[0].ID)
	require.Equal(t, int64(128000), models[0].ModelConstraints.ContextLength)
	require.Len(t, models[0].PricingRulesV2, 1)
	require.NotNil(t, models[0].PricingRulesV2[0].DetailsV2["input"].UnitPrice)
	require.Equal(t, 0.004, *models[0].PricingRulesV2[0].DetailsV2["input"].UnitPrice)
	require.Equal(t, "工具调用", models[0].Features[0])
}

func TestQiniuKeyClientQueryMarketModelsDistinguishesMissingAndZeroUnitPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id": "free-and-missing",
					"pricing_rules_v2": []map[string]any{
						{
							"details_v2": map[string]any{
								"missing": map[string]any{
									"unit_name": "token",
									"unit_size": 1000,
									"name":      "缺失价格",
								},
								"free": map[string]any{
									"unit_name":  "token",
									"unit_size":  1000,
									"unit_price": 0,
									"name":       "免费额度",
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
		MarketCatalogEnabled: true,
		MarketCatalogBaseURL: server.URL,
		RequestTimeout:       5,
	})
	require.NoError(t, err)

	models, err := client.QueryMarketModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)
	details := models[0].PricingRulesV2[0].DetailsV2
	require.Nil(t, details["missing"].UnitPrice)
	require.NotNil(t, details["free"].UnitPrice)
	require.Equal(t, float64(0), *details["free"].UnitPrice)
}

func TestQiniuKeyClientQueryMarketModelsRejectsMalformedData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data":   map[string]any{"not": "a-list"},
		})
	}))
	defer server.Close()

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		MarketCatalogEnabled:  true,
		MarketCatalogBaseURL:  server.URL,
		MarketCatalogOverseas: true,
		RequestTimeout:        5,
	})
	require.NoError(t, err)

	models, err := client.QueryMarketModels(context.Background())
	require.Error(t, err)
	require.Empty(t, models)
}

func TestFetchQiniuOverseasMarketModelsForcesOverseasWithoutCatalogSwitch(t *testing.T) {
	var observed struct {
		Method   string
		Path     string
		RawQuery string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed.Method = r.Method
		observed.Path = r.URL.Path
		observed.RawQuery = r.URL.RawQuery
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id":   "qwen3-coder",
					"name": "Qwen3 Coder",
				},
			},
		})
	}))
	defer server.Close()
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = false
		setting.MarketCatalogBaseURL = server.URL
		setting.MarketCatalogOverseas = false
		setting.RequestTimeout = 5
	})

	models, err := FetchQiniuOverseasMarketModels(context.Background())

	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "qwen3-coder", models[0].ID)
	require.Equal(t, http.MethodGet, observed.Method)
	require.Equal(t, "/v1/market/models", observed.Path)
	require.Equal(t, "overseas=true", observed.RawQuery)
}

func TestQiniuMarketCatalogUsesStaleSnapshotOnFetchFailure(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogTTLSeconds = 1
		setting.MarketCatalogOverseas = true
	})
	resetQiniuMarketCatalogCacheForTest()

	calls := 0
	setQiniuMarketCatalogFetcherForTest(func(context.Context) ([]dto.QiniuMarketModel, error) {
		calls++
		if calls == 1 {
			return []dto.QiniuMarketModel{{ID: "kimi-k2", Name: "Kimi K2"}}, nil
		}
		return nil, errors.New("remote market unavailable with secret=sk-test")
	})

	first := GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, QiniuMarketCatalogStatusFresh, first.Status)
	require.False(t, first.Stale)
	require.Len(t, first.Models, 1)

	qiniuMarketCatalogCache.expiresAt = time.Now().Add(-time.Second)
	second := GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, QiniuMarketCatalogStatusStale, second.Status)
	require.True(t, second.Stale)
	require.Equal(t, "kimi-k2", second.Models[0].ID)
	require.NotContains(t, second.LastError, "sk-test")
	require.Equal(t, 2, calls)
}

func TestRefreshQiniuMarketCatalogOnceWarmsCurrentSnapshot(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogTTLSeconds = 60
		setting.MarketCatalogOverseas = true
	})
	resetQiniuMarketCatalogCacheForTest()

	calls := 0
	setQiniuMarketCatalogFetcherForTest(func(context.Context) ([]dto.QiniuMarketModel, error) {
		calls++
		return []dto.QiniuMarketModel{{ID: "deepseek/deepseek-v4-flash", Name: "DeepSeek V4 Flash"}}, nil
	})

	snapshot := RefreshQiniuMarketCatalogOnce(context.Background())
	require.Equal(t, QiniuMarketCatalogStatusFresh, snapshot.Status)
	require.Equal(t, 1, calls)

	current := GetCurrentQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, QiniuMarketCatalogStatusFresh, current.Status)
	require.Len(t, current.Models, 1)
	require.Equal(t, "deepseek/deepseek-v4-flash", current.Models[0].ID)
}

func TestRefreshQiniuMarketCatalogOnceBypassesFreshCache(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogTTLSeconds = 3600
		setting.MarketCatalogOverseas = true
	})
	resetQiniuMarketCatalogCacheForTest()

	calls := 0
	setQiniuMarketCatalogFetcherForTest(func(context.Context) ([]dto.QiniuMarketModel, error) {
		calls++
		modelID := "qiniu-market-first"
		if calls == 2 {
			modelID = "qiniu-market-second"
		}
		return []dto.QiniuMarketModel{{ID: modelID, Name: modelID}}, nil
	})

	first := GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, QiniuMarketCatalogStatusFresh, first.Status)
	require.Equal(t, "qiniu-market-first", first.Models[0].ID)

	second := RefreshQiniuMarketCatalogOnce(context.Background())
	require.Equal(t, QiniuMarketCatalogStatusFresh, second.Status)
	require.Equal(t, 2, calls)
	require.Equal(t, "qiniu-market-second", second.Models[0].ID)

	current := GetCurrentQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, "qiniu-market-second", current.Models[0].ID)
}

func TestGetCurrentQiniuMarketCatalogSnapshotFetchesWhenMemoryEmpty(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogTTLSeconds = 3600
		setting.MarketCatalogOverseas = true
	})
	resetQiniuMarketCatalogCacheForTest()

	calls := 0
	setQiniuMarketCatalogFetcherForTest(func(context.Context) ([]dto.QiniuMarketModel, error) {
		calls++
		return []dto.QiniuMarketModel{{ID: "qiniu-current-fallback", Name: "Qiniu Current Fallback"}}, nil
	})

	snapshot := GetCurrentQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, QiniuMarketCatalogStatusFresh, snapshot.Status)
	require.Equal(t, 1, calls)
	require.Len(t, snapshot.Models, 1)
	require.Equal(t, "qiniu-current-fallback", snapshot.Models[0].ID)
	require.False(t, snapshot.FromCache)
}

func TestGetCachedQiniuMarketCatalogSnapshotDoesNotFetchWhenMemoryEmpty(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
	})
	resetQiniuMarketCatalogCacheForTest()

	calls := 0
	setQiniuMarketCatalogFetcherForTest(func(context.Context) ([]dto.QiniuMarketModel, error) {
		calls++
		return []dto.QiniuMarketModel{{ID: "must-not-fetch"}}, nil
	})

	snapshot := GetCachedQiniuMarketCatalogSnapshot()
	require.Equal(t, QiniuMarketCatalogStatusStale, snapshot.Status)
	require.True(t, snapshot.Stale)
	require.Empty(t, snapshot.Models)
	require.Equal(t, 0, calls)
}

func TestQiniuMarketCatalogDefaultRefreshAndTTL(t *testing.T) {
	require.Equal(t, 30*time.Minute, qiniuMarketCatalogRefreshTickInterval)
	require.Equal(t, 3600, operation_setting.QiniuMarketCatalogDefaultTTLSeconds)
}

func TestQiniuMarketCatalogFallsBackLocalWhenDisabledOrEmpty(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = false
	})
	resetQiniuMarketCatalogCacheForTest()

	disabled := GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, QiniuMarketCatalogStatusDisabled, disabled.Status)
	require.Empty(t, disabled.Models)

	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
	})
	setQiniuMarketCatalogFetcherForTest(func(context.Context) ([]dto.QiniuMarketModel, error) {
		return nil, errors.New("network failure")
	})

	fallback := GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, QiniuMarketCatalogStatusFallbackLocal, fallback.Status)
	require.True(t, fallback.FallbackLocal)
	require.Empty(t, fallback.Models)
}

func TestApplyQiniuMarketCatalogToPricingMergesPayloadAndSourceMetadata(t *testing.T) {
	pricing := []model.Pricing{
		{
			ModelName:              "kimi-k2",
			Description:            "local description",
			Icon:                   "local-icon",
			Tags:                   "local",
			QuotaType:              0,
			ModelRatio:             1,
			CompletionRatio:        1,
			EnableGroup:            []string{"default"},
			SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI},
		},
	}
	snapshot := QiniuMarketCatalogSnapshot{
		Status: QiniuMarketCatalogStatusFresh,
		Models: []dto.QiniuMarketModel{
			{
				ID:          "kimi-k2",
				Name:        "Kimi K2",
				Description: "official description",
				Avatar:      "https://static.qiniu.com/kimi.png",
				HotTags:     []string{"上新"},
				Features:    []string{"工具调用"},
				ModelConstraints: dto.QiniuMarketModelConstraints{
					ContextLength:       128000,
					MaxCompletionTokens: 8192,
				},
				Architecture: dto.QiniuMarketArchitecture{
					InputModalities:  []string{"text"},
					OutputModalities: []string{"text"},
					FunctionCalling: dto.QiniuMarketSupportedFeature{
						Supported: true,
					},
				},
				PricingRulesV2: []dto.QiniuMarketPricingRuleV2{
					{
						DetailsV2: map[string]dto.QiniuMarketPricingDetail{
							"input": {UnitName: "token", UnitSize: 1000, UnitPrice: qiniuFloat64Ptr(0.004), Name: "输入"},
						},
					},
				},
			},
		},
	}

	enriched := ApplyQiniuMarketCatalogToPricing(pricing, snapshot)
	require.Len(t, enriched, 1)
	require.Equal(t, "official description", enriched[0].Description)
	require.Equal(t, "https://static.qiniu.com/kimi.png", enriched[0].Icon)
	require.Contains(t, enriched[0].Tags, "上新")
	require.Contains(t, enriched[0].Tags, "工具调用")
	require.True(t, enriched[0].Enabled)
	require.True(t, enriched[0].Routable)
	require.Equal(t, "qiniu_market", enriched[0].PriceSource)
	require.Equal(t, QiniuMarketPriceSourceLabel, enriched[0].PriceSourceLabel)
	require.NotNil(t, enriched[0].QiniuMarket)
	require.Len(t, enriched[0].QiniuMarket.PricingRulesV2, 1)
	require.Equal(t, int64(128000), enriched[0].ContextLength)
	require.Equal(t, int64(8192), enriched[0].MaxOutputTokens)
	require.Equal(t, []string{"text"}, enriched[0].InputModalities)
	require.Contains(t, enriched[0].Capabilities, "function_calling")
}

func TestApplyQiniuMarketCatalogToPricingPreservesNonQiniuAndDoesNotAddMarketOnlyModel(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "local-only", QuotaType: 0, EnableGroup: []string{"default"}},
	}
	snapshot := QiniuMarketCatalogSnapshot{
		Status: QiniuMarketCatalogStatusFresh,
		Models: []dto.QiniuMarketModel{
			{ID: "market-only", Name: "Market Only"},
		},
	}

	enriched := ApplyQiniuMarketCatalogToPricing(pricing, snapshot)
	require.Len(t, enriched, 1)
	require.Equal(t, "local-only", enriched[0].ModelName)
	require.Empty(t, enriched[0].PriceSource)
	require.Nil(t, enriched[0].QiniuMarket)
}

func TestApplyQiniuMarketCatalogToPricingStrictModeOnlyReturnsQiniuModels(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
	})
	resetQiniuMarketCatalogCacheForTest()
	setQiniuMarketCatalogFetcherForTest(func(context.Context) ([]dto.QiniuMarketModel, error) {
		return []dto.QiniuMarketModel{{ID: "kimi-k2", Name: "Kimi K2"}}, nil
	})

	pricing := []model.Pricing{
		{ModelName: "kimi-k2", QuotaType: 0, EnableGroup: []string{"default"}},
		{ModelName: "local-only", QuotaType: 0, EnableGroup: []string{"default"}},
	}

	snapshot := GetQiniuMarketCatalogSnapshot(context.Background())
	enriched := ApplyQiniuMarketCatalogToPricing(pricing, snapshot)

	require.Equal(t, QiniuMarketCatalogStatusFresh, snapshot.Status)
	require.Len(t, enriched, 1)
	require.Equal(t, "kimi-k2", enriched[0].ModelName)
	require.Equal(t, QiniuMarketPriceSource, enriched[0].PriceSource)
	require.NotNil(t, enriched[0].QiniuMarket)
}

func TestApplyQiniuMarketCatalogToPricingStrictModeWithoutMarketDataReturnsEmpty(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
	})
	resetQiniuMarketCatalogCacheForTest()
	setQiniuMarketCatalogFetcherForTest(func(context.Context) ([]dto.QiniuMarketModel, error) {
		return nil, errors.New("network failure")
	})

	pricing := []model.Pricing{
		{ModelName: "local-only", QuotaType: 0, EnableGroup: []string{"default"}},
	}

	snapshot := GetQiniuMarketCatalogSnapshot(context.Background())
	enriched := ApplyQiniuMarketCatalogToPricing(pricing, snapshot)

	require.Equal(t, QiniuMarketCatalogStatusStale, snapshot.Status)
	require.True(t, snapshot.Stale)
	require.Empty(t, enriched)
}

func TestQiniuMarketCatalogUnavailableDoesNotDisableOfficialLedger(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.OfficialLedgerEnabled = true
		setting.AccessKey = "ak"
		setting.SecretKey = "sk"
	})
	setQiniuMarketCatalogFetcherForTest(func(context.Context) ([]dto.QiniuMarketModel, error) {
		return nil, errors.New("market unavailable")
	})

	snapshot := GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, QiniuMarketCatalogStatusFallbackLocal, snapshot.Status)
	require.True(t, ShouldUseQiniuOfficialLedger(&relaycommon.RelayInfo{QiniuManagedToken: true}))
}

func TestQiniuOfficialLedgerSkipsMarketEstimateAsFinalCharge(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.OfficialLedgerEnabled = true
		setting.AccessKey = "ak"
		setting.SecretKey = "sk"
	})
	relayInfo := &relaycommon.RelayInfo{QiniuManagedToken: true}

	require.True(t, ShouldUseQiniuOfficialLedger(relayInfo))
	require.Equal(t, 0, QiniuOfficialLedgerLogQuota(relayInfo, 12345))

	other := map[string]interface{}{}
	MarkQiniuOfficialLedgerObservation(other, 12345)
	require.Equal(t, QiniuOfficialLedgerSource(), other["billing_source"])
	require.Equal(t, 12345, other["local_estimated_quota"])
	require.Equal(t, true, other["qiniu_realtime_billing_skipped"])
}

func TestSanitizeQiniuMarketErrorMasksSecrets(t *testing.T) {
	err := sanitizeQiniuMarketError(errors.New("failed with sk-" + strings.Repeat("a", 64) + " and secret=plain"))
	require.NotContains(t, err, strings.Repeat("a", 64))
	require.Contains(t, err, "sk-")
}

func TestQiniuMarketCatalogPublicStateDoesNotExposeLastError(t *testing.T) {
	state := QiniuMarketCatalogPublicState(QiniuMarketCatalogSnapshot{
		Status:    QiniuMarketCatalogStatusFallbackLocal,
		LastError: "remote market unavailable with secret=plain",
	})

	payload, err := common.Marshal(state)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "last_error")
	require.NotContains(t, string(payload), "secret=plain")
}

func qiniuFloat64Ptr(value float64) *float64 {
	return &value
}
