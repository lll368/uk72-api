package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

type qiniuKeyAdminListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total    int                 `json:"total"`
		Page     int                 `json:"page"`
		PageSize int                 `json:"page_size"`
		Items    []adminQiniuKeyView `json:"items"`
	} `json:"data"`
}

type qiniuKeyAdminMutationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		TokenId int    `json:"token_id"`
		UserId  int    `json:"user_id"`
		Status  int    `json:"status"`
		Key     string `json:"key"`
	} `json:"data"`
}

func TestAdminListQiniuKeysRequiresAdminAuth(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)

	anonymousRecorder := performQiniuAdminRawRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-keys", "", nil, "8601")
	require.Equal(t, http.StatusUnauthorized, anonymousRecorder.Code)

	userCookies := loginQiniuOfficialAdminTestUser(t, router, "user", "8602")
	userRecorder := performQiniuAdminRawRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-keys", "", userCookies, "8602")
	require.Equal(t, http.StatusOK, userRecorder.Code)
	var resp qiniuKeyAdminListResponse
	require.NoError(t, common.Unmarshal(userRecorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
}

func TestAdminListQiniuKeysReturnsMaskedKeysOwnerQuotaAndLatestTask(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	now := common.GetTimestamp()
	userId := 9701
	activeTokenId := 9702
	deletedTokenId := 9703
	fullKey := strings.Repeat("e", 64)
	deletedKey := strings.Repeat("f", 64)
	localKey := strings.Repeat("g", 64)
	childAccount := &model.QiniuChildAccount{
		SequenceNo: 10,
		Email:      "key-child@uk72.cn",
		UID:        "key-child-uid",
		Status:     model.QiniuChildAccountStatusEnabled,
	}
	require.NoError(t, model.DB.Create(childAccount).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          userId,
		Username:    "qiniu-admin-key-user",
		DisplayName: "Qiniu Admin Key User",
		Email:       "qiniu-admin-key@example.com",
		Password:    "password",
		Status:      common.UserStatusEnabled,
		Group:       "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:                  activeTokenId,
		UserId:              userId,
		Name:                "active-qiniu-key",
		Key:                 fullKey,
		Provider:            model.TokenProviderQiniu,
		QiniuChildAccountId: childAccount.Id,
		Status:              common.TokenStatusEnabled,
		CreatedTime:         now - 100,
		AccessedTime:        now - 10,
		ExpiredTime:         -1,
		UnlimitedQuota:      true,
		Group:               "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             deletedTokenId,
		UserId:         userId,
		Name:           "deleted-qiniu-key",
		Key:            deletedKey,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusDisabled,
		CreatedTime:    now - 90,
		AccessedTime:   now - 9,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          "default",
	}).Error)
	var deletedToken model.Token
	require.NoError(t, model.DB.First(&deletedToken, "id = ?", deletedTokenId).Error)
	require.NoError(t, model.DB.Delete(&deletedToken).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             9704,
		UserId:         userId,
		Name:           "local-key",
		Key:            localKey,
		Provider:       model.TokenProviderLocal,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    now - 80,
		AccessedTime:   now - 8,
		ExpiredTime:    -1,
		UnlimitedQuota: false,
		Group:          "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.QiniuQuotaGrant{
		UserId:            userId,
		TokenId:           activeTokenId,
		BusinessKey:       "admin-qiniu-key-applied",
		GrantAmount:       12.5,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusApplied,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QiniuQuotaGrant{
		UserId:            userId,
		TokenId:           activeTokenId,
		BusinessKey:       "admin-qiniu-key-pending",
		GrantAmount:       3.25,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusPending,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QiniuQuotaGrant{
		UserId:            userId,
		TokenId:           activeTokenId,
		BusinessKey:       "admin-qiniu-key-failed-old",
		GrantAmount:       1,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusFailed,
		LastError:         "old failed key sk-" + fullKey,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QiniuQuotaGrant{
		UserId:            userId,
		TokenId:           activeTokenId,
		BusinessKey:       "admin-qiniu-key-failed-new",
		GrantAmount:       2,
		RemoteApplyStatus: model.QiniuQuotaGrantStatusFailed,
		LastError:         "new failed key sk-" + fullKey,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QiniuKeySyncTask{
		TaskType:      model.QiniuKeyTaskTypeDefaultCreate,
		UserId:        userId,
		TokenId:       activeTokenId,
		QiniuKey:      fullKey,
		Status:        model.QiniuKeyTaskStatusSuccess,
		RetryCount:    0,
		NextRetryTime: now + 60,
		LastError:     "old task key sk-" + fullKey,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QiniuKeySyncTask{
		TaskType:      model.QiniuKeyTaskTypeRevoke,
		UserId:        userId,
		TokenId:       activeTokenId,
		QiniuKey:      fullKey,
		Status:        model.QiniuKeyTaskStatusFailed,
		RetryCount:    2,
		NextRetryTime: now + 120,
		LastError:     "latest task key sk-" + fullKey,
	}).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	defaultRecorder := performQiniuAdminRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-keys?user_id=9701&status=1&qiniu_child_account_id="+strconv.Itoa(childAccount.Id)+"&qiniu_key=eeee", "", cookies)
	var defaultResp qiniuKeyAdminListResponse
	require.NoError(t, common.Unmarshal(defaultRecorder.Body.Bytes(), &defaultResp))
	require.True(t, defaultResp.Success, defaultResp.Message)
	require.Equal(t, 1, defaultResp.Data.Total)
	require.Len(t, defaultResp.Data.Items, 1)
	item := defaultResp.Data.Items[0]
	require.Equal(t, activeTokenId, item.TokenId)
	require.Equal(t, childAccount.Id, item.QiniuChildAccountId)
	require.NotNil(t, item.QiniuChildAccount)
	require.Equal(t, "key-child@uk72.cn", item.QiniuChildAccount.Email)
	require.Equal(t, "key-child-uid", item.QiniuChildAccount.UID)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, item.QiniuChildAccount.Status)
	require.Equal(t, userId, item.User.Id)
	require.Equal(t, "qiniu-admin-key-user", item.User.Username)
	require.Equal(t, "Qiniu Admin Key User", item.User.DisplayName)
	require.Equal(t, "qiniu-admin-key@example.com", item.User.Email)
	require.Equal(t, model.MaskTokenKey(fullKey), item.Key)
	require.NotContains(t, defaultRecorder.Body.String(), fullKey)
	require.False(t, item.Deleted)
	require.Equal(t, 12.5, item.Quota.AppliedLimitAmount)
	require.Equal(t, 3.25, item.Quota.PendingLimitAmount)
	require.Equal(t, 3.0, item.Quota.FailedLimitAmount)
	require.NotContains(t, item.Quota.LatestGrantError, fullKey)
	require.Equal(t, model.QiniuKeyTaskTypeRevoke, item.LatestTask.TaskType)
	require.Equal(t, model.QiniuKeyTaskStatusFailed, item.LatestTask.Status)
	require.Equal(t, 2, item.LatestTask.RetryCount)
	require.NotContains(t, item.LatestTask.LastError, fullKey)

	deletedRecorder := performQiniuAdminRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-keys?include_deleted=true&token_id=9703", "", cookies)
	var deletedResp qiniuKeyAdminListResponse
	require.NoError(t, common.Unmarshal(deletedRecorder.Body.Bytes(), &deletedResp))
	require.True(t, deletedResp.Success, deletedResp.Message)
	require.Equal(t, 1, deletedResp.Data.Total)
	require.True(t, deletedResp.Data.Items[0].Deleted)
	require.NotZero(t, deletedResp.Data.Items[0].DeletedTime)
}

func TestAdminDisableQiniuKeyRequiresAdminAuth(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)

	anonymousRecorder := performQiniuAdminRawRequest(t, router, http.MethodPost, "/api/payment/admin/qiniu-keys/9702/disable", "", nil, "8601")
	require.Equal(t, http.StatusUnauthorized, anonymousRecorder.Code)

	userCookies := loginQiniuOfficialAdminTestUser(t, router, "user", "8602")
	userRecorder := performQiniuAdminRawRequest(t, router, http.MethodPost, "/api/payment/admin/qiniu-keys/9702/disable", "", userCookies, "8602")
	require.Equal(t, http.StatusOK, userRecorder.Code)
	var resp qiniuKeyAdminMutationResponse
	require.NoError(t, common.Unmarshal(userRecorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
}

func TestAdminDisableQiniuKeySuccess(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	keyBody := strings.Repeat("a", 64)
	fullKey := "sk-" + keyBody
	remoteCalls := 0
	var observed map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteCalls++
		if r.Method != http.MethodPut || r.URL.Path != "/ai/inapi/v2/apikey/enabled" {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		require.NoError(t, common.DecodeJson(r.Body, &observed))
		writeQiniuAdminJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()
	configureAdminDisableQiniuSettingForTest(t, server.URL)

	require.NoError(t, model.DB.Create(&model.User{
		Id:       9801,
		Username: "qiniu-disable-user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             9802,
		UserId:         9801,
		Name:           "qiniu-disable-token",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	recorder := performQiniuAdminRequest(t, router, http.MethodPost, "/api/payment/admin/qiniu-keys/9802/disable", `{"reason":"manual disable"}`, cookies)
	var resp qiniuKeyAdminMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Equal(t, 9802, resp.Data.TokenId)
	require.Equal(t, 9801, resp.Data.UserId)
	require.Equal(t, common.TokenStatusDisabled, resp.Data.Status)
	require.Equal(t, model.MaskTokenKey(keyBody), resp.Data.Key)
	require.NotContains(t, recorder.Body.String(), fullKey)
	require.Equal(t, 1, remoteCalls)
	require.Equal(t, fullKey, observed["key"])
	require.Equal(t, false, observed["enabled"])

	reloaded, err := model.GetTokenById(9802)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, reloaded.Status)
}

func TestAdminDisableQiniuKeyRemoteFailureKeepsLocalState(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	keyBody := strings.Repeat("b", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/ai/inapi/v2/apikey/enabled" {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		writeQiniuAdminJSON(t, w, map[string]any{
			"status": false,
			"code":   "remote_failed",
			"error":  "remote disable failed",
		})
	}))
	defer server.Close()
	configureAdminDisableQiniuSettingForTest(t, server.URL)

	require.NoError(t, model.DB.Create(&model.User{Id: 9803, Username: "qiniu-disable-failure", Status: common.UserStatusEnabled, Group: "default"}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             9804,
		UserId:         9803,
		Name:           "qiniu-disable-failure",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	recorder := performQiniuAdminRequest(t, router, http.MethodPost, "/api/payment/admin/qiniu-keys/9804/disable", "", cookies)
	var resp qiniuKeyAdminMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "remote disable failed")

	reloaded, err := model.GetTokenById(9804)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusEnabled, reloaded.Status)
	var tasks []model.QiniuKeySyncTask
	require.NoError(t, model.DB.Find(&tasks, "token_id = ?", 9804).Error)
	require.Empty(t, tasks)
}

func TestAdminDisableQiniuKeyValidationFailures(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	remoteCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteCalls++
		t.Fatalf("validation failure must not call qiniu: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	configureAdminDisableQiniuSettingForTest(t, server.URL)

	require.NoError(t, model.DB.Create(&model.User{Id: 9805, Username: "qiniu-disable-validation", Status: common.UserStatusEnabled, Group: "default"}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             9806,
		UserId:         9805,
		Name:           "local-token",
		Key:            "local-token",
		Provider:       model.TokenProviderLocal,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             9807,
		UserId:         9805,
		Name:           "disabled-token",
		Key:            strings.Repeat("c", 64),
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusDisabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             9808,
		UserId:         9805,
		Name:           "deleted-token",
		Key:            strings.Repeat("d", 64),
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	var deleted model.Token
	require.NoError(t, model.DB.First(&deleted, "id = ?", 9808).Error)
	require.NoError(t, model.DB.Delete(&deleted).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	cases := []struct {
		path    string
		message string
	}{
		{path: "/api/payment/admin/qiniu-keys/abc/disable", message: "Key ID 无效"},
		{path: "/api/payment/admin/qiniu-keys/9806/disable", message: "七牛"},
		{path: "/api/payment/admin/qiniu-keys/9807/disable", message: "启用"},
		{path: "/api/payment/admin/qiniu-keys/9808/disable", message: "record not found"},
		{path: "/api/payment/admin/qiniu-keys/9999/disable", message: "record not found"},
	}
	for _, tc := range cases {
		recorder := performQiniuAdminRequest(t, router, http.MethodPost, tc.path, "", cookies)
		var resp qiniuKeyAdminMutationResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
		require.False(t, resp.Success, tc.path)
		require.Contains(t, resp.Message, tc.message, tc.path)
	}
	require.Equal(t, 0, remoteCalls)
}

func TestAdminDisableQiniuKeyAlreadyDisabledRemoteResponseSucceeds(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	keyBody := strings.Repeat("e", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/ai/inapi/v2/apikey/enabled" {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		writeQiniuAdminJSON(t, w, map[string]any{
			"status": false,
			"code":   "api_key_already_disabled",
			"error":  "api key already disabled",
		})
	}))
	defer server.Close()
	configureAdminDisableQiniuSettingForTest(t, server.URL)

	require.NoError(t, model.DB.Create(&model.User{Id: 9809, Username: "qiniu-disable-idempotent", Status: common.UserStatusEnabled, Group: "default"}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             9810,
		UserId:         9809,
		Name:           "already-disabled-remote",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	recorder := performQiniuAdminRequest(t, router, http.MethodPost, "/api/payment/admin/qiniu-keys/9810/disable", "", cookies)
	var resp qiniuKeyAdminMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Equal(t, common.TokenStatusDisabled, resp.Data.Status)
}

func configureAdminDisableQiniuSettingForTest(t *testing.T, baseURL string) {
	t.Helper()
	qiniuSetting := operation_setting.GetQiniuKeySetting()
	qiniuSetting.Enabled = true
	qiniuSetting.BaseURL = baseURL
	qiniuSetting.AccessKey = "ak"
	qiniuSetting.SecretKey = "sk"
	qiniuSetting.RequestTimeout = 5
}

func writeQiniuAdminJSON(t *testing.T, w http.ResponseWriter, payload map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

func performQiniuAdminRawRequest(t *testing.T, router http.Handler, method string, path string, body string, cookies []*http.Cookie, userId string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	requestBody := bytes.NewBufferString(body)
	req := httptest.NewRequest(method, path, requestBody)
	if userId != "" {
		req.Header.Set("New-Api-User", userId)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}
