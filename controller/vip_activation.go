package controller

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/waffo-com/waffo-go/types/order"
)

type vipActivationEpayPayRequest struct {
	PaymentMethod string `json:"payment_method"`
}

type vipActivationCreemPayRequest struct {
	ProductId string `json:"product_id"`
}

type vipActivationWaffoPayRequest struct {
	PayMethodIndex *int   `json:"pay_method_index"`
	PayMethodType  string `json:"pay_method_type"`
	PayMethodName  string `json:"pay_method_name"`
}

type adminDisableVipActivationRequest struct {
	Reason string `json:"reason"`
}

func vipActivationReturnPath() string {
	return paymentReturnPath("/compute-partners")
}

// GetVipActivationInfo 查询当前用户 VVIP 状态、固定开通价格和可用外部支付方式。
func GetVipActivationInfo(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	isVvip, err := model.IsUserActiveVvip(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var activatedAt int64
	var status = model.VipActivationStatusDisabled
	if isVvip {
		status = model.VipActivationStatusSuccess
		if active, err := model.GetActiveVipActivationByUserId(userId); err == nil {
			activatedAt = active.ActivatedAt
		}
		if strings.TrimSpace(user.AffCode) == "" {
			user.AffCode = common.GetRandomString(4)
			_ = user.Update(false)
		}
	} else if latest, err := model.GetLatestVipActivationByUserId(userId); err == nil {
		status = latest.Status
		activatedAt = latest.ActivatedAt
	}

	vipActivationPrice := operation_setting.GetVipActivationPaymentAmount()
	data := gin.H{
		"is_vvip":           isVvip,
		"status":            status,
		"activated_at":      activatedAt,
		"activation_amount": vipActivationPrice,
		"paid_amount":       vipActivationPrice,
		"discount":          model.DefaultVipActivationDiscount,
		"payment_methods":   getVipActivationPaymentMethods(),
		"creem_products":    getVipActivationCreemProducts(),
		"waffo_pay_methods": setting.GetWaffoPayMethods(),
		"aff_code":          "",
		"invite_link":       "",
	}
	if isVvip {
		data["aff_code"] = user.AffCode
		data["invite_link"] = buildVipInviteLink(user.AffCode)
	}
	common.ApiSuccess(c, data)
}

// GetVipActivationOrderStatus 查询当前用户指定 VVIP 开通订单状态。
func GetVipActivationOrderStatus(c *gin.Context) {
	userId := c.GetInt("id")
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	if tradeNo == "" {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}
	record, err := model.GetVipActivationRecordByTradeNo(tradeNo)
	if err != nil {
		if errors.Is(err, model.ErrVipActivationOrderNotFound) {
			common.ApiErrorMsg(c, "订单不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	if record.UserId != userId {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}
	common.ApiSuccess(c, gin.H{
		"trade_no": record.TradeNo,
		"status":   record.Status,
	})
}

// VipActivationRequestEpay 创建易支付 VVIP 一次性开通订单。
func VipActivationRequestEpay(c *gin.Context) {
	if !ensureVipActivationPayAllowed(c) {
		return
	}
	if !requirePaymentCompliance(c) {
		return
	}
	var req vipActivationEpayPayRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if !isEpayTopUpEnabled() {
		common.ApiErrorMsg(c, "当前管理员未配置支付信息")
		return
	}
	if !operation_setting.ContainsPayMethod(req.PaymentMethod) {
		common.ApiErrorMsg(c, "支付方式不存在")
		return
	}
	client := GetEpayClient()
	if client == nil {
		common.ApiErrorMsg(c, "当前管理员未配置支付信息")
		return
	}

	userId := c.GetInt("id")
	vipOrder, err := service.CreateVipActivationOrder(userId, model.PaymentProviderEpay, req.PaymentMethod)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	callbackAddress := service.GetCallbackAddress()
	returnUrl, _ := url.Parse(vipActivationReturnPath())
	notifyUrl, _ := url.Parse(callbackAddress + "/api/vip/epay/notify")
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           req.PaymentMethod,
		ServiceTradeNo: vipOrder.TradeNo,
		Name:           "Compute Partner Activation",
		Money:          strconv.FormatFloat(vipOrder.PaidAmount, 'f', 2, 64),
		Device:         epay.PC,
		NotifyUrl:      notifyUrl,
		ReturnUrl:      returnUrl,
	})
	if err != nil {
		markVipActivationOrderFailed(vipOrder)
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": params, "url": uri})
}

// VipActivationRequestStripe 创建 Stripe VVIP 一次性开通订单。
func VipActivationRequestStripe(c *gin.Context) {
	if !ensureVipActivationPayAllowed(c) {
		return
	}
	if !requirePaymentCompliance(c) {
		return
	}
	if !isStripeVipActivationEnabled() {
		common.ApiErrorMsg(c, "Stripe 未配置或密钥无效")
		return
	}
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	vipOrder, err := service.CreateVipActivationOrder(userId, model.PaymentProviderStripe, model.PaymentMethodStripe)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	payLink, err := genStripeVipActivationLink(vipOrder.TradeNo, user.StripeCustomer, user.Email, vipOrder.PaidAmount)
	if err != nil {
		markVipActivationOrderFailed(vipOrder)
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}
	common.ApiSuccess(c, gin.H{"pay_link": payLink, "order_id": vipOrder.TradeNo})
}

// VipActivationRequestCreem 创建 Creem VVIP 一次性开通订单。
func VipActivationRequestCreem(c *gin.Context) {
	if !ensureVipActivationPayAllowed(c) {
		return
	}
	if !requirePaymentCompliance(c) {
		return
	}
	if !isCreemTopUpEnabled() {
		common.ApiErrorMsg(c, "当前管理员未配置支付信息")
		return
	}
	var req vipActivationCreemPayRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	product, err := resolveVipActivationCreemProduct(req.ProductId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	vipOrder, err := service.CreateVipActivationOrder(userId, model.PaymentProviderCreem, model.PaymentMethodCreem)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	checkoutUrl, err := genCreemLink(c.Request.Context(), vipOrder.TradeNo, product, user.Email, user.Username)
	if err != nil {
		markVipActivationOrderFailed(vipOrder)
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}
	common.ApiSuccess(c, gin.H{"checkout_url": checkoutUrl, "order_id": vipOrder.TradeNo})
}

// VipActivationRequestWaffo 创建 Waffo VVIP 一次性开通订单。
func VipActivationRequestWaffo(c *gin.Context) {
	if !ensureVipActivationPayAllowed(c) {
		return
	}
	if !requirePaymentCompliance(c) {
		return
	}
	if !isWaffoTopUpEnabled() {
		common.ApiErrorMsg(c, "当前管理员未配置支付信息")
		return
	}
	var req vipActivationWaffoPayRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	payMethodType, payMethodName, ok := resolveWaffoPayMethod(req.PayMethodIndex, req.PayMethodType, req.PayMethodName)
	if !ok {
		common.ApiErrorMsg(c, "不支持的支付方式")
		return
	}

	vipOrder, err := service.CreateVipActivationOrder(userId, model.PaymentProviderWaffo, model.PaymentMethodWaffo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sdk, err := getWaffoSDK()
	if err != nil {
		markVipActivationOrderFailed(vipOrder)
		common.ApiErrorMsg(c, "支付配置错误")
		return
	}

	callbackAddress := service.GetCallbackAddress()
	notifyUrl := callbackAddress + "/api/waffo/webhook"
	if setting.WaffoNotifyUrl != "" {
		notifyUrl = setting.WaffoNotifyUrl
	}
	returnUrl := vipActivationReturnPath()
	currency := getWaffoCurrency()
	resp, err := sdk.Order().Create(c.Request.Context(), &order.CreateOrderParams{
		PaymentRequestID: vipOrder.TradeNo,
		MerchantOrderID:  vipOrder.TradeNo,
		OrderAmount:      formatWaffoAmount(vipOrder.PaidAmount, currency),
		OrderCurrency:    currency,
		OrderDescription: "Compute Partner Activation",
		OrderRequestedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		NotifyURL:        notifyUrl,
		MerchantInfo: &order.MerchantInfo{
			MerchantID: setting.WaffoMerchantId,
		},
		UserInfo: &order.UserInfo{
			UserID:       strconv.Itoa(user.Id),
			UserEmail:    getWaffoUserEmail(user),
			UserTerminal: "WEB",
		},
		PaymentInfo: &order.PaymentInfo{
			ProductName:   "VVIP_ACTIVATION",
			PayMethodType: payMethodType,
			PayMethodName: payMethodName,
		},
		SuccessRedirectURL: returnUrl,
		FailedRedirectURL:  returnUrl,
	}, nil)
	if err != nil || !resp.IsSuccess() {
		markVipActivationOrderFailed(vipOrder)
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}
	orderData := resp.Data
	paymentUrl := orderData.FetchRedirectURL()
	if paymentUrl == "" {
		paymentUrl = orderData.OrderAction
	}
	common.ApiSuccess(c, gin.H{"payment_url": paymentUrl, "order_id": vipOrder.TradeNo})
}

// VipActivationEpayNotify 处理 VVIP 易支付异步回调。
func VipActivationEpayNotify(c *gin.Context) {
	if !isEpayWebhookEnabled() {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	params := parseEpayNotifyParams(c)
	if len(params) == 0 {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	auditLog, auditErr := service.CreatePaymentCallbackAudit(service.PaymentCallbackAuditInput{
		Provider:  model.PaymentProviderEpay,
		EventType: "notify",
		BizType:   service.PaymentBizTypeVipActivation,
		Payload:   []byte(common.GetJsonString(params)),
	})
	if auditErr != nil {
		// 审计失败不能阻塞支付渠道回调主流程，但必须留下服务日志便于排查。
		common.SysError("create vip epay callback audit failed: " + auditErr.Error())
	}
	client := GetEpayClient()
	if client == nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, "client not initialized")
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	verifyInfo, err := client.Verify(params)
	if err != nil || !verifyInfo.VerifyStatus || verifyInfo.TradeStatus != epay.StatusTradeSuccess {
		msg := "verify failed or trade not success"
		if err != nil {
			msg = err.Error()
		}
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, msg)
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	_ = service.MarkPaymentCallbackAuditVerified(auditLog, verifyInfo.ServiceTradeNo, "notify", service.PaymentBizTypeVipActivation)

	LockOrder(verifyInfo.ServiceTradeNo)
	defer UnlockOrder(verifyInfo.ServiceTradeNo)
	if err := service.CompleteVipActivationOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo), model.PaymentProviderEpay, verifyInfo.Type); err != nil {
		_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusFailed, err.Error())
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	_ = service.FinishPaymentCallbackAudit(auditLog, model.PaymentProcessStatusSuccess, "")
	_, _ = c.Writer.Write([]byte("success"))
}

// AdminListVipActivationRecords 管理员分页查询 VVIP 开通记录。
func AdminListVipActivationRecords(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListAdminVipActivationRecords(buildAdminVipActivationRecordFilter(c), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}

// AdminDisableVipActivation 管理员禁用用户 VVIP 权限。
func AdminDisableVipActivation(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var req adminDisableVipActivationRequest
	if err := decodeOptionalJsonRequest(c, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := service.DisableVipActivation(userId, c.GetInt("id"), req.Reason, c.ClientIP()); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

type adminManualActivateVvipRequest struct {
	Remark string `json:"remark"`
}

// AdminManualActivateVvip 管理员手动将用户设置为算力伙伴。
func AdminManualActivateVvip(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var req adminManualActivateVvipRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	adminUserId := c.GetInt("id")
	input := service.ManualVvipActivationInput{
		UserId:      userId,
		AdminUserId: adminUserId,
		Remark:      req.Remark,
		CallerIP:    c.ClientIP(),
	}
	if err := service.AdminManualActivateVvip(input); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func parseEpayNotifyParams(c *gin.Context) map[string]string {
	if c.Request.Method == http.MethodPost {
		if err := c.Request.ParseForm(); err != nil {
			return map[string]string{}
		}
		result := make(map[string]string, len(c.Request.PostForm))
		for key := range c.Request.PostForm {
			result[key] = c.Request.PostForm.Get(key)
		}
		return result
	}
	result := make(map[string]string, len(c.Request.URL.Query()))
	for key := range c.Request.URL.Query() {
		result[key] = c.Request.URL.Query().Get(key)
	}
	return result
}

func ensureVipActivationPayAllowed(c *gin.Context) bool {
	isVvip, err := model.IsUserActiveVvip(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if isVvip {
		common.ApiError(c, model.ErrVipActivationAlreadyActive)
		return false
	}
	return true
}

func getVipActivationPaymentMethods() []map[string]string {
	configuredPayMethods := operation_setting.GetPayMethods()
	methods := make([]map[string]string, 0, len(configuredPayMethods)+3)
	if isEpayTopUpEnabled() {
		for _, method := range configuredPayMethods {
			copied := map[string]string{}
			for key, value := range method {
				copied[key] = value
			}
			methods = append(methods, copied)
		}
	}
	if isStripeVipActivationEnabled() {
		methods = append(methods, map[string]string{"name": "Stripe", "type": model.PaymentMethodStripe})
	}
	if isAlipayTopUpEnabled() {
		methods = append(methods, map[string]string{"name": "支付宝直连", "type": model.PaymentMethodAlipayDirect})
	}
	if isWechatPayTopUpEnabled() {
		methods = append(methods, map[string]string{"name": "微信支付直连", "type": model.PaymentMethodWechatDirect})
	}
	if isWaffoTopUpEnabled() {
		methods = append(methods, map[string]string{"name": "Waffo", "type": model.PaymentMethodWaffo})
	}
	if isCreemTopUpEnabled() && len(getVipActivationCreemProducts()) > 0 {
		methods = append(methods, map[string]string{"name": "Creem", "type": model.PaymentMethodCreem})
	}
	return methods
}

func getVipActivationCreemProducts() []CreemProduct {
	var products []CreemProduct
	if strings.TrimSpace(setting.CreemProducts) == "" {
		return products
	}
	if err := common.UnmarshalJsonStr(setting.CreemProducts, &products); err != nil {
		return []CreemProduct{}
	}
	eligible := make([]CreemProduct, 0, len(products))
	vipActivationPrice := operation_setting.GetVipActivationPaymentAmount()
	for _, product := range products {
		if product.Price == vipActivationPrice {
			eligible = append(eligible, product)
		}
	}
	return eligible
}

func resolveVipActivationCreemProduct(productId string) (*CreemProduct, error) {
	products := getVipActivationCreemProducts()
	if len(products) == 0 {
		return nil, fmt.Errorf("未配置价格为 %.2f 的 Creem VVIP 产品", operation_setting.GetVipActivationPaymentAmount())
	}
	if productId == "" && len(products) == 1 {
		return &products[0], nil
	}
	for i := range products {
		if products[i].ProductId == productId {
			return &products[i], nil
		}
	}
	return nil, errors.New("产品不存在")
}

func resolveWaffoPayMethod(index *int, payMethodType string, payMethodName string) (string, string, bool) {
	methods := setting.GetWaffoPayMethods()
	if index != nil {
		if *index < 0 || *index >= len(methods) {
			return "", "", false
		}
		method := methods[*index]
		return method.PayMethodType, method.PayMethodName, true
	}
	payMethodType = strings.TrimSpace(payMethodType)
	payMethodName = strings.TrimSpace(payMethodName)
	if payMethodType == "" && payMethodName == "" {
		return "", "", true
	}
	for _, method := range methods {
		if method.PayMethodType == payMethodType && method.PayMethodName == payMethodName {
			return payMethodType, payMethodName, true
		}
	}
	return "", "", false
}

func isStripeVipActivationEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	return strings.TrimSpace(setting.StripeApiSecret) != "" &&
		strings.TrimSpace(setting.StripeWebhookSecret) != ""
}

func genStripeVipActivationLink(referenceId string, customerId string, email string, paidAmount float64) (string, error) {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return "", fmt.Errorf("无效的Stripe API密钥")
	}
	if paidAmount <= 0 {
		paidAmount = operation_setting.GetVipActivationPaymentAmount()
	}
	unitAmount, err := stripeVipActivationUnitAmount(paidAmount)
	if err != nil {
		return "", err
	}
	stripe.Key = setting.StripeApiSecret
	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceId),
		SuccessURL:        stripe.String(vipActivationReturnPath()),
		CancelURL:         stripe.String(vipActivationReturnPath()),
		Metadata: map[string]string{
			"trade_no": referenceId,
			"biz_type": service.PaymentBizTypeVipActivation,
		},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: map[string]string{
				"trade_no": referenceId,
				"biz_type": service.PaymentBizTypeVipActivation,
			},
		},
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("VVIP Activation"),
					},
					UnitAmount: stripe.Int64(unitAmount),
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
	}
	if customerId == "" {
		if email != "" {
			params.CustomerEmail = stripe.String(email)
		}
		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerId)
	}
	result, err := session.New(params)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func stripeVipActivationUnitAmount(paidAmount float64) (int64, error) {
	if math.IsNaN(paidAmount) || math.IsInf(paidAmount, 0) {
		return 0, fmt.Errorf("无效的Stripe支付金额")
	}
	unitAmount := int64(math.Round(paidAmount * 100))
	if unitAmount <= 0 {
		return 0, fmt.Errorf("无效的Stripe支付金额")
	}
	return unitAmount, nil
}

func markVipActivationOrderFailed(vipOrder *model.VipActivationRecord) {
	if vipOrder == nil || vipOrder.Id <= 0 {
		return
	}
	_ = model.DB.Model(&model.VipActivationRecord{}).
		Where("id = ? AND status = ?", vipOrder.Id, model.VipActivationStatusPending).
		Update("status", model.VipActivationStatusFailed).Error
}

func buildVipInviteLink(affCode string) string {
	if affCode == "" {
		return ""
	}
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	return base + common.ThemeAwarePath("/sign-up?aff="+url.QueryEscape(affCode))
}
