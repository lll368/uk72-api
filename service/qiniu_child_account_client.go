package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	qiniuCreateChildAccountPath  = "/v1/user/create_child"
	qiniuChildAccountKeyPath     = "/v1/user/child_key"
	qiniuDisableChildAccountPath = "/v1/user/disable-child"
	qiniuEnableChildAccountPath  = "/v1/user/enable-child"
)

var qiniuAuthorizationPattern = regexp.MustCompile(`(?i)Qiniu\s+[A-Za-z0-9_\-]+:[A-Za-z0-9_\-=]+`)
var qiniuLongSecretPattern = regexp.MustCompile(`[A-Za-z0-9_\-]{24,}`)

type qiniuChildAccountRemote struct {
	UserID     string
	UID        string
	ParentUID  string
	Email      string
	IsDisabled bool
}

type qiniuChildAccountKeys struct {
	AccessKey       string
	SecretKey       string
	State           string
	BackupAccessKey string
	BackupSecretKey string
	BackupState     string
}

func (client *qiniuKeyClient) CreateChildAccount(ctx context.Context, email string, password string) (*qiniuChildAccountRemote, error) {
	values := url.Values{}
	values.Set("email", strings.TrimSpace(email))
	values.Set("password", strings.TrimSpace(password))
	respBody, err := client.doForm(ctx, http.MethodPost, qiniuCreateChildAccountPath, values)
	if err != nil {
		return nil, err
	}
	return parseQiniuChildAccountRemote(respBody)
}

func (client *qiniuKeyClient) GetChildKey(ctx context.Context, uid string, email string) (*qiniuChildAccountKeys, error) {
	query := url.Values{}
	if strings.TrimSpace(uid) != "" {
		query.Set("uid", strings.TrimSpace(uid))
	} else if strings.TrimSpace(email) != "" {
		query.Set("email", strings.TrimSpace(email))
	} else {
		return nil, errors.New("查询子账户密钥缺少 uid 或 email")
	}
	respBody, err := client.doChildAccountJSON(ctx, http.MethodGet, qiniuChildAccountKeyPath+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	return parseQiniuChildAccountKeys(respBody)
}

func (client *qiniuKeyClient) DisableChildAccount(ctx context.Context, uid string, reason string) error {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return errors.New("禁用子账户缺少 uid")
	}
	values := url.Values{}
	values.Set("uid", uid)
	values.Set("reason", strings.TrimSpace(reason))
	_, err := client.doForm(ctx, http.MethodPost, qiniuDisableChildAccountPath, values)
	return err
}

func (client *qiniuKeyClient) EnableChildAccount(ctx context.Context, uid string) error {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return errors.New("启用子账户缺少 uid")
	}
	values := url.Values{}
	values.Set("uid", uid)
	_, err := client.doForm(ctx, http.MethodPost, qiniuEnableChildAccountPath, values)
	return err
}

// SetOEMAPIKeyEnabled 是七牛 OEM Bearer enabled 接口拷贝方法，第一阶段不替换既有 Key 禁用路径。
func (client *qiniuKeyClient) SetOEMAPIKeyEnabled(ctx context.Context, bearerToken string, keyBody string, enabled bool) error {
	bearerToken = strings.TrimSpace(bearerToken)
	if bearerToken == "" {
		return errors.New("OEM Key enabled 接口缺少 bearer token")
	}
	fullKey := fullQiniuAPIKey(keyBody)
	if _, err := normalizeQiniuAPIKey(fullKey); err != nil {
		return err
	}
	body := qiniuAPIKeyEnabledRequest{
		Key:     fullKey,
		Enabled: enabled,
	}
	payload, err := common.Marshal(body)
	if err != nil {
		return err
	}
	requestURL := strings.TrimRight(strings.TrimSpace(client.apiKeyEnabledBaseURL()), "/") + qiniuAPIKeyEnabledPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, requestURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("OEM Key enabled 接口返回异常状态 %d: %s", resp.StatusCode, readLimitedBody(resp.Body))
	}
	var decoded map[string]any
	if err := common.DecodeJson(resp.Body, &decoded); err != nil {
		return err
	}
	return qiniuBusinessStatusError(decoded)
}

func (client *qiniuKeyClient) doForm(ctx context.Context, method string, path string, values url.Values) (map[string]any, error) {
	payload := []byte(values.Encode())
	requestURL := client.childAccountBaseURL() + path
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", client.authorization(req, payload))
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("子账户接口返回异常状态 %d: %s", resp.StatusCode, readLimitedBody(resp.Body))
	}
	var decoded map[string]any
	if err := common.DecodeJson(resp.Body, &decoded); err != nil {
		return nil, err
	}
	if err := qiniuBusinessStatusError(decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func (client *qiniuKeyClient) doChildAccountJSON(ctx context.Context, method string, path string, body any) (map[string]any, error) {
	return client.doJSONWithBaseURL(ctx, client.childAccountBaseURL(), method, path, body)
}

func (client *qiniuKeyClient) childAccountBaseURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(client.setting.ChildAccountBaseURL), "/")
	if baseURL == "" {
		return operation_setting.QiniuChildAccountDefaultBaseURL
	}
	return baseURL
}

func parseQiniuChildAccountRemote(value any) (*qiniuChildAccountRemote, error) {
	root, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("创建子账户响应格式无效")
	}
	data := qiniuDataMap(root)
	userID := firstString(data, "userid", "user_id", "id")
	uid := firstString(data, "uid")
	email := firstString(data, "email")
	return &qiniuChildAccountRemote{
		UserID:     userID,
		UID:        uid,
		ParentUID:  firstString(data, "parent_uid", "parentUid"),
		Email:      email,
		IsDisabled: qiniuBool(data["is_disabled"]),
	}, nil
}

func parseQiniuChildAccountKeys(value any) (*qiniuChildAccountKeys, error) {
	root, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("查询子账户密钥响应格式无效")
	}
	data := qiniuDataMap(root)
	accessKey := firstString(data, "key", "ak", "access_key")
	secretKey := firstString(data, "secret", "sk", "secret_key")
	if accessKey == "" || secretKey == "" {
		return nil, errors.New("查询子账户密钥响应缺少 key 或 secret")
	}
	return &qiniuChildAccountKeys{
		AccessKey:       accessKey,
		SecretKey:       secretKey,
		State:           firstString(data, "state"),
		BackupAccessKey: firstString(data, "key2", "backup_key"),
		BackupSecretKey: firstString(data, "secret2", "backup_secret"),
		BackupState:     firstString(data, "state2", "backup_state"),
	}, nil
}

func qiniuDataMap(root map[string]any) map[string]any {
	if data, ok := root["data"].(map[string]any); ok {
		return data
	}
	return root
}

func qiniuBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return false
	}
}

func SanitizeQiniuChildAccountSecret(message string) string {
	safe := qiniuAuthorizationPattern.ReplaceAllString(message, "Qiniu ********")
	safe = qiniuSensitiveKeyPattern.ReplaceAllString(safe, "sk-********")
	safe = qiniuLongSecretPattern.ReplaceAllStringFunc(safe, func(value string) string {
		if len(value) <= 12 {
			return value
		}
		return value[:4] + "********" + value[len(value)-4:]
	})
	return strings.TrimSpace(safe)
}
