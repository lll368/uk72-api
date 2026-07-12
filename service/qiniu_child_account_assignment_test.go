package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveQiniuAccountIdentityUsesParentForParentOnlyAndCutoverBoundary(t *testing.T) {
	truncate(t)
	configureQiniuAssignmentSettingForTest(t, operation_setting.QiniuChildAccountAssignmentModeParentOnly, 100)
	seedQiniuAssignmentUser(t, 6301, 101, 0)

	resolution, err := ResolveQiniuAccountIdentityForNextToken(context.Background(), 6301, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, resolution.AccountId)
	assert.Equal(t, QiniuAccountIdentitySourceParent, resolution.Source)

	configureQiniuAssignmentSettingForTest(t, operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild, 100)
	seedQiniuAssignmentUser(t, 6302, 100, 0)
	resolution, err = ResolveQiniuAccountIdentityForNextToken(context.Background(), 6302, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, resolution.AccountId)
	assert.Equal(t, QiniuAccountIdentitySourceParent, resolution.Source)
}

func TestResolveQiniuAccountIdentityAssignsAndReusesExclusiveChildAccount(t *testing.T) {
	truncate(t)
	configureQiniuAssignmentSettingForTest(t, operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild, 100)
	account := seedQiniuAssignmentChildAccount(t, 301, model.QiniuChildAccountStatusEnabled)
	secondAccount := seedQiniuAssignmentChildAccount(t, 302, model.QiniuChildAccountStatusEnabled)
	seedQiniuAssignmentUser(t, 6311, 101, 0)
	seedQiniuAssignmentUser(t, 6312, 101, 0)

	resolution, err := ResolveQiniuAccountIdentityForNextToken(context.Background(), 6311, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, account.Id, resolution.AccountId)
	assert.Equal(t, QiniuAccountIdentitySourceAssignedChild, resolution.Source)
	var user model.User
	require.NoError(t, model.DB.Select("qiniu_child_account_id").First(&user, "id = ?", 6311).Error)
	assert.Equal(t, account.Id, user.QiniuChildAccountId)

	resolution, err = ResolveQiniuAccountIdentityForNextToken(context.Background(), 6311, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, account.Id, resolution.AccountId)
	assert.Equal(t, QiniuAccountIdentitySourceUserBinding, resolution.Source)

	resolution, err = ResolveQiniuAccountIdentityForNextToken(context.Background(), 6312, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, secondAccount.Id, resolution.AccountId)
}

func TestResolveQiniuAccountIdentityDoesNotReassignOccupiedBinding(t *testing.T) {
	truncate(t)
	configureQiniuAssignmentSettingForTest(t, operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild, 100)
	boundAccount := seedQiniuAssignmentChildAccount(t, 311, model.QiniuChildAccountStatusEnabled)
	seedQiniuAssignmentChildAccount(t, 312, model.QiniuChildAccountStatusEnabled)
	seedQiniuAssignmentUser(t, 6321, 101, boundAccount.Id)
	seedQiniuAssignmentToken(t, 7321, 9991, boundAccount.Id, common.TokenStatusDisabled, false, "")

	resolution, err := ResolveQiniuAccountIdentityForNextToken(context.Background(), 6321, 0, 0)
	require.Nil(t, resolution)
	var blocker *QiniuAccountIdentityRetryableBlocker
	require.True(t, errors.As(err, &blocker))
	assert.Equal(t, boundAccount.Id, blocker.AccountId)
	assert.Equal(t, 0, blocker.TaskId)
}

func TestResolveQiniuAccountIdentityPersistsTaskReservedChildAccount(t *testing.T) {
	truncate(t)
	configureQiniuAssignmentSettingForTest(t, operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild, 100)
	reservedAccount := seedQiniuAssignmentChildAccount(t, 315, model.QiniuChildAccountStatusEnabled)
	seedQiniuAssignmentUser(t, 6325, 101, 0)

	resolution, err := ResolveQiniuAccountIdentityForNextToken(context.Background(), 6325, reservedAccount.Id, 0)
	require.NoError(t, err)
	assert.Equal(t, reservedAccount.Id, resolution.AccountId)
	assert.Equal(t, QiniuAccountIdentitySourceTaskReserved, resolution.Source)
	var user model.User
	require.NoError(t, model.DB.Select("qiniu_child_account_id").First(&user, "id = ?", 6325).Error)
	assert.Equal(t, reservedAccount.Id, user.QiniuChildAccountId)
}

func TestResolveQiniuAccountIdentityConcurrentAssignmentDoesNotShareChildAccount(t *testing.T) {
	truncate(t)
	configureQiniuAssignmentSettingForTest(t, operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild, 100)
	seedQiniuAssignmentChildAccount(t, 316, model.QiniuChildAccountStatusEnabled)
	seedQiniuAssignmentChildAccount(t, 317, model.QiniuChildAccountStatusEnabled)
	seedQiniuAssignmentUser(t, 6326, 101, 0)
	seedQiniuAssignmentUser(t, 6327, 101, 0)

	start := make(chan struct{})
	type result struct {
		accountId int
		err       error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, userId := range []int{6326, 6327} {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			resolution, err := ResolveQiniuAccountIdentityForNextToken(context.Background(), id, 0, 0)
			if err != nil {
				results <- result{err: err}
				return
			}
			results <- result{accountId: resolution.AccountId}
		}(userId)
	}
	close(start)
	wg.Wait()
	close(results)

	assigned := make(map[int]bool)
	for item := range results {
		require.NoError(t, item.err)
		require.NotZero(t, item.accountId)
		require.False(t, assigned[item.accountId], "child account assigned twice")
		assigned[item.accountId] = true
	}
	require.Len(t, assigned, 2)
}

func TestResolveQiniuAccountIdentitySkipsPendingCleanupAndReusesCleanupSuccess(t *testing.T) {
	truncate(t)
	configureQiniuAssignmentSettingForTest(t, operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild, 100)
	pendingAccount := seedQiniuAssignmentChildAccount(t, 321, model.QiniuChildAccountStatusEnabled)
	nextAccount := seedQiniuAssignmentChildAccount(t, 322, model.QiniuChildAccountStatusEnabled)
	seedQiniuAssignmentUser(t, 6331, 101, 0)
	pendingDeleted := seedQiniuAssignmentToken(t, 7331, 9331, pendingAccount.Id, common.TokenStatusEnabled, true, "")
	require.NotZero(t, pendingDeleted.Id)

	resolution, err := ResolveQiniuAccountIdentityForNextToken(context.Background(), 6331, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, nextAccount.Id, resolution.AccountId)

	seedQiniuAssignmentUser(t, 6332, 101, 0)
	require.NoError(t, model.CreateQiniuKeySyncTask(&model.QiniuKeySyncTask{
		TaskType:            model.QiniuKeyTaskTypeRevoke,
		UserId:              9331,
		TokenId:             pendingDeleted.Id,
		QiniuKey:            pendingDeleted.Key,
		Status:              model.QiniuKeyTaskStatusSuccess,
		RemoteCleanupResult: model.QiniuRemoteCleanupResultIdempotentSuccess,
	}))
	resolution, err = ResolveQiniuAccountIdentityForNextToken(context.Background(), 6332, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, pendingAccount.Id, resolution.AccountId)
}

func TestResolveQiniuAccountIdentityCreatesChildAccountTaskWhenUnavailable(t *testing.T) {
	truncate(t)
	disableQiniuChildAccountAsyncForTest(t)
	configureQiniuChildAccountSettingForTest(t, "http://127.0.0.1")
	configureQiniuAssignmentSettingForTest(t, operation_setting.QiniuChildAccountAssignmentModeOneKeyOneChild, 100)
	seedQiniuAssignmentUser(t, 6341, 101, 0)

	resolution, err := ResolveQiniuAccountIdentityForNextToken(context.Background(), 6341, 0, 8601)
	require.Nil(t, resolution)
	var blocker *QiniuAccountIdentityRetryableBlocker
	require.True(t, errors.As(err, &blocker))
	assert.NotZero(t, blocker.TaskId)
	assert.NotZero(t, blocker.AccountId)
	var task model.QiniuChildAccountSyncTask
	require.NoError(t, model.DB.First(&task, "id = ?", blocker.TaskId).Error)
	assert.Equal(t, model.QiniuChildAccountTaskTypeCreate, task.TaskType)
}

func configureQiniuAssignmentSettingForTest(t *testing.T, mode string, cutover int64) {
	t.Helper()
	setting := operation_setting.GetQiniuKeySetting()
	old := *setting
	setting.ChildAccountBindingEnabled = true
	setting.ChildAccountAssignmentMode = mode
	setting.ChildAccountBindingCutoverTime = cutover
	t.Cleanup(func() {
		*setting = old
	})
}

func seedQiniuAssignmentUser(t *testing.T, id int, createdAt int64, childAccountId int) *model.User {
	t.Helper()
	user := &model.User{
		Id:                  id,
		Username:            fmt.Sprintf("qiniu_assignment_user_%d", id),
		Password:            "password",
		AffCode:             fmt.Sprintf("qiniu_assignment_aff_%d", id),
		Status:              common.UserStatusEnabled,
		CreatedAt:           createdAt,
		QiniuChildAccountId: childAccountId,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func seedQiniuAssignmentChildAccount(t *testing.T, sequenceNo int, status string) *model.QiniuChildAccount {
	t.Helper()
	account := &model.QiniuChildAccount{
		SequenceNo: sequenceNo,
		Email:      fmt.Sprintf("assignment%d@uk72.cn", sequenceNo),
		UID:        fmt.Sprintf("uid-%d", sequenceNo),
		Status:     status,
	}
	require.NoError(t, model.DB.Create(account).Error)
	return account
}

func seedQiniuAssignmentToken(t *testing.T, id int, userId int, childAccountId int, status int, softDeleted bool, cleanupResult string) *model.Token {
	t.Helper()
	token := &model.Token{
		Id:                  id,
		UserId:              userId,
		Name:                "qiniu-assignment-token",
		Key:                 fmt.Sprintf("assignment-key-%d-123456", id),
		Provider:            model.TokenProviderQiniu,
		QiniuChildAccountId: childAccountId,
		Status:              status,
		CreatedTime:         common.GetTimestamp(),
		ExpiredTime:         -1,
	}
	require.NoError(t, model.DB.Create(token).Error)
	if softDeleted {
		require.NoError(t, model.DB.Delete(token).Error)
	}
	if cleanupResult != "" {
		require.NoError(t, model.CreateQiniuKeySyncTask(&model.QiniuKeySyncTask{
			TaskType:            model.QiniuKeyTaskTypeRevoke,
			UserId:              userId,
			TokenId:             id,
			QiniuKey:            token.Key,
			Status:              model.QiniuKeyTaskStatusSuccess,
			RemoteCleanupResult: cleanupResult,
		}))
	}
	return token
}
