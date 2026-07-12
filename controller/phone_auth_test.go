package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPhoneAuthControllerTest(t *testing.T) (*gin.Engine, *service.FakeSmsSender) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.RedisEnabled = false
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordLoginEnabled = true

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.PhoneVerificationCode{},
		&model.UserProfile{},
		&model.VipActivationRecord{},
		&model.UserRelation{},
		&model.TwoFA{},
	))

	fakeSender := service.NewFakeSmsSender()
	service.SetSmsSender(fakeSender)
	t.Cleanup(func() {
		service.SetSmsSender(nil)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	router := gin.New()
	router.Use(sessions.Sessions("phone-auth-test", cookie.NewStore([]byte("phone-auth-secret"))))
	router.POST("/api/phone/verification", SendPhoneVerification)
	router.POST("/api/user/register", Register)
	router.POST("/api/user/register/phone", RegisterByPhone)
	router.POST("/api/user/login", Login)
	router.POST("/api/user/login/email", LoginByEmail)
	router.POST("/api/user/login/phone", LoginByPhone)
	router.POST("/api/user/reset", ResetPassword)
	router.POST("/api/user/reset/verify", VerifyPasswordReset)
	router.POST("/api/user/reset/confirm", ConfirmPasswordReset)
	router.POST("/api/user/reset_password/phone", ResetPasswordByPhone)
	return router, fakeSender
}

func performPhoneAuthRequest(router *gin.Engine, path string, body string) tokenAPIResponse {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return decodeTokenAPIResponse(recorder)
}

func decodeTokenAPIResponse(recorder *httptest.ResponseRecorder) tokenAPIResponse {
	var resp tokenAPIResponse
	_ = common.Unmarshal(recorder.Body.Bytes(), &resp)
	return resp
}

func TestPhoneAuthControllerRegisterLoginAndResetPassword(t *testing.T) {
	router, fakeSender := setupPhoneAuthControllerTest(t)

	verifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138000","purpose":"register"}`)
	require.True(t, verifyResp.Success, verifyResp.Message)
	require.Len(t, fakeSender.Messages(), 1)
	registerCode := fakeSender.Messages()[0].Code

	registerResp := performPhoneAuthRequest(router, "/api/user/register/phone", fmt.Sprintf(`{"username":"phoneuser","password":"password123","phone_number":"13800138000","verification_code":%q}`, registerCode))
	require.True(t, registerResp.Success, registerResp.Message)

	phone := "+8613800138000"
	profile, err := model.GetUserProfileByPhoneNumber(phone)
	require.NoError(t, err)
	assert.NotZero(t, profile.UserId)
	assert.Equal(t, phone, *profile.PhoneNumber)

	loginVerifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"+8613800138000","purpose":"login"}`)
	require.True(t, loginVerifyResp.Success, loginVerifyResp.Message)
	require.Len(t, fakeSender.Messages(), 2)
	loginCode := fakeSender.Messages()[1].Code

	loginResp := performPhoneAuthRequest(router, "/api/user/login/phone", fmt.Sprintf(`{"phone_number":"+8613800138000","verification_code":%q}`, loginCode))
	require.True(t, loginResp.Success, loginResp.Message)

	passwordLoginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"+8613800138000","password":"password123"}`)
	require.True(t, passwordLoginResp.Success, passwordLoginResp.Message)

	resetVerifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"+8613800138000","purpose":"reset_password"}`)
	require.True(t, resetVerifyResp.Success, resetVerifyResp.Message)
	require.Len(t, fakeSender.Messages(), 3)
	resetCode := fakeSender.Messages()[2].Code

	resetResp := performPhoneAuthRequest(router, "/api/user/reset_password/phone", fmt.Sprintf(`{"phone_number":"+8613800138000","verification_code":%q,"password":"newpassword123"}`, resetCode))
	require.True(t, resetResp.Success, resetResp.Message)
	require.Empty(t, resetResp.Data)

	newPasswordLoginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"+8613800138000","password":"newpassword123"}`)
	require.True(t, newPasswordLoginResp.Success, newPasswordLoginResp.Message)
}

func TestEmailRegisterStoresEmailAsUsernameAndSupportsPasswordAndCodeLogin(t *testing.T) {
	router, _ := setupPhoneAuthControllerTest(t)
	common.EmailVerificationEnabled = true
	t.Cleanup(func() {
		common.EmailVerificationEnabled = false
	})

	email := "person.with.long.email@example.com"
	code := "123456"
	common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose)

	registerResp := performPhoneAuthRequest(router, "/api/user/register", fmt.Sprintf(`{"password":"password123","email":%q,"verification_code":%q}`, email, code))
	require.True(t, registerResp.Success, registerResp.Message)

	var user model.User
	require.NoError(t, model.DB.Where("email = ?", email).First(&user).Error)
	assert.Equal(t, email, user.Username)
	assert.Equal(t, email, user.Email)
	assert.LessOrEqual(t, len(user.DisplayName), model.UserDisplayNameMaxLength)

	passwordLoginResp := performPhoneAuthRequest(router, "/api/user/login", fmt.Sprintf(`{"username":%q,"password":"password123"}`, email))
	require.True(t, passwordLoginResp.Success, passwordLoginResp.Message)

	loginCode := "654321"
	common.RegisterVerificationCodeWithKey(email, loginCode, common.EmailLoginPurpose)
	codeLoginResp := performPhoneAuthRequest(router, "/api/user/login/email", fmt.Sprintf(`{"email":%q,"verification_code":%q}`, email, loginCode))
	require.True(t, codeLoginResp.Success, codeLoginResp.Message)
}

func TestEmailRegisterWithUsernameEmailAutomaticallyBindsEmail(t *testing.T) {
	router, _ := setupPhoneAuthControllerTest(t)

	email := "username-email-register@example.com"
	code := "135790"
	common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose)

	registerResp := performPhoneAuthRequest(router, "/api/user/register", fmt.Sprintf(`{"username":" Username-Email-Register@Example.com ","password":"password123","verification_code":%q}`, code))
	require.True(t, registerResp.Success, registerResp.Message)

	var user model.User
	require.NoError(t, model.DB.Where("email = ?", email).First(&user).Error)
	assert.Equal(t, email, user.Username)
	assert.Equal(t, email, user.Email)

	selfReq := httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	selfRecorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(selfRecorder)
	ctx.Request = selfReq
	ctx.Set("id", user.Id)
	ctx.Set("role", common.RoleCommonUser)
	GetSelf(ctx)

	selfResp := decodeTokenAPIResponse(selfRecorder)
	require.True(t, selfResp.Success, selfResp.Message)
	var selfData struct {
		Email string `json:"email"`
	}
	require.NoError(t, common.Unmarshal(selfResp.Data, &selfData))
	assert.Equal(t, email, selfData.Email)
}

func TestEmailAccountLookupIsCaseInsensitive(t *testing.T) {
	router, _ := setupPhoneAuthControllerTest(t)
	originalUsingMySQL := common.UsingMySQL
	common.UsingMySQL = true
	t.Cleanup(func() {
		common.UsingMySQL = originalUsingMySQL
	})

	user := &model.User{
		Username:    "mixed-case-email",
		Password:    "password123",
		DisplayName: "mixed-email",
		Email:       "Person.With.Case@Example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, user.Insert(0))

	assert.True(t, model.IsEmailAccountAlreadyTaken("person.with.case@example.com"))
	found, err := model.GetUserByEmailAccount("person.with.case@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.Username, found.Username)

	loginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"PERSON.WITH.CASE@EXAMPLE.COM","password":"password123"}`)
	require.True(t, loginResp.Success, loginResp.Message)

	duplicateCode := "112244"
	common.RegisterVerificationCodeWithKey("person.with.case@example.com", duplicateCode, common.EmailVerificationPurpose)
	duplicateResp := performPhoneAuthRequest(router, "/api/user/register", fmt.Sprintf(`{"email":"PERSON.WITH.CASE@EXAMPLE.COM","password":"password123","verification_code":%q}`, duplicateCode))
	require.False(t, duplicateResp.Success)
}

func TestPhoneRegisterStoresNormalizedPhoneAsUsername(t *testing.T) {
	router, fakeSender := setupPhoneAuthControllerTest(t)

	verifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138005","purpose":"register"}`)
	require.True(t, verifyResp.Success, verifyResp.Message)
	require.Len(t, fakeSender.Messages(), 1)

	registerCode := fakeSender.Messages()[0].Code
	registerResp := performPhoneAuthRequest(router, "/api/user/register/phone", fmt.Sprintf(`{"password":"password123","phone_number":"13800138005","verification_code":%q}`, registerCode))
	require.True(t, registerResp.Success, registerResp.Message)

	registered, err := model.GetUserByPhoneNumber("+8613800138005")
	require.NoError(t, err)
	assert.Equal(t, "+8613800138005", registered.Username)
	assert.LessOrEqual(t, len(registered.DisplayName), model.UserDisplayNameMaxLength)

	passwordLoginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"+8613800138005","password":"password123"}`)
	require.True(t, passwordLoginResp.Success, passwordLoginResp.Message)
}

func TestPasswordLoginKeepsLegacyUsernameFallback(t *testing.T) {
	router, _ := setupPhoneAuthControllerTest(t)

	legacyUser := &model.User{
		Username:    "legacy_user",
		Password:    "password123",
		DisplayName: "legacy_user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, legacyUser.Insert(0))

	loginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"legacy_user","password":"password123"}`)
	require.True(t, loginResp.Success, loginResp.Message)
}

func TestPasswordLoginKeepsLegacyNumericUsernameFallback(t *testing.T) {
	router, _ := setupPhoneAuthControllerTest(t)

	legacyUser := &model.User{
		Username:    "13800138008",
		Password:    "password123",
		DisplayName: "numeric_legacy",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, legacyUser.Insert(0))

	loginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"13800138008","password":"password123"}`)
	require.True(t, loginResp.Success, loginResp.Message)
}

func TestPhoneRegisterRejectsLegacyNumericUsernameCollision(t *testing.T) {
	router, _ := setupPhoneAuthControllerTest(t)

	legacyUser := &model.User{
		Username:    "13800138009",
		Password:    "password123",
		DisplayName: "numeric_legacy",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, legacyUser.Insert(0))

	verifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138009","purpose":"register"}`)
	require.False(t, verifyResp.Success)

	loginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"13800138009","password":"password123"}`)
	require.True(t, loginResp.Success, loginResp.Message)
}

func TestPasswordLoginFallsBackToLegacyNumericUsernameWhenPhoneProfileCollides(t *testing.T) {
	router, _ := setupPhoneAuthControllerTest(t)

	legacyUser := &model.User{
		Username:    "13800138010",
		Password:    "legacy123",
		DisplayName: "numeric_legacy",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, legacyUser.Insert(0))

	phoneUser := &model.User{
		Username:    "+8613800138010",
		Password:    "phonepass123",
		DisplayName: "phone_user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, phoneUser.Insert(0))
	phoneNumber := "+8613800138010"
	require.NoError(t, model.DB.Create(&model.UserProfile{
		UserId:      phoneUser.Id,
		PhoneNumber: &phoneNumber,
	}).Error)

	legacyLoginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"13800138010","password":"legacy123"}`)
	require.True(t, legacyLoginResp.Success, legacyLoginResp.Message)

	phoneLoginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"13800138010","password":"phonepass123"}`)
	require.True(t, phoneLoginResp.Success, phoneLoginResp.Message)
}

func TestRegisterAndPasswordResetRejectWeakPasswords(t *testing.T) {
	router, fakeSender := setupPhoneAuthControllerTest(t)
	common.EmailVerificationEnabled = true
	t.Cleanup(func() {
		common.EmailVerificationEnabled = false
	})

	email := "weak-password@example.com"
	code := "123456"
	common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose)
	registerResp := performPhoneAuthRequest(router, "/api/user/register", fmt.Sprintf(`{"password":"abcdefgh","email":%q,"verification_code":%q}`, email, code))
	require.False(t, registerResp.Success)

	verifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138006","purpose":"register"}`)
	require.True(t, verifyResp.Success, verifyResp.Message)
	registerCode := fakeSender.Messages()[0].Code
	phoneRegisterResp := performPhoneAuthRequest(router, "/api/user/register/phone", fmt.Sprintf(`{"password":"password123","phone_number":"13800138006","verification_code":%q}`, registerCode))
	require.True(t, phoneRegisterResp.Success, phoneRegisterResp.Message)

	resetVerifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138006","purpose":"reset_password"}`)
	require.True(t, resetVerifyResp.Success, resetVerifyResp.Message)
	resetCode := fakeSender.Messages()[1].Code
	resetResp := performPhoneAuthRequest(router, "/api/user/reset_password/phone", fmt.Sprintf(`{"phone_number":"13800138006","verification_code":%q,"password":"12345678"}`, resetCode))
	require.False(t, resetResp.Success)
}

func TestPasswordMutationEndpointsRejectWeakPasswords(t *testing.T) {
	_, _ = setupPhoneAuthControllerTest(t)

	user := &model.User{
		Username:    "password_mutation_user",
		Password:    "oldpass123",
		DisplayName: "password_mutation",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, user.Insert(0))

	selfReq := httptest.NewRequest(http.MethodPost, "/api/user/self", bytes.NewBufferString(`{"original_password":"oldpass123","password":"abcdefgh"}`))
	selfReq.Header.Set("Content-Type", "application/json")
	selfRecorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(selfRecorder)
	ctx.Request = selfReq
	ctx.Set("id", user.Id)
	UpdateSelf(ctx)
	selfResp := decodeTokenAPIResponse(selfRecorder)
	require.False(t, selfResp.Success)

	createReq := httptest.NewRequest(http.MethodPost, "/api/user", bytes.NewBufferString(`{"username":"created_weak_user","password":"abcdefgh","display_name":"created","role":1}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	createCtx, _ := gin.CreateTestContext(createRecorder)
	createCtx.Request = createReq
	createCtx.Set("role", common.RoleRootUser)
	CreateUser(createCtx)
	createResp := decodeTokenAPIResponse(createRecorder)
	require.False(t, createResp.Success)

	updateReq := httptest.NewRequest(http.MethodPut, "/api/user", bytes.NewBufferString(fmt.Sprintf(`{"id":%d,"username":"password_mutation_user","password":"abcdefgh","display_name":"password_mutation","role":1}`, user.Id)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRecorder := httptest.NewRecorder()
	updateCtx, _ := gin.CreateTestContext(updateRecorder)
	updateCtx.Request = updateReq
	updateCtx.Set("role", common.RoleRootUser)
	UpdateUser(updateCtx)
	updateResp := decodeTokenAPIResponse(updateRecorder)
	require.False(t, updateResp.Success)
}

func TestCreateUserTruncatesDefaultDisplayNameForLongUsername(t *testing.T) {
	_, _ = setupPhoneAuthControllerTest(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/user", bytes.NewBufferString(`{"username":"very.long.email.address@example.com","password":"password123","role":1}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	createCtx, _ := gin.CreateTestContext(createRecorder)
	createCtx.Request = createReq
	createCtx.Set("role", common.RoleRootUser)
	CreateUser(createCtx)
	createResp := decodeTokenAPIResponse(createRecorder)
	require.True(t, createResp.Success, createResp.Message)

	var user model.User
	require.NoError(t, model.DB.Where("username = ?", "very.long.email.address@example.com").First(&user).Error)
	assert.LessOrEqual(t, len([]rune(user.DisplayName)), model.UserDisplayNameMaxLength)
}

func TestPasswordResetVerifyAndConfirmSupportsEmailAndPhone(t *testing.T) {
	router, fakeSender := setupPhoneAuthControllerTest(t)

	emailUser := &model.User{
		Username:    "reset-by-username@example.com",
		Password:    "oldpass123",
		DisplayName: "reset-email",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, emailUser.Insert(0))

	emailCode := "112233"
	common.RegisterVerificationCodeWithKey(emailUser.Username, emailCode, common.PasswordResetPurpose)
	emailVerifyResp := performPhoneAuthRequest(router, "/api/user/reset/verify", fmt.Sprintf(`{"account":%q,"verification_code":%q}`, emailUser.Username, emailCode))
	require.True(t, emailVerifyResp.Success, emailVerifyResp.Message)
	var emailSession struct {
		AccountType string `json:"account_type"`
		Account     string `json:"account"`
		ResetToken  string `json:"reset_token"`
	}
	require.NoError(t, common.Unmarshal(emailVerifyResp.Data, &emailSession))
	assert.Equal(t, "email", emailSession.AccountType)
	assert.Equal(t, emailUser.Username, emailSession.Account)
	require.NotEmpty(t, emailSession.ResetToken)

	emailConfirmResp := performPhoneAuthRequest(router, "/api/user/reset/confirm", fmt.Sprintf(`{"account_type":"email","account":%q,"reset_token":%q,"password":"newpass123"}`, emailSession.Account, emailSession.ResetToken))
	require.True(t, emailConfirmResp.Success, emailConfirmResp.Message)
	emailReuseResp := performPhoneAuthRequest(router, "/api/user/reset/confirm", fmt.Sprintf(`{"account_type":"email","account":%q,"reset_token":%q,"password":"another123"}`, emailSession.Account, emailSession.ResetToken))
	require.False(t, emailReuseResp.Success)
	emailLoginResp := performPhoneAuthRequest(router, "/api/user/login", fmt.Sprintf(`{"username":%q,"password":"newpass123"}`, emailUser.Username))
	require.True(t, emailLoginResp.Success, emailLoginResp.Message)

	phoneVerifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138007","purpose":"register"}`)
	require.True(t, phoneVerifyResp.Success, phoneVerifyResp.Message)
	phoneRegisterCode := fakeSender.Messages()[0].Code
	phoneRegisterResp := performPhoneAuthRequest(router, "/api/user/register/phone", fmt.Sprintf(`{"password":"oldpass123","phone_number":"13800138007","verification_code":%q}`, phoneRegisterCode))
	require.True(t, phoneRegisterResp.Success, phoneRegisterResp.Message)

	phoneResetVerifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138007","purpose":"reset_password"}`)
	require.True(t, phoneResetVerifyResp.Success, phoneResetVerifyResp.Message)
	phoneResetCode := fakeSender.Messages()[1].Code
	phoneVerifyResetResp := performPhoneAuthRequest(router, "/api/user/reset/verify", fmt.Sprintf(`{"account":"13800138007","verification_code":%q}`, phoneResetCode))
	require.True(t, phoneVerifyResetResp.Success, phoneVerifyResetResp.Message)
	var phoneSession struct {
		AccountType string `json:"account_type"`
		Account     string `json:"account"`
		ResetToken  string `json:"reset_token"`
	}
	require.NoError(t, common.Unmarshal(phoneVerifyResetResp.Data, &phoneSession))
	assert.Equal(t, "phone", phoneSession.AccountType)
	assert.Equal(t, "+8613800138007", phoneSession.Account)
	require.NotEmpty(t, phoneSession.ResetToken)

	phoneConfirmResp := performPhoneAuthRequest(router, "/api/user/reset/confirm", fmt.Sprintf(`{"account_type":"phone","account":%q,"reset_token":%q,"password":"newpass123"}`, phoneSession.Account, phoneSession.ResetToken))
	require.True(t, phoneConfirmResp.Success, phoneConfirmResp.Message)
	phoneReuseResp := performPhoneAuthRequest(router, "/api/user/reset/confirm", fmt.Sprintf(`{"account_type":"phone","account":%q,"reset_token":%q,"password":"another123"}`, phoneSession.Account, phoneSession.ResetToken))
	require.False(t, phoneReuseResp.Success)
	phoneLoginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"+8613800138007","password":"newpass123"}`)
	require.True(t, phoneLoginResp.Success, phoneLoginResp.Message)
}

func TestEmailVerificationLoginConsumesCode(t *testing.T) {
	router, _ := setupPhoneAuthControllerTest(t)

	user := &model.User{
		Username:    "email_code_login",
		Password:    "password123",
		DisplayName: "email_code_login",
		Email:       "email-code-login@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, user.Insert(0))

	code := "654321"
	common.RegisterVerificationCodeWithKey(user.Email, code, common.EmailLoginPurpose)

	loginBody := fmt.Sprintf(`{"email":%q,"verification_code":%q}`, user.Email, code)
	loginResp := performPhoneAuthRequest(router, "/api/user/login/email", loginBody)
	require.True(t, loginResp.Success, loginResp.Message)

	reuseResp := performPhoneAuthRequest(router, "/api/user/login/email", loginBody)
	assert.False(t, reuseResp.Success)
	assert.NotEmpty(t, reuseResp.Message)
}

func TestEmailVerificationLoginSupportsUsernameEmailAccount(t *testing.T) {
	router, _ := setupPhoneAuthControllerTest(t)

	user := &model.User{
		Username:    "username-email-login@example.com",
		Password:    "password123",
		DisplayName: "email_code_login",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, user.Insert(0))

	code := "654321"
	common.RegisterVerificationCodeWithKey(user.Username, code, common.EmailLoginPurpose)

	loginResp := performPhoneAuthRequest(router, "/api/user/login/email", fmt.Sprintf(`{"email":%q,"verification_code":%q}`, user.Username, code))
	require.True(t, loginResp.Success, loginResp.Message)
}

func TestPhoneAuthControllerRejectsDuplicatePhoneRegister(t *testing.T) {
	router, fakeSender := setupPhoneAuthControllerTest(t)

	verifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138001","purpose":"register"}`)
	require.True(t, verifyResp.Success, verifyResp.Message)
	require.Len(t, fakeSender.Messages(), 1)

	registerCode := fakeSender.Messages()[0].Code
	registerBody := fmt.Sprintf(`{"username":"firstphone","password":"password123","phone_number":"13800138001","verification_code":%q}`, registerCode)
	registerResp := performPhoneAuthRequest(router, "/api/user/register/phone", registerBody)
	require.True(t, registerResp.Success, registerResp.Message)

	duplicateVerifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138001","purpose":"register"}`)
	assert.False(t, duplicateVerifyResp.Success)
	assert.NotEmpty(t, duplicateVerifyResp.Message)
}

func TestEmailPasswordResetUsesCustomPasswordAndKeepsLegacyRandomPassword(t *testing.T) {
	router, _ := setupPhoneAuthControllerTest(t)

	customUser := &model.User{
		Username:    "email_custom_reset",
		Password:    "oldpassword123",
		DisplayName: "email_custom_reset",
		Email:       "custom-reset@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, customUser.Insert(0))

	customToken := "custom-token"
	common.RegisterVerificationCodeWithKey(customUser.Email, customToken, common.PasswordResetPurpose)
	customResetResp := performPhoneAuthRequest(router, "/api/user/reset", fmt.Sprintf(`{"email":%q,"token":%q,"password":"newpassword123"}`, customUser.Email, customToken))
	require.True(t, customResetResp.Success, customResetResp.Message)
	require.Empty(t, customResetResp.Data)
	customResetReuseResp := performPhoneAuthRequest(router, "/api/user/reset", fmt.Sprintf(`{"email":%q,"token":%q,"password":"another123"}`, customUser.Email, customToken))
	require.False(t, customResetReuseResp.Success)

	customLoginResp := performPhoneAuthRequest(router, "/api/user/login", `{"username":"custom-reset@example.com","password":"newpassword123"}`)
	require.True(t, customLoginResp.Success, customLoginResp.Message)

	legacyUser := &model.User{
		Username:    "email_legacy_reset",
		Password:    "oldpassword123",
		DisplayName: "email_legacy_reset",
		Email:       "legacy-reset@example.com",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, legacyUser.Insert(0))

	legacyToken := "legacy-token"
	common.RegisterVerificationCodeWithKey(legacyUser.Email, legacyToken, common.PasswordResetPurpose)
	legacyResetResp := performPhoneAuthRequest(router, "/api/user/reset", fmt.Sprintf(`{"email":%q,"token":%q}`, legacyUser.Email, legacyToken))
	require.True(t, legacyResetResp.Success, legacyResetResp.Message)
	require.NotEmpty(t, legacyResetResp.Data)

	var generatedPassword string
	require.NoError(t, common.Unmarshal(legacyResetResp.Data, &generatedPassword))
	require.NotEmpty(t, generatedPassword)
	require.NoError(t, validatePlainPassword(generatedPassword))

	legacyLoginResp := performPhoneAuthRequest(router, "/api/user/login", fmt.Sprintf(`{"username":"legacy-reset@example.com","password":%q}`, generatedPassword))
	require.True(t, legacyLoginResp.Success, legacyLoginResp.Message)
}

func TestPhoneAuthControllerGeneratesUsernameWhenPhoneRegisterOmitsUsername(t *testing.T) {
	router, fakeSender := setupPhoneAuthControllerTest(t)

	verifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138002","purpose":"register"}`)
	require.True(t, verifyResp.Success, verifyResp.Message)
	require.Len(t, fakeSender.Messages(), 1)

	registerCode := fakeSender.Messages()[0].Code
	registerResp := performPhoneAuthRequest(router, "/api/user/register/phone", fmt.Sprintf(`{"password":"password123","phone_number":"13800138002","verification_code":%q}`, registerCode))
	require.True(t, registerResp.Success, registerResp.Message)

	var userCount int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("username <> ''").Count(&userCount).Error)
	assert.EqualValues(t, 1, userCount)

	registered, err := model.GetUserByPhoneNumber("+8613800138002")
	require.NoError(t, err)
	assert.NotEmpty(t, registered.Username)
	assert.LessOrEqual(t, len(registered.Username), model.UserNameMaxLength)
}

func TestPhoneAuthControllerPhoneRegisterPerservesInviterId(t *testing.T) {
	router, fakeSender := setupPhoneAuthControllerTest(t)

	inviter := &model.User{
		Username:    "phone_inviter",
		Password:    "password123",
		DisplayName: "phone_inviter",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, inviter.Insert(0))
	require.NotEmpty(t, inviter.AffCode)

	verifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138003","purpose":"register"}`)
	require.True(t, verifyResp.Success, verifyResp.Message)
	require.Len(t, fakeSender.Messages(), 1)

	registerCode := fakeSender.Messages()[0].Code
	registerResp := performPhoneAuthRequest(router, "/api/user/register/phone", fmt.Sprintf(`{"username":"phoneinvitee","password":"password123","phone_number":"13800138003","verification_code":%q,"aff_code":%q}`, registerCode, inviter.AffCode))
	require.True(t, registerResp.Success, registerResp.Message)

	registered, err := model.GetUserByPhoneNumber("+8613800138003")
	require.NoError(t, err)
	assert.Equal(t, inviter.Id, registered.InviterId)
}

func TestPhoneAuthControllerRegisterRollbackKeepsVerificationCode(t *testing.T) {
	router, fakeSender := setupPhoneAuthControllerTest(t)

	verifyResp := performPhoneAuthRequest(router, "/api/phone/verification", `{"phone_number":"13800138004","purpose":"register"}`)
	require.True(t, verifyResp.Success, verifyResp.Message)
	require.Len(t, fakeSender.Messages(), 1)
	registerCode := fakeSender.Messages()[0].Code

	callbackName := "phone_auth_test:fail_user_profile_create"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*model.UserProfile); ok {
			tx.AddError(errors.New("forced user profile create failure"))
		}
	}))
	callbackRemoved := false
	removeCallback := func() {
		if callbackRemoved {
			return
		}
		_ = model.DB.Callback().Create().Remove(callbackName)
		callbackRemoved = true
	}
	t.Cleanup(removeCallback)

	registerResp := performPhoneAuthRequest(router, "/api/user/register/phone", fmt.Sprintf(`{"username":"rollbackphone","password":"password123","phone_number":"13800138004","verification_code":%q}`, registerCode))
	assert.False(t, registerResp.Success)
	removeCallback()

	err := service.ConsumePhoneVerificationCode("+8613800138004", model.PhoneVerificationPurposeRegister, registerCode)
	require.NoError(t, err)
}
