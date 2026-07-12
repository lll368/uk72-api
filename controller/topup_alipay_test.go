package controller

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateControllerAlipayPrivateKey(t *testing.T) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateDER := x509.MarshalPKCS1PrivateKey(privateKey)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateDER}))
}

func configureAlipayControllerTest(t *testing.T) {
	t.Helper()
	originalEnabled := setting.AlipayEnabled
	originalSandbox := setting.AlipaySandbox
	originalAppID := setting.AlipayAppId
	originalPrivateKey := setting.AlipayPrivateKey
	originalPublicKey := setting.AlipayPublicKey
	originalUnitPrice := setting.AlipayUnitPrice
	originalMinTopUp := setting.AlipayMinTopUp
	originalReturnURL := setting.AlipayReturnUrl
	originalNotifyURL := setting.AlipayNotifyUrl
	originalServerAddress := system_setting.ServerAddress
	t.Cleanup(func() {
		setting.AlipayEnabled = originalEnabled
		setting.AlipaySandbox = originalSandbox
		setting.AlipayAppId = originalAppID
		setting.AlipayPrivateKey = originalPrivateKey
		setting.AlipayPublicKey = originalPublicKey
		setting.AlipayUnitPrice = originalUnitPrice
		setting.AlipayMinTopUp = originalMinTopUp
		setting.AlipayReturnUrl = originalReturnURL
		setting.AlipayNotifyUrl = originalNotifyURL
		system_setting.ServerAddress = originalServerAddress
	})

	setting.AlipayEnabled = true
	setting.AlipaySandbox = true
	setting.AlipayAppId = "2021000000000000"
	setting.AlipayPrivateKey = generateControllerAlipayPrivateKey(t)
	setting.AlipayPublicKey = "public-key"
	setting.AlipayUnitPrice = 2
	setting.AlipayMinTopUp = 5
	setting.AlipayReturnUrl = "https://app.example.com/wallet"
	setting.AlipayNotifyUrl = "https://api.example.com/api/alipay/notify"
	system_setting.ServerAddress = "https://app.example.com"
}

func addAlipayRoutesForTest(router *gin.Engine) {
	router.POST("/api/user/alipay/amount", func(c *gin.Context) {
		c.Set("id", 1)
		RequestAlipayAmount(c)
	})
	router.POST("/api/user/alipay/pay", func(c *gin.Context) {
		c.Set("id", 1)
		RequestAlipayPay(c)
	})
	router.POST("/api/vip/alipay/pay", func(c *gin.Context) {
		c.Set("id", 1)
		VipActivationRequestAlipay(c)
	})
}

func TestGetTopUpInfoIncludesDirectAlipayWithoutReplacingEpayAlipay(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	configureAlipayControllerTest(t)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			PayMethods []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"pay_methods"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)

	types := map[string]bool{}
	for _, method := range resp.Data.PayMethods {
		types[method.Type] = true
	}
	assert.True(t, types["alipay"])
	assert.True(t, types[model.PaymentMethodAlipayDirect])
}

func TestRequestAlipayAmountUsesAlipayPricingAndDiscount(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayRoutesForTest(router)
	configureAlipayControllerTest(t)
	seedMockDebugUser(t, 1)
	paymentSetting := operation_setting.GetPaymentSetting()
	oldAmountDiscount := paymentSetting.AmountDiscount
	paymentSetting.AmountDiscount = map[int]float64{10: 0.5}
	t.Cleanup(func() {
		paymentSetting.AmountDiscount = oldAmountDiscount
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/alipay/amount", bytes.NewBufferString(`{"amount":10}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.Equal(t, "success", resp.Message)
	assert.Equal(t, "10.00", resp.Data)
}

func TestRequestAlipayPayCreatesPendingOrderAndSignedParams(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayRoutesForTest(router)
	configureAlipayControllerTest(t)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/user/alipay/pay", bytes.NewBufferString(`{"amount":10,"payment_method":"alipay_direct"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Message string `json:"message"`
		Data    struct {
			URL    string            `json:"url"`
			Params map[string]string `json:"params"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "success", resp.Message)
	require.NotEmpty(t, resp.Data.URL)
	require.Equal(t, "alipay.trade.page.pay", resp.Data.Params["method"])
	require.NotEmpty(t, resp.Data.Params["sign"])
	var bizContent map[string]any
	require.NoError(t, common.Unmarshal([]byte(resp.Data.Params["biz_content"]), &bizContent))
	require.Equal(t, "FAST_INSTANT_TRADE_PAY", bizContent["product_code"])

	var count int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	var topUp model.TopUp
	require.NoError(t, model.DB.First(&topUp).Error)
	assert.Equal(t, int64(10), topUp.Amount)
	assert.InDelta(t, 20.0, topUp.Money, 0.000001)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)
	assert.Equal(t, model.PaymentProviderAlipay, topUp.PaymentProvider)
	assert.Equal(t, model.PaymentMethodAlipayDirect, topUp.PaymentMethod)
	assert.Equal(t, topUp.TradeNo, bizContent["out_trade_no"])
}

func TestAlipayReturnUrlsSeparateWalletTopupAndVipActivation(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayRoutesForTest(router)
	configureAlipayControllerTest(t)
	seedMockDebugUser(t, 1)

	topupReq := httptest.NewRequest(http.MethodPost, "/api/user/alipay/pay", bytes.NewBufferString(`{"amount":10,"payment_method":"alipay_direct"}`))
	topupReq.Header.Set("Content-Type", "application/json")
	topupRecorder := httptest.NewRecorder()
	router.ServeHTTP(topupRecorder, topupReq)

	require.Equal(t, http.StatusOK, topupRecorder.Code)
	var topupResp struct {
		Message string `json:"message"`
		Data    struct {
			Params map[string]string `json:"params"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(topupRecorder.Body.Bytes(), &topupResp))
	require.Equal(t, "success", topupResp.Message)
	assert.Equal(t, "https://app.example.com/wallet", topupResp.Data.Params["return_url"])

	vipReq := httptest.NewRequest(http.MethodPost, "/api/vip/alipay/pay", bytes.NewBufferString(`{}`))
	vipReq.Header.Set("Content-Type", "application/json")
	vipRecorder := httptest.NewRecorder()
	router.ServeHTTP(vipRecorder, vipReq)

	require.Equal(t, http.StatusOK, vipRecorder.Code)
	var vipResp struct {
		Success bool `json:"success"`
		Data    struct {
			Params map[string]string `json:"params"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(vipRecorder.Body.Bytes(), &vipResp))
	require.True(t, vipResp.Success)
	assert.Equal(t, "https://app.example.com/compute-partners", vipResp.Data.Params["return_url"])
}

func TestRequestAlipayPayRejectsAmountBelowMinimum(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayRoutesForTest(router)
	configureAlipayControllerTest(t)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/user/alipay/pay", bytes.NewBufferString(`{"amount":4,"payment_method":"alipay_direct"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "不能小于")
	var count int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestAlipayMinTopupConvertsTokenDisplayMode(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayRoutesForTest(router)
	configureAlipayControllerTest(t)
	seedMockDebugUser(t, 1)
	originalQuotaPerUnit := common.QuotaPerUnit
	oldQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	common.QuotaPerUnit = 500000
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.GetGeneralSetting().QuotaDisplayType = oldQuotaDisplayType
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/alipay/amount", bytes.NewBufferString(`{"amount":`+strconv.Itoa(5*500000)+`}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success"`)
}

func TestGetTopUpInfoReturnsDirectAlipayTokenMinTopup(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	configureAlipayControllerTest(t)
	seedMockDebugUser(t, 1)
	originalQuotaPerUnit := common.QuotaPerUnit
	oldQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	common.QuotaPerUnit = 500000
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.GetGeneralSetting().QuotaDisplayType = oldQuotaDisplayType
	})

	req := httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			PayMethods []struct {
				Type     string `json:"type"`
				MinTopup string `json:"min_topup"`
			} `json:"pay_methods"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	for _, method := range resp.Data.PayMethods {
		if method.Type == model.PaymentMethodAlipayDirect {
			assert.Equal(t, strconv.FormatInt(getAlipayMinTopup(), 10), method.MinTopup)
			return
		}
	}
	t.Fatalf("direct Alipay method not found")
}

func TestGetVipInfoIncludesDirectAlipay(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	configureAlipayControllerTest(t)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/vip/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			PaymentMethods []struct {
				Type string `json:"type"`
			} `json:"payment_methods"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	types := map[string]bool{}
	for _, method := range resp.Data.PaymentMethods {
		types[method.Type] = true
	}
	assert.True(t, types[model.PaymentMethodAlipayDirect])
}

func TestVipActivationRequestAlipayCreatesPendingOrderAndSignedParams(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayRoutesForTest(router)
	configureAlipayControllerTest(t)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/vip/alipay/pay", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			URL     string            `json:"url"`
			Params  map[string]string `json:"params"`
			OrderID string            `json:"order_id"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.NotEmpty(t, resp.Data.URL)
	require.NotEmpty(t, resp.Data.Params["sign"])
	require.NotEmpty(t, resp.Data.OrderID)

	record, err := model.GetVipActivationRecordByTradeNo(resp.Data.OrderID)
	require.NoError(t, err)
	assert.Equal(t, model.VipActivationStatusPending, record.Status)
	assert.Equal(t, model.PaymentProviderAlipay, record.PaymentProvider)
	assert.Equal(t, model.PaymentMethodAlipayDirect, record.PaymentMethod)
	assert.InDelta(t, operation_setting.GetVipActivationPaymentAmount(), record.PaidAmount, 0.000001)
}

func TestVipActivationRequestAlipayRejectsActiveVvip(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayRoutesForTest(router)
	configureAlipayControllerTest(t)
	seedMockDebugUser(t, 1)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:          1,
		TradeNo:         "active-vvip-alipay",
		PaymentProvider: model.PaymentProviderAlipay,
		PaymentMethod:   model.PaymentMethodAlipayDirect,
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     now,
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserProfile{
		UserId:          1,
		IsVvip:          true,
		VvipStatus:      model.VvipStatusActive,
		VvipActivatedAt: now,
	}).Error)

	req := httptest.NewRequest(http.MethodPost, "/api/vip/alipay/pay", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "VVIP")
	var count int64
	require.NoError(t, model.DB.Model(&model.VipActivationRecord{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
