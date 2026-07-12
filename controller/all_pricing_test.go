package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetAllPricingDoesNotRequireLoginAndForcesOverseas(t *testing.T) {
	setupModelListControllerTestDB(t)
	t.Cleanup(model.InvalidatePricingCache)
	router := setupAllPricingRouter()
	var observed struct {
		Method     string
		Path       string
		RawQuery   string
		Auth       string
		NewAPIUser string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed.Method = r.Method
		observed.Path = r.URL.Path
		observed.RawQuery = r.URL.RawQuery
		observed.Auth = r.Header.Get("Authorization")
		observed.NewAPIUser = r.Header.Get("New-Api-User")
		writeAllPricingTestJSON(t, w, map[string]any{
			"status": true,
			"data": []map[string]any{
				{
					"id":          "deepseek-v3",
					"name":        "DeepSeek V3",
					"description": "DeepSeek V3 description",
					"avatar":      "https://static.example.com/deepseek.png",
					"hot_tags":    []string{"海外"},
					"features":    []string{"工具调用"},
					"issuer": map[string]any{
						"name": "Qiniu",
					},
					"model_constraints": map[string]any{
						"context_length":        64000,
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
					"support_api_protocols": []string{"openai"},
					"release_at":            "2025-08-05",
				},
			},
		})
	}))
	defer server.Close()
	configureAllPricingSettingForTest(t, server.URL, false)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/all-pricing?overseas=false&ignored=value", nil)
	req.Header.Set("Authorization", "Bearer client-token")
	req.Header.Set("New-Api-User", "123")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success           bool                 `json:"success"`
		Data              []publicPricingModel `json:"data"`
		Vendors           []map[string]any     `json:"vendors"`
		GroupRatio        map[string]float64   `json:"group_ratio"`
		UsableGroup       map[string]string    `json:"usable_group"`
		SupportedEndpoint map[string]any       `json:"supported_endpoint"`
		AutoGroups        []string             `json:"auto_groups"`
		MarketPricingSync map[string]any       `json:"market_pricing_sync"`
		PricingVersion    string               `json:"pricing_version"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Len(t, resp.Data, 1)
	require.Equal(t, "deepseek-v3", resp.Data[0].ModelName)
	require.Equal(t, "DeepSeek V3 description", resp.Data[0].Description)
	require.Equal(t, "https://static.example.com/deepseek.png", resp.Data[0].Icon)
	require.Equal(t, "海外,工具调用", resp.Data[0].Tags)
	require.Equal(t, []string{"all"}, resp.Data[0].EnableGroup)
	require.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, resp.Data[0].SupportedEndpointTypes)
	require.True(t, resp.Data[0].Enabled)
	require.True(t, resp.Data[0].Routable)
	require.Equal(t, int64(64000), resp.Data[0].ContextLength)
	require.Equal(t, int64(8192), resp.Data[0].MaxOutputTokens)
	require.Equal(t, "2025-08-05", resp.Data[0].ReleaseDate)
	require.NotNil(t, resp.Data[0].MarketPricing)
	require.Equal(t, "deepseek-v3", resp.Data[0].MarketPricing.ID)
	require.NotEmpty(t, resp.GroupRatio)
	require.NotNil(t, resp.UsableGroup)
	require.NotNil(t, resp.SupportedEndpoint)
	require.NotNil(t, resp.AutoGroups)
	require.Equal(t, "fresh", resp.MarketPricingSync["status"])
	require.NotEmpty(t, resp.PricingVersion)
	require.NotNil(t, resp.Vendors)
	require.Equal(t, http.MethodGet, observed.Method)
	require.Equal(t, "/v1/market/models", observed.Path)
	require.Equal(t, "overseas=true", observed.RawQuery)
	require.Empty(t, observed.Auth)
	require.Empty(t, observed.NewAPIUser)
}

func TestGetAllPricingReturnsPricingLikeApiErrorOnUpstreamFailure(t *testing.T) {
	setupModelListControllerTestDB(t)
	t.Cleanup(model.InvalidatePricingCache)
	router := setupAllPricingRouter()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/market/models", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("overseas"))
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"upstream failed"}`))
	}))
	defer server.Close()
	configureAllPricingSettingForTest(t, server.URL, true)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/all-pricing", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success        bool                 `json:"success"`
		Message        string               `json:"message"`
		Data           []publicPricingModel `json:"data"`
		Vendors        []map[string]any     `json:"vendors"`
		GroupRatio     map[string]float64   `json:"group_ratio"`
		UsableGroup    map[string]string    `json:"usable_group"`
		PricingVersion string               `json:"pricing_version"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "模型市场接口返回异常状态 500")
	require.Empty(t, resp.Data)
	require.NotNil(t, resp.Vendors)
	require.NotEmpty(t, resp.GroupRatio)
	require.NotNil(t, resp.UsableGroup)
	require.NotEmpty(t, resp.PricingVersion)
}

func setupAllPricingRouter() *gin.Engine {
	router := gin.New()
	router.GET("/api/all-pricing", GetAllPricing)
	return router
}

func configureAllPricingSettingForTest(t *testing.T, baseURL string, overseas bool) {
	t.Helper()
	setting := operation_setting.GetQiniuKeySetting()
	original := *setting
	setting.MarketCatalogEnabled = false
	setting.MarketCatalogBaseURL = baseURL
	setting.MarketCatalogOverseas = overseas
	setting.RequestTimeout = 5
	t.Cleanup(func() {
		*setting = original
	})
}

func writeAllPricingTestJSON(t *testing.T, w http.ResponseWriter, payload map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}
