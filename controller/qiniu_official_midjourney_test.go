package controller

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQiniuMidjourneyControllerTest(t *testing.T) {
	t.Helper()

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
	model.InitDBColumnNamesForTests()
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.Midjourney{},
		&model.WalletAccount{},
		&model.WalletFlow{},
	))
	require.NoError(t, db.Create(&model.User{
		Id:       9101,
		Username: "qiniu_mj_user",
		AffCode:  "qiniu_mj_aff",
		Quota:    0,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		_ = sqlDB.Close()
	})
}

func TestQiniuOfficialLedgerMidjourneyFailureSkipsLocalRefund(t *testing.T) {
	setupQiniuMidjourneyControllerTest(t)

	task := &model.Midjourney{
		UserId:        9101,
		MjId:          "mj-failed-qiniu",
		Action:        constant.MjActionImagine,
		ChannelId:     9101,
		Quota:         1000,
		Progress:      "50%",
		Status:        "IN_PROGRESS",
		BillingSource: service.QiniuOfficialLedgerSource(),
		TokenId:       9101,
	}
	require.NoError(t, model.DB.Create(task).Error)

	recordMidjourneyFailureRefund(task, "构图失败")

	var user model.User
	require.NoError(t, model.DB.Select("quota").First(&user, "id = ?", 9101).Error)
	require.Equal(t, 0, user.Quota)
	var flowCount int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).Count(&flowCount).Error)
	require.Equal(t, int64(0), flowCount)

	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log).Error)
	require.Equal(t, model.LogTypeRefund, log.Type)
	require.Equal(t, 0, log.Quota)
	require.Equal(t, 9101, log.TokenId)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	require.Equal(t, service.QiniuOfficialLedgerSource(), other["billing_source"])
	require.Equal(t, true, other["qiniu_official_ledger_pending"])
	require.Equal(t, float64(1000), other["local_estimated_quota"])
}

func TestQiniuMarketMidjourneyFailureRefundsWalletAndTokenSilently(t *testing.T) {
	setupQiniuMidjourneyControllerTest(t)

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 9101).Update("quota", 15000).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:          9101,
		UserId:      9101,
		Key:         "qiniu-token-key",
		Status:      common.TokenStatusEnabled,
		Name:        "qiniu-token",
		RemainQuota: 15000,
		UsedQuota:   5000,
	}).Error)
	require.NoError(t, model.DB.Create(&model.WalletAccount{
		UserId:        9101,
		BalanceAmount: float64(15000) / common.QuotaPerUnit,
	}).Error)

	task := &model.Midjourney{
		UserId:        9101,
		MjId:          "mj-qiniu-market-failed",
		Action:        constant.MjActionImagine,
		ChannelId:     9101,
		Quota:         5000,
		Progress:      "50%",
		Status:        "IN_PROGRESS",
		BillingSource: service.QiniuMarketRealtimeBillingSource,
		FundingSource: service.BillingSourceWallet,
		TokenId:       9101,
	}
	require.NoError(t, model.DB.Create(task).Error)

	recordMidjourneyFailureRefund(task, "构图失败")

	var user model.User
	require.NoError(t, model.DB.Select("quota").First(&user, "id = ?", 9101).Error)
	require.Equal(t, 20000, user.Quota)

	var token model.Token
	require.NoError(t, model.DB.Select("remain_quota", "used_quota").First(&token, "id = ?", 9101).Error)
	require.Equal(t, 20000, token.RemainQuota)
	require.Equal(t, 0, token.UsedQuota)

	var walletAccount model.WalletAccount
	require.NoError(t, model.DB.Select("balance_amount").First(&walletAccount, "user_id = ?", 9101).Error)
	require.InDelta(t, float64(20000)/common.QuotaPerUnit, walletAccount.BalanceAmount, 0.000001)

	var walletFlowCount int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).
		Where("user_id = ? AND biz_no LIKE ?", 9101, "midjourney:%").
		Count(&walletFlowCount).Error)
	require.Equal(t, int64(0), walletFlowCount)

	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, "type = ?", model.LogTypeRefund).Error)
	require.Equal(t, 5000, log.Quota)
	require.Equal(t, 9101, log.TokenId)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	require.Equal(t, service.QiniuMarketRealtimeBillingSource, other["billing_source"])
	require.Equal(t, service.BillingSourceWallet, other["funding_source"])
	require.Equal(t, "mj-qiniu-market-failed", other["task_id"])
}
