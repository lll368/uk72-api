package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type adminUserDiscountResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Data    model.User `json:"data"`
}

func setupAdminUserDiscountControllerTest(t *testing.T, adminRole int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())

	oldUsingSQLite := common.UsingSQLite
	oldRedisEnabled := common.RedisEnabled
	common.UsingSQLite = true
	common.RedisEnabled = false
	model.InitDBColumnNamesForTests()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.UserRelation{}, &model.VipActivationRecord{}, &model.UserProfile{}))

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		_ = sqlDB.Close()
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 9001)
		c.Set("username", "admin")
		c.Set("role", adminRole)
		c.Next()
	})
	router.PUT("/api/user/:id/discount", UpdateUserTopupDiscount)
	router.DELETE("/api/user/:id/discount", ResetUserTopupDiscount)
	router.GET("/api/user/:id", GetUser)
	return router
}

func seedAdminUserDiscountUser(t *testing.T, id int, username string, role int, discount *float64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:            id,
		Username:      username,
		DisplayName:   username,
		AffCode:       fmt.Sprintf("discount%d", id),
		Status:        common.UserStatusEnabled,
		Role:          role,
		Group:         "default",
		TopupDiscount: discount,
	}).Error)
}

func performAdminUserDiscountRequest(router *gin.Engine, method string, path string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestAdminSetGetAndResetUserTopupDiscount(t *testing.T) {
	router := setupAdminUserDiscountControllerTest(t, common.RoleAdminUser)
	seedAdminUserDiscountUser(t, 2101, "discount_target", common.RoleCommonUser, nil)

	updateRecorder := performAdminUserDiscountRequest(router, http.MethodPut, "/api/user/2101/discount", `{"topup_discount":0.85}`)
	var updateResp adminUserDiscountResponse
	require.NoError(t, common.Unmarshal(updateRecorder.Body.Bytes(), &updateResp))
	require.True(t, updateResp.Success, updateResp.Message)
	require.NotNil(t, updateResp.Data.TopupDiscount)
	assert.InDelta(t, 0.85, *updateResp.Data.TopupDiscount, 0.000001)

	getRecorder := performAdminUserDiscountRequest(router, http.MethodGet, "/api/user/2101", "")
	var getResp adminUserDiscountResponse
	require.NoError(t, common.Unmarshal(getRecorder.Body.Bytes(), &getResp))
	require.True(t, getResp.Success, getResp.Message)
	require.NotNil(t, getResp.Data.TopupDiscount)
	assert.InDelta(t, 0.85, *getResp.Data.TopupDiscount, 0.000001)

	resetRecorder := performAdminUserDiscountRequest(router, http.MethodDelete, "/api/user/2101/discount", "")
	var resetResp adminUserDiscountResponse
	require.NoError(t, common.Unmarshal(resetRecorder.Body.Bytes(), &resetResp))
	require.True(t, resetResp.Success, resetResp.Message)
	require.NotNil(t, resetResp.Data.TopupDiscount)
	assert.InDelta(t, 1, *resetResp.Data.TopupDiscount, 0.000001)
}

func TestAdminUserTopupDiscountValidatesRangeAndRole(t *testing.T) {
	router := setupAdminUserDiscountControllerTest(t, common.RoleAdminUser)
	seedAdminUserDiscountUser(t, 2201, "range_target", common.RoleCommonUser, nil)
	seedAdminUserDiscountUser(t, 2202, "same_level_admin", common.RoleAdminUser, nil)

	testCases := []struct {
		name   string
		path   string
		body   string
		errMsg string
	}{
		{name: "zero rejected", path: "/api/user/2201/discount", body: `{"topup_discount":0}`, errMsg: "discount"},
		{name: "greater than one rejected", path: "/api/user/2201/discount", body: `{"topup_discount":1.01}`, errMsg: "discount"},
		{name: "same level rejected", path: "/api/user/2202/discount", body: `{"topup_discount":0.9}`, errMsg: "permission"},
		{name: "bad id rejected", path: "/api/user/bad/discount", body: `{"topup_discount":0.9}`, errMsg: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performAdminUserDiscountRequest(router, http.MethodPut, tc.path, tc.body)
			var resp adminUserDiscountResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
			assert.False(t, resp.Success)
			if tc.errMsg != "" {
				assert.Contains(t, strings.ToLower(resp.Message), tc.errMsg)
			}
		})
	}

	var stored model.User
	require.NoError(t, model.DB.First(&stored, 2201).Error)
	assert.Nil(t, stored.TopupDiscount)
}

func TestRootCanSetAdminUserTopupDiscount(t *testing.T) {
	router := setupAdminUserDiscountControllerTest(t, common.RoleRootUser)
	seedAdminUserDiscountUser(t, 2301, "admin_target", common.RoleAdminUser, nil)

	recorder := performAdminUserDiscountRequest(router, http.MethodPut, "/api/user/2301/discount", `{"topup_discount":1}`)
	var resp adminUserDiscountResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.NotNil(t, resp.Data.TopupDiscount)
	assert.InDelta(t, 1, *resp.Data.TopupDiscount, 0.000001)
}

func TestUserTopupDiscountResetDoesNotFallbackToRelationDiscount(t *testing.T) {
	setupAdminUserDiscountControllerTest(t, common.RoleRootUser)
	ownDiscount := 0.75
	seedAdminUserDiscountUser(t, 2401, "parent_vvip", common.RoleCommonUser, nil)
	seedAdminUserDiscountUser(t, 2402, "relation_child", common.RoleCommonUser, &ownDiscount)
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:          2401,
		TradeNo:         "active-vvip-2401",
		PaymentProvider: model.PaymentProviderEpay,
		PaymentMethod:   "alipay",
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     1,
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserRelation{
		ParentUserId: 2401,
		ChildUserId:  2402,
		Source:       model.UserRelationSourceAdmin,
		Status:       model.UserRelationStatusActive,
	}).Error)

	discount, ok, err := model.GetUserTopupDiscount(2402)
	require.NoError(t, err)
	require.True(t, ok)
	assert.InDelta(t, 0.75, discount, 0.000001)

	require.NoError(t, model.UpdateUserTopupDiscount(2402, nil))
	discount, ok, err = model.GetUserTopupDiscount(2402)
	require.NoError(t, err)
	require.True(t, ok)
	assert.InDelta(t, 1, discount, 0.000001)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	users, _, err := model.GetAllUsers(pageInfo)
	require.NoError(t, err)
	var child *model.User
	for _, user := range users {
		if user.Id == 2402 {
			child = user
			break
		}
	}
	require.NotNil(t, child)
	require.NotNil(t, child.TopupDiscount)
	assert.InDelta(t, 1, *child.TopupDiscount, 0.000001)
}

func TestAdminDiscountRouteRejectsInvalidUserId(t *testing.T) {
	router := setupAdminUserDiscountControllerTest(t, common.RoleRootUser)
	recorder := performAdminUserDiscountRequest(router, http.MethodDelete, "/api/user/"+strconv.Itoa(0)+"/discount", "")

	var resp adminUserDiscountResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
}
