package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func configureQiniuChildAccountSettingForTest(t *testing.T, baseURL string) {
	t.Helper()
	configureQiniuKeySettingForTest(t, baseURL)
	setting := operation_setting.GetQiniuKeySetting()
	setting.ChildAccountEmailDomain = "uk72.cn"
	setting.ChildAccountEmailPrefix = "child"
	setting.ChildAccountPasswordLength = 18
	setting.ChildAccountRequestTimeout = 5
	setting.ChildAccountRetryIntervalSeconds = 1
}

func TestQiniuChildAccountCreateTaskSuccess(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)

	ak := strings.Repeat("a", 40)
	sk := strings.Repeat("b", 40)
	var createdEmail string
	var fetchedUID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/create_child":
			require.NoError(t, r.ParseForm())
			createdEmail = r.Form.Get("email")
			require.Equal(t, "child1@uk72.cn", createdEmail)
			require.NotEmpty(t, r.Form.Get("password"))
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"userid":      "remote-userid",
					"uid":         "remote-uid",
					"parent_uid":  "parent-uid",
					"email":       createdEmail,
					"is_disabled": false,
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user/child_key":
			fetchedUID = r.URL.Query().Get("uid")
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"key":    ak,
					"secret": sk,
					"state":  "enabled",
				},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	account, task, err := CreateQiniuChildAccount(context.Background(), 8601)
	require.NoError(t, err)
	require.Equal(t, "child1@uk72.cn", account.Email)
	require.Equal(t, model.QiniuChildAccountStatusCreating, account.Status)
	require.Equal(t, model.QiniuChildAccountTaskTypeCreate, task.TaskType)

	require.NoError(t, ExecuteQiniuChildAccountTask(context.Background(), task.Id))
	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, reloaded.Status)
	require.Equal(t, "remote-uid", reloaded.UID)
	require.Equal(t, "remote-userid", reloaded.RemoteUserID)
	require.Equal(t, ak, reloaded.AccessKey)
	require.NotEmpty(t, reloaded.SecretKey)
	require.Equal(t, "remote-uid", fetchedUID)
	childClient, err := NewQiniuAccountIdentityClient(account.Id, QiniuAccountOperationCreate)
	require.NoError(t, err)
	require.Equal(t, ak, childClient.setting.AccessKey)
	require.Equal(t, sk, childClient.setting.SecretKey)

	reloadedTask, err := model.GetQiniuChildAccountSyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountTaskStatusSuccess, reloadedTask.Status)
}

func TestQiniuChildAccountRetryWithExistingUIDOnlyFetchesKey(t *testing.T) {
	truncate(t)
	configureQiniuChildAccountSettingForTest(t, "http://127.0.0.1")
	ak := strings.Repeat("c", 40)
	sk := strings.Repeat("d", 40)
	var createCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/create_child":
			createCalled = true
			t.Fatalf("retry with saved uid must not create another child account")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user/child_key":
			require.Equal(t, "saved-uid", r.URL.Query().Get("uid"))
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data":   map[string]any{"key": ak, "secret": sk, "state": "enabled"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	account, err := model.CreateQiniuChildAccountWithSequence(model.DB, "child", "uk72.cn", "protected-password")
	require.NoError(t, err)
	require.NoError(t, model.MarkQiniuChildAccountRemoteCreated(account.Id, "remote-userid", "saved-uid", "parent-uid", false))
	task := &model.QiniuChildAccountSyncTask{
		AccountId: account.Id,
		TaskType:  model.QiniuChildAccountTaskTypeCreate,
		Status:    model.QiniuChildAccountTaskStatusFailed,
	}
	require.NoError(t, model.CreateQiniuChildAccountSyncTask(task))

	require.NoError(t, ExecuteQiniuChildAccountTask(context.Background(), task.Id))
	require.False(t, createCalled)
	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, reloaded.Status)
}

func TestQiniuChildAccountCreateTaskFetchesKeyByEmailWhenCreateResponseOmitsUID(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)

	ak := strings.Repeat("m", 40)
	sk := strings.Repeat("n", 40)
	var fetchedEmail string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/create_child":
			writeQiniuJSON(t, w, map[string]any{"status": true, "data": map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user/child_key":
			fetchedEmail = r.URL.Query().Get("email")
			require.Equal(t, "child1@uk72.cn", fetchedEmail)
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data":   map[string]any{"key": ak, "secret": sk, "state": "enabled"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	account, task, err := CreateQiniuChildAccount(context.Background(), 8601)
	require.NoError(t, err)
	require.NoError(t, ExecuteQiniuChildAccountTask(context.Background(), task.Id))

	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, reloaded.Status)
	require.Empty(t, reloaded.UID)
	require.Empty(t, reloaded.LoginPassword)
	require.Equal(t, ak, reloaded.AccessKey)
	require.Equal(t, "child1@uk72.cn", fetchedEmail)
}

func TestQiniuChildAccountCreateTaskTreatsEmailExistsAsCreatedAndFetchesKey(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)

	ak := strings.Repeat("p", 40)
	sk := strings.Repeat("q", 40)
	var fetchedEmail string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/create_child":
			w.WriteHeader(http.StatusBadRequest)
			writeQiniuJSON(t, w, map[string]any{
				"error":             "email_exist",
				"error_code":        11303,
				"error_description": "email: child1@uk72.cn has alrealy exists!",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user/child_key":
			fetchedEmail = r.URL.Query().Get("email")
			require.Equal(t, "child1@uk72.cn", fetchedEmail)
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data":   map[string]any{"key": ak, "secret": sk, "state": "enabled"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	account, task, err := CreateQiniuChildAccount(context.Background(), 8601)
	require.NoError(t, err)
	require.NoError(t, ExecuteQiniuChildAccountTask(context.Background(), task.Id))

	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, reloaded.Status)
	require.Empty(t, reloaded.UID)
	require.Empty(t, reloaded.LoginPassword)
	require.Equal(t, ak, reloaded.AccessKey)
	require.Equal(t, "child1@uk72.cn", fetchedEmail)
}

func TestQiniuChildAccountCreateTaskFetchesKeyByEmailWhenLoginPasswordCleared(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)

	ak := strings.Repeat("u", 40)
	sk := strings.Repeat("v", 40)
	var createCalled bool
	var fetchedEmail string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/create_child":
			createCalled = true
			t.Fatalf("cleared login password must recover by child_key email lookup instead of creating again")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user/child_key":
			fetchedEmail = r.URL.Query().Get("email")
			require.Equal(t, "child1@uk72.cn", fetchedEmail)
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data":   map[string]any{"key": ak, "secret": sk, "state": "enabled"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	account, err := model.CreateQiniuChildAccountWithSequence(model.DB, "child", "uk72.cn", "protected-password")
	require.NoError(t, err)
	require.NoError(t, model.MarkQiniuChildAccountRemoteCreated(account.Id, "", "", "", false))
	task := &model.QiniuChildAccountSyncTask{
		AccountId: account.Id,
		TaskType:  model.QiniuChildAccountTaskTypeCreate,
		Status:    model.QiniuChildAccountTaskStatusFailed,
	}
	require.NoError(t, model.CreateQiniuChildAccountSyncTask(task))

	require.NoError(t, ExecuteQiniuChildAccountTask(context.Background(), task.Id))
	require.False(t, createCalled)
	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, reloaded.Status)
	require.Empty(t, reloaded.UID)
	require.Empty(t, reloaded.LoginPassword)
	require.Equal(t, ak, reloaded.AccessKey)
	require.Equal(t, "child1@uk72.cn", fetchedEmail)
}

func TestRetryDueQiniuChildAccountTasksProcessesFailedTask(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)

	ak := strings.Repeat("r", 40)
	sk := strings.Repeat("s", 40)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/create_child":
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"userid": "remote-userid",
					"uid":    "remote-uid",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user/child_key":
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data":   map[string]any{"key": ak, "secret": sk, "state": "enabled"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	account, err := model.CreateQiniuChildAccountWithSequence(model.DB, "child", "uk72.cn", "protected-password")
	require.NoError(t, err)
	task := &model.QiniuChildAccountSyncTask{
		AccountId:     account.Id,
		TaskType:      model.QiniuChildAccountTaskTypeCreate,
		Status:        model.QiniuChildAccountTaskStatusFailed,
		NextRetryTime: common.GetTimestamp() - 1,
		CompletedTime: common.GetTimestamp() - 2,
	}
	require.NoError(t, model.CreateQiniuChildAccountSyncTask(task))

	result, err := RetryDueQiniuChildAccountTasks(10)
	require.NoError(t, err)
	require.Equal(t, 1, result.ProcessedCount)
	require.Equal(t, 1, result.SuccessCount)
	require.Empty(t, result.Errors)

	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, reloaded.Status)
	require.Equal(t, ak, reloaded.AccessKey)
}

func TestQiniuChildAccountCreateFailureMarksAccountFailed(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)
	sensitiveKey := "sk-" + strings.Repeat("e", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/user/create_child", r.URL.Path)
		writeQiniuJSON(t, w, map[string]any{
			"status": false,
			"error":  "remote create denied " + sensitiveKey,
		})
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	account, task, err := CreateQiniuChildAccount(context.Background(), 8601)
	require.NoError(t, err)
	err = ExecuteQiniuChildAccountTask(context.Background(), task.Id)
	require.Error(t, err)

	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusFailed, reloaded.Status)
	require.Empty(t, reloaded.UID)
	require.NotContains(t, reloaded.LastError, sensitiveKey)

	reloadedTask, err := model.GetQiniuChildAccountSyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountTaskStatusFailed, reloadedTask.Status)
	require.NotContains(t, reloadedTask.LastError, sensitiveKey)
}

func TestQiniuChildAccountFetchKeyFailureCanRetryWithSavedUID(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)

	ak := strings.Repeat("f", 40)
	sk := strings.Repeat("g", 40)
	var createCalls int
	var keyCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/create_child":
			createCalls++
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data": map[string]any{
					"userid":     "remote-userid",
					"uid":        "remote-uid",
					"parent_uid": "parent-uid",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user/child_key":
			keyCalls++
			require.Equal(t, "remote-uid", r.URL.Query().Get("uid"))
			if keyCalls == 1 {
				writeQiniuJSON(t, w, map[string]any{
					"status": false,
					"error":  "key temporarily unavailable",
				})
				return
			}
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data":   map[string]any{"key": ak, "secret": sk, "state": "enabled"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	account, task, err := CreateQiniuChildAccount(context.Background(), 8601)
	require.NoError(t, err)
	require.Error(t, ExecuteQiniuChildAccountTask(context.Background(), task.Id))

	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, "remote-uid", reloaded.UID)
	require.Equal(t, model.QiniuChildAccountStatusFailed, reloaded.Status)
	require.Empty(t, reloaded.LoginPassword)
	require.Empty(t, reloaded.SecretKey)

	require.NoError(t, RetryQiniuChildAccountTaskById(task.Id))
	reloaded, err = model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, reloaded.Status)
	require.NotEmpty(t, reloaded.SecretKey)
	require.Equal(t, 1, createCalls)
	require.Equal(t, 2, keyCalls)
}

func TestQiniuChildAccountRejectsNonRetryableTask(t *testing.T) {
	truncate(t)
	configureQiniuChildAccountSettingForTest(t, "http://127.0.0.1")
	account, err := model.CreateQiniuChildAccountWithSequence(model.DB, "child", "uk72.cn", "protected-password")
	require.NoError(t, err)
	task := &model.QiniuChildAccountSyncTask{
		AccountId: account.Id,
		TaskType:  model.QiniuChildAccountTaskTypeCreate,
		Status:    model.QiniuChildAccountTaskStatusSuccess,
	}
	require.NoError(t, model.CreateQiniuChildAccountSyncTask(task))

	err = RetryQiniuChildAccountTaskById(task.Id)
	require.Error(t, err)
	require.Contains(t, err.Error(), "不可重试")
}

func TestQiniuChildAccountEnableDisableRemoteFirst(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)
	account, err := model.CreateQiniuChildAccountWithSequence(model.DB, "child", "uk72.cn", "protected-password")
	require.NoError(t, err)
	require.NoError(t, model.MarkQiniuChildAccountRemoteCreated(account.Id, "remote-userid", "remote-uid", "parent-uid", false))
	require.NoError(t, model.MarkQiniuChildAccountCredentials(account.Id, "ak", "sk", "enabled", "", "", ""))
	require.NoError(t, model.MarkQiniuChildAccountStatus(account.Id, model.QiniuChildAccountStatusEnabled, ""))
	seedUser(t, 3910, int(10*common.QuotaPerUnit))
	require.NoError(t, model.DB.Create(&model.Token{
		Id:                  3910,
		UserId:              3910,
		Name:                "child-bound-token",
		Key:                 strings.Repeat("a", 64),
		Provider:            model.TokenProviderQiniu,
		QiniuChildAccountId: account.Id,
		Status:              common.TokenStatusEnabled,
		CreatedTime:         common.GetTimestamp(),
		AccessedTime:        common.GetTimestamp(),
		ExpiredTime:         -1,
		UnlimitedQuota:      true,
	}).Error)

	var disableCalled bool
	var enableCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/disable-child":
			disableCalled = true
			writeQiniuJSON(t, w, map[string]any{"status": true})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/enable-child":
			enableCalled = true
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	_, disableTask, err := DisableQiniuChildAccount(context.Background(), account.Id, 8601, "ops")
	require.NoError(t, err)
	require.NoError(t, ExecuteQiniuChildAccountTask(context.Background(), disableTask.Id))
	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusDisabled, reloaded.Status)
	require.True(t, disableCalled)
	reloadedToken, err := model.GetTokenById(3910)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, reloadedToken.Status)

	_, enableTask, err := EnableQiniuChildAccount(context.Background(), account.Id, 8601)
	require.NoError(t, err)
	require.NoError(t, ExecuteQiniuChildAccountTask(context.Background(), enableTask.Id))
	reloaded, err = model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, reloaded.Status)
	require.True(t, enableCalled)
	reloadedToken, err = model.GetTokenById(3910)
	require.NoError(t, err)
	require.Equal(t, common.TokenStatusDisabled, reloadedToken.Status)
}

func TestQiniuChildAccountRemoteFailurePreservesLocalStatus(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)
	account, err := model.CreateQiniuChildAccountWithSequence(model.DB, "child", "uk72.cn", "protected-password")
	require.NoError(t, err)
	require.NoError(t, model.MarkQiniuChildAccountRemoteCreated(account.Id, "remote-userid", "remote-uid", "parent-uid", false))
	require.NoError(t, model.MarkQiniuChildAccountStatus(account.Id, model.QiniuChildAccountStatusEnabled, ""))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeQiniuJSON(t, w, map[string]any{"status": false, "error": "remote denied sk-" + strings.Repeat("e", 64)})
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	_, task, err := DisableQiniuChildAccount(context.Background(), account.Id, 8601, "ops")
	require.NoError(t, err)
	err = ExecuteQiniuChildAccountTask(context.Background(), task.Id)
	require.Error(t, err)
	reloaded, err := model.GetQiniuChildAccountById(account.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountStatusEnabled, reloaded.Status)
	reloadedTask, err := model.GetQiniuChildAccountSyncTaskById(task.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuChildAccountTaskStatusFailed, reloadedTask.Status)
	require.NotContains(t, reloadedTask.LastError, strings.Repeat("e", 64))
}

func TestQiniuChildAccountRejectsInvalidDomainBeforeRemoteCreate(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)
	configureQiniuChildAccountSettingForTest(t, "http://127.0.0.1")
	operation_setting.GetQiniuKeySetting().ChildAccountEmailDomain = "localhost"

	_, _, err := CreateQiniuChildAccount(context.Background(), 8601)
	require.Error(t, err)
}

func TestQiniuChildAccountRegistrationStillUsesDefaultQiniuKeyTask(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)
	disableQiniuChildAccountAsyncForTest(t)
	configureQiniuChildAccountSettingForTest(t, "http://127.0.0.1")
	seedUser(t, 3901, int(10*common.QuotaPerUnit))

	require.NoError(t, EnqueueDefaultQiniuKeyCreateTask(3901, "ignored"))
	keyTasks, err := model.ListUserQiniuKeySyncTasks(3901, model.QiniuKeyTaskTypeDefaultCreate, "", 10)
	require.NoError(t, err)
	require.Len(t, keyTasks, 1)

	childTasks, err := model.ListQiniuChildAccountSyncTasks(model.QiniuChildAccountTaskQuery{}, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(0), childTasks.Total)
}

func TestQiniuChildAccountManualTokenCreationStillUsesParentAPIKeyPath(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)
	disableQiniuChildAccountAsyncForTest(t)

	keyBody := strings.Repeat("9", 64)
	fullKey := "sk-" + keyBody
	var createAPIKeyCalled bool
	var quotaCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/user/create_child":
			t.Fatalf("manual Token creation must not create child account")
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apikeys":
			createAPIKeyCalled = true
			writeQiniuJSON(t, w, map[string]any{
				"status": true,
				"data":   map[string]any{"keys": []map[string]any{{"key": fullKey}}},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/v1/apikey/quota/"+fullKey:
			quotaCalled = true
			writeQiniuJSON(t, w, map[string]any{"status": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	configureQiniuChildAccountSettingForTest(t, server.URL)

	seedUser(t, 3902, int(12*common.QuotaPerUnit))
	require.NoError(t, CreateQiniuManagedToken(context.Background(), 3902, model.Token{Name: "managed", ExpiredTime: -1}))
	keyTasks, err := model.ListUserQiniuKeySyncTasks(3902, model.QiniuKeyTaskTypeManualCreate, model.QiniuKeyTaskStatusPending, 10)
	require.NoError(t, err)
	require.Len(t, keyTasks, 1)
	require.NoError(t, ExecuteQiniuKeyTask(context.Background(), keyTasks[0].Id))
	require.True(t, createAPIKeyCalled)
	require.True(t, quotaCalled)

	token, err := model.GetFirstUserToken(3902)
	require.NoError(t, err)
	require.Equal(t, keyBody, token.Key)
	require.Equal(t, model.TokenProviderQiniu, token.Provider)

	childTasks, err := model.ListQiniuChildAccountSyncTasks(model.QiniuChildAccountTaskQuery{}, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(0), childTasks.Total)
}
