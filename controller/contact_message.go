package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// SubmitContactMessage 提交首页访客留言。
func SubmitContactMessage(c *gin.Context) {
	var req service.ContactMessageSubmitRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.ClientIP = c.ClientIP()
	if _, err := service.SubmitContactMessage(req); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminListContactMessages 管理员分页查询首页访客留言。
func AdminListContactMessages(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := service.ListContactMessages(pageInfo, c.Query("status"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}

// AdminUpdateContactMessage 管理员更新留言状态和备注。
func AdminUpdateContactMessage(c *gin.Context) {
	id, ok := parseContactMessageId(c)
	if !ok {
		return
	}
	var req service.ContactMessageUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	record, err := service.UpdateContactMessage(id, c.GetInt("id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, record)
}

// AdminDeleteContactMessage 管理员删除留言记录。
func AdminDeleteContactMessage(c *gin.Context) {
	id, ok := parseContactMessageId(c)
	if !ok {
		return
	}
	if err := service.DeleteContactMessage(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func parseContactMessageId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return 0, false
	}
	return id, true
}
