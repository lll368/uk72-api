package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func configureQiniuKeySettingForTest(t *testing.T, baseURL string) {
	t.Helper()
	setting := operation_setting.GetQiniuKeySetting()
	old := *setting
	oldEnabledBaseURL := qiniuAPIKeyEnabledBaseURL
	setting.Enabled = true
	setting.BaseURL = baseURL
	setting.ChildAccountBaseURL = baseURL
	qiniuAPIKeyEnabledBaseURL = baseURL
	setting.AccessKey = "ak"
	setting.SecretKey = "sk"
	setting.RequestTimeout = 5
	setting.RetryIntervalSeconds = 1
	t.Cleanup(func() {
		*setting = old
		qiniuAPIKeyEnabledBaseURL = oldEnabledBaseURL
	})
}

func configureQiniuChildBindingForTest(t *testing.T) {
	t.Helper()
	setting := operation_setting.GetQiniuKeySetting()
	setting.ChildAccountBindingEnabled = true
	setting.ChildAccountAssignmentMode = operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild
	setting.ChildAccountBindingCutoverTime = common.GetTimestamp() - 1
}

func disableQiniuAsyncForTest(t *testing.T) {
	t.Helper()
	qiniuTaskAsyncDisabled.Store(true)
	t.Cleanup(func() {
		qiniuTaskAsyncDisabled.Store(false)
	})
}

func TestExecuteQiniuDefaultKeyTaskCreatesTokenWithBalanceInitialLimit(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("c", 64)
	fullKey := "sk-" + keyBody
	var created bool
	var targetLimit float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			created = true
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			var payload map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			totalQuota := payload["total_quota"].(map[string]any)
			require.Equal(t, true, totalQuota["enabled"])
			targetLimit = totalQuota["limit"].(float64)
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3001, int(25*common.QuotaPerUnit))
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeDefaultCreate,
		UserId:   3001,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))

	token, err := model.GetFirstUserToken(3001)
	require.NoError(t, err)
	require.Equal(t, keyBody, token.Key)
	require.Equal(t, model.TokenProviderQiniu, token.Provider)
	require.True(t, token.UnlimitedQuota)
	require.Equal(t, 0, token.RemainQuota)
	require.True(t, created)
	require.Equal(t, 25.0, targetLimit)
	var baseline model.QiniuQuotaGrant
	require.NoError(t, model.DB.First(&baseline, "business_key = ?", qiniuInitialQuotaBaselineBusinessKey(token.Id)).Error)
	require.Equal(t, token.Id, baseline.TokenId)
	require.Equal(t, 25.0, baseline.GrantAmount)
	require.Equal(t, model.QiniuQuotaGrantStatusApplied, baseline.RemoteApplyStatus)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusSuccess, reloadedTask.Status)
	require.Equal(t, token.Id, reloadedTask.TokenId)
}

func TestGetUserQiniuKeyStatusEnqueuesDefaultCreateTaskWhenMissing(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)
	configureQiniuKeySettingForTest(t, "http://127.0.0.1")

	seedUser(t, 3301, 0)

	status, err := GetUserQiniuKeyStatus(3301)
	require.NoError(t, err)
	require.True(t, status.Enabled)
	require.False(t, status.HasKey)
	require.Equal(t, model.QiniuKeyTaskStatusPending, status.TaskStatus)
	require.False(t, status.CanCreateToken)

	tasks, err := model.ListUserQiniuKeySyncTasks(3301, model.QiniuKeyTaskTypeDefaultCreate, "", 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	status, err = GetUserQiniuKeyStatus(3301)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusPending, status.TaskStatus)

	tasks, err = model.ListUserQiniuKeySyncTasks(3301, model.QiniuKeyTaskTypeDefaultCreate, "", 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
}

func TestEnqueueDefaultQiniuKeyCreateTaskUsesUserSpecificName(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)
	configureQiniuKeySettingForTest(t, "http://127.0.0.1")

	seedUser(t, 3303, 0)
	require.NoError(t, EnqueueDefaultQiniuKeyCreateTask(3303, "ignored"))

	tasks, err := model.ListUserQiniuKeySyncTasks(3303, model.QiniuKeyTaskTypeDefaultCreate, "", 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	var payload qiniuKeyCreateTaskPayload
	require.NoError(t, common.UnmarshalJsonStr(tasks[0].Payload, &payload))
	require.Equal(t, "dk-3303", payload.Name)
}

func TestExecuteQiniuDefaultKeyTaskUsesUserSpecificName(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("6", 64)
	fullKey := "sk-" + keyBody
	var remoteName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			var payload map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			names := payload["names"].([]any)
			remoteName = names[0].(string)
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3304, 0)
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeDefaultCreate,
		UserId:   3304,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))

	token, err := model.GetFirstUserToken(3304)
	require.NoError(t, err)
	require.Equal(t, "dk-3304", remoteName)
	require.Equal(t, "dk-3304", token.Name)
}

func TestExecuteQiniuDefaultKeyTaskRewritesLegacyDefaultName(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("5", 64)
	fullKey := "sk-" + keyBody
	var remoteName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			var payload map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			names := payload["names"].([]any)
			remoteName = names[0].(string)
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3305, 0)
	payloadBytes, err := common.Marshal(qiniuKeyCreateTaskPayload{
		Name:             "默认 Key",
		ExpiredTime:      -1,
		Group:            qiniuDefaultTokenGroup,
		InitialLimitMode: qiniuKeyInitialLimitZero,
	})
	require.NoError(t, err)
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeDefaultCreate,
		UserId:   3305,
		Status:   model.QiniuKeyTaskStatusPending,
		Payload:  string(payloadBytes),
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))

	token, err := model.GetFirstUserToken(3305)
	require.NoError(t, err)
	require.Equal(t, "dk-3305", remoteName)
	require.Equal(t, "dk-3305", token.Name)
}

func TestExecuteQiniuDefaultKeyTaskUsesDefaultGroupWhenAutoGroupEnabled(t *testing.T) {
	truncate(t)

	oldDefaultUseAutoGroup := setting.DefaultUseAutoGroup
	setting.DefaultUseAutoGroup = true
	t.Cleanup(func() {
		setting.DefaultUseAutoGroup = oldDefaultUseAutoGroup
	})

	keyBody := strings.Repeat("7", 64)
	fullKey := "sk-" + keyBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3302, 0)
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeDefaultCreate,
		UserId:   3302,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))

	token, err := model.GetFirstUserToken(3302)
	require.NoError(t, err)
	require.Equal(t, "default", token.Group)
	require.False(t, token.CrossGroupRetry)
}

func TestExecuteQiniuDefaultKeyTaskFailureKeepsUserAndRecordsRetry(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("d", 64)
	fullKey := "sk-" + keyBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			http.Error(w, "quota failed", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3002, 0)
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeDefaultCreate,
		UserId:   3002,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.Error(t, ExecuteQiniuKeyTask(context.Background(), task.Id))

	_, err := model.GetUserById(3002, false)
	require.NoError(t, err)
	count, err := model.CountUserTokens(3002)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusFailed, reloadedTask.Status)
	require.Equal(t, 1, reloadedTask.RetryCount)
	require.Equal(t, keyBody, reloadedTask.QiniuKey)
	require.NotEmpty(t, reloadedTask.LastError)
	require.NotZero(t, reloadedTask.NextRetryTime)
}

func TestExecuteQiniuCreateKeyTaskPersistsRemoteKeyBeforeQuotaUpdate(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("9", 64)
	fullKey := "sk-" + keyBody
	observedTaskKey := make(chan string, 1)
	var task *model.QiniuKeySyncTask
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			reloadedTask, err := model.GetQiniuKeySyncTaskById(task.Id)
			if err != nil {
				observedTaskKey <- "load-error:" + err.Error()
			} else {
				observedTaskKey <- reloadedTask.QiniuKey
			}
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3006, 0)
	task = &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeDefaultCreate,
		UserId:   3006,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))
	require.Equal(t, keyBody, <-observedTaskKey)
}

func TestExecuteQiniuQuotaSyncSkipsLegacyTaskWithoutCallingQiniu(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("e", 64)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Fatalf("legacy qiniu quota sync must not call qiniu after grant migration: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3003, int(120*common.QuotaPerUnit))
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             3003,
		UserId:         3003,
		Name:           "qiniu-token",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeQuotaSync,
		UserId:   3003,
		TokenId:  3003,
		QiniuKey: keyBody,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))

	require.Equal(t, 0, requestCount)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusSkipped, reloadedTask.Status)
	require.Contains(t, reloadedTask.LastError, "quota grant")
}

func TestEnqueueQiniuQuotaSyncDefersWhenOnlyDisabledTokenExists(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)

	configureQiniuKeySettingForTest(t, "http://127.0.0.1")
	keyBody := strings.Repeat("1", 64)
	seedUser(t, 3201, int(100*common.QuotaPerUnit))
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             3201,
		UserId:         3201,
		Name:           "disabled-qiniu-token",
		Key:            keyBody,
		Status:         common.TokenStatusDisabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	require.NoError(t, EnqueueQiniuQuotaSyncTask(3201))
	tasks, err := model.ListUserQiniuKeySyncTasks(3201, model.QiniuKeyTaskTypeQuotaSync, "", 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Zero(t, tasks[0].TokenId)
	require.Empty(t, tasks[0].QiniuKey)
	require.Equal(t, model.QiniuKeyTaskStatusPending, tasks[0].Status)
}

func TestEnqueueQiniuQuotaSyncCreatesPendingTaskWithoutToken(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)

	configureQiniuKeySettingForTest(t, "http://127.0.0.1")
	seedUser(t, 3204, int(100*common.QuotaPerUnit))

	require.NoError(t, EnqueueQiniuQuotaSyncTask(3204))
	tasks, err := model.ListUserQiniuKeySyncTasks(3204, model.QiniuKeyTaskTypeQuotaSync, "", 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Zero(t, tasks[0].TokenId)
	require.Empty(t, tasks[0].QiniuKey)
	require.Equal(t, model.QiniuKeyTaskStatusPending, tasks[0].Status)
}

func TestExecuteQiniuQuotaSyncWithoutTokenSkipsLegacyTask(t *testing.T) {
	truncate(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Fatalf("qiniu should not be called while user has no token: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3205, int(100*common.QuotaPerUnit))
	createTask := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeDefaultCreate,
		UserId:   3205,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(createTask))
	syncTask := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeQuotaSync,
		UserId:   3205,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(syncTask))

	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), syncTask.Id))
	require.Equal(t, 0, requestCount)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(syncTask.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusSkipped, reloadedTask.Status)
	require.Zero(t, reloadedTask.NextRetryTime)
}

func TestExecuteQiniuQuotaSyncSkipsDisabledTokenWithoutCallingQiniu(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("2", 64)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		writeQiniuJSON(t, w, map[string]any{"status": true, "data": map[string]any{"total_fee": 9}})
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3202, int(100*common.QuotaPerUnit))
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             3202,
		UserId:         3202,
		Name:           "disabled-qiniu-token",
		Key:            keyBody,
		Status:         common.TokenStatusDisabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeQuotaSync,
		UserId:   3202,
		TokenId:  3202,
		QiniuKey: keyBody,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))

	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))
	require.Equal(t, 0, requestCount)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusSkipped, reloadedTask.Status)
}

func TestCreateQiniuManagedTokenCreatesPendingTaskWithoutCallingQiniu(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Fatalf("qiniu should not be called while enqueueing manual create task: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3101, int(20*common.QuotaPerUnit))
	require.NoError(t, CreateQiniuManagedToken(context.Background(), 3101, model.Token{
		Name:        "manual-key",
		ExpiredTime: -1,
	}))
	require.Equal(t, 0, requestCount)

	tasks, err := model.ListUserQiniuKeySyncTasks(3101, model.QiniuKeyTaskTypeManualCreate, model.QiniuKeyTaskStatusPending, 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	err = CreateQiniuManagedToken(context.Background(), 3101, model.Token{Name: "second-key", ExpiredTime: -1})
	require.Error(t, err)
}

func TestCreateQiniuManagedTokenRejectsNameLongerThanQiniuLimit(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)

	configureQiniuKeySettingForTest(t, "http://127.0.0.1")
	seedUser(t, 3106, int(20*common.QuotaPerUnit))

	err := CreateQiniuManagedToken(context.Background(), 3106, model.Token{
		Name:        strings.Repeat("a", qiniuKeyNameMaxRunes+1),
		ExpiredTime: -1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "20")

	tasks, err := model.ListUserQiniuKeySyncTasks(3106, model.QiniuKeyTaskTypeManualCreate, "", 10)
	require.NoError(t, err)
	require.Empty(t, tasks)
}

func TestExecuteQiniuManualCreateTaskUsesBalanceInitialLimit(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)

	keyBody := strings.Repeat("f", 64)
	fullKey := "sk-" + keyBody
	var targetLimit float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			var payload map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			targetLimit = payload["total_quota"].(map[string]any)["limit"].(float64)
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3102, int(40*common.QuotaPerUnit))
	require.NoError(t, CreateQiniuManagedToken(context.Background(), 3102, model.Token{Name: "manual-key", ExpiredTime: -1}))
	tasks, err := model.ListUserQiniuKeySyncTasks(3102, model.QiniuKeyTaskTypeManualCreate, model.QiniuKeyTaskStatusPending, 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), tasks[0].Id))

	token, err := model.GetFirstUserToken(3102)
	require.NoError(t, err)
	require.Equal(t, keyBody, token.Key)
	require.Equal(t, model.TokenProviderQiniu, token.Provider)
	require.Equal(t, "manual-key", token.Name)
	require.Equal(t, 40.0, targetLimit)
	var baseline model.QiniuQuotaGrant
	require.NoError(t, model.DB.First(&baseline, "business_key = ?", qiniuInitialQuotaBaselineBusinessKey(token.Id)).Error)
	require.Equal(t, token.Id, baseline.TokenId)
	require.Equal(t, 40.0, baseline.GrantAmount)
	require.Equal(t, model.QiniuQuotaGrantStatusApplied, baseline.RemoteApplyStatus)
}

func TestExecuteQiniuCreateKeyTaskUsesResolvedChildAccountIdentity(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)

	keyBody := strings.Repeat("b", 64)
	fullKey := "sk-" + keyBody
	var createAuth string
	var quotaAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			createAuth = r.Header.Get("Authorization")
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			quotaAuth = r.Header.Get("Authorization")
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)
	childAccountServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("creating qiniu managed key must use key base URL, got child account request %s %s", r.Method, r.URL.String())
	}))
	defer childAccountServer.Close()
	operation_setting.GetQiniuKeySetting().ChildAccountBaseURL = childAccountServer.URL
	configureQiniuChildBindingForTest(t)

	seedUser(t, 3140, int(30*common.QuotaPerUnit))
	account := seedQiniuIdentityClientAccount(t, 804, model.QiniuChildAccountStatusEnabled, "child-create-ak", "child-create-sk")
	require.NoError(t, CreateQiniuManagedToken(context.Background(), 3140, model.Token{Name: "child-key", ExpiredTime: -1}))
	tasks, err := model.ListUserQiniuKeySyncTasks(3140, model.QiniuKeyTaskTypeManualCreate, model.QiniuKeyTaskStatusPending, 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), tasks[0].Id))

	require.True(t, strings.HasPrefix(createAuth, "Qiniu child-create-ak:"))
	require.True(t, strings.HasPrefix(quotaAuth, "Qiniu child-create-ak:"))
	token, err := model.GetFirstUserToken(3140)
	require.NoError(t, err)
	require.Equal(t, account.Id, token.QiniuChildAccountId)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(tasks[0].Id)
	require.NoError(t, err)
	var payload qiniuKeyCreateTaskPayload
	require.NoError(t, common.UnmarshalJsonStr(reloadedTask.Payload, &payload))
	require.Equal(t, account.Id, payload.QiniuChildAccountId)
}

func TestExecuteQiniuCreateKeyTaskDoesNotPersistTokenChildAccountBeforeQuotaSuccess(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("0", 64)
	fullKey := "sk-" + keyBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			http.Error(w, "quota failed", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)
	configureQiniuChildBindingForTest(t)

	seedUser(t, 3141, int(30*common.QuotaPerUnit))
	account := seedQiniuIdentityClientAccount(t, 805, model.QiniuChildAccountStatusEnabled, "child-fail-ak", "child-fail-sk")
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeDefaultCreate,
		UserId:   3141,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.Error(t, ExecuteQiniuKeyTask(context.Background(), task.Id))

	count, err := model.CountUserTokens(3141)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
	user, err := model.GetUserById(3141, false)
	require.NoError(t, err)
	require.Equal(t, account.Id, user.QiniuChildAccountId)
}

func TestExecuteQiniuDefaultKeyTaskRetriesWhenChildAccountAllocationBlocked(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)
	disableQiniuChildAccountAsyncForTest(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Fatalf("qiniu key API must not be called while child account allocation is blocked: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)
	configureQiniuChildBindingForTest(t)

	seedUser(t, 3142, int(30*common.QuotaPerUnit))
	require.NoError(t, EnqueueDefaultQiniuKeyCreateTask(3142, "ignored"))
	tasks, err := model.ListUserQiniuKeySyncTasks(3142, model.QiniuKeyTaskTypeDefaultCreate, model.QiniuKeyTaskStatusPending, 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	require.Error(t, ExecuteQiniuKeyTask(context.Background(), tasks[0].Id))

	require.Equal(t, 0, requestCount)
	count, err := model.CountUserTokens(3142)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(tasks[0].Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusFailed, reloadedTask.Status)
	require.NotZero(t, reloadedTask.NextRetryTime)
	var payload qiniuKeyCreateTaskPayload
	require.NoError(t, common.UnmarshalJsonStr(reloadedTask.Payload, &payload))
	require.NotZero(t, payload.QiniuChildAccountId)
	childTasks, err := model.ListQiniuChildAccountSyncTasks(model.QiniuChildAccountTaskQuery{TaskType: model.QiniuChildAccountTaskTypeCreate}, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, childTasks.Items, 1)
}

func TestExecuteQiniuCreateKeyTaskKeepsExistingRemoteKeyOnParentAccount(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("1", 64)
	fullKey := "sk-" + keyBody
	var requestCount int
	var quotaAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			t.Fatalf("existing remote key retry must not create another key")
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			quotaAuth = r.Header.Get("Authorization")
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)
	configureQiniuChildBindingForTest(t)

	seedUser(t, 3143, int(30*common.QuotaPerUnit))
	_ = seedQiniuIdentityClientAccount(t, 806, model.QiniuChildAccountStatusEnabled, "child-existing-ak", "child-existing-sk")
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeDefaultCreate,
		UserId:   3143,
		QiniuKey: keyBody,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))

	require.Equal(t, 1, requestCount)
	require.True(t, strings.HasPrefix(quotaAuth, "Qiniu ak:"))
	token, err := model.GetFirstUserToken(3143)
	require.NoError(t, err)
	require.Equal(t, keyBody, token.Key)
	require.Zero(t, token.QiniuChildAccountId)
	user, err := model.GetUserById(3143, false)
	require.NoError(t, err)
	require.Zero(t, user.QiniuChildAccountId)
}

func TestExecuteQiniuCreateKeyTaskClampsNegativeBalanceInitialLimitToZero(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("4", 64)
	fullKey := "sk-" + keyBody
	var targetLimit float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"keys": []map[string]any{{"key": fullKey}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			var payload map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			targetLimit = payload["total_quota"].(map[string]any)["limit"].(float64)
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3107, int(-5*common.QuotaPerUnit))
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeDefaultCreate,
		UserId:   3107,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))

	token, err := model.GetFirstUserToken(3107)
	require.NoError(t, err)
	require.Equal(t, 0.0, targetLimit)
	var baseline model.QiniuQuotaGrant
	require.NoError(t, model.DB.First(&baseline, "business_key = ?", qiniuInitialQuotaBaselineBusinessKey(token.Id)).Error)
	require.Equal(t, 0.0, baseline.GrantAmount)
	require.Equal(t, model.QiniuQuotaGrantStatusApplied, baseline.RemoteApplyStatus)
}

func TestCreateQiniuManagedTokenBlocksWhenRevokeTaskUnfinished(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Fatalf("qiniu must not be called while revoke task blocks replacement: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3108, int(20*common.QuotaPerUnit))
	require.NoError(t, model.CreateQiniuKeySyncTask(&model.QiniuKeySyncTask{
		TaskType:  model.QiniuKeyTaskTypeRevoke,
		UserId:    3108,
		TokenId:   9008,
		QiniuKey:  strings.Repeat("2", 64),
		Status:    model.QiniuKeyTaskStatusFailed,
		LastError: "Key 接口返回异常状态 500: remote failed",
	}))

	err := CreateQiniuManagedToken(context.Background(), 3108, model.Token{Name: "replacement-key", ExpiredTime: -1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "远端禁用任务")
	require.Equal(t, 0, requestCount)
	tasks, err := model.ListUserQiniuKeySyncTasks(3108, model.QiniuKeyTaskTypeManualCreate, "", 10)
	require.NoError(t, err)
	require.Empty(t, tasks)
}

func TestDeleteTokenEnqueuesRevokeBeforeReplacementCanBeCreated(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Fatalf("qiniu must not be called while revoke task blocks replacement: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	keyBody := strings.Repeat("9", 64)
	seedUser(t, 3111, int(20*common.QuotaPerUnit))
	require.NoError(t, model.DB.Create(&model.Token{
		UserId:         3111,
		Name:           "managed-key",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          qiniuDefaultTokenGroup,
	}).Error)
	token, err := model.GetFirstUserToken(3111)
	require.NoError(t, err)

	deletedToken, err := DeleteTokenAndEnqueueQiniuRevoke(3111, token.Id)
	require.NoError(t, err)
	require.Equal(t, token.Id, deletedToken.Id)

	tasks, err := model.ListUserQiniuKeySyncTasks(3111, model.QiniuKeyTaskTypeRevoke, "", 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, model.QiniuKeyTaskStatusPending, tasks[0].Status)
	require.Equal(t, token.Id, tasks[0].TokenId)
	require.Equal(t, keyBody, tasks[0].QiniuKey)

	err = CreateQiniuManagedToken(context.Background(), 3111, model.Token{Name: "replacement-key", ExpiredTime: -1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "远端禁用任务")
	require.Equal(t, 0, requestCount)
}

func TestGetUserQiniuKeyStatusReportsRevokeBlocked(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)
	configureQiniuKeySettingForTest(t, "http://127.0.0.1")

	seedUser(t, 3109, int(20*common.QuotaPerUnit))
	require.NoError(t, model.CreateQiniuKeySyncTask(&model.QiniuKeySyncTask{
		TaskType:  model.QiniuKeyTaskTypeRevoke,
		UserId:    3109,
		TokenId:   9009,
		QiniuKey:  strings.Repeat("3", 64),
		Status:    model.QiniuKeyTaskStatusFailed,
		LastError: "Key 接口返回异常状态 500: remote failed sk-" + strings.Repeat("3", 64),
	}))

	status, err := GetUserQiniuKeyStatus(3109)
	require.NoError(t, err)
	require.True(t, status.Enabled)
	require.False(t, status.HasKey)
	require.False(t, status.CanCreateToken)
	require.True(t, status.RevokeBlocked)
	require.Equal(t, model.QiniuKeyTaskTypeRevoke, status.BlockingTaskType)
	require.Equal(t, model.QiniuKeyTaskStatusFailed, status.BlockingTaskStatus)
	require.Equal(t, model.QiniuKeyTaskStatusFailed, status.TaskStatus)
	require.True(t, status.TaskRetryable)
	require.Equal(t, "API Key 远端禁用失败，请等待重试或联系管理员", status.LastError)
	require.NotContains(t, status.LastError, "sk-"+strings.Repeat("3", 64))
	require.NotContains(t, status.LastError, "remote failed")
	require.NotContains(t, status.LastError, "Key 接口返回异常状态")

	createTasks, err := model.ListUserQiniuKeySyncTasks(3109, model.QiniuKeyTaskTypeDefaultCreate, "", 10)
	require.NoError(t, err)
	require.Empty(t, createTasks)
}

func TestHasBlockingQiniuRevokeTaskStatuses(t *testing.T) {
	truncate(t)

	blocked, err := model.HasBlockingQiniuRevokeTaskWithTx(nil, 3110)
	require.NoError(t, err)
	require.False(t, blocked)

	for index, status := range []string{model.QiniuKeyTaskStatusSuccess, model.QiniuKeyTaskStatusSkipped} {
		require.NoError(t, model.CreateQiniuKeySyncTask(&model.QiniuKeySyncTask{
			TaskType: model.QiniuKeyTaskTypeRevoke,
			UserId:   3110 + index,
			QiniuKey: strings.Repeat("4", 64),
			Status:   status,
		}))
		blocked, err = model.HasBlockingQiniuRevokeTaskWithTx(nil, 3110+index)
		require.NoError(t, err)
		require.False(t, blocked)
	}

	for index, status := range []string{model.QiniuKeyTaskStatusPending, model.QiniuKeyTaskStatusRunning, model.QiniuKeyTaskStatusFailed} {
		userId := 3120 + index
		require.NoError(t, model.CreateQiniuKeySyncTask(&model.QiniuKeySyncTask{
			TaskType: model.QiniuKeyTaskTypeRevoke,
			UserId:   userId,
			QiniuKey: strings.Repeat("5", 64),
			Status:   status,
		}))
		blocked, err = model.HasBlockingQiniuRevokeTaskWithTx(nil, userId)
		require.NoError(t, err)
		require.True(t, blocked)
	}
}

func TestExecuteQiniuRevokeTaskSetsTotalLimitToZeroWithoutUsageQuery(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("a", 64)
	fullKey := "sk-" + keyBody
	var targetLimit float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/stat/usage/apikey/cost-detail":
			t.Fatalf("revoke must not query per-key used amount: %s", r.URL.String())
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			var payload map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &payload))
			targetLimit = payload["total_quota"].(map[string]any)["limit"].(float64)
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3103, 0)
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeRevoke,
		UserId:   3103,
		TokenId:  3103,
		QiniuKey: keyBody,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))
	require.Equal(t, 0.0, targetLimit)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusSuccess, reloadedTask.Status)
}

func TestExecuteQiniuRevokeTaskUsesDeletedTokenChildAccountIdentityAndCleanupResult(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)

	keyBody := strings.Repeat("c", 64)
	fullKey := "sk-" + keyBody
	var observedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/apikey/quota/"+fullKey {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		observedAuth = r.Header.Get("Authorization")
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	account := seedQiniuIdentityClientAccount(t, 812, model.QiniuChildAccountStatusEnabled, "child-revoke-ak", "child-revoke-sk")
	seedUser(t, 3133, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:                  3133,
		UserId:              3133,
		Name:                "child-token",
		Key:                 keyBody,
		Provider:            model.TokenProviderQiniu,
		QiniuChildAccountId: account.Id,
		Status:              common.TokenStatusEnabled,
		CreatedTime:         common.GetTimestamp(),
		AccessedTime:        common.GetTimestamp(),
		ExpiredTime:         -1,
		UnlimitedQuota:      true,
	}).Error)

	_, err := DeleteTokenAndEnqueueQiniuRevoke(3133, 3133)
	require.NoError(t, err)
	tasks, err := model.ListUserQiniuKeySyncTasks(3133, model.QiniuKeyTaskTypeRevoke, model.QiniuKeyTaskStatusPending, 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), tasks[0].Id))

	require.True(t, strings.HasPrefix(observedAuth, "Qiniu child-revoke-ak:"))
	reloadedTask, err := model.GetQiniuKeySyncTaskById(tasks[0].Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusSuccess, reloadedTask.Status)
	require.Equal(t, model.QiniuRemoteCleanupResultSuccess, reloadedTask.RemoteCleanupResult)
}

func TestExecuteQiniuRevokeTaskFallsBackToDailyZeroWhenTotalBelowUsed(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("6", 64)
	fullKey := "sk-" + keyBody
	var totalZeroSeen bool
	var dailyZeroSeen bool
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/apikey/quota/"+fullKey {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		callCount++
		var payload map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &payload))
		switch callCount {
		case 1:
			totalQuota := payload["total_quota"].(map[string]any)
			totalZeroSeen = totalQuota["enabled"] == true && totalQuota["limit"] == float64(0)
			writeQiniuJSON(t, w, map[string]any{
				"status": false,
				"error":  "total quota cannot be lower than used amount",
			})
		case 2:
			dailyQuota := payload["daily_quota"].(map[string]any)
			dailyZeroSeen = dailyQuota["enabled"] == true && dailyQuota["limit"] == float64(0)
			require.NotContains(t, payload, "total_quota")
			require.NotContains(t, payload, "monthly_quota")
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected extra qiniu revoke request #%d", callCount)
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3130, 0)
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeRevoke,
		UserId:   3130,
		TokenId:  3130,
		QiniuKey: keyBody,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), task.Id))
	require.Equal(t, 2, callCount)
	require.True(t, totalZeroSeen)
	require.True(t, dailyZeroSeen)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusSuccess, reloadedTask.Status)
}

func TestExecuteQiniuRevokeTaskKeepsFailedWhenDailyFallbackFails(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("7", 64)
	fullKey := "sk-" + keyBody
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/apikey/quota/"+fullKey {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		callCount++
		if callCount == 1 {
			writeQiniuJSON(t, w, map[string]any{
				"status": false,
				"error":  "total quota cannot be less than used amount",
			})
			return
		}
		http.Error(w, "daily quota failed", http.StatusInternalServerError)
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3131, 0)
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeRevoke,
		UserId:   3131,
		TokenId:  3131,
		QiniuKey: keyBody,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.Error(t, ExecuteQiniuKeyTask(context.Background(), task.Id))
	require.Equal(t, 2, callCount)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusFailed, reloadedTask.Status)
	require.Equal(t, 1, reloadedTask.RetryCount)
	require.Contains(t, reloadedTask.LastError, "daily quota failed")
}

func TestExecuteQiniuRevokeTaskDoesNotFallbackForTransientFailure(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("8", 64)
	fullKey := "sk-" + keyBody
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/apikey/quota/"+fullKey {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		callCount++
		http.Error(w, "temporary outage", http.StatusInternalServerError)
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3132, 0)
	task := &model.QiniuKeySyncTask{
		TaskType: model.QiniuKeyTaskTypeRevoke,
		UserId:   3132,
		TokenId:  3132,
		QiniuKey: keyBody,
		Status:   model.QiniuKeyTaskStatusPending,
	}
	require.NoError(t, model.CreateQiniuKeySyncTask(task))
	require.Error(t, ExecuteQiniuKeyTask(context.Background(), task.Id))
	require.Equal(t, 1, callCount)
	reloadedTask, err := model.GetQiniuKeySyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuKeyTaskStatusFailed, reloadedTask.Status)
}

func TestRetryDueQiniuKeyTasksIncludesPendingFailedAndStaleRunning(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("8", 64)
	fullKey := "sk-" + keyBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/stat/usage/apikey/cost-detail":
			t.Fatalf("retry revoke must not query per-key used amount: %s", r.URL.String())
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3104, 0)
	now := common.GetTimestamp()
	for _, task := range []*model.QiniuKeySyncTask{
		{TaskType: model.QiniuKeyTaskTypeRevoke, UserId: 3104, QiniuKey: keyBody, Status: model.QiniuKeyTaskStatusPending},
		{TaskType: model.QiniuKeyTaskTypeRevoke, UserId: 3104, QiniuKey: "not-qiniu", Status: model.QiniuKeyTaskStatusPending},
		{TaskType: model.QiniuKeyTaskTypeRevoke, UserId: 3104, QiniuKey: "not-qiniu", Status: model.QiniuKeyTaskStatusFailed, NextRetryTime: now - 1},
		{TaskType: model.QiniuKeyTaskTypeRevoke, UserId: 3104, QiniuKey: "not-qiniu", Status: model.QiniuKeyTaskStatusRunning, StartedTime: now - 3600},
	} {
		require.NoError(t, model.CreateQiniuKeySyncTask(task))
	}

	result, err := RetryDueQiniuKeyTasks(10)
	require.NoError(t, err)
	require.Empty(t, result.Errors)
	require.Equal(t, 4, result.ProcessedCount)
	require.Equal(t, 1, result.SuccessCount)
	require.Equal(t, 3, result.SkippedCount)
}

func TestRevokeLegacyQiniuKeysOnceDeletesLocalAndQueuesRemoteRevoke(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)

	keyBody := strings.Repeat("b", 64)
	fullKey := "sk-" + keyBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/stat/usage/apikey/cost-detail":
			t.Fatalf("legacy revoke must not query per-key used amount: %s", r.URL.String())
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)
	seedUser(t, 3105, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:           3105,
		UserId:       3105,
		Name:         "legacy-qiniu",
		Key:          keyBody,
		Status:       common.TokenStatusEnabled,
		CreatedTime:  common.GetTimestamp() - 10,
		AccessedTime: common.GetTimestamp() - 10,
		ExpiredTime:  -1,
	}).Error)

	require.NoError(t, RevokeLegacyQiniuKeysOnce())
	_, err := model.GetFirstUserToken(3105)
	require.Error(t, err)
	tasks, err := model.ListUserQiniuKeySyncTasks(3105, model.QiniuKeyTaskTypeRevoke, "", 10)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, keyBody, tasks[0].QiniuKey)
}

func TestAdminDisableQiniuKeyDisablesRemoteThenLocalToken(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("d", 64)
	fullKey := "sk-" + keyBody
	var observed map[string]any
	requestCount := 0
	lookupCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/ai/inapi/v2/apikey/enabled":
			require.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ak:"), "admin disable must keep Qiniu signed authorization")
			require.False(t, strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "), "admin disable must not use copied OEM Bearer client")
			require.NoError(t, common.DecodeJson(r.Body, &observed))
			writeQiniuJSON(t, w, map[string]any{"status": true})
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/apikey/quota/"):
			t.Fatalf("admin disable must not call quota revoke path: %s %s", r.Method, r.URL.String())
		default:
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	oldLookup := getAdminDisableQiniuTokenById
	getAdminDisableQiniuTokenById = func(tokenId int) (*model.Token, error) {
		lookupCount++
		return getAdminDisableQiniuTokenByIdFromDB(tokenId)
	}
	t.Cleanup(func() {
		getAdminDisableQiniuTokenById = oldLookup
	})

	seedUser(t, 3401, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             3402,
		UserId:         3401,
		Name:           "admin-disable-qiniu",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	token, err := AdminDisableQiniuKey(context.Background(), 3402, 8601, "manual risk control")
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, token.Status)
	require.Equal(t, 1, requestCount)
	require.Equal(t, 1, lookupCount)
	require.Equal(t, fullKey, observed["key"])
	require.Equal(t, false, observed["enabled"])

	reloaded, err := model.GetTokenById(3402)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, reloaded.Status)

	tasks, err := model.ListUserQiniuKeySyncTasks(3401, model.QiniuKeyTaskTypeRevoke, "", 10)
	require.NoError(t, err)
	require.Empty(t, tasks)

	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", 3401, model.LogTypeManage).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Contains(t, logs[0].Content, "禁用七牛")
	require.NotContains(t, logs[0].Other, fullKey)
	require.NotContains(t, logs[0].Other, keyBody)
	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, float64(8601), adminInfo["admin_user_id"])
	require.Equal(t, float64(3402), adminInfo["token_id"])
	require.Equal(t, model.QiniuTokenKeyFingerprint(keyBody), adminInfo["key_fingerprint"])
	require.Equal(t, "manual risk control", adminInfo["reason"])
}

func TestGetAdminDisableQiniuTokenByIdFromDBDoesNotRequireRedis(t *testing.T) {
	truncate(t)

	oldRedisEnabled := common.RedisEnabled
	oldRedisClient := common.RDB
	common.RedisEnabled = true
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRedisClient
	})

	seedUser(t, 3413, 0)
	keyBody := strings.Repeat("8", 64)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             3414,
		UserId:         3413,
		Name:           "admin-disable-direct-db-read",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	token, err := getAdminDisableQiniuTokenByIdFromDB(3414)
	require.NoError(t, err)
	require.Equal(t, 3414, token.Id)
	require.Equal(t, keyBody, token.Key)
}

func TestAdminDisableQiniuKeyCacheInvalidationFailureStillReturnsSuccessAndAudits(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("7", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/ai/inapi/v2/apikey/enabled" {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		writeQiniuJSON(t, w, map[string]any{"status": true})
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	oldInvalidate := invalidateAdminDisabledQiniuTokenCache
	invalidateAdminDisabledQiniuTokenCache = func(userId int) error {
		require.Equal(t, 3411, userId)
		return errors.New("redis unavailable")
	}
	t.Cleanup(func() {
		invalidateAdminDisabledQiniuTokenCache = oldInvalidate
	})

	seedUser(t, 3411, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             3412,
		UserId:         3411,
		Name:           "admin-disable-cache-failure",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	token, err := AdminDisableQiniuKey(context.Background(), 3412, 8601, "cache failure")
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, token.Status)

	reloaded, err := model.GetTokenById(3412)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, reloaded.Status)

	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", 3411, model.LogTypeManage).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Contains(t, logs[0].Content, "禁用七牛")
	require.NotContains(t, logs[0].Other, keyBody)
}

func TestAdminDisableQiniuKeyRemoteFailureKeepsLocalTokenEnabledAndNoRevokeTask(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("e", 64)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodPut || r.URL.Path != "/ai/inapi/v2/apikey/enabled" {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		writeQiniuJSON(t, w, map[string]any{
			"status": false,
			"code":   "remote_failed",
			"error":  "remote disable failed",
		})
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3403, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             3404,
		UserId:         3403,
		Name:           "admin-disable-failure",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	_, err := AdminDisableQiniuKey(context.Background(), 3404, 8601, "remote failure")
	require.Error(t, err)
	require.Equal(t, 1, requestCount)

	reloaded, err := model.GetTokenById(3404)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusEnabled, reloaded.Status)

	tasks, err := model.ListUserQiniuKeySyncTasks(3403, model.QiniuKeyTaskTypeRevoke, "", 10)
	require.NoError(t, err)
	require.Empty(t, tasks)
	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", 3403, model.LogTypeManage).Find(&logs).Error)
	require.Empty(t, logs)
}

func TestAdminDisableQiniuKeyAlreadyDisabledRemoteResponseIsSuccess(t *testing.T) {
	truncate(t)

	keyBody := strings.Repeat("f", 64)
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
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3405, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             3406,
		UserId:         3405,
		Name:           "admin-disable-idempotent",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	token, err := AdminDisableQiniuKey(context.Background(), 3406, 8601, "already disabled remotely")
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, token.Status)

	reloaded, err := model.GetTokenById(3406)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, reloaded.Status)
}

func TestAdminDisableQiniuKeyRejectsInvalidLocalTokensWithoutRemoteCall(t *testing.T) {
	truncate(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Fatalf("invalid local token must not call qiniu: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	configureQiniuKeySettingForTest(t, server.URL)

	seedUser(t, 3407, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             3408,
		UserId:         3407,
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
		Id:             3409,
		UserId:         3407,
		Name:           "disabled-qiniu-token",
		Key:            strings.Repeat("a", 64),
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusDisabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             3410,
		UserId:         3407,
		Name:           "deleted-qiniu-token",
		Key:            strings.Repeat("b", 64),
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	var deleted model.Token
	require.NoError(t, model.DB.First(&deleted, "id = ?", 3410).Error)
	require.NoError(t, model.DB.Delete(&deleted).Error)

	for _, tokenId := range []int{3408, 3409, 3410, 0, 9999} {
		_, err := AdminDisableQiniuKey(context.Background(), tokenId, 8601, "")
		require.Error(t, err, "token_id=%d should be rejected", tokenId)
	}
	require.Equal(t, 0, requestCount)
}

func TestQiniuTaskLockRemainsReusableAfterUnlock(t *testing.T) {
	lockKey := "qiniu-test-lock"
	qiniuTaskLocks.Delete(lockKey)
	t.Cleanup(func() {
		qiniuTaskLocks.Delete(lockKey)
	})

	unlock, ok := tryAcquireQiniuTaskLock(lockKey)
	require.True(t, ok)
	unlock()

	_, stillTracked := qiniuTaskLocks.Load(lockKey)
	require.True(t, stillTracked)
	unlockAgain, ok := tryAcquireQiniuTaskLock(lockKey)
	require.True(t, ok)
	unlockAgain()
}
