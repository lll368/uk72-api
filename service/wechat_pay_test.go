package service

import (
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateWechatPayTestKeys(t *testing.T) (privatePEM string, privatePlain string, publicPEM string, publicPlain string, key *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateDER}))
	privatePlain = base64.StdEncoding.EncodeToString(privateDER)

	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	publicPlain = base64.StdEncoding.EncodeToString(publicDER)
	return privatePEM, privatePlain, publicPEM, publicPlain, privateKey
}

func generateWechatPayTestCertificate(t *testing.T, key *rsa.PrivateKey) (certPEM string, certPlain string) {
	t.Helper()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Wechat Pay Platform Test",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	certPlain = base64.StdEncoding.EncodeToString(certDER)
	return certPEM, certPlain
}

func configureWechatPayNativeOrderServiceTest(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	merchantPrivatePEM, _, platformPublicPEM, _, platformPrivateKey := generateWechatPayTestKeys(t)
	originalAppID := setting.WechatPayAppId
	originalMchID := setting.WechatPayMchId
	originalMerchantSerialNo := setting.WechatPayMerchantSerialNo
	originalMerchantPrivateKey := setting.WechatPayMerchantPrivateKey
	originalPlatformSerialNo := setting.WechatPayPlatformSerialNo
	originalPlatformPublicKey := setting.WechatPayPlatformPublicKey
	t.Cleanup(func() {
		setting.WechatPayAppId = originalAppID
		setting.WechatPayMchId = originalMchID
		setting.WechatPayMerchantSerialNo = originalMerchantSerialNo
		setting.WechatPayMerchantPrivateKey = originalMerchantPrivateKey
		setting.WechatPayPlatformSerialNo = originalPlatformSerialNo
		setting.WechatPayPlatformPublicKey = originalPlatformPublicKey
	})

	setting.WechatPayAppId = "wx1234567890abcdef"
	setting.WechatPayMchId = "1900000001"
	setting.WechatPayMerchantSerialNo = "7777777777777777777777777777777777777777"
	setting.WechatPayMerchantPrivateKey = merchantPrivatePEM
	setting.WechatPayPlatformSerialNo = "8888888888888888888888888888888888888888"
	setting.WechatPayPlatformPublicKey = platformPublicPEM
	return platformPrivateKey, platformPublicPEM
}

func TestBuildWechatPayAuthorizationMessage(t *testing.T) {
	message := buildWechatPayAuthorizationMessage("POST", "/v3/pay/transactions/native", "1710000000", "nonce-1", `{"amount":100}`)

	assert.Equal(t, "POST\n/v3/pay/transactions/native\n1710000000\nnonce-1\n{\"amount\":100}\n", message)
}

func TestWechatPayKeyParsingSupportsPEMAndPlainBase64(t *testing.T) {
	privatePEM, privatePlain, publicPEM, publicPlain, privateKey := generateWechatPayTestKeys(t)
	certPEM, certPlain := generateWechatPayTestCertificate(t, privateKey)

	_, err := parseWechatPayPrivateKey(privatePEM)
	require.NoError(t, err)
	_, err = parseWechatPayPrivateKey(privatePlain)
	require.NoError(t, err)
	_, err = parseWechatPayPublicKey(publicPEM)
	require.NoError(t, err)
	_, err = parseWechatPayPublicKey(publicPlain)
	require.NoError(t, err)
	_, err = parseWechatPayPublicKey(certPEM)
	require.NoError(t, err)
	_, err = parseWechatPayPublicKey(certPlain)
	require.NoError(t, err)
}

func TestWechatPayAmountToCentsRoundsToMinorUnits(t *testing.T) {
	cents, err := wechatPayAmountToCents(12.345)

	require.NoError(t, err)
	assert.Equal(t, int64(1235), cents)
}

func TestCreateWechatPayNativeOrderSignsAndSendsRequest(t *testing.T) {
	platformPrivateKey, publicPEM := configureWechatPayNativeOrderServiceTest(t)

	var capturedBody []byte
	var capturedAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v3/pay/transactions/native", r.URL.Path)
		capturedAuthorization = r.Header.Get("Authorization")
		capturedBody, _ = io.ReadAll(r.Body)
		body := []byte(`{"code_url":"weixin://wxpay/bizpayurl?pr=abc"}`)
		writeSignedWechatPayResponseForTest(t, w, body, platformPrivateKey, strconv.FormatInt(time.Now().Unix(), 10), "response-nonce", setting.WechatPayPlatformSerialNo)
	}))
	defer server.Close()

	originalGatewayURL := WechatPayGatewayURL
	t.Cleanup(func() {
		WechatPayGatewayURL = originalGatewayURL
	})
	WechatPayGatewayURL = server.URL

	result, err := CreateWechatPayNativeOrder(context.Background(), WechatPayNativeOrderRequest{
		OutTradeNo:  "WX-NATIVE-ORDER",
		Description: "Recharge",
		PaidAmount:  12.34,
		NotifyURL:   "https://api.example.com/api/wechat/notify",
		Attach: map[string]string{
			"biz_type": PaymentBizTypeTopUp,
		},
		Now:   time.Unix(1710000000, 0),
		Nonce: "nonce-1",
	})
	require.NoError(t, err)

	assert.Equal(t, "weixin://wxpay/bizpayurl?pr=abc", result.CodeURL)
	assert.Equal(t, "WX-NATIVE-ORDER", result.TradeNo)
	require.Contains(t, capturedAuthorization, `mchid="1900000001"`)
	require.Contains(t, capturedAuthorization, `serial_no="7777777777777777777777777777777777777777"`)
	require.Contains(t, capturedAuthorization, `nonce_str="nonce-1"`)
	require.Contains(t, capturedAuthorization, `timestamp="1710000000"`)

	signature := extractWechatPayAuthorizationValueForTest(t, capturedAuthorization, "signature")
	message := buildWechatPayAuthorizationMessage(http.MethodPost, "/v3/pay/transactions/native", "1710000000", "nonce-1", string(capturedBody))
	require.NoError(t, verifyWechatPaySignature(message, signature, publicPEM))

	var payload map[string]any
	require.NoError(t, common.Unmarshal(capturedBody, &payload))
	assert.Equal(t, "wx1234567890abcdef", payload["appid"])
	assert.Equal(t, "1900000001", payload["mchid"])
	assert.Equal(t, "WX-NATIVE-ORDER", payload["out_trade_no"])
	assert.Equal(t, "https://api.example.com/api/wechat/notify", payload["notify_url"])
	require.IsType(t, map[string]any{}, payload["amount"])
	amount := payload["amount"].(map[string]any)
	assert.Equal(t, float64(1234), amount["total"])
	assert.Equal(t, "CNY", amount["currency"])
	assert.NotEmpty(t, payload["attach"])
}

func TestCreateWechatPayNativeOrderRejectsInvalidSuccessResponseSignature(t *testing.T) {
	tests := []struct {
		name                  string
		writeResponse         func(t *testing.T, w http.ResponseWriter, privateKey *rsa.PrivateKey)
		expectedErrorContains []string
	}{
		{
			name: "missing signature",
			writeResponse: func(t *testing.T, w http.ResponseWriter, privateKey *rsa.PrivateKey) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code_url":"weixin://wxpay/bizpayurl?pr=missing-signature"}`))
			},
		},
		{
			name: "tampered body",
			writeResponse: func(t *testing.T, w http.ResponseWriter, privateKey *rsa.PrivateKey) {
				signedBody := []byte(`{"code_url":"weixin://wxpay/bizpayurl?pr=original"}`)
				writtenBody := []byte(`{"code_url":"weixin://wxpay/bizpayurl?pr=tampered"}`)
				writeSignedWechatPayResponseForTest(t, w, writtenBody, privateKey, strconv.FormatInt(time.Now().Unix(), 10), "response-nonce", setting.WechatPayPlatformSerialNo, signedBody)
			},
		},
		{
			name: "serial mismatch",
			writeResponse: func(t *testing.T, w http.ResponseWriter, privateKey *rsa.PrivateKey) {
				body := []byte(`{"code_url":"weixin://wxpay/bizpayurl?pr=serial-mismatch"}`)
				writeSignedWechatPayResponseForTest(t, w, body, privateKey, strconv.FormatInt(time.Now().Unix(), 10), "response-nonce", "9999999999999999999999999999999999999999")
			},
			expectedErrorContains: []string{
				"configured_serial=8888888888888888888888888888888888888888",
				"response_serial=9999999999999999999999999999999999999999",
			},
		},
		{
			name: "expired timestamp",
			writeResponse: func(t *testing.T, w http.ResponseWriter, privateKey *rsa.PrivateKey) {
				body := []byte(`{"code_url":"weixin://wxpay/bizpayurl?pr=expired"}`)
				writeSignedWechatPayResponseForTest(t, w, body, privateKey, strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10), "response-nonce", setting.WechatPayPlatformSerialNo)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platformPrivateKey, _ := configureWechatPayNativeOrderServiceTest(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.writeResponse(t, w, platformPrivateKey)
			}))
			defer server.Close()

			originalGatewayURL := WechatPayGatewayURL
			t.Cleanup(func() {
				WechatPayGatewayURL = originalGatewayURL
			})
			WechatPayGatewayURL = server.URL

			result, err := CreateWechatPayNativeOrder(context.Background(), WechatPayNativeOrderRequest{
				OutTradeNo:  "WX-NATIVE-ORDER-" + strings.ReplaceAll(tt.name, " ", "-"),
				Description: "Recharge",
				PaidAmount:  12.34,
				NotifyURL:   "https://api.example.com/api/wechat/notify",
				Now:         time.Unix(1710000000, 0),
				Nonce:       "nonce-1",
			})

			require.Error(t, err)
			assert.Nil(t, result)
			for _, expected := range tt.expectedErrorContains {
				assert.Contains(t, err.Error(), expected)
			}
		})
	}
}

func TestVerifyWechatPayNotifySignatureRejectsTamperedBody(t *testing.T) {
	privatePEM, _, publicPEM, _, privateKey := generateWechatPayTestKeys(t)
	_ = privatePEM
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := buildWechatPayNotifyMessage(timestamp, "nonce-1", []byte(`{"id":"notify"}`))
	signature := signWechatPayMessageForTest(t, privateKey, message)

	err := VerifyWechatPayNotifySignature(timestamp, "nonce-1", []byte(`{"id":"tampered"}`), signature, publicPEM)

	require.Error(t, err)
}

func TestVerifyWechatPayNotifySignatureRejectsExpiredTimestamp(t *testing.T) {
	_, _, publicPEM, _, privateKey := generateWechatPayTestKeys(t)
	body := []byte(`{"id":"notify"}`)
	timestamp := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	message := buildWechatPayNotifyMessage(timestamp, "nonce-1", body)
	signature := signWechatPayMessageForTest(t, privateKey, message)

	err := VerifyWechatPayNotifySignature(timestamp, "nonce-1", body, signature, publicPEM)

	require.Error(t, err)
}

func TestDecryptWechatPayResource(t *testing.T) {
	apiKey := "0123456789abcdef0123456789abcdef"
	plaintext := []byte(`{"out_trade_no":"WX-NOTIFY","trade_state":"SUCCESS"}`)
	associatedData := "transaction"
	nonce := "notify-nonce"
	cipherText := encryptWechatPayResourceForTest(t, apiKey, nonce, associatedData, plaintext)

	decrypted, err := DecryptWechatPayResource(WechatPayNotifyResource{
		Algorithm:      "AEAD_AES_256_GCM",
		Ciphertext:     cipherText,
		AssociatedData: associatedData,
		Nonce:          nonce,
	}, apiKey)

	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func extractWechatPayAuthorizationValueForTest(t *testing.T, header string, key string) string {
	t.Helper()
	prefix := key + `="`
	start := strings.Index(header, prefix)
	require.NotEqual(t, -1, start)
	start += len(prefix)
	end := strings.Index(header[start:], `"`)
	require.NotEqual(t, -1, end)
	return header[start : start+end]
}

func signWechatPayMessageForTest(t *testing.T, privateKey *rsa.PrivateKey, message string) string {
	t.Helper()
	hash := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(signature)
}

func writeSignedWechatPayResponseForTest(t *testing.T, w http.ResponseWriter, body []byte, privateKey *rsa.PrivateKey, timestamp string, nonce string, serial string, signedBody ...[]byte) {
	t.Helper()
	bodyForSignature := body
	if len(signedBody) > 0 {
		bodyForSignature = signedBody[0]
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Wechatpay-Timestamp", timestamp)
	w.Header().Set("Wechatpay-Nonce", nonce)
	w.Header().Set("Wechatpay-Serial", serial)
	w.Header().Set("Wechatpay-Signature", signWechatPayMessageForTest(t, privateKey, buildWechatPayNotifyMessage(timestamp, nonce, bodyForSignature)))
	_, _ = w.Write(body)
}

func encryptWechatPayResourceForTest(t *testing.T, apiKey string, nonce string, associatedData string, plaintext []byte) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(apiKey))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	ciphertext := aead.Seal(nil, []byte(nonce), plaintext, []byte(associatedData))
	return base64.StdEncoding.EncodeToString(ciphertext)
}
