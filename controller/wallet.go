package controller

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type commissionTransferRequest struct {
	Amount float64 `json:"amount"`
}

type withdrawSubmitRequest struct {
	Amount         float64 `json:"amount"`
	FeeAmount      float64 `json:"fee_amount"`
	ReceiveType    string  `json:"receive_type"`
	ReceiveAccount string  `json:"receive_account"`
	Remark         string  `json:"remark"`
}

type adminWithdrawReviewRequest struct {
	Reason         string `json:"reason"`
	Remark         string `json:"remark"`
	PaymentVoucher string `json:"payment_voucher"`
}

type piggyWithdrawSubmitRequest struct {
	Amount float64 `json:"amount"`
	Remark string  `json:"remark"`
}

type piggyTaxTrialRequest struct {
	Amount float64 `json:"amount"`
}

type adminPiggyManualRequest struct {
	Result             string `json:"result"`
	CompensationStatus string `json:"compensation_status"`
	Reason             string `json:"reason"`
}

func GetWalletAccount(c *gin.Context) {
	userId := c.GetInt("id")
	account, err := service.GetOrCreateWalletAccount(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"account":                        account,
		"commission_min_withdraw_amount": operation_setting.GetPaymentSetting().CommissionMinWithdrawAmount,
	})
}

func GetWalletFlows(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.WalletFlow{}).Where("user_id = ?", userId)
	flowType := strings.TrimSpace(c.Query("flow_type"))
	if flowType != "" {
		query = query.Where("flow_type = ?", flowType)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var flows []model.WalletFlow
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&flows).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(flows)
	common.ApiSuccess(c, pageInfo)
}

func GetWalletCommissions(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.CommissionRecord{}).Where("beneficiary_user_id = ?", userId)
	status := strings.TrimSpace(c.Query("status"))
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var records []model.CommissionRecord
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&records).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}

func TransferWalletCommission(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req commissionTransferRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := service.TransferCommissionToBalance(c.GetInt("id"), req.Amount); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func SubmitWalletWithdraw(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req withdrawSubmitRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	order, err := service.SubmitWithdrawOrder(service.SubmitWithdrawOrderRequest{
		UserId:         c.GetInt("id"),
		Amount:         req.Amount,
		FeeAmount:      req.FeeAmount,
		ReceiveType:    req.ReceiveType,
		ReceiveAccount: req.ReceiveAccount,
		Remark:         req.Remark,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, order)
}

func GetWalletWithdraws(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.WithdrawOrder{}).Where("user_id = ?", userId)
	status := strings.TrimSpace(c.Query("status"))
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var orders []model.WithdrawOrder
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&orders).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(orders)
	common.ApiSuccess(c, pageInfo)
}

func GetWalletWithdrawalProfile(c *gin.Context) {
	profile, err := service.GetWithdrawalProfile(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profile)
}

func SaveWalletWithdrawalProfile(c *gin.Context) {
	var req service.WithdrawalProfileInput
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	profile, err := service.SaveWithdrawalProfile(c.GetInt("id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profile)
}

func GetWalletWithdrawalEligibility(c *gin.Context) {
	eligibility, err := service.GetPiggyWithdrawalEligibility(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, eligibility)
}

func GetWalletPiggySignURL(c *gin.Context) {
	result, err := service.GetPiggyContractSignURL(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func RefreshWalletPiggyContractStatus(c *gin.Context) {
	profile, err := service.RefreshPiggyContractStatus(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profile)
}

func GetWalletPiggyContractPreview(c *gin.Context) {
	result, err := service.GetPiggyContractPreviewURL(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func TrialWalletPiggyWithdrawTax(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req piggyTaxTrialRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	result, err := service.TrialPiggyWithdrawTax(c.Request.Context(), service.PiggyTaxTrialRequest{
		UserId: c.GetInt("id"),
		Amount: req.Amount,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func SubmitWalletPiggyWithdraw(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req piggyWithdrawSubmitRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	order, err := service.SubmitPiggyWithdrawOrder(c.Request.Context(), service.PiggyWithdrawSubmitRequest{
		UserId: c.GetInt("id"),
		Amount: req.Amount,
		Remark: req.Remark,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, order)
}

func PiggyContractNotify(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.HandlePiggyContractCallback(body, firstHeader(c, "sig", "sign")); err != nil {
		common.ApiError(c, err)
		return
	}
	c.String(http.StatusOK, "success")
}

func PiggyPaymentNotify(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.HandlePiggyPaymentCallback(c.Request.Context(), body, firstHeader(c, "sig", "sign")); err != nil {
		common.ApiError(c, err)
		return
	}
	c.String(http.StatusOK, "success")
}

func AdminListWalletCommissions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.CommissionRecord{})
	if userId, _ := strconv.Atoi(c.Query("user_id")); userId > 0 {
		query = query.Where("beneficiary_user_id = ?", userId)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var records []model.CommissionRecord
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&records).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}

func AdminListWalletWithdraws(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.WithdrawOrder{})
	if userId, _ := strconv.Atoi(c.Query("user_id")); userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var orders []model.WithdrawOrder
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&orders).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(orders)
	common.ApiSuccess(c, pageInfo)
}

func AdminListPiggyWithdrawCallbacks(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logs, total, err := service.AdminListPiggyCallbackLogs(c.Query("callback_type"), c.Query("order_no"), c.Query("process_status"), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func AdminRecordPiggyWithdrawManualResult(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var req adminPiggyManualRequest
	if err := decodeOptionalJsonRequest(c, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := service.AdminRecordPiggyManualResult(id, c.GetInt("id"), req.Result, req.CompensationStatus); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminRetryPiggyWithdrawConfirm(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := service.AdminRetryPiggyConfirm(c.Request.Context(), id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminRecoverPiggyWithdrawSubmit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	result, err := service.AdminRecoverPiggyApprovedSubmissionWithResult(c.Request.Context(), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminCancelPiggyWithdraw(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var req adminPiggyManualRequest
	if err := decodeOptionalJsonRequest(c, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := service.AdminCancelPiggyOrder(c.Request.Context(), id, c.GetInt("id"), firstNonEmptyString(req.Reason, req.Result, "管理员取消小猪提现")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminScanPiggyWithdrawCompensations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	processed, err := service.ScanPiggyWithdrawCompensations(c.Request.Context(), limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"processed": processed})
}

func AdminApproveWalletWithdraw(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var req adminWithdrawReviewRequest
	if err := decodeOptionalJsonRequest(c, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	result, err := service.AdminApproveWithdrawOrderWithResult(c.Request.Context(), id, c.GetInt("id"), req.Remark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminRejectWalletWithdraw(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var req adminWithdrawReviewRequest
	if err := decodeOptionalJsonRequest(c, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := service.AdminRejectWithdrawOrder(c.Request.Context(), id, c.GetInt("id"), req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminPayWalletWithdraw(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var req adminWithdrawReviewRequest
	if err := decodeOptionalJsonRequest(c, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := service.MarkWithdrawOrderPaid(id, c.GetInt("id"), req.PaymentVoucher, req.Remark); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminFailWalletWithdraw(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var req adminWithdrawReviewRequest
	if err := decodeOptionalJsonRequest(c, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := service.MarkWithdrawOrderFailed(id, c.GetInt("id"), req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func firstHeader(c *gin.Context, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(c.GetHeader(name)); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
