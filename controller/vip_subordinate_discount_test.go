package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type vipSubordinateMutationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type vipSubordinateListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Page                        int     `json:"page"`
		PageSize                    int     `json:"page_size"`
		Total                       int     `json:"total"`
		ParentTopupDiscount         float64 `json:"parent_topup_discount"`
		CanSetSubordinateDiscount   bool    `json:"can_set_subordinate_discount"`
		MinSubordinateTopupDiscount float64 `json:"min_subordinate_topup_discount"`
		Items                       []struct {
			RelationId    int     `json:"relation_id"`
			ChildUserId   int     `json:"child_user_id"`
			Username      string  `json:"username"`
			DisplayName   string  `json:"display_name"`
			Status        int     `json:"status"`
			Group         string  `json:"group"`
			BindTime      int64   `json:"bind_time"`
			TopupDiscount float64 `json:"topup_discount"`
		} `json:"items"`
	} `json:"data"`
}

func setupVipSubordinateDiscountControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

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
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.UserProfile{},
		&model.VipActivationRecord{},
		&model.UserRelation{},
		&model.TopUp{},
	))

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		_ = sqlDB.Close()
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		userId, _ := strconv.Atoi(c.GetHeader("X-Test-User-Id"))
		if userId == 0 {
			userId = 9001
		}
		c.Set("id", userId)
		c.Set("role", common.RoleCommonUser)
		c.Next()
	})
	router.GET("/api/vip/subordinates", ListVipSubordinates)
	router.PUT("/api/vip/subordinates/:child_user_id/discount", UpdateVipSubordinateDiscount)
	router.DELETE("/api/vip/subordinates/:child_user_id/discount", ResetVipSubordinateDiscount)
	router.POST("/api/user/creem/pay", RequestCreemPay)
	return router
}

func seedVipSubordinateUser(t *testing.T, id int, username string, activeVvip bool) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:          id,
		Username:    username,
		DisplayName: username + "_display",
		AffCode:     fmt.Sprintf("sub%d", id),
		Group:       "default",
		Status:      common.UserStatusEnabled,
		Role:        common.RoleCommonUser,
	}).Error)
	if activeVvip {
		now := time.Now().Unix()
		require.NoError(t, model.DB.Create(&model.VipActivationRecord{
			UserId:          id,
			TradeNo:         fmt.Sprintf("subordinate-vvip-%d", id),
			PaymentProvider: model.PaymentProviderEpay,
			PaymentMethod:   "alipay",
			Status:          model.VipActivationStatusSuccess,
			ActivatedAt:     now,
		}).Error)
		require.NoError(t, model.DB.Create(&model.UserProfile{
			UserId:          id,
			IsVvip:          true,
			VvipStatus:      model.VvipStatusActive,
			VvipActivatedAt: now,
		}).Error)
	}
}

func seedVipSubordinateRelation(t *testing.T, parentId int, childId int, discount float64) {
	t.Helper()
	var topupDiscount *float64
	if discount > 0 {
		topupDiscount = &discount
	}
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", childId).Update("topup_discount", topupDiscount).Error)
	require.NoError(t, model.DB.Create(&model.UserRelation{
		ParentUserId: parentId,
		ChildUserId:  childId,
		Source:       model.UserRelationSourceAdmin,
		Status:       model.UserRelationStatusActive,
	}).Error)
}

func performVipSubordinateRequest(router *gin.Engine, userId int, method string, path string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-Id", strconv.Itoa(userId))
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestVipSubordinatesRequireActiveVvip(t *testing.T) {
	router := setupVipSubordinateDiscountControllerTest(t)
	seedVipSubordinateUser(t, 1101, "plain_parent", false)

	recorder := performVipSubordinateRequest(router, 1101, http.MethodGet, "/api/vip/subordinates", "")

	var resp vipSubordinateMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "Compute Partner")
}

func TestUpdateVipSubordinateDiscountAllowsOnlyOneWhenParentDiscountIsOne(t *testing.T) {
	router := setupVipSubordinateDiscountControllerTest(t)
	seedVipSubordinateUser(t, 1201, "top_level_vvip", true)
	seedVipSubordinateUser(t, 1202, "direct_child", false)
	seedVipSubordinateRelation(t, 1201, 1202, 0)

	recorder := performVipSubordinateRequest(router, 1201, http.MethodPut, "/api/vip/subordinates/1202/discount", `{"topup_discount":0.8}`)

	var resp vipSubordinateMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "current user discount")

	recorder = performVipSubordinateRequest(router, 1201, http.MethodPut, "/api/vip/subordinates/1202/discount", `{"topup_discount":1}`)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.True(t, resp.Success, resp.Message)
}

func TestUpdateListAndResetVipSubordinateDiscount(t *testing.T) {
	router := setupVipSubordinateDiscountControllerTest(t)
	seedVipSubordinateUser(t, 1301, "upper_vvip", true)
	seedVipSubordinateUser(t, 1302, "middle_vvip", true)
	seedVipSubordinateUser(t, 1303, "discount_child", false)
	seedVipSubordinateRelation(t, 1301, 1302, 0.7)
	seedVipSubordinateRelation(t, 1302, 1303, 0)

	updateRecorder := performVipSubordinateRequest(router, 1302, http.MethodPut, "/api/vip/subordinates/1303/discount", `{"topup_discount":0.8}`)
	var updateResp vipSubordinateMutationResponse
	require.NoError(t, common.Unmarshal(updateRecorder.Body.Bytes(), &updateResp))
	require.True(t, updateResp.Success, updateResp.Message)

	listRecorder := performVipSubordinateRequest(router, 1302, http.MethodGet, "/api/vip/subordinates?p=1&page_size=10", "")
	var listResp vipSubordinateListResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Equal(t, 1, listResp.Data.Total)
	require.Len(t, listResp.Data.Items, 1)
	assert.Equal(t, 1303, listResp.Data.Items[0].ChildUserId)
	assert.Equal(t, "discount_child", listResp.Data.Items[0].Username)
	assert.InDelta(t, 0.8, listResp.Data.Items[0].TopupDiscount, 0.000001)
	assert.InDelta(t, 0.7, listResp.Data.ParentTopupDiscount, 0.000001)
	assert.True(t, listResp.Data.CanSetSubordinateDiscount)
	assert.InDelta(t, 0.7, listResp.Data.MinSubordinateTopupDiscount, 0.000001)

	resetRecorder := performVipSubordinateRequest(router, 1302, http.MethodDelete, "/api/vip/subordinates/1303/discount", "")
	var resetResp vipSubordinateMutationResponse
	require.NoError(t, common.Unmarshal(resetRecorder.Body.Bytes(), &resetResp))
	require.True(t, resetResp.Success, resetResp.Message)

	var child model.User
	require.NoError(t, model.DB.Where("id = ?", 1303).First(&child).Error)
	require.NotNil(t, child.TopupDiscount)
	assert.InDelta(t, 1, *child.TopupDiscount, 0.000001)
}

func TestUpdateVipSubordinateDiscountValidatesRangeAndOwnership(t *testing.T) {
	router := setupVipSubordinateDiscountControllerTest(t)
	seedVipSubordinateUser(t, 1401, "range_upper", true)
	seedVipSubordinateUser(t, 1402, "range_parent", true)
	seedVipSubordinateUser(t, 1403, "range_child", false)
	seedVipSubordinateUser(t, 1404, "other_child", false)
	seedVipSubordinateRelation(t, 1401, 1402, 0.7)
	seedVipSubordinateRelation(t, 1402, 1403, 0)
	seedVipSubordinateRelation(t, 1401, 1404, 0.8)

	testCases := []struct {
		name   string
		path   string
		body   string
		errMsg string
	}{
		{name: "below parent discount", path: "/api/vip/subordinates/1403/discount", body: `{"topup_discount":0.69}`, errMsg: "current user discount"},
		{name: "above one", path: "/api/vip/subordinates/1403/discount", body: `{"topup_discount":1.01}`, errMsg: "less than or equal to 1"},
		{name: "non direct child", path: "/api/vip/subordinates/1404/discount", body: `{"topup_discount":0.9}`, errMsg: "subordinate"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performVipSubordinateRequest(router, 1402, http.MethodPut, tc.path, tc.body)
			var resp vipSubordinateMutationResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
			assert.False(t, resp.Success)
			assert.Contains(t, resp.Message, tc.errMsg)
		})
	}

	equalRecorder := performVipSubordinateRequest(router, 1402, http.MethodPut, "/api/vip/subordinates/1403/discount", `{"topup_discount":0.7}`)
	var equalResp vipSubordinateMutationResponse
	require.NoError(t, common.Unmarshal(equalRecorder.Body.Bytes(), &equalResp))
	assert.True(t, equalResp.Success, equalResp.Message)

	oneRecorder := performVipSubordinateRequest(router, 1402, http.MethodPut, "/api/vip/subordinates/1403/discount", `{"topup_discount":1}`)
	var oneResp vipSubordinateMutationResponse
	require.NoError(t, common.Unmarshal(oneRecorder.Body.Bytes(), &oneResp))
	assert.True(t, oneResp.Success, oneResp.Message)
}

func TestTopupPaymentDiscountUsesUserDiscountBeforeAmountDiscount(t *testing.T) {
	setupVipSubordinateDiscountControllerTest(t)
	seedVipSubordinateUser(t, 1501, "discount_provider", true)
	seedVipSubordinateUser(t, 1502, "discount_buyer", false)
	seedVipSubordinateRelation(t, 1501, 1502, 0.8)

	originalPrice := operation_setting.Price
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalDiscounts := operation_setting.GetPaymentSetting().AmountDiscount
	operation_setting.Price = 2
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{100: 0.5}
	t.Cleanup(func() {
		operation_setting.Price = originalPrice
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
		operation_setting.GetPaymentSetting().AmountDiscount = originalDiscounts
	})

	assert.InDelta(t, 160, getPayMoneyForUser(100, 1502, "default"), 0.000001)
	assert.InDelta(t, 100, getPayMoneyForUser(100, 9999, "default"), 0.000001)
}

func TestCreemTopupRejectedWhenUserDiscountExists(t *testing.T) {
	router := setupVipSubordinateDiscountControllerTest(t)
	seedVipSubordinateUser(t, 1601, "creem_parent", true)
	seedVipSubordinateUser(t, 1602, "creem_child", false)
	seedVipSubordinateRelation(t, 1601, 1602, 0.8)

	originalProducts := setting.CreemProducts
	setting.CreemProducts = `[{"productId":"prod_discount","name":"Discount Product","price":10,"currency":"USD","quota":10}]`
	t.Cleanup(func() {
		setting.CreemProducts = originalProducts
	})

	recorder := performVipSubordinateRequest(router, 1602, http.MethodPost, "/api/user/creem/pay", `{"payment_method":"creem","product_id":"prod_discount"}`)

	var resp vipSubordinateMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "Creem")

	var count int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestVipActivationOrderIgnoresRelationDiscount(t *testing.T) {
	setupVipSubordinateDiscountControllerTest(t)
	seedVipSubordinateUser(t, 1701, "activation_parent", true)
	seedVipSubordinateUser(t, 1702, "activation_child", false)
	require.NoError(t, model.DB.Create(&model.UserRelation{
		ParentUserId: 1701,
		ChildUserId:  1702,
		Source:       model.UserRelationSourceAdmin,
		Status:       model.UserRelationStatusActive,
	}).Error)

	order, err := service.CreateVipActivationOrder(1702, model.PaymentProviderEpay, "alipay")
	require.NoError(t, err)

	assert.InDelta(t, model.DefaultVipActivationAmount, order.ActivationAmount, 0.000001)
	assert.InDelta(t, model.DefaultVipActivationPaid, order.PaidAmount, 0.000001)
	assert.InDelta(t, model.DefaultVipActivationDiscount, order.Discount, 0.000001)
}
