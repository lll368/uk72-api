package controller

import (
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

type qiniuChildAccountAdminListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int                          `json:"total"`
		Items []adminQiniuChildAccountView `json:"items"`
	} `json:"data"`
}

type qiniuChildAccountAdminDetailResponse struct {
	Success bool                             `json:"success"`
	Message string                           `json:"message"`
	Data    adminQiniuChildAccountDetailView `json:"data"`
}

type qiniuChildAccountAdminMutationResponse struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Data    adminQiniuChildAccountView `json:"data"`
}

func TestAdminQiniuChildAccountsRequireAdminAuth(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)

	anonymousRecorder := performQiniuAdminRawRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-child-accounts", "", nil, "8601")
	require.Equal(t, http.StatusUnauthorized, anonymousRecorder.Code)

	userCookies := loginQiniuOfficialAdminTestUser(t, router, "user", "8602")
	userRecorder := performQiniuAdminRawRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-child-accounts", "", userCookies, "8602")
	require.Equal(t, http.StatusOK, userRecorder.Code)
	var resp qiniuChildAccountAdminListResponse
	require.NoError(t, common.Unmarshal(userRecorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
}

func TestAdminQiniuChildAccountListDetailAndSensitiveRedaction(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	fullAK := strings.Repeat("a", 40)
	fullSK := strings.Repeat("b", 40)
	account := &model.QiniuChildAccount{
		SequenceNo:    1,
		Email:         "child1@uk72.cn",
		RemoteUserID:  "remote-userid",
		UID:           "remote-uid",
		ParentUID:     "parent-uid",
		AccessKey:     fullAK,
		SecretKey:     fullSK,
		KeyState:      "enabled",
		Status:        model.QiniuChildAccountStatusEnabled,
		LoginPassword: "login-password",
	}
	require.NoError(t, model.DB.Create(account).Error)
	require.NoError(t, model.DB.Create(&model.QiniuChildAccountSyncTask{
		AccountId:     account.Id,
		TaskType:      model.QiniuChildAccountTaskTypeCreate,
		Status:        model.QiniuChildAccountTaskStatusFailed,
		RetryCount:    2,
		NextRetryTime: common.GetTimestamp() + 60,
		LastError:     "remote failed ak=" + fullAK + " sk=" + fullSK + " auth=Qiniu qiniu-ak-secret:signature",
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:                  8701,
		Username:            "child-bound-user",
		DisplayName:         "Child Bound",
		Email:               "child-bound@example.com",
		Status:              common.UserStatusEnabled,
		QiniuChildAccountId: account.Id,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:                  8702,
		UserId:              8701,
		Name:                "child-bound-key",
		Key:                 strings.Repeat("d", 64),
		Provider:            model.TokenProviderQiniu,
		QiniuChildAccountId: account.Id,
		Status:              common.TokenStatusEnabled,
		CreatedTime:         common.GetTimestamp(),
		AccessedTime:        common.GetTimestamp(),
		ExpiredTime:         -1,
		UnlimitedQuota:      true,
	}).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	recorder := performQiniuAdminRawRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-child-accounts?status=enabled&email=child1", "", cookies, "8601")
	require.Equal(t, http.StatusOK, recorder.Code)
	var listResp qiniuChildAccountAdminListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Equal(t, 1, listResp.Data.Total)
	require.Len(t, listResp.Data.Items, 1)
	item := listResp.Data.Items[0]
	require.Equal(t, account.Id, item.Id)
	require.Equal(t, "child1@uk72.cn", item.Email)
	require.NotEqual(t, fullAK, item.AccessKey)
	require.NotContains(t, item.AccessKey, fullAK)
	require.NotContains(t, item.LatestTask.LastError, fullAK)
	require.NotContains(t, item.LatestTask.LastError, fullSK)
	require.Equal(t, 1, item.UserCount)

	detailRecorder := performQiniuAdminRawRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-child-accounts/"+strconv.Itoa(account.Id), "", cookies, "8601")
	require.Equal(t, http.StatusOK, detailRecorder.Code)
	var detailResp qiniuChildAccountAdminDetailResponse
	require.NoError(t, common.Unmarshal(detailRecorder.Body.Bytes(), &detailResp))
	require.True(t, detailResp.Success, detailResp.Message)
	require.Equal(t, account.Id, detailResp.Data.Id)
	require.Len(t, detailResp.Data.Users, 1)
	require.Equal(t, 8701, detailResp.Data.Users[0].Id)
	require.Len(t, detailResp.Data.Tokens, 1)
	require.Equal(t, 8702, detailResp.Data.Tokens[0].Id)
	require.Equal(t, int64(1), detailResp.Data.Impact.AssociatedUserCount)
	require.Equal(t, int64(1), detailResp.Data.Impact.AssociatedTokenCount)
	require.Equal(t, int64(1), detailResp.Data.Impact.EnabledTokenCount)
	require.NotContains(t, detailResp.Data.Tasks[0].LastError, fullAK)
	require.NotContains(t, detailResp.Data.Tasks[0].LastError, fullSK)
}

func TestAdminQiniuChildAccountRejectsInvalidParameters(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")

	recorder := performQiniuAdminRawRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-child-accounts?status=invalid", "", cookies, "8601")
	require.Equal(t, http.StatusOK, recorder.Code)
	var resp qiniuChildAccountAdminListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)

	recorder = performQiniuAdminRawRequest(t, router, http.MethodPost, "/api/payment/admin/qiniu-child-accounts/0/disable", `{"reason":""}`, cookies, "8601")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
}

func TestAdminQiniuChildAccountDisableResponseIncludesImpact(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/user/disable-child", r.URL.Path)
		writeQiniuAdminJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()
	setting := operation_setting.GetQiniuKeySetting()
	setting.Enabled = true
	setting.BaseURL = server.URL
	setting.AccessKey = "ak"
	setting.SecretKey = "sk"
	setting.ChildAccountEmailDomain = "uk72.cn"
	setting.ChildAccountEmailPrefix = "child"
	setting.ChildAccountRequestTimeout = 5
	setting.ChildAccountRetryIntervalSeconds = 1

	account := &model.QiniuChildAccount{
		SequenceNo: 3,
		Email:      "child-impact@uk72.cn",
		UID:        "impact-uid",
		Status:     model.QiniuChildAccountStatusEnabled,
	}
	require.NoError(t, model.DB.Create(account).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:                  8703,
		Username:            "child-impact-user",
		Status:              common.UserStatusEnabled,
		QiniuChildAccountId: account.Id,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:                  8704,
		UserId:              8703,
		Name:                "child-impact-key",
		Key:                 strings.Repeat("e", 64),
		Provider:            model.TokenProviderQiniu,
		QiniuChildAccountId: account.Id,
		Status:              common.TokenStatusEnabled,
		CreatedTime:         common.GetTimestamp(),
		AccessedTime:        common.GetTimestamp(),
		ExpiredTime:         -1,
		UnlimitedQuota:      true,
	}).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	recorder := performQiniuAdminRawRequest(t, router, http.MethodPost, "/api/payment/admin/qiniu-child-accounts/"+strconv.Itoa(account.Id)+"/disable", `{"reason":"ops"}`, cookies, "8601")
	require.Equal(t, http.StatusOK, recorder.Code)
	var resp qiniuChildAccountAdminMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Equal(t, int64(1), resp.Data.Impact.AssociatedUserCount)
	require.Equal(t, int64(1), resp.Data.Impact.AssociatedTokenCount)
	require.Equal(t, int64(1), resp.Data.Impact.EnabledTokenCount)
}

func TestAdminQiniuChildAccountRetryRemoteFailureResponse(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	fullSK := "sk-" + strings.Repeat("c", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/user/disable-child", r.URL.Path)
		writeQiniuAdminJSON(t, w, map[string]any{
			"status": false,
			"error":  "remote denied " + fullSK,
		})
	}))
	defer server.Close()
	setting := operation_setting.GetQiniuKeySetting()
	setting.Enabled = true
	setting.BaseURL = server.URL
	setting.ChildAccountEmailDomain = "uk72.cn"
	setting.ChildAccountEmailPrefix = "child"
	setting.ChildAccountRequestTimeout = 5
	setting.ChildAccountRetryIntervalSeconds = 1

	account := &model.QiniuChildAccount{
		SequenceNo:    2,
		Email:         "child2@uk72.cn",
		UID:           "remote-uid",
		Status:        model.QiniuChildAccountStatusEnabled,
		LoginPassword: "login-password",
	}
	require.NoError(t, model.DB.Create(account).Error)
	task := &model.QiniuChildAccountSyncTask{
		AccountId: account.Id,
		TaskType:  model.QiniuChildAccountTaskTypeDisable,
		Status:    model.QiniuChildAccountTaskStatusFailed,
		Payload:   `{"reason":"risk control"}`,
	}
	require.NoError(t, model.DB.Create(task).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	recorder := performQiniuAdminRawRequest(t, router, http.MethodPost, "/api/payment/admin/qiniu-child-account-tasks/"+strconv.Itoa(task.Id)+"/retry", "", cookies, "8601")
	require.Equal(t, http.StatusOK, recorder.Code)
	var resp qiniuOfficialRetryResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.NotContains(t, resp.Message, fullSK)

	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, reloaded.Status)
	reloadedTask, err := model.GetQiniuChildAccountSyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountTaskStatusFailed, reloadedTask.Status)
	require.NotContains(t, reloadedTask.LastError, fullSK)
}
