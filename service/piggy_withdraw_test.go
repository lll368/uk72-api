package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configurePiggyWithdrawForTest(t *testing.T, domain string) {
	t.Helper()
	setting := operation_setting.GetPiggyWithdrawSetting()
	original := *setting
	paymentSetting := operation_setting.GetPaymentSetting()
	originalMinWithdraw := paymentSetting.CommissionMinWithdrawAmount
	t.Cleanup(func() {
		*setting = original
		paymentSetting.CommissionMinWithdrawAmount = originalMinWithdraw
	})
	*setting = *testPiggySetting()
	setting.Domain = domain
	setting.SignJumpPage = "https://app.example.com/wallet"
	paymentSetting.CommissionMinWithdrawAmount = 1
}

func mockPiggyClientForTest(t *testing.T, handler func(req *http.Request) string) {
	t.Helper()
	mockPiggyClientRoundTripForTest(t, func(req *http.Request) (*http.Response, error) {
		body := handler(req)
		if body == "" {
			body = `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(body),
			Header:     make(http.Header),
		}, nil
	})
}

func mockPiggyClientRoundTripForTest(t *testing.T, handler func(req *http.Request) (*http.Response, error)) {
	t.Helper()
	original := newConfiguredPiggyClient
	originalPreview := newPiggyPreviewClient
	newConfiguredPiggyClient = func(setting *operation_setting.PiggyWithdrawSetting) (*PiggyClient, error) {
		return newPiggyClientWithHTTP(setting, piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return handler(req)
		})), nil
	}
	newPiggyPreviewClient = func(setting *operation_setting.PiggyWithdrawSetting) (*PiggyClient, error) {
		return newPiggyClientWithHTTP(setting, piggyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return handler(req)
		})), nil
	}
	t.Cleanup(func() {
		newConfiguredPiggyClient = original
		newPiggyPreviewClient = originalPreview
	})
}

func ioNopCloser(value string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(value))
}

func waitForPiggyConfirmAttempt(t *testing.T, attempts <-chan struct{}) {
	t.Helper()
	select {
	case <-attempts:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for piggy confirm attempt")
	}
}

func waitForPiggyCancelAttempt(t *testing.T, attempts <-chan struct{}) {
	t.Helper()
	select {
	case <-attempts:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for piggy cancel attempt")
	}
}

func approvePiggyWithdrawForTest(t *testing.T, order *model.WithdrawOrder) model.WithdrawOrder {
	t.Helper()
	require.NotNil(t, order)
	require.NoError(t, AdminApproveWithdrawOrder(context.Background(), order.Id, 7, "approved"))
	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	return refreshed
}

func parsePiggySubmitOrderRequestForTest(t *testing.T, req *http.Request) PiggySubmitOrderRequest {
	t.Helper()
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	var envelope struct {
		BizAESContent string `json:"bizAESContent"`
	}
	require.NoError(t, common.Unmarshal(body, &envelope))
	setting := operation_setting.GetPiggyWithdrawSetting()
	plain, err := piggyDecryptAES(envelope.BizAESContent, setting.AppSecret, setting.AESIV)
	require.NoError(t, err)
	var submit PiggySubmitOrderRequest
	require.NoError(t, common.Unmarshal(plain, &submit))
	return submit
}

func seedVerifiedWithdrawalPhone(t *testing.T, userId int, phone string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.UserProfile{
		UserId:          userId,
		PhoneNumber:     &phone,
		PhoneVerifiedAt: common.GetTimestamp(),
	}).Error)
}

func seedSignedPiggyProfile(t *testing.T, userId int) {
	t.Helper()
	_, err := SaveWithdrawalProfile(userId, WithdrawalProfileInput{
		AccountType: model.WithdrawAccountTypeBankcard,
		RealName:    "张三",
		IdCardNo:    "110101199001011234",
		Mobile:      "13812345678",
		BankCardNo:  "6222000011118888",
		BankName:    "招商银行",
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawalProfile{}).Where("user_id = ?", userId).Updates(map[string]interface{}{
		"piggy_sign_status": model.PiggySignStatusSigned,
		"piggy_signed_at":   common.GetTimestamp(),
	}).Error)
}

func buildSignedPiggyPaymentCallback(t *testing.T, content PiggyPaymentCallbackContent) []byte {
	return buildSignedPiggyPaymentCallbackWithIsSuccess(t, content, true)
}

func buildSignedPiggyPaymentCallbackWithIsSuccess(t *testing.T, content PiggyPaymentCallbackContent, isSuccess any) []byte {
	t.Helper()
	setting := operation_setting.GetPiggyWithdrawSetting()
	plain, err := piggyCompactJSON(content)
	require.NoError(t, err)
	encrypted, err := piggyEncryptAES([]byte(plain), setting.AppSecret, setting.AESIV)
	require.NoError(t, err)
	payload := map[string]any{
		"code":      "0",
		"msg":       "success",
		"isSuccess": isSuccess,
		"data": map[string]any{
			"bizAESContent": encrypted,
		},
	}
	sign, err := piggySignJSON(setting.AppSecret, payload)
	require.NoError(t, err)
	payload["sign"] = sign
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	return body
}

func buildSignedPiggyPaymentCallbackFromMap(t *testing.T, content map[string]any) []byte {
	t.Helper()
	setting := operation_setting.GetPiggyWithdrawSetting()
	plain, err := common.Marshal(content)
	require.NoError(t, err)
	encrypted, err := piggyEncryptAES(plain, setting.AppSecret, setting.AESIV)
	require.NoError(t, err)
	payload := map[string]any{
		"code":      "0",
		"msg":       "success",
		"isSuccess": true,
		"data": map[string]any{
			"bizAESContent": encrypted,
		},
	}
	sign, err := piggySignJSON(setting.AppSecret, payload)
	require.NoError(t, err)
	payload["sign"] = sign
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	return body
}

func buildUnsignedPiggyPaymentCallback(t *testing.T, content PiggyPaymentCallbackContent) []byte {
	t.Helper()
	setting := operation_setting.GetPiggyWithdrawSetting()
	plain, err := piggyCompactJSON(content)
	require.NoError(t, err)
	encrypted, err := piggyEncryptAES([]byte(plain), setting.AppSecret, setting.AESIV)
	require.NoError(t, err)
	body, err := common.Marshal(map[string]any{
		"code":      "0",
		"msg":       "success",
		"isSuccess": "T",
		"data": map[string]any{
			"bizAESContent": encrypted,
		},
	})
	require.NoError(t, err)
	return body
}

func buildPiggyEncryptedQueryResponseForTest(t *testing.T, content PiggyPaymentCallbackContent) string {
	t.Helper()
	setting := operation_setting.GetPiggyWithdrawSetting()
	plain, err := piggyCompactJSON(content)
	require.NoError(t, err)
	encrypted, err := piggyEncryptAES([]byte(plain), setting.AppSecret, setting.AESIV)
	require.NoError(t, err)
	body, err := common.Marshal(map[string]any{
		"code":      "0",
		"msg":       "success",
		"isSuccess": "T",
		"data": map[string]any{
			"outerTradeNo":  content.OuterTradeNo,
			"bizAESContent": encrypted,
		},
	})
	require.NoError(t, err)
	return string(body)
}

func piggyCallbackContentFromOrderForTest(order *model.WithdrawOrder, notifyType string, tradeStatus string) PiggyPaymentCallbackContent {
	pretaxCents := expectedPiggyPretaxAmountCents(order)
	taxCents := int64(50)
	if pretaxCents < taxCents {
		taxCents = 0
	}
	return PiggyPaymentCallbackContent{
		OuterTradeNo:        order.WithdrawNo,
		NotifyType:          notifyType,
		TradeStatus:         tradeStatus,
		FrontLogNo:          "front-" + order.WithdrawNo,
		LaborOrderNo:        "labor-" + order.WithdrawNo,
		EmpName:             order.AccountName,
		EmpPhone:            order.PayoutMobile,
		LicenseType:         "ID_CARD",
		LicenseId:           order.PayoutIdCardNo,
		SettleType:          model.WithdrawAccountTypeBankcard,
		PayAccount:          order.PayoutBankCardNo,
		BankName:            order.BankName,
		PositionName:        firstNonEmpty(order.PositionName, operation_setting.GetPiggyWithdrawSetting().PositionName),
		PretaxAmount:        centsToYuanString(pretaxCents),
		IndividualTaxAmount: centsToYuanString(taxCents),
		AfterTaxAmount:      centsToYuanString(pretaxCents - taxCents),
		FeeAmount:           "0.00",
		CalcType:            firstNonEmpty(order.CalcType, operation_setting.GetPiggyWithdrawSetting().CalcType),
	}
}

func buildSignedPiggyContractCallback(t *testing.T, userId int) []byte {
	t.Helper()
	return buildSignedPiggyContractCallbackWithCustomParams(t, map[string]any{"userId": userId}, "110101199001011234")
}

func buildSignedPiggyContractCallbackWithCustomParams(t *testing.T, customParams any, idCardNo string) []byte {
	t.Helper()
	setting := operation_setting.GetPiggyWithdrawSetting()
	payload := map[string]any{
		"userName":   "张三",
		"idCardNo":   idCardNo,
		"mobile":     "13812345678",
		"signStatus": "success",
	}
	if customParams != nil {
		payload["customParams"] = customParams
	}
	sign, err := piggySignJSON(setting.AppSecret, payload)
	require.NoError(t, err)
	payload["sign"] = sign
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	return body
}

func buildOfficialPiggyContractCallback(t *testing.T, customParams any, overrides map[string]any) []byte {
	t.Helper()
	data := map[string]any{
		"contract_url":    "https://uat.xzsz.ltd/contract/official-2102",
		"name":            "张三",
		"idCardNo":        "110101199001011234",
		"mobile":          "13812345678",
		"bankAccount":     "6222000011118888",
		"subsidiary_name": "小猪签约结算公司",
		"document_id":     "DOC-2102",
	}
	if customParams != nil {
		data["customParams"] = customParams
	}
	for key, value := range overrides {
		data[key] = value
	}
	body, err := common.Marshal(map[string]any{
		"code": "0",
		"msg":  "合同签署成功",
		"data": data,
	})
	require.NoError(t, err)
	return body
}

func TestWithdrawalProfileSaveMasksAndRejectsUnsupportedAccount(t *testing.T) {
	truncate(t)
	seedUser(t, 2101, 0)
	seedVerifiedWithdrawalPhone(t, 2101, "+8613812345678")

	_, err := SaveWithdrawalProfile(2101, WithdrawalProfileInput{AccountType: "alipay"})
	require.Error(t, err)

	profile, err := SaveWithdrawalProfile(2101, WithdrawalProfileInput{
		AccountType: model.WithdrawAccountTypeBankcard,
		RealName:    "张三",
		IdCardNo:    "110101199001011234",
		Mobile:      "13812345678",
		BankCardNo:  "6222000011118888",
		BankName:    "招商银行",
	})
	require.NoError(t, err)
	assert.Equal(t, "110************234", profile.MaskedIdCardNo)
	assert.Equal(t, "138****5678", profile.MaskedMobile)
	assert.Equal(t, "6222********8888", profile.MaskedBankCardNo)
}

func TestWithdrawalProfileSaveNormalizesMobileWithoutAccountBinding(t *testing.T) {
	t.Run("success with verified mainland phone", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2130, 0)
		seedVerifiedWithdrawalPhone(t, 2130, "+8613812345678")

		profile, err := SaveWithdrawalProfile(2130, WithdrawalProfileInput{
			AccountType: model.WithdrawAccountTypeBankcard,
			RealName:    "张三",
			IdCardNo:    "110101199001011234",
			Mobile:      "13812345678",
			BankCardNo:  "6222000011118888",
			BankName:    "招商银行",
		})

		require.NoError(t, err)
		assert.Equal(t, "138****5678", profile.MaskedMobile)

		raw, err := getRawWithdrawalProfile(2130)
		require.NoError(t, err)
		assert.Equal(t, "13812345678", raw.Mobile)
	})

	t.Run("missing verified phone is allowed", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2131, 0)

		profile, err := SaveWithdrawalProfile(2131, WithdrawalProfileInput{
			AccountType: model.WithdrawAccountTypeBankcard,
			RealName:    "张三",
			IdCardNo:    "110101199001011234",
			Mobile:      "13812345678",
			BankCardNo:  "6222000011118888",
			BankName:    "招商银行",
		})

		require.NoError(t, err)
		assert.Equal(t, "138****5678", profile.MaskedMobile)

		raw, err := getRawWithdrawalProfile(2131)
		require.NoError(t, err)
		assert.Equal(t, "13812345678", raw.Mobile)
	})

	t.Run("unverified account phone is allowed", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2132, 0)
		phone := "+8613812345678"
		require.NoError(t, model.DB.Create(&model.UserProfile{
			UserId:      2132,
			PhoneNumber: &phone,
		}).Error)

		profile, err := SaveWithdrawalProfile(2132, WithdrawalProfileInput{
			AccountType: model.WithdrawAccountTypeBankcard,
			RealName:    "张三",
			IdCardNo:    "110101199001011234",
			Mobile:      "13812345678",
			BankCardNo:  "6222000011118888",
			BankName:    "招商银行",
		})

		require.NoError(t, err)
		assert.Equal(t, "138****5678", profile.MaskedMobile)
	})

	t.Run("mismatched account phone is allowed", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2133, 0)
		seedVerifiedWithdrawalPhone(t, 2133, "+8613812345678")

		profile, err := SaveWithdrawalProfile(2133, WithdrawalProfileInput{
			AccountType: model.WithdrawAccountTypeBankcard,
			RealName:    "张三",
			IdCardNo:    "110101199001011234",
			Mobile:      "13912345678",
			BankCardNo:  "6222000011118888",
			BankName:    "招商银行",
		})

		require.NoError(t, err)
		assert.Equal(t, "139****5678", profile.MaskedMobile)

		raw, err := getRawWithdrawalProfile(2133)
		require.NoError(t, err)
		assert.Equal(t, "13912345678", raw.Mobile)
	})

	t.Run("invalid phone format is rejected", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2134, 0)
		seedVerifiedWithdrawalPhone(t, 2134, "+8613812345678")

		_, err := SaveWithdrawalProfile(2134, WithdrawalProfileInput{
			AccountType: model.WithdrawAccountTypeBankcard,
			RealName:    "张三",
			IdCardNo:    "110101199001011234",
			Mobile:      "1381234567",
			BankCardNo:  "6222000011118888",
			BankName:    "招商银行",
		})

		require.Error(t, err)
	})

	t.Run("invalid mainland phone with country code is rejected", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2135, 0)
		seedVerifiedWithdrawalPhone(t, 2135, "+8601234567890")

		_, err := SaveWithdrawalProfile(2135, WithdrawalProfileInput{
			AccountType: model.WithdrawAccountTypeBankcard,
			RealName:    "张三",
			IdCardNo:    "110101199001011234",
			Mobile:      "+8601234567890",
			BankCardNo:  "6222000011118888",
			BankName:    "招商银行",
		})

		require.ErrorIs(t, err, ErrWithdrawalPhoneInvalid)
	})
}

func TestWithdrawalProfileSavePreservesSigningForPaymentDetailChanges(t *testing.T) {
	truncate(t)
	seedUser(t, 2148, 0)
	seedSignedPiggyProfile(t, 2148)

	before, err := getRawWithdrawalProfile(2148)
	require.NoError(t, err)
	require.Equal(t, model.PiggySignStatusSigned, before.PiggySignStatus)

	_, err = SaveWithdrawalProfile(2148, WithdrawalProfileInput{
		AccountType: model.WithdrawAccountTypeBankcard,
		RealName:    "张三",
		IdCardNo:    "110101199001011234",
		Mobile:      "13912345678",
		BankCardNo:  "6222000099998888",
		BankName:    "建设银行",
	})

	require.NoError(t, err)
	after, err := getRawWithdrawalProfile(2148)
	require.NoError(t, err)
	assert.Equal(t, model.PiggySignStatusSigned, after.PiggySignStatus)
	assert.Equal(t, before.PiggySignedAt, after.PiggySignedAt)
}

func TestWithdrawalProfileSaveResetsSigningForContractIdentityChanges(t *testing.T) {
	tests := []struct {
		name  string
		user  int
		input WithdrawalProfileInput
	}{
		{
			name: "real name changed",
			user: 2149,
			input: WithdrawalProfileInput{
				AccountType: model.WithdrawAccountTypeBankcard,
				RealName:    "李四",
				IdCardNo:    "110101199001011234",
				Mobile:      "13812345678",
				BankCardNo:  "6222000011118888",
				BankName:    "招商银行",
			},
		},
		{
			name: "id card changed",
			user: 2150,
			input: WithdrawalProfileInput{
				AccountType: model.WithdrawAccountTypeBankcard,
				RealName:    "张三",
				IdCardNo:    "110101199001011235",
				Mobile:      "13812345678",
				BankCardNo:  "6222000011118888",
				BankName:    "招商银行",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncate(t)
			seedUser(t, tt.user, 0)
			seedSignedPiggyProfile(t, tt.user)

			_, err := SaveWithdrawalProfile(tt.user, tt.input)

			require.NoError(t, err)
			after, err := getRawWithdrawalProfile(tt.user)
			require.NoError(t, err)
			assert.Equal(t, model.PiggySignStatusUnsigned, after.PiggySignStatus)
			assert.Zero(t, after.PiggySignedAt)
		})
	}
}

func TestPiggyContractScopeSnapshotCurrentness(t *testing.T) {
	setting := testPiggySetting()

	assert.True(t, isPiggyContractSignedForCurrentScope(&model.WithdrawalProfile{
		PiggySignStatus: model.PiggySignStatusSigned,
	}, setting))
	assert.True(t, isPiggyContractSignedForCurrentScope(&model.WithdrawalProfile{
		PiggySignStatus:           model.PiggySignStatusSigned,
		PiggyContractPosition:     setting.PositionName,
		PiggyContractPositionName: setting.PositionName,
		PiggyContractTaxFundID:    setting.TaxFundId,
	}, setting))
	assert.True(t, isPiggyContractSignedForCurrentScope(&model.WithdrawalProfile{
		PiggySignStatus:           model.PiggySignStatusSigned,
		PiggyContractPosition:     setting.Position,
		PiggyContractPositionName: setting.PositionName,
		PiggyContractTaxFundID:    setting.TaxFundId,
	}, setting))
	assert.False(t, isPiggyContractSignedForCurrentScope(&model.WithdrawalProfile{
		PiggySignStatus:           model.PiggySignStatusSigned,
		PiggyContractPosition:     setting.PositionName,
		PiggyContractPositionName: setting.PositionName,
		PiggyContractTaxFundID:    "old-tax-fund",
	}, setting))
}

func TestPiggyContractSignURLAndCallback(t *testing.T) {
	truncate(t)
	seedUser(t, 2102, 0)
	seedVerifiedWithdrawalPhone(t, 2102, "+8613812345678")
	_, err := SaveWithdrawalProfile(2102, WithdrawalProfileInput{
		AccountType: model.WithdrawAccountTypeBankcard,
		RealName:    "张三",
		IdCardNo:    "110101199001011234",
		Mobile:      "13812345678",
		BankCardNo:  "6222000011118888",
		BankName:    "招商银行",
	})
	require.NoError(t, err)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		assert.Equal(t, "/contract/sign/hasKeyByUrl", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		assert.Equal(t, "技术服务", form.Get("position"))
		return `{"code":"0","msg":"success","isSuccess":true,"data":{"signUrl":"https://sign.example.com/2102"}}`
	})

	result, err := GetPiggyContractSignURL(context.Background(), 2102)
	require.NoError(t, err)
	assert.Equal(t, "https://sign.example.com/2102", result["sign_url"])

	require.NoError(t, HandlePiggyContractCallback(buildSignedPiggyContractCallback(t, 2102), ""))
	profile, err := getRawWithdrawalProfile(2102)
	require.NoError(t, err)
	assert.Equal(t, model.PiggySignStatusSigned, profile.PiggySignStatus)
}

func TestPiggyContractSignURLRejectsEmptyURL(t *testing.T) {
	truncate(t)
	seedUser(t, 2157, 0)
	seedVerifiedWithdrawalPhone(t, 2157, "+8613812345678")
	_, err := SaveWithdrawalProfile(2157, WithdrawalProfileInput{
		AccountType: model.WithdrawAccountTypeBankcard,
		RealName:    "张三",
		IdCardNo:    "110101199001011234",
		Mobile:      "13812345678",
		BankCardNo:  "6222000011118888",
		BankName:    "招商银行",
	})
	require.NoError(t, err)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		assert.Equal(t, "/contract/sign/hasKeyByUrl", r.URL.Path)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	result, err := GetPiggyContractSignURL(context.Background(), 2157)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "签约地址为空")
}

func TestPiggyContractStatusRefreshUpdatesProfileFromSignedResult(t *testing.T) {
	truncate(t)
	seedUser(t, 2159, 0)
	seedVerifiedWithdrawalPhone(t, 2159, "+8613812345678")
	_, err := SaveWithdrawalProfile(2159, WithdrawalProfileInput{
		AccountType: model.WithdrawAccountTypeBankcard,
		RealName:    "张三",
		IdCardNo:    "110101199001011234",
		Mobile:      "13812345678",
		BankCardNo:  "6222000011118888",
		BankName:    "招商银行",
	})
	require.NoError(t, err)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		assert.Equal(t, "/contract/sign/getSignedResult", r.URL.Path)
		return `{"code":0,"msg":"success","data":[{"company_name":"小猪平台","name":"张三","idCardNo":"110101199001011234","mobile":"13812345678","document_id":"DOC-2159","subsidiary_name":"小猪签约结算公司","sign_time":"2026-06-11 10:20:30","position":"技术服务","bankAccount":"6222000011118888"}]}`
	})

	profile, err := RefreshPiggyContractStatus(context.Background(), 2159)

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, model.PiggySignStatusSigned, profile.PiggySignStatus)
	assert.Equal(t, "DOC-2159", profile.PiggyContractDocumentID)
	assert.Equal(t, "小猪签约结算公司", profile.PiggyContractSubsidiaryName)
	raw, err := getRawWithdrawalProfile(2159)
	require.NoError(t, err)
	assert.Equal(t, model.PiggySignStatusSigned, raw.PiggySignStatus)
	assert.NotZero(t, raw.PiggySignedAt)
	assert.Equal(t, "DOC-2159", raw.PiggyContractDocumentID)
	assert.Equal(t, "小猪签约结算公司", raw.PiggyContractSubsidiaryName)
	assert.Equal(t, "技术服务", raw.PiggyContractPosition)
	assert.Equal(t, "技术服务", raw.PiggyContractPositionName)
	assert.Equal(t, "tax-fund", raw.PiggyContractTaxFundID)
	assert.NotEmpty(t, raw.LastCallbackDigest)
}

func TestPiggyContractStatusRefreshRejectsMissingResult(t *testing.T) {
	truncate(t)
	seedUser(t, 2160, 0)
	seedVerifiedWithdrawalPhone(t, 2160, "+8613812345678")
	_, err := SaveWithdrawalProfile(2160, WithdrawalProfileInput{
		AccountType: model.WithdrawAccountTypeBankcard,
		RealName:    "张三",
		IdCardNo:    "110101199001011234",
		Mobile:      "13812345678",
		BankCardNo:  "6222000011118888",
		BankName:    "招商银行",
	})
	require.NoError(t, err)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		assert.Equal(t, "/contract/sign/getSignedResult", r.URL.Path)
		return `{"code":0,"msg":"success","data":[]}`
	})

	profile, err := RefreshPiggyContractStatus(context.Background(), 2160)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "未查询到")
	assert.Nil(t, profile)
	raw, err := getRawWithdrawalProfile(2160)
	require.NoError(t, err)
	assert.Equal(t, model.PiggySignStatusUnsigned, raw.PiggySignStatus)
	assert.Zero(t, raw.PiggySignedAt)
}

func TestPiggyContractPreviewUsesStoredDocumentIDRegardlessWithdrawEligibility(t *testing.T) {
	truncate(t)
	seedUser(t, 2168, 0)
	seedWalletAccount(t, 2168, 0, 0, 0)
	seedSignedPiggyProfile(t, 2168)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPaymentSetting().CommissionMinWithdrawAmount = 100
	operation_setting.GetPiggyWithdrawSetting().ForbiddenWithdrawTime = "00:00-23:59"
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 60
	require.NoError(t, model.DB.Create(&model.WithdrawOrder{
		UserId:     2168,
		WithdrawNo: "PWDR2168COOLDOWN",
		Status:     model.WithdrawStatusPending,
		Provider:   model.WithdrawProviderPiggyLaborV3,
		CreatedAt:  common.GetTimestamp(),
	}).Error)
	require.NoError(t, model.DB.Model(&model.WithdrawalProfile{}).Where("user_id = ?", 2168).Updates(map[string]interface{}{
		"piggy_contract_document_id":   "DOC-2168",
		"piggy_contract_position":      "技术服务",
		"piggy_contract_position_name": "技术服务",
		"piggy_contract_tax_fund_id":   "tax-fund",
	}).Error)

	var paths []string
	mockPiggyClientForTest(t, func(r *http.Request) string {
		paths = append(paths, r.URL.Path)
		assert.Equal(t, "/contract/sign/viewContract", r.URL.Path)
		assert.Equal(t, "DOC-2168", r.URL.Query().Get("documentId"))
		return `{"code":0,"msg":"success","data":"https://preview.example.com/contracts/DOC-2168"}`
	})

	result, err := GetPiggyContractPreviewURL(context.Background(), 2168)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "DOC-2168", result.DocumentID)
	assert.Equal(t, "https://preview.example.com/contracts/DOC-2168", result.PreviewURL)
	assert.Equal(t, []string{"/contract/sign/viewContract"}, paths)
}

func TestPiggyContractPreviewRefreshesMissingDocumentIDBeforePreview(t *testing.T) {
	truncate(t)
	seedUser(t, 2169, 0)
	seedSignedPiggyProfile(t, 2169)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	require.NoError(t, model.DB.Model(&model.WithdrawalProfile{}).Where("user_id = ?", 2169).Updates(map[string]interface{}{
		"piggy_contract_position":      "技术服务",
		"piggy_contract_position_name": "技术服务",
		"piggy_contract_tax_fund_id":   "tax-fund",
	}).Error)

	var paths []string
	mockPiggyClientForTest(t, func(r *http.Request) string {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/contract/sign/getSignedResult":
			return `{"code":0,"msg":"success","data":[{"company_name":"小猪平台","name":"张三","idCardNo":"110101199001011234","mobile":"13812345678","document_id":"DOC-2169","subsidiary_name":"小猪签约结算公司","sign_time":"2026-06-11 10:20:30","position":"技术服务","bankAccount":"6222000011118888"}]}`
		case "/contract/sign/viewContract":
			assert.Equal(t, "DOC-2169", r.URL.Query().Get("documentId"))
			return `{"code":0,"msg":"success","data":"https://preview.example.com/contracts/DOC-2169"}`
		default:
			t.Fatalf("unexpected Piggy endpoint: %s", r.URL.Path)
			return ""
		}
	})

	result, err := GetPiggyContractPreviewURL(context.Background(), 2169)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "DOC-2169", result.DocumentID)
	assert.Equal(t, "https://preview.example.com/contracts/DOC-2169", result.PreviewURL)
	assert.Equal(t, []string{"/contract/sign/getSignedResult", "/contract/sign/viewContract"}, paths)
	raw, err := getRawWithdrawalProfile(2169)
	require.NoError(t, err)
	assert.Equal(t, "DOC-2169", raw.PiggyContractDocumentID)
}

func TestPiggyContractPreviewRejectsUnavailableContractStates(t *testing.T) {
	t.Run("unsigned", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2170, 0)
		seedVerifiedWithdrawalPhone(t, 2170, "+8613812345678")
		_, err := SaveWithdrawalProfile(2170, WithdrawalProfileInput{
			AccountType: model.WithdrawAccountTypeBankcard,
			RealName:    "张三",
			IdCardNo:    "110101199001011234",
			Mobile:      "13812345678",
			BankCardNo:  "6222000011118888",
			BankName:    "招商银行",
		})
		require.NoError(t, err)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		mockPiggyClientForTest(t, func(r *http.Request) string {
			t.Fatal("unsigned contract must not call Piggy preview")
			return ""
		})

		result, err := GetPiggyContractPreviewURL(context.Background(), 2170)

		require.ErrorIs(t, err, ErrPiggyContractUnsigned)
		assert.Nil(t, result)
	})

	t.Run("incomplete profile", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2171, 0)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		mockPiggyClientForTest(t, func(r *http.Request) string {
			t.Fatal("incomplete profile must not call Piggy preview")
			return ""
		})

		result, err := GetPiggyContractPreviewURL(context.Background(), 2171)

		require.ErrorIs(t, err, ErrWithdrawalProfileIncomplete)
		assert.Nil(t, result)
	})

	t.Run("signed but scope changed", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2172, 0)
		seedSignedPiggyProfile(t, 2172)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		require.NoError(t, model.DB.Model(&model.WithdrawalProfile{}).Where("user_id = ?", 2172).Updates(map[string]interface{}{
			"piggy_contract_document_id":   "DOC-2172",
			"piggy_contract_position":      "历史服务",
			"piggy_contract_position_name": "历史服务",
			"piggy_contract_tax_fund_id":   "old-tax-fund",
		}).Error)
		mockPiggyClientForTest(t, func(r *http.Request) string {
			t.Fatal("scope changed contract must not call Piggy preview")
			return ""
		})

		result, err := GetPiggyContractPreviewURL(context.Background(), 2172)

		require.ErrorIs(t, err, ErrPiggyContractUnsigned)
		assert.Nil(t, result)
	})

	t.Run("signed result still has no document id", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2173, 0)
		seedSignedPiggyProfile(t, 2173)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		var paths []string
		mockPiggyClientForTest(t, func(r *http.Request) string {
			paths = append(paths, r.URL.Path)
			assert.Equal(t, "/contract/sign/getSignedResult", r.URL.Path)
			return `{"code":0,"msg":"success","data":[{"name":"张三","idCardNo":"110101199001011234","mobile":"13812345678","position":"技术服务"}]}`
		})

		result, err := GetPiggyContractPreviewURL(context.Background(), 2173)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "合同编号为空")
		assert.Equal(t, []string{"/contract/sign/getSignedResult"}, paths)
	})
}

func TestPiggyOfficialContractCallbackWithoutSignatureRequiresStrongUserMatch(t *testing.T) {
	truncate(t)
	seedUser(t, 2151, 0)
	seedVerifiedWithdrawalPhone(t, 2151, "+8613812345678")
	_, err := SaveWithdrawalProfile(2151, WithdrawalProfileInput{
		AccountType: model.WithdrawAccountTypeBankcard,
		RealName:    "张三",
		IdCardNo:    "110101199001011234",
		Mobile:      "13812345678",
		BankCardNo:  "6222000011118888",
		BankName:    "招商银行",
	})
	require.NoError(t, err)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")

	body := buildOfficialPiggyContractCallback(t, map[string]any{"userId": 2151}, nil)
	require.NoError(t, HandlePiggyContractCallback(body, ""))

	profile, err := getRawWithdrawalProfile(2151)
	require.NoError(t, err)
	assert.Equal(t, model.PiggySignStatusSigned, profile.PiggySignStatus)
	assert.NotZero(t, profile.PiggySignedAt)
	assert.Equal(t, "DOC-2102", profile.PiggyContractDocumentID)
	assert.Equal(t, "https://uat.xzsz.ltd/contract/official-2102", profile.PiggyContractURL)
	assert.Equal(t, "小猪签约结算公司", profile.PiggyContractSubsidiaryName)
	assert.Equal(t, "技术服务", profile.PiggyContractPosition)
	assert.Equal(t, "技术服务", profile.PiggyContractPositionName)
	assert.Equal(t, "tax-fund", profile.PiggyContractTaxFundID)
}

func TestPiggyOfficialContractCallbackWithoutSignatureAcceptsStringCustomParams(t *testing.T) {
	truncate(t)
	seedUser(t, 2158, 0)
	seedVerifiedWithdrawalPhone(t, 2158, "+8613812345678")
	_, err := SaveWithdrawalProfile(2158, WithdrawalProfileInput{
		AccountType: model.WithdrawAccountTypeBankcard,
		RealName:    "张三",
		IdCardNo:    "110101199001011234",
		Mobile:      "13812345678",
		BankCardNo:  "6222000011118888",
		BankName:    "招商银行",
	})
	require.NoError(t, err)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")

	body := buildOfficialPiggyContractCallback(t, `{"userId":2158}`, nil)
	require.NoError(t, HandlePiggyContractCallback(body, ""))

	profile, err := getRawWithdrawalProfile(2158)
	require.NoError(t, err)
	assert.Equal(t, model.PiggySignStatusSigned, profile.PiggySignStatus)
	assert.Equal(t, "DOC-2102", profile.PiggyContractDocumentID)
	assert.Equal(t, "https://uat.xzsz.ltd/contract/official-2102", profile.PiggyContractURL)
}

func TestPiggyOfficialContractCallbackWithoutSignatureRejectsWeakMatch(t *testing.T) {
	t.Run("missing user id does not fallback to identity", func(t *testing.T) {
		truncate(t)
		require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
			UserId:          2152,
			AccountType:     model.WithdrawAccountTypeBankcard,
			RealName:        "张三",
			IdCardNo:        "110101199001011234",
			Mobile:          "13812345678",
			BankCardNo:      "6222000011118888",
			BankName:        "招商银行",
			PiggySignStatus: model.PiggySignStatusUnsigned,
		}).Error)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")

		err := HandlePiggyContractCallback(buildOfficialPiggyContractCallback(t, nil, nil), "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "userId")
		profile, err := getRawWithdrawalProfile(2152)
		require.NoError(t, err)
		assert.Equal(t, model.PiggySignStatusUnsigned, profile.PiggySignStatus)
	})

	t.Run("mismatched profile identity is rejected", func(t *testing.T) {
		truncate(t)
		require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
			UserId:          2153,
			AccountType:     model.WithdrawAccountTypeBankcard,
			RealName:        "张三",
			IdCardNo:        "110101199001011234",
			Mobile:          "13912345678",
			BankCardNo:      "6222000011118888",
			BankName:        "招商银行",
			PiggySignStatus: model.PiggySignStatusUnsigned,
		}).Error)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")

		err := HandlePiggyContractCallback(buildOfficialPiggyContractCallback(t, `{"userId":2153}`, nil), "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "身份")
		profile, err := getRawWithdrawalProfile(2153)
		require.NoError(t, err)
		assert.Equal(t, model.PiggySignStatusUnsigned, profile.PiggySignStatus)
	})

	t.Run("missing explicit success status is rejected", func(t *testing.T) {
		truncate(t)
		require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
			UserId:          2154,
			AccountType:     model.WithdrawAccountTypeBankcard,
			RealName:        "张三",
			IdCardNo:        "110101199001011234",
			Mobile:          "13812345678",
			BankCardNo:      "6222000011118888",
			BankName:        "招商银行",
			PiggySignStatus: model.PiggySignStatusUnsigned,
		}).Error)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		body, err := common.Marshal(map[string]any{
			"msg": "合同签署成功",
			"data": map[string]any{
				"name":        "张三",
				"idCardNo":    "110101199001011234",
				"mobile":      "13812345678",
				"bankAccount": "6222000011118888",
				"customParams": map[string]any{
					"userId": 2154,
				},
			},
		})
		require.NoError(t, err)

		err = HandlePiggyContractCallback(body, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "签约状态")
		profile, err := getRawWithdrawalProfile(2154)
		require.NoError(t, err)
		assert.Equal(t, model.PiggySignStatusUnsigned, profile.PiggySignStatus)
		var log model.PiggyWithdrawCallbackLog
		require.NoError(t, model.DB.Order("id desc").First(&log).Error)
		assert.Equal(t, model.PaymentProcessStatusFailed, log.ProcessStatus)
	})

	t.Run("unsigned success requires official top-level code zero", func(t *testing.T) {
		truncate(t)
		require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
			UserId:          2156,
			AccountType:     model.WithdrawAccountTypeBankcard,
			RealName:        "张三",
			IdCardNo:        "110101199001011234",
			Mobile:          "13812345678",
			BankCardNo:      "6222000011118888",
			BankName:        "招商银行",
			PiggySignStatus: model.PiggySignStatusUnsigned,
		}).Error)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		body, err := common.Marshal(map[string]any{
			"msg": "合同签署成功",
			"data": map[string]any{
				"name":        "张三",
				"idCardNo":    "110101199001011234",
				"mobile":      "13812345678",
				"bankAccount": "6222000011118888",
				"signStatus":  "success",
				"customParams": map[string]any{
					"userId": 2156,
				},
			},
		})
		require.NoError(t, err)

		err = HandlePiggyContractCallback(body, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "签约状态")
		profile, err := getRawWithdrawalProfile(2156)
		require.NoError(t, err)
		assert.Equal(t, model.PiggySignStatusUnsigned, profile.PiggySignStatus)
	})

	t.Run("unsigned failure status does not downgrade signed profile", func(t *testing.T) {
		truncate(t)
		require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
			UserId:          2155,
			AccountType:     model.WithdrawAccountTypeBankcard,
			RealName:        "张三",
			IdCardNo:        "110101199001011234",
			Mobile:          "13812345678",
			BankCardNo:      "6222000011118888",
			BankName:        "招商银行",
			PiggySignStatus: model.PiggySignStatusSigned,
			PiggySignedAt:   common.GetTimestamp(),
		}).Error)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")

		body, err := common.Marshal(map[string]any{
			"code": "1",
			"msg":  "签约失败",
			"data": map[string]any{
				"name":        "张三",
				"idCardNo":    "110101199001011234",
				"mobile":      "13812345678",
				"bankAccount": "6222000011118888",
				"customParams": map[string]any{
					"userId": 2155,
				},
			},
		})
		require.NoError(t, err)

		err = HandlePiggyContractCallback(body, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "签约状态")
		profile, err := getRawWithdrawalProfile(2155)
		require.NoError(t, err)
		assert.Equal(t, model.PiggySignStatusSigned, profile.PiggySignStatus)
		var log model.PiggyWithdrawCallbackLog
		require.NoError(t, model.DB.Order("id desc").First(&log).Error)
		assert.Equal(t, model.PaymentProcessStatusFailed, log.ProcessStatus)
	})
}

func TestPiggyContractCallbackAcceptsStringCustomParams(t *testing.T) {
	truncate(t)
	seedUser(t, 2136, 0)
	require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
		UserId:          2136,
		AccountType:     model.WithdrawAccountTypeBankcard,
		RealName:        "张三",
		IdCardNo:        "110101199001011236",
		Mobile:          "13812345678",
		BankCardNo:      "6222000011118888",
		BankName:        "招商银行",
		PiggySignStatus: model.PiggySignStatusUnsigned,
	}).Error)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")

	body := buildSignedPiggyContractCallbackWithCustomParams(t, `{"userId":2136}`, "110101199001011236")
	require.NoError(t, HandlePiggyContractCallback(body, ""))

	profile, err := getRawWithdrawalProfile(2136)
	require.NoError(t, err)
	assert.Equal(t, model.PiggySignStatusSigned, profile.PiggySignStatus)
}

func TestPiggyContractCallbackFallsBackToIdCardWhenStringCustomParamsInvalid(t *testing.T) {
	truncate(t)
	seedUser(t, 2137, 0)
	require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
		UserId:          2137,
		AccountType:     model.WithdrawAccountTypeBankcard,
		RealName:        "张三",
		IdCardNo:        "110101199001011237",
		Mobile:          "13812345678",
		BankCardNo:      "6222000011118888",
		BankName:        "招商银行",
		PiggySignStatus: model.PiggySignStatusUnsigned,
	}).Error)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")

	body := buildSignedPiggyContractCallbackWithCustomParams(t, `{"userId":`, "110101199001011237")
	require.NoError(t, HandlePiggyContractCallback(body, ""))

	profile, err := getRawWithdrawalProfile(2137)
	require.NoError(t, err)
	assert.Equal(t, model.PiggySignStatusSigned, profile.PiggySignStatus)
}

func TestPiggyContractCallbackFallbackRequiresUniqueIdentityMatch(t *testing.T) {
	t.Run("duplicate identity is rejected", func(t *testing.T) {
		truncate(t)
		require.NoError(t, model.DB.Create(&[]model.WithdrawalProfile{
			{
				UserId:          2144,
				AccountType:     model.WithdrawAccountTypeBankcard,
				RealName:        "张三",
				IdCardNo:        "110101199001011244",
				Mobile:          "13812345678",
				BankCardNo:      "6222000011118888",
				BankName:        "招商银行",
				PiggySignStatus: model.PiggySignStatusUnsigned,
			},
			{
				UserId:          2145,
				AccountType:     model.WithdrawAccountTypeBankcard,
				RealName:        "张三",
				IdCardNo:        "110101199001011244",
				Mobile:          "13812345678",
				BankCardNo:      "6222000011119999",
				BankName:        "招商银行",
				PiggySignStatus: model.PiggySignStatusUnsigned,
			},
		}).Error)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")

		body := buildSignedPiggyContractCallbackWithCustomParams(t, `{"userId":`, "110101199001011244")
		require.Error(t, HandlePiggyContractCallback(body, ""))

		var signedCount int64
		require.NoError(t, model.DB.Model(&model.WithdrawalProfile{}).
			Where("user_id IN ? AND piggy_sign_status = ?", []int{2144, 2145}, model.PiggySignStatusSigned).
			Count(&signedCount).Error)
		assert.Equal(t, int64(0), signedCount)

		var log model.PiggyWithdrawCallbackLog
		require.NoError(t, model.DB.Order("id desc").First(&log).Error)
		assert.Equal(t, model.PaymentProcessStatusFailed, log.ProcessStatus)
	})

	t.Run("mismatched mobile is rejected", func(t *testing.T) {
		truncate(t)
		require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
			UserId:          2146,
			AccountType:     model.WithdrawAccountTypeBankcard,
			RealName:        "张三",
			IdCardNo:        "110101199001011246",
			Mobile:          "13912345678",
			BankCardNo:      "6222000011118888",
			BankName:        "招商银行",
			PiggySignStatus: model.PiggySignStatusUnsigned,
		}).Error)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")

		body := buildSignedPiggyContractCallbackWithCustomParams(t, `{"userId":`, "110101199001011246")
		require.Error(t, HandlePiggyContractCallback(body, ""))

		profile, err := getRawWithdrawalProfile(2146)
		require.NoError(t, err)
		assert.Equal(t, model.PiggySignStatusUnsigned, profile.PiggySignStatus)

		var log model.PiggyWithdrawCallbackLog
		require.NoError(t, model.DB.Order("id desc").First(&log).Error)
		assert.Equal(t, model.PaymentProcessStatusFailed, log.ProcessStatus)
	})
}

func TestPiggyContractCallbackRejectsEmptyAppSecret(t *testing.T) {
	truncate(t)
	seedUser(t, 2135, 0)
	require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
		UserId:          2135,
		AccountType:     model.WithdrawAccountTypeBankcard,
		RealName:        "张三",
		IdCardNo:        "110101199001011234",
		Mobile:          "13812345678",
		BankCardNo:      "6222000011118888",
		BankName:        "招商银行",
		PiggySignStatus: model.PiggySignStatusUnsigned,
	}).Error)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().AppSecret = ""
	payload := map[string]any{
		"userName":   "张三",
		"idCardNo":   "110101199001011234",
		"mobile":     "13812345678",
		"signStatus": "success",
		"customParams": map[string]any{
			"userId": 2135,
		},
	}
	sign, err := piggySignJSON("", payload)
	require.NoError(t, err)
	payload["sign"] = sign
	body, err := common.Marshal(payload)
	require.NoError(t, err)

	require.Error(t, HandlePiggyContractCallback(body, ""))

	profile, err := getRawWithdrawalProfile(2135)
	require.NoError(t, err)
	assert.Equal(t, model.PiggySignStatusUnsigned, profile.PiggySignStatus)

	var log model.PiggyWithdrawCallbackLog
	require.NoError(t, model.DB.Order("id desc").First(&log).Error)
	assert.Equal(t, model.PaymentProcessStatusFailed, log.ProcessStatus)
}

func TestSubmitPiggyWithdrawUsesCommissionOnlyAndStoresTaxBeforeAmount(t *testing.T) {
	truncate(t)
	seedUser(t, 2103, 0)
	seedWalletAccount(t, 2103, 100, 0, 0)
	seedSignedPiggyProfile(t, 2103)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		assert.Equal(t, "/open/payment/singleOrderSubmit", r.URL.Path)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	_, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2103, Amount: 10})
	require.ErrorIs(t, err, ErrCommissionInsufficient)

	require.NoError(t, model.DB.Model(&model.WalletAccount{}).Where("user_id = ?", 2103).Updates(map[string]interface{}{
		"commission_amount": 25.0,
	}).Error)
	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2103, Amount: 10})
	require.NoError(t, err)
	assert.Equal(t, model.WithdrawProviderPiggyLaborV3, order.Provider)
	assert.Equal(t, int64(1000), order.TaxBeforeAmountCents)
	assert.Equal(t, "10.00", order.PiggyPayAmount)

	account, err := model.GetWalletAccountByUserId(2103)
	require.NoError(t, err)
	assert.InDelta(t, 100.0, account.BalanceAmount, 0.000001)
	assert.InDelta(t, 15.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
}

func TestTrialPiggyWithdrawTaxAppliesPlatformFee(t *testing.T) {
	truncate(t)
	seedUser(t, 2306, 0)
	seedWalletAccount(t, 2306, 0, 120, 0)
	seedSignedPiggyProfile(t, 2306)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8
	var capturedCalcAmount string
	mockPiggyClientForTest(t, func(r *http.Request) string {
		assert.Equal(t, "/open/payment/singleTaxTrialCalc", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, common.Unmarshal(body, &payload))
		capturedCalcAmount = payload["calcAmount"].(string)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{"pretaxAmount":92,"individualTaxAmount":3.5,"addedTaxAmount":1.06,"afterTaxAmount":87.44}}`
	})

	result, err := TrialPiggyWithdrawTax(context.Background(), PiggyTaxTrialRequest{
		UserId: 2306,
		Amount: 100,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "92.00", capturedCalcAmount)
	assert.Equal(t, "100.00", result.RequestedAmount)
	assert.Equal(t, int64(10000), result.RequestedAmountCents)
	assert.Equal(t, 8.0, result.PlatformFeeRate)
	assert.Equal(t, "8.00", result.PlatformFeeAmount)
	assert.Equal(t, int64(800), result.PlatformFeeAmountCents)
	assert.Equal(t, "92.00", result.PiggyTaxBeforeAmount)
	assert.Equal(t, int64(9200), result.PiggyTaxBeforeAmountCents)
	assert.Equal(t, "92.00", result.PretaxAmount)
	assert.Equal(t, "87.44", result.AfterTaxAmount)
}

func TestSubmitPiggyWithdrawStoresPlatformFeeSnapshot(t *testing.T) {
	truncate(t)
	seedUser(t, 2307, 0)
	seedWalletAccount(t, 2307, 0, 120, 0)
	seedSignedPiggyProfile(t, 2307)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2307, Amount: 100})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, 100.0, order.Amount)
	assert.Equal(t, int64(10000), order.FrozenAmountCents)
	assert.Equal(t, 8.0, order.PlatformFeeRate)
	assert.Equal(t, int64(800), order.PlatformFeeAmountCents)
	assert.Equal(t, int64(9200), order.TaxBeforeAmountCents)
	assert.Equal(t, int64(9200), order.PiggyPayAmountCents)
	assert.Equal(t, "92.00", order.PiggyPayAmount)
	assert.Zero(t, order.FeeAmount)
	assert.Zero(t, order.PiggyFeeAmountCents)

	account, err := model.GetWalletAccountByUserId(2307)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 100.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 2307, model.WalletFlowTypeWithdrawFreeze, order.WithdrawNo))
	assert.Equal(t, int64(0), countWalletFlows(t, 2307, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	assert.Equal(t, int64(0), countWalletFlows(t, 2307, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
}

func TestAdminApprovePiggyWithdrawSubmitsPostPlatformFeeAmount(t *testing.T) {
	truncate(t)
	seedUser(t, 2308, 0)
	seedWalletAccount(t, 2308, 0, 120, 0)
	seedSignedPiggyProfile(t, 2308)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8
	var captured PiggySubmitOrderRequest
	mockPiggyClientForTest(t, func(r *http.Request) string {
		assert.Equal(t, "/open/payment/singleOrderSubmit", r.URL.Path)
		captured = parsePiggySubmitOrderRequestForTest(t, r)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2308, Amount: 100})
	require.NoError(t, err)
	require.NoError(t, AdminApproveWithdrawOrder(context.Background(), order.Id, 7, "approved"))

	assert.Equal(t, order.WithdrawNo, captured.OuterTradeNo)
	assert.Equal(t, "92.00", captured.PayAmount)
}

func TestPiggyPaymentCallbackValidatesPostPlatformFeePretaxAmount(t *testing.T) {
	t.Run("accepts post fee pretax amount", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2309, 0)
		seedWalletAccount(t, 2309, 0, 120, 0)
		seedSignedPiggyProfile(t, 2309)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8
		mockPiggyClientForTest(t, func(r *http.Request) string {
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2309, Amount: 100})
		require.NoError(t, err)
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()
		content := piggyCallbackContentFromOrderForTest(order, "tradeResult", "success")
		content.PretaxAmount = "92.00"
		content.IndividualTaxAmount = "2.00"
		content.AfterTaxAmount = "90.00"

		require.NoError(t, HandlePiggyPaymentCallback(context.Background(), buildSignedPiggyPaymentCallback(t, content), ""))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
		assert.Equal(t, int64(9200), refreshed.PiggyPretaxAmountCents)
		assert.Equal(t, int64(0), refreshed.PiggyFeeAmountCents)
	})

	t.Run("moves full requested amount pretax to manual review", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2310, 0)
		seedWalletAccount(t, 2310, 0, 120, 0)
		seedSignedPiggyProfile(t, 2310)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8
		mockPiggyClientForTest(t, func(r *http.Request) string {
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2310, Amount: 100})
		require.NoError(t, err)
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()
		content := piggyCallbackContentFromOrderForTest(order, "tradeResult", "success")
		content.PretaxAmount = "100.00"

		require.NoError(t, HandlePiggyPaymentCallback(context.Background(), buildSignedPiggyPaymentCallback(t, content), ""))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
		assert.Contains(t, refreshed.ManualReviewReason, "金额")

		account, err := model.GetWalletAccountByUserId(2310)
		require.NoError(t, err)
		assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
		assert.InDelta(t, 100.0, account.FrozenCommissionAmount, 0.000001)
		assert.Equal(t, int64(0), countWalletFlows(t, 2310, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	})
}

func TestPiggyQueryValidationUsesPostPlatformFeePretaxAmount(t *testing.T) {
	truncate(t)
	seedUser(t, 2315, 0)
	seedWalletAccount(t, 2315, 0, 120, 0)
	seedSignedPiggyProfile(t, 2315)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2315, Amount: 100})
	require.NoError(t, err)

	assert.NoError(t, validatePiggyQueryCallbackContent(order.WithdrawNo, PiggyPaymentCallbackContent{
		OuterTradeNo: order.WithdrawNo,
		PretaxAmount: "92.00",
	}))
	assert.Error(t, validatePiggyQueryCallbackContent(order.WithdrawNo, PiggyPaymentCallbackContent{
		OuterTradeNo: order.WithdrawNo,
		PretaxAmount: "100.00",
	}))
}

func TestPiggyPlatformFeeAccountingUsesFullRequestedAmount(t *testing.T) {
	t.Run("admin reject releases full requested amount", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2312, 0)
		seedWalletAccount(t, 2312, 0, 120, 0)
		seedSignedPiggyProfile(t, 2312)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2312, Amount: 100})
		require.NoError(t, err)
		require.NoError(t, AdminRejectWithdrawOrder(context.Background(), order.Id, 7, "reject"))

		account, err := model.GetWalletAccountByUserId(2312)
		require.NoError(t, err)
		assert.InDelta(t, 120.0, account.CommissionAmount, 0.000001)
		assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
		assert.Equal(t, int64(1), countWalletFlows(t, 2312, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
	})

	t.Run("paid callback deducts full requested amount", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2313, 0)
		seedWalletAccount(t, 2313, 0, 120, 0)
		seedSignedPiggyProfile(t, 2313)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8
		mockPiggyClientForTest(t, func(r *http.Request) string {
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2313, Amount: 100})
		require.NoError(t, err)
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()
		content := piggyCallbackContentFromOrderForTest(order, "tradeResult", "success")
		content.IndividualTaxAmount = "2.00"
		content.AfterTaxAmount = "90.00"
		require.NoError(t, HandlePiggyPaymentCallback(context.Background(), buildSignedPiggyPaymentCallback(t, content), ""))

		account, err := model.GetWalletAccountByUserId(2313)
		require.NoError(t, err)
		assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
		assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
		assert.InDelta(t, 100.0, account.TotalWithdrawAmount, 0.000001)
		assert.Equal(t, int64(1), countWalletFlows(t, 2313, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	})

	t.Run("manual review keeps full requested amount frozen", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2314, 0)
		seedWalletAccount(t, 2314, 0, 120, 0)
		seedSignedPiggyProfile(t, 2314)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8
		mockPiggyClientForTest(t, func(r *http.Request) string {
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2314, Amount: 100})
		require.NoError(t, err)
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()
		content := piggyCallbackContentFromOrderForTest(order, "tradeResult", "success")
		content.PretaxAmount = "100.00"
		require.NoError(t, HandlePiggyPaymentCallback(context.Background(), buildSignedPiggyPaymentCallback(t, content), ""))

		account, err := model.GetWalletAccountByUserId(2314)
		require.NoError(t, err)
		assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
		assert.InDelta(t, 100.0, account.FrozenCommissionAmount, 0.000001)
		assert.Equal(t, int64(0), countWalletFlows(t, 2314, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	})
}

func TestPiggyLegacyOrderCallbackUsesLegacyPretaxAmount(t *testing.T) {
	truncate(t)
	seedUser(t, 2311, 0)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8
	legacy := &model.WithdrawOrder{
		UserId:              2311,
		WithdrawNo:          "PWDR2311LEGACY",
		Amount:              100,
		Status:              model.WithdrawStatusSubmitted,
		Provider:            model.WithdrawProviderPiggyLaborV3,
		AccountName:         "张三",
		BankName:            "招商银行",
		PayoutMobile:        "13812345678",
		PayoutIdCardNo:      "110101199001011234",
		PayoutBankCardNo:    "6222000011118888",
		FrozenAmountCents:   10000,
		PiggyPayAmountCents: 0,
		CreatedAt:           common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(legacy).Error)

	assert.NoError(t, validatePiggyCallbackPretaxAmount(legacy, PiggyPaymentCallbackContent{PretaxAmount: "100.00"}))
	assert.Error(t, validatePiggyCallbackPretaxAmount(legacy, PiggyPaymentCallbackContent{PretaxAmount: "92.00"}))
	assert.NoError(t, validatePiggyQueryCallbackContent(legacy.WithdrawNo, PiggyPaymentCallbackContent{
		OuterTradeNo: legacy.WithdrawNo,
		PretaxAmount: "100.00",
	}))
	assert.Error(t, validatePiggyQueryCallbackContent(legacy.WithdrawNo, PiggyPaymentCallbackContent{
		OuterTradeNo: legacy.WithdrawNo,
		PretaxAmount: "92.00",
	}))
}

func TestTrialPiggyWithdrawTaxRejectsNonWithdrawableAmountWithoutProviderCall(t *testing.T) {
	tests := []struct {
		name        string
		userId      int
		commission  float64
		amount      float64
		setup       func(t *testing.T, userId int)
		wantErrorIs error
		wantMessage string
	}{
		{
			name:       "below min amount",
			userId:     2301,
			commission: 100,
			amount:     10,
			setup: func(t *testing.T, userId int) {
				operation_setting.GetPaymentSetting().CommissionMinWithdrawAmount = 100
			},
			wantMessage: "提现金额不能小于 100.00",
		},
		{
			name:       "forbidden withdraw time",
			userId:     2303,
			commission: 100,
			amount:     10,
			setup: func(t *testing.T, userId int) {
				operation_setting.GetPiggyWithdrawSetting().ForbiddenWithdrawTime = "00:00-23:59"
			},
			wantMessage: "当前时间禁止提现",
		},
		{
			name:       "cooldown",
			userId:     2304,
			commission: 100,
			amount:     10,
			setup: func(t *testing.T, userId int) {
				operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 60
				require.NoError(t, model.DB.Create(&model.WithdrawOrder{
					UserId:     userId,
					WithdrawNo: "PWDR2304COOLDOWN",
					Status:     model.WithdrawStatusPending,
					Provider:   model.WithdrawProviderPiggyLaborV3,
					CreatedAt:  common.GetTimestamp(),
				}).Error)
			},
			wantMessage: "提现冷却中",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncate(t)
			seedUser(t, tt.userId, 0)
			seedWalletAccount(t, tt.userId, 0, tt.commission, 0)
			seedSignedPiggyProfile(t, tt.userId)
			configurePiggyWithdrawForTest(t, "https://piggy.example.com")
			operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
			if tt.setup != nil {
				tt.setup(t, tt.userId)
			}
			var trialCalls int32
			mockPiggyClientForTest(t, func(r *http.Request) string {
				atomic.AddInt32(&trialCalls, 1)
				return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
			})

			result, err := TrialPiggyWithdrawTax(context.Background(), PiggyTaxTrialRequest{
				UserId: tt.userId,
				Amount: tt.amount,
			})

			require.Error(t, err)
			assert.Nil(t, result)
			if tt.wantErrorIs != nil {
				assert.ErrorIs(t, err, tt.wantErrorIs)
			}
			if tt.wantMessage != "" {
				assert.Contains(t, err.Error(), tt.wantMessage)
			}
			assert.Equal(t, int32(0), atomic.LoadInt32(&trialCalls))
		})
	}
}

func TestTrialPiggyWithdrawTaxAllowsAmountAboveAvailableCommission(t *testing.T) {
	truncate(t)
	seedUser(t, 2305, 0)
	seedWalletAccount(t, 2305, 0, 5, 0)
	seedSignedPiggyProfile(t, 2305)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().PlatformFeeRate = 8
	var capturedCalcAmount string
	mockPiggyClientForTest(t, func(r *http.Request) string {
		assert.Equal(t, "/open/payment/singleTaxTrialCalc", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, common.Unmarshal(body, &payload))
		capturedCalcAmount = payload["calcAmount"].(string)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{"pretaxAmount":460,"individualTaxAmount":0,"addedTaxAmount":0,"afterTaxAmount":460}}`
	})

	result, err := TrialPiggyWithdrawTax(context.Background(), PiggyTaxTrialRequest{
		UserId: 2305,
		Amount: 500,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "460.00", capturedCalcAmount)
	assert.Equal(t, "500.00", result.RequestedAmount)
	assert.Equal(t, "40.00", result.PlatformFeeAmount)
	assert.Equal(t, "460.00", result.PiggyTaxBeforeAmount)
	assert.Equal(t, "460.00", result.PretaxAmount)
	assert.Equal(t, "460.00", result.AfterTaxAmount)

	_, err = SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{
		UserId: 2305,
		Amount: 500,
	})
	assert.ErrorIs(t, err, ErrCommissionInsufficient)
}

func TestSubmitPiggyWithdrawCreatesLocalReviewOrderWithoutCallingPiggy(t *testing.T) {
	truncate(t)
	seedUser(t, 2151, 0)
	seedWalletAccount(t, 2151, 100, 30, 0)
	seedSignedPiggyProfile(t, 2151)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var submitCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		atomic.AddInt32(&submitCalls, 1)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2151, Amount: 10})
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, model.WithdrawProviderPiggyLaborV3, order.Provider)
	assert.Equal(t, model.WithdrawStatusPending, order.Status)
	assert.Equal(t, model.WithdrawStatusPending, order.PiggyStatus)
	assert.Empty(t, order.ExternalTradeNo)
	assert.Empty(t, order.RequestPayloadDigest)
	assert.Zero(t, order.SubmittedAt)
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))

	account, err := model.GetWalletAccountByUserId(2151)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 2151, model.WalletFlowTypeWithdrawFreeze, order.WithdrawNo))
}

func TestPiggyPaymentCallbackBeforeAdminApprovalMovesManualReviewWithoutSettlement(t *testing.T) {
	truncate(t)
	seedUser(t, 2154, 0)
	seedWalletAccount(t, 2154, 0, 30, 0)
	seedSignedPiggyProfile(t, 2154)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2154, Amount: 10})
	require.NoError(t, err)
	body := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo:        order.WithdrawNo,
		NotifyType:          "tradeResult",
		TradeStatus:         "success",
		FrontLogNo:          "front-before-review",
		LaborOrderNo:        "labor-before-review",
		PretaxAmount:        "10.00",
		AfterTaxAmount:      "9.10",
		IndividualTaxAmount: "0.90",
		FeeAmount:           "0.00",
		CalcType:            "C",
	})

	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.PiggyStatus)
	assert.Contains(t, refreshed.ManualReviewReason, "管理员审核")

	account, err := model.GetWalletAccountByUserId(2154)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(0), countWalletFlows(t, 2154, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
}

func TestAdminApprovePiggyWithdrawSubmitsOrderToPiggy(t *testing.T) {
	truncate(t)
	seedUser(t, 2152, 0)
	seedWalletAccount(t, 2152, 0, 30, 0)
	seedSignedPiggyProfile(t, 2152)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var submitCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		assert.Equal(t, "/open/payment/singleOrderSubmit", r.URL.Path)
		atomic.AddInt32(&submitCalls, 1)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2152, Amount: 10, Remark: "apply"})
	require.NoError(t, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))

	require.NoError(t, AdminApproveWithdrawOrder(context.Background(), order.Id, 7, "approved"))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusSubmitted, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusSubmitted, refreshed.PiggyStatus)
	assert.Equal(t, order.WithdrawNo, refreshed.ExternalTradeNo)
	assert.Equal(t, 7, refreshed.ReviewerId)
	assert.NotZero(t, refreshed.ReviewedAt)
	assert.NotZero(t, refreshed.SubmittedAt)
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)
	assert.NotEmpty(t, refreshed.ResponsePayloadDigest)
	assert.Equal(t, int32(1), atomic.LoadInt32(&submitCalls))

	account, err := model.GetWalletAccountByUserId(2152)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
}

func TestAdminApprovePiggyWithdrawRejectsDuplicateApprovalWhileSubmitting(t *testing.T) {
	truncate(t)
	seedUser(t, 2161, 0)
	seedWalletAccount(t, 2161, 0, 30, 0)
	seedSignedPiggyProfile(t, 2161)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	firstSubmitStarted := make(chan struct{})
	releaseFirstSubmit := make(chan struct{})
	var submitCalls int32
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open/payment/singleOrderSubmit" {
			if atomic.AddInt32(&submitCalls, 1) == 1 {
				close(firstSubmitStarted)
				<-releaseFirstSubmit
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2161, Amount: 10})
	require.NoError(t, err)
	approveDone := make(chan error, 1)
	go func() {
		approveDone <- AdminApproveWithdrawOrder(context.Background(), order.Id, 7, "first approve")
	}()
	waitForPiggyConfirmAttempt(t, firstSubmitStarted)

	err = AdminApproveWithdrawOrder(context.Background(), order.Id, 8, "duplicate approve")
	assert.ErrorIs(t, err, ErrWithdrawStatusInvalid)
	close(releaseFirstSubmit)
	require.NoError(t, <-approveDone)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusSubmitted, refreshed.Status)
	assert.Equal(t, int32(1), atomic.LoadInt32(&submitCalls))
}

func TestAdminRejectPiggyWithdrawWhileSubmitInFlightIsRejected(t *testing.T) {
	truncate(t)
	seedUser(t, 2162, 0)
	seedWalletAccount(t, 2162, 0, 30, 0)
	seedSignedPiggyProfile(t, 2162)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	firstSubmitStarted := make(chan struct{})
	releaseFirstSubmit := make(chan struct{})
	var submitCalls int32
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open/payment/singleOrderSubmit" {
			atomic.AddInt32(&submitCalls, 1)
			close(firstSubmitStarted)
			<-releaseFirstSubmit
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2162, Amount: 10})
	require.NoError(t, err)
	approveDone := make(chan error, 1)
	go func() {
		approveDone <- AdminApproveWithdrawOrder(context.Background(), order.Id, 7, "approved")
	}()
	waitForPiggyConfirmAttempt(t, firstSubmitStarted)

	err = AdminRejectWithdrawOrder(context.Background(), order.Id, 8, "reject while submitting")
	assert.ErrorIs(t, err, ErrWithdrawStatusInvalid)
	close(releaseFirstSubmit)
	require.NoError(t, <-approveDone)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusSubmitted, refreshed.Status)
	assert.Equal(t, int32(1), atomic.LoadInt32(&submitCalls))

	account, err := model.GetWalletAccountByUserId(2162)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
}

func TestAdminApprovePiggyWithdrawUsesImmutableOrderPayoutSnapshot(t *testing.T) {
	truncate(t)
	seedUser(t, 2163, 0)
	seedWalletAccount(t, 2163, 0, 30, 0)
	seedSignedPiggyProfile(t, 2163)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var captured PiggySubmitOrderRequest
	mockPiggyClientForTest(t, func(r *http.Request) string {
		captured = parsePiggySubmitOrderRequestForTest(t, r)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2163, Amount: 10})
	require.NoError(t, err)
	_, err = SaveWithdrawalProfile(2163, WithdrawalProfileInput{
		AccountType: model.WithdrawAccountTypeBankcard,
		RealName:    "李四",
		IdCardNo:    "110101199001019999",
		Mobile:      "13912345678",
		BankCardNo:  "6222000099998888",
		BankName:    "建设银行",
	})
	require.NoError(t, err)

	require.NoError(t, AdminApproveWithdrawOrder(context.Background(), order.Id, 7, "approved"))

	assert.Equal(t, "张三", captured.EmpName)
	assert.Equal(t, "13812345678", captured.EmpPhone)
	assert.Equal(t, "110101199001011234", captured.LicenseId)
	assert.Equal(t, "6222000011118888", captured.PayAccount)
	assert.Equal(t, "招商银行", captured.BankName)
	assert.Equal(t, order.WithdrawNo, captured.OuterTradeNo)
}

func TestAdminRecoverPiggyApprovedSubmissionKeepsApprovedWhenQueryCannotProveStatus(t *testing.T) {
	truncate(t)
	seedUser(t, 2164, 0)
	seedWalletAccount(t, 2164, 0, 30, 0)
	seedSignedPiggyProfile(t, 2164)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 5
	var submitCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderSubmit" {
			atomic.AddInt32(&submitCalls, 1)
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2164, Amount: 10})
	require.NoError(t, err)
	staleAt := common.GetTimestamp() - 120
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusApproved,
		"piggy_status": model.WithdrawStatusApproved,
		"reviewed_at":  staleAt,
		"updated_at":   staleAt,
	}).Error)

	require.NoError(t, AdminRecoverPiggyApprovedSubmission(context.Background(), order.Id))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.PiggyStatus)
	assert.Empty(t, refreshed.ExternalTradeNo)
	assert.Contains(t, refreshed.ManualReviewReason, "无法判定")
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))
}

func TestAdminRecoverPiggyApprovedSubmissionKeepsRecoveringOrderApprovedWhenQueryCannotProveStatus(t *testing.T) {
	truncate(t)
	seedUser(t, 2166, 0)
	seedWalletAccount(t, 2166, 0, 30, 0)
	seedSignedPiggyProfile(t, 2166)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 5
	var submitCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderSubmit" {
			atomic.AddInt32(&submitCalls, 1)
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2166, Amount: 10})
	require.NoError(t, err)
	staleAt := common.GetTimestamp() - 120
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":              model.WithdrawStatusApproved,
		"piggy_status":        model.WithdrawStatusApproved,
		"reviewed_at":         staleAt,
		"updated_at":          staleAt,
		"compensation_status": piggyCompensationStatusSubmitRecovering,
	}).Error)

	require.NoError(t, AdminRecoverPiggyApprovedSubmission(context.Background(), order.Id))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.PiggyStatus)
	assert.Empty(t, refreshed.ExternalTradeNo)
	assert.Contains(t, refreshed.ManualReviewReason, "无法判定")
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))
}

func TestAdminRecoverPiggyApprovedSubmissionUnknownResultRequiresDelayBeforeRetry(t *testing.T) {
	truncate(t)
	seedUser(t, 2167, 0)
	seedWalletAccount(t, 2167, 0, 30, 0)
	seedSignedPiggyProfile(t, 2167)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 5
	var submitCalls int32
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open/payment/singleOrderSubmit" {
			call := atomic.AddInt32(&submitCalls, 1)
			if call == 1 {
				return nil, errors.New("dial tcp: no route to host")
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2167, Amount: 10})
	require.NoError(t, err)
	staleAt := common.GetTimestamp() - 120
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":              model.WithdrawStatusApproved,
		"piggy_status":        model.WithdrawStatusApproved,
		"reviewed_at":         staleAt,
		"updated_at":          staleAt,
		"compensation_status": piggyCompensationStatusPending,
	}).Error)

	require.NoError(t, AdminRecoverPiggyApprovedSubmission(context.Background(), order.Id))
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))
	var unknownResult model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&unknownResult).Error)
	assert.Equal(t, model.WithdrawStatusApproved, unknownResult.Status)
	assert.Contains(t, unknownResult.ManualReviewReason, "小猪订单提交结果未知")

	err = AdminRecoverPiggyApprovedSubmission(context.Background(), order.Id)
	require.ErrorIs(t, err, ErrWithdrawStatusInvalid)
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))

	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"updated_at": staleAt,
	}).Error)
	require.NoError(t, AdminRecoverPiggyApprovedSubmission(context.Background(), order.Id))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.Status)
	assert.Empty(t, refreshed.ExternalTradeNo)
	assert.Contains(t, refreshed.ManualReviewReason, "无法判定")
	assert.Empty(t, refreshed.FailReason)
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))
}

func TestAdminRecoverPiggyApprovedSubmissionRejectsFreshApprovedOrder(t *testing.T) {
	truncate(t)
	seedUser(t, 2165, 0)
	seedWalletAccount(t, 2165, 0, 30, 0)
	seedSignedPiggyProfile(t, 2165)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 5
	var submitCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		atomic.AddInt32(&submitCalls, 1)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2165, Amount: 10})
	require.NoError(t, err)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusApproved,
		"piggy_status": model.WithdrawStatusApproved,
		"reviewed_at":  now,
		"updated_at":   now,
	}).Error)

	err = AdminRecoverPiggyApprovedSubmission(context.Background(), order.Id)

	assert.ErrorIs(t, err, ErrWithdrawStatusInvalid)
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))
}

func TestAdminRecoverPiggyApprovedSubmissionRejectsPendingOrder(t *testing.T) {
	truncate(t)
	seedUser(t, 2169, 0)
	seedWalletAccount(t, 2169, 0, 30, 0)
	seedSignedPiggyProfile(t, 2169)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 5
	var submitCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		atomic.AddInt32(&submitCalls, 1)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2169, Amount: 10})
	require.NoError(t, err)
	require.NotNil(t, order)

	result, err := AdminRecoverPiggyApprovedSubmissionWithResult(context.Background(), order.Id)

	require.ErrorIs(t, err, ErrWithdrawStatusInvalid)
	assert.Nil(t, result)
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))
}

func TestAdminRecoverPiggyApprovedSubmissionReturnsCurrentResultWhenCallbackAlreadyPaid(t *testing.T) {
	truncate(t)
	seedUser(t, 2168, 0)
	seedWalletAccount(t, 2168, 0, 30, 0)
	seedSignedPiggyProfile(t, 2168)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 5
	var submitCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		atomic.AddInt32(&submitCalls, 1)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2168, Amount: 10})
	require.NoError(t, err)
	staleAt := common.GetTimestamp() - 120
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":            model.WithdrawStatusPaid,
		"piggy_status":      model.WithdrawStatusPaid,
		"external_trade_no": order.WithdrawNo,
		"reviewed_at":       staleAt,
		"submitted_at":      staleAt,
		"terminal_at":       staleAt,
		"paid_at":           staleAt,
		"updated_at":        staleAt,
	}).Error)

	result, err := AdminRecoverPiggyApprovedSubmissionWithResult(context.Background(), order.Id)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Submitted)
	assert.False(t, result.Recoverable)
	assert.Equal(t, model.WithdrawStatusPaid, result.Status)
	assert.Equal(t, "小猪提现已支付", result.Message)
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))
}

func TestAdminRecoverPiggyApprovedSubmissionStopsWhenQueryFails(t *testing.T) {
	truncate(t)
	seedUser(t, 2182, 0)
	seedWalletAccount(t, 2182, 0, 30, 0)
	seedSignedPiggyProfile(t, 2182)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 1
	var submitCalls int32
	var queryCalls int32
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/open/payment/singleOrderQuery":
			atomic.AddInt32(&queryCalls, 1)
			return nil, context.DeadlineExceeded
		case "/open/payment/singleOrderSubmit":
			atomic.AddInt32(&submitCalls, 1)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2182, Amount: 10})
	require.NoError(t, err)
	staleAt := common.GetTimestamp() - 120
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":               model.WithdrawStatusApproved,
		"piggy_status":         model.WithdrawStatusApproved,
		"reviewed_at":          staleAt,
		"updated_at":           staleAt,
		"compensation_status":  piggyCompensationStatusPending,
		"manual_review_reason": "小猪提交结果未知，可使用原流水号恢复提交",
	}).Error)

	result, err := AdminRecoverPiggyApprovedSubmissionWithResult(context.Background(), order.Id)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Submitted)
	assert.True(t, result.Recoverable)
	assert.Equal(t, model.WithdrawStatusApproved, result.Status)
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.Status)
	assert.Contains(t, refreshed.ManualReviewReason, "查询失败")
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)
	assert.Empty(t, refreshed.ResponsePayloadDigest)
}

func TestAdminRecoverPiggyApprovedSubmissionClearsStaleSubmitResponseDigestWhenQueryFails(t *testing.T) {
	truncate(t)
	seedUser(t, 2194, 0)
	seedWalletAccount(t, 2194, 0, 30, 0)
	seedSignedPiggyProfile(t, 2194)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 1
	var queryCalls int32
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open/payment/singleOrderQuery" {
			atomic.AddInt32(&queryCalls, 1)
			return nil, context.DeadlineExceeded
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2194, Amount: 10})
	require.NoError(t, err)
	staleAt := common.GetTimestamp() - 120
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":                  model.WithdrawStatusApproved,
		"piggy_status":            model.WithdrawStatusApproved,
		"reviewed_at":             staleAt,
		"updated_at":              staleAt,
		"compensation_status":     piggyCompensationStatusPending,
		"manual_review_reason":    "小猪提交结果未知，可使用原流水号恢复提交",
		"request_payload_digest":  "stale-submit-request",
		"response_payload_digest": "stale-submit-response",
	}).Error)

	result, err := AdminRecoverPiggyApprovedSubmissionWithResult(context.Background(), order.Id)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Submitted)
	assert.True(t, result.Recoverable)
	assert.Equal(t, model.WithdrawStatusApproved, result.Status)
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.Status)
	assert.Contains(t, refreshed.ManualReviewReason, "查询失败")
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)
	assert.Empty(t, refreshed.ResponsePayloadDigest)
}

func TestAdminRecoverPiggyApprovedSubmissionStopsWhenQueryCannotProveStatus(t *testing.T) {
	truncate(t)
	seedUser(t, 2191, 0)
	seedWalletAccount(t, 2191, 0, 30, 0)
	seedSignedPiggyProfile(t, 2191)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 1
	var submitCalls int32
	var queryCalls int32
	queryResponse := `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderQuery":
			atomic.AddInt32(&queryCalls, 1)
			return queryResponse
		case "/open/payment/singleOrderSubmit":
			atomic.AddInt32(&submitCalls, 1)
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2191, Amount: 10})
	require.NoError(t, err)
	staleAt := common.GetTimestamp() - 120
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":               model.WithdrawStatusApproved,
		"piggy_status":         model.WithdrawStatusApproved,
		"reviewed_at":          staleAt,
		"updated_at":           staleAt,
		"compensation_status":  piggyCompensationStatusPending,
		"manual_review_reason": "小猪提交结果未知，可使用原流水号恢复提交",
	}).Error)

	result, err := AdminRecoverPiggyApprovedSubmissionWithResult(context.Background(), order.Id)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Submitted)
	assert.True(t, result.Recoverable)
	assert.Equal(t, model.WithdrawStatusApproved, result.Status)
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.Status)
	assert.Contains(t, refreshed.ManualReviewReason, "无法判定")
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)
	assert.Equal(t, digestPayload([]byte(queryResponse)), refreshed.ResponsePayloadDigest)
}

func TestAdminRecoverPiggyApprovedSubmissionTurnsMissingProviderOrderIntoManualReview(t *testing.T) {
	truncate(t)
	seedUser(t, 2195, 0)
	seedWalletAccount(t, 2195, 0, 30, 0)
	seedSignedPiggyProfile(t, 2195)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 1
	var submitCalls int32
	var queryCalls int32
	queryResponse := `{"code":"QUERY_NOT_FOUND","msg":"fail","isSuccess":false,"errorMessage":"劳务订单不存在"}`
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderQuery":
			atomic.AddInt32(&queryCalls, 1)
			return queryResponse
		case "/open/payment/singleOrderSubmit":
			atomic.AddInt32(&submitCalls, 1)
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2195, Amount: 10})
	require.NoError(t, err)
	staleAt := common.GetTimestamp() - 120
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":               model.WithdrawStatusApproved,
		"piggy_status":         model.WithdrawStatusApproved,
		"reviewed_at":          staleAt,
		"updated_at":           staleAt,
		"compensation_status":  piggyCompensationStatusPending,
		"manual_review_reason": "小猪提交结果未知，可使用原流水号恢复提交",
	}).Error)

	result, err := AdminRecoverPiggyApprovedSubmissionWithResult(context.Background(), order.Id)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Submitted)
	assert.False(t, result.Recoverable)
	assert.Equal(t, model.WithdrawStatusManualReview, result.Status)
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.PiggyStatus)
	assert.Equal(t, piggyCompensationStatusPending, refreshed.CompensationStatus)
	assert.Contains(t, refreshed.ManualReviewReason, "劳务订单不存在")
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)
	assert.Equal(t, digestPayload([]byte(queryResponse)), refreshed.ResponsePayloadDigest)
}

func TestAdminRejectPiggyWithdrawBeforeSubmitReleasesFrozenCommission(t *testing.T) {
	truncate(t)
	seedUser(t, 2153, 0)
	seedWalletAccount(t, 2153, 0, 30, 0)
	seedSignedPiggyProfile(t, 2153)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var submitCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		atomic.AddInt32(&submitCalls, 1)
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2153, Amount: 10})
	require.NoError(t, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))

	require.NoError(t, AdminRejectWithdrawOrder(context.Background(), order.Id, 7, "risk rejected"))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusRejected, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusRejected, refreshed.PiggyStatus)
	assert.Equal(t, 7, refreshed.ReviewerId)
	assert.Equal(t, "risk rejected", refreshed.FailReason)
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))

	account, err := model.GetWalletAccountByUserId(2153)
	require.NoError(t, err)
	assert.InDelta(t, 30.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 2153, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
}

func TestSubmitPiggyWithdrawRejectsTransferredCommissionAndBusinessFailureReleasesFrozenCommission(t *testing.T) {
	truncate(t)
	seedUser(t, 2113, 0)
	seedWalletAccount(t, 2113, 100, 20, 0)
	seedSignedPiggyProfile(t, 2113)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")

	require.NoError(t, TransferCommissionToBalance(2113, 15))
	_, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2113, Amount: 10})
	require.ErrorIs(t, err, ErrCommissionInsufficient)

	require.NoError(t, model.DB.Model(&model.WalletAccount{}).Where("user_id = ?", 2113).Updates(map[string]interface{}{
		"commission_amount": 25.0,
	}).Error)
	mockPiggyClientForTest(t, func(r *http.Request) string {
		assert.Equal(t, "/open/payment/singleOrderSubmit", r.URL.Path)
		return `{"code":"0","msg":"success","isSuccess":false,"errorMessage":"remote unknown"}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2113, Amount: 10})
	require.NoError(t, err)
	require.NotNil(t, order)
	err = AdminApproveWithdrawOrder(context.Background(), order.Id, 7, "approved")
	require.Error(t, err)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusFailed, refreshed.Status)
	assert.Contains(t, refreshed.FailReason, "remote unknown")
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)
	assert.NotEmpty(t, refreshed.ResponsePayloadDigest)

	account, err := model.GetWalletAccountByUserId(2113)
	require.NoError(t, err)
	assert.InDelta(t, 115.0, account.BalanceAmount, 0.000001)
	assert.InDelta(t, 25.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 2113, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
}

func TestSubmitPiggyWithdrawDuplicateOuterTradeNoMovesManualReviewAndKeepsFrozenCommission(t *testing.T) {
	truncate(t)
	seedUser(t, 2147, 0)
	seedWalletAccount(t, 2147, 100, 30, 0)
	seedSignedPiggyProfile(t, 2147)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderSubmit" {
			return `{"code":"DUPLICATE_OUTER_TRADE_NO","msg":"outerTradeNo already exists","isSuccess":false,"errorMessage":"outerTradeNo already exists"}`
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2147, Amount: 10})
	require.NoError(t, err)
	require.NotNil(t, order)
	require.NoError(t, AdminApproveWithdrawOrder(context.Background(), order.Id, 7, "approved"))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.PiggyStatus)
	assert.Contains(t, refreshed.ManualReviewReason, "outerTradeNo")
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)
	assert.NotEmpty(t, refreshed.ResponsePayloadDigest)

	account, err := model.GetWalletAccountByUserId(2147)
	require.NoError(t, err)
	assert.InDelta(t, 100.0, account.BalanceAmount, 0.000001)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(0), countWalletFlows(t, 2147, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
}

func TestSubmitPiggyWithdrawDuplicateOuterTradeNoReturnsCurrentResultWhenCallbackAlreadyPaid(t *testing.T) {
	truncate(t)
	seedUser(t, 2149, 0)
	seedWalletAccount(t, 2149, 100, 30, 0)
	seedSignedPiggyProfile(t, 2149)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open/payment/singleOrderSubmit" {
			var created model.WithdrawOrder
			require.NoError(t, model.DB.Where("user_id = ? AND provider = ?", 2149, model.WithdrawProviderPiggyLaborV3).Order("id desc").First(&created).Error)
			require.NoError(t, markPiggyOrderPaid(PiggyPaymentCallbackContent{
				OuterTradeNo:        created.WithdrawNo,
				NotifyType:          "tradeResult",
				TradeStatus:         "success",
				FrontLogNo:          "front-duplicate-race",
				LaborOrderNo:        "labor-duplicate-race",
				PretaxAmount:        "10.00",
				AfterTaxAmount:      "9.10",
				IndividualTaxAmount: "0.90",
				FeeAmount:           "0.00",
				CalcType:            "C",
			}))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioNopCloser(`{"code":"DUPLICATE_OUTER_TRADE_NO","msg":"outerTradeNo already exists","isSuccess":false,"errorMessage":"outerTradeNo already exists"}`),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2149, Amount: 10})
	require.NoError(t, err)
	require.NotNil(t, order)
	result, err := AdminApproveWithdrawOrderWithResult(context.Background(), order.Id, 7, "approved")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Submitted)
	assert.False(t, result.Recoverable)
	assert.Equal(t, model.WithdrawStatusPaid, result.Status)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.PiggyStatus)
	assert.Equal(t, order.WithdrawNo, refreshed.ExternalTradeNo)
	assert.NotZero(t, refreshed.SubmittedAt)
	assert.Equal(t, int64(1), countWalletFlows(t, 2149, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))

	account, err := model.GetWalletAccountByUserId(2149)
	require.NoError(t, err)
	assert.InDelta(t, 100.0, account.BalanceAmount, 0.000001)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
}

func TestSubmitPiggyWithdrawBusinessFailureDoesNotOverrideConcurrentManualReview(t *testing.T) {
	truncate(t)
	seedUser(t, 2148, 0)
	seedWalletAccount(t, 2148, 100, 30, 0)
	seedSignedPiggyProfile(t, 2148)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/open/payment/singleOrderSubmit", r.URL.Path)
		var created model.WithdrawOrder
		require.NoError(t, model.DB.Where("user_id = ? AND provider = ?", 2148, model.WithdrawProviderPiggyLaborV3).Order("id desc").First(&created).Error)
		require.NoError(t, markPiggyOrderManualReviewByIdIfActive(created.Id, "小猪回调金额不一致，请人工核对", "", "", model.WithdrawStatusApproved))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"REMOTE_FAILED","msg":"remote failed","isSuccess":false,"errorMessage":"remote failed"}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2148, Amount: 10})
	require.NoError(t, err)
	require.NotNil(t, order)
	result, err := AdminApproveWithdrawOrderWithResult(context.Background(), order.Id, 7, "approved")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Submitted)
	assert.False(t, result.Recoverable)
	assert.Equal(t, model.WithdrawStatusManualReview, result.Status)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.PiggyStatus)
	assert.Contains(t, refreshed.ManualReviewReason, "金额不一致")

	account, err := model.GetWalletAccountByUserId(2148)
	require.NoError(t, err)
	assert.InDelta(t, 100.0, account.BalanceAmount, 0.000001)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(0), countWalletFlows(t, 2148, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
}

func TestPiggyDuplicateOuterTradeNoFailureDetection(t *testing.T) {
	tests := []struct {
		name string
		resp *PiggyAPIResponse
		err  error
		want bool
	}{
		{
			name: "camel case outer trade no already exists",
			resp: &PiggyAPIResponse{
				ErrorCode: "DUPLICATE_OUTER_TRADE_NO",
				Msg:       "outerTradeNo already exists",
			},
			want: true,
		},
		{
			name: "snake case outer trade no exists in Chinese",
			resp: &PiggyAPIResponse{Msg: "outer_trade_no 已存在"},
			want: true,
		},
		{
			name: "account does not exist is not duplicate order",
			resp: &PiggyAPIResponse{Msg: "account does not exist"},
			want: false,
		},
		{
			name: "merchant already exists is not duplicate order",
			resp: &PiggyAPIResponse{Msg: "merchant already exists"},
			want: false,
		},
		{
			name: "duplicate request without outer trade no is not duplicate order",
			err:  errors.New("duplicate request"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isPiggyDuplicateOuterTradeNoFailure(tt.resp, tt.err))
		})
	}
}

func TestSubmitPiggyWithdrawUnknownSubmitResultKeepsFrozenCommissionAndReturnsOrder(t *testing.T) {
	truncate(t)
	seedUser(t, 2138, 0)
	seedWalletAccount(t, 2138, 100, 30, 0)
	seedSignedPiggyProfile(t, 2138)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 5
	var submitCalls int32
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open/payment/singleOrderSubmit" {
			call := atomic.AddInt32(&submitCalls, 1)
			if call == 1 {
				return nil, errors.New("dial tcp: no route to host")
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2138, Amount: 10})
	require.NoError(t, err)
	require.NotNil(t, order)
	require.NoError(t, AdminApproveWithdrawOrder(context.Background(), order.Id, 7, "approved"))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.PiggyStatus)
	assert.Empty(t, refreshed.ExternalTradeNo)
	assert.Equal(t, int64(0), refreshed.SubmittedAt)
	assert.Equal(t, piggyCompensationStatusPending, refreshed.CompensationStatus)
	assert.Contains(t, refreshed.ManualReviewReason, "小猪订单提交结果未知")
	assert.Contains(t, refreshed.ManualReviewReason, "可使用原流水号恢复提交")
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)

	account, err := model.GetWalletAccountByUserId(2138)
	require.NoError(t, err)
	assert.InDelta(t, 100.0, account.BalanceAmount, 0.000001)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(0), countWalletFlows(t, 2138, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))

	staleAt := common.GetTimestamp() - 120
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"reviewed_at": staleAt,
		"updated_at":  staleAt,
	}).Error)
	require.NoError(t, AdminRecoverPiggyApprovedSubmission(context.Background(), order.Id))

	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.PiggyStatus)
	assert.Empty(t, refreshed.ExternalTradeNo)
	assert.Contains(t, refreshed.ManualReviewReason, "无法判定")
	assert.Equal(t, int32(1), atomic.LoadInt32(&submitCalls))
}

func TestSubmitPiggyWithdrawTimeoutDoesNotDowngradeConcurrentTerminalCallback(t *testing.T) {
	truncate(t)
	seedUser(t, 2141, 0)
	seedWalletAccount(t, 2141, 0, 30, 0)
	seedSignedPiggyProfile(t, 2141)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/open/payment/singleOrderSubmit", r.URL.Path)
		var created model.WithdrawOrder
		require.NoError(t, model.DB.Where("user_id = ? AND provider = ?", 2141, model.WithdrawProviderPiggyLaborV3).Order("id desc").First(&created).Error)
		require.NoError(t, markPiggyOrderPaid(PiggyPaymentCallbackContent{
			OuterTradeNo:        created.WithdrawNo,
			NotifyType:          "tradeResult",
			TradeStatus:         "success",
			FrontLogNo:          "front-submit-race",
			LaborOrderNo:        "labor-submit-race",
			PretaxAmount:        "10.00",
			AfterTaxAmount:      "9.10",
			IndividualTaxAmount: "0.90",
			FeeAmount:           "0.00",
			CalcType:            "C",
		}))
		return nil, errors.New("timeout after remote accepted")
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2141, Amount: 10})
	require.NoError(t, err)
	require.NotNil(t, order)
	result, err := AdminApproveWithdrawOrderWithResult(context.Background(), order.Id, 7, "approved")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Submitted)
	assert.False(t, result.Recoverable)
	assert.Equal(t, model.WithdrawStatusPaid, result.Status)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.PiggyStatus)
	assert.Equal(t, order.WithdrawNo, refreshed.ExternalTradeNo)
	assert.NotZero(t, refreshed.SubmittedAt)
	assert.Equal(t, int64(1), countWalletFlows(t, 2141, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))

	account, err := model.GetWalletAccountByUserId(2141)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
}

func TestSubmitPiggyWithdrawSuccessDoesNotDowngradeConcurrentTerminalCallback(t *testing.T) {
	truncate(t)
	seedUser(t, 2143, 0)
	seedWalletAccount(t, 2143, 0, 30, 0)
	seedSignedPiggyProfile(t, 2143)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/open/payment/singleOrderSubmit", r.URL.Path)
		var created model.WithdrawOrder
		require.NoError(t, model.DB.Where("user_id = ? AND provider = ?", 2143, model.WithdrawProviderPiggyLaborV3).Order("id desc").First(&created).Error)
		require.NoError(t, markPiggyOrderPaid(PiggyPaymentCallbackContent{
			OuterTradeNo:        created.WithdrawNo,
			NotifyType:          "tradeResult",
			TradeStatus:         "success",
			FrontLogNo:          "front-submit-success-race",
			LaborOrderNo:        "labor-submit-success-race",
			PretaxAmount:        "10.00",
			AfterTaxAmount:      "9.10",
			IndividualTaxAmount: "0.90",
			FeeAmount:           "0.00",
			CalcType:            "C",
		}))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2143, Amount: 10})
	require.NoError(t, err)
	require.NotNil(t, order)
	require.NoError(t, AdminApproveWithdrawOrder(context.Background(), order.Id, 7, "approved"))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.PiggyStatus)
	assert.Equal(t, order.WithdrawNo, refreshed.ExternalTradeNo)
	assert.NotZero(t, refreshed.SubmittedAt)
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)
	assert.NotEmpty(t, refreshed.ResponsePayloadDigest)
	assert.Equal(t, int64(1), countWalletFlows(t, 2143, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))

	account, err := model.GetWalletAccountByUserId(2143)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
}

func TestPiggyQueryFailureDoesNotDowngradeTerminalOrder(t *testing.T) {
	truncate(t)
	seedUser(t, 2142, 0)
	seedWalletAccount(t, 2142, 0, 30, 0)
	seedSignedPiggyProfile(t, 2142)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open/payment/singleOrderQuery" {
			return nil, errors.New("query timeout")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2142, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()
	require.NoError(t, markPiggyOrderPaid(PiggyPaymentCallbackContent{
		OuterTradeNo:        order.WithdrawNo,
		NotifyType:          "tradeResult",
		TradeStatus:         "success",
		FrontLogNo:          "front-query-race",
		LaborOrderNo:        "labor-query-race",
		PretaxAmount:        "10.00",
		AfterTaxAmount:      "9.10",
		IndividualTaxAmount: "0.90",
		FeeAmount:           "0.00",
		CalcType:            "C",
	}))

	require.NoError(t, queryPiggyOrderStatus(context.Background(), order.WithdrawNo))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.PiggyStatus)
	assert.Equal(t, int64(1), countWalletFlows(t, 2142, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
}

func TestPiggySubmitResultAwaitConfirmsAndFailureReleasesFrozenCommission(t *testing.T) {
	truncate(t)
	seedUser(t, 2114, 0)
	seedWalletAccount(t, 2114, 0, 50, 0)
	seedSignedPiggyProfile(t, 2114)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientForTest(t, func(r *http.Request) string {
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	awaitOrder, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2114, Amount: 10})
	require.NoError(t, err)
	awaitOrder = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, awaitOrder)
		return &refreshed
	}()
	awaitBody := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo:        awaitOrder.WithdrawNo,
		NotifyType:          "submitResult",
		TradeStatus:         "await",
		FrontLogNo:          "front-await",
		PretaxAmount:        "10.00",
		IndividualTaxAmount: "0.80",
		AfterTaxAmount:      "9.20",
		FeeAmount:           "0.00",
		CalcType:            "C",
	})
	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), awaitBody, ""))

	var confirmed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", awaitOrder.Id).First(&confirmed).Error)
	assert.Equal(t, model.WithdrawStatusConfirmed, confirmed.Status)
	assert.Equal(t, int64(920), confirmed.PiggyAfterTaxAmountCents)

	failedOrder, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2114, Amount: 5})
	require.NoError(t, err)
	failedOrder = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, failedOrder)
		return &refreshed
	}()
	failedBody := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo:        failedOrder.WithdrawNo,
		NotifyType:          "submitResult",
		TradeStatus:         "failure",
		FrontLogNo:          "front-failure",
		TradeFailCode:       "SUBMIT_FAIL",
		TradeResultDescribe: "submit rejected",
	})
	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), failedBody, ""))
	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), failedBody, ""))

	account, err := model.GetWalletAccountByUserId(2114)
	require.NoError(t, err)
	assert.InDelta(t, 40.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 2114, model.WalletFlowTypeWithdrawReject, failedOrder.WithdrawNo))
}

func TestPiggyPaymentCallbackAcceptsNumericAmountFields(t *testing.T) {
	truncate(t)
	seedUser(t, 2194, 0)
	seedWalletAccount(t, 2194, 0, 30, 0)
	seedSignedPiggyProfile(t, 2194)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientForTest(t, func(r *http.Request) string {
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2194, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()
	body := buildSignedPiggyPaymentCallbackFromMap(t, map[string]any{
		"outerTradeNo":        order.WithdrawNo,
		"notifyType":          "tradeResult",
		"tradeStatus":         "success",
		"frontLogNo":          "front-numeric-amount",
		"laborOrderNo":        "labor-numeric-amount",
		"pretaxAmount":        10,
		"individualTaxAmount": 0.9,
		"addedTaxAmount":      0,
		"afterTaxAmount":      9.1,
		"feeAmount":           0,
		"calcType":            "C",
	})

	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, int64(1000), refreshed.PiggyPretaxAmountCents)
	assert.Equal(t, int64(90), refreshed.PiggyIndividualTaxCents)
	assert.Equal(t, int64(0), refreshed.PiggyAddedTaxCents)
	assert.Equal(t, int64(910), refreshed.PiggyAfterTaxAmountCents)
	assert.Equal(t, int64(0), refreshed.PiggyFeeAmountCents)
}

func TestPiggySubmitResultAwaitDoesNotReconfirmConfirmedOrder(t *testing.T) {
	truncate(t)
	seedUser(t, 2117, 0)
	seedWalletAccount(t, 2117, 0, 30, 0)
	seedSignedPiggyProfile(t, 2117)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	confirmCalls := 0
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderConfirmPay" {
			confirmCalls++
			if confirmCalls > 1 {
				return `{"code":"FAIL","msg":"duplicate confirm","isSuccess":false,"errorMessage":"already confirmed"}`
			}
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2117, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()
	firstAwait := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo: order.WithdrawNo,
		NotifyType:   "submitResult",
		TradeStatus:  "await",
		FrontLogNo:   "front-await-1",
	})
	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), firstAwait, ""))

	secondAwait := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo: order.WithdrawNo,
		NotifyType:   "submitResult",
		TradeStatus:  "await",
		FrontLogNo:   "front-await-2",
	})
	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), secondAwait, ""))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusConfirmed, refreshed.Status)
	assert.Equal(t, 1, confirmCalls)
}

func TestPiggyUnsignedPaymentCallbackSubmitAwaitUsesOrderSnapshot(t *testing.T) {
	truncate(t)
	seedUser(t, 2171, 0)
	seedWalletAccount(t, 2171, 0, 30, 0)
	seedSignedPiggyProfile(t, 2171)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var confirmCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderConfirmPay" {
			atomic.AddInt32(&confirmCalls, 1)
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2171, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()
	body := buildUnsignedPiggyPaymentCallback(t, piggyCallbackContentFromOrderForTest(order, "submitResult", "await"))

	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusConfirmed, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusConfirmed, refreshed.PiggyStatus)
	assert.Equal(t, int32(1), atomic.LoadInt32(&confirmCalls))
}

func TestPiggyUnsignedPaymentCallbackTradeResultFinalizesFundsOnce(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2172, 0)
		seedWalletAccount(t, 2172, 0, 30, 0)
		seedSignedPiggyProfile(t, 2172)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		mockPiggyClientForTest(t, func(r *http.Request) string {
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2172, Amount: 10})
		require.NoError(t, err)
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()
		body := buildUnsignedPiggyPaymentCallback(t, piggyCallbackContentFromOrderForTest(order, "tradeResult", "success"))

		require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))
		require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
		assert.Equal(t, int64(1), countWalletFlows(t, 2172, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
		account, err := model.GetWalletAccountByUserId(2172)
		require.NoError(t, err)
		assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
		assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
		assert.InDelta(t, 10.0, account.TotalWithdrawAmount, 0.000001)
	})

	t.Run("failure", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2173, 0)
		seedWalletAccount(t, 2173, 0, 30, 0)
		seedSignedPiggyProfile(t, 2173)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		mockPiggyClientForTest(t, func(r *http.Request) string {
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2173, Amount: 10})
		require.NoError(t, err)
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()
		content := piggyCallbackContentFromOrderForTest(order, "tradeResult", "failure")
		content.TradeFailCode = "BANK_FAIL"
		content.TradeResultDescribe = "bank rejected"
		body := buildUnsignedPiggyPaymentCallback(t, content)

		require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))
		require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusFailed, refreshed.Status)
		assert.Equal(t, int64(1), countWalletFlows(t, 2173, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
		account, err := model.GetWalletAccountByUserId(2173)
		require.NoError(t, err)
		assert.InDelta(t, 30.0, account.CommissionAmount, 0.000001)
		assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
	})
}

func TestPiggyUnsignedPaymentCallbackMissingNotifyTypeDoesNotInferTradeResult(t *testing.T) {
	truncate(t)
	seedUser(t, 2189, 0)
	seedWalletAccount(t, 2189, 0, 30, 0)
	seedSignedPiggyProfile(t, 2189)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientForTest(t, func(r *http.Request) string {
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2189, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()
	content := piggyCallbackContentFromOrderForTest(order, "", "success")

	err = HandlePiggyPaymentCallback(context.Background(), buildUnsignedPiggyPaymentCallback(t, content), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notifyType")

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.Equal(t, int64(0), countWalletFlows(t, 2189, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	assert.Contains(t, refreshed.ManualReviewReason, "notifyType")

	var log model.PiggyWithdrawCallbackLog
	require.NoError(t, model.DB.Order("id desc").First(&log).Error)
	assert.Equal(t, model.PaymentProcessStatusFailed, log.ProcessStatus)
	assert.Contains(t, log.ErrorMessage, "notifyType")
}

func TestPiggyUnsignedPaymentCallbackRejectsMismatchButAllowsMissingBankName(t *testing.T) {
	t.Run("amount mismatch", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2174, 0)
		seedWalletAccount(t, 2174, 0, 30, 0)
		seedSignedPiggyProfile(t, 2174)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		var confirmCalls int32
		mockPiggyClientForTest(t, func(r *http.Request) string {
			if r.URL.Path == "/open/payment/singleOrderConfirmPay" {
				atomic.AddInt32(&confirmCalls, 1)
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2174, Amount: 10})
		require.NoError(t, err)
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()
		content := piggyCallbackContentFromOrderForTest(order, "submitResult", "await")
		content.PretaxAmount = "9.99"

		require.Error(t, HandlePiggyPaymentCallback(context.Background(), buildUnsignedPiggyPaymentCallback(t, content), ""))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusSubmitted, refreshed.Status)
		assert.Equal(t, int32(0), atomic.LoadInt32(&confirmCalls))
	})

	t.Run("payout snapshot mismatch", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2175, 0)
		seedWalletAccount(t, 2175, 0, 30, 0)
		seedSignedPiggyProfile(t, 2175)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		mockPiggyClientForTest(t, func(r *http.Request) string {
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2175, Amount: 10})
		require.NoError(t, err)
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()
		content := piggyCallbackContentFromOrderForTest(order, "tradeResult", "success")
		content.PayAccount = "6222000099998888"

		require.Error(t, HandlePiggyPaymentCallback(context.Background(), buildUnsignedPiggyPaymentCallback(t, content), ""))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusSubmitted, refreshed.Status)
		assert.Equal(t, int64(0), countWalletFlows(t, 2175, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	})

	t.Run("bank name omitted", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2176, 0)
		seedWalletAccount(t, 2176, 0, 30, 0)
		seedSignedPiggyProfile(t, 2176)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		var confirmCalls int32
		mockPiggyClientForTest(t, func(r *http.Request) string {
			if r.URL.Path == "/open/payment/singleOrderConfirmPay" {
				atomic.AddInt32(&confirmCalls, 1)
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2176, Amount: 10})
		require.NoError(t, err)
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()
		content := piggyCallbackContentFromOrderForTest(order, "submitResult", "await")
		content.BankName = ""

		require.NoError(t, HandlePiggyPaymentCallback(context.Background(), buildUnsignedPiggyPaymentCallback(t, content), ""))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusConfirmed, refreshed.Status)
		assert.Equal(t, int32(1), atomic.LoadInt32(&confirmCalls))
	})
}

func TestPiggySubmitResultAwaitAmountMismatchMovesManualReviewWithoutConfirm(t *testing.T) {
	truncate(t)
	seedUser(t, 2148, 0)
	seedWalletAccount(t, 2148, 0, 30, 0)
	seedSignedPiggyProfile(t, 2148)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var confirmCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderConfirmPay" {
			atomic.AddInt32(&confirmCalls, 1)
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2148, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()
	body := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo:   order.WithdrawNo,
		NotifyType:     "submitResult",
		TradeStatus:    "await",
		FrontLogNo:     "front-amount-mismatch",
		PretaxAmount:   "9.99",
		AfterTaxAmount: "9.49",
		FeeAmount:      "0.00",
		CalcType:       "C",
	})

	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.PiggyStatus)
	assert.Contains(t, refreshed.ManualReviewReason, "金额")
	assert.Equal(t, int32(0), atomic.LoadInt32(&confirmCalls))

	account, err := model.GetWalletAccountByUserId(2148)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(0), countWalletFlows(t, 2148, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
}

func TestPiggyTradeResultSuccessIsTerminalAndIdempotent(t *testing.T) {
	truncate(t)
	seedUser(t, 2104, 0)
	seedWalletAccount(t, 2104, 0, 30, 0)
	seedSignedPiggyProfile(t, 2104)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})
	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2104, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()

	body := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo:        order.WithdrawNo,
		NotifyType:          "tradeResult",
		TradeStatus:         "success",
		FrontLogNo:          "front-2104",
		LaborOrderNo:        "labor-2104",
		PretaxAmount:        "10.00",
		IndividualTaxAmount: "0.60",
		AddedTaxAmount:      "0.00",
		AfterTaxAmount:      "9.40",
		FeeAmount:           "0.00",
		CalcType:            "C",
	})
	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))
	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("withdraw_no = ?", order.WithdrawNo).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, int64(940), refreshed.PiggyAfterTaxAmountCents)
	assert.Equal(t, int64(60), refreshed.PiggyIndividualTaxCents)

	account, err := model.GetWalletAccountByUserId(2104)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.TotalWithdrawAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 2104, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
}

func TestPiggyPaymentCallbackAcceptsStringIsSuccessT(t *testing.T) {
	truncate(t)
	seedUser(t, 2170, 0)
	seedWalletAccount(t, 2170, 0, 30, 0)
	seedSignedPiggyProfile(t, 2170)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})
	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2170, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()

	body := buildSignedPiggyPaymentCallbackWithIsSuccess(t, PiggyPaymentCallbackContent{
		OuterTradeNo:        order.WithdrawNo,
		NotifyType:          "tradeResult",
		TradeStatus:         "success",
		FrontLogNo:          "front-string-success",
		LaborOrderNo:        "labor-string-success",
		PretaxAmount:        "10.00",
		IndividualTaxAmount: "0.60",
		AfterTaxAmount:      "9.40",
		FeeAmount:           "0.00",
		CalcType:            "C",
	}, "T")

	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("withdraw_no = ?", order.WithdrawNo).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, int64(1), countWalletFlows(t, 2170, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))

	var callbackLog model.PiggyWithdrawCallbackLog
	require.NoError(t, model.DB.Where("order_no = ?", order.WithdrawNo).Order("id desc").First(&callbackLog).Error)
	assert.Equal(t, model.PaymentProcessStatusSuccess, callbackLog.ProcessStatus)
	assert.Empty(t, callbackLog.ErrorMessage)
}

func TestPiggyTradeResultSuccessAmountMismatchMovesManualReviewWithoutSettlement(t *testing.T) {
	truncate(t)
	seedUser(t, 2150, 0)
	seedWalletAccount(t, 2150, 0, 30, 0)
	seedSignedPiggyProfile(t, 2150)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})
	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2150, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()

	body := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo:        order.WithdrawNo,
		NotifyType:          "tradeResult",
		TradeStatus:         "success",
		FrontLogNo:          "front-success-mismatch",
		LaborOrderNo:        "labor-success-mismatch",
		PretaxAmount:        "10.01",
		IndividualTaxAmount: "0.60",
		AfterTaxAmount:      "9.41",
		FeeAmount:           "0.00",
		CalcType:            "C",
	})
	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("withdraw_no = ?", order.WithdrawNo).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.Contains(t, refreshed.ManualReviewReason, "金额")

	account, err := model.GetWalletAccountByUserId(2150)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.TotalWithdrawAmount, 0.000001)
	assert.Equal(t, int64(0), countWalletFlows(t, 2150, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
}

func TestPiggyTradeResultFailureReleasesFrozenCommissionOnce(t *testing.T) {
	truncate(t)
	seedUser(t, 2105, 0)
	seedWalletAccount(t, 2105, 100, 30, 0)
	seedSignedPiggyProfile(t, 2105)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})
	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2105, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()

	body := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo:        order.WithdrawNo,
		NotifyType:          "tradeResult",
		TradeStatus:         "failure",
		FrontLogNo:          "front-2105",
		TradeFailCode:       "BANK_FAIL",
		TradeResultDescribe: "bank rejected",
	})
	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))
	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

	account, err := model.GetWalletAccountByUserId(2105)
	require.NoError(t, err)
	assert.InDelta(t, 30.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 2105, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
}

func TestPiggyPaymentCallbackInvalidSignatureIsAudited(t *testing.T) {
	truncate(t)
	seedUser(t, 2115, 0)
	seedWalletAccount(t, 2115, 0, 20, 0)
	seedSignedPiggyProfile(t, 2115)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})
	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2115, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()

	body := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo: order.WithdrawNo,
		NotifyType:   "tradeResult",
		TradeStatus:  "success",
	})
	var tampered map[string]any
	require.NoError(t, common.Unmarshal(body, &tampered))
	tampered["sign"] = "bad-signature"
	body, err = common.Marshal(tampered)
	require.NoError(t, err)
	require.Error(t, HandlePiggyPaymentCallback(context.Background(), body, ""))

	var log model.PiggyWithdrawCallbackLog
	require.NoError(t, model.DB.Order("id desc").First(&log).Error)
	assert.Equal(t, model.PaymentProcessStatusFailed, log.ProcessStatus)
	assert.Contains(t, log.ErrorMessage, "验签")
}

func TestPiggyEncryptedQueryBizAESContentProcessesTradeStatusWithoutNotifyType(t *testing.T) {
	t.Run("sparse encrypted success still pays", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2176, 0)
		seedWalletAccount(t, 2176, 0, 30, 0)
		seedSignedPiggyProfile(t, 2176)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		var orderNo string
		mockPiggyClientForTest(t, func(r *http.Request) string {
			if r.URL.Path == "/open/payment/singleOrderQuery" {
				return buildPiggyEncryptedQueryResponseForTest(t, PiggyPaymentCallbackContent{
					OuterTradeNo: orderNo,
					TradeStatus:  "success",
					PretaxAmount: "10.00",
				})
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2176, Amount: 10})
		require.NoError(t, err)
		orderNo = order.WithdrawNo
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()

		require.NoError(t, queryPiggyOrderStatus(context.Background(), order.WithdrawNo))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
		assert.Equal(t, int64(1), countWalletFlows(t, 2176, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	})

	t.Run("missing outer trade no rejects", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2186, 0)
		seedWalletAccount(t, 2186, 0, 30, 0)
		seedSignedPiggyProfile(t, 2186)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		var orderNo string
		mockPiggyClientForTest(t, func(r *http.Request) string {
			if r.URL.Path == "/open/payment/singleOrderQuery" {
				return buildPiggyEncryptedQueryResponseForTest(t, PiggyPaymentCallbackContent{
					TradeStatus:  "success",
					PretaxAmount: "10.00",
				})
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2186, Amount: 10})
		require.NoError(t, err)
		orderNo = order.WithdrawNo
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()

		err = queryPiggyOrderStatus(context.Background(), orderNo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outerTradeNo")

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusSubmitted, refreshed.Status)
		assert.Equal(t, int64(0), countWalletFlows(t, 2186, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	})

	t.Run("missing pretax amount rejects", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2187, 0)
		seedWalletAccount(t, 2187, 0, 30, 0)
		seedSignedPiggyProfile(t, 2187)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		var orderNo string
		mockPiggyClientForTest(t, func(r *http.Request) string {
			if r.URL.Path == "/open/payment/singleOrderQuery" {
				return buildPiggyEncryptedQueryResponseForTest(t, PiggyPaymentCallbackContent{
					OuterTradeNo: orderNo,
					TradeStatus:  "success",
				})
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2187, Amount: 10})
		require.NoError(t, err)
		orderNo = order.WithdrawNo
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()

		err = queryPiggyOrderStatus(context.Background(), orderNo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pretaxAmount")

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusSubmitted, refreshed.Status)
		assert.Equal(t, int64(0), countWalletFlows(t, 2187, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	})

	t.Run("mismatched outer trade no rejects", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2188, 0)
		seedWalletAccount(t, 2188, 0, 30, 0)
		seedSignedPiggyProfile(t, 2188)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		mockPiggyClientForTest(t, func(r *http.Request) string {
			if r.URL.Path == "/open/payment/singleOrderQuery" {
				return buildPiggyEncryptedQueryResponseForTest(t, PiggyPaymentCallbackContent{
					OuterTradeNo: "other-withdraw-no",
					TradeStatus:  "success",
					PretaxAmount: "10.00",
				})
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2188, Amount: 10})
		require.NoError(t, err)
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()

		err = queryPiggyOrderStatus(context.Background(), order.WithdrawNo)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outerTradeNo")

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusSubmitted, refreshed.Status)
		assert.Equal(t, int64(0), countWalletFlows(t, 2188, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	})

	t.Run("await confirms", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2177, 0)
		seedWalletAccount(t, 2177, 0, 30, 0)
		seedSignedPiggyProfile(t, 2177)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		var orderNo string
		var confirmCalls int32
		mockPiggyClientForTest(t, func(r *http.Request) string {
			switch r.URL.Path {
			case "/open/payment/singleOrderQuery":
				content := piggyCallbackContentFromOrderForTest(&model.WithdrawOrder{
					WithdrawNo:        orderNo,
					AccountName:       "张三",
					PayoutMobile:      "13812345678",
					PayoutIdCardNo:    "110101199001011234",
					PayoutBankCardNo:  "6222000011118888",
					BankName:          "招商银行",
					PositionName:      operation_setting.GetPiggyWithdrawSetting().PositionName,
					FrozenAmountCents: 1000,
					CalcType:          "C",
				}, "", "await")
				return buildPiggyEncryptedQueryResponseForTest(t, content)
			case "/open/payment/singleOrderConfirmPay":
				atomic.AddInt32(&confirmCalls, 1)
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2177, Amount: 10})
		require.NoError(t, err)
		orderNo = order.WithdrawNo
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()

		require.NoError(t, queryPiggyOrderStatus(context.Background(), order.WithdrawNo))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusConfirmed, refreshed.Status)
		assert.Equal(t, int32(1), atomic.LoadInt32(&confirmCalls))
	})

	t.Run("success pays", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2178, 0)
		seedWalletAccount(t, 2178, 0, 30, 0)
		seedSignedPiggyProfile(t, 2178)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		var orderNo string
		mockPiggyClientForTest(t, func(r *http.Request) string {
			if r.URL.Path == "/open/payment/singleOrderQuery" {
				return buildPiggyEncryptedQueryResponseForTest(t, piggyCallbackContentFromOrderForTest(&model.WithdrawOrder{
					WithdrawNo:        orderNo,
					AccountName:       "张三",
					PayoutMobile:      "13812345678",
					PayoutIdCardNo:    "110101199001011234",
					PayoutBankCardNo:  "6222000011118888",
					BankName:          "招商银行",
					PositionName:      operation_setting.GetPiggyWithdrawSetting().PositionName,
					FrozenAmountCents: 1000,
					CalcType:          "C",
				}, "", "success"))
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2178, Amount: 10})
		require.NoError(t, err)
		orderNo = order.WithdrawNo
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()

		require.NoError(t, queryPiggyOrderStatus(context.Background(), order.WithdrawNo))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
		assert.Equal(t, int64(1), countWalletFlows(t, 2178, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	})

	t.Run("failure releases", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2179, 0)
		seedWalletAccount(t, 2179, 0, 30, 0)
		seedSignedPiggyProfile(t, 2179)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		var orderNo string
		mockPiggyClientForTest(t, func(r *http.Request) string {
			if r.URL.Path == "/open/payment/singleOrderQuery" {
				content := piggyCallbackContentFromOrderForTest(&model.WithdrawOrder{
					WithdrawNo:        orderNo,
					AccountName:       "张三",
					PayoutMobile:      "13812345678",
					PayoutIdCardNo:    "110101199001011234",
					PayoutBankCardNo:  "6222000011118888",
					BankName:          "招商银行",
					PositionName:      operation_setting.GetPiggyWithdrawSetting().PositionName,
					FrozenAmountCents: 1000,
					CalcType:          "C",
				}, "", "failure")
				content.TradeFailCode = "BANK_FAIL"
				content.TradeResultDescribe = "bank rejected"
				return buildPiggyEncryptedQueryResponseForTest(t, content)
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2179, Amount: 10})
		require.NoError(t, err)
		orderNo = order.WithdrawNo
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()

		require.NoError(t, queryPiggyOrderStatus(context.Background(), order.WithdrawNo))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusFailed, refreshed.Status)
		assert.Equal(t, int64(1), countWalletFlows(t, 2179, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
		account, err := model.GetWalletAccountByUserId(2179)
		require.NoError(t, err)
		assert.InDelta(t, 30.0, account.CommissionAmount, 0.000001)
		assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
	})

	t.Run("pretax mismatch rejects", func(t *testing.T) {
		truncate(t)
		seedUser(t, 2184, 0)
		seedWalletAccount(t, 2184, 0, 30, 0)
		seedSignedPiggyProfile(t, 2184)
		configurePiggyWithdrawForTest(t, "https://piggy.example.com")
		operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
		var orderNo string
		mockPiggyClientForTest(t, func(r *http.Request) string {
			if r.URL.Path == "/open/payment/singleOrderQuery" {
				content := piggyCallbackContentFromOrderForTest(&model.WithdrawOrder{
					WithdrawNo:        orderNo,
					AccountName:       "张三",
					PayoutMobile:      "13812345678",
					PayoutIdCardNo:    "110101199001011234",
					PayoutBankCardNo:  "6222000011118888",
					BankName:          "招商银行",
					PositionName:      operation_setting.GetPiggyWithdrawSetting().PositionName,
					FrozenAmountCents: 1000,
					CalcType:          "C",
				}, "", "success")
				content.PretaxAmount = "9.99"
				return buildPiggyEncryptedQueryResponseForTest(t, content)
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		})

		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2184, Amount: 10})
		require.NoError(t, err)
		orderNo = order.WithdrawNo
		order = func() *model.WithdrawOrder {
			refreshed := approvePiggyWithdrawForTest(t, order)
			return &refreshed
		}()

		require.Error(t, queryPiggyOrderStatus(context.Background(), order.WithdrawNo))

		var refreshed model.WithdrawOrder
		require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
		assert.Equal(t, model.WithdrawStatusSubmitted, refreshed.Status)
		assert.Equal(t, int64(0), countWalletFlows(t, 2184, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
	})
}

func TestSubmitPiggyWithdrawDuplicateOuterTradeNoQueriesOriginalOrderBeforeManualReview(t *testing.T) {
	truncate(t)
	seedUser(t, 2180, 0)
	seedWalletAccount(t, 2180, 0, 30, 0)
	seedSignedPiggyProfile(t, 2180)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var orderNo string
	var submitCalls int32
	var queryCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit":
			atomic.AddInt32(&submitCalls, 1)
			return `{"code":"DUPLICATE_OUTER_TRADE_NO","msg":"outerTradeNo already exists","isSuccess":false,"errorMessage":"outerTradeNo already exists"}`
		case "/open/payment/singleOrderQuery":
			atomic.AddInt32(&queryCalls, 1)
			content := piggyCallbackContentFromOrderForTest(&model.WithdrawOrder{
				WithdrawNo:        orderNo,
				AccountName:       "张三",
				PayoutMobile:      "13812345678",
				PayoutIdCardNo:    "110101199001011234",
				PayoutBankCardNo:  "6222000011118888",
				BankName:          "招商银行",
				PositionName:      operation_setting.GetPiggyWithdrawSetting().PositionName,
				FrozenAmountCents: 1000,
				CalcType:          "C",
			}, "", "success")
			content.FrontLogNo = "front-duplicate-query"
			content.LaborOrderNo = "labor-duplicate-query"
			return buildPiggyEncryptedQueryResponseForTest(t, content)
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2180, Amount: 10})
	require.NoError(t, err)
	orderNo = order.WithdrawNo
	result, err := AdminApproveWithdrawOrderWithResult(context.Background(), order.Id, 7, "approved")
	require.NoError(t, err)
	require.NotNil(t, result)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusPaid, result.Status)
	assert.Equal(t, int32(1), atomic.LoadInt32(&submitCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))
	assert.Equal(t, int64(1), countWalletFlows(t, 2180, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
}

func TestSubmitPiggyWithdrawDuplicateOuterTradeNoStopsWhenQueryCannotProveStatus(t *testing.T) {
	truncate(t)
	seedUser(t, 2192, 0)
	seedWalletAccount(t, 2192, 0, 30, 0)
	seedSignedPiggyProfile(t, 2192)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var submitCalls int32
	var queryCalls int32
	queryResponse := `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit":
			atomic.AddInt32(&submitCalls, 1)
			return `{"code":"DUPLICATE_OUTER_TRADE_NO","msg":"outerTradeNo already exists","isSuccess":false,"errorMessage":"outerTradeNo already exists"}`
		case "/open/payment/singleOrderQuery":
			atomic.AddInt32(&queryCalls, 1)
			return queryResponse
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2192, Amount: 10})
	require.NoError(t, err)
	result, err := AdminApproveWithdrawOrderWithResult(context.Background(), order.Id, 7, "approved")
	require.NoError(t, err)
	require.NotNil(t, result)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.False(t, result.Submitted)
	assert.False(t, result.Recoverable)
	assert.Equal(t, model.WithdrawStatusManualReview, result.Status)
	assert.Equal(t, int32(1), atomic.LoadInt32(&submitCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))
	assert.Contains(t, refreshed.ManualReviewReason, "无法判定")
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)
	assert.Equal(t, digestPayload([]byte(queryResponse)), refreshed.ResponsePayloadDigest)
	assert.Equal(t, int64(0), countWalletFlows(t, 2192, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
}

func TestSubmitPiggyWithdrawDuplicateOuterTradeNoQueryFailureUsesQueryOnlyEvidence(t *testing.T) {
	truncate(t)
	seedUser(t, 2193, 0)
	seedWalletAccount(t, 2193, 0, 30, 0)
	seedSignedPiggyProfile(t, 2193)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var submitCalls int32
	var queryCalls int32
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit":
			atomic.AddInt32(&submitCalls, 1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioNopCloser(`{"code":"DUPLICATE_OUTER_TRADE_NO","msg":"outerTradeNo already exists","isSuccess":false,"errorMessage":"outerTradeNo already exists"}`),
				Header:     make(http.Header),
			}, nil
		case "/open/payment/singleOrderQuery":
			atomic.AddInt32(&queryCalls, 1)
			return nil, context.DeadlineExceeded
		default:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
				Header:     make(http.Header),
			}, nil
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2193, Amount: 10})
	require.NoError(t, err)
	result, err := AdminApproveWithdrawOrderWithResult(context.Background(), order.Id, 7, "approved")
	require.NoError(t, err)
	require.NotNil(t, result)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.False(t, result.Submitted)
	assert.False(t, result.Recoverable)
	assert.Equal(t, model.WithdrawStatusManualReview, result.Status)
	assert.Equal(t, int32(1), atomic.LoadInt32(&submitCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))
	assert.Contains(t, refreshed.ManualReviewReason, "查询失败")
	assert.NotEmpty(t, refreshed.RequestPayloadDigest)
	assert.Empty(t, refreshed.ResponsePayloadDigest)
}

func TestPiggyCompensationScanRecoversApprovedOrderByQueryingBeforeResubmit(t *testing.T) {
	truncate(t)
	seedUser(t, 2181, 0)
	seedWalletAccount(t, 2181, 0, 30, 0)
	seedSignedPiggyProfile(t, 2181)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 1
	var orderNo string
	var submitCalls int32
	var queryCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit":
			atomic.AddInt32(&submitCalls, 1)
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		case "/open/payment/singleOrderQuery":
			atomic.AddInt32(&queryCalls, 1)
			content := piggyCallbackContentFromOrderForTest(&model.WithdrawOrder{
				WithdrawNo:        orderNo,
				AccountName:       "张三",
				PayoutMobile:      "13812345678",
				PayoutIdCardNo:    "110101199001011234",
				PayoutBankCardNo:  "6222000011118888",
				BankName:          "招商银行",
				PositionName:      operation_setting.GetPiggyWithdrawSetting().PositionName,
				FrozenAmountCents: 1000,
				CalcType:          "C",
			}, "", "success")
			content.FrontLogNo = "front-approved-query"
			content.LaborOrderNo = "labor-approved-query"
			return buildPiggyEncryptedQueryResponseForTest(t, content)
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2181, Amount: 10})
	require.NoError(t, err)
	orderNo = order.WithdrawNo
	staleAt := common.GetTimestamp() - 120
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":               model.WithdrawStatusApproved,
		"piggy_status":         model.WithdrawStatusApproved,
		"reviewed_at":          staleAt,
		"updated_at":           staleAt,
		"compensation_status":  piggyCompensationStatusPending,
		"manual_review_reason": "小猪提交结果未知，可使用原流水号恢复提交",
	}).Error)

	processed, err := ScanPiggyWithdrawCompensations(context.Background(), 10)
	require.NoError(t, err)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.GreaterOrEqual(t, processed, 1)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&submitCalls))
	assert.Equal(t, int64(1), countWalletFlows(t, 2181, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
}

func TestPiggyCompensationScanDoesNotCountFailedQueryAsProcessed(t *testing.T) {
	truncate(t)
	seedUser(t, 2183, 0)
	seedWalletAccount(t, 2183, 0, 30, 0)
	seedSignedPiggyProfile(t, 2183)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var queryCalls int32
	mockPiggyClientRoundTripForTest(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/open/payment/singleOrderQuery" {
			atomic.AddInt32(&queryCalls, 1)
			return nil, context.DeadlineExceeded
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`{"code":"0","msg":"success","isSuccess":true,"data":{}}`),
			Header:     make(http.Header),
		}, nil
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2183, Amount: 10})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusSubmitted,
		"piggy_status": model.WithdrawStatusSubmitted,
	}).Error)

	processed, err := ScanPiggyWithdrawCompensations(context.Background(), 10)

	require.NoError(t, err)
	assert.Equal(t, 0, processed)
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.Contains(t, refreshed.ManualReviewReason, "查询失败")
}

func TestPiggyAdminOperationsAndCompensationScan(t *testing.T) {
	truncate(t)
	seedUser(t, 2116, 0)
	seedWalletAccount(t, 2116, 0, 80, 0)
	seedSignedPiggyProfile(t, 2116)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit",
			"/open/payment/singleOrderConfirmPay",
			"/open/payment/singleOrderCancel",
			"/open/payment/singleOrderQuery":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	retryOrder, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2116, Amount: 10})
	require.NoError(t, err)
	retryOrder = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, retryOrder)
		return &refreshed
	}()
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", retryOrder.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusAwaitConfirm,
		"piggy_status": model.WithdrawStatusAwaitConfirm,
	}).Error)
	require.NoError(t, AdminRetryPiggyConfirm(context.Background(), retryOrder.Id))

	var confirmed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", retryOrder.Id).First(&confirmed).Error)
	assert.Equal(t, model.WithdrawStatusConfirmed, confirmed.Status)

	cancelOrder, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2116, Amount: 10})
	require.NoError(t, err)
	cancelOrder = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, cancelOrder)
		return &refreshed
	}()
	require.NoError(t, AdminCancelPiggyOrder(context.Background(), cancelOrder.Id, 1, "admin cancel"))
	require.ErrorIs(t, AdminCancelPiggyOrder(context.Background(), cancelOrder.Id, 1, "again"), ErrWithdrawStatusInvalid)

	manualOrder, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2116, Amount: 10})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", manualOrder.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusManualReview,
		"piggy_status": model.WithdrawStatusManualReview,
	}).Error)
	require.NoError(t, AdminRecordPiggyManualResult(manualOrder.Id, 7, "checked manually", "pending_compensation"))

	scanOrder, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2116, Amount: 5})
	require.NoError(t, err)
	scanOrder = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, scanOrder)
		return &refreshed
	}()
	processed, err := ScanPiggyWithdrawCompensations(context.Background(), 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, processed, 1)

	var manual model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", manualOrder.Id).First(&manual).Error)
	assert.Equal(t, 7, manual.ManualHandledBy)
	assert.Equal(t, "pending_compensation", manual.CompensationStatus)

	var scanned model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", scanOrder.Id).First(&scanned).Error)
	assert.NotEmpty(t, scanned.RequestPayloadDigest)

	account, err := model.GetWalletAccountByUserId(2116)
	require.NoError(t, err)
	assert.InDelta(t, 55.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 25.0, account.FrozenCommissionAmount, 0.000001)
}

func TestPiggyManualResultPaidSettlesFrozenCommission(t *testing.T) {
	truncate(t)
	seedUser(t, 2129, 0)
	seedWalletAccount(t, 2129, 0, 20, 0)
	seedSignedPiggyProfile(t, 2129)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2129, Amount: 10})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusManualReview,
		"piggy_status": model.WithdrawStatusManualReview,
	}).Error)
	require.NoError(t, AdminRecordPiggyManualResult(order.Id, 7, "paid outside system", "manual_paid"))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.PiggyStatus)
	assert.Equal(t, piggyCompensationStatusManualProcessed, refreshed.CompensationStatus)
	assert.Equal(t, 7, refreshed.ManualHandledBy)
	assert.NotZero(t, refreshed.PaidAt)
	assert.NotZero(t, refreshed.TerminalAt)

	account, err := model.GetWalletAccountByUserId(2129)
	require.NoError(t, err)
	assert.InDelta(t, 10.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.TotalWithdrawAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 2129, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
}

func TestPiggyManualResultFailedReleasesFrozenCommission(t *testing.T) {
	truncate(t)
	seedUser(t, 2139, 0)
	seedWalletAccount(t, 2139, 0, 20, 0)
	seedSignedPiggyProfile(t, 2139)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2139, Amount: 10})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusManualReview,
		"piggy_status": model.WithdrawStatusManualReview,
	}).Error)
	require.NoError(t, AdminRecordPiggyManualResult(order.Id, 7, "remote rejected manually", "manual_failed"))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusFailed, refreshed.Status)
	assert.Equal(t, model.WithdrawStatusFailed, refreshed.PiggyStatus)
	assert.Equal(t, piggyCompensationStatusManualProcessed, refreshed.CompensationStatus)
	assert.Equal(t, "remote rejected manually", refreshed.FailReason)
	assert.NotZero(t, refreshed.TerminalAt)

	account, err := model.GetWalletAccountByUserId(2139)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 2139, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
}

func TestPiggyManualResultRejectsNonTerminalOutcome(t *testing.T) {
	truncate(t)
	seedUser(t, 2140, 0)
	seedWalletAccount(t, 2140, 0, 20, 0)
	seedSignedPiggyProfile(t, 2140)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2140, Amount: 10})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusManualReview,
		"piggy_status": model.WithdrawStatusManualReview,
	}).Error)

	require.ErrorIs(t, AdminRecordPiggyManualResult(order.Id, 7, "note only", "manual_processed"), ErrWithdrawStatusInvalid)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.Empty(t, refreshed.CompensationStatus)

	account, err := model.GetWalletAccountByUserId(2140)
	require.NoError(t, err)
	assert.InDelta(t, 10.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
}

func TestPiggyAdminRetryConfirmRequiresAwaitConfirm(t *testing.T) {
	truncate(t)
	seedUser(t, 2118, 0)
	seedWalletAccount(t, 2118, 0, 30, 0)
	seedSignedPiggyProfile(t, 2118)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	mockPiggyClientForTest(t, func(r *http.Request) string {
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2118, Amount: 10})
	require.NoError(t, err)

	require.ErrorIs(t, AdminRetryPiggyConfirm(context.Background(), order.Id), ErrWithdrawStatusInvalid)
}

func TestPiggyConfirmOrderClaimsAwaitConfirmOnce(t *testing.T) {
	truncate(t)
	seedUser(t, 2121, 0)
	seedWalletAccount(t, 2121, 0, 30, 0)
	seedSignedPiggyProfile(t, 2121)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0

	var confirmCalls int32
	confirmAttempts := make(chan struct{}, 2)
	releaseConfirm := make(chan struct{})
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderConfirmPay" {
			atomic.AddInt32(&confirmCalls, 1)
			confirmAttempts <- struct{}{}
			<-releaseConfirm
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2121, Amount: 10})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusAwaitConfirm,
		"piggy_status": model.WithdrawStatusAwaitConfirm,
	}).Error)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- confirmPiggyOrder(context.Background(), order.WithdrawNo)
	}()
	waitForPiggyConfirmAttempt(t, confirmAttempts)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- confirmPiggyOrder(context.Background(), order.WithdrawNo)
	}()

	select {
	case <-confirmAttempts:
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseConfirm)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&confirmCalls))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusConfirmed, refreshed.Status)
}

func TestPiggyCallbackConfirmRaceWithCompensationScanConfirmsOnce(t *testing.T) {
	truncate(t)
	seedUser(t, 2122, 0)
	seedWalletAccount(t, 2122, 0, 30, 0)
	seedSignedPiggyProfile(t, 2122)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0

	var confirmCalls int32
	confirmAttempts := make(chan struct{}, 2)
	releaseConfirm := make(chan struct{})
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderConfirmPay" {
			atomic.AddInt32(&confirmCalls, 1)
			confirmAttempts <- struct{}{}
			<-releaseConfirm
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2122, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()
	body := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo: order.WithdrawNo,
		NotifyType:   "submitResult",
		TradeStatus:  "await",
		FrontLogNo:   "front-race",
	})

	callbackErr := make(chan error, 1)
	go func() {
		callbackErr <- HandlePiggyPaymentCallback(context.Background(), body, "")
	}()
	waitForPiggyConfirmAttempt(t, confirmAttempts)

	scanErr := make(chan error, 1)
	go func() {
		_, err := ScanPiggyWithdrawCompensations(context.Background(), 10)
		scanErr <- err
	}()

	select {
	case <-confirmAttempts:
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseConfirm)

	require.NoError(t, <-callbackErr)
	require.NoError(t, <-scanErr)
	assert.Equal(t, int32(1), atomic.LoadInt32(&confirmCalls))
}

func TestPiggyCompensationScanRecoversExpiredConfirmingOrder(t *testing.T) {
	truncate(t)
	seedUser(t, 2123, 0)
	seedWalletAccount(t, 2123, 0, 30, 0)
	seedSignedPiggyProfile(t, 2123)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().CallbackLockTTL = 1
	var scanOrderNo string
	var confirmCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit", "/open/payment/singleOrderConfirmPay":
			if r.URL.Path == "/open/payment/singleOrderConfirmPay" {
				atomic.AddInt32(&confirmCalls, 1)
			}
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		case "/open/payment/singleOrderQuery":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{"outerTradeNo":"` + scanOrderNo + `","notifyType":"submitResult","tradeStatus":"await","frontLogNo":"front-expired","pretaxAmount":"10.00","afterTaxAmount":"9.50","individualTaxAmount":"0.50","feeAmount":"0.00","calcType":"C"}}`
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2123, Amount: 10})
	require.NoError(t, err)
	scanOrderNo = order.WithdrawNo
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).UpdateColumns(map[string]interface{}{
		"status":       model.WithdrawStatusConfirming,
		"piggy_status": model.WithdrawStatusConfirming,
		"updated_at":   common.GetTimestamp() - 2,
	}).Error)

	processed, err := ScanPiggyWithdrawCompensations(context.Background(), 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, processed, 1)
	assert.Equal(t, int32(1), atomic.LoadInt32(&confirmCalls))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusConfirmed, refreshed.Status)
	assert.Equal(t, int64(950), refreshed.PiggyAfterTaxAmountCents)
}

func TestPiggyAdminCancelRejectsConfirmingOrder(t *testing.T) {
	truncate(t)
	seedUser(t, 2124, 0)
	seedWalletAccount(t, 2124, 0, 30, 0)
	seedSignedPiggyProfile(t, 2124)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	var cancelCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderCancel" {
			atomic.AddInt32(&cancelCalls, 1)
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2124, Amount: 10})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusConfirming,
		"piggy_status": model.WithdrawStatusConfirming,
	}).Error)

	require.ErrorIs(t, AdminCancelPiggyOrder(context.Background(), order.Id, 1, "admin cancel"), ErrWithdrawStatusInvalid)
	assert.Equal(t, int32(0), atomic.LoadInt32(&cancelCalls))
}

func TestPiggyAdminCancelClaimsAwaitConfirmBeforeExternalCancel(t *testing.T) {
	truncate(t)
	seedUser(t, 2125, 0)
	seedWalletAccount(t, 2125, 0, 30, 0)
	seedSignedPiggyProfile(t, 2125)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0

	var orderNo string
	statusDuringCancel := make(chan string, 1)
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderCancel" {
			var order model.WithdrawOrder
			require.NoError(t, model.DB.Where("withdraw_no = ?", orderNo).First(&order).Error)
			statusDuringCancel <- order.Status
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2125, Amount: 10})
	require.NoError(t, err)
	orderNo = order.WithdrawNo
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusAwaitConfirm,
		"piggy_status": model.WithdrawStatusAwaitConfirm,
	}).Error)

	require.NoError(t, AdminCancelPiggyOrder(context.Background(), order.Id, 1, "admin cancel"))
	assert.Equal(t, model.WithdrawStatusCancelling, <-statusDuringCancel)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusCancelled, refreshed.Status)

	account, err := model.GetWalletAccountByUserId(2125)
	require.NoError(t, err)
	assert.InDelta(t, 30.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
}

func TestPiggyCancelClaimPreventsConcurrentConfirm(t *testing.T) {
	truncate(t)
	seedUser(t, 2126, 0)
	seedWalletAccount(t, 2126, 0, 30, 0)
	seedSignedPiggyProfile(t, 2126)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0

	var confirmCalls int32
	cancelAttempts := make(chan struct{}, 1)
	releaseCancel := make(chan struct{})
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderCancel":
			cancelAttempts <- struct{}{}
			<-releaseCancel
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		case "/open/payment/singleOrderConfirmPay":
			atomic.AddInt32(&confirmCalls, 1)
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2126, Amount: 10})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusAwaitConfirm,
		"piggy_status": model.WithdrawStatusAwaitConfirm,
	}).Error)

	cancelErr := make(chan error, 1)
	go func() {
		cancelErr <- AdminCancelPiggyOrder(context.Background(), order.Id, 1, "admin cancel")
	}()
	waitForPiggyCancelAttempt(t, cancelAttempts)

	require.NoError(t, confirmPiggyOrder(context.Background(), order.WithdrawNo))
	assert.Equal(t, int32(0), atomic.LoadInt32(&confirmCalls))

	close(releaseCancel)
	require.NoError(t, <-cancelErr)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusCancelled, refreshed.Status)
	assert.Equal(t, int64(1), countWalletFlows(t, 2126, model.WalletFlowTypeWithdrawReject, order.WithdrawNo))
}

func TestPiggySubmitResultAwaitSkipsCancellingOrder(t *testing.T) {
	truncate(t)
	seedUser(t, 2127, 0)
	seedWalletAccount(t, 2127, 0, 30, 0)
	seedSignedPiggyProfile(t, 2127)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var confirmCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		if r.URL.Path == "/open/payment/singleOrderConfirmPay" {
			atomic.AddInt32(&confirmCalls, 1)
		}
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2127, Amount: 10})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusCancelling,
		"piggy_status": model.WithdrawStatusCancelling,
	}).Error)
	body := buildSignedPiggyPaymentCallback(t, PiggyPaymentCallbackContent{
		OuterTradeNo: order.WithdrawNo,
		NotifyType:   "submitResult",
		TradeStatus:  "await",
		FrontLogNo:   "front-cancelling",
	})

	require.NoError(t, HandlePiggyPaymentCallback(context.Background(), body, ""))
	assert.Equal(t, int32(0), atomic.LoadInt32(&confirmCalls))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusCancelling, refreshed.Status)
}

func TestPiggyCompensationScanMovesExpiredCancellingOrderToManualReview(t *testing.T) {
	truncate(t)
	seedUser(t, 2128, 0)
	seedWalletAccount(t, 2128, 0, 30, 0)
	seedSignedPiggyProfile(t, 2128)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().CallbackLockTTL = 1
	mockPiggyClientForTest(t, func(r *http.Request) string {
		return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2128, Amount: 10})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).UpdateColumns(map[string]interface{}{
		"status":       model.WithdrawStatusCancelling,
		"piggy_status": model.WithdrawStatusCancelling,
		"updated_at":   common.GetTimestamp() - 2,
	}).Error)

	processed, err := ScanPiggyWithdrawCompensations(context.Background(), 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, processed, 1)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusManualReview, refreshed.Status)
	assert.Contains(t, refreshed.ManualReviewReason, "取消结果")

	account, err := model.GetWalletAccountByUserId(2128)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 10.0, account.FrozenCommissionAmount, 0.000001)
}

func TestPiggyCompensationScanSkipsManualProcessedRowsInQueryLimit(t *testing.T) {
	truncate(t)
	seedUser(t, 2149, 0)
	seedWalletAccount(t, 2149, 0, 40, 0)
	seedSignedPiggyProfile(t, 2149)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var actionableOrderNo string
	var queryCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderQuery":
			atomic.AddInt32(&queryCalls, 1)
			return `{"code":"0","msg":"success","isSuccess":true,"data":{"outerTradeNo":"` + actionableOrderNo + `","notifyType":"tradeResult","tradeStatus":"success","frontLogNo":"front-manual-scan","laborOrderNo":"labor-manual-scan","pretaxAmount":"5.00","afterTaxAmount":"4.80","individualTaxAmount":"0.20","feeAmount":"0.00","calcType":"C"}}`
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	for i := 0; i < 3; i++ {
		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2149, Amount: 5})
		require.NoError(t, err)
		require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
			"status":              model.WithdrawStatusManualReview,
			"piggy_status":        model.WithdrawStatusManualReview,
			"compensation_status": piggyCompensationStatusManualProcessed,
		}).Error)
	}
	actionableOrder, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2149, Amount: 5})
	require.NoError(t, err)
	actionableOrderNo = actionableOrder.WithdrawNo
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", actionableOrder.Id).Updates(map[string]interface{}{
		"status":              model.WithdrawStatusManualReview,
		"piggy_status":        model.WithdrawStatusManualReview,
		"compensation_status": piggyCompensationStatusPending,
	}).Error)

	processed, err := ScanPiggyWithdrawCompensations(context.Background(), 3)
	require.NoError(t, err)
	assert.Equal(t, 1, processed)
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", actionableOrder.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, int64(1), countWalletFlows(t, 2149, model.WalletFlowTypeWithdrawSuccess, actionableOrder.WithdrawNo))
}

func TestPiggyCompensationScanSkipsFreshApprovedRowsInQueryLimit(t *testing.T) {
	truncate(t)
	seedUser(t, 2185, 0)
	seedWalletAccount(t, 2185, 0, 40, 0)
	seedSignedPiggyProfile(t, 2185)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	operation_setting.GetPiggyWithdrawSetting().RequestTimeout = 5
	var actionableOrderNo string
	var queryCalls int32
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderQuery":
			atomic.AddInt32(&queryCalls, 1)
			return buildPiggyEncryptedQueryResponseForTest(t, piggyCallbackContentFromOrderForTest(&model.WithdrawOrder{
				WithdrawNo:        actionableOrderNo,
				AccountName:       "张三",
				PayoutMobile:      "13812345678",
				PayoutIdCardNo:    "110101199001011234",
				PayoutBankCardNo:  "6222000011118888",
				BankName:          "招商银行",
				PositionName:      operation_setting.GetPiggyWithdrawSetting().PositionName,
				FrozenAmountCents: 500,
				CalcType:          "C",
			}, "", "success"))
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	for i := 0; i < 3; i++ {
		order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2185, Amount: 5})
		require.NoError(t, err)
		require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
			"status":              model.WithdrawStatusApproved,
			"piggy_status":        model.WithdrawStatusApproved,
			"reviewed_at":         common.GetTimestamp(),
			"updated_at":          common.GetTimestamp(),
			"compensation_status": piggyCompensationStatusPending,
		}).Error)
	}

	actionableOrder, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2185, Amount: 5})
	require.NoError(t, err)
	actionableOrderNo = actionableOrder.WithdrawNo
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", actionableOrder.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusConfirmed,
		"piggy_status": model.WithdrawStatusConfirmed,
	}).Error)

	processed, err := ScanPiggyWithdrawCompensations(context.Background(), 3)
	require.NoError(t, err)
	assert.Equal(t, 1, processed)
	assert.Equal(t, int32(1), atomic.LoadInt32(&queryCalls))

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", actionableOrder.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, int64(1), countWalletFlows(t, 2185, model.WalletFlowTypeWithdrawSuccess, actionableOrder.WithdrawNo))
}

func TestPiggyCompensationScanAppliesQueryAwaitStatus(t *testing.T) {
	truncate(t)
	seedUser(t, 2119, 0)
	seedWalletAccount(t, 2119, 0, 30, 0)
	seedSignedPiggyProfile(t, 2119)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var scanOrderNo string
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit", "/open/payment/singleOrderConfirmPay":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		case "/open/payment/singleOrderQuery":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{"outerTradeNo":"` + scanOrderNo + `","notifyType":"submitResult","tradeStatus":"await","frontLogNo":"front-scan","pretaxAmount":"10.00","afterTaxAmount":"9.30","individualTaxAmount":"0.70","feeAmount":"0.00","calcType":"C"}}`
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2119, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()
	scanOrderNo = order.WithdrawNo

	processed, err := ScanPiggyWithdrawCompensations(context.Background(), 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, processed, 1)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusConfirmed, refreshed.Status)
	assert.Equal(t, int64(930), refreshed.PiggyAfterTaxAmountCents)
}

func TestPiggyCompensationScanAppliesLegacyPlaintextQueryAwaitWithoutNotifyType(t *testing.T) {
	truncate(t)
	seedUser(t, 2190, 0)
	seedWalletAccount(t, 2190, 0, 30, 0)
	seedSignedPiggyProfile(t, 2190)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	operation_setting.GetPiggyWithdrawSetting().CooldownMinutes = 0
	var scanOrderNo string
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit", "/open/payment/singleOrderConfirmPay":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		case "/open/payment/singleOrderQuery":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{"outerTradeNo":"` + scanOrderNo + `","tradeStatus":"await","frontLogNo":"front-legacy-scan","pretaxAmount":"10.00","afterTaxAmount":"9.30","individualTaxAmount":"0.70","feeAmount":"0.00","calcType":"C"}}`
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2190, Amount: 10})
	require.NoError(t, err)
	order = func() *model.WithdrawOrder {
		refreshed := approvePiggyWithdrawForTest(t, order)
		return &refreshed
	}()
	scanOrderNo = order.WithdrawNo

	processed, err := ScanPiggyWithdrawCompensations(context.Background(), 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, processed, 1)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusConfirmed, refreshed.Status)
	assert.Equal(t, int64(930), refreshed.PiggyAfterTaxAmountCents)
}

func TestPiggyCompensationScanFinalizesConfirmedQueryResult(t *testing.T) {
	truncate(t)
	seedUser(t, 2120, 0)
	seedWalletAccount(t, 2120, 0, 30, 0)
	seedSignedPiggyProfile(t, 2120)
	configurePiggyWithdrawForTest(t, "https://piggy.example.com")
	var scanOrderNo string
	mockPiggyClientForTest(t, func(r *http.Request) string {
		switch r.URL.Path {
		case "/open/payment/singleOrderSubmit":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		case "/open/payment/singleOrderQuery":
			return `{"code":"0","msg":"success","isSuccess":true,"data":{"outerTradeNo":"` + scanOrderNo + `","notifyType":"tradeResult","tradeStatus":"success","frontLogNo":"front-final","laborOrderNo":"labor-final","pretaxAmount":"10.00","afterTaxAmount":"9.10","individualTaxAmount":"0.90","feeAmount":"0.00","calcType":"C"}}`
		default:
			return `{"code":"0","msg":"success","isSuccess":true,"data":{}}`
		}
	})

	order, err := SubmitPiggyWithdrawOrder(context.Background(), PiggyWithdrawSubmitRequest{UserId: 2120, Amount: 10})
	require.NoError(t, err)
	scanOrderNo = order.WithdrawNo
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"status":       model.WithdrawStatusConfirmed,
		"piggy_status": model.WithdrawStatusConfirmed,
	}).Error)

	processed, err := ScanPiggyWithdrawCompensations(context.Background(), 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, processed, 1)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, int64(910), refreshed.PiggyAfterTaxAmountCents)
	assert.Equal(t, int64(1), countWalletFlows(t, 2120, model.WalletFlowTypeWithdrawSuccess, order.WithdrawNo))
}
