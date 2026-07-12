package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type adminUserRelationAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    common.PageInfo `json:"data"`
}

type adminUserRelationMutationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupAdminUserRelationControllerTest(t *testing.T) *gin.Engine {
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
		&model.Log{},
		&model.VipActivationRecord{},
		&model.UserRelation{},
	))

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		_ = sqlDB.Close()
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 9001)
		c.Next()
	})
	router.GET("/api/vip/admin/relations", AdminListUserRelations)
	router.POST("/api/vip/admin/relations", AdminCreateUserRelation)
	router.POST("/api/vip/admin/relations/:id/disable", AdminDisableUserRelation)
	return router
}

func seedAdminUserRelationUser(t *testing.T, id int, username string, activeVvip bool) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       id,
		Username: username,
		AffCode:  fmt.Sprintf("rel%d", id),
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
	}).Error)
	if activeVvip {
		require.NoError(t, model.DB.Create(&model.VipActivationRecord{
			UserId:          id,
			TradeNo:         fmt.Sprintf("relation-vvip-%d", id),
			PaymentProvider: model.PaymentProviderEpay,
			PaymentMethod:   "alipay",
			Status:          model.VipActivationStatusSuccess,
			ActivatedAt:     time.Now().Unix(),
		}).Error)
	}
}

func disableAdminUserRelationUser(t *testing.T, id int) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", id).Update("status", common.UserStatusDisabled).Error)
}

func TestAdminCreateUserRelationRequiresActiveVvipParent(t *testing.T) {
	router := setupAdminUserRelationControllerTest(t)
	seedAdminUserRelationUser(t, 1001, "non_vvip_parent", false)
	seedAdminUserRelationUser(t, 1002, "relation_child", false)

	req := httptest.NewRequest(http.MethodPost, "/api/vip/admin/relations", bytes.NewBufferString(`{"parent_user_id":1001,"child_user_id":1002}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp adminUserRelationMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "Compute Partner")

	var count int64
	require.NoError(t, model.DB.Model(&model.UserRelation{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAdminCreateUserRelationRequiresEnabledUsers(t *testing.T) {
	router := setupAdminUserRelationControllerTest(t)
	seedAdminUserRelationUser(t, 1051, "enabled_vvip_parent", true)
	seedAdminUserRelationUser(t, 1052, "disabled_relation_child", false)
	disableAdminUserRelationUser(t, 1052)

	req := httptest.NewRequest(http.MethodPost, "/api/vip/admin/relations", bytes.NewBufferString(`{"parent_user_id":1051,"child_user_id":1052}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp adminUserRelationMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "enabled")

	var count int64
	require.NoError(t, model.DB.Model(&model.UserRelation{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAdminCreateListAndDisableUserRelation(t *testing.T) {
	router := setupAdminUserRelationControllerTest(t)
	seedAdminUserRelationUser(t, 1101, "active_vvip_parent", true)
	seedAdminUserRelationUser(t, 1102, "active_relation_child", false)

	createReq := httptest.NewRequest(http.MethodPost, "/api/vip/admin/relations", bytes.NewBufferString(`{"parent_user_id":1101,"child_user_id":1102,"source_trade_no":"manual-1102","remark":"manual binding"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, createReq)

	var createResp adminUserRelationMutationResponse
	require.NoError(t, common.Unmarshal(createRecorder.Body.Bytes(), &createResp))
	require.True(t, createResp.Success, createResp.Message)

	listReq := httptest.NewRequest(http.MethodGet, "/api/vip/admin/relations?parent_user_id=1101&status=active", nil)
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listReq)

	var listResp adminUserRelationAPIResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Equal(t, 1, listResp.Data.Total)

	var relation model.UserRelation
	require.NoError(t, model.DB.Where("parent_user_id = ? AND child_user_id = ?", 1101, 1102).First(&relation).Error)
	assert.Equal(t, model.UserRelationSourceAdmin, relation.Source)
	assert.Equal(t, model.UserRelationStatusActive, relation.Status)
	require.NotNil(t, relation.ActiveChildId)

	disableReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/vip/admin/relations/%d/disable", relation.Id), bytes.NewBufferString(`{"reason":"invalid relation"}`))
	disableReq.Header.Set("Content-Type", "application/json")
	disableRecorder := httptest.NewRecorder()
	router.ServeHTTP(disableRecorder, disableReq)

	var disableResp adminUserRelationMutationResponse
	require.NoError(t, common.Unmarshal(disableRecorder.Body.Bytes(), &disableResp))
	require.True(t, disableResp.Success, disableResp.Message)

	var disabled model.UserRelation
	require.NoError(t, model.DB.First(&disabled, relation.Id).Error)
	assert.Equal(t, model.UserRelationStatusDisabled, disabled.Status)
	assert.Nil(t, disabled.ActiveChildId)
}

func TestAdminDisableUserRelationRejectsMalformedBody(t *testing.T) {
	router := setupAdminUserRelationControllerTest(t)
	seedAdminUserRelationUser(t, 1201, "malformed_vvip_parent", true)
	seedAdminUserRelationUser(t, 1202, "malformed_relation_child", false)

	relation, err := model.CreateActiveUserRelationTx(model.DB, 1201, 1202, model.UserRelationSourceAdmin, "malformed-disable-1202")
	require.NoError(t, err)
	require.NotNil(t, relation.ActiveChildId)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/vip/admin/relations/%d/disable", relation.Id), bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp adminUserRelationMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)

	var unchanged model.UserRelation
	require.NoError(t, model.DB.First(&unchanged, relation.Id).Error)
	assert.Equal(t, model.UserRelationStatusActive, unchanged.Status)
	assert.NotNil(t, unchanged.ActiveChildId)
}
