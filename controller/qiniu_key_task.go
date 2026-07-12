package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func AdminListQiniuKeyTasks(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := 0
	if rawUserId := c.Query("user_id"); rawUserId != "" {
		parsed, err := strconv.Atoi(rawUserId)
		if err != nil {
			common.ApiErrorMsg(c, "user_id 参数无效")
			return
		}
		userId = parsed
	}
	tasks, total, err := model.ListQiniuKeySyncTasks(userId, c.Query("task_type"), c.Query("status"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for _, task := range tasks {
		if task != nil {
			task.QiniuKey = model.MaskTokenKey(task.QiniuKey)
		}
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasks)
	common.ApiSuccess(c, pageInfo)
}

func AdminRetryQiniuKeyTask(c *gin.Context) {
	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "任务 ID 无效")
		return
	}
	if err := service.RetryQiniuKeyTaskById(taskId); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func AdminScanQiniuKeyTasks(c *gin.Context) {
	limit := 100
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			common.ApiErrorMsg(c, "limit 参数无效")
			return
		}
		limit = parsed
	}
	result, err := service.RetryDueQiniuKeyTasks(limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}
