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
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateControllerWechatPayPrivateKey(t *testing.T) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateDER := x509.MarshalPKCS1PrivateKey(privateKey)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateDER}))
}

func configureWechatPayControllerTest(t *testing.T, responseStatus int, responseBody string) {
	t.Helper()
	originalEnabled := setting.WechatPayEnabled
	originalSandbox := setting.WechatPaySandbox
	originalAppID := setting.WechatPayAppId
	originalMchID := setting.WechatPayMchId
	originalSerialNo := setting.WechatPayMerchantSerialNo
	originalPrivateKey := setting.WechatPayMerchantPrivateKey
	originalAPIv3Key := setting.WechatPayAPIv3Key
	originalPlatformSerialNo := setting.WechatPayPlatformSerialNo
	originalPlatformPublicKey := setting.WechatPayPlatformPublicKey
	originalUnitPrice := setting.WechatPayUnitPrice
	originalMinTopUp := setting.WechatPayMinTopUp
	originalNotifyURL := setting.WechatPayNotifyUrl
	originalGatewayURL := service.WechatPayGatewayURL
	t.Cleanup(func() {
		setting.WechatPayEnabled = originalEnabled
		setting.WechatPaySandbox = originalSandbox
		setting.WechatPayAppId = originalAppID
		setting.WechatPayMchId = originalMchID
		setting.WechatPayMerchantSerialNo = originalSerialNo
		setting.WechatPayMerchantPrivateKey = originalPrivateKey
		setting.WechatPayAPIv3Key = originalAPIv3Key
		setting.WechatPayPlatformSerialNo = originalPlatformSerialNo
		setting.WechatPayPlatformPublicKey = originalPlatformPublicKey
		setting.WechatPayUnitPrice = originalUnitPrice
		setting.WechatPayMinTopUp = originalMinTopUp
		setting.WechatPayNotifyUrl = originalNotifyURL
		service.WechatPayGatewayURL = originalGatewayURL
	})

	status := responseStatus
	if status == 0 {
		status = http.StatusOK
	}
	body := responseBody
	if body == "" {
		body = `{"code_url":"weixin://wxpay/bizpayurl?pr=test"}`
	}
	platformPrivateKey, platformPublicKey := generateControllerWechatPayPlatformKeyPair(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v3/pay/transactions/native", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if status >= http.StatusOK && status < http.StatusMultipleChoices {
			timestamp := strconv.FormatInt(time.Now().Unix(), 10)
			nonce := "response-nonce"
			w.Header().Set("Wechatpay-Timestamp", timestamp)
			w.Header().Set("Wechatpay-Nonce", nonce)
			w.Header().Set("Wechatpay-Serial", setting.WechatPayPlatformSerialNo)
			w.Header().Set("Wechatpay-Signature", signWechatPayNotifyMessageForControllerTest(t, platformPrivateKey, timestamp+"\n"+nonce+"\n"+body+"\n"))
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	setting.WechatPayEnabled = true
	setting.WechatPaySandbox = false
	setting.WechatPayAppId = "wx1234567890abcdef"
	setting.WechatPayMchId = "1900000001"
	setting.WechatPayMerchantSerialNo = "7777777777777777777777777777777777777777"
	setting.WechatPayMerchantPrivateKey = generateControllerWechatPayPrivateKey(t)
	setting.WechatPayAPIv3Key = "0123456789abcdef0123456789abcdef"
	setting.WechatPayPlatformSerialNo = "8888888888888888888888888888888888888888"
	setting.WechatPayPlatformPublicKey = platformPublicKey
	setting.WechatPayUnitPrice = 2
	setting.WechatPayMinTopUp = 5
	setting.WechatPayNotifyUrl = "https://api.example.com/api/wechat/notify"
	service.WechatPayGatewayURL = server.URL
}

func addWechatPayRoutesForTest(router *gin.Engine) {
	router.POST("/api/user/wechat/amount", func(c *gin.Context) {
		c.Set("id", 1)
		RequestWechatPayAmount(c)
	})
	router.POST("/api/user/wechat/pay", func(c *gin.Context) {
		c.Set("id", 1)
		RequestWechatPayPay(c)
	})
	router.POST("/api/vip/wechat/pay", func(c *gin.Context) {
		c.Set("id", 1)
		VipActivationRequestWechatPay(c)
	})
}

func TestGetTopUpInfoIncludesDirectWechatPayWithoutReplacingEpayWechat(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	configureWechatPayControllerTest(t, http.StatusOK, "")
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			PayMethods []struct {
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
	assert.True(t, types["wxpay"])
	assert.True(t, types[model.PaymentMethodWechatDirect])
}

func TestRequestWechatPayAmountUsesWechatPricingAndDiscount(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayRoutesForTest(router)
	configureWechatPayControllerTest(t, http.StatusOK, "")
	seedMockDebugUser(t, 1)
	paymentSetting := operation_setting.GetPaymentSetting()
	oldAmountDiscount := paymentSetting.AmountDiscount
	paymentSetting.AmountDiscount = map[int]float64{10: 0.5}
	t.Cleanup(func() {
		paymentSetting.AmountDiscount = oldAmountDiscount
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/wechat/amount", bytes.NewBufferString(`{"amount":10}`))
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

func TestRequestWechatPayPayCreatesPendingOrderAndQRCode(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayRoutesForTest(router)
	configureWechatPayControllerTest(t, http.StatusOK, "")
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/user/wechat/pay", bytes.NewBufferString(`{"amount":10,"payment_method":"wechat_direct"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Message string `json:"message"`
		Data    struct {
			CodeURL   string `json:"code_url"`
			TradeNo   string `json:"trade_no"`
			ExpiresAt int64  `json:"expires_at"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "success", resp.Message)
	require.NotEmpty(t, resp.Data.CodeURL)
	require.NotEmpty(t, resp.Data.TradeNo)
	require.Greater(t, resp.Data.ExpiresAt, time.Now().Unix())

	topUp := model.GetTopUpByTradeNo(resp.Data.TradeNo)
	require.NotNil(t, topUp)
	assert.Equal(t, int64(10), topUp.Amount)
	assert.InDelta(t, 20.0, topUp.Money, 0.000001)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)
	assert.Equal(t, model.PaymentProviderWechat, topUp.PaymentProvider)
	assert.Equal(t, model.PaymentMethodWechatDirect, topUp.PaymentMethod)
}

func TestRequestWechatPayPayMarksOrderFailedWhenRemoteCreateFails(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayRoutesForTest(router)
	configureWechatPayControllerTest(t, http.StatusInternalServerError, `{"code":"SYSTEMERROR","message":"remote failed"}`)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/user/wechat/pay", bytes.NewBufferString(`{"amount":10,"payment_method":"wechat_direct"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "拉起支付失败")
	var topUp model.TopUp
	require.NoError(t, model.DB.First(&topUp).Error)
	assert.Equal(t, common.TopUpStatusFailed, topUp.Status)
	assert.Equal(t, model.PaymentProviderWechat, topUp.PaymentProvider)
}

func TestRequestWechatPayPayRejectsAmountBelowMinimum(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayRoutesForTest(router)
	configureWechatPayControllerTest(t, http.StatusOK, "")
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/user/wechat/pay", bytes.NewBufferString(`{"amount":4,"payment_method":"wechat_direct"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "不能小于")
	var count int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestGetVipInfoIncludesDirectWechatPay(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	configureWechatPayControllerTest(t, http.StatusOK, "")
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
	assert.True(t, types[model.PaymentMethodWechatDirect])
}

func TestVipActivationRequestWechatPayCreatesPendingOrderAndQRCode(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayRoutesForTest(router)
	configureWechatPayControllerTest(t, http.StatusOK, "")
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/vip/wechat/pay", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			CodeURL   string `json:"code_url"`
			OrderID   string `json:"order_id"`
			ExpiresAt int64  `json:"expires_at"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.NotEmpty(t, resp.Data.CodeURL)
	require.NotEmpty(t, resp.Data.OrderID)
	require.Greater(t, resp.Data.ExpiresAt, time.Now().Unix())

	record, err := model.GetVipActivationRecordByTradeNo(resp.Data.OrderID)
	require.NoError(t, err)
	assert.Equal(t, model.VipActivationStatusPending, record.Status)
	assert.Equal(t, model.PaymentProviderWechat, record.PaymentProvider)
	assert.Equal(t, model.PaymentMethodWechatDirect, record.PaymentMethod)
	assert.InDelta(t, operation_setting.GetVipActivationPaymentAmount(), record.PaidAmount, 0.000001)
}

func TestVipActivationRequestWechatPayMarksOrderFailedWhenRemoteCreateFails(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayRoutesForTest(router)
	configureWechatPayControllerTest(t, http.StatusInternalServerError, `{"code":"SYSTEMERROR","message":"remote failed"}`)
	seedMockDebugUser(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/vip/wechat/pay", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "拉起支付失败")
	var record model.VipActivationRecord
	require.NoError(t, model.DB.First(&record).Error)
	assert.Equal(t, model.VipActivationStatusFailed, record.Status)
	assert.Equal(t, model.PaymentProviderWechat, record.PaymentProvider)
}

func TestVipActivationRequestWechatPayRejectsActiveVvip(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayRoutesForTest(router)
	configureWechatPayControllerTest(t, http.StatusOK, "")
	seedMockDebugUser(t, 1)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:          1,
		TradeNo:         "active-vvip-wechat",
		PaymentProvider: model.PaymentProviderWechat,
		PaymentMethod:   model.PaymentMethodWechatDirect,
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     now,
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserProfile{
		UserId:          1,
		IsVvip:          true,
		VvipStatus:      model.VvipStatusActive,
		VvipActivatedAt: now,
	}).Error)

	req := httptest.NewRequest(http.MethodPost, "/api/vip/wechat/pay", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "VVIP")
	var count int64
	require.NoError(t, model.DB.Model(&model.VipActivationRecord{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
