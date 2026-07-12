package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminCreateUserRelationRequest struct {
	ParentUserId  int    `json:"parent_user_id"`
	ChildUserId   int    `json:"child_user_id"`
	SourceTradeNo string `json:"source_trade_no"`
	Remark        string `json:"remark"`
}

type adminDisableUserRelationRequest struct {
	Reason string `json:"reason"`
}

type updateVipSubordinateDiscountRequest struct {
	TopupDiscount float64 `json:"topup_discount"`
}

type vipSubordinatePageResponse struct {
	Page                        int                      `json:"page"`
	PageSize                    int                      `json:"page_size"`
	Total                       int                      `json:"total"`
	Items                       []*model.UserSubordinate `json:"items"`
	ParentTopupDiscount         float64                  `json:"parent_topup_discount"`
	CanSetSubordinateDiscount   bool                     `json:"can_set_subordinate_discount"`
	MinSubordinateTopupDiscount float64                  `json:"min_subordinate_topup_discount"`
}

// AdminListUserRelations 管理员分页查询邀请上下级关系。
func AdminListUserRelations(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	parentUserId, _ := strconv.Atoi(c.Query("parent_user_id"))
	childUserId, _ := strconv.Atoi(c.Query("child_user_id"))
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && status != model.UserRelationStatusActive && status != model.UserRelationStatusDisabled {
		common.ApiErrorI18n(c, i18n.MsgUserRelationInvalidStatus)
		return
	}

	relations, total, err := model.ListUserRelations(pageInfo, parentUserId, childUserId, status)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(relations)
	common.ApiSuccess(c, pageInfo)
}

// ListVipSubordinates 查询当前 VVIP 的 active 直接下级用户。
func ListVipSubordinates(c *gin.Context) {
	userId := c.GetInt("id")
	if !ensureCurrentUserActiveVvip(c, userId) {
		return
	}
	parentDiscount, err := model.GetEffectiveUserTopupDiscount(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	subordinates, total, err := model.ListActiveUserSubordinates(pageInfo, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, vipSubordinatePageResponse{
		Page:                        pageInfo.GetPage(),
		PageSize:                    pageInfo.GetPageSize(),
		Total:                       int(total),
		Items:                       subordinates,
		ParentTopupDiscount:         parentDiscount,
		CanSetSubordinateDiscount:   true,
		MinSubordinateTopupDiscount: parentDiscount,
	})
}

// UpdateVipSubordinateDiscount 设置当前 VVIP 直接下级的普通充值折扣。
func UpdateVipSubordinateDiscount(c *gin.Context) {
	userId := c.GetInt("id")
	if !ensureCurrentUserActiveVvip(c, userId) {
		return
	}
	childUserId, ok := parseVipSubordinateChildUserId(c)
	if !ok {
		return
	}
	var req updateVipSubordinateDiscountRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	parentDiscount, err := model.GetEffectiveUserTopupDiscount(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ValidateSubordinateTopupDiscount(parentDiscount, req.TopupDiscount); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	relation, err := model.UpdateDirectSubordinateTopupDiscount(userId, childUserId, req.TopupDiscount)
	if err != nil {
		if errors.Is(err, model.ErrUserRelationNotFound) {
			common.ApiErrorMsg(c, "subordinate relation not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(childUserId, model.LogTypeManage, "算力伙伴 调整下级充值折扣", map[string]interface{}{
		"parent_user_id": userId,
		"child_user_id":  childUserId,
		"relation_id":    relation.Id,
		"topup_discount": req.TopupDiscount,
		"caller_ip":      c.ClientIP(),
	})
	common.ApiSuccess(c, relation)
}

// ResetVipSubordinateDiscount 清空当前 VVIP 直接下级的普通充值折扣。
func ResetVipSubordinateDiscount(c *gin.Context) {
	userId := c.GetInt("id")
	if !ensureCurrentUserActiveVvip(c, userId) {
		return
	}
	childUserId, ok := parseVipSubordinateChildUserId(c)
	if !ok {
		return
	}
	relation, err := model.ResetDirectSubordinateTopupDiscount(userId, childUserId)
	if err != nil {
		if errors.Is(err, model.ErrUserRelationNotFound) {
			common.ApiErrorMsg(c, "subordinate relation not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(childUserId, model.LogTypeManage, "算力伙伴 重置下级充值折扣", map[string]interface{}{
		"parent_user_id": userId,
		"child_user_id":  childUserId,
		"relation_id":    relation.Id,
		"caller_ip":      c.ClientIP(),
	})
	common.ApiSuccess(c, relation)
}

func ensureCurrentUserActiveVvip(c *gin.Context, userId int) bool {
	active, err := model.IsUserActiveVvip(userId)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if !active {
		common.ApiErrorMsg(c, "current user must be active Compute Partner")
		return false
	}
	return true
}

func parseVipSubordinateChildUserId(c *gin.Context) (int, bool) {
	childUserId, err := strconv.Atoi(c.Param("child_user_id"))
	if err != nil || childUserId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return 0, false
	}
	return childUserId, true
}

// AdminCreateUserRelation 管理员手动绑定有效邀请关系。
func AdminCreateUserRelation(c *gin.Context) {
	var req adminCreateUserRelationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if req.ParentUserId <= 0 || req.ChildUserId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgUserRelationInvalidUserId)
		return
	}
	if req.ParentUserId == req.ChildUserId {
		common.ApiErrorI18n(c, i18n.MsgUserRelationSelfBinding)
		return
	}
	parentAvailable, err := isRelationUserAvailable(req.ParentUserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !parentAvailable {
		common.ApiErrorI18n(c, i18n.MsgUserRelationUserNotAvailable)
		return
	}
	childAvailable, err := isRelationUserAvailable(req.ChildUserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !childAvailable {
		common.ApiErrorI18n(c, i18n.MsgUserRelationUserNotAvailable)
		return
	}

	active, err := model.IsUserActiveVvip(req.ParentUserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !active {
		common.ApiErrorI18n(c, i18n.MsgUserRelationParentMustActiveVvip)
		return
	}

	relation, err := model.CreateActiveUserRelationTx(model.DB, req.ParentUserId, req.ChildUserId, model.UserRelationSourceAdmin, strings.TrimSpace(req.SourceTradeNo))
	if err != nil {
		switch err {
		case model.ErrUserRelationSelfBinding:
			common.ApiErrorI18n(c, i18n.MsgUserRelationSelfBinding)
			return
		case model.ErrUserRelationAlreadyBound:
			common.ApiErrorI18n(c, i18n.MsgUserRelationAlreadyBound)
			return
		case model.ErrUserRelationCycle:
			common.ApiErrorI18n(c, i18n.MsgUserRelationCycle)
			return
		}
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(req.ChildUserId, model.LogTypeManage, "管理员绑定邀请关系", map[string]interface{}{
		"admin_user_id":  c.GetInt("id"),
		"caller_ip":      c.ClientIP(),
		"relation_id":    relation.Id,
		"parent_user_id": req.ParentUserId,
		"child_user_id":  req.ChildUserId,
		"remark":         strings.TrimSpace(req.Remark),
	})
	common.ApiSuccess(c, relation)
}

// AdminDisableUserRelation 管理员禁用有效邀请关系，保留历史记录。
func AdminDisableUserRelation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	var req adminDisableUserRelationRequest
	if !decodeAdminDisableUserRelationRequest(c, &req) {
		return
	}
	relation, err := model.DisableUserRelationById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(relation.ChildUserId, model.LogTypeManage, "管理员禁用邀请关系", map[string]interface{}{
		"admin_user_id":  c.GetInt("id"),
		"caller_ip":      c.ClientIP(),
		"relation_id":    relation.Id,
		"parent_user_id": relation.ParentUserId,
		"child_user_id":  relation.ChildUserId,
		"reason":         strings.TrimSpace(req.Reason),
	})
	common.ApiSuccess(c, nil)
}

// decodeAdminDisableUserRelationRequest 允许空请求体，但非空请求体必须是合法 JSON，避免错误请求继续执行禁用操作。
func decodeAdminDisableUserRelationRequest(c *gin.Context, req *adminDisableUserRelationRequest) bool {
	if err := decodeOptionalJsonRequest(c, req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return false
	}
	return true
}

func isRelationUserAvailable(userId int) (bool, error) {
	var user model.User
	err := model.DB.Select("id", "status").Where("id = ?", userId).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return user.Status == common.UserStatusEnabled, nil
}
