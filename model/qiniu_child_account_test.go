package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestQiniuChildAccountModelDefaultsAndEmailGeneration(t *testing.T) {
	truncateTables(t)

	account, err := CreateQiniuChildAccountWithSequence(DB, "child", "uk72.cn", "protected-password")
	require.NoError(t, err)
	require.NotZero(t, account.Id)
	require.Equal(t, 1, account.SequenceNo)
	require.Equal(t, "child1@uk72.cn", account.Email)
	require.Equal(t, QiniuChildAccountStatusCreating, account.Status)
	require.NotZero(t, account.CreatedTime)
	require.NotZero(t, account.UpdatedTime)
	require.NotEmpty(t, account.LoginPassword)
	require.NotEqual(t, "protected-password", account.LoginPassword)
	revealedPassword, err := RevealQiniuChildAccountLoginPassword(account.LoginPassword)
	require.NoError(t, err)
	require.Equal(t, "protected-password", revealedPassword)

	next, err := CreateQiniuChildAccountWithSequence(DB, "child", "@uk72.cn", "protected-password-2")
	require.NoError(t, err)
	require.Equal(t, 2, next.SequenceNo)
	require.Equal(t, "child2@uk72.cn", next.Email)
}

func TestQiniuChildAccountModelUniqueSequenceAndEmail(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&QiniuChildAccount{
		SequenceNo:    7,
		Email:         "child7@uk72.cn",
		LoginPassword: "protected-password",
	}).Error)

	err := DB.Create(&QiniuChildAccount{
		SequenceNo:    7,
		Email:         "child-other@uk72.cn",
		LoginPassword: "protected-password",
	}).Error
	require.Error(t, err)

	err = DB.Create(&QiniuChildAccount{
		SequenceNo:    8,
		Email:         "child7@uk72.cn",
		LoginPassword: "protected-password",
	}).Error
	require.Error(t, err)
}

func TestListAdminQiniuChildAccountsEscapesEmailFilter(t *testing.T) {
	truncateTables(t)

	account, err := CreateQiniuChildAccountWithSequence(DB, "team_", "uk72.cn", "protected-password")
	require.NoError(t, err)
	require.Equal(t, "team_1@uk72.cn", account.Email)

	items, total, err := ListAdminQiniuChildAccounts(AdminQiniuChildAccountQuery{
		Email: "team_1",
	}, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, account.Id, items[0].Account.Id)
}

func TestQiniuChildAccountTaskDefaultsAndRetryableList(t *testing.T) {
	truncateTables(t)

	account, err := CreateQiniuChildAccountWithSequence(DB, "child", "uk72.cn", "protected-password")
	require.NoError(t, err)

	task := &QiniuChildAccountSyncTask{
		AccountId: account.Id,
		TaskType:  QiniuChildAccountTaskTypeCreate,
	}
	require.NoError(t, CreateQiniuChildAccountSyncTask(task))
	require.NotZero(t, task.Id)
	require.Equal(t, QiniuChildAccountTaskStatusPending, task.Status)
	require.NotZero(t, task.CreatedTime)
	require.NotZero(t, task.UpdatedTime)

	tasks, err := ListRetryableQiniuChildAccountSyncTasks(10, common.GetTimestamp()-60)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, task.Id, tasks[0].Id)
}

func TestMigratePhaseOneModelsIncludesQiniuChildAccountTables(t *testing.T) {
	require.NoError(t, migratePhaseOneModels())
	require.True(t, DB.Migrator().HasTable(&QiniuChildAccount{}))
	require.True(t, DB.Migrator().HasTable(&QiniuChildAccountSyncTask{}))
	require.True(t, DB.Migrator().HasIndex(&QiniuChildAccount{}, "idx_qiniu_child_accounts_sequence_no"))
	require.True(t, DB.Migrator().HasIndex(&QiniuChildAccount{}, "idx_qiniu_child_accounts_email"))
}

func TestNormalizeQiniuChildAccountDomain(t *testing.T) {
	domain, err := NormalizeQiniuChildAccountDomain(" @UK72.CN ")
	require.NoError(t, err)
	require.Equal(t, "uk72.cn", domain)

	for _, value := range []string{"", "localhost", "bad domain.com", "https://uk72.cn"} {
		_, err := NormalizeQiniuChildAccountDomain(value)
		require.Error(t, err)
	}
}

func TestQiniuChildAccountMigrationCompatibility(t *testing.T) {
	originalDB := DB
	tempDB, err := gorm.Open(sqlite.Open("file:qiniu_child_account_migration?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := tempDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		DB = originalDB
		_ = sqlDB.Close()
	})
	DB = tempDB

	require.NoError(t, migratePhaseOneModels())
	require.NoError(t, ensureQiniuChildAccountColumns())
	require.NoError(t, tempDB.Create(&QiniuChildAccount{
		SequenceNo:    1,
		Email:         "child1@uk72.cn",
		LoginPassword: strings.Repeat("p", 12),
	}).Error)
	require.NoError(t, ensureQiniuChildAccountColumns())
}
