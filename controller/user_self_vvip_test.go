package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type getSelfVvipResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Id              int     `json:"id"`
		Username        string  `json:"username"`
		IsVvip          *bool   `json:"is_vvip"`
		VvipStatus      *string `json:"vvip_status"`
		VvipActivatedAt *int64  `json:"vvip_activated_at"`
		VvipDisabledAt  *int64  `json:"vvip_disabled_at"`
	} `json:"data"`
}

func setupGetSelfVvipControllerTest(t *testing.T) {
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
		&model.UserProfile{},
		&model.VipActivationRecord{},
	))

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		_ = sqlDB.Close()
	})
}

func seedGetSelfVvipUser(t *testing.T, id int, username string, profile *model.UserProfile) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:          id,
		Username:    username,
		Password:    "password123",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     fmt.Sprintf("self%d", id),
	}).Error)
	if profile != nil {
		profile.UserId = id
		require.NoError(t, model.DB.Create(profile).Error)
	}
}

func performGetSelfVvipRequest(t *testing.T, userId int) getSelfVvipResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	ctx.Set("id", userId)
	ctx.Set("role", common.RoleCommonUser)

	GetSelf(ctx)

	var resp getSelfVvipResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	return resp
}

func TestGetSelfIncludesVvipStatusFields(t *testing.T) {
	setupGetSelfVvipControllerTest(t)
	seedGetSelfVvipUser(t, 1801, "active_self_vvip", &model.UserProfile{
		IsVvip:          true,
		VvipStatus:      model.VvipStatusActive,
		VvipActivatedAt: 1700000101,
	})
	seedGetSelfVvipUser(t, 1802, "plain_self_user", nil)

	activeResp := performGetSelfVvipRequest(t, 1801)
	require.NotNil(t, activeResp.Data.IsVvip)
	require.NotNil(t, activeResp.Data.VvipStatus)
	require.NotNil(t, activeResp.Data.VvipActivatedAt)
	require.NotNil(t, activeResp.Data.VvipDisabledAt)
	assert.True(t, *activeResp.Data.IsVvip)
	assert.Equal(t, model.VvipStatusActive, *activeResp.Data.VvipStatus)
	assert.Equal(t, int64(1700000101), *activeResp.Data.VvipActivatedAt)
	assert.Zero(t, *activeResp.Data.VvipDisabledAt)

	plainResp := performGetSelfVvipRequest(t, 1802)
	require.NotNil(t, plainResp.Data.IsVvip)
	require.NotNil(t, plainResp.Data.VvipStatus)
	assert.False(t, *plainResp.Data.IsVvip)
	assert.Equal(t, model.VvipStatusNone, *plainResp.Data.VvipStatus)
}
