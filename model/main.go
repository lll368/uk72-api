package model

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var commonGroupCol string
var commonKeyCol string
var commonTrueVal string
var commonFalseVal string

var logKeyCol string
var logGroupCol string

func initCol() {
	// init common column names
	if common.UsingPostgreSQL {
		commonGroupCol = `"group"`
		commonKeyCol = `"key"`
		commonTrueVal = "true"
		commonFalseVal = "false"
	} else {
		commonGroupCol = "`group`"
		commonKeyCol = "`key`"
		commonTrueVal = "1"
		commonFalseVal = "0"
	}
	if os.Getenv("LOG_SQL_DSN") != "" {
		switch common.LogSqlType {
		case common.DatabaseTypePostgreSQL:
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		default:
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	} else {
		// LOG_SQL_DSN 为空时，日志数据库与主数据库相同
		if common.UsingPostgreSQL {
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		} else {
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	}
	// log sql type and database type
	//common.SysLog("Using Log SQL Type: " + common.LogSqlType)
}

// InitDBColumnNamesForTests initializes cross-database quoted column names for tests
// that bootstrap model.DB directly without going through the normal DB setup path.
func InitDBColumnNamesForTests() {
	initCol()
}

var DB *gorm.DB

var LOG_DB *gorm.DB

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != common.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		common.SysLog("no user exists, create a root user for you: username is root, password is root123456")
		hashedPassword, err := common.Password2Hash("root123456")
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
		DB.Create(&rootUser)
	}
	return nil
}

func CheckSetup() {
	setup := GetSetup()
	if setup == nil {
		// No setup record exists, check if we have a root user
		if RootUserExists() {
			common.SysLog("system is not initialized, but root user exists")
			// Create setup record
			newSetup := Setup{
				Version:       common.Version,
				InitializedAt: time.Now().Unix(),
			}
			err := DB.Create(&newSetup).Error
			if err != nil {
				common.SysLog("failed to create setup record: " + err.Error())
			}
			constant.Setup = true
		} else {
			common.SysLog("system is not initialized and no root user exists")
			constant.Setup = false
		}
	} else {
		// Setup record exists, system is initialized
		common.SysLog("system is already initialized at: " + time.Unix(setup.InitializedAt, 0).String())
		constant.Setup = true
	}
}

func chooseDB(envName string, isLog bool) (*gorm.DB, error) {
	defer func() {
		initCol()
	}()
	dsn := os.Getenv(envName)
	if dsn != "" {
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			// Use PostgreSQL
			common.SysLog("using PostgreSQL as database")
			if !isLog {
				common.UsingPostgreSQL = true
			} else {
				common.LogSqlType = common.DatabaseTypePostgreSQL
			}
			return gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true, // disables implicit prepared statement usage
			}), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		if strings.HasPrefix(dsn, "local") {
			common.SysLog("SQL_DSN not set, using SQLite as database")
			if !isLog {
				common.UsingSQLite = true
			} else {
				common.LogSqlType = common.DatabaseTypeSQLite
			}
			return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		// Use MySQL
		common.SysLog("using MySQL as database")
		// check parseTime
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		if !isLog {
			common.UsingMySQL = true
		} else {
			common.LogSqlType = common.DatabaseTypeMySQL
		}
		return gorm.Open(mysql.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
	}
	// Use SQLite
	common.SysLog("SQL_DSN not set, using SQLite as database")
	common.UsingSQLite = true
	return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func InitDB() (err error) {
	db, err := chooseDB("SQL_DSN", false)
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		DB = db
		// MySQL charset/collation startup check: ensure Chinese-capable charset
		if common.UsingMySQL {
			if err := checkMySQLChineseSupport(DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		if common.UsingMySQL {
			//_, _ = sqlDB.Exec("ALTER TABLE channels MODIFY model_mapping TEXT;") // TODO: delete this line when most users have upgraded
		}
		common.SysLog("database migration started")
		err = migrateDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func InitLogDB() (err error) {
	if os.Getenv("LOG_SQL_DSN") == "" {
		LOG_DB = DB
		return
	}
	db, err := chooseDB("LOG_SQL_DSN", true)
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		LOG_DB = db
		// If log DB is MySQL, also ensure Chinese-capable charset
		if common.LogSqlType == common.DatabaseTypeMySQL {
			if err := checkMySQLChineseSupport(LOG_DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := LOG_DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		common.SysLog("database migration started")
		err = migrateLOGDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func migrateDB() error {
	// Migrate price_amount column from float/double to decimal for existing tables
	migrateSubscriptionPlanPriceAmount()
	// Migrate model_limits column from varchar to text for existing tables
	if err := migrateTokenModelLimitsToText(); err != nil {
		return err
	}
	coreModels := []interface{}{
		&Channel{},
		&Token{},
		&User{},
		&PasskeyCredential{},
		&Option{},
		&Redemption{},
		&Ability{},
		&Log{},
		&Midjourney{},
		&QuotaData{},
		&Task{},
		&Model{},
		&Vendor{},
		&PrefillGroup{},
		&Setup{},
		&TwoFA{},
		&TwoFABackupCode{},
		&Checkin{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionPreConsumeRecord{},
		&CustomOAuthProvider{},
		&UserOAuthBinding{},
		&PerfMetric{},
	}
	if !common.UsingSQLite {
		coreModels = append(coreModels, &TopUp{})
	}
	err := DB.AutoMigrate(coreModels...)
	if err != nil {
		return err
	}
	if common.UsingSQLite {
		// SQLite 对 decimal(p,s) 的既有 DDL 解析更敏感，TopUp 走缺表创建和补列路径。
		if err := autoMigrateTableIfMissing(&TopUp{}, "TopUp"); err != nil {
			return err
		}
	}
	if err := migratePhaseOneModels(); err != nil {
		return err
	}
	if err := ensureTokenProviderColumn(); err != nil {
		return err
	}
	if err := ensureQiniuChildAccountBindingColumns(); err != nil {
		return err
	}
	if err := ensureQiniuKeySyncTaskColumns(); err != nil {
		return err
	}
	if err := ensureQiniuChildAccountColumns(); err != nil {
		return err
	}
	if err := ensureUserRelationActiveChildIndex(); err != nil {
		return err
	}
	if err := ensureUserRelationTopupDiscountColumn(); err != nil {
		return err
	}
	if err := ensureUserTopupDiscountColumn(); err != nil {
		return err
	}
	if err := migrateRelationTopupDiscountsToUsers(); err != nil {
		return err
	}
	if err := ensureTopUpSnapshotColumns(); err != nil {
		return err
	}
	if err := ensurePhaseFourWalletColumns(); err != nil {
		return err
	}
	if err := migrateUserAuthAccountColumns(); err != nil {
		return err
	}
	if common.UsingSQLite {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
	} else {
		if err := DB.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	return nil
}

func migrateDBFast() error {

	var wg sync.WaitGroup

	migrations := []struct {
		model interface{}
		name  string
	}{
		{&Channel{}, "Channel"},
		{&Token{}, "Token"},
		{&User{}, "User"},
		{&PasskeyCredential{}, "PasskeyCredential"},
		{&Option{}, "Option"},
		{&Redemption{}, "Redemption"},
		{&Ability{}, "Ability"},
		{&Log{}, "Log"},
		{&Midjourney{}, "Midjourney"},
		{&QuotaData{}, "QuotaData"},
		{&Task{}, "Task"},
		{&Model{}, "Model"},
		{&Vendor{}, "Vendor"},
		{&PrefillGroup{}, "PrefillGroup"},
		{&Setup{}, "Setup"},
		{&TwoFA{}, "TwoFA"},
		{&TwoFABackupCode{}, "TwoFABackupCode"},
		{&Checkin{}, "Checkin"},
		{&SubscriptionOrder{}, "SubscriptionOrder"},
		{&UserSubscription{}, "UserSubscription"},
		{&SubscriptionPreConsumeRecord{}, "SubscriptionPreConsumeRecord"},
		{&CustomOAuthProvider{}, "CustomOAuthProvider"},
		{&UserOAuthBinding{}, "UserOAuthBinding"},
		{&PerfMetric{}, "PerfMetric"},
	}
	if !common.UsingSQLite {
		migrations = append(migrations, struct {
			model interface{}
			name  string
		}{&TopUp{}, "TopUp"})
	}
	// 动态计算migration数量，确保errChan缓冲区足够大
	errChan := make(chan error, len(migrations))

	for _, m := range migrations {
		wg.Add(1)
		go func(model interface{}, name string) {
			defer wg.Done()
			if err := DB.AutoMigrate(model); err != nil {
				errChan <- fmt.Errorf("failed to migrate %s: %v", name, err)
			}
		}(m.model, m.name)
	}

	// Wait for all migrations to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	if common.UsingSQLite {
		// 与 migrateDB 保持一致，SQLite 下避免对已存在 TopUp 表重复 AutoMigrate。
		if err := autoMigrateTableIfMissing(&TopUp{}, "TopUp"); err != nil {
			return err
		}
	}
	if err := migratePhaseOneModels(); err != nil {
		return err
	}
	if err := ensureTokenProviderColumn(); err != nil {
		return err
	}
	if err := ensureQiniuChildAccountBindingColumns(); err != nil {
		return err
	}
	if err := ensureQiniuKeySyncTaskColumns(); err != nil {
		return err
	}
	if err := ensureQiniuChildAccountColumns(); err != nil {
		return err
	}
	if err := ensureUserRelationActiveChildIndex(); err != nil {
		return err
	}
	if err := ensureUserRelationTopupDiscountColumn(); err != nil {
		return err
	}
	if err := ensureUserTopupDiscountColumn(); err != nil {
		return err
	}
	if err := migrateRelationTopupDiscountsToUsers(); err != nil {
		return err
	}
	if err := ensureTopUpSnapshotColumns(); err != nil {
		return err
	}
	if err := ensurePhaseFourWalletColumns(); err != nil {
		return err
	}
	if err := migrateUserAuthAccountColumns(); err != nil {
		return err
	}
	if common.UsingSQLite {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
	} else {
		if err := DB.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	common.SysLog("database migrated")
	return nil
}

func migrateLOGDB() error {
	var err error
	if err = LOG_DB.AutoMigrate(&Log{}); err != nil {
		return err
	}
	return nil
}

// migrationModel 描述一项模型迁移及其日志名称。
type migrationModel struct {
	model interface{}
	name  string
}

// phaseOneMigrationModels 返回阶段一新增的数据底座模型清单。
func phaseOneMigrationModels() []migrationModel {
	return []migrationModel{
		{model: &PhoneVerificationCode{}, name: "PhoneVerificationCode"},
		{model: &UserProfile{}, name: "UserProfile"},
		{model: &VipActivationRecord{}, name: "VipActivationRecord"},
		{model: &UserRelation{}, name: "UserRelation"},
		{model: &WalletAccount{}, name: "WalletAccount"},
		{model: &WalletFlow{}, name: "WalletFlow"},
		{model: &CommissionRecord{}, name: "CommissionRecord"},
		{model: &WithdrawalProfile{}, name: "WithdrawalProfile"},
		{model: &WithdrawOrder{}, name: "WithdrawOrder"},
		{model: &PiggyWithdrawCallbackLog{}, name: "PiggyWithdrawCallbackLog"},
		{model: &PaymentCallbackLog{}, name: "PaymentCallbackLog"},
		{model: &PaymentReconciliationTask{}, name: "PaymentReconciliationTask"},
		{model: &ContactMessage{}, name: "ContactMessage"},
		{model: &QiniuKeySyncTask{}, name: "QiniuKeySyncTask"},
		{model: &QiniuOfficialUsageRecord{}, name: "QiniuOfficialUsageRecord"},
		{model: &QiniuOfficialLedgerApplication{}, name: "QiniuOfficialLedgerApplication"},
		{model: &QiniuCostDetailRecord{}, name: "QiniuCostDetailRecord"},
		{model: &QiniuBillingBucket{}, name: "QiniuBillingBucket"},
		{model: &QiniuBillingBucketItem{}, name: "QiniuBillingBucketItem"},
		{model: &QiniuBillingBucketApplication{}, name: "QiniuBillingBucketApplication"},
		{model: &QiniuRealtimeWalletApplication{}, name: "QiniuRealtimeWalletApplication"},
		{model: &QiniuQuotaGrant{}, name: "QiniuQuotaGrant"},
		{model: &QiniuChildAccount{}, name: "QiniuChildAccount"},
		{model: &QiniuChildAccountSyncTask{}, name: "QiniuChildAccountSyncTask"},
	}
}

// autoMigrateTableIfMissing 只在表不存在时创建表，用于规避 SQLite 重复解析既有 DDL 的兼容问题。
func autoMigrateTableIfMissing(model interface{}, name string) error {
	if DB.Migrator().HasTable(model) {
		return nil
	}
	if err := DB.AutoMigrate(model); err != nil {
		return fmt.Errorf("failed to migrate %s: %w", name, err)
	}
	return nil
}

// migratePhaseOneModels 迁移手机号、VVIP、钱包、佣金、提现和支付审计相关模型。
func migratePhaseOneModels() error {
	models := phaseOneMigrationModels()
	if common.UsingSQLite {
		for _, item := range models {
			if err := autoMigrateTableIfMissing(item.model, item.name); err != nil {
				return err
			}
		}
		return nil
	}
	autoMigrateModels := make([]interface{}, 0, len(models))
	for _, item := range models {
		autoMigrateModels = append(autoMigrateModels, item.model)
	}
	if err := DB.AutoMigrate(autoMigrateModels...); err != nil {
		return fmt.Errorf("failed to migrate phase one models: %w", err)
	}
	return nil
}

func ensureQiniuKeySyncTaskColumns() error {
	if DB == nil || !DB.Migrator().HasTable(&QiniuKeySyncTask{}) {
		return nil
	}
	if !DB.Migrator().HasColumn(&QiniuKeySyncTask{}, "payload") {
		if err := DB.Migrator().AddColumn(&QiniuKeySyncTask{}, "Payload"); err != nil {
			return fmt.Errorf("failed to add qiniu key sync task payload column: %w", err)
		}
	}
	if !DB.Migrator().HasColumn(&QiniuKeySyncTask{}, "remote_cleanup_result") {
		if err := DB.Migrator().AddColumn(&QiniuKeySyncTask{}, "RemoteCleanupResult"); err != nil {
			return fmt.Errorf("failed to add qiniu key sync task remote cleanup result column: %w", err)
		}
	}
	if !DB.Migrator().HasIndex(&QiniuKeySyncTask{}, "idx_qiniu_key_sync_tasks_remote_cleanup_result") {
		if err := DB.Migrator().CreateIndex(&QiniuKeySyncTask{}, "RemoteCleanupResult"); err != nil {
			return fmt.Errorf("failed to create qiniu key sync task remote cleanup result index: %w", err)
		}
	}
	return nil
}

func ensureQiniuChildAccountBindingColumns() error {
	columns := []struct {
		model     interface{}
		column    string
		field     string
		indexName string
		label     string
	}{
		{model: &User{}, column: "qiniu_child_account_id", field: "QiniuChildAccountId", indexName: "idx_users_qiniu_child_account_id", label: "users.qiniu_child_account_id"},
		{model: &Token{}, column: "qiniu_child_account_id", field: "QiniuChildAccountId", indexName: "idx_tokens_qiniu_child_account_id", label: "tokens.qiniu_child_account_id"},
		{model: &QiniuOfficialUsageRecord{}, column: "qiniu_child_account_id", field: "QiniuChildAccountId", indexName: "idx_qiniu_official_usage_records_qiniu_child_account_id", label: "qiniu_official_usage_records.qiniu_child_account_id"},
		{model: &QiniuCostDetailRecord{}, column: "qiniu_child_account_id", field: "QiniuChildAccountId", indexName: "idx_qiniu_cost_detail_records_qiniu_child_account_id", label: "qiniu_cost_detail_records.qiniu_child_account_id"},
		{model: &QiniuBillingBucket{}, column: "qiniu_child_account_id", field: "QiniuChildAccountId", indexName: "idx_qiniu_billing_buckets_qiniu_child_account_id", label: "qiniu_billing_buckets.qiniu_child_account_id"},
	}
	for _, item := range columns {
		if DB == nil || !DB.Migrator().HasTable(item.model) {
			continue
		}
		if !DB.Migrator().HasColumn(item.model, item.column) {
			if err := DB.Migrator().AddColumn(item.model, item.field); err != nil {
				return fmt.Errorf("failed to add %s column: %w", item.label, err)
			}
		}
		if !DB.Migrator().HasIndex(item.model, item.indexName) {
			if err := DB.Migrator().CreateIndex(item.model, item.field); err != nil {
				return fmt.Errorf("failed to create %s index: %w", item.label, err)
			}
		}
	}
	return nil
}

func ensureQiniuChildAccountColumns() error {
	if DB == nil || !DB.Migrator().HasTable(&QiniuChildAccount{}) {
		return nil
	}
	if !DB.Migrator().HasColumn(&QiniuChildAccount{}, "login_password") {
		if err := DB.Migrator().AddColumn(&QiniuChildAccount{}, "LoginPassword"); err != nil {
			return fmt.Errorf("failed to add qiniu child account login password column: %w", err)
		}
	}
	if !DB.Migrator().HasColumn(&QiniuChildAccount{}, "backup_access_key") {
		if err := DB.Migrator().AddColumn(&QiniuChildAccount{}, "BackupAccessKey"); err != nil {
			return fmt.Errorf("failed to add qiniu child account backup access key column: %w", err)
		}
	}
	return nil
}

func ensureTokenProviderColumn() error {
	if DB == nil || !DB.Migrator().HasTable(&Token{}) {
		return nil
	}
	if !DB.Migrator().HasColumn(&Token{}, "provider") {
		if err := DB.Migrator().AddColumn(&Token{}, "Provider"); err != nil {
			return fmt.Errorf("failed to add token provider column: %w", err)
		}
	}
	if !DB.Migrator().HasIndex(&Token{}, "idx_tokens_provider") {
		if err := DB.Migrator().CreateIndex(&Token{}, "Provider"); err != nil {
			return fmt.Errorf("failed to create tokens provider index: %w", err)
		}
	}
	return backfillQiniuTokenProvider()
}

func backfillQiniuTokenProvider() error {
	if DB == nil || !DB.Migrator().HasTable(&Token{}) {
		return nil
	}
	lastID := 0
	for {
		var tokens []Token
		if err := DB.Unscoped().
			Select("id", "key", "provider").
			Where("id > ? AND (provider = ? OR provider IS NULL) AND "+commonKeyCol+" <> ?", lastID, TokenProviderLocal, "").
			Order("id asc").
			Limit(100).
			Find(&tokens).Error; err != nil {
			return fmt.Errorf("failed to list tokens for qiniu provider backfill: %w", err)
		}
		if len(tokens) == 0 {
			return nil
		}
		for _, token := range tokens {
			lastID = token.Id
			if !looksLikeQiniuTokenKey(token.Key) {
				continue
			}
			if err := DB.Unscoped().
				Model(&Token{}).
				Where("id = ? AND (provider = ? OR provider IS NULL)", token.Id, TokenProviderLocal).
				Update("provider", TokenProviderQiniu).Error; err != nil {
				return fmt.Errorf("failed to backfill qiniu token provider token_id=%d: %w", token.Id, err)
			}
		}
		if len(tokens) < 100 {
			return nil
		}
	}
}

func looksLikeQiniuTokenKey(key string) bool {
	key = strings.TrimPrefix(strings.TrimSpace(key), "sk-")
	if len(key) != 64 {
		return false
	}
	for _, ch := range key {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

// ensureUserRelationActiveChildIndex 确保邀请关系只约束 active 关系唯一，disabled 历史关系可保留。
func ensureUserRelationActiveChildIndex() error {
	if DB == nil || !DB.Migrator().HasTable(&UserRelation{}) {
		return nil
	}
	if !DB.Migrator().HasColumn(&UserRelation{}, "active_child_id") {
		if err := DB.Migrator().AddColumn(&UserRelation{}, "ActiveChildId"); err != nil {
			return fmt.Errorf("failed to add user_relations.active_child_id column: %w", err)
		}
	}
	// 旧数据没有 active_child_id 时，需要先按 active 状态回填，再创建唯一索引。
	if err := DB.Model(&UserRelation{}).
		Where("(status = ? OR status = '') AND active_child_id IS NULL", UserRelationStatusActive).
		Update("active_child_id", gorm.Expr("child_user_id")).Error; err != nil {
		return fmt.Errorf("failed to backfill user_relations.active_child_id: %w", err)
	}
	// 旧版本如果曾对 child_user_id 建全局唯一索引，需要移除，否则 disabled 历史关系无法保留。
	if DB.Migrator().HasIndex(&UserRelation{}, "idx_user_relations_child_user_id") {
		if err := DB.Migrator().DropIndex(&UserRelation{}, "idx_user_relations_child_user_id"); err != nil {
			return fmt.Errorf("failed to drop user_relations child_user_id unique index: %w", err)
		}
	}
	if !DB.Migrator().HasIndex(&UserRelation{}, "idx_user_relations_active_child") {
		if err := DB.Migrator().CreateIndex(&UserRelation{}, "idx_user_relations_active_child"); err != nil {
			return fmt.Errorf("failed to create user_relations active child unique index: %w", err)
		}
	}
	return nil
}

// ensureUserRelationTopupDiscountColumn 幂等补充关系级充值折扣字段，兼容旧库升级。
func ensureUserRelationTopupDiscountColumn() error {
	if DB == nil || !DB.Migrator().HasTable(&UserRelation{}) {
		return nil
	}
	if DB.Migrator().HasColumn(&UserRelation{}, "topup_discount") {
		return nil
	}
	if err := DB.Migrator().AddColumn(&UserRelation{}, "TopupDiscount"); err != nil {
		return fmt.Errorf("failed to add user_relations.topup_discount column: %w", err)
	}
	return nil
}

// ensureUserTopupDiscountColumn 幂等补充用户自身充值折扣字段，兼容旧库升级。
func ensureUserTopupDiscountColumn() error {
	if DB == nil || !DB.Migrator().HasTable(&User{}) {
		return nil
	}
	if DB.Migrator().HasColumn(&User{}, "topup_discount") {
		return nil
	}
	if err := DB.Migrator().AddColumn(&User{}, "TopupDiscount"); err != nil {
		return fmt.Errorf("failed to add users.topup_discount column: %w", err)
	}
	return nil
}

// ensureTopUpSnapshotColumns 幂等补充充值订单的金额快照字段，兼容旧库升级。
func migrateRelationTopupDiscountsToUsers() error {
	if DB == nil || !DB.Migrator().HasTable(&User{}) || !DB.Migrator().HasTable(&UserRelation{}) {
		return nil
	}
	if !DB.Migrator().HasColumn(&User{}, "topup_discount") || !DB.Migrator().HasColumn(&UserRelation{}, "topup_discount") {
		return nil
	}

	var relations []UserRelation
	if err := DB.Select("child_user_id", "topup_discount").
		Where("status = ? AND topup_discount > ? AND topup_discount < ?", UserRelationStatusActive, 0, 1).
		Find(&relations).Error; err != nil {
		return fmt.Errorf("failed to load legacy user relation discounts: %w", err)
	}

	for _, relation := range relations {
		var user User
		if err := DB.Select("id", "topup_discount").Where("id = ?", relation.ChildUserId).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return fmt.Errorf("failed to load user %d for discount migration: %w", relation.ChildUserId, err)
		}
		if user.TopupDiscount != nil && *user.TopupDiscount > 0 && *user.TopupDiscount <= 1 {
			continue
		}
		if err := DB.Model(&User{}).Where("id = ?", relation.ChildUserId).Update("topup_discount", relation.TopupDiscount).Error; err != nil {
			return fmt.Errorf("failed to migrate user %d topup discount: %w", relation.ChildUserId, err)
		}
	}
	return nil
}

func ensureTopUpSnapshotColumns() error {
	if DB == nil || !DB.Migrator().HasTable(&TopUp{}) {
		return nil
	}
	columns := []struct {
		field  string
		column string
	}{
		{field: "RechargeAmount", column: "recharge_amount"},
		{field: "PaidAmount", column: "paid_amount"},
		{field: "Discount", column: "discount"},
		{field: "ReversedAt", column: "reversed_at"},
	}
	for _, column := range columns {
		if DB.Migrator().HasColumn(&TopUp{}, column.column) {
			continue
		}
		if err := DB.Migrator().AddColumn(&TopUp{}, column.field); err != nil {
			return fmt.Errorf("failed to add topup.%s column: %w", column.column, err)
		}
	}
	return nil
}

// ensurePhaseFourWalletColumns 幂等补充阶段四钱包、佣金、提现新增字段。
func ensurePhaseFourWalletColumns() error {
	columnGroups := []struct {
		model   interface{}
		table   string
		columns []struct {
			field  string
			column string
		}
	}{
		{
			model: &WalletFlow{},
			table: "wallet_flows",
			columns: []struct {
				field  string
				column string
			}{
				{field: "IdempotencyKey", column: "idempotency_key"},
			},
		},
		{
			model: &CommissionRecord{},
			table: "commission_records",
			columns: []struct {
				field  string
				column string
			}{
				{field: "BaseAmount", column: "base_amount"},
				{field: "CommissionRate", column: "commission_rate"},
				{field: "SourceUserLabel", column: "source_user_label"},
				{field: "ErrorMessage", column: "error_message"},
				{field: "ReversedAt", column: "reversed_at"},
				{field: "ReverseReason", column: "reverse_reason"},
			},
		},
		{
			model: &WithdrawOrder{},
			table: "withdraw_orders",
			columns: []struct {
				field  string
				column string
			}{
				{field: "ReceiveType", column: "receive_type"},
				{field: "PaymentVoucher", column: "payment_voucher"},
				{field: "Remark", column: "remark"},
				{field: "Provider", column: "provider"},
				{field: "PiggyStatus", column: "piggy_status"},
				{field: "WithdrawalProfileId", column: "withdrawal_profile_id"},
				{field: "AccountName", column: "account_name"},
				{field: "BankName", column: "bank_name"},
				{field: "PayoutMobile", column: "payout_mobile"},
				{field: "PayoutIdCardNo", column: "payout_id_card_no"},
				{field: "PayoutBankCardNo", column: "payout_bank_card_no"},
				{field: "PlatformFeeRate", column: "platform_fee_rate"},
				{field: "PlatformFeeAmountCents", column: "platform_fee_amount_cents"},
				{field: "TaxBeforeAmountCents", column: "tax_before_amount_cents"},
				{field: "FrozenAmountCents", column: "frozen_amount_cents"},
				{field: "PiggyPayAmountCents", column: "piggy_pay_amount_cents"},
				{field: "PiggyPretaxAmountCents", column: "piggy_pretax_amount_cents"},
				{field: "PiggyIndividualTaxCents", column: "piggy_individual_tax_cents"},
				{field: "PiggyAddedTaxCents", column: "piggy_added_tax_cents"},
				{field: "PiggyAfterTaxAmountCents", column: "piggy_after_tax_amount_cents"},
				{field: "PiggyFeeAmountCents", column: "piggy_fee_amount_cents"},
				{field: "PiggyPayAmount", column: "piggy_pay_amount"},
				{field: "ExternalTradeNo", column: "external_trade_no"},
				{field: "FrontLogNo", column: "front_log_no"},
				{field: "LaborOrderNo", column: "labor_order_no"},
				{field: "NotifyType", column: "notify_type"},
				{field: "TradeStatus", column: "trade_status"},
				{field: "TradeFailCode", column: "trade_fail_code"},
				{field: "TradeResult", column: "trade_result"},
				{field: "TradeResultDescribe", column: "trade_result_describe"},
				{field: "TaxFundId", column: "tax_fund_id"},
				{field: "PositionName", column: "position_name"},
				{field: "Position", column: "position"},
				{field: "CalcType", column: "calc_type"},
				{field: "BankRemark", column: "bank_remark"},
				{field: "RequestPayloadDigest", column: "request_payload_digest"},
				{field: "ResponsePayloadDigest", column: "response_payload_digest"},
				{field: "ManualReviewReason", column: "manual_review_reason"},
				{field: "ManualHandledBy", column: "manual_handled_by"},
				{field: "ManualHandledAt", column: "manual_handled_at"},
				{field: "ManualHandleResult", column: "manual_handle_result"},
				{field: "CompensationStatus", column: "compensation_status"},
				{field: "SubmittedAt", column: "submitted_at"},
				{field: "ConfirmedAt", column: "confirmed_at"},
				{field: "TerminalAt", column: "terminal_at"},
			},
		},
	}
	for _, group := range columnGroups {
		if DB == nil || !DB.Migrator().HasTable(group.model) {
			continue
		}
		for _, column := range group.columns {
			if DB.Migrator().HasColumn(group.model, column.column) {
				continue
			}
			if err := DB.Migrator().AddColumn(group.model, column.field); err != nil {
				return fmt.Errorf("failed to add %s.%s column: %w", group.table, column.column, err)
			}
		}
	}
	if DB != nil && DB.Migrator().HasTable(&WalletFlow{}) && !DB.Migrator().HasIndex(&WalletFlow{}, "idx_wallet_flows_idempotency_key") {
		_ = DB.Migrator().CreateIndex(&WalletFlow{}, "idx_wallet_flows_idempotency_key")
	}
	if DB != nil && DB.Migrator().HasTable(&CommissionRecord{}) && !DB.Migrator().HasIndex(&CommissionRecord{}, "idx_commission_source_level_beneficiary") {
		_ = DB.Migrator().CreateIndex(&CommissionRecord{}, "idx_commission_source_level_beneficiary")
	}
	if DB != nil && DB.Migrator().HasTable(&WithdrawOrder{}) && DB.Migrator().HasColumn(&WithdrawOrder{}, "provider") {
		if err := DB.Model(&WithdrawOrder{}).
			Where("provider = ? OR provider IS NULL", "").
			Update("provider", WithdrawProviderManual).Error; err != nil {
			return fmt.Errorf("failed to backfill withdraw_orders.provider: %w", err)
		}
	}
	return nil
}

type sqliteColumnDef struct {
	Name string
	DDL  string
}

func ensureSubscriptionPlanTableSQLite() error {
	if !common.UsingSQLite {
		return nil
	}
	tableName := "subscription_plans"
	if !DB.Migrator().HasTable(tableName) {
		createSQL := `CREATE TABLE ` + "`" + tableName + "`" + ` (
` + "`id`" + ` integer,
` + "`title`" + ` varchar(128) NOT NULL,
` + "`subtitle`" + ` varchar(255) DEFAULT '',
` + "`price_amount`" + ` decimal(10,6) NOT NULL,
` + "`currency`" + ` varchar(8) NOT NULL DEFAULT 'USD',
` + "`duration_unit`" + ` varchar(16) NOT NULL DEFAULT 'month',
` + "`duration_value`" + ` integer NOT NULL DEFAULT 1,
` + "`custom_seconds`" + ` bigint NOT NULL DEFAULT 0,
` + "`enabled`" + ` numeric DEFAULT 1,
` + "`sort_order`" + ` integer DEFAULT 0,
` + "`stripe_price_id`" + ` varchar(128) DEFAULT '',
` + "`creem_product_id`" + ` varchar(128) DEFAULT '',
` + "`max_purchase_per_user`" + ` integer DEFAULT 0,
` + "`upgrade_group`" + ` varchar(64) DEFAULT '',
` + "`total_amount`" + ` bigint NOT NULL DEFAULT 0,
` + "`quota_reset_period`" + ` varchar(16) DEFAULT 'never',
` + "`quota_reset_custom_seconds`" + ` bigint DEFAULT 0,
` + "`created_at`" + ` bigint,
` + "`updated_at`" + ` bigint,
PRIMARY KEY (` + "`id`" + `)
)`
		return DB.Exec(createSQL).Error
	}
	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := DB.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		existing[c.Name] = struct{}{}
	}
	required := []sqliteColumnDef{
		{Name: "title", DDL: "`title` varchar(128) NOT NULL"},
		{Name: "subtitle", DDL: "`subtitle` varchar(255) DEFAULT ''"},
		{Name: "price_amount", DDL: "`price_amount` decimal(10,6) NOT NULL"},
		{Name: "currency", DDL: "`currency` varchar(8) NOT NULL DEFAULT 'USD'"},
		{Name: "duration_unit", DDL: "`duration_unit` varchar(16) NOT NULL DEFAULT 'month'"},
		{Name: "duration_value", DDL: "`duration_value` integer NOT NULL DEFAULT 1"},
		{Name: "custom_seconds", DDL: "`custom_seconds` bigint NOT NULL DEFAULT 0"},
		{Name: "enabled", DDL: "`enabled` numeric DEFAULT 1"},
		{Name: "sort_order", DDL: "`sort_order` integer DEFAULT 0"},
		{Name: "stripe_price_id", DDL: "`stripe_price_id` varchar(128) DEFAULT ''"},
		{Name: "creem_product_id", DDL: "`creem_product_id` varchar(128) DEFAULT ''"},
		{Name: "max_purchase_per_user", DDL: "`max_purchase_per_user` integer DEFAULT 0"},
		{Name: "upgrade_group", DDL: "`upgrade_group` varchar(64) DEFAULT ''"},
		{Name: "total_amount", DDL: "`total_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "quota_reset_period", DDL: "`quota_reset_period` varchar(16) DEFAULT 'never'"},
		{Name: "quota_reset_custom_seconds", DDL: "`quota_reset_custom_seconds` bigint DEFAULT 0"},
		{Name: "created_at", DDL: "`created_at` bigint"},
		{Name: "updated_at", DDL: "`updated_at` bigint"},
	}
	for _, col := range required {
		if _, ok := existing[col.Name]; ok {
			continue
		}
		if err := DB.Exec("ALTER TABLE `" + tableName + "` ADD COLUMN " + col.DDL).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateUserAuthAccountColumns() error {
	if DB == nil || common.UsingSQLite || !DB.Migrator().HasTable(&User{}) {
		return nil
	}
	columns := []struct {
		name string
		ddl  string
	}{
		{name: "username", ddl: "varchar(191)"},
		{name: "email", ddl: "varchar(191)"},
	}
	for _, column := range columns {
		if !DB.Migrator().HasColumn(&User{}, column.name) {
			continue
		}
		var alterSQL string
		if common.UsingPostgreSQL {
			var dataType string
			var maxLength int
			if err := DB.Raw(`SELECT data_type, COALESCE(character_maximum_length, 0)
				FROM information_schema.columns
				WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
				"users", column.name).Row().Scan(&dataType, &maxLength); err != nil {
				common.SysLog(fmt.Sprintf("Warning: failed to query metadata for users.%s: %v", column.name, err))
			} else if dataType == "character varying" && maxLength >= UserNameMaxLength {
				continue
			}
			alterSQL = fmt.Sprintf(`ALTER TABLE users ALTER COLUMN %s TYPE varchar(%d)`, column.name, UserNameMaxLength)
		} else if common.UsingMySQL {
			var columnType string
			if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
				"users", column.name).Scan(&columnType).Error; err != nil {
				common.SysLog(fmt.Sprintf("Warning: failed to query metadata for users.%s: %v", column.name, err))
			} else {
				normalizedType := strings.ToLower(strings.TrimSpace(columnType))
				if strings.HasPrefix(normalizedType, "varchar(") && strings.HasSuffix(normalizedType, ")") {
					lengthText := strings.TrimSuffix(strings.TrimPrefix(normalizedType, "varchar("), ")")
					if maxLength, err := strconv.Atoi(lengthText); err == nil && maxLength >= UserNameMaxLength {
						continue
					}
				}
			}
			alterSQL = fmt.Sprintf("ALTER TABLE users MODIFY COLUMN %s %s", column.name, column.ddl)
		}
		if alterSQL == "" {
			continue
		}
		if err := DB.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("failed to migrate users.%s to %s: %w", column.name, column.ddl, err)
		}
		common.SysLog(fmt.Sprintf("Successfully migrated users.%s to %s", column.name, column.ddl))
	}
	return nil
}

// migrateTokenModelLimitsToText migrates model_limits column from varchar(1024) to text
// This is safe to run multiple times - it checks the column type first
func migrateTokenModelLimitsToText() error {
	// SQLite uses type affinity, so TEXT and VARCHAR are effectively the same — no migration needed
	if common.UsingSQLite {
		return nil
	}

	tableName := "tokens"
	columnName := "model_limits"

	if !DB.Migrator().HasTable(tableName) {
		return nil
	}

	if !DB.Migrator().HasColumn(&Token{}, columnName) {
		return nil
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE text`, tableName, columnName)
	} else if common.UsingMySQL {
		var columnType string
		if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.ToLower(columnType) == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s text", tableName, columnName)
	} else {
		return nil
	}

	if alterSQL != "" {
		if err := DB.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("failed to migrate %s.%s to text: %w", tableName, columnName, err)
		}
		common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to text", tableName, columnName))
	}
	return nil
}

// migrateSubscriptionPlanPriceAmount migrates price_amount column from float/double to decimal(10,6)
// This is safe to run multiple times - it checks the column type first
func migrateSubscriptionPlanPriceAmount() {
	// SQLite doesn't support ALTER COLUMN, and its type affinity handles this automatically
	// Skip early to avoid GORM parsing the existing table DDL which may cause issues
	if common.UsingSQLite {
		return
	}

	tableName := "subscription_plans"
	columnName := "price_amount"

	// Check if table exists first
	if !DB.Migrator().HasTable(tableName) {
		return
	}

	// Check if column exists
	if !DB.Migrator().HasColumn(&SubscriptionPlan{}, columnName) {
		return
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		// PostgreSQL: Check if already decimal/numeric
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "numeric" {
			return // Already decimal/numeric
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE decimal(10,6) USING %s::decimal(10,6)`,
			tableName, columnName, columnName)
	} else if common.UsingMySQL {
		// MySQL: Check if already decimal
		var columnType string
		if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.HasPrefix(strings.ToLower(columnType), "decimal") {
			return // Already decimal
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s decimal(10,6) NOT NULL DEFAULT 0",
			tableName, columnName)
	} else {
		return
	}

	if alterSQL != "" {
		if err := DB.Exec(alterSQL).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to migrate %s.%s to decimal: %v", tableName, columnName, err))
		} else {
			common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to decimal(10,6)", tableName, columnName))
		}
	}
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func CloseDB() error {
	if LOG_DB != DB {
		err := closeDB(LOG_DB)
		if err != nil {
			return err
		}
	}
	return closeDB(DB)
}

// checkMySQLChineseSupport ensures the MySQL connection and current schema
// default charset/collation can store Chinese characters. It allows common
// Chinese-capable charsets (utf8mb4, utf8, gbk, big5, gb18030) and panics otherwise.
func checkMySQLChineseSupport(db *gorm.DB) error {
	// 仅检测：当前库默认字符集/排序规则 + 各表的排序规则（隐含字符集）

	// Read current schema defaults
	var schemaCharset, schemaCollation string
	err := db.Raw("SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = DATABASE()").Row().Scan(&schemaCharset, &schemaCollation)
	if err != nil {
		return fmt.Errorf("读取当前库默认字符集/排序规则失败 / Failed to read schema default charset/collation: %v", err)
	}

	toLower := func(s string) string { return strings.ToLower(s) }
	// Allowed charsets that can store Chinese text
	allowedCharsets := map[string]string{
		"utf8mb4": "utf8mb4_",
		"utf8":    "utf8_",
		"gbk":     "gbk_",
		"big5":    "big5_",
		"gb18030": "gb18030_",
	}
	isChineseCapable := func(cs, cl string) bool {
		csLower := toLower(cs)
		clLower := toLower(cl)
		if prefix, ok := allowedCharsets[csLower]; ok {
			if clLower == "" {
				return true
			}
			return strings.HasPrefix(clLower, prefix)
		}
		// 如果仅提供了排序规则，尝试按排序规则前缀判断
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(clLower, prefix) {
				return true
			}
		}
		return false
	}

	// 1) 当前库默认值必须支持中文
	if !isChineseCapable(schemaCharset, schemaCollation) {
		return fmt.Errorf("当前库默认字符集/排序规则不支持中文：schema(%s/%s)。请将库设置为 utf8mb4/utf8/gbk/big5/gb18030 / Schema default charset/collation is not Chinese-capable: schema(%s/%s). Please set to utf8mb4/utf8/gbk/big5/gb18030",
			schemaCharset, schemaCollation, schemaCharset, schemaCollation)
	}

	// 2) 所有物理表的排序规则（隐含字符集）必须支持中文
	type tableInfo struct {
		Name      string
		Collation *string
	}
	var tables []tableInfo
	if err := db.Raw("SELECT TABLE_NAME, TABLE_COLLATION FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'").Scan(&tables).Error; err != nil {
		return fmt.Errorf("读取表排序规则失败 / Failed to read table collations: %v", err)
	}

	var badTables []string
	for _, t := range tables {
		// NULL 或空表示继承库默认设置，已在上面校验库默认，视为通过
		if t.Collation == nil || *t.Collation == "" {
			continue
		}
		cl := *t.Collation
		// 仅凭排序规则判断是否中文可用
		ok := false
		lower := strings.ToLower(cl)
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(lower, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			badTables = append(badTables, fmt.Sprintf("%s(%s)", t.Name, cl))
		}
	}

	if len(badTables) > 0 {
		// 限制输出数量以避免日志过长
		maxShow := 20
		shown := badTables
		if len(shown) > maxShow {
			shown = shown[:maxShow]
		}
		return fmt.Errorf(
			"存在不支持中文的表，请修复其排序规则/字符集。示例（最多展示 %d 项）：%v / Found tables not Chinese-capable. Please fix their collation/charset. Examples (showing up to %d): %v",
			maxShow, shown, maxShow, shown,
		)
	}
	return nil
}

var (
	lastPingTime time.Time
	pingMutex    sync.Mutex
)

func PingDB() error {
	pingMutex.Lock()
	defer pingMutex.Unlock()

	if time.Since(lastPingTime) < time.Second*10 {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Error getting sql.DB from GORM: %v", err)
		return err
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Printf("Error pinging DB: %v", err)
		return err
	}

	lastPingTime = time.Now()
	common.SysLog("Database pinged successfully")
	return nil
}
