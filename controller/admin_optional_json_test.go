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
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type adminOptionalJsonResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupAdminOptionalJsonControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

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
		&model.UserProfile{},
		&model.VipActivationRecord{},
		&model.WalletAccount{},
		&model.WalletFlow{},
		&model.WithdrawOrder{},
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
	router.POST("/api/vip/admin/users/:id/disable", AdminDisableVipActivation)
	router.POST("/api/wallet/admin/withdraws/:id/approve", AdminApproveWalletWithdraw)
	router.POST("/api/wallet/admin/withdraws/:id/reject", AdminRejectWalletWithdraw)
	router.POST("/api/wallet/admin/withdraws/:id/pay", AdminPayWalletWithdraw)
	router.POST("/api/wallet/admin/withdraws/:id/fail", AdminFailWalletWithdraw)
	return router
}

func TestAdminDisableVipActivationRejectsMalformedBody(t *testing.T) {
	router := setupAdminOptionalJsonControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1301,
		Username: "malformed_vvip_user",
		Status:   common.UserStatusEnabled,
	}).Error)
	record := model.VipActivationRecord{
		UserId:          1301,
		TradeNo:         "malformed-vvip-disable",
		PaymentProvider: model.PaymentProviderEpay,
		PaymentMethod:   "alipay",
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&record).Error)

	req := httptest.NewRequest(http.MethodPost, "/api/vip/admin/users/1301/disable", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var resp adminOptionalJsonResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)

	var unchanged model.VipActivationRecord
	require.NoError(t, model.DB.First(&unchanged, record.Id).Error)
	assert.Equal(t, model.VipActivationStatusSuccess, unchanged.Status)
	assert.Zero(t, unchanged.DisabledAt)
	assert.Zero(t, unchanged.DisabledBy)
}

func TestAdminWalletWithdrawActionsRejectMalformedBody(t *testing.T) {
	tests := []struct {
		name          string
		pathSuffix    string
		initialStatus string
	}{
		{name: "approve", pathSuffix: "approve", initialStatus: model.WithdrawStatusPending},
		{name: "reject", pathSuffix: "reject", initialStatus: model.WithdrawStatusPending},
		{name: "pay", pathSuffix: "pay", initialStatus: model.WithdrawStatusApproved},
		{name: "fail", pathSuffix: "fail", initialStatus: model.WithdrawStatusApproved},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupAdminOptionalJsonControllerTest(t)
			userId := 1400 + index
			require.NoError(t, model.DB.Create(&model.User{
				Id:       userId,
				Username: "withdraw_" + tt.name,
				Status:   common.UserStatusEnabled,
			}).Error)
			require.NoError(t, model.DB.Create(&model.WalletAccount{
				UserId:                 userId,
				CommissionAmount:       0,
				FrozenCommissionAmount: 100,
				TotalWithdrawAmount:    0,
			}).Error)
			order := model.WithdrawOrder{
				UserId:         userId,
				WithdrawNo:     fmt.Sprintf("WDR-MALFORMED-%s", tt.name),
				Amount:         25,
				ActualAmount:   25,
				Status:         tt.initialStatus,
				ReceiveType:    "bank",
				ReceiveAccount: "account",
			}
			require.NoError(t, model.DB.Create(&order).Error)

			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/wallet/admin/withdraws/%d/%s", order.Id, tt.pathSuffix), bytes.NewBufferString(`{`))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			var resp adminOptionalJsonResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
			assert.False(t, resp.Success)

			var unchanged model.WithdrawOrder
			require.NoError(t, model.DB.First(&unchanged, order.Id).Error)
			assert.Equal(t, tt.initialStatus, unchanged.Status)
			assert.Zero(t, unchanged.ReviewerId)
			assert.Empty(t, unchanged.PaymentVoucher)
			assert.Empty(t, unchanged.FailReason)

			var account model.WalletAccount
			require.NoError(t, model.DB.Where("user_id = ?", userId).First(&account).Error)
			assert.InDelta(t, 0, account.CommissionAmount, 0.000001)
			assert.InDelta(t, 100, account.FrozenCommissionAmount, 0.000001)
			assert.InDelta(t, 0, account.TotalWithdrawAmount, 0.000001)

			var flowCount int64
			require.NoError(t, model.DB.Model(&model.WalletFlow{}).Where("biz_no = ?", order.WithdrawNo).Count(&flowCount).Error)
			assert.Zero(t, flowCount)
		})
	}
}
