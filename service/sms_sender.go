package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// SmsSendRequest 是发送短信验证码的标准入参。
type SmsSendRequest struct {
	PhoneNumber string
	Code        string
	Purpose     string
}

// SmsSender 定义短信发送供应商接口，控制器和业务服务不直接依赖第三方 SDK。
type SmsSender interface {
	Send(ctx context.Context, req SmsSendRequest) error
}

var (
	smsSenderMu      sync.RWMutex
	configuredSender SmsSender
)

// SetSmsSender 设置短信发送器，测试可注入 fake sender；传 nil 时恢复默认配置发送器。
func SetSmsSender(sender SmsSender) {
	smsSenderMu.Lock()
	defer smsSenderMu.Unlock()
	configuredSender = sender
}

func getSmsSender() SmsSender {
	smsSenderMu.RLock()
	sender := configuredSender
	smsSenderMu.RUnlock()
	if sender != nil {
		return sender
	}
	return NewSmsSenderFromConfig()
}

// NewSmsSenderFromConfig 根据系统配置创建短信发送器。
func NewSmsSenderFromConfig() SmsSender {
	provider := strings.ToLower(strings.TrimSpace(common.SmsProvider))
	switch provider {
	case "", "aliyun":
		return NewAliyunSmsSenderFromConfig()
	default:
		return &UnsupportedSmsSender{Provider: common.SmsProvider}
	}
}

// UnsupportedSmsSender 用于兜底不支持的短信供应商配置。
type UnsupportedSmsSender struct {
	Provider string
}

// Send 返回明确的供应商不支持错误，避免误走默认供应商。
func (s *UnsupportedSmsSender) Send(ctx context.Context, req SmsSendRequest) error {
	return fmt.Errorf("unsupported SMS provider: %s", s.Provider)
}

// FakeSmsMessage 记录测试短信发送内容。
type FakeSmsMessage struct {
	PhoneNumber string
	Code        string
	Purpose     string
}

// FakeSmsSender 用于单元测试，避免真实调用短信供应商。
type FakeSmsSender struct {
	SendErr  error
	mu       sync.Mutex
	messages []FakeSmsMessage
}

// NewFakeSmsSender 创建测试短信发送器。
func NewFakeSmsSender() *FakeSmsSender {
	return &FakeSmsSender{}
}

// Send 记录短信发送请求。
func (s *FakeSmsSender) Send(ctx context.Context, req SmsSendRequest) error {
	if s.SendErr != nil {
		return s.SendErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, FakeSmsMessage{
		PhoneNumber: req.PhoneNumber,
		Code:        req.Code,
		Purpose:     req.Purpose,
	})
	return nil
}

// Messages 返回已发送短信快照。
func (s *FakeSmsSender) Messages() []FakeSmsMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]FakeSmsMessage, len(s.messages))
	copy(result, s.messages)
	return result
}

// AliyunSmsSender 调用阿里云 Dysmsapi SendSms 接口发送验证码。
type AliyunSmsSender struct {
	Endpoint        string
	AccessKeyId     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
	HTTPClient      *http.Client
}

// NewAliyunSmsSenderFromConfig 从系统配置构造阿里云短信发送器。
func NewAliyunSmsSenderFromConfig() *AliyunSmsSender {
	return &AliyunSmsSender{
		Endpoint:        common.AliyunSmsEndpoint,
		AccessKeyId:     common.AliyunSmsAccessKeyId,
		AccessKeySecret: common.AliyunSmsAccessKeySecret,
		SignName:        common.AliyunSmsSignName,
		TemplateCode:    common.AliyunSmsTemplateCode,
		HTTPClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

// Send 发送阿里云短信验证码。
func (s *AliyunSmsSender) Send(ctx context.Context, req SmsSendRequest) error {
	if !common.SmsEnabled {
		return errors.New("SMS service is disabled")
	}
	if strings.TrimSpace(common.SmsProvider) != "" && strings.TrimSpace(common.SmsProvider) != "aliyun" {
		return fmt.Errorf("unsupported SMS provider: %s", common.SmsProvider)
	}
	if strings.TrimSpace(s.AccessKeyId) == "" || strings.TrimSpace(s.AccessKeySecret) == "" || strings.TrimSpace(s.SignName) == "" || strings.TrimSpace(s.TemplateCode) == "" {
		return errors.New("Aliyun SMS configuration is incomplete")
	}

	endpoint := strings.TrimSpace(s.Endpoint)
	if endpoint == "" {
		endpoint = "https://dysmsapi.aliyuncs.com"
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}

	templateParamBytes, err := common.Marshal(map[string]string{"code": req.Code})
	if err != nil {
		return err
	}
	params := map[string]string{
		"AccessKeyId":      s.AccessKeyId,
		"Action":           "SendSms",
		"Format":           "JSON",
		"PhoneNumbers":     phoneNumberForAliyun(req.PhoneNumber),
		"RegionId":         "cn-hangzhou",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   common.GetUUID(),
		"SignatureVersion": "1.0",
		"SignName":         s.SignName,
		"TemplateCode":     s.TemplateCode,
		"TemplateParam":    string(templateParamBytes),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          "2017-05-25",
	}
	params["Signature"] = aliyunSmsSignature("POST", params, s.AccessKeySecret)

	form := url.Values{}
	for key, value := range params {
		form.Set(key, value)
	}

	httpClient := s.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code      string `json:"Code"`
		Message   string `json:"Message"`
		RequestId string `json:"RequestId"`
	}
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Aliyun SMS HTTP status %d: %s", resp.StatusCode, result.Message)
	}
	if result.Code != "OK" {
		if result.Message == "" {
			result.Message = result.Code
		}
		common.SysLog(fmt.Sprintf("Aliyun SMS send failed phone=%s request_id=%s code=%s", common.MaskPhone(req.PhoneNumber), result.RequestId, result.Code))
		return errors.New(result.Message)
	}
	return nil
}

func phoneNumberForAliyun(phoneNumber string) string {
	if strings.HasPrefix(phoneNumber, "+86") && len(phoneNumber) == 14 {
		return strings.TrimPrefix(phoneNumber, "+86")
	}
	return strings.TrimPrefix(phoneNumber, "+")
}

func aliyunSmsSignature(method string, params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	canonicalized := make([]string, 0, len(keys))
	for _, key := range keys {
		canonicalized = append(canonicalized, aliyunPercentEncode(key)+"="+aliyunPercentEncode(params[key]))
	}
	stringToSign := method + "&%2F&" + aliyunPercentEncode(strings.Join(canonicalized, "&"))
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func aliyunPercentEncode(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}
