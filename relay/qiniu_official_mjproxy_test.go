package relay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQiniuMjProxyTest(t *testing.T, path string, body string) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	oldUsingSQLite := common.UsingSQLite
	oldRedisEnabled := common.RedisEnabled
	oldLogConsumeEnabled := common.LogConsumeEnabled
	common.UsingSQLite = true
	common.RedisEnabled = false
	common.LogConsumeEnabled = true

	qiniuSetting := operation_setting.GetQiniuKeySetting()
	oldQiniuSetting := *qiniuSetting
	qiniuSetting.OfficialLedgerEnabled = true
	qiniuSetting.AccessKey = "ak"
	qiniuSetting.SecretKey = "sk"
	qiniuSetting.MarketCatalogEnabled = true
	qiniuSetting.MarketCatalogFallbackEnabled = false
	qiniuSetting.MarketCatalogTTLSeconds = 60
	market := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/market/models" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"data":[` +
			qiniuMjMarketModelJSON("mj_imagine") + `,` +
			qiniuMjMarketModelJSON("swap_face") +
			`]}`))
	}))
	qiniuSetting.MarketCatalogBaseURL = market.URL
	oldModelPrice := ratio_setting.ModelPrice2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"mj_imagine":0.01,"swap_face":0.01}`))
	service.InitHttpClient()
	snapshot := service.GetQiniuMarketCatalogSnapshot(context.Background())
	require.Equal(t, service.QiniuMarketCatalogStatusFresh, snapshot.Status)

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
		&model.Channel{},
		&model.WalletAccount{},
		&model.WalletFlow{},
		&model.QiniuRealtimeWalletApplication{},
	))
	require.NoError(t, db.Create(&model.User{
		Id:       9101,
		Username: "qiniu_mj_user",
		AffCode:  "qiniu_mj_aff",
		Quota:    20000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          9101,
		UserId:      9101,
		Key:         "qiniu-token-key",
		Status:      common.TokenStatusEnabled,
		Name:        "qiniu-token",
		RemainQuota: 20000,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     9101,
		Name:   "qiniu-mj-channel",
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
	}).Error)

	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
		common.LogConsumeEnabled = oldLogConsumeEnabled
		*qiniuSetting = oldQiniuSetting
		_ = ratio_setting.UpdateModelPriceByJSONString(oldModelPrice)
		market.Close()
		_ = sqlDB.Close()
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 9101)
	ctx.Set("token_name", "qiniu-token")
	ctx.Set("channel_id", 9101)
	ctx.Set("base_url", "")
	requestID := "req-" + strings.ReplaceAll(t.Name(), "/", "-")
	ctx.Set(common.RequestIdKey, requestID)

	info := &relaycommon.RelayInfo{
		UserId:            9101,
		TokenId:           9101,
		TokenKey:          "qiniu-token-key",
		QiniuManagedToken: true,
		UsingGroup:        "default",
		UserGroup:         "default",
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 9101},
		RequestURLPath:    path,
		RequestId:         requestID,
	}
	info.UserSetting.BillingPreference = "wallet_only"
	return ctx, recorder, info
}

func qiniuMjMarketModelJSON(modelID string) string {
	return fmt.Sprintf(`{"id":%q,"pricing_rules_v2":[{"details_v2":{"request":{"unit_name":"request","unit_size":1,"unit_price":0.01}}}]}`, modelID)
}

func TestRelayMidjourneySubmitQiniuMarketRealtimeBilling(t *testing.T) {
	ctx, _, info := setupQiniuMjProxyTest(t, "/mj/submit/imagine", `{"prompt":"test prompt"}`)
	preConsumeCheck := make(chan error, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mj/submit/imagine" {
			preConsumeCheck <- fmt.Errorf("unexpected upstream path: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		preConsumeCheck <- checkQiniuMarketPreConsumed(5000)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"description":"submitted","result":"mj-qiniu-1"}`))
	}))
	defer upstream.Close()
	ctx.Set("base_url", upstream.URL)
	info.OriginModelName = "mj_imagine"
	info.RelayMode = relayconstant.RelayModeMidjourneyImagine

	resp := RelayMidjourneySubmit(ctx, info)
	require.Nil(t, resp)
	require.NoError(t, receiveQiniuMarketCheck(preConsumeCheck))

	var user model.User
	require.NoError(t, model.DB.Select("quota", "used_quota").First(&user, "id = ?", 9101).Error)
	require.Equal(t, 15000, user.Quota)
	require.Equal(t, 5000, user.UsedQuota)
	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log).Error)
	require.Equal(t, 5000, log.Quota)
	require.Contains(t, log.Content, "市场价实时扣费")
	require.NotContains(t, log.Content, "模型固定价格")
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	require.Equal(t, service.QiniuMarketRealtimeBillingSource, other["billing_source"])
	require.Equal(t, service.QiniuMarketPriceSource, other["price_source"])
	require.Equal(t, "request", other["qiniu_market_unit_name"])
	require.Equal(t, float64(5000), other["qiniu_market_converted_quota"])
	requireQiniuRealtimeWalletFlow(t, info.RequestId, log.Id, 5000)
}

func TestRelaySwapFaceQiniuMarketRealtimeBilling(t *testing.T) {
	ctx, _, info := setupQiniuMjProxyTest(t, "/mj/insight-face/swap", `{"sourceBase64":"a","targetBase64":"b"}`)
	preConsumeCheck := make(chan error, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mj/insight-face/swap" {
			preConsumeCheck <- fmt.Errorf("unexpected upstream path: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		preConsumeCheck <- checkQiniuMarketPreConsumed(5000)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"description":"submitted","result":"swap-qiniu-1"}`))
	}))
	defer upstream.Close()
	ctx.Set("base_url", upstream.URL)
	info.OriginModelName = "swap_face"
	info.RelayMode = relayconstant.RelayModeSwapFace
	info.StartTime = time.Now()

	resp := RelaySwapFace(ctx, info)
	require.Nil(t, resp)
	require.NoError(t, receiveQiniuMarketCheck(preConsumeCheck))

	var user model.User
	require.NoError(t, model.DB.Select("quota", "used_quota").First(&user, "id = ?", 9101).Error)
	require.Equal(t, 15000, user.Quota)
	require.Equal(t, 5000, user.UsedQuota)
	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log).Error)
	require.Equal(t, 5000, log.Quota)
	require.Contains(t, log.Content, "市场价实时扣费")
	require.NotContains(t, log.Content, "模型固定价格")
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	require.Equal(t, service.QiniuMarketRealtimeBillingSource, other["billing_source"])
	require.Equal(t, service.QiniuMarketPriceSource, other["price_source"])
	require.Equal(t, "request", other["qiniu_market_unit_name"])
	require.Equal(t, float64(5000), other["qiniu_market_converted_quota"])
	requireQiniuRealtimeWalletFlow(t, info.RequestId, log.Id, 5000)
}

func TestRelayMidjourneySubmitRefundsQiniuMarketPreConsumeOnNonBillableResponse(t *testing.T) {
	ctx, _, info := setupQiniuMjProxyTest(t, "/mj/submit/imagine", `{"prompt":"test prompt"}`)
	preConsumeCheck := make(chan error, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mj/submit/imagine" {
			preConsumeCheck <- fmt.Errorf("unexpected upstream path: %s", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		preConsumeCheck <- checkQiniuMarketPreConsumed(5000)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":23,"description":"queue full","result":"mj-qiniu-rejected"}`))
	}))
	defer upstream.Close()
	ctx.Set("base_url", upstream.URL)
	info.OriginModelName = "mj_imagine"
	info.RelayMode = relayconstant.RelayModeMidjourneyImagine

	resp := RelayMidjourneySubmit(ctx, info)
	require.Nil(t, resp)
	require.NoError(t, receiveQiniuMarketCheck(preConsumeCheck))
	requireQiniuMarketRefunded(t)

	var logCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logCount).Error)
	require.Equal(t, int64(0), logCount)
}

func TestRelayMidjourneySubmitStandardModelDoesNotPreConsumeWallet(t *testing.T) {
	ctx, _, info := setupQiniuMjProxyTest(t, "/mj/submit/imagine", `{"prompt":"test prompt"}`)
	info.QiniuManagedToken = false
	preConsumeCheck := make(chan error, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var count int64
		if err := model.DB.Model(&model.WalletFlow{}).Where("biz_no = ?", "wallet-preconsume").Count(&count).Error; err != nil {
			preConsumeCheck <- err
		} else if count != 0 {
			preConsumeCheck <- fmt.Errorf("expected no wallet preconsume before standard MJ upstream call, got %d", count)
		} else {
			preConsumeCheck <- nil
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"description":"submitted","result":"mj-standard-1"}`))
	}))
	defer upstream.Close()
	ctx.Set("base_url", upstream.URL)
	info.OriginModelName = "mj_imagine"
	info.RelayMode = relayconstant.RelayModeMidjourneyImagine

	resp := RelayMidjourneySubmit(ctx, info)
	require.Nil(t, resp)
	require.NoError(t, receiveQiniuMarketCheck(preConsumeCheck))

	var user model.User
	require.NoError(t, model.DB.Select("quota", "used_quota").First(&user, "id = ?", 9101).Error)
	require.Equal(t, 15000, user.Quota)
	require.Equal(t, 5000, user.UsedQuota)
}

func TestRelaySwapFaceRefundsQiniuMarketPreConsumeOnUpstreamError(t *testing.T) {
	ctx, _, info := setupQiniuMjProxyTest(t, "/mj/insight-face/swap", `{"sourceBase64":"a","targetBase64":"b"}`)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	baseURL := upstream.URL
	upstream.Close()
	ctx.Set("base_url", baseURL)
	info.OriginModelName = "swap_face"
	info.RelayMode = relayconstant.RelayModeSwapFace
	info.StartTime = time.Now()

	resp := RelaySwapFace(ctx, info)
	require.NotNil(t, resp)
	requireQiniuMarketRefunded(t)

	var logCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logCount).Error)
	require.Equal(t, int64(0), logCount)
	var taskCount int64
	require.NoError(t, model.DB.Model(&model.Midjourney{}).Count(&taskCount).Error)
	require.Equal(t, int64(0), taskCount)
}

func TestSettleMidjourneyBillingReportsFailure(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		Billing: &failingBillingSession{err: errors.New("settle failed")},
	}

	require.False(t, settleMidjourneyBilling(ctx, info, 5000))
}

func checkQiniuMarketPreConsumed(expectedQuota int) error {
	var user model.User
	if err := model.DB.Select("quota").First(&user, "id = ?", 9101).Error; err != nil {
		return err
	}
	if user.Quota != 20000-expectedQuota {
		return fmt.Errorf("expected user quota %d, got %d", 20000-expectedQuota, user.Quota)
	}

	var token model.Token
	if err := model.DB.Select("remain_quota", "used_quota").First(&token, "id = ?", 9101).Error; err != nil {
		return err
	}
	if token.RemainQuota != 20000-expectedQuota {
		return fmt.Errorf("expected token remain quota %d, got %d", 20000-expectedQuota, token.RemainQuota)
	}
	if token.UsedQuota != expectedQuota {
		return fmt.Errorf("expected token used quota %d, got %d", expectedQuota, token.UsedQuota)
	}

	var account model.WalletAccount
	if err := model.DB.First(&account, "user_id = ?", 9101).Error; err != nil {
		return err
	}
	expectedBalance := float64(20000-expectedQuota) / common.QuotaPerUnit
	if account.BalanceAmount < expectedBalance-0.000001 || account.BalanceAmount > expectedBalance+0.000001 {
		return fmt.Errorf("expected wallet balance %.6f, got %.6f", expectedBalance, account.BalanceAmount)
	}
	var internalFlowCount int64
	if err := model.DB.Model(&model.WalletFlow{}).
		Where("user_id = ? AND biz_no IN ?", 9101, []string{"wallet-preconsume", "wallet-reserve", "wallet-settle", "wallet-refund"}).
		Count(&internalFlowCount).Error; err != nil {
		return err
	}
	if internalFlowCount != 0 {
		return fmt.Errorf("expected no qiniu internal wallet flow, got %d", internalFlowCount)
	}
	return nil
}

func requireQiniuRealtimeWalletFlow(t *testing.T, requestID string, logID int, quota int) {
	t.Helper()

	var app model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.First(&app, "request_id = ?", requestID).Error)
	require.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, app.Status)
	require.Equal(t, logID, app.ConsumeLogId)
	require.Equal(t, quota, app.Quota)
	require.NotZero(t, app.WalletFlowId)

	var flow model.WalletFlow
	require.NoError(t, model.DB.First(&flow, "id = ?", app.WalletFlowId).Error)
	require.Equal(t, "qiniu:realtime:request:"+requestID, flow.BizNo)
	require.Equal(t, model.WalletFlowTypeBalanceConsume, flow.FlowType)
	require.Equal(t, model.WalletFlowDirectionOut, flow.Direction)
	expectedAmount := float64(quota) / common.QuotaPerUnit
	require.InDelta(t, expectedAmount, flow.Amount, 0.000001)
}

func receiveQiniuMarketCheck(ch <-chan error) error {
	select {
	case err := <-ch:
		return err
	case <-time.After(time.Second):
		return errors.New("upstream preconsume check was not reported")
	}
}

func requireQiniuMarketRefunded(t *testing.T) {
	t.Helper()

	require.Eventually(t, func() bool {
		var user model.User
		if err := model.DB.Select("quota", "used_quota").First(&user, "id = ?", 9101).Error; err != nil {
			return false
		}
		var token model.Token
		if err := model.DB.Select("remain_quota", "used_quota").First(&token, "id = ?", 9101).Error; err != nil {
			return false
		}
		var internalFlowCount int64
		if err := model.DB.Model(&model.WalletFlow{}).
			Where("user_id = ? AND biz_no IN ?", 9101, []string{"wallet-preconsume", "wallet-reserve", "wallet-settle", "wallet-refund"}).
			Count(&internalFlowCount).Error; err != nil {
			return false
		}
		return user.Quota == 20000 &&
			user.UsedQuota == 0 &&
			token.RemainQuota == 20000 &&
			token.UsedQuota == 0 &&
			internalFlowCount == 0
	}, 2*time.Second, 20*time.Millisecond)
}

type failingBillingSession struct {
	err error
}

func (s *failingBillingSession) Settle(int) error {
	return s.err
}

func (s *failingBillingSession) Refund(*gin.Context) {}

func (s *failingBillingSession) NeedsRefund() bool {
	return true
}

func (s *failingBillingSession) GetPreConsumedQuota() int {
	return 0
}

func (s *failingBillingSession) Reserve(int) error {
	return nil
}

func TestRelayMidjourneySubmitPersistsQiniuMarketBillingSource(t *testing.T) {
	ctx, _, info := setupQiniuMjProxyTest(t, "/mj/submit/imagine", `{"prompt":"test prompt"}`)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"description":"submitted","result":"mj-qiniu-source"}`))
	}))
	defer upstream.Close()
	ctx.Set("base_url", upstream.URL)
	info.OriginModelName = "mj_imagine"
	info.RelayMode = relayconstant.RelayModeMidjourneyImagine

	resp := RelayMidjourneySubmit(ctx, info)
	require.Nil(t, resp)

	var task model.Midjourney
	require.NoError(t, model.DB.First(&task, "mj_id = ?", "mj-qiniu-source").Error)
	require.Equal(t, service.QiniuMarketRealtimeBillingSource, task.BillingSource)
	require.Equal(t, service.BillingSourceWallet, task.FundingSource)
	require.Equal(t, 0, task.SubscriptionId)
	require.Equal(t, 9101, task.TokenId)
}
