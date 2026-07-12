package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

func maskChineseIDCard(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 6 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:3]) + strings.Repeat("*", len(runes)-6) + string(runes[len(runes)-3:])
}

func maskMobile(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) < 7 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}

func maskBankCard(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= 8 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:4]) + strings.Repeat("*", len(runes)-8) + string(runes[len(runes)-4:])
}

func maskPiggySecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 8 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:4]) + strings.Repeat("*", len(runes)-8) + string(runes[len(runes)-4:])
}

func yuanToCents(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	amount, err := decimal.NewFromString(value)
	if err != nil {
		return 0, fmt.Errorf("金额格式错误: %w", err)
	}
	return amount.Mul(decimal.NewFromInt(100)).Round(0).IntPart(), nil
}

func floatYuanToCents(value float64) int64 {
	return decimal.NewFromFloat(value).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

func centsToYuanString(cents int64) string {
	return decimal.NewFromInt(cents).Div(decimal.NewFromInt(100)).StringFixed(2)
}

func centsToFloat(cents int64) float64 {
	return decimal.NewFromInt(cents).Div(decimal.NewFromInt(100)).InexactFloat64()
}

type piggyPlatformFeeCalculation struct {
	RequestedAmountCents   int64
	PlatformFeeRate        float64
	PlatformFeeAmountCents int64
	PiggyPayAmountCents    int64
	RequestedAmount        string
	PlatformFeeAmount      string
	PiggyTaxBeforeAmount   string
}

func calculatePiggyPlatformFee(requestedAmountCents int64, platformFeeRate float64) (*piggyPlatformFeeCalculation, error) {
	if requestedAmountCents <= 0 {
		return nil, ErrWithdrawAmountInvalid
	}
	if err := operation_setting.ValidatePiggyWithdrawPlatformFeeRate(platformFeeRate); err != nil {
		return nil, err
	}
	// 平台费必须用分和 decimal 计算，避免 trial、submit、callback 之间出现浮点分歧。
	feeCents := decimal.NewFromInt(requestedAmountCents).
		Mul(decimal.NewFromFloat(platformFeeRate)).
		Div(decimal.NewFromInt(100)).
		Round(0).
		IntPart()
	piggyPayCents := requestedAmountCents - feeCents
	if piggyPayCents <= 0 {
		return nil, errors.New("小猪提现扣除平台服务费后的打款金额必须大于 0")
	}
	return &piggyPlatformFeeCalculation{
		RequestedAmountCents:   requestedAmountCents,
		PlatformFeeRate:        platformFeeRate,
		PlatformFeeAmountCents: feeCents,
		PiggyPayAmountCents:    piggyPayCents,
		RequestedAmount:        centsToYuanString(requestedAmountCents),
		PlatformFeeAmount:      centsToYuanString(feeCents),
		PiggyTaxBeforeAmount:   centsToYuanString(piggyPayCents),
	}, nil
}

func digestPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func digestAny(value any) string {
	data, err := common.Marshal(value)
	if err != nil {
		return ""
	}
	return digestPayload(data)
}

func piggyMD5Lower(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func piggyCompactJSON(value any) (string, error) {
	data, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func piggySignJSON(appSecret string, params map[string]any) (string, error) {
	filtered := make(map[string]any)
	for key, value := range params {
		if key == "sign" || value == nil {
			continue
		}
		if str, ok := value.(string); ok && strings.TrimSpace(str) == "" {
			continue
		}
		filtered[key] = value
	}
	jsonText, err := piggyCompactJSON(filtered)
	if err != nil {
		return "", err
	}
	return piggyMD5Lower(appSecret + jsonText), nil
}

func piggySignForm(appSecret string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	builder.WriteString(appSecret)
	for index, key := range keys {
		if index > 0 {
			builder.WriteString("&")
		}
		builder.WriteString(key)
		builder.WriteString("=")
		// 电子合同 Form 接口使用小猪签名 V1：签名原文中的参数值需要 UrlEncode，请求体仍交给 HTTP client 正常编码。
		builder.WriteString(url.QueryEscape(params[key]))
	}
	return piggyMD5Lower(builder.String())
}

func pickPiggyFormSignParams(form map[string]string, signedFields []string) map[string]string {
	if len(signedFields) == 0 {
		return form
	}
	result := make(map[string]string, len(signedFields))
	for _, key := range signedFields {
		if value, ok := form[key]; ok {
			result[key] = value
		}
	}
	return result
}

func piggyEncryptAES(plainText []byte, appSecret string, iv string) (string, error) {
	block, err := aes.NewCipher([]byte(appSecret))
	if err != nil {
		return "", err
	}
	ivBytes := []byte(iv)
	if len(ivBytes) != block.BlockSize() {
		return "", fmt.Errorf("AES IV 必须是 %d 字节", block.BlockSize())
	}
	padded := pkcs5Padding(plainText, block.BlockSize())
	cipherText := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, ivBytes).CryptBlocks(cipherText, padded)
	return url.QueryEscape(base64.StdEncoding.EncodeToString(cipherText)), nil
}

func piggyDecryptAES(cipherText string, appSecret string, iv string) ([]byte, error) {
	decodedURL, err := url.QueryUnescape(strings.TrimSpace(cipherText))
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(decodedURL)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(appSecret))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || len(raw)%block.BlockSize() != 0 {
		return nil, errors.New("AES 密文长度无效")
	}
	ivBytes := []byte(iv)
	if len(ivBytes) != block.BlockSize() {
		return nil, fmt.Errorf("AES IV 必须是 %d 字节", block.BlockSize())
	}
	plain := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, ivBytes).CryptBlocks(plain, raw)
	return pkcs5UnPadding(plain)
}

func pkcs5Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := strings.Repeat(string(rune(padding)), padding)
	return append(data, []byte(padText)...)
}

func pkcs5UnPadding(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("AES 明文为空")
	}
	padding := int(data[len(data)-1])
	if padding <= 0 || padding > len(data) {
		return nil, errors.New("AES padding 无效")
	}
	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return nil, errors.New("AES padding 不一致")
		}
	}
	return data[:len(data)-padding], nil
}

func isValidUTF8String(value string) bool {
	return utf8.ValidString(value)
}
