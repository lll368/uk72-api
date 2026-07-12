package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type adminQiniuChildAccountTaskView struct {
	Id            int    `json:"id"`
	AccountId     int    `json:"account_id"`
	TaskType      string `json:"task_type"`
	Status        string `json:"status"`
	RetryCount    int    `json:"retry_count"`
	NextRetryTime int64  `json:"next_retry_time"`
	LastError     string `json:"last_error"`
	CreatedBy     int    `json:"created_by"`
	CreatedTime   int64  `json:"created_time"`
	UpdatedTime   int64  `json:"updated_time"`
	StartedTime   int64  `json:"started_time"`
	CompletedTime int64  `json:"completed_time"`
}

type adminQiniuChildAccountView struct {
	Id              int                              `json:"id"`
	SequenceNo      int                              `json:"sequence_no"`
	Email           string                           `json:"email"`
	RemoteUserID    string                           `json:"remote_user_id"`
	UID             string                           `json:"uid"`
	ParentUID       string                           `json:"parent_uid"`
	AccessKey       string                           `json:"access_key"`
	BackupAccessKey string                           `json:"backup_access_key"`
	KeyState        string                           `json:"key_state"`
	BackupKeyState  string                           `json:"backup_key_state"`
	Status          string                           `json:"status"`
	LastError       string                           `json:"last_error"`
	UserCount       int                              `json:"user_count"`
	LatestTask      *adminQiniuChildAccountTaskView  `json:"latest_task"`
	CreatedBy       int                              `json:"created_by"`
	DisabledBy      int                              `json:"disabled_by"`
	DisabledReason  string                           `json:"disabled_reason"`
	Impact          adminQiniuChildAccountImpactView `json:"impact"`
	CreatedTime     int64                            `json:"created_time"`
	UpdatedTime     int64                            `json:"updated_time"`
	DisabledTime    int64                            `json:"disabled_time"`
}

type adminQiniuChildAccountDetailView struct {
	adminQiniuChildAccountView
	Tasks  []adminQiniuChildAccountTaskView  `json:"tasks"`
	Users  []adminQiniuChildAccountUserView  `json:"users"`
	Tokens []adminQiniuChildAccountTokenView `json:"tokens"`
}

type adminQiniuChildAccountUserView struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type adminQiniuChildAccountTokenView struct {
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

type adminQiniuChildAccountImpactView struct {
	AssociatedUserCount  int64 `json:"associated_user_count"`
	AssociatedTokenCount int64 `json:"associated_token_count"`
	EnabledTokenCount    int64 `json:"enabled_token_count"`
}

type adminQiniuChildAccountDisableRequest struct {
	Reason string `json:"reason"`
}

func AdminListQiniuChildAccounts(c *gin.Context) {
	filter, ok := parseAdminQiniuChildAccountFilter(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListAdminQiniuChildAccounts(filter, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	impacts, err := buildAdminQiniuChildAccountImpacts(items)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]adminQiniuChildAccountView, 0, len(items))
	for _, item := range items {
		view := toAdminQiniuChildAccountView(item)
		view.Impact = impacts[item.Account.Id]
		views = append(views, view)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(views)
	common.ApiSuccess(c, pageInfo)
}

func AdminCreateQiniuChildAccount(c *gin.Context) {
	account, task, err := service.CreateQiniuChildAccount(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	view := toAdminQiniuChildAccountView(model.AdminQiniuChildAccountListItem{
		Account: accountValue(account),
		LatestTask: &model.AdminQiniuChildAccountTaskSummary{
			Id:            task.Id,
			TaskType:      task.TaskType,
			Status:        task.Status,
			RetryCount:    task.RetryCount,
			NextRetryTime: task.NextRetryTime,
			LastError:     task.LastError,
			UpdatedTime:   task.UpdatedTime,
		},
	})
	common.ApiSuccess(c, view)
}

func AdminGetQiniuChildAccount(c *gin.Context) {
	accountId, ok := parseQiniuChildAccountIdParam(c)
	if !ok {
		return
	}
	account, err := model.GetQiniuChildAccountById(accountId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	taskPage, err := model.ListQiniuChildAccountSyncTasks(model.QiniuChildAccountTaskQuery{AccountId: accountId}, &common.PageInfo{Page: 1, PageSize: 20})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	view := adminQiniuChildAccountDetailView{
		adminQiniuChildAccountView: toAdminQiniuChildAccountView(model.AdminQiniuChildAccountListItem{Account: *account}),
		Tasks:                      make([]adminQiniuChildAccountTaskView, 0, len(taskPage.Items)),
		Users:                      []adminQiniuChildAccountUserView{},
		Tokens:                     []adminQiniuChildAccountTokenView{},
	}
	for _, task := range taskPage.Items {
		view.Tasks = append(view.Tasks, toAdminQiniuChildAccountTaskView(task))
	}
	if err := fillAdminQiniuChildAccountAssociations(accountId, &view); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func AdminDisableQiniuChildAccount(c *gin.Context) {
	accountId, ok := parseQiniuChildAccountIdParam(c)
	if !ok {
		return
	}
	req := adminQiniuChildAccountDisableRequest{}
	if c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiErrorMsg(c, "参数错误")
			return
		}
	}
	account, task, err := service.DisableQiniuChildAccount(c.Request.Context(), accountId, c.GetInt("id"), req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	view := toAdminQiniuChildAccountView(model.AdminQiniuChildAccountListItem{
		Account: accountValue(account),
		LatestTask: &model.AdminQiniuChildAccountTaskSummary{
			Id:            task.Id,
			TaskType:      task.TaskType,
			Status:        task.Status,
			RetryCount:    task.RetryCount,
			NextRetryTime: task.NextRetryTime,
			LastError:     task.LastError,
			UpdatedTime:   task.UpdatedTime,
		},
	})
	if impact, impactErr := buildAdminQiniuChildAccountImpact(accountId); impactErr == nil {
		view.Impact = impact
	}
	common.ApiSuccess(c, view)
}

func AdminEnableQiniuChildAccount(c *gin.Context) {
	accountId, ok := parseQiniuChildAccountIdParam(c)
	if !ok {
		return
	}
	account, task, err := service.EnableQiniuChildAccount(c.Request.Context(), accountId, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toAdminQiniuChildAccountView(model.AdminQiniuChildAccountListItem{
		Account: accountValue(account),
		LatestTask: &model.AdminQiniuChildAccountTaskSummary{
			Id:            task.Id,
			TaskType:      task.TaskType,
			Status:        task.Status,
			RetryCount:    task.RetryCount,
			NextRetryTime: task.NextRetryTime,
			LastError:     task.LastError,
			UpdatedTime:   task.UpdatedTime,
		},
	}))
}

func AdminListQiniuChildAccountTasks(c *gin.Context) {
	filter, ok := parseAdminQiniuChildAccountTaskFilter(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	taskPage, err := model.ListQiniuChildAccountSyncTasks(filter, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]adminQiniuChildAccountTaskView, 0, len(taskPage.Items))
	for _, task := range taskPage.Items {
		views = append(views, toAdminQiniuChildAccountTaskView(task))
	}
	pageInfo.SetTotal(int(taskPage.Total))
	pageInfo.SetItems(views)
	common.ApiSuccess(c, pageInfo)
}

func AdminRetryQiniuChildAccountTask(c *gin.Context) {
	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil || taskId <= 0 {
		common.ApiErrorMsg(c, "任务 ID 无效")
		return
	}
	if err := service.RetryQiniuChildAccountTaskById(taskId); err != nil {
		common.ApiError(c, err)
		return
	}
	task, err := model.GetQiniuChildAccountSyncTaskById(taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toAdminQiniuChildAccountTaskView(task))
}

func parseAdminQiniuChildAccountFilter(c *gin.Context) (model.AdminQiniuChildAccountQuery, bool) {
	filter := model.AdminQiniuChildAccountQuery{
		Email: strings.TrimSpace(c.Query("email")),
		UID:   strings.TrimSpace(c.Query("uid")),
	}
	var ok bool
	if filter.Id, ok = parseOptionalIntQuery(c, "id"); !ok {
		return filter, false
	}
	filter.Status = strings.TrimSpace(c.Query("status"))
	if filter.Status != "" && !isValidQiniuChildAccountStatus(filter.Status) {
		common.ApiErrorMsg(c, "status 参数无效")
		return filter, false
	}
	return filter, true
}

func parseAdminQiniuChildAccountTaskFilter(c *gin.Context) (model.QiniuChildAccountTaskQuery, bool) {
	filter := model.QiniuChildAccountTaskQuery{
		TaskType: strings.TrimSpace(c.Query("task_type")),
		Status:   strings.TrimSpace(c.Query("status")),
	}
	var ok bool
	if filter.AccountId, ok = parseOptionalIntQuery(c, "account_id"); !ok {
		return filter, false
	}
	if filter.Status != "" && !isValidQiniuChildAccountTaskStatus(filter.Status) {
		common.ApiErrorMsg(c, "status 参数无效")
		return filter, false
	}
	return filter, true
}

func parseQiniuChildAccountIdParam(c *gin.Context) (int, bool) {
	accountId, err := strconv.Atoi(c.Param("id"))
	if err != nil || accountId <= 0 {
		common.ApiErrorMsg(c, "子账户 ID 无效")
		return 0, false
	}
	return accountId, true
}

func isValidQiniuChildAccountStatus(status string) bool {
	switch status {
	case model.QiniuChildAccountStatusCreating, model.QiniuChildAccountStatusEnabled, model.QiniuChildAccountStatusDisabled, model.QiniuChildAccountStatusFailed:
		return true
	default:
		return false
	}
}

func isValidQiniuChildAccountTaskStatus(status string) bool {
	switch status {
	case model.QiniuChildAccountTaskStatusPending, model.QiniuChildAccountTaskStatusRunning, model.QiniuChildAccountTaskStatusSuccess, model.QiniuChildAccountTaskStatusFailed, model.QiniuChildAccountTaskStatusSkipped:
		return true
	default:
		return false
	}
}

func toAdminQiniuChildAccountView(item model.AdminQiniuChildAccountListItem) adminQiniuChildAccountView {
	account := item.Account
	view := adminQiniuChildAccountView{
		Id:              account.Id,
		SequenceNo:      account.SequenceNo,
		Email:           account.Email,
		RemoteUserID:    account.RemoteUserID,
		UID:             account.UID,
		ParentUID:       account.ParentUID,
		AccessKey:       model.MaskQiniuChildAccountAK(account.AccessKey),
		BackupAccessKey: model.MaskQiniuChildAccountAK(account.BackupAccessKey),
		KeyState:        account.KeyState,
		BackupKeyState:  account.BackupKeyState,
		Status:          account.Status,
		LastError:       service.SanitizeQiniuChildAccountSecret(account.LastError),
		UserCount:       item.UserCount,
		CreatedBy:       account.CreatedBy,
		DisabledBy:      account.DisabledBy,
		DisabledReason:  account.DisabledReason,
		CreatedTime:     account.CreatedTime,
		UpdatedTime:     account.UpdatedTime,
		DisabledTime:    account.DisabledTime,
	}
	if item.LatestTask != nil {
		view.LatestTask = &adminQiniuChildAccountTaskView{
			Id:            item.LatestTask.Id,
			AccountId:     account.Id,
			TaskType:      item.LatestTask.TaskType,
			Status:        item.LatestTask.Status,
			RetryCount:    item.LatestTask.RetryCount,
			NextRetryTime: item.LatestTask.NextRetryTime,
			LastError:     service.SanitizeQiniuChildAccountSecret(item.LatestTask.LastError),
			UpdatedTime:   item.LatestTask.UpdatedTime,
		}
	}
	return view
}

func toAdminQiniuChildAccountTaskView(task *model.QiniuChildAccountSyncTask) adminQiniuChildAccountTaskView {
	if task == nil {
		return adminQiniuChildAccountTaskView{}
	}
	return adminQiniuChildAccountTaskView{
		Id:            task.Id,
		AccountId:     task.AccountId,
		TaskType:      task.TaskType,
		Status:        task.Status,
		RetryCount:    task.RetryCount,
		NextRetryTime: task.NextRetryTime,
		LastError:     service.SanitizeQiniuChildAccountSecret(task.LastError),
		CreatedBy:     task.CreatedBy,
		CreatedTime:   task.CreatedTime,
		UpdatedTime:   task.UpdatedTime,
		StartedTime:   task.StartedTime,
		CompletedTime: task.CompletedTime,
	}
}

func fillAdminQiniuChildAccountAssociations(accountId int, view *adminQiniuChildAccountDetailView) error {
	if view == nil {
		return nil
	}
	users, err := model.ListUsersByQiniuChildAccountId(accountId, 20)
	if err != nil {
		return err
	}
	for _, user := range users {
		view.Users = append(view.Users, adminQiniuChildAccountUserView{
			Id:          user.Id,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Email:       user.Email,
		})
	}
	tokens, err := model.ListQiniuChildAccountTokenSummaries(accountId, 20)
	if err != nil {
		return err
	}
	for _, token := range tokens {
		view.Tokens = append(view.Tokens, adminQiniuChildAccountTokenView{
			Id:                  token.Id,
			UserId:              token.UserId,
			Username:            token.Username,
			DisplayName:         token.DisplayName,
			Email:               token.Email,
			Name:                token.Name,
			Status:              token.Status,
			QiniuChildAccountId: token.QiniuChildAccountId,
			KeyFingerprint:      token.KeyFingerprint,
			RemoteCleanupResult: token.RemoteCleanupResult,
			CreatedTime:         token.CreatedTime,
			AccessedTime:        token.AccessedTime,
			ExpiredTime:         token.ExpiredTime,
			Deleted:             token.Deleted,
			DeletedTime:         token.DeletedTime,
		})
	}
	impact, err := buildAdminQiniuChildAccountImpact(accountId)
	if err != nil {
		return err
	}
	view.Impact = impact
	view.UserCount = int(impact.AssociatedUserCount)
	return nil
}

func buildAdminQiniuChildAccountImpact(accountId int) (adminQiniuChildAccountImpactView, error) {
	userCount, err := model.CountUsersByQiniuChildAccountId(accountId)
	if err != nil {
		return adminQiniuChildAccountImpactView{}, err
	}
	tokenCount, err := model.CountNonDeletedQiniuManagedTokensByChildAccountId(accountId)
	if err != nil {
		return adminQiniuChildAccountImpactView{}, err
	}
	enabledTokenCount, err := model.CountEnabledQiniuManagedTokensByChildAccountId(accountId)
	if err != nil {
		return adminQiniuChildAccountImpactView{}, err
	}
	return adminQiniuChildAccountImpactView{
		AssociatedUserCount:  userCount,
		AssociatedTokenCount: tokenCount,
		EnabledTokenCount:    enabledTokenCount,
	}, nil
}

func buildAdminQiniuChildAccountImpacts(items []model.AdminQiniuChildAccountListItem) (map[int]adminQiniuChildAccountImpactView, error) {
	impacts := make(map[int]adminQiniuChildAccountImpactView, len(items))
	accountIds := make([]int, 0, len(items))
	for _, item := range items {
		if item.Account.Id <= 0 {
			continue
		}
		accountIds = append(accountIds, item.Account.Id)
		impacts[item.Account.Id] = adminQiniuChildAccountImpactView{
			AssociatedUserCount: int64(item.UserCount),
		}
	}
	tokenCounts, err := model.CountQiniuManagedTokensByChildAccountIds(accountIds)
	if err != nil {
		return nil, err
	}
	for accountId, tokenCount := range tokenCounts {
		impact := impacts[accountId]
		impact.AssociatedTokenCount = tokenCount.NonDeletedTokenCount
		impact.EnabledTokenCount = tokenCount.EnabledTokenCount
		impacts[accountId] = impact
	}
	return impacts, nil
}

func accountValue(account *model.QiniuChildAccount) model.QiniuChildAccount {
	if account == nil {
		return model.QiniuChildAccount{}
	}
	return *account
}
