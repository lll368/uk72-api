package controller

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateControllerAlipayKeyPair(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateDER}))

	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	return privatePEM, publicPEM
}

func configureAlipayNotifyControllerTest(t *testing.T) string {
	t.Helper()

	privateKey, publicKey := generateControllerAlipayKeyPair(t)
	originalEnabled := setting.AlipayEnabled
	originalSandbox := setting.AlipaySandbox
	originalAppID := setting.AlipayAppId
	originalPrivateKey := setting.AlipayPrivateKey
	originalPublicKey := setting.AlipayPublicKey
	originalUnitPrice := setting.AlipayUnitPrice
	originalMinTopUp := setting.AlipayMinTopUp
	originalReturnURL := setting.AlipayReturnUrl
	originalNotifyURL := setting.AlipayNotifyUrl
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
	})

	setting.AlipayEnabled = true
	setting.AlipaySandbox = true
	setting.AlipayAppId = "2021000000000000"
	setting.AlipayPrivateKey = privateKey
	setting.AlipayPublicKey = publicKey
	setting.AlipayUnitPrice = 2
	setting.AlipayMinTopUp = 1
	setting.AlipayReturnUrl = "https://app.example.com/wallet"
	setting.AlipayNotifyUrl = "https://api.example.com/api/alipay/notify"
	return privateKey
}

func addAlipayNotifyRouteForTest(router *gin.Engine) {
	router.POST("/api/alipay/notify", AlipayNotify)
}

func createPendingAlipayTopUpForNotifyTest(t *testing.T, tradeNo string, provider string, paidAmount float64) {
	t.Helper()
	topUp := &model.TopUp{
		UserId:          1,
		Amount:          10,
		Money:           paidAmount,
		RechargeAmount:  10,
		PaidAmount:      paidAmount,
		Discount:        decimal.NewFromFloat(paidAmount).Div(decimal.NewFromInt(10)).InexactFloat64(),
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodAlipayDirect,
		PaymentProvider: provider,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())
}

func buildSignedAlipayNotifyForm(t *testing.T, privateKeyText string, params map[string]string) url.Values {
	t.Helper()
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	if values.Get("sign_type") == "" {
		values.Set("sign_type", "RSA2")
	}
	signature := signAlipayNotifyParamsForTest(t, privateKeyText, values)
	values.Set("sign", signature)
	return values
}

func signAlipayNotifyParamsForTest(t *testing.T, privateKeyText string, values url.Values) string {
	t.Helper()

	block, _ := pem.Decode([]byte(privateKeyText))
	require.NotNil(t, block)
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)

	keys := make([]string, 0, len(values))
	for key := range values {
		value := values.Get(key)
		if strings.TrimSpace(key) == "" || key == "sign" || key == "sign_type" || value == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	content := strings.Join(parts, "&")
	hash := sha256.Sum256([]byte(content))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(signature)
}

func postAlipayNotifyForTest(t *testing.T, router *gin.Engine, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/alipay/notify", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func baseAlipayNotifyParams(tradeNo string, tradeStatus string, totalAmount string) map[string]string {
	return map[string]string{
		"app_id":         setting.AlipayAppId,
		"notify_id":      "20260527000000000000000000000000",
		"out_trade_no":   tradeNo,
		"trade_no":       "2026052722001400000000000000",
		"trade_status":   tradeStatus,
		"total_amount":   totalAmount,
		"receipt_amount": totalAmount,
		"charset":        "utf-8",
		"gmt_payment":    "2026-05-27 10:20:30",
	}
}

func countPaymentCallbackLogsForTest(t *testing.T, tradeNo string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.PaymentCallbackLog{}).Where("trade_no = ?", tradeNo).Count(&count).Error)
	return count
}

func getWalletBalanceForNotifyTest(t *testing.T, userId int) float64 {
	t.Helper()
	var account model.WalletAccount
	err := model.DB.Where("user_id = ?", userId).First(&account).Error
	if err != nil {
		return 0
	}
	return account.BalanceAmount
}

func TestAlipayNotifyRejectsInvalidSignatureAndAuditsFailed(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayNotifyRouteForTest(router)
	privateKey := configureAlipayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingAlipayTopUpForNotifyTest(t, "ALIPAY-NOTIFY-INVALID-SIGN", model.PaymentProviderAlipay, 20)

	form := buildSignedAlipayNotifyForm(t, privateKey, baseAlipayNotifyParams("ALIPAY-NOTIFY-INVALID-SIGN", service.AlipayTradeStatusSuccess, "20.00"))
	form.Set("total_amount", "19.99")
	recorder := postAlipayNotifyForTest(t, router, form)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "fail", recorder.Body.String())
	topUp := model.GetTopUpByTradeNo("ALIPAY-NOTIFY-INVALID-SIGN")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)

	var log model.PaymentCallbackLog
	require.NoError(t, model.DB.First(&log).Error)
	assert.Equal(t, model.PaymentProviderAlipay, log.Provider)
	assert.False(t, log.VerifyStatus)
	assert.Equal(t, model.PaymentProcessStatusFailed, log.ProcessStatus)
}

func TestAlipayNotifyRejectsAppIdProviderAndAmountMismatch(t *testing.T) {
	tests := []struct {
		name        string
		tradeNo     string
		provider    string
		appID       string
		totalAmount string
	}{
		{name: "app id mismatch", tradeNo: "ALIPAY-NOTIFY-APPID", provider: model.PaymentProviderAlipay, appID: "2021999999999999", totalAmount: "20.00"},
		{name: "provider mismatch", tradeNo: "ALIPAY-NOTIFY-PROVIDER", provider: model.PaymentProviderEpay, appID: setting.AlipayAppId, totalAmount: "20.00"},
		{name: "amount mismatch", tradeNo: "ALIPAY-NOTIFY-AMOUNT", provider: model.PaymentProviderAlipay, appID: setting.AlipayAppId, totalAmount: "19.99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupMockDebugPaymentControllerTest(t)
			addAlipayNotifyRouteForTest(router)
			privateKey := configureAlipayNotifyControllerTest(t)
			seedMockDebugUser(t, 1)
			createPendingAlipayTopUpForNotifyTest(t, tt.tradeNo, tt.provider, 20)

			params := baseAlipayNotifyParams(tt.tradeNo, service.AlipayTradeStatusSuccess, tt.totalAmount)
			if tt.appID != "" {
				params["app_id"] = tt.appID
			}
			form := buildSignedAlipayNotifyForm(t, privateKey, params)
			recorder := postAlipayNotifyForTest(t, router, form)

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, "fail", recorder.Body.String())
			topUp := model.GetTopUpByTradeNo(tt.tradeNo)
			require.NotNil(t, topUp)
			assert.Equal(t, common.TopUpStatusPending, topUp.Status)
			assert.Equal(t, int64(1), countPaymentCallbackLogsForTest(t, tt.tradeNo))
		})
	}
}

func TestAlipayNotifyCompletesTopUpAndDuplicateIsIdempotent(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayNotifyRouteForTest(router)
	privateKey := configureAlipayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingAlipayTopUpForNotifyTest(t, "ALIPAY-NOTIFY-TOPUP", model.PaymentProviderAlipay, 20)

	form := buildSignedAlipayNotifyForm(t, privateKey, baseAlipayNotifyParams("ALIPAY-NOTIFY-TOPUP", service.AlipayTradeStatusSuccess, "20.00"))
	recorder := postAlipayNotifyForTest(t, router, form)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())

	topUp := model.GetTopUpByTradeNo("ALIPAY-NOTIFY-TOPUP")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.InDelta(t, 10.0, getWalletBalanceForNotifyTest(t, 1), 0.000001)

	recorder = postAlipayNotifyForTest(t, router, form)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())
	assert.InDelta(t, 10.0, getWalletBalanceForNotifyTest(t, 1), 0.000001)

	var flowCount int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).Where("biz_no = ?", "ALIPAY-NOTIFY-TOPUP").Count(&flowCount).Error)
	assert.Equal(t, int64(1), flowCount)
	assert.Equal(t, int64(2), countPaymentCallbackLogsForTest(t, "ALIPAY-NOTIFY-TOPUP"))
}

func TestAlipayNotifyCompletesTopUpWhenNewPaymentDisabled(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayNotifyRouteForTest(router)
	privateKey := configureAlipayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingAlipayTopUpForNotifyTest(t, "ALIPAY-NOTIFY-DISABLED", model.PaymentProviderAlipay, 20)
	setting.AlipayEnabled = false

	form := buildSignedAlipayNotifyForm(t, privateKey, baseAlipayNotifyParams("ALIPAY-NOTIFY-DISABLED", service.AlipayTradeStatusSuccess, "20.00"))
	recorder := postAlipayNotifyForTest(t, router, form)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())

	topUp := model.GetTopUpByTradeNo("ALIPAY-NOTIFY-DISABLED")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.InDelta(t, 10.0, getWalletBalanceForNotifyTest(t, 1), 0.000001)
}

func TestAlipayNotifyCompletesVipActivationAndDuplicateIsIdempotent(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayNotifyRouteForTest(router)
	privateKey := configureAlipayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	order, err := service.CreateVipActivationOrder(1, model.PaymentProviderAlipay, model.PaymentMethodAlipayDirect)
	require.NoError(t, err)

	form := buildSignedAlipayNotifyForm(t, privateKey, baseAlipayNotifyParams(order.TradeNo, service.AlipayTradeStatusFinished, "1680.00"))
	recorder := postAlipayNotifyForTest(t, router, form)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())

	record, err := model.GetVipActivationRecordByTradeNo(order.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, model.VipActivationStatusSuccess, record.Status)
	assert.Greater(t, record.ActivatedAt, int64(0))

	recorder = postAlipayNotifyForTest(t, router, form)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())

	var flowCount int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).Where("biz_no = ?", order.TradeNo).Count(&flowCount).Error)
	assert.Equal(t, int64(1), flowCount)
	assert.Equal(t, int64(2), countPaymentCallbackLogsForTest(t, order.TradeNo))
}

func TestAlipayNotifyIgnoresNonSuccessStatus(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayNotifyRouteForTest(router)
	privateKey := configureAlipayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingAlipayTopUpForNotifyTest(t, "ALIPAY-NOTIFY-WAIT", model.PaymentProviderAlipay, 20)

	form := buildSignedAlipayNotifyForm(t, privateKey, baseAlipayNotifyParams("ALIPAY-NOTIFY-WAIT", "WAIT_BUYER_PAY", "20.00"))
	recorder := postAlipayNotifyForTest(t, router, form)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())
	topUp := model.GetTopUpByTradeNo("ALIPAY-NOTIFY-WAIT")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)
	assert.InDelta(t, 0.0, getWalletBalanceForNotifyTest(t, 1), 0.000001)

	var log model.PaymentCallbackLog
	require.NoError(t, model.DB.Where("trade_no = ?", "ALIPAY-NOTIFY-WAIT").First(&log).Error)
	assert.True(t, log.VerifyStatus)
	assert.Equal(t, model.PaymentProcessStatusSuccess, log.ProcessStatus)
}

func TestAlipayNotifyMarksTopUpFailedOnTradeClosed(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayNotifyRouteForTest(router)
	privateKey := configureAlipayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingAlipayTopUpForNotifyTest(t, "ALIPAY-NOTIFY-CLOSED", model.PaymentProviderAlipay, 20)

	form := buildSignedAlipayNotifyForm(t, privateKey, baseAlipayNotifyParams("ALIPAY-NOTIFY-CLOSED", service.AlipayTradeStatusClosed, "20.00"))
	recorder := postAlipayNotifyForTest(t, router, form)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())
	topUp := model.GetTopUpByTradeNo("ALIPAY-NOTIFY-CLOSED")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusFailed, topUp.Status)
	assert.InDelta(t, 0.0, getWalletBalanceForNotifyTest(t, 1), 0.000001)

	var log model.PaymentCallbackLog
	require.NoError(t, model.DB.Where("trade_no = ?", "ALIPAY-NOTIFY-CLOSED").First(&log).Error)
	assert.True(t, log.VerifyStatus)
	assert.Equal(t, model.PaymentProcessStatusSuccess, log.ProcessStatus)
}

func TestAlipayNotifyMarksVipActivationFailedOnTradeClosed(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addAlipayNotifyRouteForTest(router)
	privateKey := configureAlipayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	order, err := service.CreateVipActivationOrder(1, model.PaymentProviderAlipay, model.PaymentMethodAlipayDirect)
	require.NoError(t, err)

	form := buildSignedAlipayNotifyForm(t, privateKey, baseAlipayNotifyParams(order.TradeNo, service.AlipayTradeStatusClosed, "1680.00"))
	recorder := postAlipayNotifyForTest(t, router, form)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())
	record, err := model.GetVipActivationRecordByTradeNo(order.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, model.VipActivationStatusFailed, record.Status)
}
