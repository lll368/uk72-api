package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMockDebugPaymentControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldUsingSQLite := common.UsingSQLite
	oldRedisEnabled := common.RedisEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	oldLogConsumeEnabled := common.LogConsumeEnabled
	oldDebugEnabled := common.DebugEnabled
	oldPayMethods := operation_setting.PayMethods
	oldPayAddress := operation_setting.PayAddress
	oldEpayId := operation_setting.EpayId
	oldEpayKey := operation_setting.EpayKey
	oldMinTopUp := operation_setting.MinTopUp
	oldPrice := operation_setting.Price
	oldQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	paymentSetting := operation_setting.GetPaymentSetting()
	oldComplianceConfirmed := paymentSetting.ComplianceConfirmed
	oldComplianceTermsVersion := paymentSetting.ComplianceTermsVersion
	oldAmountDiscount := paymentSetting.AmountDiscount

	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true
	common.DebugEnabled = true
	model.InitDBColumnNamesForTests()

	operation_setting.PayMethods = []map[string]string{
		{"name": "支付宝", "type": "alipay", "color": "blue"},
		{"name": "微信", "type": "wxpay", "color": "green"},
		{"name": "模拟支付成功", "type": "mock_debug_success", "color": "green"},
		{"name": "模拟支付失败", "type": "mock_debug_failed", "color": "red"},
	}
	operation_setting.PayAddress = "https://pay.example.com/"
	operation_setting.EpayId = "1000"
	operation_setting.EpayKey = "secret"
	operation_setting.MinTopUp = 1
	operation_setting.Price = 1
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	paymentSetting.AmountDiscount = map[int]float64{}

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
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
		&model.UserProfile{},
		&model.VipActivationRecord{},
		&model.UserRelation{},
		&model.WalletAccount{},
		&model.WalletFlow{},
		&model.CommissionRecord{},
		&model.PaymentCallbackLog{},
	))

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
		common.LogConsumeEnabled = oldLogConsumeEnabled
		common.DebugEnabled = oldDebugEnabled
		operation_setting.PayMethods = oldPayMethods
		operation_setting.PayAddress = oldPayAddress
		operation_setting.EpayId = oldEpayId
		operation_setting.EpayKey = oldEpayKey
		operation_setting.MinTopUp = oldMinTopUp
		operation_setting.Price = oldPrice
		operation_setting.GetGeneralSetting().QuotaDisplayType = oldQuotaDisplayType
		paymentSetting.ComplianceConfirmed = oldComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = oldComplianceTermsVersion
		paymentSetting.AmountDiscount = oldAmountDiscount
		_ = sqlDB.Close()
	})

	router := gin.New()
	router.GET("/api/user/topup/info", func(c *gin.Context) {
		c.Set("id", 1)
		GetTopUpInfo(c)
	})
	router.POST("/api/user/pay", func(c *gin.Context) {
		c.Set("id", 1)
		RequestEpay(c)
	})
	router.GET("/api/vip/info", func(c *gin.Context) {
		c.Set("id", 1)
		GetVipActivationInfo(c)
	})
	router.GET("/api/vip/orders/:trade_no/status", func(c *gin.Context) {
		c.Set("id", 1)
		GetVipActivationOrderStatus(c)
	})
	router.POST("/api/vip/epay/pay", func(c *gin.Context) {
		c.Set("id", 1)
		VipActivationRequestEpay(c)
	})
	return router
}

func seedMockDebugUser(t *testing.T, id int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:        id,
		Username:  fmt.Sprintf("mock_user_%d", id),
		Password:  "password123",
		Group:     "default",
		Status:    common.UserStatusEnabled,
		Role:      common.RoleCommonUser,
		CreatedAt: time.Now().Unix(),
	}).Error)
}

func TestGetTopUpInfoDoesNotExposeDebugPaymentMethodsWhenDebugEnabled(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.NotContains(t, body, "debug_payment_methods")
	assert.NotContains(t, body, "mock_debug_success")
	assert.NotContains(t, body, "mock_debug_failed")
}

func TestGetVipInfoDoesNotExposeDebugPaymentMethodsWhenDebugEnabled(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/vip/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.NotContains(t, body, "debug_payment_methods")
	assert.NotContains(t, body, "mock_debug_success")
	assert.NotContains(t, body, "mock_debug_failed")
}

func TestGetVipInfoUsesRoundedVipActivationPaymentAmount(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	seedMockDebugUser(t, 1)
	paymentSetting := operation_setting.GetPaymentSetting()
	oldVipActivationPrice := paymentSetting.VipActivationPrice
	paymentSetting.VipActivationPrice = 19.999
	t.Cleanup(func() {
		paymentSetting.VipActivationPrice = oldVipActivationPrice
	})

	req := httptest.NewRequest(http.MethodGet, "/api/vip/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			ActivationAmount float64 `json:"activation_amount"`
			PaidAmount       float64 `json:"paid_amount"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	assert.InDelta(t, 20.0, resp.Data.ActivationAmount, 0.000001)
	assert.InDelta(t, 20.0, resp.Data.PaidAmount, 0.000001)
}

func TestGetVipActivationOrderStatusReturnsCurrentUsersOrder(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	seedMockDebugUser(t, 1)
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:           1,
		TradeNo:          "VIPUSR1NOtest123",
		ActivationAmount: 1680,
		PaidAmount:       1680,
		Discount:         1,
		PaymentProvider:  model.PaymentProviderWechat,
		PaymentMethod:    model.PaymentMethodWechatDirect,
		Status:           model.VipActivationStatusPending,
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/vip/orders/VIPUSR1NOtest123/status", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			TradeNo string `json:"trade_no"`
			Status  string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	assert.Equal(t, "VIPUSR1NOtest123", resp.Data.TradeNo)
	assert.Equal(t, model.VipActivationStatusPending, resp.Data.Status)
}

func TestGetVipActivationOrderStatusRejectsOtherUsersOrder(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	seedMockDebugUser(t, 1)
	require.NoError(t, model.DB.Create(&model.User{
		Id:        2,
		Username:  "mock_user_2",
		Password:  "password123",
		Group:     "default",
		AffCode:   "mock2",
		Status:    common.UserStatusEnabled,
		Role:      common.RoleCommonUser,
		CreatedAt: time.Now().Unix(),
	}).Error)
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:           2,
		TradeNo:          "VIPUSR2NOtest123",
		ActivationAmount: 1680,
		PaidAmount:       1680,
		Discount:         1,
		PaymentProvider:  model.PaymentProviderWechat,
		PaymentMethod:    model.PaymentMethodWechatDirect,
		Status:           model.VipActivationStatusSuccess,
		ActivatedAt:      time.Now().Unix(),
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/vip/orders/VIPUSR2NOtest123/status", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	assert.Equal(t, "订单不存在", resp.Message)
}

func TestRequestEpayRejectsFormerMockDebugPaymentMethod(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/user/pay", bytes.NewBufferString(`{"amount":10,"payment_method":"mock_debug_success"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "支付方式不存在")
	assert.NotContains(t, recorder.Body.String(), "\"mock\":true")
	var count int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
	_, err := model.GetWalletAccountByUserId(1)
	assert.Error(t, err)
}

func TestVipActivationEpayRejectsFormerMockDebugPaymentMethod(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/vip/epay/pay", bytes.NewBufferString(`{"payment_method":"mock_debug_success"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "支付方式不存在")
	assert.NotContains(t, recorder.Body.String(), "\"mock\":true")
	var count int64
	require.NoError(t, model.DB.Model(&model.VipActivationRecord{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
	active, err := model.IsUserActiveVvip(1)
	require.NoError(t, err)
	assert.False(t, active)
}
