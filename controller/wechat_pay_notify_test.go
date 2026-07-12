package controller

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
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
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateControllerWechatPayPlatformKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	return privateKey, publicPEM
}

func configureWechatPayNotifyControllerTest(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	platformPrivateKey, platformPublicKey := generateControllerWechatPayPlatformKeyPair(t)
	originalEnabled := setting.WechatPayEnabled
	originalAppID := setting.WechatPayAppId
	originalMchID := setting.WechatPayMchId
	originalMerchantSerialNo := setting.WechatPayMerchantSerialNo
	originalMerchantPrivateKey := setting.WechatPayMerchantPrivateKey
	originalAPIv3Key := setting.WechatPayAPIv3Key
	originalPlatformSerialNo := setting.WechatPayPlatformSerialNo
	originalPlatformPublicKey := setting.WechatPayPlatformPublicKey
	originalUnitPrice := setting.WechatPayUnitPrice
	originalMinTopUp := setting.WechatPayMinTopUp
	t.Cleanup(func() {
		setting.WechatPayEnabled = originalEnabled
		setting.WechatPayAppId = originalAppID
		setting.WechatPayMchId = originalMchID
		setting.WechatPayMerchantSerialNo = originalMerchantSerialNo
		setting.WechatPayMerchantPrivateKey = originalMerchantPrivateKey
		setting.WechatPayAPIv3Key = originalAPIv3Key
		setting.WechatPayPlatformSerialNo = originalPlatformSerialNo
		setting.WechatPayPlatformPublicKey = originalPlatformPublicKey
		setting.WechatPayUnitPrice = originalUnitPrice
		setting.WechatPayMinTopUp = originalMinTopUp
	})

	setting.WechatPayEnabled = true
	setting.WechatPayAppId = "wx1234567890abcdef"
	setting.WechatPayMchId = "1900000001"
	setting.WechatPayMerchantSerialNo = "7777777777777777777777777777777777777777"
	setting.WechatPayMerchantPrivateKey = generateControllerWechatPayPrivateKey(t)
	setting.WechatPayAPIv3Key = "0123456789abcdef0123456789abcdef"
	setting.WechatPayPlatformSerialNo = "8888888888888888888888888888888888888888"
	setting.WechatPayPlatformPublicKey = platformPublicKey
	setting.WechatPayUnitPrice = 2
	setting.WechatPayMinTopUp = 1
	return platformPrivateKey
}

func addWechatPayNotifyRouteForTest(router *gin.Engine) {
	router.POST("/api/wechat/notify", WechatPayNotify)
}

func createPendingWechatTopUpForNotifyTest(t *testing.T, tradeNo string, provider string, paidAmount float64) {
	t.Helper()
	topUp := &model.TopUp{
		UserId:          1,
		Amount:          10,
		Money:           paidAmount,
		RechargeAmount:  10,
		PaidAmount:      paidAmount,
		Discount:        decimal.NewFromFloat(paidAmount).Div(decimal.NewFromInt(10)).InexactFloat64(),
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodWechatDirect,
		PaymentProvider: provider,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())
}

func buildSignedWechatPayNotifyBody(t *testing.T, platformPrivateKey *rsa.PrivateKey, transaction map[string]any, eventType string) ([]byte, map[string]string) {
	t.Helper()
	transactionBytes, err := common.Marshal(transaction)
	require.NoError(t, err)
	resource := encryptWechatPayNotifyResourceForControllerTest(t, transactionBytes)
	payload := map[string]any{
		"id":            "EV-20260527000000000000000000000000",
		"create_time":   "2026-05-27T10:20:30+08:00",
		"event_type":    eventType,
		"resource_type": "encrypt-resource",
		"summary":       "payment notification",
		"resource":      resource,
	}
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	headers := map[string]string{
		"Wechatpay-Timestamp": timestamp,
		"Wechatpay-Nonce":     "nonce-1",
		"Wechatpay-Serial":    setting.WechatPayPlatformSerialNo,
	}
	message := timestamp + "\nnonce-1\n" + string(body) + "\n"
	headers["Wechatpay-Signature"] = signWechatPayNotifyMessageForControllerTest(t, platformPrivateKey, message)
	return body, headers
}

func encryptWechatPayNotifyResourceForControllerTest(t *testing.T, plaintext []byte) map[string]any {
	t.Helper()
	block, err := aes.NewCipher([]byte(setting.WechatPayAPIv3Key))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := "notify-nonce"
	associatedData := "transaction"
	ciphertext := aead.Seal(nil, []byte(nonce), plaintext, []byte(associatedData))
	return map[string]any{
		"algorithm":       service.WechatPayResourceAlgorithm,
		"ciphertext":      base64.StdEncoding.EncodeToString(ciphertext),
		"associated_data": associatedData,
		"nonce":           nonce,
		"original_type":   "transaction",
	}
}

func signWechatPayNotifyMessageForControllerTest(t *testing.T, privateKey *rsa.PrivateKey, message string) string {
	t.Helper()
	hash := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(signature)
}

func postWechatPayNotifyForTest(t *testing.T, router *gin.Engine, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/wechat/notify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func baseWechatPayTransaction(tradeNo string, providerAmount int64, state string) map[string]any {
	return map[string]any{
		"appid":          setting.WechatPayAppId,
		"mchid":          setting.WechatPayMchId,
		"out_trade_no":   tradeNo,
		"transaction_id": "4200000000202605270000000000",
		"trade_type":     "NATIVE",
		"trade_state":    state,
		"amount": map[string]any{
			"total":          providerAmount,
			"payer_total":    providerAmount,
			"currency":       "CNY",
			"payer_currency": "CNY",
		},
	}
}

func TestWechatPayNotifyRejectsInvalidSignatureAndAuditsFailed(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayNotifyRouteForTest(router)
	platformPrivateKey := configureWechatPayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingWechatTopUpForNotifyTest(t, "WECHAT-NOTIFY-INVALID-SIGN", model.PaymentProviderWechat, 20)

	body, headers := buildSignedWechatPayNotifyBody(t, platformPrivateKey, baseWechatPayTransaction("WECHAT-NOTIFY-INVALID-SIGN", 2000, service.WechatPayTradeStateSuccess), service.WechatPayNotifySuccessEvent)
	body = append(body, []byte(" ")...)
	recorder := postWechatPayNotifyForTest(t, router, body, headers)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	topUp := model.GetTopUpByTradeNo("WECHAT-NOTIFY-INVALID-SIGN")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)

	var log model.PaymentCallbackLog
	require.NoError(t, model.DB.First(&log).Error)
	assert.Equal(t, model.PaymentProviderWechat, log.Provider)
	assert.False(t, log.VerifyStatus)
	assert.Equal(t, model.PaymentProcessStatusFailed, log.ProcessStatus)
}

func TestWechatPayNotifyRejectsMissingPlatformSerialWhenConfigured(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayNotifyRouteForTest(router)
	platformPrivateKey := configureWechatPayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingWechatTopUpForNotifyTest(t, "WECHAT-NOTIFY-MISSING-SERIAL", model.PaymentProviderWechat, 20)

	body, headers := buildSignedWechatPayNotifyBody(t, platformPrivateKey, baseWechatPayTransaction("WECHAT-NOTIFY-MISSING-SERIAL", 2000, service.WechatPayTradeStateSuccess), service.WechatPayNotifySuccessEvent)
	delete(headers, "Wechatpay-Serial")
	recorder := postWechatPayNotifyForTest(t, router, body, headers)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	topUp := model.GetTopUpByTradeNo("WECHAT-NOTIFY-MISSING-SERIAL")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)

	var log model.PaymentCallbackLog
	require.NoError(t, model.DB.First(&log).Error)
	assert.Equal(t, model.PaymentProviderWechat, log.Provider)
	assert.False(t, log.VerifyStatus)
	assert.Equal(t, model.PaymentProcessStatusFailed, log.ProcessStatus)
}

func TestWechatPayNotifyRejectsAppMerchantProviderAndAmountMismatch(t *testing.T) {
	const expectedAppID = "wx1234567890abcdef"
	const expectedMchID = "1900000001"
	tests := []struct {
		name     string
		tradeNo  string
		provider string
		appID    string
		mchID    string
		amount   int64
	}{
		{name: "app id mismatch", tradeNo: "WECHAT-NOTIFY-APPID", provider: model.PaymentProviderWechat, appID: "wx9999999999999999", mchID: expectedMchID, amount: 2000},
		{name: "merchant id mismatch", tradeNo: "WECHAT-NOTIFY-MCHID", provider: model.PaymentProviderWechat, appID: expectedAppID, mchID: "1900000099", amount: 2000},
		{name: "provider mismatch", tradeNo: "WECHAT-NOTIFY-PROVIDER", provider: model.PaymentProviderEpay, appID: expectedAppID, mchID: expectedMchID, amount: 2000},
		{name: "amount mismatch", tradeNo: "WECHAT-NOTIFY-AMOUNT", provider: model.PaymentProviderWechat, appID: expectedAppID, mchID: expectedMchID, amount: 1999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupMockDebugPaymentControllerTest(t)
			addWechatPayNotifyRouteForTest(router)
			platformPrivateKey := configureWechatPayNotifyControllerTest(t)
			seedMockDebugUser(t, 1)
			createPendingWechatTopUpForNotifyTest(t, tt.tradeNo, tt.provider, 20)

			transaction := baseWechatPayTransaction(tt.tradeNo, tt.amount, service.WechatPayTradeStateSuccess)
			transaction["appid"] = tt.appID
			transaction["mchid"] = tt.mchID
			body, headers := buildSignedWechatPayNotifyBody(t, platformPrivateKey, transaction, service.WechatPayNotifySuccessEvent)
			recorder := postWechatPayNotifyForTest(t, router, body, headers)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			topUp := model.GetTopUpByTradeNo(tt.tradeNo)
			require.NotNil(t, topUp)
			assert.Equal(t, common.TopUpStatusPending, topUp.Status)
			assert.Equal(t, int64(1), countPaymentCallbackLogsForTest(t, tt.tradeNo))
		})
	}
}

func TestWechatPayNotifyCompletesTopUpAndDuplicateIsIdempotent(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayNotifyRouteForTest(router)
	platformPrivateKey := configureWechatPayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingWechatTopUpForNotifyTest(t, "WECHAT-NOTIFY-TOPUP", model.PaymentProviderWechat, 20)

	body, headers := buildSignedWechatPayNotifyBody(t, platformPrivateKey, baseWechatPayTransaction("WECHAT-NOTIFY-TOPUP", 2000, service.WechatPayTradeStateSuccess), service.WechatPayNotifySuccessEvent)
	recorder := postWechatPayNotifyForTest(t, router, body, headers)
	require.Equal(t, http.StatusOK, recorder.Code)

	topUp := model.GetTopUpByTradeNo("WECHAT-NOTIFY-TOPUP")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.InDelta(t, 10.0, getWalletBalanceForNotifyTest(t, 1), 0.000001)

	recorder = postWechatPayNotifyForTest(t, router, body, headers)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.InDelta(t, 10.0, getWalletBalanceForNotifyTest(t, 1), 0.000001)

	var flowCount int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).Where("biz_no = ?", "WECHAT-NOTIFY-TOPUP").Count(&flowCount).Error)
	assert.Equal(t, int64(1), flowCount)
	assert.Equal(t, int64(2), countPaymentCallbackLogsForTest(t, "WECHAT-NOTIFY-TOPUP"))
}

func TestWechatPayNotifyCompletesPaidTopUpWhenNewOrdersDisabled(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayNotifyRouteForTest(router)
	platformPrivateKey := configureWechatPayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingWechatTopUpForNotifyTest(t, "WECHAT-NOTIFY-DISABLED", model.PaymentProviderWechat, 20)
	setting.WechatPayEnabled = false

	body, headers := buildSignedWechatPayNotifyBody(t, platformPrivateKey, baseWechatPayTransaction("WECHAT-NOTIFY-DISABLED", 2000, service.WechatPayTradeStateSuccess), service.WechatPayNotifySuccessEvent)
	recorder := postWechatPayNotifyForTest(t, router, body, headers)
	require.Equal(t, http.StatusOK, recorder.Code)

	topUp := model.GetTopUpByTradeNo("WECHAT-NOTIFY-DISABLED")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.InDelta(t, 10.0, getWalletBalanceForNotifyTest(t, 1), 0.000001)

	recorder = postWechatPayNotifyForTest(t, router, body, headers)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.InDelta(t, 10.0, getWalletBalanceForNotifyTest(t, 1), 0.000001)
}

func TestWechatPayNotifyCompletesVipActivationAndDuplicateIsIdempotent(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayNotifyRouteForTest(router)
	platformPrivateKey := configureWechatPayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	order, err := service.CreateVipActivationOrder(1, model.PaymentProviderWechat, model.PaymentMethodWechatDirect)
	require.NoError(t, err)

	body, headers := buildSignedWechatPayNotifyBody(t, platformPrivateKey, baseWechatPayTransaction(order.TradeNo, 168000, service.WechatPayTradeStateSuccess), service.WechatPayNotifySuccessEvent)
	recorder := postWechatPayNotifyForTest(t, router, body, headers)
	require.Equal(t, http.StatusOK, recorder.Code)

	record, err := model.GetVipActivationRecordByTradeNo(order.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, model.VipActivationStatusSuccess, record.Status)
	assert.Greater(t, record.ActivatedAt, int64(0))

	recorder = postWechatPayNotifyForTest(t, router, body, headers)
	require.Equal(t, http.StatusOK, recorder.Code)

	var flowCount int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).Where("biz_no = ?", order.TradeNo).Count(&flowCount).Error)
	assert.Equal(t, int64(1), flowCount)
	assert.Equal(t, int64(2), countPaymentCallbackLogsForTest(t, order.TradeNo))
}

func TestWechatPayNotifyIgnoresNonSuccessStatus(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayNotifyRouteForTest(router)
	platformPrivateKey := configureWechatPayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingWechatTopUpForNotifyTest(t, "WECHAT-NOTIFY-WAIT", model.PaymentProviderWechat, 20)

	body, headers := buildSignedWechatPayNotifyBody(t, platformPrivateKey, baseWechatPayTransaction("WECHAT-NOTIFY-WAIT", 2000, "NOTPAY"), service.WechatPayNotifySuccessEvent)
	recorder := postWechatPayNotifyForTest(t, router, body, headers)

	require.Equal(t, http.StatusOK, recorder.Code)
	topUp := model.GetTopUpByTradeNo("WECHAT-NOTIFY-WAIT")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)
	assert.InDelta(t, 0.0, getWalletBalanceForNotifyTest(t, 1), 0.000001)

	var log model.PaymentCallbackLog
	require.NoError(t, model.DB.Where("trade_no = ?", "WECHAT-NOTIFY-WAIT").First(&log).Error)
	assert.True(t, log.VerifyStatus)
	assert.Equal(t, model.PaymentProcessStatusSuccess, log.ProcessStatus)
}

func TestWechatPayNotifyRejectsOversizedBodyBeforeProcessing(t *testing.T) {
	router := setupMockDebugPaymentControllerTest(t)
	addWechatPayNotifyRouteForTest(router)
	_ = configureWechatPayNotifyControllerTest(t)
	seedMockDebugUser(t, 1)
	createPendingWechatTopUpForNotifyTest(t, "WECHAT-NOTIFY-LARGE", model.PaymentProviderWechat, 20)

	recorder := postWechatPayNotifyForTest(t, router, bytes.Repeat([]byte("x"), 1024*1024+1), nil)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "过大")
	topUp := model.GetTopUpByTradeNo("WECHAT-NOTIFY-LARGE")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)
	assert.Equal(t, int64(0), countPaymentCallbackLogsForTest(t, "WECHAT-NOTIFY-LARGE"))
}
