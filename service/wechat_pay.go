package service

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/shopspring/decimal"
)

const (
	WechatPayNativeTransactionsPath = "/v3/pay/transactions/native"
	WechatPayAuthScheme             = "WECHATPAY2-SHA256-RSA2048"
	WechatPayCurrencyCNY            = "CNY"
	WechatPayNotifySuccessEvent     = "TRANSACTION.SUCCESS"
	WechatPayTradeStateSuccess      = "SUCCESS"
	WechatPayResourceAlgorithm      = "AEAD_AES_256_GCM"
	wechatPaySignatureMaxSkew       = 5 * time.Minute
)

var (
	wechatPayHTTPClient = &http.Client{Timeout: 10 * time.Second}
	WechatPayGatewayURL = setting.GetWechatPayGatewayURL()
)

type WechatPayNativeOrderRequest struct {
	OutTradeNo  string
	Description string
	PaidAmount  float64
	NotifyURL   string
	Attach      map[string]string
	Now         time.Time
	Nonce       string
}

type WechatPayNativeOrderResult struct {
	TradeNo   string `json:"trade_no"`
	CodeURL   string `json:"code_url"`
	ExpiresAt int64  `json:"expires_at"`
}

type wechatPayNativeOrderPayload struct {
	AppID       string                `json:"appid"`
	MchID       string                `json:"mchid"`
	Description string                `json:"description"`
	OutTradeNo  string                `json:"out_trade_no"`
	NotifyURL   string                `json:"notify_url"`
	Amount      wechatPayNativeAmount `json:"amount"`
	Attach      string                `json:"attach,omitempty"`
}

type wechatPayNativeAmount struct {
	Total    int64  `json:"total"`
	Currency string `json:"currency"`
}

type wechatPayNativeOrderResponse struct {
	CodeURL string `json:"code_url"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WechatPayNotifyResource struct {
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	AssociatedData string `json:"associated_data"`
	Nonce          string `json:"nonce"`
	OriginalType   string `json:"original_type"`
}

type WechatPayNotifyPayload struct {
	ID           string                  `json:"id"`
	CreateTime   string                  `json:"create_time"`
	EventType    string                  `json:"event_type"`
	ResourceType string                  `json:"resource_type"`
	Summary      string                  `json:"summary"`
	Resource     WechatPayNotifyResource `json:"resource"`
}

type WechatPayTransaction struct {
	AppID          string                     `json:"appid"`
	MchID          string                     `json:"mchid"`
	OutTradeNo     string                     `json:"out_trade_no"`
	TransactionID  string                     `json:"transaction_id"`
	TradeType      string                     `json:"trade_type"`
	TradeState     string                     `json:"trade_state"`
	TradeStateDesc string                     `json:"trade_state_desc"`
	Attach         string                     `json:"attach"`
	Amount         WechatPayTransactionAmount `json:"amount"`
}

type WechatPayTransactionAmount struct {
	Total         *int64 `json:"total"`
	PayerTotal    *int64 `json:"payer_total"`
	Currency      string `json:"currency"`
	PayerCurrency string `json:"payer_currency"`
}

func CreateWechatPayNativeOrder(ctx context.Context, req WechatPayNativeOrderRequest) (*WechatPayNativeOrderResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(setting.WechatPayAppId) == "" {
		return nil, errors.New("未配置微信支付应用 ID")
	}
	if strings.TrimSpace(setting.WechatPayMchId) == "" {
		return nil, errors.New("未配置微信支付商户号")
	}
	if strings.TrimSpace(setting.WechatPayMerchantSerialNo) == "" {
		return nil, errors.New("未配置微信支付商户证书序列号")
	}
	if strings.TrimSpace(setting.WechatPayMerchantPrivateKey) == "" {
		return nil, errors.New("未配置微信支付商户私钥")
	}
	if strings.TrimSpace(req.OutTradeNo) == "" {
		return nil, errors.New("未提供微信支付商户订单号")
	}
	if strings.TrimSpace(req.Description) == "" {
		return nil, errors.New("未提供微信支付订单描述")
	}
	if strings.TrimSpace(req.NotifyURL) == "" {
		return nil, errors.New("未提供微信支付通知地址")
	}
	totalCents, err := wechatPayAmountToCents(req.PaidAmount)
	if err != nil {
		return nil, err
	}

	attach := ""
	if len(req.Attach) > 0 {
		attachBytes, err := common.Marshal(req.Attach)
		if err != nil {
			return nil, err
		}
		attach = string(attachBytes)
	}

	payload := wechatPayNativeOrderPayload{
		AppID:       strings.TrimSpace(setting.WechatPayAppId),
		MchID:       strings.TrimSpace(setting.WechatPayMchId),
		Description: strings.TrimSpace(req.Description),
		OutTradeNo:  strings.TrimSpace(req.OutTradeNo),
		NotifyURL:   strings.TrimSpace(req.NotifyURL),
		Amount: wechatPayNativeAmount{
			Total:    totalCents,
			Currency: WechatPayCurrencyCNY,
		},
		Attach: attach,
	}
	bodyBytes, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	nonce := strings.TrimSpace(req.Nonce)
	if nonce == "" {
		nonce = common.GetRandomString(32)
	}
	requestURL := strings.TrimRight(WechatPayGatewayURL, "/") + WechatPayNativeTransactionsPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	authorization, err := buildWechatPayAuthorizationHeader(
		http.MethodPost,
		WechatPayNativeTransactionsPath,
		now.Unix(),
		nonce,
		string(bodyBytes),
	)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", authorization)

	resp, err := wechatPayHTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("微信支付 Native 下单请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取微信支付 Native 下单响应失败: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var parsed wechatPayNativeOrderResponse
		if len(respBody) > 0 {
			if err := common.Unmarshal(respBody, &parsed); err != nil {
				return nil, fmt.Errorf("解析微信支付 Native 下单响应失败: %w", err)
			}
		}
		message := strings.TrimSpace(parsed.Message)
		if message == "" {
			message = string(respBody)
		}
		return nil, fmt.Errorf("微信支付 Native 下单失败 status=%d code=%s message=%s", resp.StatusCode, parsed.Code, message)
	}
	if err := VerifyConfiguredWechatPayResponseSignature(resp.Header, respBody); err != nil {
		return nil, fmt.Errorf(
			"微信支付 Native 下单响应验签失败 configured_serial=%s response_serial=%s: %w",
			strings.TrimSpace(setting.WechatPayPlatformSerialNo),
			strings.TrimSpace(resp.Header.Get("Wechatpay-Serial")),
			err,
		)
	}
	var parsed wechatPayNativeOrderResponse
	if len(respBody) > 0 {
		if err := common.Unmarshal(respBody, &parsed); err != nil {
			return nil, fmt.Errorf("解析微信支付 Native 下单响应失败: %w", err)
		}
	}
	if strings.TrimSpace(parsed.CodeURL) == "" {
		return nil, errors.New("微信支付 Native 下单响应缺少 code_url")
	}

	return &WechatPayNativeOrderResult{
		TradeNo:   strings.TrimSpace(req.OutTradeNo),
		CodeURL:   strings.TrimSpace(parsed.CodeURL),
		ExpiresAt: now.Add(2 * time.Hour).Unix(),
	}, nil
}

func IsWechatPaySuccessNotification(eventType string, tradeState string) bool {
	return strings.TrimSpace(eventType) == WechatPayNotifySuccessEvent &&
		strings.TrimSpace(tradeState) == WechatPayTradeStateSuccess
}

func ParseWechatPayNotifyPayload(body []byte) (*WechatPayNotifyPayload, error) {
	var payload WechatPayNotifyPayload
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func ParseWechatPayTransaction(body []byte) (*WechatPayTransaction, error) {
	var transaction WechatPayTransaction
	if err := common.Unmarshal(body, &transaction); err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (a WechatPayTransactionAmount) PaidCents() int64 {
	if a.PayerTotal != nil && *a.PayerTotal > 0 {
		return *a.PayerTotal
	}
	if a.Total != nil {
		return *a.Total
	}
	return 0
}

func VerifyConfiguredWechatPayResponseSignature(header http.Header, body []byte) error {
	return verifyWechatPayPlatformSignature(
		header.Get("Wechatpay-Timestamp"),
		header.Get("Wechatpay-Nonce"),
		header.Get("Wechatpay-Serial"),
		body,
		header.Get("Wechatpay-Signature"),
		setting.WechatPayPlatformSerialNo,
		setting.WechatPayPlatformPublicKey,
	)
}

func VerifyConfiguredWechatPayNotifySignature(timestamp string, nonce string, serial string, body []byte, signature string) error {
	return verifyWechatPayPlatformSignature(timestamp, nonce, serial, body, signature, setting.WechatPayPlatformSerialNo, setting.WechatPayPlatformPublicKey)
}

func VerifyWechatPayNotifySignature(timestamp string, nonce string, body []byte, signature string, publicKey string) error {
	return verifyWechatPayPlatformSignature(timestamp, nonce, "", body, signature, "", publicKey)
}

func verifyWechatPayPlatformSignature(timestamp string, nonce string, serial string, body []byte, signature string, configuredSerial string, publicKey string) error {
	if strings.TrimSpace(timestamp) == "" {
		return errors.New("微信支付回调缺少时间戳")
	}
	if strings.TrimSpace(nonce) == "" {
		return errors.New("微信支付回调缺少随机串")
	}
	if err := validateWechatPaySignatureTimestamp(timestamp, time.Now()); err != nil {
		return err
	}
	configuredSerial = strings.TrimSpace(configuredSerial)
	if configuredSerial != "" {
		serial = strings.TrimSpace(serial)
		if serial == "" {
			return errors.New("微信支付平台证书序列号为空")
		}
		if serial != configuredSerial {
			return fmt.Errorf("微信支付平台证书序列号不匹配 configured_serial=%s response_serial=%s", configuredSerial, serial)
		}
	}
	message := buildWechatPayNotifyMessage(timestamp, nonce, body)
	return verifyWechatPaySignature(message, signature, publicKey)
}

func validateWechatPaySignatureTimestamp(timestamp string, now time.Time) error {
	signedAtUnix, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return fmt.Errorf("微信支付回调时间戳格式错误: %w", err)
	}
	skew := now.Sub(time.Unix(signedAtUnix, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > wechatPaySignatureMaxSkew {
		return errors.New("微信支付回调时间戳已过期")
	}
	return nil
}

func DecryptConfiguredWechatPayResource(resource WechatPayNotifyResource) ([]byte, error) {
	return DecryptWechatPayResource(resource, setting.WechatPayAPIv3Key)
}

func DecryptWechatPayResource(resource WechatPayNotifyResource, apiV3Key string) ([]byte, error) {
	if strings.TrimSpace(resource.Algorithm) != "" && resource.Algorithm != WechatPayResourceAlgorithm {
		return nil, fmt.Errorf("不支持的微信支付回调资源加密算法: %s", resource.Algorithm)
	}
	key := []byte(strings.TrimSpace(apiV3Key))
	if len(key) != 32 {
		return nil, errors.New("微信支付 API v3 密钥必须是 32 字节")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(resource.Ciphertext))
	if err != nil {
		return nil, fmt.Errorf("微信支付回调密文格式错误: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, []byte(resource.Nonce), ciphertext, []byte(resource.AssociatedData))
	if err != nil {
		return nil, fmt.Errorf("微信支付回调资源解密失败: %w", err)
	}
	return plaintext, nil
}

func buildWechatPayAuthorizationHeader(method string, requestPath string, timestamp int64, nonce string, body string) (string, error) {
	signature, err := signWechatPayMessage(
		buildWechatPayAuthorizationMessage(method, requestPath, fmt.Sprintf("%d", timestamp), nonce, body),
		setting.WechatPayMerchantPrivateKey,
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`%s mchid="%s",nonce_str="%s",signature="%s",timestamp="%d",serial_no="%s"`,
		WechatPayAuthScheme,
		escapeWechatPayHeaderValue(setting.WechatPayMchId),
		escapeWechatPayHeaderValue(nonce),
		escapeWechatPayHeaderValue(signature),
		timestamp,
		escapeWechatPayHeaderValue(setting.WechatPayMerchantSerialNo),
	), nil
}

func buildWechatPayAuthorizationMessage(method string, requestPath string, timestamp string, nonce string, body string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + "\n" +
		requestPath + "\n" +
		strings.TrimSpace(timestamp) + "\n" +
		strings.TrimSpace(nonce) + "\n" +
		body + "\n"
}

func buildWechatPayNotifyMessage(timestamp string, nonce string, body []byte) string {
	return strings.TrimSpace(timestamp) + "\n" +
		strings.TrimSpace(nonce) + "\n" +
		string(body) + "\n"
}

func signWechatPayMessage(message string, privateKey string) (string, error) {
	key, err := parseWechatPayPrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func verifyWechatPaySignature(message string, signatureText string, publicKey string) error {
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signatureText))
	if err != nil {
		return fmt.Errorf("微信支付回调签名格式错误: %w", err)
	}
	key, err := parseWechatPayPublicKey(publicKey)
	if err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(message))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], signature); err != nil {
		return fmt.Errorf("微信支付回调验签失败: %w", err)
	}
	return nil
}

func parseWechatPayPrivateKey(raw string) (*rsa.PrivateKey, error) {
	der, err := decodeWechatPayKey(raw)
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("微信支付商户私钥解析失败: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("微信支付商户私钥不是 RSA 私钥")
	}
	return key, nil
}

func parseWechatPayPublicKey(raw string) (*rsa.PublicKey, error) {
	der, err := decodeWechatPayKey(raw)
	if err != nil {
		return nil, err
	}
	parsed, parsePKIXErr := x509.ParsePKIXPublicKey(der)
	if parsePKIXErr == nil {
		key, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("微信支付平台公钥不是 RSA 公钥")
		}
		return key, nil
	}
	if key, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return key, nil
	}
	if cert, err := x509.ParseCertificate(der); err == nil {
		key, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("微信支付平台证书公钥不是 RSA 公钥")
		}
		return key, nil
	}
	return nil, fmt.Errorf("微信支付平台公钥或证书解析失败: %w", parsePKIXErr)
}

func decodeWechatPayKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("微信支付密钥为空")
	}
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		return block.Bytes, nil
	}
	compact := strings.NewReplacer("\n", "", "\r", "", "\t", "", " ", "").Replace(raw)
	der, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return nil, fmt.Errorf("微信支付密钥必须是 PEM 或 base64 DER 格式: %w", err)
	}
	return der, nil
}

func wechatPayAmountToCents(amount float64) (int64, error) {
	if amount <= 0 {
		return 0, errors.New("微信支付金额必须大于 0")
	}
	cents := decimal.NewFromFloat(amount).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if cents <= 0 {
		return 0, errors.New("微信支付金额过低")
	}
	return cents, nil
}

func WechatPayAmountToCents(amount float64) (int64, error) {
	return wechatPayAmountToCents(amount)
}

func escapeWechatPayHeaderValue(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(strings.TrimSpace(value))
}

func encodeWechatPayAttach(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	query := url.Values{}
	for _, key := range keys {
		query.Set(key, values[key])
	}
	return query.Encode()
}
