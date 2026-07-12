package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestQiniuBillingSettlementModelsMigrateDefaultsAndUniqueness(t *testing.T) {
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	tempDB, err := gorm.Open(sqlite.Open("file:qiniu_billing_settlement_models?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := tempDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		_ = sqlDB.Close()
	})
	DB = tempDB
	common.UsingSQLite = true

	require.NoError(t, migratePhaseOneModels())

	assert.True(t, DB.Migrator().HasTable(&QiniuCostDetailRecord{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuBillingBucket{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuBillingBucketItem{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuBillingBucketApplication{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuQuotaGrant{}))
	assert.True(t, DB.Migrator().HasColumn(&QiniuCostDetailRecord{}, "qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasColumn(&QiniuBillingBucket{}, "qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasColumn(&QiniuKeySyncTask{}, "remote_cleanup_result"))
	assert.True(t, DB.Migrator().HasIndex(&QiniuCostDetailRecord{}, "idx_qiniu_cost_detail_records_qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasIndex(&QiniuBillingBucket{}, "idx_qiniu_billing_buckets_qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasIndex(&QiniuKeySyncTask{}, "idx_qiniu_key_sync_tasks_remote_cleanup_result"))

	raw := &QiniuCostDetailRecord{
		QiniuMaskedKey: "abc******123456",
		KeyPrefix:      "abc",
		KeySuffix:      "123456",
		BillingDate:    "2026-06-04",
		ModelName:      "qiniu-model",
		BillingItem:    "tokens",
		UsageCount:     100,
		UsageUnit:      "token",
		FeeAmount:      0.25,
		RecordHash:     "raw-hash-1",
		RawResponse:    `{"fee":0.25}`,
	}
	require.NoError(t, DB.Create(raw).Error)
	assert.NotZero(t, raw.CreatedTime)
	assert.NotZero(t, raw.UpdatedTime)
	assert.Equal(t, QiniuBillingOwnerStatusUnmapped, raw.OwnerStatus)
	assert.Equal(t, "CNY", raw.Currency)
	assert.Equal(t, 0, raw.QiniuChildAccountId)

	duplicateRaw := *raw
	duplicateRaw.Id = 0
	require.Error(t, DB.Create(&duplicateRaw).Error)

	bucket := &QiniuBillingBucket{
		UserId:            1001,
		TokenId:           2001,
		BillingDate:       "2026-06-04",
		QiniuMaskedKey:    "abc******123456",
		KeyFingerprint:    "fingerprint-1",
		OwnerStatus:       QiniuBillingOwnerStatusResolved,
		OfficialAmount:    0.25,
		OfficialQuota:     25,
		PendingDeltaQuota: 25,
	}
	require.NoError(t, DB.Create(bucket).Error)
	assert.NotZero(t, bucket.CreatedTime)
	assert.NotZero(t, bucket.UpdatedTime)
	assert.Equal(t, QiniuBillingBucketStatusPending, bucket.Status)
	assert.Equal(t, 0, bucket.QiniuChildAccountId)

	duplicateBucket := *bucket
	duplicateBucket.Id = 0
	require.Error(t, DB.Create(&duplicateBucket).Error)

	item := &QiniuBillingBucketItem{
		BucketId:     bucket.Id,
		ModelName:    "qiniu-model",
		BillingItem:  "tokens",
		UsageCount:   100,
		FeeAmount:    0.25,
		Currency:     "CNY",
		RawRecordIds: "1",
	}
	require.NoError(t, DB.Create(item).Error)
	duplicateItem := *item
	duplicateItem.Id = 0
	require.Error(t, DB.Create(&duplicateItem).Error)

	application := &QiniuBillingBucketApplication{
		BucketId:           bucket.Id,
		ApplyVersion:       1,
		DeltaQuota:         25,
		DeltaAmount:        0.25,
		IdempotencyKey:     "qiniu:billing_bucket:1:v1",
		BalanceBeforeQuota: 10,
		BalanceAfterQuota:  -15,
		DebtQuota:          15,
		OperationSource:    QiniuBillingOperationSourceSystem,
	}
	require.NoError(t, DB.Create(application).Error)
	assert.Equal(t, QiniuBillingApplicationStatusSuccess, application.Status)

	duplicateApplication := *application
	duplicateApplication.Id = 0
	require.Error(t, DB.Create(&duplicateApplication).Error)

	grant := &QiniuQuotaGrant{
		UserId:            1001,
		TokenId:           2001,
		BusinessKey:       "recharge:trade-1",
		GrantAmount:       100,
		RemoteApplyStatus: QiniuQuotaGrantStatusPending,
	}
	require.NoError(t, DB.Create(grant).Error)
	assert.NotZero(t, grant.CreatedTime)
	assert.NotZero(t, grant.UpdatedTime)

	duplicateGrant := *grant
	duplicateGrant.Id = 0
	require.Error(t, DB.Create(&duplicateGrant).Error)

	syncTask := &QiniuKeySyncTask{
		TaskType: QiniuKeyTaskTypeRevoke,
		UserId:   1001,
		TokenId:  2001,
		QiniuKey: "abc123",
	}
	require.NoError(t, DB.Create(syncTask).Error)
	assert.Equal(t, "", syncTask.RemoteCleanupResult)
	assert.Equal(t, "success", QiniuRemoteCleanupResultSuccess)
	assert.Equal(t, "idempotent_success", QiniuRemoteCleanupResultIdempotentSuccess)
}

func TestQiniuBillingSettlementQueryStructsExposeFilters(t *testing.T) {
	rawFilter := QiniuCostDetailRecordQuery{
		OwnerStatus:         QiniuBillingOwnerStatusUnmapped,
		UserId:              1001,
		TokenId:             2001,
		QiniuChildAccountId: 3001,
		BillingDate:         "2026-06-04",
		ModelName:           "qiniu-model",
		BillingItem:         "tokens",
		CreatedFrom:         1,
		CreatedTo:           2,
	}
	assert.Equal(t, QiniuBillingOwnerStatusUnmapped, rawFilter.OwnerStatus)
	assert.Equal(t, 3001, rawFilter.QiniuChildAccountId)

	bucketFilter := QiniuBillingBucketQuery{
		Status:              QiniuBillingBucketStatusPending,
		OwnerStatus:         QiniuBillingOwnerStatusResolved,
		UserId:              1001,
		TokenId:             2001,
		QiniuChildAccountId: 3001,
		BillingDate:         "2026-06-04",
		QiniuMaskedKey:      "abc******123456",
	}
	assert.Equal(t, QiniuBillingBucketStatusPending, bucketFilter.Status)
	assert.Equal(t, 3001, bucketFilter.QiniuChildAccountId)

	applicationFilter := QiniuBillingBucketApplicationQuery{
		Status:      QiniuBillingApplicationStatusSuccess,
		BucketId:    1,
		CreatedFrom: 1,
		CreatedTo:   2,
	}
	assert.Equal(t, QiniuBillingApplicationStatusSuccess, applicationFilter.Status)

	grantFilter := QiniuQuotaGrantQuery{
		RemoteApplyStatus: QiniuQuotaGrantStatusPending,
		UserId:            1001,
		TokenId:           2001,
		BusinessKey:       "recharge:trade-1",
	}
	assert.Equal(t, QiniuQuotaGrantStatusPending, grantFilter.RemoteApplyStatus)
}

func TestQiniuTokenOwnershipUsesSoftDeletedTokensAndProtectsHardDelete(t *testing.T) {
	useQiniuBillingTempDB(t, "qiniu_token_ownership")
	require.NoError(t, DB.AutoMigrate(&Token{}))

	rawKey := "sk-abcDEF0123456789xyz123456"
	identity := BuildQiniuTokenKeyIdentity(rawKey)
	require.Equal(t, "abc", identity.KeyPrefix)
	require.Equal(t, "123456", identity.KeySuffix)
	require.NotEmpty(t, identity.KeyFingerprint)

	qiniuToken := &Token{
		UserId:      2001,
		Key:         strings.TrimPrefix(rawKey, "sk-"),
		Provider:    TokenProviderQiniu,
		Status:      common.TokenStatusEnabled,
		Name:        "qiniu-soft-deleted-token",
		CreatedTime: common.GetTimestamp(),
		ExpiredTime: -1,
	}
	require.NoError(t, DB.Create(qiniuToken).Error)
	require.NoError(t, DB.Delete(qiniuToken).Error)

	matches, err := ListQiniuManagedTokensByKeyAffixes(identity.KeyPrefix, identity.KeySuffix, 10)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, qiniuToken.Id, matches[0].Id)
	assert.True(t, matches[0].DeletedAt.Valid)

	err = DB.Unscoped().Delete(qiniuToken).Error
	require.Error(t, err)
	require.Contains(t, err.Error(), "七牛托管 token 不允许物理删除")

	var retained Token
	require.NoError(t, DB.Unscoped().First(&retained, "id = ?", qiniuToken.Id).Error)
	assert.Equal(t, TokenProviderQiniu, retained.Provider)
}

func TestQiniuTokenPhysicalCleanupCandidatesExcludeManagedTokens(t *testing.T) {
	useQiniuBillingTempDB(t, "qiniu_token_cleanup")
	require.NoError(t, DB.AutoMigrate(&Token{}))

	now := common.GetTimestamp()
	localToken := &Token{
		UserId:      2101,
		Key:         "local-cleanup-token",
		Provider:    TokenProviderLocal,
		Status:      common.TokenStatusEnabled,
		Name:        "local-cleanup-token",
		CreatedTime: now,
		ExpiredTime: -1,
	}
	qiniuToken := &Token{
		UserId:      2102,
		Key:         "abc-cleanup-qiniu-token-123456",
		Provider:    TokenProviderQiniu,
		Status:      common.TokenStatusEnabled,
		Name:        "qiniu-cleanup-token",
		CreatedTime: now,
		ExpiredTime: -1,
	}
	require.NoError(t, DB.Create(localToken).Error)
	require.NoError(t, DB.Create(qiniuToken).Error)
	require.NoError(t, DB.Delete(localToken).Error)
	require.NoError(t, DB.Delete(qiniuToken).Error)

	candidates, err := ListSoftDeletedTokensForPhysicalCleanup(0, 10)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, localToken.Id, candidates[0].Id)
	assert.NotEqual(t, qiniuToken.Id, candidates[0].Id)
}

func useQiniuBillingTempDB(t *testing.T, name string) {
	t.Helper()
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	tempDB, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := tempDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		InitDBColumnNamesForTests()
		_ = sqlDB.Close()
	})
	DB = tempDB
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	InitDBColumnNamesForTests()
}
