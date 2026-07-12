package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type getPricingTestResponse struct {
	Success           bool                            `json:"success"`
	Data              []getPricingTestModel           `json:"data"`
	Vendors           []getPricingTestVendor          `json:"vendors"`
	MarketPricingSync service.QiniuMarketCatalogState `json:"market_pricing_sync"`
}

type getPricingTestModel struct {
	ModelName        string                `json:"model_name"`
	VendorID         int                   `json:"vendor_id,omitempty"`
	Description      string                `json:"description,omitempty"`
	Icon             string                `json:"icon,omitempty"`
	Tags             string                `json:"tags,omitempty"`
	EnableGroup      []string              `json:"enable_groups"`
	PriceSourceLabel string                `json:"price_source_label,omitempty"`
	MarketPricing    *getPricingTestMarket `json:"market_pricing,omitempty"`
}

type getPricingTestVendor struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

type getPricingTestMarket struct {
	PricingRulesV2 []map[string]any `json:"pricing_rules_v2,omitempty"`
}

func TestFilterPricingByUsableGroupsTrimsUnavailableGroups(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "visible", EnableGroup: []string{"default", "vip"}},
		{ModelName: "hidden", EnableGroup: []string{"vip"}},
		{ModelName: "all-visible", EnableGroup: []string{"all"}},
	}

	filtered := filterPricingByUsableGroups(pricing, map[string]string{
		"default": "Default",
	})

	require.Len(t, filtered, 2)
	require.Equal(t, "visible", filtered[0].ModelName)
	require.Equal(t, []string{"default"}, filtered[0].EnableGroup)
	require.Equal(t, "all-visible", filtered[1].ModelName)
	require.Equal(t, []string{"all"}, filtered[1].EnableGroup)
}

func TestGetPricingSanitizesMarketPricingPayloadForUsers(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	setting := operation_setting.GetQiniuKeySetting()
	originalSetting := *setting
	t.Cleanup(func() {
		*setting = originalSetting
		model.InvalidatePricingCache()
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/market/models", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("overseas"))
		writeControllerJSON(t, w, map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id":          "kimi-k2",
					"name":        "Kimi K2",
					"description": "QiNiu Qiniu 七牛 official description",
					"avatar":      "https://static.qiniu.com/kimi.png",
					"hot_tags":    []string{"七牛上新", "Qiniu realtime", "QiNiu mixed"},
					"features":    []string{"工具调用", "qiniu_market", "QiNiu feature"},
					"model_constraints": map[string]any{
						"context_length":        128000,
						"max_completion_tokens": 8192,
					},
					"architecture": map[string]any{
						"input_modalities":  []string{"text"},
						"output_modalities": []string{"text"},
						"function_calling":  map[string]any{"supported": true},
					},
					"pricing_rules_v2": []map[string]any{
						{
							"details_v2": map[string]any{
								"input": map[string]any{
									"unit_name":  "token",
									"unit_size":  1000,
									"unit_price": 0.004,
									"name":       "输入",
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	*setting = originalSetting
	setting.MarketCatalogEnabled = true
	setting.MarketCatalogBaseURL = server.URL
	setting.MarketCatalogOverseas = true
	setting.MarketCatalogTTLSeconds = 60
	setting.MarketCatalogFallbackEnabled = true
	model.InvalidatePricingCache()

	require.NoError(t, db.Create(&model.User{
		Id:       11001,
		Username: "pricing-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     11001,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Name:   "openai",
		Group:  "default",
	}).Error)
	require.NoError(t, db.Create(&model.Vendor{
		Id:          11001,
		Name:        "Qiniu vendor",
		Description: "QiNiu 七牛 vendor",
		Icon:        "https://assets.qiniu.com/vendor.png",
	}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "kimi-k2",
		VendorID:  11001,
		Status:    1,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "kimi-k2",
		ChannelId: 11001,
		Enabled:   true,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", 11001)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload getPricingTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, service.QiniuMarketCatalogStatusFresh, payload.MarketPricingSync.Status)
	require.Len(t, payload.Data, 1)
	require.Equal(t, service.QiniuMarketPriceSourceLabel, payload.Data[0].PriceSourceLabel)
	require.NotNil(t, payload.Data[0].MarketPricing)
	require.Equal(t, "https://static.qiniu.com/kimi.png", payload.Data[0].Icon)
	require.NotContains(t, payload.Data[0].Description, "Qiniu")
	require.NotContains(t, payload.Data[0].Description, "七牛")
	require.NotContains(t, payload.Data[0].Tags, "qiniu")
	require.NotContains(t, payload.Data[0].Tags, "Qiniu")
	require.NotContains(t, payload.Data[0].Tags, "七牛")
	require.Len(t, payload.Data[0].MarketPricing.PricingRulesV2, 1)
	require.Equal(t, []string{"default"}, payload.Data[0].EnableGroup)
	require.Len(t, payload.Vendors, 1)
	require.Equal(t, "https://assets.qiniu.com/vendor.png", payload.Vendors[0].Icon)

	visibleText := payload.Data[0].Description + " " + payload.Data[0].Tags + " " + payload.Vendors[0].Name + " " + payload.Vendors[0].Description
	require.NotContains(t, visibleText, "Qiniu")
	require.NotContains(t, visibleText, "QiNiu")
	require.NotContains(t, visibleText, "七牛")
	require.NotContains(t, strings.ToLower(visibleText), "qiniu")
}

func TestGetPricingInfersDefaultVendorForExistingModelWithoutVendor(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	disableQiniuMarketCatalogForPricingTest(t)

	require.NoError(t, db.Create(&model.User{
		Id:       11004,
		Username: "pricing-vendor-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     11004,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Name:   "openai",
		Group:  "default",
	}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "deepseek/deepseek-v3.2-exp",
		Status:    1,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "deepseek/deepseek-v3.2-exp",
		ChannelId: 11004,
		Enabled:   true,
	}).Error)
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", 11004)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload getPricingTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Len(t, payload.Data, 1)
	require.Equal(t, "deepseek/deepseek-v3.2-exp", payload.Data[0].ModelName)

	deepSeekVendor := findPricingTestVendorByName(payload.Vendors, "DeepSeek")
	require.NotNil(t, deepSeekVendor)
	require.Equal(t, "DeepSeek.Color", deepSeekVendor.Icon)
	require.Equal(t, deepSeekVendor.ID, payload.Data[0].VendorID)

	var persisted model.Model
	require.NoError(t, db.Where("model_name = ?", "deepseek/deepseek-v3.2-exp").First(&persisted).Error)
	require.Zero(t, persisted.VendorID)
}

func TestGetPricingKeepsConfiguredVendorWhenModelNameMatchesDefaultRule(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	disableQiniuMarketCatalogForPricingTest(t)

	customVendor := model.Vendor{
		Name:   "Custom DeepSeek Proxy",
		Icon:   "Custom.Color",
		Status: 1,
	}
	require.NoError(t, db.Create(&customVendor).Error)
	require.NoError(t, db.Create(&model.User{
		Id:       11005,
		Username: "pricing-custom-vendor-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     11005,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Name:   "openai",
		Group:  "default",
	}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "deepseek/deepseek-v3.2-exp",
		VendorID:  customVendor.Id,
		Status:    1,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "deepseek/deepseek-v3.2-exp",
		ChannelId: 11005,
		Enabled:   true,
	}).Error)
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", 11005)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload getPricingTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Len(t, payload.Data, 1)
	require.Equal(t, customVendor.Id, payload.Data[0].VendorID)

	vendor := findPricingTestVendorByName(payload.Vendors, customVendor.Name)
	require.NotNil(t, vendor)
	require.Equal(t, customVendor.Icon, vendor.Icon)
	require.Nil(t, findPricingTestVendorByName(payload.Vendors, "DeepSeek"))
}

func TestGetPricingDoesNotExposeQiniuMarketSyncError(t *testing.T) {
	setupModelListControllerTestDB(t)
	setting := operation_setting.GetQiniuKeySetting()
	originalSetting := *setting
	t.Cleanup(func() {
		*setting = originalSetting
		model.InvalidatePricingCache()
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/market/models", r.URL.Path)
		w.WriteHeader(http.StatusBadGateway)
		_, err := w.Write([]byte("upstream failed with secret=plain"))
		require.NoError(t, err)
	}))
	defer server.Close()

	*setting = originalSetting
	setting.MarketCatalogEnabled = true
	setting.MarketCatalogBaseURL = server.URL
	setting.MarketCatalogTTLSeconds = 60
	setting.MarketCatalogFallbackEnabled = true
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "market_pricing_sync")
	require.NotContains(t, recorder.Body.String(), "qiniu_market_sync")
	require.Contains(t, recorder.Body.String(), service.QiniuMarketCatalogStatusFallbackLocal)
	require.NotContains(t, recorder.Body.String(), "last_error")
	require.NotContains(t, recorder.Body.String(), "secret=plain")
	require.NotContains(t, recorder.Body.String(), "upstream failed")
}

func TestGetPricingStrictQiniuMarketOnlyReturnsQiniuModels(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	setting := operation_setting.GetQiniuKeySetting()
	originalSetting := *setting
	t.Cleanup(func() {
		*setting = originalSetting
		model.InvalidatePricingCache()
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/market/models", r.URL.Path)
		writeControllerJSON(t, w, map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id":          "kimi-k2",
					"name":        "Kimi K2",
					"description": "official description",
					"pricing_rules_v2": []map[string]any{
						{
							"details_v2": map[string]any{
								"input": map[string]any{
									"unit_name":  "token",
									"unit_size":  1000,
									"unit_price": 0.004,
									"name":       "输入",
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	*setting = originalSetting
	setting.MarketCatalogEnabled = true
	setting.MarketCatalogBaseURL = server.URL
	setting.MarketCatalogTTLSeconds = 60
	setting.MarketCatalogFallbackEnabled = false
	model.InvalidatePricingCache()

	require.NoError(t, db.Create(&model.User{
		Id:       11002,
		Username: "strict-pricing-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     11002,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Name:   "openai",
		Group:  "default",
	}).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "kimi-k2", ChannelId: 11002, Enabled: true}).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "local-only", ChannelId: 11002, Enabled: true}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", 11002)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload getPricingTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, service.QiniuMarketCatalogStatusFresh, payload.MarketPricingSync.Status)
	require.Len(t, payload.Data, 1)
	require.Equal(t, "kimi-k2", payload.Data[0].ModelName)
	require.NotNil(t, payload.Data[0].MarketPricing)
	require.NotContains(t, recorder.Body.String(), "qiniu")
	require.NotContains(t, recorder.Body.String(), "qiniu_")
}

func TestGetPricingStrictQiniuMarketFailureReturnsEmptyData(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	setting := operation_setting.GetQiniuKeySetting()
	originalSetting := *setting
	t.Cleanup(func() {
		*setting = originalSetting
		model.InvalidatePricingCache()
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/market/models", r.URL.Path)
		w.WriteHeader(http.StatusBadGateway)
		_, err := w.Write([]byte("upstream failed with secret=plain"))
		require.NoError(t, err)
	}))
	defer server.Close()

	*setting = originalSetting
	setting.MarketCatalogEnabled = true
	setting.MarketCatalogBaseURL = server.URL
	setting.MarketCatalogTTLSeconds = 60
	setting.MarketCatalogFallbackEnabled = false
	model.InvalidatePricingCache()

	require.NoError(t, db.Create(&model.User{
		Id:       11003,
		Username: "strict-failure-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     11003,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Name:   "openai",
		Group:  "default",
	}).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "local-only", ChannelId: 11003, Enabled: true}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", 11003)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload getPricingTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, service.QiniuMarketCatalogStatusStale, payload.MarketPricingSync.Status)
	require.True(t, payload.MarketPricingSync.Stale)
	require.Empty(t, payload.Data)
	require.NotContains(t, recorder.Body.String(), "last_error")
	require.NotContains(t, recorder.Body.String(), "secret=plain")
	require.NotContains(t, recorder.Body.String(), "upstream failed")
}

func writeControllerJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	data, err := common.Marshal(payload)
	require.NoError(t, err)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	require.NoError(t, err)
}

func disableQiniuMarketCatalogForPricingTest(t *testing.T) {
	t.Helper()
	setting := operation_setting.GetQiniuKeySetting()
	originalSetting := *setting
	t.Cleanup(func() {
		*setting = originalSetting
		model.InvalidatePricingCache()
	})

	*setting = originalSetting
	setting.MarketCatalogEnabled = false
	model.InvalidatePricingCache()
}

func findPricingTestVendorByName(vendors []getPricingTestVendor, name string) *getPricingTestVendor {
	for i := range vendors {
		if vendors[i].Name == name {
			return &vendors[i]
		}
	}
	return nil
}
