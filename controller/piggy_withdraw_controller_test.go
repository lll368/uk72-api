package controller

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type piggyControllerResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

func setupPiggyWithdrawControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldUsingSQLite := common.UsingSQLite
	oldRedisEnabled := common.RedisEnabled
	oldPiggySetting := *operation_setting.GetPiggyWithdrawSetting()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	common.UsingSQLite = true
	common.RedisEnabled = false
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.UserProfile{},
		&model.WalletAccount{},
		&model.WalletFlow{},
		&model.WithdrawalProfile{},
		&model.WithdrawOrder{},
		&model.PiggyWithdrawCallbackLog{},
	))

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		*operation_setting.GetPiggyWithdrawSetting() = oldPiggySetting
		_ = sqlDB.Close()
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 2201)
		c.Next()
	})
	router.PUT("/api/wallet/withdraw/profile", SaveWalletWithdrawalProfile)
	router.GET("/api/wallet/withdraw/profile", GetWalletWithdrawalProfile)
	router.GET("/api/wallet/withdraw/eligibility", GetWalletWithdrawalEligibility)
	router.POST("/api/wallet/withdraw/piggy/contract-preview", GetWalletPiggyContractPreview)
	router.POST("/api/wallet/withdraw/piggy/tax-trial", TrialWalletPiggyWithdrawTax)
	router.POST("/api/wallet/withdraw/piggy/payment/notify", PiggyPaymentNotify)
	router.POST("/api/wallet/admin/withdraws/:id/approve", AdminApproveWalletWithdraw)
	router.POST("/api/wallet/admin/withdraws/:id/piggy/recover-submit", AdminRecoverPiggyWithdrawSubmit)
	return router
}

func buildControllerUnsignedPiggyPaymentCallback(t *testing.T, content service.PiggyPaymentCallbackContent) []byte {
	t.Helper()
	setting := operation_setting.GetPiggyWithdrawSetting()
	plain, err := common.Marshal(content)
	require.NoError(t, err)
	encrypted, err := controllerPiggyEncryptAES(plain, setting.AppSecret, setting.AESIV)
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

func controllerPiggyEncryptAES(plainText []byte, appSecret string, iv string) (string, error) {
	block, err := aes.NewCipher([]byte(appSecret))
	if err != nil {
		return "", err
	}
	padding := block.BlockSize() - len(plainText)%block.BlockSize()
	padded := append(append([]byte{}, plainText...), bytes.Repeat([]byte{byte(padding)}, padding)...)
	cipherText := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(iv)).CryptBlocks(cipherText, padded)
	return url.QueryEscape(base64.StdEncoding.EncodeToString(cipherText)), nil
}

func TestPiggyWithdrawProfileControllerMasksSensitiveFields(t *testing.T) {
	router := setupPiggyWithdrawControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 2201, Username: "piggy_profile_user", Status: common.UserStatusEnabled}).Error)
	phone := "+8613812345678"
	require.NoError(t, model.DB.Create(&model.UserProfile{
		UserId:          2201,
		PhoneNumber:     &phone,
		PhoneVerifiedAt: common.GetTimestamp(),
	}).Error)

	body := `{"account_type":"bankcard","real_name":"张三","id_card_no":"110101199001011234","mobile":"13812345678","bank_card_no":"6222000011118888","bank_name":"招商银行"}`
	req := httptest.NewRequest(http.MethodPut, "/api/wallet/withdraw/profile", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp piggyControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	assert.Equal(t, "110************234", resp.Data["masked_id_card_no"])
	assert.Equal(t, "138****5678", resp.Data["masked_mobile"])
	assert.Equal(t, "6222********8888", resp.Data["masked_bank_card_no"])
	_, hasRawID := resp.Data["id_card_no"]
	assert.False(t, hasRawID)
}

func TestPiggyWithdrawEligibilityControllerReturnsBlockingReasons(t *testing.T) {
	router := setupPiggyWithdrawControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 2201, Username: "piggy_eligibility_user", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.WalletAccount{UserId: 2201, CommissionAmount: 20}).Error)
	setting := operation_setting.GetPiggyWithdrawSetting()
	*setting = operation_setting.PiggyWithdrawSetting{Enabled: false}

	req := httptest.NewRequest(http.MethodGet, "/api/wallet/withdraw/eligibility", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp piggyControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	assert.Equal(t, false, resp.Data["can_withdraw"])
	assert.NotEmpty(t, resp.Data["blocking_reasons"])
}

func TestPiggyTaxTrialControllerReturnsEstimatedAmountsWithoutCreatingOrder(t *testing.T) {
	var capturedPayload map[string]any
	piggyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/open/payment/singleTaxTrialCalc", req.URL.Path)
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &capturedPayload))
		require.NotEmpty(t, capturedPayload["sign"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code":"0",
			"msg":"success",
			"isSuccess":"T",
			"data":{
				"outerTradeNo":"PTRIAL2201",
				"calcMonth":"2026-06",
				"pretaxAmount":92,
				"individualTaxAmount":3.5,
				"addedTaxAmount":1.06,
				"afterTaxAmount":87.44
			}
		}`))
	}))
	defer piggyServer.Close()
	router := setupPiggyWithdrawControllerTest(t)
	paymentSetting := operation_setting.GetPaymentSetting()
	oldPaymentSetting := *paymentSetting
	t.Cleanup(func() { *paymentSetting = oldPaymentSetting })
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	setting := operation_setting.GetPiggyWithdrawSetting()
	*setting = operation_setting.PiggyWithdrawSetting{
		Enabled:         true,
		Domain:          piggyServer.URL,
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
		CalcType:        "C",
		PlatformFeeRate: 8,
	}
	require.NoError(t, model.DB.Create(&model.User{Id: 2201, Username: "piggy_trial_user", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.WalletAccount{UserId: 2201, CommissionAmount: 120}).Error)
	require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
		UserId:          2201,
		AccountType:     model.WithdrawAccountTypeBankcard,
		RealName:        "张三",
		IdCardNo:        "110101199001011234",
		Mobile:          "13812345678",
		BankCardNo:      "6222000011118888",
		BankName:        "招商银行",
		PiggySignStatus: model.PiggySignStatusSigned,
	}).Error)

	req := httptest.NewRequest(http.MethodPost, "/api/wallet/withdraw/piggy/tax-trial", bytes.NewBufferString(`{"amount":100}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp piggyControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.NotNil(t, capturedPayload)
	assert.Equal(t, "app-key", capturedPayload["appKey"])
	assert.Equal(t, "utf-8", capturedPayload["charset"])
	assert.Equal(t, "3.0", capturedPayload["version"])
	assert.Equal(t, "tax-fund", capturedPayload["taxFundId"])
	assert.Equal(t, "110101199001011234", capturedPayload["licenseId"])
	assert.Equal(t, "92.00", capturedPayload["calcAmount"])
	assert.Equal(t, "C", capturedPayload["calcType"])
	assert.True(t, strings.HasPrefix(fmt.Sprint(capturedPayload["outerTradeNo"]), "PTRIAL2201"))
	assert.Equal(t, "100.00", resp.Data["requested_amount"])
	assert.Equal(t, float64(10000), resp.Data["requested_amount_cents"])
	assert.Equal(t, float64(8), resp.Data["platform_fee_rate"])
	assert.Equal(t, "8.00", resp.Data["platform_fee_amount"])
	assert.Equal(t, float64(800), resp.Data["platform_fee_amount_cents"])
	assert.Equal(t, "92.00", resp.Data["piggy_tax_before_amount"])
	assert.Equal(t, float64(9200), resp.Data["piggy_tax_before_amount_cents"])
	assert.Equal(t, "92.00", resp.Data["pretax_amount"])
	assert.Equal(t, "3.50", resp.Data["individual_tax_amount"])
	assert.Equal(t, "1.06", resp.Data["added_tax_amount"])
	assert.Equal(t, "87.44", resp.Data["after_tax_amount"])
	assert.Equal(t, "C", resp.Data["calc_type"])
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Count(&orderCount).Error)
	assert.Equal(t, int64(0), orderCount)
}

func TestPiggyContractPreviewControllerReturnsPreviewURL(t *testing.T) {
	piggyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/contract/sign/viewContract", req.URL.Path)
		assert.Equal(t, "DOC-2201", req.URL.Query().Get("documentId"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":"https://preview.example.com/contracts/DOC-2201"}`))
	}))
	t.Cleanup(piggyServer.Close)

	router := setupPiggyWithdrawControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 2201, Username: "piggy_preview_user", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.WithdrawalProfile{
		UserId:                      2201,
		AccountType:                 model.WithdrawAccountTypeBankcard,
		RealName:                    "张三",
		IdCardNo:                    "110101199001011234",
		Mobile:                      "13812345678",
		BankCardNo:                  "6222000011118888",
		BankName:                    "招商银行",
		PiggySignStatus:             model.PiggySignStatusSigned,
		PiggySignedAt:               common.GetTimestamp(),
		PiggyContractDocumentID:     "DOC-2201",
		PiggyContractPosition:       "技术服务",
		PiggyContractPositionName:   "技术服务",
		PiggyContractTaxFundID:      "tax-fund",
		PiggyContractSubsidiaryName: "小猪签约结算公司",
	}).Error)
	setting := operation_setting.GetPiggyWithdrawSetting()
	*setting = operation_setting.PiggyWithdrawSetting{
		Enabled:         true,
		Domain:          piggyServer.URL,
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
	}

	req := httptest.NewRequest(http.MethodPost, "/api/wallet/withdraw/piggy/contract-preview", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp piggyControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	assert.Equal(t, "DOC-2201", resp.Data["document_id"])
	assert.Equal(t, "https://preview.example.com/contracts/DOC-2201", resp.Data["preview_url"])
}

func TestPiggyPaymentNotifyReturnsPlainTextSuccess(t *testing.T) {
	router := setupPiggyWithdrawControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 2201, Username: "piggy_payment_notify_user", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.WalletAccount{UserId: 2201, CommissionAmount: 20, FrozenCommissionAmount: 10}).Error)
	setting := operation_setting.GetPiggyWithdrawSetting()
	*setting = operation_setting.PiggyWithdrawSetting{
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
		CooldownMinutes: 0,
		CalcType:        "C",
	}
	order := model.WithdrawOrder{
		UserId:               2201,
		WithdrawNo:           "PWDR2201NOTIFY",
		Amount:               10,
		Status:               model.WithdrawStatusSubmitted,
		Provider:             model.WithdrawProviderPiggyLaborV3,
		PiggyStatus:          model.WithdrawStatusSubmitted,
		ReceiveType:          model.WithdrawAccountTypeBankcard,
		ReceiveAccount:       "6222********8888",
		AccountName:          "张三",
		BankName:             "招商银行",
		PayoutMobile:         "13812345678",
		PayoutIdCardNo:       "110101199001011234",
		PayoutBankCardNo:     "6222000011118888",
		TaxBeforeAmountCents: 1000,
		FrozenAmountCents:    1000,
		PiggyPayAmountCents:  1000,
		PiggyPayAmount:       "10.00",
		TaxFundId:            "tax-fund",
		PositionName:         "技术服务",
		Position:             "tech",
		CalcType:             "C",
		ExternalTradeNo:      "PWDR2201NOTIFY",
		SubmittedAt:          common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(&order).Error)
	body := buildControllerUnsignedPiggyPaymentCallback(t, service.PiggyPaymentCallbackContent{
		OuterTradeNo:        order.WithdrawNo,
		NotifyType:          "tradeResult",
		TradeStatus:         "success",
		FrontLogNo:          "front-notify",
		LaborOrderNo:        "labor-notify",
		EmpName:             "张三",
		EmpPhone:            "13812345678",
		LicenseType:         "ID_CARD",
		LicenseId:           "110101199001011234",
		SettleType:          model.WithdrawAccountTypeBankcard,
		PayAccount:          "6222000011118888",
		BankName:            "招商银行",
		PositionName:        "技术服务",
		PretaxAmount:        "10.00",
		IndividualTaxAmount: "0.50",
		AfterTaxAmount:      "9.50",
		FeeAmount:           "0.00",
		CalcType:            "C",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/wallet/withdraw/piggy/payment/notify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "success", recorder.Body.String())
}

func TestAdminApprovePiggyWithdrawNetworkErrorReturnsRecoverableWarning(t *testing.T) {
	router := setupPiggyWithdrawControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 2201, Username: "piggy_admin_approve_user", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.WalletAccount{UserId: 2201, CommissionAmount: 20, FrozenCommissionAmount: 10}).Error)
	setting := operation_setting.GetPiggyWithdrawSetting()
	*setting = operation_setting.PiggyWithdrawSetting{
		Enabled:         true,
		Domain:          "http://127.0.0.1:1",
		AppKey:          "app-key",
		AppSecret:       "1234567890abcdef",
		AESIV:           "0000000000000000",
		TaxFundId:       "tax-fund",
		PositionName:    "技术服务",
		Position:        "tech",
		SignNotifyUrl:   "https://app.example.com/api/withdraw/piggy/contract/notify",
		PayNotifyUrl:    "https://app.example.com/api/withdraw/piggy/payment/notify",
		RequestTimeout:  1,
		CallbackLockTTL: 60,
		CooldownMinutes: 0,
		CalcType:        "C",
	}
	order := model.WithdrawOrder{
		UserId:               2201,
		WithdrawNo:           "PWDR2201NETWORK",
		Amount:               10,
		Status:               model.WithdrawStatusPending,
		Provider:             model.WithdrawProviderPiggyLaborV3,
		PiggyStatus:          model.WithdrawStatusPending,
		ReceiveType:          model.WithdrawAccountTypeBankcard,
		ReceiveAccount:       "6222********8888",
		AccountName:          "张三",
		BankName:             "招商银行",
		PayoutMobile:         "13812345678",
		PayoutIdCardNo:       "110101199001011234",
		PayoutBankCardNo:     "6222000011118888",
		TaxBeforeAmountCents: 1000,
		FrozenAmountCents:    1000,
		PiggyPayAmountCents:  1000,
		PiggyPayAmount:       "10.00",
		TaxFundId:            "tax-fund",
		PositionName:         "技术服务",
		Position:             "tech",
		CalcType:             "C",
	}
	require.NoError(t, model.DB.Create(&order).Error)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/wallet/admin/withdraws/%d/approve", order.Id), bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp piggyControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	assert.Equal(t, false, resp.Data["submitted"])
	assert.Equal(t, true, resp.Data["recoverable"])
	assert.Equal(t, model.WithdrawStatusApproved, resp.Data["status"])
	assert.Contains(t, fmt.Sprint(resp.Data["message"]), "小猪提交结果未知")

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.Where("id = ?", order.Id).First(&refreshed).Error)
	assert.Equal(t, model.WithdrawStatusApproved, refreshed.Status)
	assert.Empty(t, refreshed.ExternalTradeNo)
	assert.Contains(t, refreshed.ManualReviewReason, "可使用原流水号恢复提交")
}

func TestAdminRecoverPiggyWithdrawNetworkErrorReturnsRecoverableWarning(t *testing.T) {
	router := setupPiggyWithdrawControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 2201, Username: "piggy_admin_recover_user", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.WalletAccount{UserId: 2201, CommissionAmount: 20, FrozenCommissionAmount: 10}).Error)
	setting := operation_setting.GetPiggyWithdrawSetting()
	*setting = operation_setting.PiggyWithdrawSetting{
		Enabled:         true,
		Domain:          "http://127.0.0.1:1",
		AppKey:          "app-key",
		AppSecret:       "1234567890abcdef",
		AESIV:           "0000000000000000",
		TaxFundId:       "tax-fund",
		PositionName:    "技术服务",
		Position:        "tech",
		SignNotifyUrl:   "https://app.example.com/api/withdraw/piggy/contract/notify",
		PayNotifyUrl:    "https://app.example.com/api/withdraw/piggy/payment/notify",
		RequestTimeout:  1,
		CallbackLockTTL: 60,
		CooldownMinutes: 0,
		CalcType:        "C",
	}
	staleAt := common.GetTimestamp() - 120
	order := model.WithdrawOrder{
		UserId:               2201,
		WithdrawNo:           "PWDR2201RECOVER",
		Amount:               10,
		Status:               model.WithdrawStatusApproved,
		Provider:             model.WithdrawProviderPiggyLaborV3,
		PiggyStatus:          model.WithdrawStatusApproved,
		ReceiveType:          model.WithdrawAccountTypeBankcard,
		ReceiveAccount:       "6222********8888",
		AccountName:          "张三",
		BankName:             "招商银行",
		PayoutMobile:         "13812345678",
		PayoutIdCardNo:       "110101199001011234",
		PayoutBankCardNo:     "6222000011118888",
		TaxBeforeAmountCents: 1000,
		FrozenAmountCents:    1000,
		PiggyPayAmountCents:  1000,
		PiggyPayAmount:       "10.00",
		TaxFundId:            "tax-fund",
		PositionName:         "技术服务",
		Position:             "tech",
		CalcType:             "C",
		ReviewedAt:           staleAt,
		UpdatedAt:            staleAt,
		CompensationStatus:   "pending_compensation",
	}
	require.NoError(t, model.DB.Create(&order).Error)
	require.NoError(t, model.DB.Model(&model.WithdrawOrder{}).Where("id = ?", order.Id).Updates(map[string]interface{}{
		"reviewed_at": staleAt,
		"updated_at":  staleAt,
	}).Error)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/wallet/admin/withdraws/%d/piggy/recover-submit", order.Id), bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp piggyControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	assert.Equal(t, false, resp.Data["submitted"])
	assert.Equal(t, true, resp.Data["recoverable"])
	assert.Equal(t, model.WithdrawStatusApproved, resp.Data["status"])
	assert.Contains(t, fmt.Sprint(resp.Data["message"]), "小猪提交结果未知")
}
