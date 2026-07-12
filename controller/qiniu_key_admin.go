package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type adminQiniuKeyOwnerView struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type adminQiniuKeyQuotaView struct {
	AppliedLimitAmount float64 `json:"applied_limit_amount"`
	PendingLimitAmount float64 `json:"pending_limit_amount"`
	FailedLimitAmount  float64 `json:"failed_limit_amount"`
	LatestGrantError   string  `json:"latest_grant_error"`
}

type adminQiniuKeyTaskView struct {
	Id            int    `json:"id"`
	TaskType      string `json:"task_type"`
	Status        string `json:"status"`
	RetryCount    int    `json:"retry_count"`
	NextRetryTime int64  `json:"next_retry_time"`
	LastError     string `json:"last_error"`
	UpdatedTime   int64  `json:"updated_time"`
}

type adminQiniuKeyChildAccountView struct {
	Id     int    `json:"id"`
	Email  string `json:"email"`
	UID    string `json:"uid"`
	Status string `json:"status"`
}

type adminQiniuKeyView struct {
	TokenId             int                            `json:"token_id"`
	UserId              int                            `json:"user_id"`
	Name                string                         `json:"name"`
	Key                 string                         `json:"key"`
	Status              int                            `json:"status"`
	Group               string                         `json:"group"`
	QiniuChildAccountId int                            `json:"qiniu_child_account_id"`
	QiniuChildAccount   *adminQiniuKeyChildAccountView `json:"qiniu_child_account"`
	CreatedTime         int64                          `json:"created_time"`
	AccessedTime        int64                          `json:"accessed_time"`
	Deleted             bool                           `json:"deleted"`
	DeletedTime         int64                          `json:"deleted_time"`
	User                adminQiniuKeyOwnerView         `json:"user"`
	Quota               adminQiniuKeyQuotaView         `json:"quota"`
	LatestTask          *adminQiniuKeyTaskView         `json:"latest_task"`
}

type adminQiniuKeyDisableRequest struct {
	Reason string `json:"reason"`
}

type adminQiniuKeyMutationView struct {
	TokenId int    `json:"token_id"`
	UserId  int    `json:"user_id"`
	Status  int    `json:"status"`
	Key     string `json:"key"`
}

func AdminListQiniuKeys(c *gin.Context) {
	filter, ok := parseAdminQiniuKeyFilter(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListAdminQiniuKeys(filter, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]adminQiniuKeyView, 0, len(items))
	for _, item := range items {
		views = append(views, toAdminQiniuKeyView(item))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(views)
	common.ApiSuccess(c, pageInfo)
}

// AdminDisableQiniuKey 处理管理员禁用七牛托管 Key 请求。
func AdminDisableQiniuKey(c *gin.Context) {
	tokenId, err := strconv.Atoi(c.Param("id"))
	if err != nil || tokenId <= 0 {
		common.ApiErrorMsg(c, "Key ID 无效")
		return
	}
	req := adminQiniuKeyDisableRequest{}
	if c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiErrorMsg(c, "参数错误")
			return
		}
	}
	token, err := service.AdminDisableQiniuKey(c.Request.Context(), tokenId, c.GetInt("id"), req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, adminQiniuKeyMutationView{
		TokenId: token.Id,
		UserId:  token.UserId,
		Status:  token.Status,
		Key:     model.MaskTokenKey(token.Key),
	})
}

func parseAdminQiniuKeyFilter(c *gin.Context) (model.AdminQiniuKeyQuery, bool) {
	filter := model.AdminQiniuKeyQuery{
		KeyFragment: firstNonEmptyQuery(c, "qiniu_key", "key", "key_fragment"),
	}
	var ok bool
	if filter.UserId, ok = parseOptionalIntQuery(c, "user_id"); !ok {
		return filter, false
	}
	if filter.TokenId, ok = parseOptionalIntQuery(c, "token_id"); !ok {
		return filter, false
	}
	if filter.Status, ok = parseOptionalIntQuery(c, "status"); !ok {
		return filter, false
	}
	if filter.QiniuChildAccountId, ok = parseOptionalIntQuery(c, "qiniu_child_account_id"); !ok {
		return filter, false
	}
	includeDeleted, ok := parseOptionalBoolQuery(c, "include_deleted")
	if !ok {
		return filter, false
	}
	filter.IncludeDeleted = includeDeleted
	return filter, true
}

func parseOptionalBoolQuery(c *gin.Context, key string) (bool, bool) {
	value := c.Query(key)
	if value == "" {
		return false, true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		common.ApiErrorMsg(c, key+" 参数无效")
		return false, false
	}
	return parsed, true
}

func toAdminQiniuKeyView(item model.AdminQiniuKeyListItem) adminQiniuKeyView {
	token := item.Token
	view := adminQiniuKeyView{
		TokenId:             token.Id,
		UserId:              token.UserId,
		Name:                token.Name,
		Key:                 token.GetMaskedKey(),
		Status:              token.Status,
		Group:               token.Group,
		QiniuChildAccountId: token.QiniuChildAccountId,
		CreatedTime:         token.CreatedTime,
		AccessedTime:        token.AccessedTime,
		Deleted:             token.DeletedAt.Valid,
		Quota: adminQiniuKeyQuotaView{
			AppliedLimitAmount: item.Quota.AppliedLimitAmount,
			PendingLimitAmount: item.Quota.PendingLimitAmount,
			FailedLimitAmount:  item.Quota.FailedLimitAmount,
			LatestGrantError:   service.SanitizeQiniuOfficialAdminText(item.Quota.LatestGrantError),
		},
	}
	if token.DeletedAt.Valid {
		view.DeletedTime = token.DeletedAt.Time.Unix()
	}
	if item.User != nil {
		view.User = adminQiniuKeyOwnerView{
			Id:          item.User.Id,
			Username:    item.User.Username,
			DisplayName: item.User.DisplayName,
			Email:       item.User.Email,
		}
	}
	if item.ChildAccount != nil {
		view.QiniuChildAccount = &adminQiniuKeyChildAccountView{
			Id:     item.ChildAccount.Id,
			Email:  item.ChildAccount.Email,
			UID:    item.ChildAccount.UID,
			Status: item.ChildAccount.Status,
		}
	}
	if item.LatestTask != nil {
		view.LatestTask = &adminQiniuKeyTaskView{
			Id:            item.LatestTask.Id,
			TaskType:      item.LatestTask.TaskType,
			Status:        item.LatestTask.Status,
			RetryCount:    item.LatestTask.RetryCount,
			NextRetryTime: item.LatestTask.NextRetryTime,
			LastError:     service.SanitizeQiniuOfficialAdminText(item.LatestTask.LastError),
			UpdatedTime:   item.LatestTask.UpdatedTime,
		}
	}
	return view
}
