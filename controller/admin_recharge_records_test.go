package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type adminTopUpRecordListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total    int                      `json:"total"`
		Page     int                      `json:"page"`
		PageSize int                      `json:"page_size"`
		Items    []model.AdminTopUpRecord `json:"items"`
	} `json:"data"`
}

type adminVipActivationRecordListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total    int                              `json:"total"`
		Page     int                              `json:"page"`
		PageSize int                              `json:"page_size"`
		Items    []model.AdminVipActivationRecord `json:"items"`
	} `json:"data"`
}

type adminRechargeRecordMutationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupAdminRechargeRecordControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())

	oldUsingSQLite := common.UsingSQLite
	oldRedisEnabled := common.RedisEnabled
	common.UsingSQLite = true
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.UserProfile{},
		&model.TopUp{},
		&model.VipActivationRecord{},
	))

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		_ = sqlDB.Close()
	})

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("admin-recharge-record-test"))))
	router.GET("/login/:role", func(c *gin.Context) {
		role := common.RoleCommonUser
		id := 9502
		if c.Param("role") == "admin" {
			role = common.RoleAdminUser
			id = 9501
		}
		session := sessions.Default(c)
		session.Set("username", c.Param("role"))
		session.Set("role", role)
		session.Set("id", id)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	admin := router.Group("/api")
	admin.Use(middleware.AdminAuth())
	admin.GET("/user/topup", GetAllTopUps)
	admin.GET("/vip/admin/records", AdminListVipActivationRecords)
	return router
}

func loginAdminRechargeRecordTestUser(t *testing.T, router *gin.Engine, role string, userId string) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/login/"+role, nil)
	request.Header.Set("New-Api-User", userId)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func TestAdminRechargeRecordRoutesRequireAdminAuth(t *testing.T) {
	router := setupAdminRechargeRecordControllerTest(t)

	anonymousRecorder := httptest.NewRecorder()
	anonymousReq := httptest.NewRequest(http.MethodGet, "/api/user/topup", nil)
	router.ServeHTTP(anonymousRecorder, anonymousReq)
	require.Equal(t, http.StatusUnauthorized, anonymousRecorder.Code)

	anonymousVipRecorder := httptest.NewRecorder()
	anonymousVipReq := httptest.NewRequest(http.MethodGet, "/api/vip/admin/records", nil)
	router.ServeHTTP(anonymousVipRecorder, anonymousVipReq)
	require.Equal(t, http.StatusUnauthorized, anonymousVipRecorder.Code)

	userCookies := loginAdminRechargeRecordTestUser(t, router, "user", "9502")
	userRecorder := httptest.NewRecorder()
	userReq := httptest.NewRequest(http.MethodGet, "/api/user/topup", nil)
	userReq.Header.Set("New-Api-User", "9502")
	for _, cookie := range userCookies {
		userReq.AddCookie(cookie)
	}
	router.ServeHTTP(userRecorder, userReq)

	var userResp adminRechargeRecordMutationResponse
	require.NoError(t, common.Unmarshal(userRecorder.Body.Bytes(), &userResp))
	require.False(t, userResp.Success)

	userVipRecorder := httptest.NewRecorder()
	userVipReq := httptest.NewRequest(http.MethodGet, "/api/vip/admin/records", nil)
	userVipReq.Header.Set("New-Api-User", "9502")
	for _, cookie := range userCookies {
		userVipReq.AddCookie(cookie)
	}
	router.ServeHTTP(userVipRecorder, userVipReq)

	var userVipResp adminRechargeRecordMutationResponse
	require.NoError(t, common.Unmarshal(userVipRecorder.Body.Bytes(), &userVipResp))
	require.False(t, userVipResp.Success)

	adminCookies := loginAdminRechargeRecordTestUser(t, router, "admin", "9501")
	adminRecorder := httptest.NewRecorder()
	adminReq := httptest.NewRequest(http.MethodGet, "/api/user/topup", nil)
	adminReq.Header.Set("New-Api-User", "9501")
	for _, cookie := range adminCookies {
		adminReq.AddCookie(cookie)
	}
	router.ServeHTTP(adminRecorder, adminReq)

	var adminResp adminTopUpRecordListResponse
	require.NoError(t, common.Unmarshal(adminRecorder.Body.Bytes(), &adminResp))
	require.True(t, adminResp.Success, adminResp.Message)

	adminVipRecorder := httptest.NewRecorder()
	adminVipReq := httptest.NewRequest(http.MethodGet, "/api/vip/admin/records", nil)
	adminVipReq.Header.Set("New-Api-User", "9501")
	for _, cookie := range adminCookies {
		adminVipReq.AddCookie(cookie)
	}
	router.ServeHTTP(adminVipRecorder, adminVipReq)

	var adminVipResp adminVipActivationRecordListResponse
	require.NoError(t, common.Unmarshal(adminVipRecorder.Body.Bytes(), &adminVipResp))
	require.True(t, adminVipResp.Success, adminVipResp.Message)
}

func TestGetAllTopUpsParsesAdminFiltersAndReturnsCurrentContact(t *testing.T) {
	router := setupAdminRechargeRecordControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          4501,
		Username:    "api_topup_user",
		DisplayName: "API Topup User",
		Email:       "api-topup@example.com",
		AffCode:     "api4501",
		Status:      common.UserStatusEnabled,
	}).Error)
	phone := "13700137001"
	require.NoError(t, model.DB.Create(&model.UserProfile{
		UserId:      4501,
		PhoneNumber: &phone,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          4501,
		Amount:          50,
		Money:           50,
		TradeNo:         "TOPUP-API-CURRENT",
		PaymentProvider: model.PaymentProviderStripe,
		PaymentMethod:   model.PaymentMethodStripe,
		CreateTime:      1710200000,
		CompleteTime:    1710200300,
		Status:          common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          4501,
		Amount:          60,
		Money:           60,
		TradeNo:         "TOPUP-API-PENDING",
		PaymentProvider: model.PaymentProviderWechat,
		PaymentMethod:   model.PaymentMethodWechatDirect,
		CreateTime:      1710201000,
		Status:          common.TopUpStatusPending,
	}).Error)

	adminCookies := loginAdminRechargeRecordTestUser(t, router, "admin", "9501")
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/topup?email=api-topup@example.com&phone_number=13700137001&trade_no=CURRENT&status=success&payment_provider=stripe&payment_method=stripe&created_from=1710199999&created_to=1710200001&completed_from=1710200200&completed_to=1710200400&p=1&page_size=10", nil)
	req.Header.Set("New-Api-User", "9501")
	for _, cookie := range adminCookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, req)

	var resp adminTopUpRecordListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Equal(t, 1, resp.Data.Total)
	require.Len(t, resp.Data.Items, 1)
	require.Equal(t, "TOPUP-API-CURRENT", resp.Data.Items[0].TradeNo)
	require.Equal(t, "api_topup_user", resp.Data.Items[0].Username)
	require.Equal(t, "API Topup User", resp.Data.Items[0].DisplayName)
	require.Equal(t, "api-topup@example.com", resp.Data.Items[0].Email)
	require.Equal(t, "13700137001", resp.Data.Items[0].PhoneNumber)
}

func TestAdminListVipActivationRecordsParsesFiltersAndReturnsCurrentContact(t *testing.T) {
	router := setupAdminRechargeRecordControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          4601,
		Username:    "api_vvip_user",
		DisplayName: "API VVIP User",
		Email:       "api-vvip@example.com",
		AffCode:     "api4601",
		Status:      common.UserStatusEnabled,
	}).Error)
	phone := "13600136001"
	require.NoError(t, model.DB.Create(&model.UserProfile{
		UserId:      4601,
		PhoneNumber: &phone,
	}).Error)
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:          4601,
		TradeNo:         "VVIP-API-CURRENT",
		PaymentProvider: model.PaymentProviderStripe,
		PaymentMethod:   model.PaymentMethodStripe,
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     1710300300,
		CreatedAt:       1710300000,
	}).Error)
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:          4601,
		TradeNo:         "VVIP-API-PENDING",
		PaymentProvider: model.PaymentProviderWechat,
		PaymentMethod:   model.PaymentMethodWechatDirect,
		Status:          model.VipActivationStatusPending,
		CreatedAt:       1710301000,
	}).Error)

	adminCookies := loginAdminRechargeRecordTestUser(t, router, "admin", "9501")
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/vip/admin/records?email=api-vvip@example.com&phone_number=13600136001&trade_no=CURRENT&status=success&payment_provider=stripe&payment_method=stripe&created_from=1710299999&created_to=1710300001&activated_from=1710300200&activated_to=1710300400&p=1&page_size=10", nil)
	req.Header.Set("New-Api-User", "9501")
	for _, cookie := range adminCookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, req)

	var resp adminVipActivationRecordListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Equal(t, 1, resp.Data.Total)
	require.Len(t, resp.Data.Items, 1)
	require.Equal(t, "VVIP-API-CURRENT", resp.Data.Items[0].TradeNo)
	require.Equal(t, "api_vvip_user", resp.Data.Items[0].Username)
	require.Equal(t, "API VVIP User", resp.Data.Items[0].DisplayName)
	require.Equal(t, "api-vvip@example.com", resp.Data.Items[0].Email)
	require.Equal(t, "13600136001", resp.Data.Items[0].PhoneNumber)
}
