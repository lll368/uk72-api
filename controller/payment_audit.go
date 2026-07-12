package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type createPaymentReconciliationTaskRequest struct {
	Provider string                         `json:"provider"`
	DateFrom int64                          `json:"date_from"`
	DateTo   int64                          `json:"date_to"`
	Orders   []service.ProviderPaymentOrder `json:"orders"`
}

type reversePaymentOrderRequest struct {
	Provider string `json:"provider"`
	Reason   string `json:"reason"`
}

// AdminListPaymentCallbackLogs 管理员分页查询支付回调审计日志。
func AdminListPaymentCallbackLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logs, total, err := model.ListPaymentCallbackLogs(c.Query("provider"), c.Query("trade_no"), c.Query("process_status"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

// AdminCreatePaymentReconciliationTask 创建本地支付对账任务。
func AdminCreatePaymentReconciliationTask(c *gin.Context) {
	var req createPaymentReconciliationTaskRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	task, diffs, err := service.ReconcilePaymentOrders(service.ReconcilePaymentOrdersRequest{
		Provider: req.Provider,
		DateFrom: req.DateFrom,
		DateTo:   req.DateTo,
		Orders:   req.Orders,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"task": task, "diffs": diffs})
}

// AdminListPaymentReconciliationTasks 管理员分页查询支付对账任务。
func AdminListPaymentReconciliationTasks(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	tasks, total, err := model.ListPaymentReconciliationTasks(c.Query("provider"), c.Query("status"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasks)
	common.ApiSuccess(c, pageInfo)
}

// AdminReverseTopUpOrder 管理员对普通充值订单做后台冲正。
func AdminReverseTopUpOrder(c *gin.Context) {
	tradeNo := c.Param("trade_no")
	var req reversePaymentOrderRequest
	if !decodeReversePaymentOrderRequest(c, &req) {
		return
	}
	if err := service.ReverseTopUpOrder(tradeNo, req.Provider, req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminReverseVipActivationOrder 管理员对 VVIP 开通订单做后台冲正。
func AdminReverseVipActivationOrder(c *gin.Context) {
	tradeNo := c.Param("trade_no")
	var req reversePaymentOrderRequest
	if !decodeReversePaymentOrderRequest(c, &req) {
		return
	}
	if err := service.ReverseVipActivationOrder(tradeNo, req.Provider, req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminRepairVipActivationSettlement 幂等补齐 VVIP 开通成功订单的资金流水和分佣记录。
func AdminRepairVipActivationSettlement(c *gin.Context) {
	tradeNo := c.Param("trade_no")
	var req reversePaymentOrderRequest
	if err := decodeOptionalJsonRequest(c, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := service.RepairVipActivationSettlement(tradeNo, req.Provider); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func decodeReversePaymentOrderRequest(c *gin.Context, req *reversePaymentOrderRequest) bool {
	if err := decodeOptionalJsonRequest(c, req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return false
	}
	return true
}
