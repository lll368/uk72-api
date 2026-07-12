package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestQiniuChildAccountClientOfficialContractsAndOEMEnabled(t *testing.T) {
	var createCalled bool
	var keyCalled bool
	var disableCalled bool
	var enableCalled bool
	var oemEnabledCalled bool
	ak := strings.Repeat("a", 40)
	sk := strings.Repeat("b", 40)
	childAccountServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/create_child":
			createCalled = true
			require.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu parent-ak:"))
			require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
			require.NoError(t, r.ParseForm())
			require.Equal(t, "child1@uk72.cn", r.Form.Get("email"))
			require.Equal(t, "login-password", r.Form.Get("password"))
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"userid":      "child-userid",
					"uid":         "child-uid",
					"parent_uid":  "parent-uid",
					"email":       "child1@uk72.cn",
					"is_disabled": false,
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user/child_key":
			keyCalled = true
			require.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu parent-ak:"))
			require.Equal(t, "child-uid", r.URL.Query().Get("uid"))
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"key":     ak,
					"secret":  sk,
					"state":   "enabled",
					"key2":    "backup-ak",
					"secret2": "backup-sk",
					"state2":  "disabled",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/disable-child":
			disableCalled = true
			require.NoError(t, r.ParseForm())
			require.Equal(t, "child-uid", r.Form.Get("uid"))
			require.Equal(t, "risk control", r.Form.Get("reason"))
			writeQiniuJSON(t, w, map[string]any{"status": true})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/enable-child":
			enableCalled = true
			require.NoError(t, r.ParseForm())
			require.Equal(t, "child-uid", r.Form.Get("uid"))
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected qiniu child account request %s %s", r.Method, r.URL.String())
		}
	}))
	defer childAccountServer.Close()

	keyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/ai/inapi/v2/apikey/enabled":
			oemEnabledCalled = true
			require.Equal(t, "Bearer child-token", r.Header.Get("Authorization"))
			var payload map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			require.Equal(t, "sk-"+strings.Repeat("c", 64), payload["key"])
			require.Equal(t, false, payload["enabled"])
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected qiniu key request %s %s", r.Method, r.URL.String())
		}
	}))
	defer keyServer.Close()

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:             true,
		BaseURL:             keyServer.URL,
		ChildAccountBaseURL: childAccountServer.URL,
		AccessKey:           "parent-ak",
		SecretKey:           "parent-sk",
		RequestTimeout:      5,
	})
	require.NoError(t, err)

	created, err := client.CreateChildAccount(context.Background(), "child1@uk72.cn", "login-password")
	require.NoError(t, err)
	require.Equal(t, "child-uid", created.UID)
	require.Equal(t, "child-userid", created.UserID)
	require.Equal(t, "parent-uid", created.ParentUID)
	require.False(t, created.IsDisabled)

	keys, err := client.GetChildKey(context.Background(), created.UID, "")
	require.NoError(t, err)
	require.Equal(t, ak, keys.AccessKey)
	require.Equal(t, sk, keys.SecretKey)
	require.Equal(t, "backup-ak", keys.BackupAccessKey)

	require.NoError(t, client.DisableChildAccount(context.Background(), created.UID, "risk control"))
	require.NoError(t, client.EnableChildAccount(context.Background(), created.UID))
	require.NoError(t, client.SetOEMAPIKeyEnabled(context.Background(), "child-token", strings.Repeat("c", 64), false))
	require.True(t, createCalled)
	require.True(t, keyCalled)
	require.True(t, disableCalled)
	require.True(t, enableCalled)
	require.True(t, oemEnabledCalled)
}

func TestQiniuChildAccountClientAllowsCreateResponseWithoutUIDAndSanitizes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeQiniuJSON(t, w, map[string]any{"status": true, "data": map[string]any{}})
	}))
	defer server.Close()

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:             true,
		BaseURL:             server.URL,
		ChildAccountBaseURL: server.URL,
		AccessKey:           "parent-ak",
		SecretKey:           "parent-sk",
		RequestTimeout:      5,
	})
	require.NoError(t, err)

	remote, err := client.CreateChildAccount(context.Background(), "child1@uk72.cn", "login-password")
	require.NoError(t, err)
	require.Empty(t, remote.UID)
	require.Empty(t, remote.UserID)
	_, err = client.GetChildKey(context.Background(), "child-uid", "")
	require.Error(t, err)

	safe := SanitizeQiniuChildAccountSecret("Authorization: Qiniu parent-ak:sign secret=" + strings.Repeat("d", 40) + " key=sk-" + strings.Repeat("e", 64))
	require.NotContains(t, safe, "parent-ak")
	require.NotContains(t, safe, strings.Repeat("d", 40))
	require.NotContains(t, safe, strings.Repeat("e", 64))
}

func TestQiniuChildAccountClientRejectsBusinessAndHTTPFailures(t *testing.T) {
	t.Run("business_error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v1/user/create_child", r.URL.Path)
			writeQiniuJSON(t, w, map[string]any{
				"status": false,
				"code":   "child_create_denied",
				"error":  "child create denied",
			})
		}))
		defer server.Close()

		client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
			Enabled:             true,
			BaseURL:             server.URL,
			ChildAccountBaseURL: server.URL,
			AccessKey:           "parent-ak",
			SecretKey:           "parent-sk",
			RequestTimeout:      5,
		})
		require.NoError(t, err)

		_, err = client.CreateChildAccount(context.Background(), "child1@uk72.cn", "login-password")
		require.Error(t, err)
		require.Contains(t, err.Error(), "child create denied")
	})

	t.Run("non_2xx", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/v1/user/child_key", r.URL.Path)
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		}))
		defer server.Close()

		client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
			Enabled:             true,
			BaseURL:             server.URL,
			ChildAccountBaseURL: server.URL,
			AccessKey:           "parent-ak",
			SecretKey:           "parent-sk",
			RequestTimeout:      5,
		})
		require.NoError(t, err)

		_, err = client.GetChildKey(context.Background(), "child-uid", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "502")
	})
}

func TestQiniuChildAccountClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()

	client, err := newQiniuKeyClient(&operation_setting.QiniuKeySetting{
		Enabled:             true,
		BaseURL:             server.URL,
		ChildAccountBaseURL: server.URL,
		AccessKey:           "parent-ak",
		SecretKey:           "parent-sk",
		RequestTimeout:      1,
	})
	require.NoError(t, err)
	client.httpClient.Timeout = time.Millisecond

	_, err = client.CreateChildAccount(context.Background(), "child1@uk72.cn", "login-password")
	require.Error(t, err)
}
