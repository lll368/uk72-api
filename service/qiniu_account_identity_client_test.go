package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestNewQiniuAccountIdentityClientUsesParentAndChildCredentials(t *testing.T) {
	truncate(t)
	var authHeaders []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	parentClient, err := NewQiniuAccountIdentityClient(0, QiniuAccountOperationQuota)
	require.NoError(t, err)
	require.NoError(t, parentClient.SetAPIKeyTotalQuota(context.Background(), strings.Repeat("a", 64), 1))
	require.True(t, strings.HasPrefix(authHeaders[0], "Qiniu ak:"))

	account := seedQiniuIdentityClientAccount(t, 701, model.QiniuChildAccountStatusEnabled, "child-ak", "child-sk")
	childClient, err := NewQiniuAccountIdentityClient(account.Id, QiniuAccountOperationCreate)
	require.NoError(t, err)
	require.NoError(t, childClient.SetAPIKeyTotalQuota(context.Background(), strings.Repeat("b", 64), 1))
	require.True(t, strings.HasPrefix(authHeaders[1], "Qiniu child-ak:"))
}

func TestNewQiniuAccountIdentityClientDistinguishesCreateAndHistoricalState(t *testing.T) {
	truncate(t)
	configureQiniuKeySettingForTest(t, "http://127.0.0.1")
	account := seedQiniuIdentityClientAccount(t, 702, model.QiniuChildAccountStatusDisabled, "child-disabled-ak", "child-disabled-sk")

	_, err := NewQiniuAccountIdentityClient(account.Id, QiniuAccountOperationCreate)
	require.Error(t, err)
	require.Contains(t, err.Error(), "未启用")

	client, err := NewQiniuAccountIdentityClient(account.Id, QiniuAccountOperationHistoricalRevoke)
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestNewQiniuAccountIdentityClientRejectsUndecryptableCredentialsWithoutLeaks(t *testing.T) {
	truncate(t)
	configureQiniuKeySettingForTest(t, "http://127.0.0.1")
	account := seedQiniuIdentityClientAccount(t, 703, model.QiniuChildAccountStatusEnabled, "child-raw-ak", "child-raw-sk")
	require.NoError(t, model.DB.Model(&model.QiniuChildAccount{}).Where("id = ?", account.Id).Updates(map[string]interface{}{
		"access_key": model.MaskQiniuChildAccountAK("child-raw-ak"),
		"secret_key": "hmac:legacy-digest",
	}).Error)

	_, err := NewQiniuAccountIdentityClient(account.Id, QiniuAccountOperationCreate)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "child-raw-ak")
	require.NotContains(t, err.Error(), "child-raw-sk")
	require.NotContains(t, err.Error(), "Authorization")
}

func TestNewQiniuAccountIdentityClientUsesStoredEncryptedChildCredentials(t *testing.T) {
	truncate(t)
	configureQiniuKeySettingForTest(t, "http://127.0.0.1")
	account := seedQiniuIdentityClientAccount(t, 704, model.QiniuChildAccountStatusEnabled, "", "")
	require.NoError(t, model.MarkQiniuChildAccountCredentials(account.Id, "child-stored-ak", "child-stored-sk", "enabled", "", "", ""))

	client, err := NewQiniuAccountIdentityClient(account.Id, QiniuAccountOperationCreate)
	require.NoError(t, err)
	require.Equal(t, "child-stored-ak", client.setting.AccessKey)
	require.Equal(t, "child-stored-sk", client.setting.SecretKey)
}

func TestSanitizeQiniuTaskErrorRedactsChildAccountSigningCredentials(t *testing.T) {
	rawAK := "child-ak-" + strings.Repeat("a", 32)
	rawSK := "child-sk-" + strings.Repeat("b", 32)
	rawKey := "sk-" + strings.Repeat("c", 64)
	safe := sanitizeQiniuTaskError(errors.New("Authorization: Qiniu " + rawAK + ":signature secret=" + rawSK + " key=" + rawKey))

	require.NotContains(t, safe, rawAK)
	require.NotContains(t, safe, rawSK)
	require.NotContains(t, safe, rawKey)
	require.Contains(t, safe, "Qiniu ********")
}

func seedQiniuIdentityClientAccount(t *testing.T, sequenceNo int, status string, accessKey string, secretKey string) *model.QiniuChildAccount {
	t.Helper()
	account := &model.QiniuChildAccount{
		SequenceNo: sequenceNo,
		Email:      "identity-client@uk72.cn",
		UID:        "identity-client-uid",
		AccessKey:  accessKey,
		SecretKey:  secretKey,
		Status:     status,
	}
	require.NoError(t, model.DB.Create(account).Error)
	return account
}
