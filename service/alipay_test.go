package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateAlipayTestKeys(t *testing.T) (privatePEM string, privatePlain string, publicPEM string, publicPlain string) {
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
	return
}

func TestBuildAlipayRequestSignContentIncludesSignType(t *testing.T) {
	params := map[string]string{
		"b":         "2",
		"sign":      "skip-me",
		"a":         "1",
		"sign_type": "RSA2",
		"empty":     "",
	}

	content := buildAlipayRequestSignContent(params)

	assert.Equal(t, "a=1&b=2&sign_type=RSA2", content)
}

func TestBuildAlipayNotifySignContentSkipsSignAndSignType(t *testing.T) {
	params := map[string]string{
		"b":         "2",
		"sign":      "skip-me",
		"a":         "1",
		"sign_type": "RSA2",
		"empty":     "",
	}

	content := buildAlipayNotifySignContent(params)

	assert.Equal(t, "a=1&b=2", content)
}

func TestAlipayKeyParsingSupportsPEMAndPlainBase64(t *testing.T) {
	privatePEM, privatePlain, publicPEM, publicPlain := generateAlipayTestKeys(t)

	_, err := parseAlipayPrivateKey(privatePEM)
	require.NoError(t, err)
	_, err = parseAlipayPrivateKey(privatePlain)
	require.NoError(t, err)
	_, err = parseAlipayPublicKey(publicPEM)
	require.NoError(t, err)
	_, err = parseAlipayPublicKey(publicPlain)
	require.NoError(t, err)
}

func TestSignAndVerifyAlipayParams(t *testing.T) {
	privatePEM, _, publicPEM, _ := generateAlipayTestKeys(t)
	params := map[string]string{
		"app_id":       "2021000000000000",
		"method":       "alipay.trade.page.pay",
		"sign_type":    "RSA2",
		"total_amount": "10.00",
	}

	sign, err := signAlipayParams(params, privatePEM)
	require.NoError(t, err)
	params["sign"] = sign

	require.NoError(t, verifyAlipayRequestParams(params, publicPEM))

	params["sign_type"] = "RSA"
	assert.Error(t, verifyAlipayRequestParams(params, publicPEM))
}

func TestBuildAlipayPagePayParamsUsesCommonJsonAndSignsRequest(t *testing.T) {
	privatePEM, _, publicPEM, _ := generateAlipayTestKeys(t)
	originalAppID := setting.AlipayAppId
	originalPrivateKey := setting.AlipayPrivateKey
	originalReturnURL := setting.AlipayReturnUrl
	originalNotifyURL := setting.AlipayNotifyUrl
	t.Cleanup(func() {
		setting.AlipayAppId = originalAppID
		setting.AlipayPrivateKey = originalPrivateKey
		setting.AlipayReturnUrl = originalReturnURL
		setting.AlipayNotifyUrl = originalNotifyURL
	})

	setting.AlipayAppId = "2021000000000000"
	setting.AlipayPrivateKey = privatePEM
	setting.AlipayReturnUrl = "https://app.example.com/wallet"
	setting.AlipayNotifyUrl = "https://api.example.com/api/alipay/notify"

	result, err := BuildAlipayPagePayParams(AlipayPagePayRequest{
		OutTradeNo:  "TRADE123",
		Subject:     "Recharge",
		TotalAmount: "10.00",
		PassbackParams: map[string]string{
			"biz_type": "topup",
		},
		Now: time.Date(2026, 5, 27, 10, 20, 30, 0, time.FixedZone("CST", 8*3600)),
	})
	require.NoError(t, err)

	assert.Equal(t, buildAlipayGatewayURLWithCharset(setting.GetAlipayGatewayURL(), AlipayCharsetUTF8), result.GatewayURL)
	assert.Equal(t, "2021000000000000", result.Params["app_id"])
	assert.Equal(t, "alipay.trade.page.pay", result.Params["method"])
	assert.Equal(t, "RSA2", result.Params["sign_type"])
	assert.NotEmpty(t, result.Params["sign"])
	assert.NotEmpty(t, result.Params["biz_content"])
	gatewayURL, err := url.Parse(result.GatewayURL)
	require.NoError(t, err)
	assert.Equal(t, "utf-8", gatewayURL.Query().Get("charset"))
	signedParams := make(map[string]string, len(result.Params)+1)
	for key, value := range result.Params {
		signedParams[key] = value
	}
	signedParams["charset"] = gatewayURL.Query().Get("charset")
	require.NoError(t, verifyAlipayRequestParams(signedParams, publicPEM))

	var bizContent map[string]any
	require.NoError(t, common.Unmarshal([]byte(result.Params["biz_content"]), &bizContent))
	assert.Equal(t, "TRADE123", bizContent["out_trade_no"])
	assert.Equal(t, "FAST_INSTANT_TRADE_PAY", bizContent["product_code"])
	assert.Equal(t, "10.00", bizContent["total_amount"])
	assert.Equal(t, "Recharge", bizContent["subject"])
}

func TestBuildAlipayPagePayParamsPutsCharsetInGatewayURLForFormPost(t *testing.T) {
	privatePEM, _, publicPEM, _ := generateAlipayTestKeys(t)
	originalAppID := setting.AlipayAppId
	originalPrivateKey := setting.AlipayPrivateKey
	t.Cleanup(func() {
		setting.AlipayAppId = originalAppID
		setting.AlipayPrivateKey = originalPrivateKey
	})

	setting.AlipayAppId = "2021000000000000"
	setting.AlipayPrivateKey = privatePEM

	result, err := BuildAlipayPagePayParams(AlipayPagePayRequest{
		OutTradeNo:  "TRADE_CHARSET",
		Subject:     "Recharge",
		TotalAmount: "10.00",
		Now:         time.Date(2026, 5, 27, 10, 20, 30, 0, time.FixedZone("CST", 8*3600)),
	})
	require.NoError(t, err)

	gatewayURL, err := url.Parse(result.GatewayURL)
	require.NoError(t, err)
	assert.Equal(t, "utf-8", gatewayURL.Query().Get("charset"))
	assert.NotContains(t, result.Params, "charset")

	signedParams := make(map[string]string, len(result.Params)+1)
	for key, value := range result.Params {
		signedParams[key] = value
	}
	signedParams["charset"] = gatewayURL.Query().Get("charset")
	require.NoError(t, verifyAlipayRequestParams(signedParams, publicPEM))
}

func TestNormalizeAlipayNotifyValues(t *testing.T) {
	values := url.Values{}
	values.Add("out_trade_no", "TRADE123")
	values.Add("trade_status", "TRADE_SUCCESS")
	values.Add("total_amount", "10.00")
	values.Add("total_amount", "11.00")

	params := NormalizeAlipayNotifyValues(values)

	assert.Equal(t, map[string]string{
		"out_trade_no": "TRADE123",
		"trade_status": "TRADE_SUCCESS",
		"total_amount": "10.00",
	}, params)
}
