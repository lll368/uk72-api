package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type piggyRoundTripFunc func(req *http.Request) (*http.Response, error)

func (f piggyRoundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testPiggySetting() *operation_setting.PiggyWithdrawSetting {
	return &operation_setting.PiggyWithdrawSetting{
		Enabled:         true,
		Domain:          "https://piggy.example.com",
		AppKey:          "app-key",
		AppSecret:       "1234567890abcdef",
		AESIV:           "0000000000000000",
		TaxFundId:       "tax-fund",
		PositionName:    "技术服务",
		Position:        "tech",
		SignNotifyUrl:   "https://app.example.com/api/withdraw/piggy/contract/notify",
		PayNotifyUrl:    "https://app.example.com/api/withdraw/piggy/payment/notify",
		RequestTimeout:  5,
		CallbackLockTTL: 60,
		CooldownMinutes: 30,
		CalcType:        "C",
		PlatformFeeRate: 0,
	}
}

func TestPiggyAmountAndMaskUtilities(t *testing.T) {
	cents, err := yuanToCents("12.345")
	require.NoError(t, err)
	assert.Equal(t, int64(1235), cents)
	assert.Equal(t, "12.35", centsToYuanString(cents))
	assert.Equal(t, "110***********123", maskChineseIDCard("11010119900101123"))
	assert.Equal(t, "138****5678", maskMobile("13812345678"))
	assert.Equal(t, "6222********8888", maskBankCard("6222000011118888"))
	assert.Equal(t, "1234********cdef", maskPiggySecret("1234567890abcdef"))
}

func TestCalculatePiggyPlatformFee(t *testing.T) {
	tests := []struct {
		name              string
		requestedCents    int64
		rate              float64
		wantFeeCents      int64
		wantPiggyPayCents int64
		wantError         bool
	}{
		{name: "rounds cents half up", requestedCents: 9999, rate: 8, wantFeeCents: 800, wantPiggyPayCents: 9199},
		{name: "tiny amount keeps positive piggy amount", requestedCents: 1, rate: 8, wantFeeCents: 0, wantPiggyPayCents: 1},
		{name: "zero fee", requestedCents: 10000, rate: 0, wantFeeCents: 0, wantPiggyPayCents: 10000},
		{name: "near one hundred percent can leave one cent", requestedCents: 9999, rate: 99.99, wantFeeCents: 9998, wantPiggyPayCents: 1},
		{name: "near one hundred percent rejects non positive piggy amount", requestedCents: 1, rate: 99.99, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculatePiggyPlatformFee(tt.requestedCents, tt.rate)
			if tt.wantError {
				require.Error(t, err)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.requestedCents, result.RequestedAmountCents)
			assert.Equal(t, tt.wantFeeCents, result.PlatformFeeAmountCents)
			assert.Equal(t, tt.wantPiggyPayCents, result.PiggyPayAmountCents)
			assert.Equal(t, centsToYuanString(tt.wantFeeCents), result.PlatformFeeAmount)
			assert.Equal(t, centsToYuanString(tt.wantPiggyPayCents), result.PiggyTaxBeforeAmount)
		})
	}
}

func TestPiggyAESRoundTrip(t *testing.T) {
	encrypted, err := piggyEncryptAES([]byte(`{"outerTradeNo":"PWDR1"}`), "1234567890abcdef", "0000000000000000")
	require.NoError(t, err)
	decrypted, err := piggyDecryptAES(encrypted, "1234567890abcdef", "0000000000000000")
	require.NoError(t, err)
	assert.JSONEq(t, `{"outerTradeNo":"PWDR1"}`, string(decrypted))
}

func TestPiggySignFormMatchesOfficialV1Sample(t *testing.T) {
	sign := piggySignForm("testSecret", map[string]string{
		"appKey":    "testAppKey",
		"idCardNo":  "110275199911112222",
		"jumpPage":  "https://www.baidu.com",
		"mobile":    "13355566777",
		"notifyUrl": "https://test.xzsz.ltd/notify",
		"position":  "销售推广",
		"userName":  "李四",
	})
	assert.Equal(t, "d8effae18127b0f504c64d26faa5a048", sign)
}

func TestPiggyClientPostsEncryptedSubmitPayload(t *testing.T) {
	var capturedPath string
	var capturedBody string
	client := newPiggyClientWithHTTP(testPiggySetting(), piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedPath = req.URL.Path
		body, _ := io.ReadAll(req.Body)
		capturedBody = string(body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`)),
			Header:     make(http.Header),
		}, nil
	}))
	resp, digest, err := client.SingleOrderSubmit(context.Background(), PiggySubmitOrderRequest{
		NotifyUrl:    "https://app.example.com/api/withdraw/piggy/payment/notify",
		TaxFundId:    "tax-fund",
		OuterTradeNo: "PWDR1",
		EmpName:      "张三",
		EmpPhone:     "13812345678",
		LicenseType:  "ID_CARD",
		LicenseId:    "110101199001011234",
		SettleType:   "bankcard",
		PayAccount:   "6222000011118888",
		BankName:     "招商银行",
		PositionName: "技术服务",
		PayAmount:    "12.35",
		CalcType:     "C",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "/open/payment/singleOrderSubmit", capturedPath)
	assert.Contains(t, capturedBody, `"bizAESContent"`)
	assert.Contains(t, capturedBody, `"sign"`)
	assert.NotEmpty(t, digest)
}

func TestPiggyClientPostsTaxTrialCalcPayload(t *testing.T) {
	var capturedPath string
	var capturedBody string
	client := newPiggyClientWithHTTP(testPiggySetting(), piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedPath = req.URL.Path
		body, _ := io.ReadAll(req.Body)
		capturedBody = string(body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"code":"0",
				"msg":"success",
				"isSuccess":"T",
				"data":{
					"outerTradeNo":"PTRIAL1",
					"calcMonth":"2026-06",
					"pretaxAmount":100,
					"individualTaxAmount":3.5,
					"addedTaxAmount":1.06,
					"afterTaxAmount":95.44
				}
			}`)),
			Header: make(http.Header),
		}, nil
	}))

	result, digest, err := client.SingleTaxTrialCalc(context.Background(), PiggyTaxTrialCalcRequest{
		OuterTradeNo: "PTRIAL1",
		TaxFundId:    "tax-fund",
		LicenseId:    "110101199001011234",
		CalcAmount:   "100.00",
		CalcType:     "C",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "/open/payment/singleTaxTrialCalc", capturedPath)
	assert.JSONEq(t, `{
		"appKey":"app-key",
		"charset":"utf-8",
		"version":"3.0",
		"outerTradeNo":"PTRIAL1",
		"taxFundId":"tax-fund",
		"licenseId":"110101199001011234",
		"calcAmount":"100.00",
		"calcType":"C",
		"sign":"ignored"
	}`, replaceJSONSignForTest(t, capturedBody))
	assert.Equal(t, "PTRIAL1", result.OuterTradeNo)
	assert.Equal(t, "2026-06", result.CalcMonth)
	assert.Equal(t, "100.00", result.PretaxAmount)
	assert.Equal(t, "3.50", result.IndividualTaxAmount)
	assert.Equal(t, "1.06", result.AddedTaxAmount)
	assert.Equal(t, "95.44", result.AfterTaxAmount)
	assert.Equal(t, "C", result.CalcType)
	assert.NotEmpty(t, digest)
}

func replaceJSONSignForTest(t *testing.T, body string) string {
	t.Helper()
	var payload map[string]any
	require.NoError(t, common.Unmarshal([]byte(body), &payload))
	require.NotEmpty(t, payload["sign"])
	payload["sign"] = "ignored"
	normalized, err := common.Marshal(payload)
	require.NoError(t, err)
	return string(normalized)
}

func TestPiggyContractSignURL(t *testing.T) {
	setting := testPiggySetting()
	var capturedSig string
	var capturedBody string
	client := newPiggyClientWithHTTP(setting, piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/contract/sign/hasKeyByUrl", req.URL.Path)
		capturedSig = req.Header.Get("sig")
		body, _ := io.ReadAll(req.Body)
		capturedBody = string(body)
		assert.NotEmpty(t, capturedSig)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code":"0","msg":"success","isSuccess":true,"data":{"signUrl":"https://sign.example.com/u/1"}}`)),
			Header:     make(http.Header),
		}, nil
	}))
	result, digest, err := client.GetContractSignURL(context.Background(), PiggyContractSignURLRequest{
		UserName:     "张三",
		IdCardNo:     "110101199001011234",
		Mobile:       "13812345678",
		BankAccount:  "6222000011118888",
		Position:     "tech",
		NotifyUrl:    "https://app.example.com/api/withdraw/piggy/contract/notify",
		JumpPage:     "https://app.example.com/wallet",
		CustomParams: `{"userId":1}`,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://sign.example.com/u/1", result.SignURL)
	assert.NotEmpty(t, digest)

	form, err := url.ParseQuery(capturedBody)
	require.NoError(t, err)
	assert.Equal(t, "6222000011118888", form.Get("bankAccount"))
	expectedSignedFields := map[string]string{
		"appKey":       setting.AppKey,
		"userName":     "张三",
		"idCardNo":     "110101199001011234",
		"mobile":       "13812345678",
		"position":     "tech",
		"notifyUrl":    "https://app.example.com/api/withdraw/piggy/contract/notify",
		"jumpPage":     "https://app.example.com/wallet",
		"customParams": `{"userId":1}`,
	}
	assert.Equal(t, piggySignForm(setting.AppSecret, expectedSignedFields), capturedSig)
	// bankAccount 按小猪文档不参与发起签约 URL 的验签，但仍随表单传给小猪做四要素认证。
	expectedSignedFields["bankAccount"] = "6222000011118888"
	assert.NotEqual(t, piggySignForm(setting.AppSecret, expectedSignedFields), capturedSig)
}

func TestPiggyContractSignURLAcceptsNumericCode(t *testing.T) {
	client := newPiggyClientWithHTTP(testPiggySetting(), piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code":0,"msg":"success","isSuccess":true,"data":{"signUrl":"https://sign.example.com/u/1"}}`)),
			Header:     make(http.Header),
		}, nil
	}))
	result, digest, err := client.GetContractSignURL(context.Background(), PiggyContractSignURLRequest{
		UserName:     "张三",
		IdCardNo:     "110101199001011234",
		Mobile:       "13812345678",
		BankAccount:  "6222000011118888",
		Position:     "tech",
		NotifyUrl:    "https://app.example.com/api/withdraw/piggy/contract/notify",
		JumpPage:     "https://app.example.com/wallet",
		CustomParams: `{"userId":1}`,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Raw)
	assert.Equal(t, "0", result.Raw.Code)
	assert.Equal(t, "https://sign.example.com/u/1", result.SignURL)
	assert.NotEmpty(t, digest)
}

func TestPiggyContractSignURLAcceptsOfficialStringData(t *testing.T) {
	client := newPiggyClientWithHTTP(testPiggySetting(), piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/contract/sign/hasKeyByUrl", req.URL.Path)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code":0,"msg":"success","data":"https://sign.example.com/u/official"}`)),
			Header:     make(http.Header),
		}, nil
	}))
	result, digest, err := client.GetContractSignURL(context.Background(), PiggyContractSignURLRequest{
		UserName:     "张三",
		IdCardNo:     "110101199001011234",
		Mobile:       "13812345678",
		BankAccount:  "6222000011118888",
		Position:     "tech",
		NotifyUrl:    "https://app.example.com/api/withdraw/piggy/contract/notify",
		JumpPage:     "https://app.example.com/wallet",
		CustomParams: `{"userId":1}`,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "https://sign.example.com/u/official", result.SignURL)
	assert.NotEmpty(t, digest)
}

func TestPiggyContractPreviewReturnsOfficialDataString(t *testing.T) {
	client := newPiggyClientWithHTTP(testPiggySetting(), piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "/contract/sign/viewContract", req.URL.Path)
		assert.Equal(t, "DOC-2102", req.URL.Query().Get("documentId"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code":0,"msg":"success","data":"https://preview.example.com/contracts/DOC-2102"}`)),
			Header:     make(http.Header),
		}, nil
	}))

	previewURL, resp, err := client.PreviewContract(context.Background(), " DOC-2102 ")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "https://preview.example.com/contracts/DOC-2102", previewURL)
	assert.Equal(t, "https://preview.example.com/contracts/DOC-2102", resp.DataString)
}

func TestPiggyContractPreviewRejectsFailureAndEmptyData(t *testing.T) {
	t.Run("provider failure", func(t *testing.T) {
		client := newPiggyClientWithHTTP(testPiggySetting(), piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"code":1,"msg":"document not found","data":""}`)),
				Header:     make(http.Header),
			}, nil
		}))

		previewURL, resp, err := client.PreviewContract(context.Background(), "DOC-MISSING")

		require.Error(t, err)
		require.NotNil(t, resp)
		assert.Empty(t, previewURL)
		assert.Contains(t, err.Error(), "小猪接口失败")
	})

	t.Run("success with empty data", func(t *testing.T) {
		client := newPiggyClientWithHTTP(testPiggySetting(), piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"code":0,"msg":"success","data":""}`)),
				Header:     make(http.Header),
			}, nil
		}))

		previewURL, resp, err := client.PreviewContract(context.Background(), "DOC-EMPTY")

		require.Error(t, err)
		require.NotNil(t, resp)
		assert.Empty(t, previewURL)
		assert.Contains(t, err.Error(), "预览地址为空")
	})

	t.Run("malformed response body", func(t *testing.T) {
		client := newPiggyClientWithHTTP(testPiggySetting(), piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"code":0,"msg":"success","data":`)),
				Header:     make(http.Header),
			}, nil
		}))

		previewURL, resp, err := client.PreviewContract(context.Background(), "DOC-MALFORMED")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Empty(t, previewURL)
	})

	t.Run("empty document id", func(t *testing.T) {
		client := newPiggyClientWithHTTP(testPiggySetting(), piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("empty document id must not call Piggy")
			return nil, nil
		}))

		previewURL, resp, err := client.PreviewContract(context.Background(), " ")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Empty(t, previewURL)
		assert.Contains(t, err.Error(), "合同编号为空")
	})
}

func TestPiggyContractResultQueryUsesOfficialEndpointAndArrayData(t *testing.T) {
	var capturedBody string
	client := newPiggyClientWithHTTP(testPiggySetting(), piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/contract/sign/getSignedResult", req.URL.Path)
		body, _ := io.ReadAll(req.Body)
		capturedBody = string(body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code":0,"msg":"success","data":[{"name":"张三","idCardNo":"110101199001011234","document_id":"DOC-2102"}]}`)),
			Header:     make(http.Header),
		}, nil
	}))

	resp, digest, err := client.QueryContractResult(context.Background(), PiggyContractStatusRequest{
		UserName: "张三",
		IdCardNo: "110101199001011234",
		Position: "tech",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	form, err := url.ParseQuery(capturedBody)
	require.NoError(t, err)
	assert.Equal(t, "张三", form.Get("userName"))
	assert.Equal(t, "110101199001011234", form.Get("idCardNo"))
	assert.Empty(t, form.Get("position"))
	assert.JSONEq(t, `[{"name":"张三","idCardNo":"110101199001011234","document_id":"DOC-2102"}]`, string(resp.RawData))
	assert.NotEmpty(t, digest)
}

func TestPiggyAPIResponseUnmarshalCompatibleIsSuccess(t *testing.T) {
	tt := []struct {
		name      string
		body      string
		expectNil bool
		expectVal bool
	}{
		{
			name:      "boolean true",
			body:      `{"code":"0","msg":"success","isSuccess":true}`,
			expectVal: true,
		},
		{
			name:      "string true",
			body:      `{"code":"0","msg":"success","isSuccess":"true"}`,
			expectVal: true,
		},
		{
			name:      "number one",
			body:      `{"code":"0","msg":"success","isSuccess":1}`,
			expectVal: true,
		},
		{
			name:      "number one decimal",
			body:      `{"code":"0","msg":"success","isSuccess":1.0}`,
			expectVal: true,
		},
		{
			name:      "number zero",
			body:      `{"code":"0","msg":"success","isSuccess":0}`,
			expectVal: false,
		},
		{
			name:      "number zero decimal",
			body:      `{"code":"0","msg":"success","isSuccess":0.0}`,
			expectVal: false,
		},
		{
			name:      "nil",
			body:      `{"code":"0","msg":"success","isSuccess":null}`,
			expectNil: true,
		},
	}

	for _, item := range tt {
		item := item
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()
			var resp PiggyAPIResponse
			err := json.Unmarshal([]byte(item.body), &resp)
			require.NoError(t, err)
			if item.expectNil {
				assert.Nil(t, resp.IsSuccess)
				return
			}
			require.NotNil(t, resp.IsSuccess)
			assert.Equal(t, item.expectVal, *resp.IsSuccess)
		})
	}
}
