package helper

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}

func withQiniuMarketHelperSetting(t *testing.T, mutate func(*operation_setting.QiniuKeySetting)) {
	t.Helper()
	setting := operation_setting.GetQiniuKeySetting()
	original := *setting
	mutate(setting)
	t.Cleanup(func() {
		*setting = original
	})
}

func TestModelPriceHelperQiniuManagedUsesCurrentMarketSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/market/models", r.URL.Path)
		payload, err := common.Marshal(map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id": "qiniu-helper-market-only",
					"pricing_rules_v2": []map[string]any{
						{
							"input_range":  []int{0, 999999},
							"output_range": []int{0, 999999},
							"details_v2": map[string]any{
								"input": map[string]any{
									"unit_name":  "token",
									"unit_size":  1000,
									"unit_price": 0.004,
								},
								"output": map[string]any{
									"unit_name":  "token",
									"unit_size":  1000,
									"unit_price": 0.016,
								},
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	withQiniuMarketHelperSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
		setting.MarketCatalogBaseURL = server.URL
		setting.MarketCatalogTTLSeconds = 60
		setting.OfficialLedgerEnabled = true
		setting.AccessKey = "ak"
		setting.SecretKey = "sk"
	})
	snapshot := service.GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, service.QiniuMarketCatalogStatusFresh, snapshot.Status)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		QiniuManagedToken: true,
		OriginModelName:   "qiniu-helper-market-only",
		UserGroup:         "default",
		UsingGroup:        "default",
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{MaxTokens: 200})
	require.NoError(t, err)
	require.NotNil(t, priceData.QiniuMarket)
	require.Equal(t, service.QiniuMarketRealtimeBillingSource, priceData.QiniuMarket.BillingSource)
	require.Equal(t, 3600, priceData.QuotaToPreConsume)
	require.Equal(t, priceData, info.PriceData)
}

func TestModelPriceHelperQiniuManagedUsesMappedCatalogModelID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/market/models", r.URL.Path)
		payload, err := common.Marshal(map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id": "qiniu-catalog-model-id",
					"pricing_rules_v2": []map[string]any{
						{
							"input_range":  []int{0, 999999},
							"output_range": []int{0, 999999},
							"details_v2": map[string]any{
								"input":  map[string]any{"unit_name": "token", "unit_size": 1000, "unit_price": 0.004},
								"output": map[string]any{"unit_name": "token", "unit_size": 1000, "unit_price": 0.016},
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	withQiniuMarketHelperSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
		setting.MarketCatalogBaseURL = server.URL
		setting.MarketCatalogTTLSeconds = 60
	})
	snapshot := service.GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, service.QiniuMarketCatalogStatusFresh, snapshot.Status)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		QiniuManagedToken: true,
		OriginModelName:   "local-alias-model",
		UserGroup:         "default",
		UsingGroup:        "default",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "qiniu-catalog-model-id",
			IsModelMapped:     true,
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{MaxTokens: 200})
	require.NoError(t, err)
	require.NotNil(t, priceData.QiniuMarket)
	require.Equal(t, "qiniu-catalog-model-id", priceData.QiniuMarket.MarketModelID)
}

func TestModelPriceHelperQiniuManagedUsesCatalogDefaultOutputTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/market/models", r.URL.Path)
		payload, err := common.Marshal(map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id": "qiniu-helper-default-output",
					"model_constraints": map[string]any{
						"max_default_completion_tokens": 200,
					},
					"pricing_rules_v2": []map[string]any{
						{
							"input_range":  []int{0, 999999},
							"output_range": []int{0, 100},
							"details_v2": map[string]any{
								"input":  map[string]any{"unit_name": "token", "unit_size": 1000, "unit_price": 0.004},
								"output": map[string]any{"unit_name": "token", "unit_size": 1000, "unit_price": 0.004},
							},
						},
						{
							"input_range":  []int{0, 999999},
							"output_range": []int{101, 500},
							"details_v2": map[string]any{
								"input":  map[string]any{"unit_name": "token", "unit_size": 1000, "unit_price": 0.004},
								"output": map[string]any{"unit_name": "token", "unit_size": 1000, "unit_price": 0.016},
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	withQiniuMarketHelperSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
		setting.MarketCatalogBaseURL = server.URL
		setting.MarketCatalogTTLSeconds = 60
	})
	snapshot := service.GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, service.QiniuMarketCatalogStatusFresh, snapshot.Status)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		QiniuManagedToken: true,
		OriginModelName:   "qiniu-helper-default-output",
		UserGroup:         "default",
		UsingGroup:        "default",
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.NotNil(t, priceData.QiniuMarket)
	require.Equal(t, 1, priceData.QiniuMarket.RuleIndex)
	require.Equal(t, 200, priceData.QiniuMarket.EstimatedOutputTokens)
	require.Equal(t, 3600, priceData.QuotaToPreConsume)
}

func TestModelPriceHelperPerCallQiniuManagedUsesMarketUnitPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/market/models", r.URL.Path)
		payload, err := common.Marshal(map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id": "qiniu-task-market-only",
					"pricing_rules_v2": []map[string]any{
						{
							"details_v2": map[string]any{
								"request": map[string]any{
									"unit_name":  "request",
									"unit_size":  1,
									"unit_price": 0.01,
									"name":       "单次请求",
								},
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	withQiniuMarketHelperSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
		setting.MarketCatalogBaseURL = server.URL
		setting.MarketCatalogTTLSeconds = 60
		setting.OfficialLedgerEnabled = true
		setting.AccessKey = "ak"
		setting.SecretKey = "sk"
	})
	snapshot := service.GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, service.QiniuMarketCatalogStatusFresh, snapshot.Status)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		QiniuManagedToken: true,
		OriginModelName:   "qiniu-task-market-only",
		UserGroup:         "default",
		UsingGroup:        "default",
	}

	priceData, err := ModelPriceHelperPerCall(ctx, info)
	require.NoError(t, err)
	require.NotNil(t, priceData.QiniuMarket)
	require.Equal(t, service.QiniuMarketRealtimeBillingSource, priceData.QiniuMarket.BillingSource)
	require.Equal(t, "request", priceData.QiniuMarket.UnitName)
	require.Equal(t, 5000, priceData.Quota)
	require.False(t, service.ShouldUseQiniuOfficialLedger(info))
}

func TestQiniuMarketPriceMissingLogIncludesContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withQiniuMarketHelperSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
		setting.MarketCatalogBaseURL = "http://127.0.0.1:1/qiniu-helper-log-context"
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-qiniu-price-missing")

	info := &relaycommon.RelayInfo{
		UserId:            1001,
		TokenId:           2002,
		TokenKey:          "sk-secret-token-value",
		QiniuManagedToken: true,
		OriginModelName:   "qiniu-helper-log-context",
		RequestId:         "req-qiniu-price-missing",
	}

	message := qiniuMarketPriceMissingLog(ctx, info, errors.New("price_missing: 市场价不可用：模型缺失"))
	require.Contains(t, message, "user_id=1001")
	require.Contains(t, message, "token_id=2002")
	require.Contains(t, message, "request_id=req-qiniu-price-missing")
	require.Contains(t, message, "token_fingerprint=")
	require.Contains(t, message, "catalog_status=")
	require.Contains(t, message, "model=qiniu-helper-log-context")
	require.Contains(t, message, "origin_model=qiniu-helper-log-context")
	require.Contains(t, message, "billing_model_id=qiniu-helper-log-context")
	require.NotContains(t, message, "sk-secret-token-value")
}

func TestQiniuMarketPriceMissingLogDoesNotFetchCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		payload, err := common.Marshal(map[string]any{
			"status": true,
			"data":   []map[string]any{{"id": "qiniu-log-fetch-should-not-happen"}},
		})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	withQiniuMarketHelperSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
		setting.MarketCatalogBaseURL = server.URL
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		QiniuManagedToken: true,
		OriginModelName:   "qiniu-log-fetch-should-not-happen",
	}

	message := qiniuMarketPriceMissingLog(ctx, info, errors.New("price_missing"))
	require.Contains(t, message, "catalog_status=stale")
	require.Equal(t, 0, calls)
}

func TestModelPriceHelperQiniuManagedRejectsMissingCurrentMarketSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	withQiniuMarketHelperSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
		setting.MarketCatalogBaseURL = "http://127.0.0.1:1/qiniu-helper-missing"
		setting.OfficialLedgerEnabled = false
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		QiniuManagedToken: true,
		OriginModelName:   "qiniu-helper-missing",
		UserGroup:         "default",
		UsingGroup:        "default",
		UserSetting:       dto.UserSetting{AcceptUnsetRatioModel: true},
	}

	_, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{MaxTokens: 200})
	require.Error(t, err)
	require.Contains(t, err.Error(), "price_missing")
	require.Nil(t, info.PriceData.QiniuMarket)
}
