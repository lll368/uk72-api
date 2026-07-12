package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestQiniuChildAccountDoesNotChangeExistingQiniuClientPaths(t *testing.T) {
	keyBody := strings.Repeat("8", 64)
	fullKey := "sk-" + keyBody
	called := map[string]bool{}
	childAccountServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/create_child":
			called["child_account_create"] = true
			require.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:"))
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"userid": "child-userid",
					"uid":    "child-uid",
					"email":  "child1@uk72.cn",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user/child_key":
			called["child_account_key"] = true
			require.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:"))
			require.Equal(t, "child-uid", r.URL.Query().Get("uid"))
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"key":    "child-ak",
					"secret": "child-sk",
					"state":  "enabled",
				},
			})
		default:
			t.Fatalf("child account base URL must only receive child-account management requests: %s %s", r.Method, r.URL.String())
		}
	}))
	defer childAccountServer.Close()

	keyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			called["create_key"] = true
			require.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:"))
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			called["quota"] = true
			require.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:"))
			writeQiniuJSON(t, w, map[string]any{"status": true})
		case r.Method == http.MethodGet && r.URL.Path == "/v2/stat/usage":
			called["official_usage"] = true
			require.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:"))
			require.Equal(t, fullKey, r.URL.Query().Get("api_key"))
			writeQiniuJSON(t, w, map[string]any{"status": true, "data": map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == "/v2/stat/usage/apikey/cost-detail":
			called["cost_detail"] = true
			require.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:"))
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data":   map[string]any{"currency": "CNY", "api_keys": []any{}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/market/models":
			called["market"] = true
			require.Empty(t, r.Header.Get("Authorization"))
			require.Equal(t, "true", r.URL.Query().Get("overseas"))
			writeQiniuJSON(t, w, map[string]any{"status": true, "data": []any{}})
		case strings.HasPrefix(r.URL.Path, "/v1/user/"):
			t.Fatalf("key base URL must not receive child account management endpoint: %s %s", r.Method, r.URL.String())
		default:
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
	}))
	defer keyServer.Close()

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:               true,
		BaseURL:               keyServer.URL,
		ChildAccountBaseURL:   childAccountServer.URL,
		AccessKey:             "ak",
		SecretKey:             "sk",
		RequestTimeout:        5,
		MarketCatalogEnabled:  true,
		MarketCatalogBaseURL:  keyServer.URL,
		MarketCatalogOverseas: true,
	})
	require.NoError(t, err)

	_, err = client.CreateChildAccount(context.Background(), "child1@uk72.cn", "login-password")
	require.NoError(t, err)
	_, err = client.GetChildKey(context.Background(), "child-uid", "")
	require.NoError(t, err)
	createdKey, err := client.CreateAPIKey(context.Background(), "child-bound-key")
	require.NoError(t, err)
	require.Equal(t, keyBody, createdKey)
	require.NoError(t, client.SetAPIKeyTotalQuota(context.Background(), keyBody, 12))
	usageStart := time.Date(2026, 1, 1, 0, 0, 0, 0, qiniuCSTLocation)
	_, err = client.QueryOfficialTokenUsage(context.Background(), qiniuOfficialUsageQuery{
		Granularity: "hour",
		Start:       usageStart,
		End:         usageStart.Add(time.Hour),
		APIKey:      keyBody,
	})
	require.NoError(t, err)
	_, err = client.QueryOfficialCostDetails(context.Background(), qiniuOfficialCostDetailQuery{
		StartDate: usageStart,
		EndDate:   usageStart.AddDate(0, 0, 1),
		Grain:     "day",
	})
	require.NoError(t, err)
	_, err = client.QueryMarketModels(context.Background())
	require.NoError(t, err)

	require.True(t, called["child_account_create"])
	require.True(t, called["child_account_key"])
	require.True(t, called["create_key"])
	require.True(t, called["quota"])
	require.True(t, called["official_usage"])
	require.True(t, called["cost_detail"])
	require.True(t, called["market"])
}
