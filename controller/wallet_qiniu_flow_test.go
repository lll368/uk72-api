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
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWalletFlowControllerTest(t *testing.T) {
	t.Helper()
	oldUsingSQLite := common.UsingSQLite
	common.UsingSQLite = true

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	model.LOG_DB = db
	model.InitDBColumnNamesForTests()
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.WalletAccount{}, &model.WalletFlow{}))
	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		_ = sqlDB.Close()
	})
}

func TestGetWalletFlowsReturnsQiniuFinalConsumptionWithoutInternalRows(t *testing.T) {
	setupWalletFlowControllerTest(t)

	const userID = 9401
	require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: "qiniu_wallet_user", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.WalletFlow{
		UserId:       userID,
		BizNo:        "qiniu:realtime:request:req-wallet-flow",
		FlowType:     model.WalletFlowTypeBalanceConsume,
		WalletType:   model.WalletTypeBalance,
		Direction:    model.WalletFlowDirectionOut,
		Amount:       0.01,
		BalanceAfter: 9.99,
		Remark:       "市场价实时 token/model 消费 request=req-wallet-flow",
	}).Error)
	require.NoError(t, model.DB.Create(&model.WalletFlow{
		UserId:       userID,
		BizNo:        "topup-wallet-flow",
		FlowType:     model.WalletFlowTypeRechargeBalance,
		WalletType:   model.WalletTypeBalance,
		Direction:    model.WalletFlowDirectionIn,
		Amount:       10,
		BalanceAfter: 10,
		Remark:       "充值入消费余额",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/wallet/flows?p=1&page_size=10", nil)
	ctx.Set("id", userID)
	GetWalletFlows(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response map[string]interface{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, float64(2), data["total"])
	items, ok := data["items"].([]interface{})
	require.True(t, ok)
	require.Len(t, items, 2)

	var foundQiniu bool
	for _, item := range items {
		flow, ok := item.(map[string]interface{})
		require.True(t, ok)
		require.NotContains(t, []string{"wallet-preconsume", "wallet-reserve", "wallet-settle", "wallet-refund"}, flow["biz_no"])
		if flow["biz_no"] == "qiniu:realtime:request:req-wallet-flow" {
			foundQiniu = true
			require.Equal(t, model.WalletFlowTypeBalanceConsume, flow["flow_type"])
			require.Equal(t, model.WalletFlowDirectionOut, flow["direction"])
			require.Contains(t, flow["remark"], "token/model")
			require.NotContains(t, flow["remark"], "七牛")
			require.NotContains(t, flow["remark"], "Qiniu")
		}
	}
	require.True(t, foundQiniu)
}
