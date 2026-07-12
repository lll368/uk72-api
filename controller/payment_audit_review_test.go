package controller

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"github.com/waffo-com/waffo-go/core"
	"gorm.io/gorm"
)

type paymentAuditAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupPaymentAuditControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldUsingSQLite := common.UsingSQLite
	oldRedisEnabled := common.RedisEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.TopUp{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.UserProfile{},
		&model.VipActivationRecord{},
		&model.UserRelation{},
		&model.WalletAccount{},
		&model.WalletFlow{},
		&model.CommissionRecord{},
		&model.PaymentCallbackLog{},
		&model.PaymentReconciliationTask{},
	))

	oldStripeSecret := setting.StripeApiSecret
	oldStripeWebhookSecret := setting.StripeWebhookSecret
	oldStripePriceId := setting.StripePriceId
	paymentSetting := operation_setting.GetPaymentSetting()
	oldComplianceConfirmed := paymentSetting.ComplianceConfirmed
	oldComplianceTermsVersion := paymentSetting.ComplianceTermsVersion

	setting.StripeApiSecret = "sk_test_payment_audit"
	setting.StripeWebhookSecret = "whsec_payment_audit"
	setting.StripePriceId = "price_payment_audit"
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
		setting.StripeApiSecret = oldStripeSecret
		setting.StripeWebhookSecret = oldStripeWebhookSecret
		setting.StripePriceId = oldStripePriceId
		paymentSetting.ComplianceConfirmed = oldComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = oldComplianceTermsVersion
		_ = sqlDB.Close()
	})

	router := gin.New()
	router.POST("/stripe/webhook", StripeWebhook)
	router.POST("/admin/topups/:trade_no/reverse", AdminReverseTopUpOrder)
	router.POST("/admin/vip-activations/:trade_no/repair", AdminRepairVipActivationSettlement)
	return router
}

func TestStripeWebhookUpdatesAuditBizTypeForTopUp(t *testing.T) {
	router := setupPaymentAuditControllerTest(t)
	tradeNo := "stripe-audit-topup"
	require.NoError(t, model.DB.Create(&model.User{
		Id:       8101,
		Username: "stripe_audit_user",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          8101,
		Amount:          10,
		Money:           10,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}).Error)

	payload, err := common.Marshal(map[string]any{
		"id":      "evt_payment_audit_topup",
		"object":  "event",
		"type":    string(stripe.EventTypeCheckoutSessionCompleted),
		"created": time.Now().Unix(),
		"data": map[string]any{
			"object": map[string]any{
				"object":              "checkout.session",
				"client_reference_id": tradeNo,
				"customer":            "cus_payment_audit",
				"status":              "complete",
				"payment_status":      "paid",
				"amount_total":        1000,
				"currency":            "usd",
			},
		},
	})
	require.NoError(t, err)
	signedPayload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  setting.StripeWebhookSecret,
	})

	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(signedPayload.Payload))
	req.Header.Set("Stripe-Signature", signedPayload.Header)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var auditLog model.PaymentCallbackLog
	require.NoError(t, model.DB.Where("provider = ? AND trade_no = ?", model.PaymentProviderStripe, tradeNo).First(&auditLog).Error)
	assert.True(t, auditLog.VerifyStatus)
	assert.Equal(t, model.PaymentProcessStatusSuccess, auditLog.ProcessStatus)
	assert.Equal(t, service.PaymentBizTypeTopUp, auditLog.BizType)
}

func TestApplyStripeTopUpSnapshotStoresActualPaidAmount(t *testing.T) {
	topUp := &model.TopUp{
		Amount:          100,
		Money:           120,
		PaymentProvider: model.PaymentProviderStripe,
	}

	applyStripeTopUpSnapshot(topUp, 96)

	assert.InDelta(t, 120.0, topUp.RechargeAmount, 0.000001)
	assert.InDelta(t, 96.0, topUp.PaidAmount, 0.000001)
	assert.InDelta(t, 0.8, topUp.Discount, 0.000001)
}

func TestStripeWebhookOverwritesEstimatedPaidAmountWithActualAmountTotal(t *testing.T) {
	router := setupPaymentAuditControllerTest(t)
	tradeNo := "stripe-actual-paid"
	require.NoError(t, model.DB.Create(&model.User{
		Id:       8102,
		Username: "stripe_actual_paid_user",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          8102,
		Amount:          100,
		Money:           120,
		RechargeAmount:  120,
		PaidAmount:      96,
		Discount:        0.8,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}).Error)

	payload, err := common.Marshal(map[string]any{
		"id":      "evt_payment_actual_paid",
		"object":  "event",
		"type":    string(stripe.EventTypeCheckoutSessionCompleted),
		"created": time.Now().Unix(),
		"data": map[string]any{
			"object": map[string]any{
				"object":              "checkout.session",
				"client_reference_id": tradeNo,
				"customer":            "cus_actual_paid",
				"status":              "complete",
				"payment_status":      "paid",
				"amount_total":        8800,
				"currency":            "usd",
			},
		},
	})
	require.NoError(t, err)
	signedPayload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  setting.StripeWebhookSecret,
	})

	req := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(signedPayload.Payload))
	req.Header.Set("Stripe-Signature", signedPayload.Header)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var topUp model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", tradeNo).First(&topUp).Error)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.InDelta(t, 120.0, topUp.RechargeAmount, 0.000001)
	assert.InDelta(t, 88.0, topUp.PaidAmount, 0.000001)
	assert.InDelta(t, 88.0/120.0, topUp.Discount, 0.000001)
}

func TestCreemWebhookOverwritesEstimatedPaidAmountWithAmountPaid(t *testing.T) {
	setupPaymentAuditControllerTest(t)
	tradeNo := "creem-actual-paid"
	require.NoError(t, model.DB.Create(&model.User{
		Id:       8103,
		Username: "creem_actual_paid_user",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          8103,
		Amount:          int64(20 * common.QuotaPerUnit),
		Money:           16,
		RechargeAmount:  20,
		PaidAmount:      16,
		Discount:        0.8,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodCreem,
		PaymentProvider: model.PaymentProviderCreem,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/creem/webhook", nil)
	event := &CreemWebhookEvent{EventType: "checkout.completed"}
	event.Object.RequestId = tradeNo
	event.Object.Order.Id = "creem-order-actual-paid"
	event.Object.Order.Status = "paid"
	event.Object.Order.Type = "onetime"
	event.Object.Order.AmountPaid = 1234
	event.Object.Order.Currency = "usd"

	require.NoError(t, handleCheckoutCompleted(c, event, nil))

	var topUp model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", tradeNo).First(&topUp).Error)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.InDelta(t, 20.0, topUp.RechargeAmount, 0.000001)
	assert.InDelta(t, 12.34, topUp.PaidAmount, 0.000001)
	assert.InDelta(t, 12.34/20.0, topUp.Discount, 0.000001)
}

func TestWaffoWebhookOverwritesEstimatedPaidAmountWithOrderAmount(t *testing.T) {
	setupPaymentAuditControllerTest(t)
	tradeNo := "waffo-actual-paid"
	require.NoError(t, model.DB.Create(&model.User{
		Id:       8104,
		Username: "waffo_actual_paid_user",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          8104,
		Amount:          100,
		Money:           90,
		RechargeAmount:  100,
		PaidAmount:      90,
		Discount:        0.9,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodWaffo,
		PaymentProvider: model.PaymentProviderWaffo,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/waffo/webhook", nil)
	result := &core.PaymentNotificationResult{
		MerchantOrderID: tradeNo,
		OrderStatus:     "PAY_SUCCESS",
		OrderAmount:     "77.50",
	}

	require.NoError(t, handleWaffoPayment(c, result, nil))

	var topUp model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", tradeNo).First(&topUp).Error)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.InDelta(t, 77.50, topUp.PaidAmount, 0.000001)
	assert.InDelta(t, 0.775, topUp.Discount, 0.000001)
}

func TestStripeReverseEventRejectsMissingTradeNo(t *testing.T) {
	event := stripe.Event{
		Type: stripe.EventTypeChargeRefunded,
		Data: &stripe.EventData{Object: map[string]interface{}{}},
	}

	var err error
	require.NotPanics(t, func() {
		err = stripeReverseEvent(context.Background(), event, "127.0.0.1")
	})
	require.Error(t, err)
}

func TestAdminReverseTopUpOrderRejectsMalformedJSON(t *testing.T) {
	router := setupPaymentAuditControllerTest(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/topups/not-exists/reverse", bytes.NewBufferString(`{"provider":`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp paymentAuditAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "参数错误", resp.Message)
}

func TestAdminRepairVipActivationSettlementAllowsEmptyBody(t *testing.T) {
	router := setupPaymentAuditControllerTest(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	oldVipActivationPrice := paymentSetting.VipActivationPrice
	oldLevel1Amount := paymentSetting.VipActivationCommissionLevel1Amount
	oldLevel2Amount := paymentSetting.VipActivationCommissionLevel2Amount
	paymentSetting.VipActivationPrice = model.DefaultVipActivationPaid
	paymentSetting.VipActivationCommissionLevel1Amount = 1000
	paymentSetting.VipActivationCommissionLevel2Amount = 0
	t.Cleanup(func() {
		paymentSetting.VipActivationPrice = oldVipActivationPrice
		paymentSetting.VipActivationCommissionLevel1Amount = oldLevel1Amount
		paymentSetting.VipActivationCommissionLevel2Amount = oldLevel2Amount
	})

	require.NoError(t, model.DB.Create(&model.User{Id: 8201, Username: "repair_parent", AffCode: "repair8201", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 8202, Username: "repair_child", AffCode: "repair8202", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:          8201,
		TradeNo:         "active-parent-8201",
		PaymentProvider: model.PaymentProviderEpay,
		PaymentMethod:   "alipay",
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     time.Now().Unix(),
	}).Error)
	_, err := model.CreateActiveUserRelationTx(model.DB, 8201, 8202, model.UserRelationSourceAdmin, "repair-api-relation")
	require.NoError(t, err)
	order := &model.VipActivationRecord{
		UserId:          8202,
		TradeNo:         "vip-repair-api-empty-body",
		PaidAmount:      model.DefaultVipActivationPaid,
		PaymentProvider: model.PaymentProviderEpay,
		PaymentMethod:   "alipay",
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(order).Error)

	req := httptest.NewRequest(http.MethodPost, "/admin/vip-activations/vip-repair-api-empty-body/repair", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp paymentAuditAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var vipFlowCount int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).
		Where("user_id = ? AND flow_type = ? AND biz_no = ?", 8202, model.WalletFlowTypeVipActivation, order.TradeNo).
		Count(&vipFlowCount).Error)
	assert.Equal(t, int64(1), vipFlowCount)

	var commissionCount int64
	require.NoError(t, model.DB.Model(&model.CommissionRecord{}).
		Where("beneficiary_user_id = ? AND source_order_no = ? AND level = ?", 8201, order.TradeNo, 1).
		Count(&commissionCount).Error)
	assert.Equal(t, int64(1), commissionCount)
}

func TestAdminRepairVipActivationSettlementRejectsProviderMismatch(t *testing.T) {
	router := setupPaymentAuditControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 8211, Username: "repair_provider_user", AffCode: "repair8211", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:          8211,
		TradeNo:         "vip-repair-provider-mismatch",
		PaymentProvider: model.PaymentProviderEpay,
		PaymentMethod:   "alipay",
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     time.Now().Unix(),
	}).Error)

	req := httptest.NewRequest(http.MethodPost, "/admin/vip-activations/vip-repair-provider-mismatch/repair", bytes.NewBufferString(`{"provider":"stripe"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp paymentAuditAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "payment")
}
