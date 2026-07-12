package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	piggyCharset = "utf-8"
	piggyVersion = "3.0"

	piggyEndpointSingleOrderSubmit     = "/open/payment/singleOrderSubmit"
	piggyEndpointSingleOrderConfirmPay = "/open/payment/singleOrderConfirmPay"
	piggyEndpointSingleOrderCancel     = "/open/payment/singleOrderCancel"
	piggyEndpointSingleOrderQuery      = "/open/payment/singleOrderQuery"
	piggyEndpointSingleTaxTrialCalc    = "/open/payment/singleTaxTrialCalc"
	piggyEndpointContractScope         = "/contract/sign/getContractScope"
	piggyEndpointContractSignURL       = "/contract/sign/hasKeyByUrl"
	piggyEndpointContractResult        = "/contract/sign/getSignedResult"
	piggyEndpointContractPreview       = "/contract/sign/viewContract"
)

var piggyContractSignURLSignedFields = []string{
	"appKey",
	"userName",
	"idCardNo",
	"mobile",
	"position",
	"notifyUrl",
	"jumpPage",
	"customParams",
}

type piggyHTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type PiggyClient struct {
	setting *operation_setting.PiggyWithdrawSetting
	http    piggyHTTPDoer
}

var newConfiguredPiggyClient = NewPiggyClient
var newPiggyPreviewClient = NewPiggyPreviewClient

type PiggyAPIResponse struct {
	Code         string          `json:"code"`
	Msg          string          `json:"msg"`
	IsSuccess    *bool           `json:"isSuccess"`
	ErrorCode    string          `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage"`
	Data         map[string]any  `json:"data"`
	DataString   string          `json:"-"`
	RawData      json.RawMessage `json:"-"`
	RawBody      string          `json:"-"`
}

// UnmarshalJSON 兼容小猪接口同一字段在不同接口中返回字符串、数字或对象的情况。
func (r *PiggyAPIResponse) UnmarshalJSON(data []byte) error {
	var payload struct {
		Code         json.RawMessage `json:"code"`
		Msg          json.RawMessage `json:"msg"`
		IsSuccess    json.RawMessage `json:"isSuccess"`
		ErrorCode    json.RawMessage `json:"errorCode"`
		ErrorMessage json.RawMessage `json:"errorMessage"`
		Data         json.RawMessage `json:"data"`
	}
	if err := common.Unmarshal(data, &payload); err != nil {
		return err
	}
	r.Code = common.JsonRawMessageToString(payload.Code)
	r.Msg = common.JsonRawMessageToString(payload.Msg)
	isSuccess, err := parsePiggyBool(payload.IsSuccess)
	if err != nil {
		return err
	}
	r.IsSuccess = isSuccess
	r.ErrorCode = common.JsonRawMessageToString(payload.ErrorCode)
	r.ErrorMessage = common.JsonRawMessageToString(payload.ErrorMessage)
	r.Data = nil
	r.DataString = ""
	r.RawData = nil
	if len(payload.Data) > 0 && common.GetJsonType(payload.Data) != "null" {
		r.RawData = append(r.RawData[:0], payload.Data...)
		if common.GetJsonType(payload.Data) == "object" {
			if err := common.Unmarshal(payload.Data, &r.Data); err != nil {
				return err
			}
		} else {
			r.DataString = strings.TrimSpace(common.JsonRawMessageToString(payload.Data))
		}
	}
	return nil
}

func parsePiggyBool(rawValue json.RawMessage) (*bool, error) {
	trimmed := bytes.TrimSpace(rawValue)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	switch trimmed[0] {
	case 't', 'f', 'T', 'F':
		var boolValue bool
		if err := common.Unmarshal(trimmed, &boolValue); err != nil {
			return nil, err
		}
		return &boolValue, nil
	case '"':
		var stringValue string
		if err := common.Unmarshal(trimmed, &stringValue); err != nil {
			return nil, err
		}
		switch strings.ToLower(strings.TrimSpace(stringValue)) {
		case "1", "true", "t", "yes", "y", "on":
			value := true
			return &value, nil
		case "", "0", "false", "f", "no", "off", "n":
			value := false
			return &value, nil
		default:
			return nil, errors.New("PiggyAPIResponse isSuccess 字段值无法解析")
		}
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-':
		var floatValue float64
		if err := common.Unmarshal(trimmed, &floatValue); err != nil {
			return nil, err
		}
		value := floatValue > 0
		return &value, nil
	default:
		return nil, errors.New("PiggyAPIResponse isSuccess 字段值无法解析")
	}
}

type PiggySubmitOrderRequest struct {
	NotifyUrl    string `json:"notifyUrl"`
	TaxFundId    string `json:"taxFundId"`
	OuterTradeNo string `json:"outerTradeNo"`
	EmpName      string `json:"empName"`
	EmpPhone     string `json:"empPhone"`
	LicenseType  string `json:"licenseType"`
	LicenseId    string `json:"licenseId"`
	SettleType   string `json:"settleType"`
	PayAccount   string `json:"payAccount"`
	BankName     string `json:"bankName,omitempty"`
	PositionName string `json:"positionName"`
	PayAmount    string `json:"payAmount"`
	BankRemo     string `json:"bankRemo,omitempty"`
	CalcType     string `json:"calcType"`
}

type PiggyTaxTrialCalcRequest struct {
	OuterTradeNo string `json:"outerTradeNo"`
	TaxFundId    string `json:"taxFundId"`
	LicenseId    string `json:"licenseId"`
	CalcAmount   string `json:"calcAmount"`
	CalcType     string `json:"calcType"`
	CalcMonth    string `json:"calcMonth,omitempty"`
	AddTaxRate   string `json:"addTaxRate,omitempty"`
}

type PiggyTaxTrialCalcResult struct {
	OuterTradeNo              string            `json:"outer_trade_no"`
	CalcMonth                 string            `json:"calc_month"`
	RequestedAmount           string            `json:"requested_amount"`
	RequestedAmountCents      int64             `json:"requested_amount_cents"`
	PlatformFeeRate           float64           `json:"platform_fee_rate"`
	PlatformFeeAmount         string            `json:"platform_fee_amount"`
	PlatformFeeAmountCents    int64             `json:"platform_fee_amount_cents"`
	PiggyTaxBeforeAmount      string            `json:"piggy_tax_before_amount"`
	PiggyTaxBeforeAmountCents int64             `json:"piggy_tax_before_amount_cents"`
	PretaxAmount              string            `json:"pretax_amount"`
	IndividualTaxAmount       string            `json:"individual_tax_amount"`
	AddedTaxAmount            string            `json:"added_tax_amount"`
	AfterTaxAmount            string            `json:"after_tax_amount"`
	CalcType                  string            `json:"calc_type"`
	Raw                       *PiggyAPIResponse `json:"-"`
}

type PiggyContractSignURLRequest struct {
	UserName         string `json:"userName"`
	IdCardNo         string `json:"idCardNo"`
	Mobile           string `json:"mobile"`
	BankAccount      string `json:"bankAccount"`
	Position         string `json:"position"`
	NotifyUrl        string `json:"notifyUrl"`
	JumpPage         string `json:"jumpPage"`
	CustomParams     string `json:"customParams"`
	InfoSource       string `json:"infoSource,omitempty"`
	InfoSourcePrefix string `json:"infoSourcePrefix,omitempty"`
}

type PiggyContractStatusRequest struct {
	UserName string `json:"userName"`
	IdCardNo string `json:"idCardNo"`
	Position string `json:"position"`
}

type PiggyContractSignURLResult struct {
	SignURL string
	Raw     *PiggyAPIResponse
}

func NewPiggyClient(setting *operation_setting.PiggyWithdrawSetting) (*PiggyClient, error) {
	if setting == nil {
		setting = operation_setting.GetPiggyWithdrawSetting()
	}
	if err := operation_setting.ValidatePiggyWithdrawSettingForEnable(setting); err != nil {
		return nil, err
	}
	timeout := time.Duration(setting.RequestTimeout) * time.Second
	return &PiggyClient{
		setting: setting,
		http:    &http.Client{Timeout: timeout},
	}, nil
}

func NewPiggyPreviewClient(setting *operation_setting.PiggyWithdrawSetting) (*PiggyClient, error) {
	if setting == nil {
		setting = operation_setting.GetPiggyWithdrawSetting()
	}
	domain := strings.TrimRight(strings.TrimSpace(setting.Domain), "/")
	if domain == "" {
		return nil, errors.New("小猪合同预览域名未配置")
	}
	timeout := setting.RequestTimeout
	if timeout <= 0 {
		timeout = operation_setting.PiggyWithdrawDefaultRequestTimeout
	}
	copySetting := *setting
	copySetting.Domain = domain
	copySetting.RequestTimeout = timeout
	return &PiggyClient{
		setting: &copySetting,
		http:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func newPiggyClientWithHTTP(setting *operation_setting.PiggyWithdrawSetting, httpClient piggyHTTPDoer) *PiggyClient {
	if setting == nil {
		setting = operation_setting.GetPiggyWithdrawSetting()
	}
	_ = operation_setting.ValidatePiggyWithdrawSettingForEnable(setting)
	return &PiggyClient{setting: setting, http: httpClient}
}

func (c *PiggyClient) SingleOrderSubmit(ctx context.Context, biz PiggySubmitOrderRequest) (*PiggyAPIResponse, string, error) {
	if c == nil {
		return nil, "", errors.New("小猪 client 未初始化")
	}
	bizContent, err := piggyCompactJSON(biz)
	if err != nil {
		return nil, "", err
	}
	encrypted, err := piggyEncryptAES([]byte(bizContent), c.setting.AppSecret, c.setting.AESIV)
	if err != nil {
		return nil, "", err
	}
	payload := map[string]any{
		"appKey":        c.setting.AppKey,
		"charset":       piggyCharset,
		"version":       piggyVersion,
		"bizAESContent": encrypted,
	}
	sign, err := piggySignJSON(c.setting.AppSecret, payload)
	if err != nil {
		return nil, "", err
	}
	payload["sign"] = sign
	response, err := c.postJSON(ctx, piggyEndpointSingleOrderSubmit, payload)
	return response, digestAny(payload), err
}

func (c *PiggyClient) SingleOrderConfirmPay(ctx context.Context, outerTradeNo string) (*PiggyAPIResponse, string, error) {
	return c.postOuterTradeNo(ctx, piggyEndpointSingleOrderConfirmPay, outerTradeNo)
}

func (c *PiggyClient) SingleOrderCancel(ctx context.Context, outerTradeNo string) (*PiggyAPIResponse, string, error) {
	return c.postOuterTradeNo(ctx, piggyEndpointSingleOrderCancel, outerTradeNo)
}

func (c *PiggyClient) SingleOrderQuery(ctx context.Context, outerTradeNo string) (*PiggyAPIResponse, string, error) {
	return c.postOuterTradeNo(ctx, piggyEndpointSingleOrderQuery, outerTradeNo)
}

func (c *PiggyClient) SingleTaxTrialCalc(ctx context.Context, req PiggyTaxTrialCalcRequest) (*PiggyTaxTrialCalcResult, string, error) {
	if c == nil {
		return nil, "", errors.New("小猪 client 未初始化")
	}
	payload := map[string]any{
		"appKey":       c.setting.AppKey,
		"charset":      piggyCharset,
		"version":      piggyVersion,
		"outerTradeNo": strings.TrimSpace(req.OuterTradeNo),
		"taxFundId":    strings.TrimSpace(req.TaxFundId),
		"licenseId":    strings.TrimSpace(req.LicenseId),
		"calcAmount":   strings.TrimSpace(req.CalcAmount),
		"calcType":     strings.ToUpper(strings.TrimSpace(req.CalcType)),
	}
	if strings.TrimSpace(req.CalcMonth) != "" {
		payload["calcMonth"] = strings.TrimSpace(req.CalcMonth)
	}
	if strings.TrimSpace(req.AddTaxRate) != "" {
		payload["addTaxRate"] = strings.TrimSpace(req.AddTaxRate)
	}
	sign, err := piggySignJSON(c.setting.AppSecret, payload)
	if err != nil {
		return nil, "", err
	}
	payload["sign"] = sign
	response, err := c.postJSON(ctx, piggyEndpointSingleTaxTrialCalc, payload)
	result := &PiggyTaxTrialCalcResult{
		OuterTradeNo: strings.TrimSpace(req.OuterTradeNo),
		CalcType:     strings.ToUpper(strings.TrimSpace(req.CalcType)),
		Raw:          response,
	}
	if response != nil {
		result.OuterTradeNo = firstNonEmpty(pickPiggyString(response.Data, "outerTradeNo", "outer_trade_no"), result.OuterTradeNo)
		result.CalcMonth = pickPiggyString(response.Data, "calcMonth", "calc_month")
		result.PretaxAmount = normalizePiggyMoneyText(pickPiggyString(response.Data, "pretaxAmount", "pretax_amount"))
		result.IndividualTaxAmount = normalizePiggyMoneyText(pickPiggyString(response.Data, "individualTaxAmount", "individual_tax_amount"))
		result.AddedTaxAmount = normalizePiggyMoneyText(pickPiggyString(response.Data, "addedTaxAmount", "added_tax_amount"))
		result.AfterTaxAmount = normalizePiggyMoneyText(pickPiggyString(response.Data, "afterTaxAmount", "after_tax_amount"))
	}
	return result, digestAny(payload), err
}

func (c *PiggyClient) postOuterTradeNo(ctx context.Context, endpoint string, outerTradeNo string) (*PiggyAPIResponse, string, error) {
	payload := map[string]any{
		"appKey":       c.setting.AppKey,
		"charset":      piggyCharset,
		"version":      piggyVersion,
		"outerTradeNo": strings.TrimSpace(outerTradeNo),
	}
	sign, err := piggySignJSON(c.setting.AppSecret, payload)
	if err != nil {
		return nil, "", err
	}
	payload["sign"] = sign
	response, err := c.postJSON(ctx, endpoint, payload)
	return response, digestAny(payload), err
}

func (c *PiggyClient) GetContractStatus(ctx context.Context, req PiggyContractStatusRequest) (*PiggyAPIResponse, string, error) {
	form := map[string]string{
		"appKey":   c.setting.AppKey,
		"userName": strings.TrimSpace(req.UserName),
		"idCardNo": strings.TrimSpace(req.IdCardNo),
		"position": strings.TrimSpace(req.Position),
	}
	response, err := c.postForm(ctx, piggyEndpointContractScope, form)
	return response, digestAny(form), err
}

func (c *PiggyClient) GetContractSignURL(ctx context.Context, req PiggyContractSignURLRequest) (*PiggyContractSignURLResult, string, error) {
	form := map[string]string{
		"appKey":       c.setting.AppKey,
		"userName":     strings.TrimSpace(req.UserName),
		"idCardNo":     strings.TrimSpace(req.IdCardNo),
		"mobile":       strings.TrimSpace(req.Mobile),
		"bankAccount":  strings.TrimSpace(req.BankAccount),
		"position":     strings.TrimSpace(req.Position),
		"notifyUrl":    strings.TrimSpace(req.NotifyUrl),
		"jumpPage":     strings.TrimSpace(req.JumpPage),
		"customParams": strings.TrimSpace(req.CustomParams),
	}
	if strings.TrimSpace(req.InfoSource) != "" {
		form["infoSource"] = strings.TrimSpace(req.InfoSource)
	}
	if strings.TrimSpace(req.InfoSourcePrefix) != "" {
		form["infoSourcePrefix"] = strings.TrimSpace(req.InfoSourcePrefix)
	}
	// 小猪签约 URL 文档按字段声明“是否验签”，bankAccount 仅参与四要素认证但不参与签名。
	response, err := c.postFormWithSignedFields(ctx, piggyEndpointContractSignURL, form, piggyContractSignURLSignedFields)
	result := &PiggyContractSignURLResult{Raw: response}
	if response != nil {
		// 小猪官方文档返回 data 字符串，历史联调环境曾返回对象字段，这里两种结构都兼容。
		result.SignURL = firstNonEmpty(response.DataString, pickPiggyString(response.Data, "signUrl", "signURL", "url", "contractUrl"))
	}
	return result, digestAny(form), err
}

func (c *PiggyClient) QueryContractResult(ctx context.Context, req PiggyContractStatusRequest) (*PiggyAPIResponse, string, error) {
	form := map[string]string{
		"appKey":   c.setting.AppKey,
		"userName": strings.TrimSpace(req.UserName),
		"idCardNo": strings.TrimSpace(req.IdCardNo),
	}
	response, err := c.postForm(ctx, piggyEndpointContractResult, form)
	return response, digestAny(form), err
}

func (c *PiggyClient) PreviewContract(ctx context.Context, documentId string) (string, *PiggyAPIResponse, error) {
	if c == nil {
		return "", nil, errors.New("小猪 client 未初始化")
	}
	documentId = strings.TrimSpace(documentId)
	if documentId == "" {
		return "", nil, errors.New("小猪合同编号为空")
	}
	values := url.Values{}
	values.Set("documentId", documentId)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url(piggyEndpointContractPreview)+"?"+values.Encode(), nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Accept", "application/json")
	response, err := c.do(req)
	if err != nil {
		return "", response, err
	}
	previewURL := ""
	if response != nil {
		previewURL = strings.TrimSpace(response.DataString)
	}
	if previewURL == "" {
		return "", response, errors.New("小猪合同预览地址为空")
	}
	return previewURL, response, nil
}

func (c *PiggyClient) postJSON(ctx context.Context, endpoint string, payload map[string]any) (*PiggyAPIResponse, error) {
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(endpoint), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return c.do(req)
}

func (c *PiggyClient) postForm(ctx context.Context, endpoint string, form map[string]string) (*PiggyAPIResponse, error) {
	return c.postFormWithSignedFields(ctx, endpoint, form, nil)
}

func (c *PiggyClient) postFormWithSignedFields(ctx context.Context, endpoint string, form map[string]string, signedFields []string) (*PiggyAPIResponse, error) {
	values := url.Values{}
	for key, value := range form {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(endpoint), strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("sig", piggySignForm(c.setting.AppSecret, pickPiggyFormSignParams(form, signedFields)))
	return c.do(req)
}

func (c *PiggyClient) do(req *http.Request) (*PiggyAPIResponse, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("小猪接口 HTTP %d: %s", resp.StatusCode, string(body))
	}
	var result PiggyAPIResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	result.RawBody = string(body)
	if !result.Success() {
		return &result, fmt.Errorf("小猪接口失败 code=%s msg=%s error=%s", result.Code, result.Msg, result.ErrorMessage)
	}
	return &result, nil
}

func (c *PiggyClient) url(endpoint string) string {
	return strings.TrimRight(c.setting.Domain, "/") + endpoint
}

func (r *PiggyAPIResponse) Success() bool {
	if r == nil {
		return false
	}
	if r.IsSuccess != nil {
		return *r.IsSuccess
	}
	code := strings.TrimSpace(strings.ToLower(r.Code))
	msg := strings.TrimSpace(strings.ToLower(r.Msg))
	return code == "0" || code == "success" || msg == "success"
}

func pickPiggyString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := data[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(typed)
		default:
			return strings.TrimSpace(fmt.Sprintf("%v", typed))
		}
	}
	return ""
}

func normalizePiggyMoneyText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	cents, err := yuanToCents(value)
	if err != nil {
		return value
	}
	return centsToYuanString(cents)
}
