package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyTopUpWithoutReversedAt struct {
	Id             int
	UserId         int
	Amount         int64
	Money          float64
	RechargeAmount float64
	PaidAmount     float64
	Discount       float64
	TradeNo        string
	CreateTime     int64
	CompleteTime   int64
	Status         string
}

func (legacyTopUpWithoutReversedAt) TableName() string {
	return "top_ups"
}

type legacyCommissionRecordWithoutSourceUserLabel struct {
	Id                  int
	BeneficiaryUserId   int
	SourceUserId        int
	SourceOrderNo       string
	SourceType          string
	Level               int
	BaseAmount          float64
	CommissionRate      float64
	Amount              float64
	QualificationStatus string
	Status              string
	ErrorMessage        string
	SettledAt           int64
	ReversedAt          int64
	ReverseReason       string
	CreatedAt           int64
	UpdatedAt           int64
}

func (legacyCommissionRecordWithoutSourceUserLabel) TableName() string {
	return "commission_records"
}

type legacyTokenWithoutProvider struct {
	Id     int
	UserId int
	Key    string
}

func (legacyTokenWithoutProvider) TableName() string {
	return "tokens"
}

func TestMigratePhaseOneModels_SQLite(t *testing.T) {
	require.NoError(t, migratePhaseOneModels())
	require.NoError(t, autoMigrateTableIfMissing(&TopUp{}, "TopUp"))
	require.NoError(t, ensureTopUpSnapshotColumns())
	require.NoError(t, migratePhaseOneModels())

	assert.True(t, DB.Migrator().HasTable(&PhoneVerificationCode{}))
	assert.True(t, DB.Migrator().HasTable(&UserProfile{}))
	assert.True(t, DB.Migrator().HasTable(&VipActivationRecord{}))
	assert.True(t, DB.Migrator().HasTable(&UserRelation{}))
	assert.True(t, DB.Migrator().HasTable(&WalletAccount{}))
	assert.True(t, DB.Migrator().HasTable(&WalletFlow{}))
	assert.True(t, DB.Migrator().HasTable(&CommissionRecord{}))
	assert.True(t, DB.Migrator().HasTable(&WithdrawalProfile{}))
	assert.True(t, DB.Migrator().HasTable(&WithdrawOrder{}))
	assert.True(t, DB.Migrator().HasTable(&PiggyWithdrawCallbackLog{}))
	assert.True(t, DB.Migrator().HasTable(&PaymentCallbackLog{}))
	assert.True(t, DB.Migrator().HasTable(&PaymentReconciliationTask{}))
	assert.True(t, DB.Migrator().HasTable(&ContactMessage{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuKeySyncTask{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuOfficialUsageRecord{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuOfficialLedgerApplication{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuCostDetailRecord{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuBillingBucket{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuBillingBucketItem{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuBillingBucketApplication{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuRealtimeWalletApplication{}))
	assert.True(t, DB.Migrator().HasTable(&QiniuQuotaGrant{}))

	assert.True(t, DB.Migrator().HasColumn(&User{}, "topup_discount"))
	assert.True(t, DB.Migrator().HasColumn(&TopUp{}, "recharge_amount"))
	assert.True(t, DB.Migrator().HasColumn(&TopUp{}, "paid_amount"))
	assert.True(t, DB.Migrator().HasColumn(&TopUp{}, "discount"))
	assert.True(t, DB.Migrator().HasColumn(&TopUp{}, "reversed_at"))
	assert.True(t, DB.Migrator().HasColumn(&CommissionRecord{}, "source_user_label"))
}

func TestEnsureTokenProviderColumnAddsColumnAndBackfillsQiniuKeys(t *testing.T) {
	originalDB := DB
	tempDB, err := gorm.Open(sqlite.Open("file:token_provider_migration?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := tempDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		DB = originalDB
		_ = sqlDB.Close()
	})
	DB = tempDB

	qiniuKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	require.NoError(t, DB.AutoMigrate(&legacyTokenWithoutProvider{}))
	require.NoError(t, DB.Create(&legacyTokenWithoutProvider{Id: 1, UserId: 1, Key: qiniuKey}).Error)
	require.NoError(t, DB.Create(&legacyTokenWithoutProvider{Id: 2, UserId: 2, Key: "local-token"}).Error)
	require.False(t, DB.Migrator().HasColumn(&Token{}, "provider"))

	require.NoError(t, ensureTokenProviderColumn())
	assert.True(t, DB.Migrator().HasColumn(&Token{}, "provider"))

	var qiniuToken Token
	require.NoError(t, DB.Unscoped().Select("provider").Where("id = ?", 1).First(&qiniuToken).Error)
	assert.Equal(t, TokenProviderQiniu, qiniuToken.Provider)

	var localToken Token
	require.NoError(t, DB.Unscoped().Select("provider").Where("id = ?", 2).First(&localToken).Error)
	assert.Empty(t, localToken.Provider)
}

func TestEnsureQiniuChildAccountBindingColumnsAddsColumnsAndIndexesToExistingTables(t *testing.T) {
	originalDB := DB
	tempDB, err := gorm.Open(sqlite.Open("file:qiniu_child_account_binding_columns?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := tempDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		DB = originalDB
		_ = sqlDB.Close()
	})
	DB = tempDB

	require.NoError(t, DB.Table("users").AutoMigrate(&struct {
		Id       int
		Username string
	}{}))
	require.NoError(t, DB.Table("tokens").AutoMigrate(&struct {
		Id     int
		UserId int
		Key    string
	}{}))
	require.NoError(t, DB.Table("qiniu_official_usage_records").AutoMigrate(&struct {
		Id        int
		RecordKey string
		UserId    int
		TokenId   int
	}{}))
	require.NoError(t, DB.Table("qiniu_cost_detail_records").AutoMigrate(&struct {
		Id         int
		RecordHash string
		UserId     int
		TokenId    int
	}{}))
	require.NoError(t, DB.Table("qiniu_billing_buckets").AutoMigrate(&struct {
		Id          int
		UserId      int
		TokenId     int
		BillingDate string
	}{}))
	require.NoError(t, DB.Table("qiniu_key_sync_tasks").AutoMigrate(&struct {
		Id       int
		TaskType string
		Status   string
	}{}))

	require.False(t, DB.Migrator().HasColumn(&User{}, "qiniu_child_account_id"))
	require.False(t, DB.Migrator().HasColumn(&Token{}, "qiniu_child_account_id"))
	require.False(t, DB.Migrator().HasColumn(&QiniuOfficialUsageRecord{}, "qiniu_child_account_id"))
	require.False(t, DB.Migrator().HasColumn(&QiniuCostDetailRecord{}, "qiniu_child_account_id"))
	require.False(t, DB.Migrator().HasColumn(&QiniuBillingBucket{}, "qiniu_child_account_id"))
	require.False(t, DB.Migrator().HasColumn(&QiniuKeySyncTask{}, "remote_cleanup_result"))

	require.NoError(t, ensureQiniuChildAccountBindingColumns())
	require.NoError(t, ensureQiniuKeySyncTaskColumns())

	assert.True(t, DB.Migrator().HasColumn(&User{}, "qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasColumn(&Token{}, "qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasColumn(&QiniuOfficialUsageRecord{}, "qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasColumn(&QiniuCostDetailRecord{}, "qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasColumn(&QiniuBillingBucket{}, "qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasColumn(&QiniuKeySyncTask{}, "remote_cleanup_result"))
	assert.True(t, DB.Migrator().HasIndex(&User{}, "idx_users_qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasIndex(&Token{}, "idx_tokens_qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasIndex(&QiniuOfficialUsageRecord{}, "idx_qiniu_official_usage_records_qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasIndex(&QiniuCostDetailRecord{}, "idx_qiniu_cost_detail_records_qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasIndex(&QiniuBillingBucket{}, "idx_qiniu_billing_buckets_qiniu_child_account_id"))
	assert.True(t, DB.Migrator().HasIndex(&QiniuKeySyncTask{}, "idx_qiniu_key_sync_tasks_remote_cleanup_result"))
}

func TestEnsureTopUpSnapshotColumnsAddsReversedAtToExistingSQLiteTable(t *testing.T) {
	originalDB := DB
	tempDB, err := gorm.Open(sqlite.Open("file:topup_reversed_at_migration?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := tempDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		DB = originalDB
		_ = sqlDB.Close()
	})
	DB = tempDB

	require.NoError(t, DB.AutoMigrate(&legacyTopUpWithoutReversedAt{}))
	require.False(t, DB.Migrator().HasColumn(&TopUp{}, "reversed_at"))

	require.NoError(t, ensureTopUpSnapshotColumns())
	assert.True(t, DB.Migrator().HasColumn(&TopUp{}, "reversed_at"))
}

func TestEnsurePhaseFourWalletColumnsAddsSourceUserLabelToExistingCommissionRecords(t *testing.T) {
	originalDB := DB
	tempDB, err := gorm.Open(sqlite.Open("file:commission_source_user_label_migration?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := tempDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		DB = originalDB
		_ = sqlDB.Close()
	})
	DB = tempDB

	require.NoError(t, DB.AutoMigrate(&legacyCommissionRecordWithoutSourceUserLabel{}))
	require.False(t, DB.Migrator().HasColumn(&CommissionRecord{}, "source_user_label"))

	require.NoError(t, ensurePhaseFourWalletColumns())
	assert.True(t, DB.Migrator().HasColumn(&CommissionRecord{}, "source_user_label"))
}

func TestEnsureUserTopupDiscountColumnAddsColumnToExistingSQLiteTable(t *testing.T) {
	originalDB := DB
	tempDB, err := gorm.Open(sqlite.Open("file:user_topup_discount_migration?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := tempDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		DB = originalDB
		_ = sqlDB.Close()
	})
	DB = tempDB

	type legacyUserWithoutTopupDiscount struct {
		Id       int
		Username string
		Password string
	}

	require.NoError(t, DB.Table("users").AutoMigrate(&legacyUserWithoutTopupDiscount{}))
	require.False(t, DB.Migrator().HasColumn(&User{}, "topup_discount"))

	require.NoError(t, ensureUserTopupDiscountColumn())
	assert.True(t, DB.Migrator().HasColumn(&User{}, "topup_discount"))
}

func TestMigrateRelationTopupDiscountsToUsersPreservesExistingUserDiscount(t *testing.T) {
	truncateTables(t)

	existingDiscount := 0.9
	require.NoError(t, DB.Create(&User{
		Id:            1201,
		Username:      "existing_discount_user",
		Password:      "password",
		AffCode:       "existing_discount_user",
		TopupDiscount: &existingDiscount,
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:       1202,
		Username: "legacy_relation_discount_user",
		Password: "password",
		AffCode:  "legacy_relation_discount_user",
	}).Error)
	require.NoError(t, DB.Create(&UserRelation{
		ParentUserId:  1301,
		ChildUserId:   1201,
		Status:        UserRelationStatusActive,
		Source:        UserRelationSourceAdmin,
		TopupDiscount: 0.7,
	}).Error)
	require.NoError(t, DB.Create(&UserRelation{
		ParentUserId:  1301,
		ChildUserId:   1202,
		Status:        UserRelationStatusActive,
		Source:        UserRelationSourceAdmin,
		TopupDiscount: 0.8,
	}).Error)

	require.NoError(t, migrateRelationTopupDiscountsToUsers())

	var existing User
	require.NoError(t, DB.Where("id = ?", 1201).First(&existing).Error)
	require.NotNil(t, existing.TopupDiscount)
	assert.InDelta(t, 0.9, *existing.TopupDiscount, 0.000001)

	var migrated User
	require.NoError(t, DB.Where("id = ?", 1202).First(&migrated).Error)
	require.NotNil(t, migrated.TopupDiscount)
	assert.InDelta(t, 0.8, *migrated.TopupDiscount, 0.000001)
}

func TestWalletAccount_SeparateBalanceAndCommission(t *testing.T) {
	truncateTables(t)

	account := &WalletAccount{UserId: 801}
	require.NoError(t, DB.Create(account).Error)

	created, err := GetWalletAccountByUserId(801)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Zero(t, created.BalanceAmount)
	assert.Zero(t, created.CommissionAmount)
	assert.Zero(t, created.FrozenCommissionAmount)

	require.NoError(t, DB.Model(&WalletAccount{}).Where("user_id = ?", 801).Updates(map[string]interface{}{
		"balance_amount":           12.5,
		"commission_amount":        34.75,
		"frozen_commission_amount": 6.25,
	}).Error)

	updated, err := GetWalletAccountByUserId(801)
	require.NoError(t, err)
	assert.Equal(t, 12.5, updated.BalanceAmount)
	assert.Equal(t, 34.75, updated.CommissionAmount)
	assert.Equal(t, 6.25, updated.FrozenCommissionAmount)
}

func TestVipActivationRecord_DefaultSnapshot(t *testing.T) {
	truncateTables(t)

	record := &VipActivationRecord{
		UserId:          901,
		TradeNo:         "vip-activation-default",
		PaymentProvider: PaymentProviderEpay,
		PaymentMethod:   "alipay",
		Status:          VipActivationStatusSuccess,
		ActivatedAt:     time.Now().Unix(),
	}
	require.NoError(t, DB.Create(record).Error)

	created, err := GetVipActivationByUserId(901)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, 1680.0, created.ActivationAmount)
	assert.Equal(t, 1680.0, created.PaidAmount)
	assert.Equal(t, 1.0, created.Discount)
}

func TestVipActivationRecord_GetActiveIgnoresLatestPending(t *testing.T) {
	truncateTables(t)

	activatedAt := time.Now().Add(-time.Hour).Unix()
	successRecord := &VipActivationRecord{
		UserId:      902,
		TradeNo:     "vip-activation-success",
		Status:      VipActivationStatusSuccess,
		ActivatedAt: activatedAt,
	}
	require.NoError(t, DB.Create(successRecord).Error)

	pendingRecord := &VipActivationRecord{
		UserId:  902,
		TradeNo: "vip-activation-pending",
		Status:  VipActivationStatusPending,
	}
	require.NoError(t, DB.Create(pendingRecord).Error)

	active, err := GetVipActivationByUserId(902)
	require.NoError(t, err)
	assert.Equal(t, "vip-activation-success", active.TradeNo)

	latest, err := GetLatestVipActivationByUserId(902)
	require.NoError(t, err)
	assert.Equal(t, "vip-activation-pending", latest.TradeNo)
}

func TestUserRelation_OnlyOneActiveChild(t *testing.T) {
	truncateTables(t)

	first := &UserRelation{
		ParentUserId: 1001,
		ChildUserId:  1002,
		Source:       UserRelationSourceRegister,
		Status:       UserRelationStatusActive,
		BindTime:     time.Now().Unix(),
	}
	require.NoError(t, DB.Create(first).Error)

	second := &UserRelation{
		ParentUserId: 1003,
		ChildUserId:  1002,
		Source:       UserRelationSourceRegister,
		Status:       UserRelationStatusActive,
		BindTime:     time.Now().Unix(),
	}
	require.Error(t, DB.Create(second).Error)

	disabled := &UserRelation{
		ParentUserId: 1004,
		ChildUserId:  1002,
		Source:       UserRelationSourceAdmin,
		Status:       UserRelationStatusDisabled,
		BindTime:     time.Now().Unix(),
	}
	require.NoError(t, DB.Create(disabled).Error)

	found, err := GetUserRelationByChildId(1002)
	require.NoError(t, err)
	assert.Equal(t, 1001, found.ParentUserId)
}

func TestUserRelation_GetActiveIgnoresDisabled(t *testing.T) {
	truncateTables(t)

	disabled := &UserRelation{
		ParentUserId: 1001,
		ChildUserId:  1005,
		Source:       UserRelationSourceAdmin,
		Status:       UserRelationStatusDisabled,
		BindTime:     time.Now().Unix(),
	}
	require.NoError(t, DB.Create(disabled).Error)

	_, err := GetUserRelationByChildId(1005)
	require.Error(t, err)
}

func TestTopUpSnapshotColumns(t *testing.T) {
	truncateTables(t)

	topUp := &TopUp{
		UserId:          1101,
		Amount:          100,
		Money:           90,
		TradeNo:         "topup-snapshot",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		RechargeAmount:  100,
		PaidAmount:      90,
		Discount:        0.9,
		Status:          "pending",
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())

	created := GetTopUpByTradeNo("topup-snapshot")
	require.NotNil(t, created)
	assert.Equal(t, 100.0, created.RechargeAmount)
	assert.Equal(t, 90.0, created.PaidAmount)
	assert.Equal(t, 0.9, created.Discount)
}
