package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	qiniuChildAccountBindingDefaultLimit = 100
	qiniuChildAccountBindingMaxLimit     = 500
)

// QiniuChildAccountTokenSummary 是后台展示子账号关联 Token 的脱敏摘要，不包含原始 Key。
type QiniuChildAccountTokenSummary struct {
	Id                  int    `json:"id"`
	UserId              int    `json:"user_id"`
	Username            string `json:"username"`
	DisplayName         string `json:"display_name"`
	Email               string `json:"email"`
	Name                string `json:"name"`
	Status              int    `json:"status"`
	QiniuChildAccountId int    `json:"qiniu_child_account_id"`
	KeyFingerprint      string `json:"key_fingerprint"`
	RemoteCleanupResult string `json:"remote_cleanup_result"`
	CreatedTime         int64  `json:"created_time"`
	AccessedTime        int64  `json:"accessed_time"`
	ExpiredTime         int64  `json:"expired_time"`
	Deleted             bool   `json:"deleted"`
	DeletedTime         int64  `json:"deleted_time"`
}

type QiniuChildAccountTokenCountSummary struct {
	NonDeletedTokenCount int64
	EnabledTokenCount    int64
}

func ListUsersByQiniuChildAccountId(childAccountId int, limit int) ([]User, error) {
	return ListUsersByQiniuChildAccountIdWithTx(DB, childAccountId, limit)
}

func CountUsersByQiniuChildAccountId(childAccountId int) (int64, error) {
	return CountUsersByQiniuChildAccountIdWithTx(DB, childAccountId)
}

func CountUsersByQiniuChildAccountIdWithTx(tx *gorm.DB, childAccountId int) (int64, error) {
	if childAccountId < 0 {
		return 0, errors.New("七牛子账号 ID 无效")
	}
	tx = qiniuChildAccountBindingDB(tx)
	var count int64
	err := tx.Model(&User{}).Where("qiniu_child_account_id = ?", childAccountId).Count(&count).Error
	return count, err
}

func ListUsersByQiniuChildAccountIdWithTx(tx *gorm.DB, childAccountId int, limit int) ([]User, error) {
	if childAccountId < 0 {
		return nil, errors.New("七牛子账号 ID 无效")
	}
	tx = qiniuChildAccountBindingDB(tx)
	var users []User
	err := tx.Select("id", "username", "display_name", "email", "status", "qiniu_child_account_id", "created_at").
		Where("qiniu_child_account_id = ?", childAccountId).
		Order("id asc").
		Limit(normalizeQiniuChildAccountBindingLimit(limit)).
		Find(&users).Error
	return users, err
}

func ListTokensByQiniuChildAccountId(childAccountId int, includeDeleted bool, limit int) ([]Token, error) {
	return ListTokensByQiniuChildAccountIdWithTx(DB, childAccountId, includeDeleted, limit)
}

func ListTokensByQiniuChildAccountIdWithTx(tx *gorm.DB, childAccountId int, includeDeleted bool, limit int) ([]Token, error) {
	if childAccountId < 0 {
		return nil, errors.New("七牛子账号 ID 无效")
	}
	tx = qiniuChildAccountBindingDB(tx)
	if includeDeleted {
		tx = tx.Unscoped()
	}
	var tokens []Token
	err := tx.Where("provider = ? AND qiniu_child_account_id = ?", TokenProviderQiniu, childAccountId).
		Order("id asc").
		Limit(normalizeQiniuChildAccountBindingLimit(limit)).
		Find(&tokens).Error
	return tokens, err
}

func CountNonDeletedQiniuManagedTokensByChildAccountId(childAccountId int) (int64, error) {
	return CountNonDeletedQiniuManagedTokensByChildAccountIdWithTx(DB, childAccountId)
}

func CountNonDeletedQiniuManagedTokensByChildAccountIdWithTx(tx *gorm.DB, childAccountId int) (int64, error) {
	if childAccountId < 0 {
		return 0, errors.New("七牛子账号 ID 无效")
	}
	tx = qiniuChildAccountBindingDB(tx)
	var count int64
	err := tx.Model(&Token{}).
		Where("provider = ? AND qiniu_child_account_id = ?", TokenProviderQiniu, childAccountId).
		Count(&count).Error
	return count, err
}

func CountEnabledQiniuManagedTokensByChildAccountId(childAccountId int) (int64, error) {
	return CountEnabledQiniuManagedTokensByChildAccountIdWithTx(DB, childAccountId)
}

func CountEnabledQiniuManagedTokensByChildAccountIdWithTx(tx *gorm.DB, childAccountId int) (int64, error) {
	if childAccountId < 0 {
		return 0, errors.New("七牛子账号 ID 无效")
	}
	tx = qiniuChildAccountBindingDB(tx)
	var count int64
	err := tx.Model(&Token{}).
		Where("provider = ? AND qiniu_child_account_id = ? AND status = ?", TokenProviderQiniu, childAccountId, common.TokenStatusEnabled).
		Count(&count).Error
	return count, err
}

func CountQiniuManagedTokensByChildAccountIds(childAccountIds []int) (map[int]QiniuChildAccountTokenCountSummary, error) {
	tx := qiniuChildAccountBindingDB(DB)
	result := make(map[int]QiniuChildAccountTokenCountSummary, len(childAccountIds))
	normalizedIds := normalizePositiveIds(childAccountIds)
	if len(normalizedIds) == 0 {
		return result, nil
	}
	type countRow struct {
		QiniuChildAccountId  int
		NonDeletedTokenCount int64
		EnabledTokenCount    int64
	}
	var rows []countRow
	err := tx.Model(&Token{}).
		Select(
			"qiniu_child_account_id, COUNT(*) AS non_deleted_token_count, SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS enabled_token_count",
			common.TokenStatusEnabled,
		).
		Where("provider = ? AND qiniu_child_account_id IN ?", TokenProviderQiniu, normalizedIds).
		Group("qiniu_child_account_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.QiniuChildAccountId] = QiniuChildAccountTokenCountSummary{
			NonDeletedTokenCount: row.NonDeletedTokenCount,
			EnabledTokenCount:    row.EnabledTokenCount,
		}
	}
	return result, nil
}

func HasReusableQiniuRemoteCleanupForToken(tokenId int) (bool, error) {
	return HasReusableQiniuRemoteCleanupForTokenWithTx(DB, tokenId)
}

func HasReusableQiniuRemoteCleanupForTokenWithTx(tx *gorm.DB, tokenId int) (bool, error) {
	if tokenId <= 0 {
		return false, errors.New("token ID 无效")
	}
	tx = qiniuChildAccountBindingDB(tx)
	var count int64
	err := tx.Model(&QiniuKeySyncTask{}).
		Where("token_id = ? AND task_type = ? AND remote_cleanup_result IN ?", tokenId, QiniuKeyTaskTypeRevoke, qiniuReusableRemoteCleanupResults()).
		Count(&count).Error
	return count > 0, err
}

func CountRemoteCleanupPendingSoftDeletedQiniuTokensByChildAccountId(childAccountId int) (int64, error) {
	return CountRemoteCleanupPendingSoftDeletedQiniuTokensByChildAccountIdWithTx(DB, childAccountId)
}

func CountRemoteCleanupPendingSoftDeletedQiniuTokensByChildAccountIdWithTx(tx *gorm.DB, childAccountId int) (int64, error) {
	if childAccountId < 0 {
		return 0, errors.New("七牛子账号 ID 无效")
	}
	tx = qiniuChildAccountBindingDB(tx)
	var tokens []Token
	if err := tx.Unscoped().
		Select("id").
		Where("provider = ? AND qiniu_child_account_id = ? AND deleted_at IS NOT NULL", TokenProviderQiniu, childAccountId).
		Order("id asc").
		Find(&tokens).Error; err != nil {
		return 0, err
	}
	var pending int64
	for _, token := range tokens {
		reusable, err := HasReusableQiniuRemoteCleanupForTokenWithTx(tx, token.Id)
		if err != nil {
			return 0, err
		}
		if !reusable {
			pending++
		}
	}
	return pending, nil
}

func ListQiniuChildAccountTokenSummaries(childAccountId int, limit int) ([]QiniuChildAccountTokenSummary, error) {
	return ListQiniuChildAccountTokenSummariesWithTx(DB, childAccountId, limit)
}

func ListQiniuChildAccountTokenSummariesWithTx(tx *gorm.DB, childAccountId int, limit int) ([]QiniuChildAccountTokenSummary, error) {
	tokens, err := ListTokensByQiniuChildAccountIdWithTx(tx, childAccountId, true, limit)
	if err != nil {
		return nil, err
	}
	tx = qiniuChildAccountBindingDB(tx)
	users, err := loadQiniuChildAccountTokenSummaryUsers(tx, tokens)
	if err != nil {
		return nil, err
	}
	cleanupResults, err := loadLatestQiniuRemoteCleanupResultsByTokenIds(tx, tokens)
	if err != nil {
		return nil, err
	}
	summaries := make([]QiniuChildAccountTokenSummary, 0, len(tokens))
	for _, token := range tokens {
		user := users[token.UserId]
		summary := QiniuChildAccountTokenSummary{
			Id:                  token.Id,
			UserId:              token.UserId,
			Username:            user.Username,
			DisplayName:         user.DisplayName,
			Email:               user.Email,
			Name:                token.Name,
			Status:              token.Status,
			QiniuChildAccountId: token.QiniuChildAccountId,
			KeyFingerprint:      QiniuTokenKeyFingerprint(token.Key),
			RemoteCleanupResult: cleanupResults[token.Id],
			CreatedTime:         token.CreatedTime,
			AccessedTime:        token.AccessedTime,
			ExpiredTime:         token.ExpiredTime,
			Deleted:             token.DeletedAt.Valid,
		}
		if token.DeletedAt.Valid {
			summary.DeletedTime = token.DeletedAt.Time.Unix()
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func loadQiniuChildAccountTokenSummaryUsers(tx *gorm.DB, tokens []Token) (map[int]User, error) {
	userIds := make([]int, 0, len(tokens))
	seen := make(map[int]bool, len(tokens))
	for _, token := range tokens {
		if token.UserId <= 0 || seen[token.UserId] {
			continue
		}
		seen[token.UserId] = true
		userIds = append(userIds, token.UserId)
	}
	users := make(map[int]User, len(userIds))
	if len(userIds) == 0 {
		return users, nil
	}
	var rows []User
	if err := tx.Select("id", "username", "display_name", "email").
		Where("id IN ?", userIds).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, user := range rows {
		users[user.Id] = user
	}
	return users, nil
}

func loadLatestQiniuRemoteCleanupResultsByTokenIds(tx *gorm.DB, tokens []Token) (map[int]string, error) {
	tokenIds := make([]int, 0, len(tokens))
	for _, token := range tokens {
		tokenIds = append(tokenIds, token.Id)
	}
	results := make(map[int]string, len(tokenIds))
	if len(tokenIds) == 0 {
		return results, nil
	}
	var tasks []QiniuKeySyncTask
	if err := tx.Select("token_id", "remote_cleanup_result").
		Where("token_id IN ? AND task_type = ? AND remote_cleanup_result IN ?", tokenIds, QiniuKeyTaskTypeRevoke, qiniuReusableRemoteCleanupResults()).
		Order("id desc").
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if _, exists := results[task.TokenId]; exists {
			continue
		}
		results[task.TokenId] = task.RemoteCleanupResult
	}
	return results, nil
}

func qiniuReusableRemoteCleanupResults() []string {
	return []string{QiniuRemoteCleanupResultSuccess, QiniuRemoteCleanupResultIdempotentSuccess}
}

func normalizeQiniuChildAccountBindingLimit(limit int) int {
	if limit <= 0 {
		return qiniuChildAccountBindingDefaultLimit
	}
	if limit > qiniuChildAccountBindingMaxLimit {
		return qiniuChildAccountBindingMaxLimit
	}
	return limit
}

func normalizePositiveIds(ids []int) []int {
	normalizedIds := make([]int, 0, len(ids))
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		normalizedIds = append(normalizedIds, id)
	}
	return normalizedIds
}

func qiniuChildAccountBindingDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return DB
}
