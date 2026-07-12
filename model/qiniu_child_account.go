package model

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	QiniuChildAccountStatusCreating = "creating"
	QiniuChildAccountStatusEnabled  = "enabled"
	QiniuChildAccountStatusDisabled = "disabled"
	QiniuChildAccountStatusFailed   = "failed"

	qiniuChildAccountEncryptedPrefix = "enc:"
)

var qiniuChildAccountDomainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
var qiniuChildAccountPrefixPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// QiniuChildAccount 保存七牛 OEM 子账户本地快照。
type QiniuChildAccount struct {
	Id              int    `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	SequenceNo      int    `json:"sequence_no" gorm:"uniqueIndex:idx_qiniu_child_accounts_sequence_no"`
	Email           string `json:"email" gorm:"type:varchar(191);uniqueIndex:idx_qiniu_child_accounts_email"`
	RemoteUserID    string `json:"remote_user_id" gorm:"type:varchar(128);index"`
	UID             string `json:"uid" gorm:"type:varchar(128);index"`
	ParentUID       string `json:"parent_uid" gorm:"type:varchar(128);index"`
	AccessKey       string `json:"access_key" gorm:"type:varchar(128)"`
	SecretKey       string `json:"-" gorm:"type:varchar(512)"`
	KeyState        string `json:"key_state" gorm:"type:varchar(32)"`
	BackupAccessKey string `json:"backup_access_key" gorm:"type:varchar(128)"`
	BackupSecretKey string `json:"-" gorm:"type:varchar(512)"`
	BackupKeyState  string `json:"backup_key_state" gorm:"type:varchar(32)"`
	Status          string `json:"status" gorm:"type:varchar(32);index"`
	LastError       string `json:"last_error" gorm:"type:text"`
	LoginPassword   string `json:"-" gorm:"type:varchar(512)"`
	CreatedBy       int    `json:"created_by" gorm:"index"`
	DisabledBy      int    `json:"disabled_by" gorm:"index"`
	DisabledReason  string `json:"disabled_reason" gorm:"type:varchar(512)"`
	CreatedTime     int64  `json:"created_time" gorm:"bigint;index"`
	UpdatedTime     int64  `json:"updated_time" gorm:"bigint"`
	DisabledTime    int64  `json:"disabled_time" gorm:"bigint;index"`
}

func (account *QiniuChildAccount) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if account.CreatedTime == 0 {
		account.CreatedTime = now
	}
	if account.UpdatedTime == 0 {
		account.UpdatedTime = now
	}
	if account.Status == "" {
		account.Status = QiniuChildAccountStatusCreating
	}
	account.Email = strings.TrimSpace(strings.ToLower(account.Email))
	return nil
}

func (account *QiniuChildAccount) BeforeUpdate(tx *gorm.DB) error {
	account.UpdatedTime = common.GetTimestamp()
	account.Email = strings.TrimSpace(strings.ToLower(account.Email))
	return nil
}

// NormalizeQiniuChildAccountDomain 归一化并校验子账户邮箱域名。
func NormalizeQiniuChildAccountDomain(value string) (string, error) {
	domain := strings.TrimSpace(strings.ToLower(value))
	domain = strings.TrimPrefix(domain, "@")
	if domain == "" {
		return "", errors.New("七牛子账户邮箱域名不能为空")
	}
	if strings.Contains(domain, "://") || !qiniuChildAccountDomainPattern.MatchString(domain) {
		return "", fmt.Errorf("七牛子账户邮箱域名无效: %s", value)
	}
	return domain, nil
}

func NormalizeQiniuChildAccountPrefix(value string) string {
	prefix := strings.TrimSpace(value)
	if prefix == "" || !qiniuChildAccountPrefixPattern.MatchString(prefix) {
		return "child"
	}
	return prefix
}

func BuildQiniuChildAccountEmail(prefix string, sequenceNo int, domain string) (string, error) {
	if sequenceNo <= 0 {
		return "", errors.New("七牛子账户序号无效")
	}
	normalizedDomain, err := NormalizeQiniuChildAccountDomain(domain)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%d@%s", NormalizeQiniuChildAccountPrefix(prefix), sequenceNo, normalizedDomain), nil
}

// CreateQiniuChildAccountWithSequence 使用数据库中最大序号生成下一个本地子账户。
func CreateQiniuChildAccountWithSequence(db *gorm.DB, prefix string, domain string, loginPassword string) (*QiniuChildAccount, error) {
	if db == nil {
		db = DB
	}
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}
	normalizedDomain, err := NormalizeQiniuChildAccountDomain(domain)
	if err != nil {
		return nil, err
	}
	prefix = NormalizeQiniuChildAccountPrefix(prefix)
	loginPassword = strings.TrimSpace(loginPassword)
	if loginPassword == "" {
		return nil, errors.New("七牛子账户登录密码不能为空")
	}
	var created *QiniuChildAccount
	err = db.Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&QiniuChildAccount{})
		if !common.UsingSQLite {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var latest QiniuChildAccount
		err := query.Select("sequence_no").Order("sequence_no desc").Limit(1).First(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		sequenceNo := latest.SequenceNo + 1
		if sequenceNo <= 0 {
			sequenceNo = 1
		}
		email, err := BuildQiniuChildAccountEmail(prefix, sequenceNo, normalizedDomain)
		if err != nil {
			return err
		}
		protectedPassword, err := ProtectQiniuChildAccountLoginPassword(loginPassword)
		if err != nil {
			return err
		}
		account := &QiniuChildAccount{
			SequenceNo:    sequenceNo,
			Email:         email,
			LoginPassword: protectedPassword,
			Status:        QiniuChildAccountStatusCreating,
		}
		if err := tx.Create(account).Error; err != nil {
			return err
		}
		created = account
		return nil
	})
	return created, err
}

func GetQiniuChildAccountById(id int) (*QiniuChildAccount, error) {
	var account QiniuChildAccount
	if err := DB.First(&account, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func MarkQiniuChildAccountRemoteCreated(id int, remoteUserID string, uid string, parentUID string, disabled bool) error {
	status := QiniuChildAccountStatusCreating
	if disabled {
		status = QiniuChildAccountStatusDisabled
	}
	return DB.Model(&QiniuChildAccount{}).Where("id = ?", id).Updates(map[string]interface{}{
		"remote_user_id": strings.TrimSpace(remoteUserID),
		"uid":            strings.TrimSpace(uid),
		"parent_uid":     strings.TrimSpace(parentUID),
		"status":         status,
		"login_password": "",
		"updated_time":   common.GetTimestamp(),
	}).Error
}

func MarkQiniuChildAccountCredentials(id int, accessKey string, secretKey string, keyState string, backupAccessKey string, backupSecretKey string, backupKeyState string) error {
	protectedSecretKey, err := ProtectQiniuChildAccountSecret(secretKey)
	if err != nil {
		return err
	}
	protectedBackupSecretKey, err := ProtectQiniuChildAccountSecret(backupSecretKey)
	if err != nil {
		return err
	}
	return DB.Model(&QiniuChildAccount{}).Where("id = ?", id).Updates(map[string]interface{}{
		"access_key":        strings.TrimSpace(accessKey),
		"secret_key":        protectedSecretKey,
		"key_state":         strings.TrimSpace(keyState),
		"backup_access_key": strings.TrimSpace(backupAccessKey),
		"backup_secret_key": protectedBackupSecretKey,
		"backup_key_state":  strings.TrimSpace(backupKeyState),
		"updated_time":      common.GetTimestamp(),
	}).Error
}

func MarkQiniuChildAccountStatus(id int, status string, lastError string) error {
	updates := map[string]interface{}{
		"status":       strings.TrimSpace(status),
		"last_error":   strings.TrimSpace(lastError),
		"updated_time": common.GetTimestamp(),
	}
	if status == QiniuChildAccountStatusDisabled {
		updates["disabled_time"] = common.GetTimestamp()
	}
	return DB.Model(&QiniuChildAccount{}).Where("id = ?", id).Updates(updates).Error
}

func MaskQiniuChildAccountAK(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + strings.Repeat("*", 8) + value[len(value)-4:]
}

func ProtectQiniuChildAccountSecret(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return encryptQiniuChildAccountValue(value)
}

func ProtectQiniuChildAccountLoginPassword(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("七牛子账户登录密码不能为空")
	}
	return encryptQiniuChildAccountValue(value)
}

func encryptQiniuChildAccountValue(value string) (string, error) {
	block, err := aes.NewCipher(qiniuChildAccountCryptoKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	cipherText := gcm.Seal(nil, nonce, []byte(value), nil)
	payload := append(nonce, cipherText...)
	return qiniuChildAccountEncryptedPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func RevealQiniuChildAccountLoginPassword(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("七牛子账户登录密码不能为空")
	}
	if !strings.HasPrefix(value, qiniuChildAccountEncryptedPrefix) {
		// 兼容历史未加密任务，避免失败任务无法继续补偿。
		return value, nil
	}
	return decryptQiniuChildAccountValue(value)
}

func RevealQiniuChildAccountSecret(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("七牛子账户凭证不能为空")
	}
	if strings.HasPrefix(value, "hmac:") {
		return "", errors.New("七牛子账户凭证不可解密")
	}
	if !strings.HasPrefix(value, qiniuChildAccountEncryptedPrefix) {
		// 兼容测试和历史明文凭证；接口输出层仍会脱敏，不直接暴露这里的值。
		return value, nil
	}
	return decryptQiniuChildAccountValue(value)
}

func decryptQiniuChildAccountValue(value string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, qiniuChildAccountEncryptedPrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(qiniuChildAccountCryptoKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) <= gcm.NonceSize() {
		return "", errors.New("七牛子账户登录密码密文无效")
	}
	nonce := raw[:gcm.NonceSize()]
	cipherText := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func qiniuChildAccountCryptoKey() []byte {
	sum := sha256.Sum256([]byte(common.CryptoSecret))
	return sum[:]
}

type AdminQiniuChildAccountQuery struct {
	Id     int
	Status string
	Email  string
	UID    string
}

type AdminQiniuChildAccountTaskSummary struct {
	Id            int
	TaskType      string
	Status        string
	RetryCount    int
	NextRetryTime int64
	LastError     string
	UpdatedTime   int64
}

type AdminQiniuChildAccountListItem struct {
	Account    QiniuChildAccount
	UserCount  int
	LatestTask *AdminQiniuChildAccountTaskSummary
}

func ListAdminQiniuChildAccounts(filter AdminQiniuChildAccountQuery, pageInfo *common.PageInfo) ([]AdminQiniuChildAccountListItem, int64, error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&QiniuChildAccount{})
	if filter.Id > 0 {
		query = query.Where("id = ?", filter.Id)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if strings.TrimSpace(filter.Email) != "" {
		query = query.Where("email LIKE ? ESCAPE '!'", "%"+escapeLikeLiteral(strings.TrimSpace(filter.Email))+"%")
	}
	if strings.TrimSpace(filter.UID) != "" {
		query = query.Where("uid = ?", strings.TrimSpace(filter.UID))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var accounts []QiniuChildAccount
	if err := query.Order("sequence_no desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&accounts).Error; err != nil {
		return nil, 0, err
	}
	accountIds := make([]int, 0, len(accounts))
	for _, account := range accounts {
		accountIds = append(accountIds, account.Id)
	}
	tasks, err := loadAdminQiniuChildAccountLatestTasks(accountIds)
	if err != nil {
		return nil, 0, err
	}
	userCounts, err := loadAdminQiniuChildAccountUserCounts(accountIds)
	if err != nil {
		return nil, 0, err
	}
	items := make([]AdminQiniuChildAccountListItem, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, AdminQiniuChildAccountListItem{
			Account:    account,
			UserCount:  userCounts[account.Id],
			LatestTask: tasks[account.Id],
		})
	}
	return items, total, nil
}

func loadAdminQiniuChildAccountUserCounts(accountIds []int) (map[int]int, error) {
	result := make(map[int]int, len(accountIds))
	if len(accountIds) == 0 {
		return result, nil
	}
	type userCountRow struct {
		QiniuChildAccountId int
		Count               int
	}
	var rows []userCountRow
	if err := DB.Model(&User{}).
		Select("qiniu_child_account_id, COUNT(*) AS count").
		Where("qiniu_child_account_id IN ?", accountIds).
		Group("qiniu_child_account_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.QiniuChildAccountId] = row.Count
	}
	return result, nil
}

func loadAdminQiniuChildAccountLatestTasks(accountIds []int) (map[int]*AdminQiniuChildAccountTaskSummary, error) {
	result := make(map[int]*AdminQiniuChildAccountTaskSummary)
	if len(accountIds) == 0 {
		return result, nil
	}
	type latestTaskRow struct {
		AccountId int
		Id        int
	}
	var rows []latestTaskRow
	if err := DB.Model(&QiniuChildAccountSyncTask{}).
		Select("account_id, MAX(id) AS id").
		Where("account_id IN ?", accountIds).
		Group("account_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	taskIds := make([]int, 0, len(rows))
	for _, row := range rows {
		if row.Id > 0 {
			taskIds = append(taskIds, row.Id)
		}
	}
	if len(taskIds) == 0 {
		return result, nil
	}
	var tasks []QiniuChildAccountSyncTask
	if err := DB.Where("id IN ?", taskIds).Find(&tasks).Error; err != nil {
		return nil, err
	}
	for _, task := range tasks {
		result[task.AccountId] = &AdminQiniuChildAccountTaskSummary{
			Id:            task.Id,
			TaskType:      task.TaskType,
			Status:        task.Status,
			RetryCount:    task.RetryCount,
			NextRetryTime: task.NextRetryTime,
			LastError:     task.LastError,
			UpdatedTime:   task.UpdatedTime,
		}
	}
	return result, nil
}
