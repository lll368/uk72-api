package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestListAdminQiniuKeysLatestContextQueriesAreBounded(t *testing.T) {
	resetQiniuKeyAdminTables(t)

	const tokenId = 88001
	const userId = 88002
	require.NoError(t, DB.Create(&User{
		Id:       userId,
		Username: "bounded-qiniu-admin-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, DB.Create(&Token{
		Id:             tokenId,
		UserId:         userId,
		Name:           "bounded-qiniu-key",
		Key:            "bounded-qiniu-admin-key-1234567890",
		Provider:       TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	for i := 0; i < 25; i++ {
		require.NoError(t, DB.Create(&QiniuQuotaGrant{
			UserId:            userId,
			TokenId:           tokenId,
			BusinessKey:       fmt.Sprintf("bounded-qiniu-admin-grant-%02d", i),
			GrantAmount:       float64(i + 1),
			RemoteApplyStatus: QiniuQuotaGrantStatusFailed,
			LastError:         fmt.Sprintf("failed grant %02d", i),
		}).Error)
		require.NoError(t, DB.Create(&QiniuKeySyncTask{
			TaskType: QiniuKeyTaskTypeRevoke,
			UserId:   userId,
			TokenId:  tokenId,
			QiniuKey: "bounded-qiniu-admin-key-1234567890",
			Status:   QiniuKeyTaskStatusFailed,
		}).Error)
	}

	maxRowsByTable := recordQiniuKeyAdminQueryRows(t)
	items, total, err := ListAdminQiniuKeys(AdminQiniuKeyQuery{TokenId: tokenId}, &common.PageInfo{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, 325.0, items[0].Quota.FailedLimitAmount)
	require.Equal(t, "failed grant 24", items[0].Quota.LatestGrantError)
	require.NotNil(t, items[0].LatestTask)

	require.LessOrEqual(t, maxRowsByTable["qiniu_quota_grants"], int64(1), "latest grant error lookup should not load every historical failed grant")
	require.LessOrEqual(t, maxRowsByTable["qiniu_key_sync_tasks"], int64(1), "latest task lookup should not load every historical task")
}

func TestListAdminQiniuKeysFiltersCopiedMaskedKey(t *testing.T) {
	resetQiniuKeyAdminTables(t)

	const tokenId = 88101
	const userId = 88102
	fullKey := "abcd-qiniu-admin-filter-middle-wxyz"
	require.NoError(t, DB.Create(&User{
		Id:       userId,
		Username: "masked-qiniu-admin-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, DB.Create(&Token{
		Id:             tokenId,
		UserId:         userId,
		Name:           "masked-qiniu-key",
		Key:            fullKey,
		Provider:       TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	items, total, err := ListAdminQiniuKeys(AdminQiniuKeyQuery{KeyFragment: MaskTokenKey(fullKey)}, &common.PageInfo{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, tokenId, items[0].Token.Id)
}

func resetQiniuKeyAdminTables(t *testing.T) {
	t.Helper()
	for _, table := range []string{"qiniu_quota_grants", "qiniu_key_sync_tasks", "tokens", "users"} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
	}
	t.Cleanup(func() {
		for _, table := range []string{"qiniu_quota_grants", "qiniu_key_sync_tasks", "tokens", "users"} {
			require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
		}
	})
}

func recordQiniuKeyAdminQueryRows(t *testing.T) map[string]int64 {
	t.Helper()
	maxRowsByTable := map[string]int64{}
	const callbackName = "qiniu_key_admin_test:record_query_rows"
	require.NoError(t, DB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		switch tx.Statement.Table {
		case "qiniu_quota_grants", "qiniu_key_sync_tasks":
			if tx.RowsAffected > maxRowsByTable[tx.Statement.Table] {
				maxRowsByTable[tx.Statement.Table] = tx.RowsAffected
			}
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, DB.Callback().Query().Remove(callbackName))
	})
	return maxRowsByTable
}
