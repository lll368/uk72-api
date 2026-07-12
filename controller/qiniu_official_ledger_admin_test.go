package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type qiniuOfficialUsageListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int                            `json:"total"`
		Items []qiniuOfficialUsageRecordView `json:"items"`
	} `json:"data"`
}

type qiniuOfficialLedgerApplicationListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int                                  `json:"total"`
		Items []qiniuOfficialLedgerApplicationView `json:"items"`
	} `json:"data"`
}

type qiniuOfficialRetryResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Applied       bool   `json:"applied"`
		Skipped       bool   `json:"skipped"`
		UsageRecordId int    `json:"usage_record_id"`
		Message       string `json:"message"`
	} `json:"data"`
}

func setupQiniuOfficialAdminControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())

	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	qiniuSetting := operation_setting.GetQiniuKeySetting()
	oldQiniuSetting := *qiniuSetting
	qiniuSetting.AccessKey = "qiniu-ak-secret"
	qiniuSetting.SecretKey = "qiniu-sk-secret"
	qiniuSetting.CostDetailAutoApplyEnabled = true
	qiniuSetting.CostDetailCutoverTime = time.Date(2026, 6, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600)).Unix()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	model.LOG_DB = db
	model.InitDBColumnNamesForTests()
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.WalletAccount{},
		&model.WalletFlow{},
		&model.Log{},
		&model.QiniuKeySyncTask{},
		&model.QiniuOfficialUsageRecord{},
		&model.QiniuOfficialLedgerApplication{},
		&model.QiniuRealtimeWalletApplication{},
		&model.QiniuCostDetailRecord{},
		&model.QiniuBillingBucket{},
		&model.QiniuBillingBucketItem{},
		&model.QiniuBillingBucketApplication{},
		&model.QiniuQuotaGrant{},
		&model.QiniuChildAccount{},
		&model.QiniuChildAccountSyncTask{},
	))

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.RedisEnabled = oldRedisEnabled
		*qiniuSetting = oldQiniuSetting
		_ = sqlDB.Close()
	})

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("qiniu-official-admin-test"))))
	router.GET("/login/:role", func(c *gin.Context) {
		role := common.RoleCommonUser
		id := 8602
		if c.Param("role") == "admin" {
			role = common.RoleAdminUser
			id = 8601
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
	admin := router.Group("/api/payment/admin")
	admin.Use(middleware.AdminAuth())
	admin.GET("/qiniu-official-records", AdminListQiniuOfficialUsageRecords)
	admin.GET("/qiniu-keys", AdminListQiniuKeys)
	admin.POST("/qiniu-keys/:id/disable", AdminDisableQiniuKey)
	admin.GET("/qiniu-child-accounts", AdminListQiniuChildAccounts)
	admin.POST("/qiniu-child-accounts", AdminCreateQiniuChildAccount)
	admin.GET("/qiniu-child-accounts/:id", AdminGetQiniuChildAccount)
	admin.POST("/qiniu-child-accounts/:id/disable", AdminDisableQiniuChildAccount)
	admin.POST("/qiniu-child-accounts/:id/enable", AdminEnableQiniuChildAccount)
	admin.GET("/qiniu-child-account-tasks", AdminListQiniuChildAccountTasks)
	admin.POST("/qiniu-child-account-tasks/:id/retry", AdminRetryQiniuChildAccountTask)
	admin.POST("/qiniu-official-records/:id/retry", AdminRetryQiniuOfficialUsageRecord)
	admin.GET("/qiniu-official-ledger-applications", AdminListQiniuOfficialLedgerApplications)
	admin.POST("/qiniu-official-ledger-applications/:id/retry", AdminRetryQiniuOfficialLedgerApplication)
	admin.GET("/qiniu-billing-summary", AdminGetQiniuBillingSummary)
	admin.GET("/qiniu-billing-buckets", AdminListQiniuBillingBuckets)
	admin.POST("/qiniu-billing-buckets/:id/recalculate", AdminRecalculateQiniuBillingBucket)
	admin.POST("/qiniu-billing-buckets/:id/resolve", AdminResolveQiniuBillingBucket)
	admin.POST("/qiniu-billing-buckets/:id/skip", AdminSkipQiniuBillingBucket)
	admin.GET("/qiniu-cost-detail-records", AdminListQiniuCostDetailRecords)
	admin.POST("/qiniu-cost-detail-records/:id/resolve", AdminResolveQiniuCostDetailRecord)
	admin.GET("/qiniu-billing-bucket-items", AdminListQiniuBillingBucketItems)
	admin.GET("/qiniu-billing-bucket-applications", AdminListQiniuBillingBucketApplications)
	admin.POST("/qiniu-billing-bucket-applications/:id/retry", AdminRetryQiniuBillingBucketApplication)
	return router
}

func loginQiniuOfficialAdminTestUser(t *testing.T, router *gin.Engine, role string, userId string) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/login/"+role, nil)
	request.Header.Set("New-Api-User", userId)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func TestAdminQiniuOfficialRecordsRequireAdminAuth(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)

	anonymousRecorder := httptest.NewRecorder()
	anonymousReq := httptest.NewRequest(http.MethodGet, "/api/payment/admin/qiniu-official-records", nil)
	router.ServeHTTP(anonymousRecorder, anonymousReq)
	require.Equal(t, http.StatusUnauthorized, anonymousRecorder.Code)

	userCookies := loginQiniuOfficialAdminTestUser(t, router, "user", "8602")
	userRecorder := httptest.NewRecorder()
	userReq := httptest.NewRequest(http.MethodGet, "/api/payment/admin/qiniu-official-records", nil)
	userReq.Header.Set("New-Api-User", "8602")
	for _, cookie := range userCookies {
		userReq.AddCookie(cookie)
	}
	router.ServeHTTP(userRecorder, userReq)
	require.Equal(t, http.StatusOK, userRecorder.Code)
	var resp qiniuOfficialUsageListResponse
	require.NoError(t, common.Unmarshal(userRecorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
}

func TestAdminListQiniuOfficialRecordsFiltersAndMasksSensitiveData(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	now := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC).Unix()
	fullKey := "sk-" + strings.Repeat("a", 64)
	require.NoError(t, model.DB.Create(&model.QiniuOfficialUsageRecord{
		RecordKey:           "qiniu-official-record-mask",
		RecordType:          model.QiniuOfficialRecordTypeBill,
		SourceAPI:           "/v2/stat/usage/apikey/cost-detail",
		RecordHash:          "hash-mask",
		QiniuKey:            fullKey,
		UserId:              8701,
		TokenId:             8701,
		QiniuChildAccountId: 8702,
		PeriodStart:         now,
		PeriodEnd:           now + 3600,
		Granularity:         "hour",
		ModelName:           "deepseek-v3",
		BillingItem:         "input",
		FeeAmount:           1.25,
		Currency:            "CNY",
		OfficialQuota:       625000,
		Status:              model.QiniuOfficialRecordStatusFailed,
		LastError:           "upstream key " + fullKey + " failed",
		RawResponse:         `{"api_key":"` + fullKey + `","ak":"qiniu-ak-secret","sk":"qiniu-sk-secret"}`,
		FetchedAt:           now,
	}).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/payment/admin/qiniu-official-records?user_id=8701&token_id=8701&qiniu_child_account_id=8702&status=failed&model=deepseek-v3&billing_item=input&period_start="+strconvFormatInt64(now), nil)
	req.Header.Set("New-Api-User", "8601")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp qiniuOfficialUsageListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Equal(t, 1, resp.Data.Total)
	require.Len(t, resp.Data.Items, 1)
	item := resp.Data.Items[0]
	require.Equal(t, 8702, item.QiniuChildAccountId)
	require.NotEqual(t, fullKey, item.QiniuKey)
	require.NotContains(t, item.RawResponse, fullKey)
	require.NotContains(t, item.RawResponse, "qiniu-ak-secret")
	require.NotContains(t, item.RawResponse, "qiniu-sk-secret")
	require.NotContains(t, item.LastError, fullKey)
}

func TestAdminListQiniuOfficialLedgerApplicationsFilters(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	require.NoError(t, model.DB.Create(&model.QiniuOfficialLedgerApplication{
		UsageRecordId:  9101,
		ApplyVersion:   2,
		UserId:         9102,
		TokenId:        9103,
		DeltaQuota:     -500,
		DeltaAmount:    -0.001,
		IdempotencyKey: "qiniu:usage_apply:9101:v2",
		Status:         model.QiniuOfficialLedgerStatusSuccess,
	}).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/payment/admin/qiniu-official-ledger-applications?status=success&user_id=9102&token_id=9103&usage_record_id=9101", nil)
	req.Header.Set("New-Api-User", "8601")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp qiniuOfficialLedgerApplicationListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Equal(t, 1, resp.Data.Total)
	require.Len(t, resp.Data.Items, 1)
	require.Equal(t, -500, resp.Data.Items[0].DeltaQuota)
}

func TestAdminRetryQiniuOfficialRecordReturnsObservationOnlyState(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	userId := 8801
	tokenId := 8801
	now := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, model.DB.Create(&model.User{
		Id:          userId,
		Username:    "qiniu-user",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       int(20 * common.QuotaPerUnit),
		DisplayName: "qiniu-user",
		Group:       "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             tokenId,
		UserId:         userId,
		Name:           "qiniu-token",
		Key:            strings.Repeat("b", 64),
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	require.NoError(t, model.DB.Create(&model.WalletAccount{
		UserId:        userId,
		BalanceAmount: 20,
	}).Error)
	record := &model.QiniuOfficialUsageRecord{
		RecordKey:     "qiniu-official-record-retry",
		RecordType:    model.QiniuOfficialRecordTypeBill,
		SourceAPI:     "/v2/stat/usage/apikey/cost-detail",
		RecordHash:    "hash-retry",
		QiniuKey:      "sk-" + strings.Repeat("b", 64),
		UserId:        userId,
		TokenId:       tokenId,
		PeriodStart:   now,
		PeriodEnd:     now + 3600,
		Granularity:   "hour",
		ModelName:     "deepseek-v3",
		BillingItem:   "input",
		FeeAmount:     2,
		Currency:      "CNY",
		OfficialQuota: int(2 * common.QuotaPerUnit),
		Status:        model.QiniuOfficialRecordStatusFailed,
		RawResponse:   `{"fee":2}`,
		FetchedAt:     now,
	}
	require.NoError(t, model.DB.Create(record).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/payment/admin/qiniu-official-records/%d/retry", record.Id), nil)
	req.Header.Set("New-Api-User", "8601")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp qiniuOfficialRetryResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.False(t, resp.Data.Applied)
	require.True(t, resp.Data.Skipped)
	require.NotEmpty(t, resp.Data.Message)
	require.Equal(t, record.Id, resp.Data.UsageRecordId)

	var reloaded model.QiniuOfficialUsageRecord
	require.NoError(t, model.DB.First(&reloaded, "id = ?", record.Id).Error)
	require.Equal(t, model.QiniuOfficialRecordStatusFailed, reloaded.Status)
	require.Equal(t, 0, reloaded.AppliedQuota)
	var flowCount int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).Count(&flowCount).Error)
	require.Equal(t, int64(0), flowCount)
	var quotaTaskCount int64
	require.NoError(t, model.DB.Model(&model.QiniuKeySyncTask{}).
		Where("user_id = ? AND token_id = ? AND task_type = ?", userId, tokenId, model.QiniuKeyTaskTypeQuotaSync).
		Count(&quotaTaskCount).Error)
	require.Equal(t, int64(0), quotaTaskCount)
}

func strconvFormatInt64(value int64) string {
	return fmt.Sprintf("%d", value)
}
