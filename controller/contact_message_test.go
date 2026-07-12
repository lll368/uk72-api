package controller

import (
	"bytes"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type contactMessageAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    common.PageInfo `json:"data"`
}

type contactMessageMutationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupContactMessageControllerTest(t *testing.T) *gin.Engine {
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
	require.NoError(t, db.AutoMigrate(&model.ContactMessage{}))

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		_ = sqlDB.Close()
	})

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("contact-message-test"))))
	router.GET("/login/:role", func(c *gin.Context) {
		role := common.RoleCommonUser
		id := 9002
		if c.Param("role") == "admin" {
			role = common.RoleAdminUser
			id = 9001
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
	router.POST("/api/contact/messages", SubmitContactMessage)
	admin := router.Group("/api/contact/admin")
	admin.Use(middleware.AdminAuth())
	admin.GET("/messages", AdminListContactMessages)
	admin.PUT("/messages/:id", AdminUpdateContactMessage)
	admin.DELETE("/messages/:id", AdminDeleteContactMessage)
	return router
}

func loginContactMessageTestUser(t *testing.T, router *gin.Engine, role string, userId string) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/login/"+role, nil)
	request.Header.Set("New-Api-User", userId)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func TestSubmitContactMessageCreatesRecord(t *testing.T) {
	router := setupContactMessageControllerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/contact/messages", bytes.NewBufferString(`{"name":"Alice","phone":"13800138000","message":"Need details"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp contactMessageMutationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var saved model.ContactMessage
	require.NoError(t, model.DB.First(&saved).Error)
	assert.Equal(t, "Alice", saved.Name)
	assert.Equal(t, model.ContactMessageStatusPending, saved.Status)
}

func TestAdminContactMessagesRequireAdminAuth(t *testing.T) {
	router := setupContactMessageControllerTest(t)
	require.NoError(t, model.CreateContactMessage(&model.ContactMessage{Name: "Alice", Phone: "13800138000"}))

	anonymousRecorder := httptest.NewRecorder()
	anonymousReq := httptest.NewRequest(http.MethodGet, "/api/contact/admin/messages", nil)
	router.ServeHTTP(anonymousRecorder, anonymousReq)
	assert.Equal(t, http.StatusUnauthorized, anonymousRecorder.Code)

	userCookies := loginContactMessageTestUser(t, router, "user", "9002")
	userRecorder := httptest.NewRecorder()
	userReq := httptest.NewRequest(http.MethodGet, "/api/contact/admin/messages", nil)
	userReq.Header.Set("New-Api-User", "9002")
	for _, cookie := range userCookies {
		userReq.AddCookie(cookie)
	}
	router.ServeHTTP(userRecorder, userReq)

	var userResp contactMessageMutationResponse
	require.NoError(t, common.Unmarshal(userRecorder.Body.Bytes(), &userResp))
	assert.False(t, userResp.Success)

	adminCookies := loginContactMessageTestUser(t, router, "admin", "9001")
	adminRecorder := httptest.NewRecorder()
	adminReq := httptest.NewRequest(http.MethodGet, "/api/contact/admin/messages", nil)
	adminReq.Header.Set("New-Api-User", "9001")
	for _, cookie := range adminCookies {
		adminReq.AddCookie(cookie)
	}
	router.ServeHTTP(adminRecorder, adminReq)

	var adminResp contactMessageAPIResponse
	require.NoError(t, common.Unmarshal(adminRecorder.Body.Bytes(), &adminResp))
	require.True(t, adminResp.Success, adminResp.Message)
	assert.Equal(t, 1, adminResp.Data.Total)
}

func TestAdminUpdateAndDeleteContactMessage(t *testing.T) {
	router := setupContactMessageControllerTest(t)
	record := &model.ContactMessage{Name: "Alice", Phone: "13800138000"}
	require.NoError(t, model.CreateContactMessage(record))

	cookies := loginContactMessageTestUser(t, router, "admin", "9001")
	updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/contact/admin/messages/%d", record.Id), bytes.NewBufferString(`{"status":"contacted","remark":"called"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("New-Api-User", "9001")
	for _, cookie := range cookies {
		updateReq.AddCookie(cookie)
	}
	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, updateReq)

	var updateResp contactMessageMutationResponse
	require.NoError(t, common.Unmarshal(updateRecorder.Body.Bytes(), &updateResp))
	require.True(t, updateResp.Success, updateResp.Message)

	var updated model.ContactMessage
	require.NoError(t, model.DB.First(&updated, record.Id).Error)
	assert.Equal(t, model.ContactMessageStatusContacted, updated.Status)
	assert.Equal(t, "called", updated.Remark)
	assert.Equal(t, 9001, updated.ProcessedBy)
	assert.NotZero(t, updated.ProcessedAt)

	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/contact/admin/messages/%d", record.Id), nil)
	deleteReq.Header.Set("New-Api-User", "9001")
	for _, cookie := range cookies {
		deleteReq.AddCookie(cookie)
	}
	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, deleteReq)

	var deleteResp contactMessageMutationResponse
	require.NoError(t, common.Unmarshal(deleteRecorder.Body.Bytes(), &deleteResp))
	require.True(t, deleteResp.Success, deleteResp.Message)

	var count int64
	require.NoError(t, model.DB.Model(&model.ContactMessage{}).Count(&count).Error)
	assert.Zero(t, count)
}
