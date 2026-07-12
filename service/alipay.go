package service

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
)

const (
	AlipayMethodPagePay       = "alipay.trade.page.pay"
	AlipayProductCodePagePay  = "FAST_INSTANT_TRADE_PAY"
	AlipayTradeStatusSuccess  = "TRADE_SUCCESS"
	AlipayTradeStatusFinished = "TRADE_FINISHED"
	AlipayTradeStatusClosed   = "TRADE_CLOSED"
	AlipayCharsetUTF8         = "utf-8"
)

type AlipayPagePayRequest struct {
	OutTradeNo     string
	Subject        string
	TotalAmount    string
	ReturnURL      string
	NotifyURL      string
	PassbackParams map[string]string
	Now            time.Time
}

type AlipayPagePayResult struct {
	GatewayURL string
	Params     map[string]string
}

func BuildAlipayPagePayParams(req AlipayPagePayRequest) (*AlipayPagePayResult, error) {
	if strings.TrimSpace(setting.AlipayAppId) == "" {
		return nil, errors.New("未配置支付宝应用 ID")
	}
	if strings.TrimSpace(setting.AlipayPrivateKey) == "" {
		return nil, errors.New("未配置支付宝商户私钥")
	}
	if strings.TrimSpace(req.OutTradeNo) == "" {
		return nil, errors.New("未提供支付宝商户订单号")
	}
	if strings.TrimSpace(req.Subject) == "" {
		return nil, errors.New("未提供支付宝订单标题")
	}
	if strings.TrimSpace(req.TotalAmount) == "" {
		return nil, errors.New("未提供支付宝订单金额")
	}

	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	returnURL := strings.TrimSpace(req.ReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(setting.AlipayReturnUrl)
	}
	notifyURL := strings.TrimSpace(req.NotifyURL)
	if notifyURL == "" {
		notifyURL = strings.TrimSpace(setting.AlipayNotifyUrl)
	}

	bizContent := map[string]any{
		"out_trade_no": strings.TrimSpace(req.OutTradeNo),
		"product_code": AlipayProductCodePagePay,
		"total_amount": strings.TrimSpace(req.TotalAmount),
		"subject":      strings.TrimSpace(req.Subject),
	}
	if len(req.PassbackParams) > 0 {
		payload, err := common.Marshal(req.PassbackParams)
		if err != nil {
			return nil, err
		}
		// 支付宝 passback_params 是字符串，包含特殊字符时按官方要求 URL 编码。
		bizContent["passback_params"] = url.QueryEscape(string(payload))
	}

	bizContentBytes, err := common.Marshal(bizContent)
	if err != nil {
		return nil, err
	}

	params := map[string]string{
		"app_id":      strings.TrimSpace(setting.AlipayAppId),
		"method":      AlipayMethodPagePay,
		"format":      "JSON",
		"charset":     AlipayCharsetUTF8,
		"sign_type":   "RSA2",
		"timestamp":   now.Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": string(bizContentBytes),
	}
	if returnURL != "" {
		params["return_url"] = returnURL
	}
	if notifyURL != "" {
		params["notify_url"] = notifyURL
	}

	sign, err := signAlipayParams(params, setting.AlipayPrivateKey)
	if err != nil {
		return nil, err
	}
	params["sign"] = sign
	// 支付宝表单 POST 会先从 action URL 读取 charset，再按该字符集解析隐藏字段。
	formParams := make(map[string]string, len(params)-1)
	for key, value := range params {
		if key == "charset" {
			continue
		}
		formParams[key] = value
	}
	return &AlipayPagePayResult{
		GatewayURL: buildAlipayGatewayURLWithCharset(setting.GetAlipayGatewayURL(), AlipayCharsetUTF8),
		Params:     formParams,
	}, nil
}

func buildAlipayGatewayURLWithCharset(gatewayURL string, charset string) string {
	gatewayURL = strings.TrimSpace(gatewayURL)
	charset = strings.TrimSpace(charset)
	if gatewayURL == "" || charset == "" {
		return gatewayURL
	}
	parsed, err := url.Parse(gatewayURL)
	if err != nil {
		separator := "?"
		if strings.Contains(gatewayURL, "?") {
			separator = "&"
		}
		return gatewayURL + separator + "charset=" + url.QueryEscape(charset)
	}
	query := parsed.Query()
	query.Set("charset", charset)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func IsAlipaySuccessTradeStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case AlipayTradeStatusSuccess, AlipayTradeStatusFinished:
		return true
	default:
		return false
	}
}

func IsAlipayClosedTradeStatus(status string) bool {
	return strings.TrimSpace(status) == AlipayTradeStatusClosed
}

func NormalizeAlipayNotifyValues(values url.Values) map[string]string {
	params := make(map[string]string, len(values))
	for key, value := range values {
		if len(value) == 0 {
			continue
		}
		params[key] = value[0]
	}
	return params
}

func VerifyConfiguredAlipayNotify(params map[string]string) error {
	return verifyAlipayParams(params, setting.AlipayPublicKey)
}

func buildAlipayRequestSignContent(params map[string]string) string {
	return buildAlipaySignContent(params, true)
}

func buildAlipayNotifySignContent(params map[string]string) string {
	return buildAlipaySignContent(params, false)
}

func buildAlipaySignContent(params map[string]string, includeSignType bool) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		key = strings.TrimSpace(key)
		if key == "" || key == "sign" || (!includeSignType && key == "sign_type") || value == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	return strings.Join(parts, "&")
}

func signAlipayParams(params map[string]string, privateKey string) (string, error) {
	key, err := parseAlipayPrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	content := buildAlipayRequestSignContent(params)
	hash := sha256.Sum256([]byte(content))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func verifyAlipayRequestParams(params map[string]string, publicKey string) error {
	return verifyAlipaySignature(params, publicKey, true)
}

func verifyAlipayParams(params map[string]string, publicKey string) error {
	return verifyAlipaySignature(params, publicKey, false)
}

func verifyAlipaySignature(params map[string]string, publicKey string, includeSignType bool) error {
	signatureText := strings.TrimSpace(params["sign"])
	if signatureText == "" {
		return errors.New("支付宝回调缺少签名")
	}
	signature, err := base64.StdEncoding.DecodeString(signatureText)
	if err != nil {
		return fmt.Errorf("支付宝回调签名格式错误: %w", err)
	}
	key, err := parseAlipayPublicKey(publicKey)
	if err != nil {
		return err
	}
	content := buildAlipaySignContent(params, includeSignType)
	hash := sha256.Sum256([]byte(content))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], signature); err != nil {
		return fmt.Errorf("支付宝回调验签失败: %w", err)
	}
	return nil
}

func parseAlipayPrivateKey(raw string) (*rsa.PrivateKey, error) {
	der, err := decodeAlipayKey(raw)
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("支付宝商户私钥解析失败: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("支付宝商户私钥不是 RSA 私钥")
	}
	return key, nil
}

func parseAlipayPublicKey(raw string) (*rsa.PublicKey, error) {
	der, err := decodeAlipayKey(raw)
	if err != nil {
		return nil, err
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err == nil {
		key, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("支付宝公钥不是 RSA 公钥")
		}
		return key, nil
	}
	if key, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("支付宝公钥解析失败: %w", err)
}

func decodeAlipayKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("支付宝密钥为空")
	}
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		return block.Bytes, nil
	}
	compact := strings.NewReplacer("\n", "", "\r", "", "\t", "", " ", "").Replace(raw)
	der, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return nil, fmt.Errorf("支付宝密钥必须是 PEM 或 base64 DER 格式: %w", err)
	}
	return der, nil
}
