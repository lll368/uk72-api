package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQiniuChildAccountBindingQueryHelpers(t *testing.T) {
	useQiniuBindingTempDB(t, "qiniu_child_account_binding_helpers")

	now := common.GetTimestamp()
	user := seedQiniuBindingUser(t, 6101, 3001)
	otherUser := seedQiniuBindingUser(t, 6102, 3002)
	enabledToken := seedQiniuBindingToken(t, 7101, user.Id, 3001, common.TokenStatusEnabled, "child-enabled-123456", now)
	disabledToken := seedQiniuBindingToken(t, 7102, user.Id, 3001, common.TokenStatusDisabled, "child-disabled-123456", now)
	localToken := seedQiniuBindingToken(t, 7103, user.Id, 3001, common.TokenStatusEnabled, "child-local-123456", now)
	require.NoError(t, DB.Model(localToken).Update("provider", TokenProviderLocal).Error)
	otherChildToken := seedQiniuBindingToken(t, 7104, otherUser.Id, 3002, common.TokenStatusEnabled, "other-child-123456", now)
	pendingDeletedToken := seedQiniuBindingToken(t, 7105, user.Id, 3001, common.TokenStatusEnabled, "pending-delete-123456", now)
	reusableDeletedToken := seedQiniuBindingToken(t, 7106, user.Id, 3001, common.TokenStatusEnabled, "reusable-delete-123456", now)
	skippedDeletedToken := seedQiniuBindingToken(t, 7107, user.Id, 3001, common.TokenStatusEnabled, "skipped-delete-123456", now)
	require.NoError(t, DB.Delete(pendingDeletedToken).Error)
	require.NoError(t, DB.Delete(reusableDeletedToken).Error)
	require.NoError(t, DB.Delete(skippedDeletedToken).Error)
	require.NoError(t, CreateQiniuKeySyncTask(&QiniuKeySyncTask{
		TaskType:            QiniuKeyTaskTypeRevoke,
		UserId:              user.Id,
		TokenId:             reusableDeletedToken.Id,
		QiniuKey:            reusableDeletedToken.Key,
		Status:              QiniuKeyTaskStatusSuccess,
		RemoteCleanupResult: QiniuRemoteCleanupResultSuccess,
	}))
	require.NoError(t, CreateQiniuKeySyncTask(&QiniuKeySyncTask{
		TaskType: QiniuKeyTaskTypeRevoke,
		UserId:   user.Id,
		TokenId:  skippedDeletedToken.Id,
		QiniuKey: skippedDeletedToken.Key,
		Status:   QiniuKeyTaskStatusSkipped,
	}))

	users, err := ListUsersByQiniuChildAccountId(3001, 10)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, user.Id, users[0].Id)

	tokens, err := ListTokensByQiniuChildAccountId(3001, true, 20)
	require.NoError(t, err)
	require.Len(t, tokens, 5)
	assert.Equal(t, enabledToken.Id, tokens[0].Id)
	assert.Equal(t, disabledToken.Id, tokens[1].Id)

	nonDeletedCount, err := CountNonDeletedQiniuManagedTokensByChildAccountId(3001)
	require.NoError(t, err)
	assert.Equal(t, int64(2), nonDeletedCount)

	enabledCount, err := CountEnabledQiniuManagedTokensByChildAccountId(3001)
	require.NoError(t, err)
	assert.Equal(t, int64(1), enabledCount)

	cleanupDone, err := HasReusableQiniuRemoteCleanupForToken(reusableDeletedToken.Id)
	require.NoError(t, err)
	assert.True(t, cleanupDone)

	skippedCleanupDone, err := HasReusableQiniuRemoteCleanupForToken(skippedDeletedToken.Id)
	require.NoError(t, err)
	assert.False(t, skippedCleanupDone)

	pendingCleanupCount, err := CountRemoteCleanupPendingSoftDeletedQiniuTokensByChildAccountId(3001)
	require.NoError(t, err)
	assert.Equal(t, int64(2), pendingCleanupCount)

	summaries, err := ListQiniuChildAccountTokenSummaries(3001, 20)
	require.NoError(t, err)
	require.Len(t, summaries, 5)
	assert.Equal(t, enabledToken.Id, summaries[0].Id)
	assert.Equal(t, user.Username, summaries[0].Username)
	assert.Equal(t, QiniuTokenKeyFingerprint(enabledToken.Key), summaries[0].KeyFingerprint)
	assert.NotContains(t, summaries[0].KeyFingerprint, enabledToken.Key)
	assert.True(t, containsQiniuTokenSummary(summaries, reusableDeletedToken.Id, QiniuRemoteCleanupResultSuccess))
	assert.False(t, containsQiniuTokenSummary(summaries, otherChildToken.Id, ""))
}

func TestQiniuChildAccountBindingDefaultsToParentAccount(t *testing.T) {
	useQiniuBindingTempDB(t, "qiniu_child_account_binding_defaults")

	user := &User{
		Id:       6201,
		Username: "qiniu_binding_default_user",
		Password: "password",
		AffCode:  "qiniu_binding_default_aff",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(user).Error)
	token := &Token{
		Id:          7201,
		UserId:      user.Id,
		Name:        "qiniu-binding-default-token",
		Key:         "default-parent-123456",
		Provider:    TokenProviderQiniu,
		Status:      common.TokenStatusEnabled,
		CreatedTime: common.GetTimestamp(),
		ExpiredTime: -1,
	}
	require.NoError(t, DB.Create(token).Error)

	var reloadedUser User
	require.NoError(t, DB.First(&reloadedUser, "id = ?", user.Id).Error)
	assert.Equal(t, 0, reloadedUser.QiniuChildAccountId)
	var reloadedToken Token
	require.NoError(t, DB.First(&reloadedToken, "id = ?", token.Id).Error)
	assert.Equal(t, 0, reloadedToken.QiniuChildAccountId)
}

func useQiniuBindingTempDB(t *testing.T, name string) {
	t.Helper()
	useQiniuBillingTempDB(t, name)
	require.NoError(t, DB.AutoMigrate(&User{}, &Token{}, &QiniuKeySyncTask{}))
	require.NoError(t, ensureQiniuChildAccountBindingColumns())
	require.NoError(t, ensureQiniuKeySyncTaskColumns())
}

func seedQiniuBindingUser(t *testing.T, id int, childAccountId int) *User {
	t.Helper()
	user := &User{
		Id:                  id,
		Username:            fmt.Sprintf("qiniu_binding_user_%d", id),
		Password:            "password",
		AffCode:             fmt.Sprintf("qiniu_binding_aff_%d", id),
		Status:              common.UserStatusEnabled,
		QiniuChildAccountId: childAccountId,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func seedQiniuBindingToken(t *testing.T, id int, userId int, childAccountId int, status int, key string, now int64) *Token {
	t.Helper()
	token := &Token{
		Id:                  id,
		UserId:              userId,
		Name:                "qiniu-binding-token",
		Key:                 key,
		Provider:            TokenProviderQiniu,
		QiniuChildAccountId: childAccountId,
		Status:              status,
		CreatedTime:         now + int64(id),
		AccessedTime:        now,
		ExpiredTime:         -1,
	}
	require.NoError(t, DB.Create(token).Error)
	return token
}

func containsQiniuTokenSummary(summaries []QiniuChildAccountTokenSummary, tokenId int, cleanupResult string) bool {
	for _, summary := range summaries {
		if summary.Id == tokenId {
			return summary.RemoteCleanupResult == cleanupResult
		}
	}
	return false
}
