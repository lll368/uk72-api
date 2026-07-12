package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	qiniuCreateAPIKeysPath       = "/v1/apikeys"
	qiniuAPIKeyEnabledPath       = "/ai/inapi/v2/apikey/enabled"
	qiniuAPIKeyLimitPathTemplate = "/v1/apikey/quota/%s"
	qiniuAPIKeyUsagePath         = "/v2/stat/usage/apikey/cost-detail"
	qiniuOfficialUsagePath       = "/v2/stat/usage"
	qiniuMarketModelsPath        = "/v1/market/models"
	qiniuUsageMaxRangeDays       = 100
)

var qiniuFullKeyPattern = regexp.MustCompile(`^sk-[0-9a-fA-F]{64}$`)
var qiniuHTTPStatusErrorPattern = regexp.MustCompile(`状态\s+(\d{3})`)
var qiniuAPIKeyEnabledBaseURL = "https://api.qiniu.com"

var qiniuCSTLocation = time.FixedZone("UTC+8", 8*60*60)

type qiniuKeyClient struct {
	setting    operation_setting.QiniuKeySetting
	httpClient *http.Client
}

type qiniuQuotaLimitPatchRequest struct {
	DailyQuota   *qiniuQuotaLimitValue `json:"daily_quota,omitempty"`
	MonthlyQuota *qiniuQuotaLimitValue `json:"monthly_quota,omitempty"`
	TotalQuota   *qiniuQuotaLimitValue `json:"total_quota,omitempty"`
}

type qiniuQuotaLimitRequest struct {
	DailyQuota   qiniuQuotaLimitValue `json:"daily_quota"`
	MonthlyQuota qiniuQuotaLimitValue `json:"monthly_quota"`
	TotalQuota   qiniuQuotaLimitValue `json:"total_quota"`
}

type qiniuQuotaLimitValue struct {
	Enabled        bool    `json:"enabled"`
	Limit          float64 `json:"limit"`
	AlertThreshold int     `json:"alert_threshold"`
}

type qiniuAPIKeyEnabledRequest struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

type qiniuBusinessError struct {
	Code    string
	Message string
}

func (err *qiniuBusinessError) Error() string {
	if err == nil {
		return "接口返回失败: unknown"
	}
	message := strings.TrimSpace(err.Message)
	if message == "" {
		message = "unknown"
	}
	code := strings.TrimSpace(err.Code)
	if code == "" {
		return fmt.Sprintf("接口返回失败: %s", message)
	}
	return fmt.Sprintf("接口返回失败: code=%s message=%s", code, message)
}

type qiniuOfficialUsageQuery struct {
	Granularity string
	Start       time.Time
	End         time.Time
	APIKey      string
}

type qiniuOfficialCostDetailQuery struct {
	StartDate time.Time
	EndDate   time.Time
	Grain     string
	APIKey    string
}

type qiniuOfficialTokenUsageItem struct {
	APIKey           string
	ModelName        string
	ModelDisplayName string
	BillingItem      string
	CategoryName     string
	Unit             string
	Value            float64
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	PeriodStart      int64
	PeriodEnd        int64
	RawResponse      string
}

type qiniuOfficialCostDetailItem struct {
	APIKey      string
	ModelName   string
	BillingItem string
	BillingName string
	UsageCount  float64
	UsageUnit   string
	FeeAmount   float64
	Currency    string
	PeriodStart int64
	PeriodEnd   int64
	RawResponse string
}

func newQiniuKeyClient(setting *operation_setting.QiniuKeySetting) (*qiniuKeyClient, error) {
	if err := operation_setting.ValidateQiniuKeySettingForEnable(setting); err != nil {
		return nil, err
	}
	if setting == nil || (!setting.Enabled && !setting.OfficialLedgerEnabled && !setting.MarketCatalogEnabled) {
		return nil, errors.New("托管 Key、官方 ledger 或模型市场未启用")
	}
	requestTimeout := setting.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = operation_setting.QiniuKeyDefaultRequestTimeout
	}
	return &qiniuKeyClient{
		setting: *setting,
		httpClient: &http.Client{
			Timeout: time.Duration(requestTimeout) * time.Second,
		},
	}, nil
}

func normalizeQiniuAPIKey(fullKey string) (string, error) {
	fullKey = strings.TrimSpace(fullKey)
	if fullKey == "" {
		return "", errors.New("Key 为空")
	}
	if !strings.HasPrefix(fullKey, "sk-") {
		fullKey = "sk-" + fullKey
	}
	if !qiniuFullKeyPattern.MatchString(fullKey) {
		return "", fmt.Errorf("Key 格式无效")
	}
	return strings.TrimPrefix(fullKey, "sk-"), nil
}

func fullQiniuAPIKey(keyBody string) string {
	keyBody = strings.TrimSpace(keyBody)
	if keyBody == "" || strings.HasPrefix(keyBody, "sk-") {
		return keyBody
	}
	return "sk-" + keyBody
}

func isQiniuAPIKeyBody(keyBody string) bool {
	_, err := normalizeQiniuAPIKey(keyBody)
	return err == nil
}

func maskQiniuAPIKey(key string) string {
	key = fullQiniuAPIKey(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:6] + "**********" + key[len(key)-4:]
}

func (client *qiniuKeyClient) CreateAPIKey(ctx context.Context, name string) (string, error) {
	body := map[string]any{
		"count": 1,
		"names": []string{strings.TrimSpace(name)},
	}
	respBody, err := client.doJSON(ctx, http.MethodPost, qiniuCreateAPIKeysPath, body)
	if err != nil {
		return "", err
	}
	fullKey := extractQiniuFullKey(respBody)
	if fullKey == "" {
		return "", errors.New("创建 Key 响应缺少 key")
	}
	return normalizeQiniuAPIKey(fullKey)
}

func (client *qiniuKeyClient) SetAPIKeyTotalQuota(ctx context.Context, keyBody string, limit float64) error {
	if limit < 0 || math.IsNaN(limit) || math.IsInf(limit, 0) {
		return errors.New("Key 总额度不能为负数或无效数字")
	}
	fullKey := fullQiniuAPIKey(keyBody)
	if _, err := normalizeQiniuAPIKey(fullKey); err != nil {
		return err
	}
	path := fmt.Sprintf(qiniuAPIKeyLimitPathTemplate, url.PathEscape(fullKey))
	body := qiniuQuotaLimitPatchRequest{
		TotalQuota: &qiniuQuotaLimitValue{
			Enabled:        true,
			Limit:          limit,
			AlertThreshold: 80,
		},
	}
	_, err := client.doJSON(ctx, http.MethodPut, path, body)
	return err
}

func (client *qiniuKeyClient) SetAPIKeyDailyQuotaZero(ctx context.Context, keyBody string) error {
	fullKey := fullQiniuAPIKey(keyBody)
	if _, err := normalizeQiniuAPIKey(fullKey); err != nil {
		return err
	}
	path := fmt.Sprintf(qiniuAPIKeyLimitPathTemplate, url.PathEscape(fullKey))
	body := qiniuQuotaLimitPatchRequest{
		DailyQuota: &qiniuQuotaLimitValue{
			Enabled:        true,
			Limit:          0,
			AlertThreshold: 0,
		},
	}
	_, err := client.doJSON(ctx, http.MethodPut, path, body)
	return err
}

// SetAPIKeyEnabled 调用七牛 Key enabled 接口切换远端 Key 状态。
func (client *qiniuKeyClient) SetAPIKeyEnabled(ctx context.Context, keyBody string, enabled bool) error {
	fullKey := fullQiniuAPIKey(keyBody)
	if _, err := normalizeQiniuAPIKey(fullKey); err != nil {
		return err
	}
	body := qiniuAPIKeyEnabledRequest{
		Key:     fullKey,
		Enabled: enabled,
	}
	_, err := client.doJSONWithBaseURL(ctx, client.apiKeyEnabledBaseURL(), http.MethodPut, qiniuAPIKeyEnabledPath, body)
	if err != nil && !enabled && isQiniuAPIKeyAlreadyDisabledError(err) {
		return nil
	}
	return err
}

func (client *qiniuKeyClient) apiKeyEnabledBaseURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(client.setting.BaseURL), "/")
	if baseURL == "" || baseURL == operation_setting.QiniuKeyDefaultBaseURL {
		return qiniuAPIKeyEnabledBaseURL
	}
	return baseURL
}

func (client *qiniuKeyClient) GetAPIKeyUsedAmount(ctx context.Context, keyBody string, createdTime int64) (float64, error) {
	fullKey := fullQiniuAPIKey(keyBody)
	if _, err := normalizeQiniuAPIKey(fullKey); err != nil {
		return 0, err
	}
	total := 0.0
	for _, queryPath := range buildQiniuUsageQueryPaths(createdTime, time.Now()) {
		respBody, err := client.doBearerJSON(ctx, queryPath, fullKey)
		if err != nil {
			return 0, err
		}
		usedAmount, ok := extractQiniuUsedAmount(respBody)
		if !ok {
			return 0, errors.New("用量统计响应缺少已用金额")
		}
		total += usedAmount
	}
	return total, nil
}

func (client *qiniuKeyClient) QueryOfficialTokenUsage(ctx context.Context, query qiniuOfficialUsageQuery) ([]qiniuOfficialTokenUsageItem, error) {
	if strings.TrimSpace(query.Granularity) == "" {
		return nil, errors.New("官方用量查询缺少 granularity")
	}
	if query.Start.IsZero() || query.End.IsZero() || !query.End.After(query.Start) {
		return nil, errors.New("官方用量查询时间范围无效")
	}
	values := url.Values{}
	values.Set("granularity", strings.TrimSpace(query.Granularity))
	values.Set("start", query.Start.In(qiniuCSTLocation).Format(time.RFC3339))
	values.Set("end", query.End.In(qiniuCSTLocation).Format(time.RFC3339))
	if strings.TrimSpace(query.APIKey) != "" {
		values.Set("api_key", fullQiniuAPIKey(query.APIKey))
	}
	respBody, err := client.doJSON(ctx, http.MethodGet, qiniuOfficialUsagePath+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	items, err := parseQiniuOfficialTokenUsage(respBody, query.Granularity)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(query.APIKey) != "" {
		for i := range items {
			items[i].APIKey = fullQiniuAPIKey(query.APIKey)
		}
	}
	return items, nil
}

func (client *qiniuKeyClient) QueryOfficialCostDetails(ctx context.Context, query qiniuOfficialCostDetailQuery) ([]qiniuOfficialCostDetailItem, error) {
	if strings.TrimSpace(query.Grain) == "" {
		query.Grain = "day"
	}
	if query.StartDate.IsZero() || query.EndDate.IsZero() || query.EndDate.Before(query.StartDate) {
		return nil, errors.New("账单明细查询时间范围无效")
	}
	values := url.Values{}
	values.Set("start_date", query.StartDate.In(qiniuCSTLocation).Format("2006-01-02"))
	values.Set("end_date", query.EndDate.In(qiniuCSTLocation).Format("2006-01-02"))
	values.Set("grain", strings.TrimSpace(query.Grain))
	var respBody map[string]any
	var err error
	if strings.TrimSpace(query.APIKey) != "" {
		respBody, err = client.doBearerJSON(ctx, qiniuAPIKeyUsagePath+"?"+values.Encode(), fullQiniuAPIKey(query.APIKey))
	} else {
		respBody, err = client.doJSON(ctx, http.MethodGet, qiniuAPIKeyUsagePath+"?"+values.Encode(), nil)
	}
	if err != nil {
		return nil, err
	}
	items, err := parseQiniuOfficialCostDetails(respBody, query.Grain)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(query.APIKey) != "" {
		for i := range items {
			items[i].APIKey = fullQiniuAPIKey(query.APIKey)
		}
	}
	return items, nil
}

func (client *qiniuKeyClient) QueryMarketModels(ctx context.Context) ([]dto.QiniuMarketModel, error) {
	values := url.Values{}
	values.Set("overseas", strconv.FormatBool(client.setting.MarketCatalogOverseas))
	respBody, err := client.doPublicMarketJSON(ctx, qiniuMarketModelsPath+"?"+values.Encode())
	if err != nil {
		return nil, err
	}
	return parseQiniuMarketModels(respBody)
}

func (client *qiniuKeyClient) doJSON(ctx context.Context, method string, path string, body any) (map[string]any, error) {
	return client.doJSONWithBaseURL(ctx, client.setting.BaseURL, method, path, body)
}

func (client *qiniuKeyClient) doJSONWithBaseURL(ctx context.Context, baseURL string, method string, path string, body any) (map[string]any, error) {
	requestURL := strings.TrimRight(strings.TrimSpace(baseURL), "/") + path
	var payload []byte
	var err error
	if body != nil {
		payload, err = common.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", client.authorization(req, payload))

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Key 接口返回异常状态 %d: %s", resp.StatusCode, readLimitedBody(resp.Body))
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

func (client *qiniuKeyClient) doPublicMarketJSON(ctx context.Context, path string) (map[string]any, error) {
	requestURL := strings.TrimRight(client.setting.MarketCatalogBaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("模型市场接口返回异常状态 %d: %s", resp.StatusCode, readLimitedBody(resp.Body))
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

func (client *qiniuKeyClient) doBearerJSON(ctx context.Context, path string, bearerToken string) (map[string]any, error) {
	requestURL := strings.TrimRight(client.setting.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Key 用量接口返回异常状态 %d: %s", resp.StatusCode, readLimitedBody(resp.Body))
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

func (client *qiniuKeyClient) authorization(req *http.Request, body []byte) string {
	signingText := req.Method + " " + req.URL.RequestURI() + "\n" +
		"Host: " + req.URL.Host
	contentType := req.Header.Get("Content-Type")
	if contentType != "" {
		signingText += "\nContent-Type: " + contentType
	}
	signingText += "\n\n"
	if len(body) > 0 && contentType != "application/octet-stream" {
		signingText += string(body)
	}
	mac := hmac.New(sha1.New, []byte(client.setting.SecretKey))
	mac.Write([]byte(signingText))
	sign := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return "Qiniu " + client.setting.AccessKey + ":" + sign
}

func extractQiniuFullKey(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, preferredKey := range []string{"key", "api_key", "apiKey", "secret_key", "secretKey"} {
			if nested, ok := typed[preferredKey]; ok {
				if key := extractQiniuFullKey(nested); key != "" {
					return key
				}
			}
		}
		for _, nested := range typed {
			if key := extractQiniuFullKey(nested); key != "" {
				return key
			}
		}
	case []any:
		for _, nested := range typed {
			if key := extractQiniuFullKey(nested); key != "" {
				return key
			}
		}
	case string:
		candidate := strings.TrimSpace(typed)
		if qiniuFullKeyPattern.MatchString(candidate) {
			return candidate
		}
	}
	return ""
}

func qiniuBusinessStatusError(decoded map[string]any) error {
	if decoded == nil {
		return nil
	}
	status, ok := decoded["status"].(bool)
	if !ok || status {
		return nil
	}
	return &qiniuBusinessError{
		Code:    qiniuBusinessErrorCode(decoded),
		Message: qiniuBusinessErrorMessage(decoded),
	}
}

func qiniuBusinessErrorMessage(decoded map[string]any) string {
	for _, key := range []string{"error", "message", "msg"} {
		if value, ok := decoded[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "unknown"
}

func qiniuBusinessErrorCode(decoded map[string]any) string {
	if decoded == nil {
		return ""
	}
	code, ok := decoded["code"]
	if !ok || code == nil {
		return ""
	}
	switch typed := code.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == math.Trunc(typed) {
			return strconv.Itoa(int(typed))
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func isQiniuTotalQuotaBelowUsedError(err error) bool {
	if err == nil {
		return false
	}
	var businessErr *qiniuBusinessError
	if errors.As(err, &businessErr) && qiniuFallbackBlockedByStatusCode(businessErr.Code) {
		return false
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return false
	}
	if matches := qiniuHTTPStatusErrorPattern.FindStringSubmatch(message); len(matches) == 2 {
		statusCode, parseErr := strconv.Atoi(matches[1])
		if parseErr == nil && qiniuFallbackBlockedByHTTPStatus(statusCode) {
			return false
		}
	}
	lower := strings.ToLower(message)
	containsEnglish := strings.Contains(lower, "total quota") &&
		(strings.Contains(lower, "less than used") ||
			strings.Contains(lower, "lower than used") ||
			strings.Contains(lower, "below used"))
	containsChinese := (strings.Contains(message, "总额度") || strings.Contains(message, "累计总额度")) &&
		(strings.Contains(message, "低于已用") || strings.Contains(message, "小于已用"))
	return containsEnglish || containsChinese
}

func qiniuFallbackBlockedByStatusCode(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	statusCode, err := strconv.Atoi(code)
	if err != nil {
		return false
	}
	return qiniuFallbackBlockedByHTTPStatus(statusCode)
}

func qiniuFallbackBlockedByHTTPStatus(statusCode int) bool {
	return statusCode >= http.StatusInternalServerError ||
		statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusForbidden ||
		statusCode == http.StatusTooManyRequests
}

func isQiniuAPIKeyAlreadyDisabledError(err error) bool {
	if err == nil {
		return false
	}
	var businessErr *qiniuBusinessError
	if errors.As(err, &businessErr) {
		code := strings.ToLower(strings.TrimSpace(businessErr.Code))
		if code == "already_disabled" ||
			code == "api_key_already_disabled" ||
			code == "apikey_already_disabled" {
			return true
		}
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return false
	}
	lower := strings.ToLower(message)
	return strings.Contains(lower, "already disabled") ||
		strings.Contains(lower, "already been disabled") ||
		strings.Contains(message, "已禁用") ||
		strings.Contains(message, "已经禁用")
}

func buildQiniuUsageQueryPaths(createdTime int64, now time.Time) []string {
	location := time.FixedZone("UTC+8", 8*60*60)
	endDate := now.In(location)
	startDate := endDate
	if createdTime > 0 {
		startDate = time.Unix(createdTime, 0).In(location)
	}
	if createdTime <= 0 || startDate.After(endDate) {
		startDate = endDate
	}
	startDate = dateOnly(startDate)
	endDate = dateOnly(endDate)

	paths := make([]string, 0)
	for current := startDate; !current.After(endDate); {
		chunkEnd := current.AddDate(0, 0, qiniuUsageMaxRangeDays-1)
		if chunkEnd.After(endDate) {
			chunkEnd = endDate
		}
		query := url.Values{}
		query.Set("start_date", current.Format("2006-01-02"))
		query.Set("end_date", chunkEnd.Format("2006-01-02"))
		query.Set("grain", "month")
		paths = append(paths, qiniuAPIKeyUsagePath+"?"+query.Encode())
		current = chunkEnd.AddDate(0, 0, 1)
	}
	return paths
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func extractQiniuUsedAmount(value any) (float64, bool) {
	root, ok := value.(map[string]any)
	if !ok {
		return 0, false
	}
	data, ok := root["data"].(map[string]any)
	if !ok {
		return 0, false
	}
	return interfaceToFloat64(data["total_fee"])
}

func parseQiniuOfficialTokenUsage(value any, granularity string) ([]qiniuOfficialTokenUsageItem, error) {
	root, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("官方用量响应格式无效")
	}
	data, exists := root["data"]
	if !exists {
		return nil, errors.New("官方用量响应缺少 data")
	}
	items := make([]qiniuOfficialTokenUsageItem, 0)
	models, ok := data.([]any)
	if !ok {
		if emptyData, ok := data.(map[string]any); ok && len(emptyData) == 0 {
			return items, nil
		}
		return nil, errors.New("官方用量响应缺少 data")
	}
	for _, modelValue := range models {
		modelMap, ok := modelValue.(map[string]any)
		if !ok {
			continue
		}
		modelName := firstString(modelMap, "id", "model_id", "model")
		modelDisplayName := firstString(modelMap, "name", "model_name")
		for _, itemValue := range arrayFromMap(modelMap, "items") {
			itemMap, ok := itemValue.(map[string]any)
			if !ok {
				continue
			}
			billingItem := firstString(itemMap, "key", "name")
			unit := firstString(itemMap, "unit")
			for _, categoryValue := range arrayFromMap(itemMap, "categories") {
				categoryMap, ok := categoryValue.(map[string]any)
				if !ok {
					continue
				}
				categoryName := firstString(categoryMap, "name")
				for _, pointValue := range arrayFromMap(categoryMap, "values") {
					pointMap, ok := pointValue.(map[string]any)
					if !ok {
						continue
					}
					pointTime, err := parseQiniuTime(firstString(pointMap, "time", "date"))
					if err != nil {
						return nil, err
					}
					value, ok := interfaceToFloat64(pointMap["value"])
					if !ok {
						continue
					}
					tokenCount := qiniuUsageValueToTokens(value, unit)
					rawResponse, err := marshalQiniuRaw(pointMap)
					if err != nil {
						return nil, err
					}
					row := qiniuOfficialTokenUsageItem{
						ModelName:        modelName,
						ModelDisplayName: modelDisplayName,
						BillingItem:      billingItem,
						CategoryName:     categoryName,
						Unit:             unit,
						Value:            value,
						TotalTokens:      tokenCount,
						PeriodStart:      pointTime.Unix(),
						PeriodEnd:        qiniuPeriodEnd(pointTime, granularity).Unix(),
						RawResponse:      rawResponse,
					}
					fillQiniuTokenDirection(&row)
					items = append(items, row)
				}
			}
		}
	}
	return items, nil
}

func parseQiniuOfficialCostDetails(value any, grain string) ([]qiniuOfficialCostDetailItem, error) {
	root, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("账单明细响应格式无效")
	}
	data, ok := root["data"].(map[string]any)
	if !ok {
		return nil, errors.New("账单明细响应缺少 data")
	}
	if apiKeys, ok := data["api_keys"].([]any); ok {
		items := make([]qiniuOfficialCostDetailItem, 0)
		for _, apiKeyValue := range apiKeys {
			apiKeyItems, err := parseQiniuCostDetailAPIKeyBlock(apiKeyValue, grain, firstString(data, "currency"))
			if err != nil {
				return nil, err
			}
			items = append(items, apiKeyItems...)
		}
		return items, nil
	}
	return parseQiniuCostDetailAPIKeyBlock(data, grain, firstString(data, "currency"))
}

func parseQiniuMarketModels(value any) ([]dto.QiniuMarketModel, error) {
	root, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("模型市场响应格式无效")
	}
	data, ok := root["data"].([]any)
	if !ok {
		return nil, errors.New("模型市场响应缺少 data")
	}
	payload, err := common.Marshal(data)
	if err != nil {
		return nil, err
	}
	var models []dto.QiniuMarketModel
	if err := common.Unmarshal(payload, &models); err != nil {
		return nil, err
	}
	filtered := make([]dto.QiniuMarketModel, 0, len(models))
	for _, item := range models {
		item.ID = strings.TrimSpace(item.ID)
		item.Name = strings.TrimSpace(item.Name)
		if item.ID == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}

func parseQiniuCostDetailAPIKeyBlock(value any, grain string, defaultCurrency string) ([]qiniuOfficialCostDetailItem, error) {
	apiKeyBlock, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("账单明细 api_key 节点格式无效")
	}
	apiKey := firstString(apiKeyBlock, "api_key", "apiKey")
	currency := firstString(apiKeyBlock, "currency")
	if currency == "" {
		currency = defaultCurrency
	}
	if currency == "" {
		currency = "CNY"
	}
	items := make([]qiniuOfficialCostDetailItem, 0)
	for _, billValue := range arrayFromMap(apiKeyBlock, "bills") {
		billMap, ok := billValue.(map[string]any)
		if !ok {
			continue
		}
		periodStart, err := parseQiniuCostDate(firstString(billMap, "date"), grain)
		if err != nil {
			return nil, err
		}
		for _, modelValue := range arrayFromMap(billMap, "models") {
			modelMap, ok := modelValue.(map[string]any)
			if !ok {
				continue
			}
			modelName := firstString(modelMap, "model_id", "id", "model")
			for _, itemValue := range arrayFromMap(modelMap, "items") {
				itemMap, ok := itemValue.(map[string]any)
				if !ok {
					continue
				}
				fee, ok := interfaceToFloat64(itemMap["fee"])
				if !ok {
					return nil, fmt.Errorf("账单明细缺少 fee: key=%s model=%s item=%s", maskQiniuAPIKey(apiKey), modelName, firstString(itemMap, "key", "name"))
				}
				usageMap, _ := itemMap["usage"].(map[string]any)
				usageCount, _ := interfaceToFloat64(usageMap["count"])
				rawResponse, err := marshalQiniuRaw(itemMap)
				if err != nil {
					return nil, err
				}
				rowCurrency := firstString(itemMap, "currency")
				if rowCurrency == "" {
					rowCurrency = currency
				}
				items = append(items, qiniuOfficialCostDetailItem{
					APIKey:      apiKey,
					ModelName:   modelName,
					BillingItem: firstString(itemMap, "key", "name"),
					BillingName: firstString(itemMap, "name"),
					UsageCount:  usageCount,
					UsageUnit:   firstString(usageMap, "unit"),
					FeeAmount:   fee,
					Currency:    rowCurrency,
					PeriodStart: periodStart.Unix(),
					PeriodEnd:   qiniuCostPeriodEnd(periodStart, grain).Unix(),
					RawResponse: rawResponse,
				})
			}
		}
	}
	return items, nil
}

func fillQiniuTokenDirection(item *qiniuOfficialTokenUsageItem) {
	if item == nil {
		return
	}
	label := strings.ToLower(item.BillingItem + " " + item.CategoryName)
	if strings.Contains(label, "input") || strings.Contains(label, "prompt") || strings.Contains(label, "输入") {
		item.PromptTokens = item.TotalTokens
		return
	}
	if strings.Contains(label, "output") || strings.Contains(label, "completion") || strings.Contains(label, "输出") {
		item.CompletionTokens = item.TotalTokens
	}
}

func qiniuUsageValueToTokens(value float64, unit string) int64 {
	unit = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(unit), " ", ""))
	multiplier := 1.0
	switch unit {
	case "ktoken", "ktokens", "k/token", "k/tokens":
		multiplier = 1000
	case "mtoken", "mtokens", "m/token", "m/tokens", "百万token", "百万tokens":
		multiplier = 1000000
	}
	return int64(math.Round(value * multiplier))
}

func qiniuPeriodEnd(start time.Time, granularity string) time.Time {
	switch strings.ToLower(strings.TrimSpace(granularity)) {
	case "hour":
		return start.Add(time.Hour)
	case "day":
		return start.AddDate(0, 0, 1)
	default:
		return start
	}
}

func qiniuCostPeriodEnd(start time.Time, grain string) time.Time {
	switch strings.ToLower(strings.TrimSpace(grain)) {
	case "month":
		return start.AddDate(0, 1, 0)
	default:
		return start.AddDate(0, 0, 1)
	}
}

func parseQiniuTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("官方用量响应缺少时间点")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, qiniuCSTLocation); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("官方用量时间格式无效: %s", value)
}

func parseQiniuCostDate(value string, grain string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("账单明细缺少 date")
	}
	if strings.EqualFold(strings.TrimSpace(grain), "month") {
		if parsed, err := time.ParseInLocation("2006-01", value, qiniuCSTLocation); err == nil {
			return parsed, nil
		}
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, qiniuCSTLocation); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("账单明细日期格式无效: %s", value)
}

func firstString(values map[string]any, keys ...string) string {
	if values == nil {
		return ""
	}
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if str, ok := value.(string); ok && strings.TrimSpace(str) != "" {
				return strings.TrimSpace(str)
			}
		}
	}
	return ""
}

func arrayFromMap(values map[string]any, key string) []any {
	if values == nil {
		return nil
	}
	array, _ := values[key].([]any)
	return array
}

func marshalQiniuRaw(value any) (string, error) {
	data, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func interfaceToFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		if typed = strings.TrimSpace(typed); typed != "" {
			amount, err := strconv.ParseFloat(typed, 64)
			return amount, err == nil
		}
	}
	return 0, false
}

func readLimitedBody(reader io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(reader, 2048))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
