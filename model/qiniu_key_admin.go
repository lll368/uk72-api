package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type AdminQiniuKeyQuery struct {
	UserId              int
	TokenId             int
	Status              int
	QiniuChildAccountId int
	KeyFragment         string
	IncludeDeleted      bool
}

type AdminQiniuKeyOwner struct {
	Id          int
	Username    string
	DisplayName string
	Email       string
}

type AdminQiniuKeyQuotaSummary struct {
	AppliedLimitAmount float64
	PendingLimitAmount float64
	FailedLimitAmount  float64
	LatestGrantError   string
}

type AdminQiniuKeyTaskSummary struct {
	Id            int
	TaskType      string
	Status        string
	RetryCount    int
	NextRetryTime int64
	LastError     string
	UpdatedTime   int64
}

type AdminQiniuKeyChildAccount struct {
	Id     int
	Email  string
	UID    string
	Status string
}

type AdminQiniuKeyListItem struct {
	Token        Token
	User         *AdminQiniuKeyOwner
	Quota        AdminQiniuKeyQuotaSummary
	LatestTask   *AdminQiniuKeyTaskSummary
	ChildAccount *AdminQiniuKeyChildAccount
}

func ListAdminQiniuKeys(filter AdminQiniuKeyQuery, pageInfo *common.PageInfo) ([]AdminQiniuKeyListItem, int64, error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&Token{})
	if filter.IncludeDeleted {
		query = DB.Unscoped().Model(&Token{})
	}
	query = query.Where("provider = ?", TokenProviderQiniu)
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.TokenId > 0 {
		query = query.Where("id = ?", filter.TokenId)
	}
	if filter.Status > 0 {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.QiniuChildAccountId > 0 {
		query = query.Where("qiniu_child_account_id = ?", filter.QiniuChildAccountId)
	}
	query = applyAdminQiniuKeyFragmentFilter(query, filter.KeyFragment)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tokens []Token
	if err := query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&tokens).Error; err != nil {
		return nil, 0, err
	}
	if len(tokens) == 0 {
		return []AdminQiniuKeyListItem{}, total, nil
	}

	tokenIds := make([]int, 0, len(tokens))
	userIds := make([]int, 0, len(tokens))
	childAccountIds := make([]int, 0, len(tokens))
	seenChildAccountIds := make(map[int]bool, len(tokens))
	for _, token := range tokens {
		tokenIds = append(tokenIds, token.Id)
		userIds = append(userIds, token.UserId)
		if token.QiniuChildAccountId > 0 && !seenChildAccountIds[token.QiniuChildAccountId] {
			seenChildAccountIds[token.QiniuChildAccountId] = true
			childAccountIds = append(childAccountIds, token.QiniuChildAccountId)
		}
	}
	users, err := loadAdminQiniuKeyOwners(userIds)
	if err != nil {
		return nil, 0, err
	}
	quotas, err := loadAdminQiniuKeyQuotaSummaries(tokenIds)
	if err != nil {
		return nil, 0, err
	}
	tasks, err := loadAdminQiniuKeyLatestTasks(tokenIds)
	if err != nil {
		return nil, 0, err
	}
	childAccounts, err := loadAdminQiniuKeyChildAccounts(childAccountIds)
	if err != nil {
		return nil, 0, err
	}

	items := make([]AdminQiniuKeyListItem, 0, len(tokens))
	for _, token := range tokens {
		item := AdminQiniuKeyListItem{
			Token:        token,
			User:         users[token.UserId],
			Quota:        quotas[token.Id],
			LatestTask:   tasks[token.Id],
			ChildAccount: childAccounts[token.QiniuChildAccountId],
		}
		items = append(items, item)
	}
	return items, total, nil
}

func loadAdminQiniuKeyChildAccounts(accountIds []int) (map[int]*AdminQiniuKeyChildAccount, error) {
	accounts := make(map[int]*AdminQiniuKeyChildAccount, len(accountIds))
	if len(accountIds) == 0 {
		return accounts, nil
	}
	var rows []QiniuChildAccount
	if err := DB.Select("id", "email", "uid", "status").Where("id IN ?", accountIds).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, account := range rows {
		accounts[account.Id] = &AdminQiniuKeyChildAccount{
			Id:     account.Id,
			Email:  account.Email,
			UID:    account.UID,
			Status: account.Status,
		}
	}
	return accounts, nil
}

func normalizeAdminQiniuKeyFragment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "sk-")
	value = strings.ReplaceAll(value, "*", "")
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func applyAdminQiniuKeyFragmentFilter(query *gorm.DB, value string) *gorm.DB {
	if !strings.Contains(value, "*") {
		keyFragment := normalizeAdminQiniuKeyFragment(value)
		if keyFragment == "" {
			return query
		}
		pattern := "%" + escapeLikeLiteral(keyFragment) + "%"
		return query.Where(commonKeyCol+" LIKE ? ESCAPE '!'", pattern)
	}
	prefix, suffix := splitAdminQiniuMaskedKeyFragment(value)
	if prefix != "" {
		prefixPattern := escapeLikeLiteral(prefix) + "%"
		prefixedPattern := "sk-" + escapeLikeLiteral(prefix) + "%"
		query = query.Where("("+commonKeyCol+" LIKE ? ESCAPE '!' OR "+commonKeyCol+" LIKE ? ESCAPE '!')", prefixPattern, prefixedPattern)
	}
	if suffix != "" {
		query = query.Where(commonKeyCol+" LIKE ? ESCAPE '!'", "%"+escapeLikeLiteral(suffix))
	}
	return query
}

func splitAdminQiniuMaskedKeyFragment(value string) (string, string) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "sk-")
	value = strings.ReplaceAll(value, " ", "")
	firstMaskIdx := strings.Index(value, "*")
	if firstMaskIdx < 0 {
		return "", ""
	}
	lastMaskIdx := strings.LastIndex(value, "*")
	prefix := strings.ReplaceAll(value[:firstMaskIdx], "*", "")
	suffix := strings.ReplaceAll(value[lastMaskIdx+1:], "*", "")
	return prefix, suffix
}

func loadAdminQiniuKeyOwners(userIds []int) (map[int]*AdminQiniuKeyOwner, error) {
	owners := make(map[int]*AdminQiniuKeyOwner)
	if len(userIds) == 0 {
		return owners, nil
	}
	var users []User
	if err := DB.Select("id", "username", "display_name", "email").Where("id IN ?", userIds).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		owners[user.Id] = &AdminQiniuKeyOwner{
			Id:          user.Id,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Email:       user.Email,
		}
	}
	return owners, nil
}

func loadAdminQiniuKeyQuotaSummaries(tokenIds []int) (map[int]AdminQiniuKeyQuotaSummary, error) {
	summaries := make(map[int]AdminQiniuKeyQuotaSummary)
	if len(tokenIds) == 0 {
		return summaries, nil
	}
	type quotaAggregate struct {
		TokenId           int
		RemoteApplyStatus string
		GrantAmount       float64
	}
	var aggregates []quotaAggregate
	if err := DB.Model(&QiniuQuotaGrant{}).
		Select("token_id, remote_apply_status, COALESCE(SUM(grant_amount), 0) AS grant_amount").
		Where("token_id IN ?", tokenIds).
		Group("token_id, remote_apply_status").
		Scan(&aggregates).Error; err != nil {
		return nil, err
	}
	for _, aggregate := range aggregates {
		summary := summaries[aggregate.TokenId]
		switch aggregate.RemoteApplyStatus {
		case QiniuQuotaGrantStatusApplied:
			summary.AppliedLimitAmount = aggregate.GrantAmount
		case QiniuQuotaGrantStatusPending:
			summary.PendingLimitAmount = aggregate.GrantAmount
		case QiniuQuotaGrantStatusFailed:
			summary.FailedLimitAmount = aggregate.GrantAmount
		}
		summaries[aggregate.TokenId] = summary
	}

	var failedGrants []QiniuQuotaGrant
	type latestGrantRow struct {
		TokenId int
		Id      int
	}
	var latestGrantRows []latestGrantRow
	if err := DB.Model(&QiniuQuotaGrant{}).
		Select("token_id, MAX(id) AS id").
		Where("token_id IN ? AND remote_apply_status = ?", tokenIds, QiniuQuotaGrantStatusFailed).
		Group("token_id").
		Scan(&latestGrantRows).Error; err != nil {
		return nil, err
	}
	latestGrantIds := make([]int, 0, len(latestGrantRows))
	for _, row := range latestGrantRows {
		if row.Id > 0 {
			latestGrantIds = append(latestGrantIds, row.Id)
		}
	}
	if len(latestGrantIds) > 0 {
		if err := DB.Select("id", "token_id", "last_error").
			Where("id IN ?", latestGrantIds).
			Find(&failedGrants).Error; err != nil {
			return nil, err
		}
	}
	for _, grant := range failedGrants {
		summary := summaries[grant.TokenId]
		summary.LatestGrantError = grant.LastError
		summaries[grant.TokenId] = summary
	}
	return summaries, nil
}

func loadAdminQiniuKeyLatestTasks(tokenIds []int) (map[int]*AdminQiniuKeyTaskSummary, error) {
	tasksByToken := make(map[int]*AdminQiniuKeyTaskSummary)
	if len(tokenIds) == 0 {
		return tasksByToken, nil
	}
	type latestTaskRow struct {
		TokenId int
		Id      int
	}
	var latestTaskRows []latestTaskRow
	if err := DB.Model(&QiniuKeySyncTask{}).
		Select("token_id, MAX(id) AS id").
		Where("token_id IN ?", tokenIds).
		Group("token_id").
		Scan(&latestTaskRows).Error; err != nil {
		return nil, err
	}
	latestTaskIds := make([]int, 0, len(latestTaskRows))
	for _, row := range latestTaskRows {
		if row.Id > 0 {
			latestTaskIds = append(latestTaskIds, row.Id)
		}
	}
	if len(latestTaskIds) == 0 {
		return tasksByToken, nil
	}
	var tasks []QiniuKeySyncTask
	if err := DB.Where("id IN ?", latestTaskIds).Find(&tasks).Error; err != nil {
		return nil, err
	}
	for _, task := range tasks {
		tasksByToken[task.TokenId] = &AdminQiniuKeyTaskSummary{
			Id:            task.Id,
			TaskType:      task.TaskType,
			Status:        task.Status,
			RetryCount:    task.RetryCount,
			NextRetryTime: task.NextRetryTime,
			LastError:     task.LastError,
			UpdatedTime:   task.UpdatedTime,
		}
	}
	return tasksByToken, nil
}
