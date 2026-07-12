package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestQiniuWalletFundingSilentlyAdjustsBalanceAndQuota(t *testing.T) {
	truncate(t)

	const userID = 4301
	initialQuota := amountToQuota(10)
	preConsumed := amountToQuota(2)
	debitDelta := amountToQuota(0.75)
	refundDelta := amountToQuota(0.25)
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 10, 0, 0)

	funding := &QiniuWalletFunding{userId: userID}
	require.NoError(t, funding.PreConsume(preConsumed))
	assert.Equal(t, initialQuota-preConsumed, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 8)
	assertNoQiniuInternalWalletFlows(t, userID)

	require.NoError(t, funding.Settle(debitDelta))
	assert.Equal(t, initialQuota-preConsumed-debitDelta, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 7.25)
	assertNoQiniuInternalWalletFlows(t, userID)

	require.NoError(t, funding.Settle(-refundDelta))
	assert.Equal(t, initialQuota-preConsumed-debitDelta+refundDelta, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 7.5)
	assertNoQiniuInternalWalletFlows(t, userID)

	require.NoError(t, funding.Refund())
	assert.Equal(t, initialQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 10)
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestQiniuWalletFundingRejectsInsufficientBalanceAndRollsBack(t *testing.T) {
	truncate(t)

	const userID = 4302
	initialQuota := amountToQuota(1)
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 1, 0, 0)

	funding := &QiniuWalletFunding{userId: userID}
	require.ErrorIs(t, funding.PreConsume(amountToQuota(1.01)), ErrWalletBalanceInsufficient)

	assert.Equal(t, initialQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 1)
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestQiniuRealtimeWalletUserFacingRemarksDoNotExposeSupplierBrand(t *testing.T) {
	remark := qiniuRealtimeWalletFlowRemark(QiniuRealtimeWalletFlowInput{
		RequestId: "req-user-facing",
		BatchId:   "batch-user-facing",
	})
	assert.Contains(t, remark, "市场价实时 token/model 消费")
	assert.NotContains(t, remark, "七牛")
	assert.NotContains(t, remark, "Qiniu")
	assert.NotContains(t, remark, "qiniu")

	consumptionRemark := qiniuRealtimeWalletConsumptionRemark(&relaycommon.RelayInfo{
		TokenId: 123,
	}, "req-consumption", "deepseek/deepseek-v4-flash")
	assert.Contains(t, consumptionRemark, "市场价实时 token/model 消费")
	assert.Contains(t, consumptionRemark, "model=deepseek/deepseek-v4-flash")
	assert.NotContains(t, consumptionRemark, "七牛")
	assert.NotContains(t, consumptionRemark, "Qiniu")
	assert.NotContains(t, consumptionRemark, "qiniu")

	assert.Equal(t, "市场价实时 token/model 消费", qiniuRealtimeWalletRemark(nil))
}

func TestQiniuRealtimeWalletFlowOnlyAppliesWithoutChangingCounters(t *testing.T) {
	truncate(t)

	const userID = 4303
	const tokenID = 4303
	const channelID = 4303
	initialQuota := amountToQuota(20)
	initialTokenRemain := amountToQuota(12)
	initialTokenUsed := amountToQuota(3)
	initialUserUsed := amountToQuota(4)
	initialChannelUsed := amountToQuota(5)
	flowQuota := amountToQuota(1.5)
	seedUser(t, userID, initialQuota)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"used_quota":    initialUserUsed,
		"request_count": 7,
	}).Error)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-flow-token", initialTokenRemain)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", tokenID).Update("used_quota", initialTokenUsed).Error)
	seedChannel(t, channelID)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("used_quota", initialChannelUsed).Error)
	logID := seedQiniuConsumeLog(t, userID, tokenID, channelID, "req-qiniu-flow-1", flowQuota)

	app, err := ApplyQiniuRealtimeWalletFlow(QiniuRealtimeWalletFlowInput{
		UserId:        userID,
		TokenId:       tokenID,
		RequestId:     "req-qiniu-flow-1",
		BatchId:       "batch-qiniu-flow-1",
		ConsumeLogIds: []int{logID},
		Quota:         flowQuota,
		Remark:        "token/model 消费 req-qiniu-flow-1",
	})
	require.NoError(t, err)
	require.NotNil(t, app)
	require.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, app.Status)
	require.NotZero(t, app.WalletFlowId)

	var flow model.WalletFlow
	require.NoError(t, model.DB.First(&flow, "id = ?", app.WalletFlowId).Error)
	assert.Equal(t, model.WalletFlowTypeBalanceConsume, flow.FlowType)
	assert.Equal(t, model.WalletFlowDirectionOut, flow.Direction)
	assert.InDelta(t, quotaToWalletAmount(flowQuota), flow.Amount, 0.000001)
	assert.InDelta(t, 20.0, flow.BalanceAfter, 0.000001)
	assert.Contains(t, flow.BizNo, "qiniu:realtime")

	assert.Equal(t, initialQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 20)
	assert.Equal(t, initialTokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, initialTokenUsed, getTokenUsedQuota(t, tokenID))
	assert.Equal(t, initialUserUsed, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 7)
	assertChannelUsedQuota(t, channelID, initialChannelUsed)

	duplicate, err := ApplyQiniuRealtimeWalletFlow(QiniuRealtimeWalletFlowInput{
		UserId:        userID,
		TokenId:       tokenID,
		RequestId:     "req-qiniu-flow-1",
		BatchId:       "batch-qiniu-flow-1",
		ConsumeLogIds: []int{logID},
		Quota:         flowQuota,
		Remark:        "token/model 消费 req-qiniu-flow-1",
	})
	require.NoError(t, err)
	require.Equal(t, app.Id, duplicate.Id)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, flow.BizNo))
}

func TestQiniuRealtimeWalletApplicationRepairRetriesWithoutChangingBalances(t *testing.T) {
	truncate(t)

	const userID = 4304
	const tokenID = 4304
	const channelID = 4304
	initialQuota := amountToQuota(30)
	flowQuota := amountToQuota(2.25)
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 30, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-repair-token", amountToQuota(20))
	seedChannel(t, channelID)
	firstLogID := seedQiniuConsumeLog(t, userID, tokenID, channelID, "req-qiniu-repair-1", amountToQuota(1))
	secondLogID := seedQiniuConsumeLog(t, userID, tokenID, channelID, "req-qiniu-repair-2", amountToQuota(1.25))

	app := &model.QiniuRealtimeWalletApplication{
		UserId:            userID,
		TokenId:           tokenID,
		RequestId:         "req-qiniu-repair",
		BatchId:           "batch-qiniu-repair",
		ConsumeLogIds:     "1,2",
		IdempotencyKey:    "qiniu:realtime:batch:batch-qiniu-repair",
		Quota:             flowQuota,
		Amount:            quotaToWalletAmount(flowQuota),
		Status:            model.QiniuRealtimeWalletApplicationStatusFailed,
		RetryCount:        1,
		LastError:         "wallet flow write failed",
		BalanceAfter:      30,
		SettlementApplied: true,
		CoveredLogCount:   2,
	}
	app.SetConsumeLogIds([]int{firstLogID, secondLogID})
	require.NoError(t, model.DB.Create(app).Error)

	repaired, err := RepairQiniuRealtimeWalletApplication(app.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, repaired.Status)
	require.NotZero(t, repaired.WalletFlowId)
	require.Equal(t, 2, repaired.RetryCount)

	assert.Equal(t, initialQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 30)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:batch:batch-qiniu-repair"))

	repairedAgain, err := RepairQiniuRealtimeWalletApplication(app.Id)
	require.NoError(t, err)
	require.Equal(t, repaired.WalletFlowId, repairedAgain.WalletFlowId)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:batch:batch-qiniu-repair"))
}

func TestQiniuMarketRealtimeDisablesTrustBypass(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4305
	const tokenID = 4305
	preConsumed := amountToQuota(1)
	initialQuota := common.GetTrustQuota() + amountToQuota(20)
	initialTokenQuota := common.GetTrustQuota() + amountToQuota(20)
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, quotaToWalletAmount(initialQuota), 0, 0)
	seedToken(t, tokenID, userID, "qiniu-trust-token", initialTokenQuota)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("token_quota", initialTokenQuota)
	relayInfo := &relaycommon.RelayInfo{
		UserId:            userID,
		TokenId:           tokenID,
		TokenKey:          "qiniu-trust-token",
		QiniuManagedToken: true,
		PriceData: types.PriceData{
			QiniuMarket: qiniuMarketTextPriceSnapshotForTest(1),
		},
	}

	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market realtime pre-consume to succeed, got %v", apiErr)
	}

	assert.Equal(t, preConsumed, relayInfo.FinalPreConsumedQuota)
	assert.Equal(t, initialQuota-preConsumed, getUserQuota(t, userID))
	assert.Equal(t, initialTokenQuota-preConsumed, getTokenRemainQuota(t, tokenID))
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestQiniuMarketRealtimeUsesQiniuWalletFundingWhenSubscriptionFirst(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4306
	const tokenID = 4306
	const subscriptionID = 4306
	preConsumed := amountToQuota(1)
	initialQuota := amountToQuota(10)
	initialTokenQuota := amountToQuota(10)
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 10, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-subscription-first-token", initialTokenQuota)
	seedSubscription(t, subscriptionID, userID, int64(amountToQuota(100)), 0)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("token_quota", initialTokenQuota)
	ctx.Set(common.RequestIdKey, "req-qiniu-subscription-first")
	relayInfo := &relaycommon.RelayInfo{
		UserId:            userID,
		TokenId:           tokenID,
		TokenKey:          "qiniu-subscription-first-token",
		RequestId:         "req-qiniu-subscription-first",
		QiniuManagedToken: true,
		UserSetting:       dto.UserSetting{BillingPreference: "subscription_first"},
		PriceData: types.PriceData{
			QiniuMarket: qiniuMarketTextPriceSnapshotForTest(1),
		},
		OriginModelName: "deepseek/deepseek-v4-flash",
	}

	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market realtime pre-consume to succeed, got %v", apiErr)
	}

	assert.Equal(t, BillingSourceWallet, relayInfo.BillingSource)
	assert.Zero(t, relayInfo.SubscriptionId)
	assert.Equal(t, int64(0), getSubscriptionUsed(t, subscriptionID))
	assert.Equal(t, initialQuota-preConsumed, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 9)
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestQiniuMarketRealtimeSuccessCreatesFinalWalletFlowMatchingUsageLog(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4307
	const tokenID = 4307
	const channelID = 4307
	initialQuota := amountToQuota(20)
	preConsumed := 5000
	actualQuota := 3600
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-success-token", initialQuota)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-success-token", "req-qiniu-success")
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}
	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
	}, []string{"请求成功"})

	log := getLastLog(t)
	require.NotNil(t, log)
	require.Equal(t, actualQuota, log.Quota)
	assert.Equal(t, initialQuota-actualQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 20-quotaToWalletAmount(actualQuota))
	assert.Equal(t, initialQuota-actualQuota, getTokenRemainQuota(t, tokenID))
	assertNoQiniuInternalWalletFlows(t, userID)
	assertQiniuRealtimeWalletApplication(t, userID, tokenID, log.Id, "req-qiniu-success", actualQuota)
}

func TestQiniuMarketRealtimeDuplicateSuccessDoesNotDuplicateUsageFacts(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4334
	const tokenID = 4334
	const channelID = 4334
	initialQuota := amountToQuota(20)
	preConsumed := 5000
	actualQuota := 3600
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-duplicate-success-token", initialQuota)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-duplicate-success-token", "req-qiniu-duplicate-success")
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}
	for i := 0; i < 2; i++ {
		PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{
			PromptTokens:     1000,
			CompletionTokens: 200,
		}, []string{"请求成功"})
	}

	assert.Equal(t, initialQuota-actualQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 20-quotaToWalletAmount(actualQuota))
	assert.Equal(t, initialQuota-actualQuota, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)
	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-duplicate-success"))
}

func TestQiniuMarketRealtimeEstimateLowerThanActualCreatesOnlyFinalWalletFlow(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4308
	const tokenID = 4308
	const channelID = 4308
	initialQuota := amountToQuota(20)
	preConsumed := 3000
	actualQuota := 3600
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-low-estimate-token", initialQuota)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-low-estimate-token", "req-qiniu-low-estimate")
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}
	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
	}, []string{"请求成功"})

	log := getLastLog(t)
	require.NotNil(t, log)
	require.Equal(t, actualQuota, log.Quota)
	assert.Equal(t, initialQuota-actualQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 20-quotaToWalletAmount(actualQuota))
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-low-estimate"))
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestQiniuMarketRealtimePostSuccessDeltaCanCreateDebt(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4313
	const tokenID = 4313
	const channelID = 4313
	preConsumed := 3000
	actualQuota := 3600
	initialQuota := preConsumed
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, quotaToWalletAmount(preConsumed), 0, 0)
	seedToken(t, tokenID, userID, "qiniu-debt-token", preConsumed)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-debt-token", "req-qiniu-debt")
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}
	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
	}, []string{"请求成功"})

	log := getLastLog(t)
	require.NotNil(t, log)
	require.Equal(t, actualQuota, log.Quota)
	assert.Equal(t, initialQuota-actualQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, -quotaToWalletAmount(actualQuota-initialQuota))
	assert.Equal(t, initialQuota-actualQuota, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, getTokenUsedQuota(t, tokenID))
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertQiniuRealtimeWalletApplication(t, userID, tokenID, log.Id, "req-qiniu-debt", actualQuota)
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestQiniuMarketRealtimeSettlementFailureRecordsFailedApplicationWithoutFinalFlow(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4314
	const tokenID = 4314
	const channelID = 4314
	initialQuota := amountToQuota(20)
	preConsumed := 3000
	actualQuota := 3600
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-settle-fail-token", initialQuota)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-settle-fail-token", "req-qiniu-settle-fail")
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}

	callbackName := "test_fail_qiniu_realtime_settlement_wallet_update"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "wallet_accounts" {
			tx.AddError(errors.New("forced qiniu realtime settlement failure"))
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Update().Remove(callbackName)
	})

	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
	}, []string{"请求成功"})

	assert.Equal(t, initialQuota-preConsumed, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 20-quotaToWalletAmount(preConsumed))
	assert.Equal(t, 0, getUserUsedQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))
	assert.Equal(t, int64(0), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-settle-fail"))

	var app model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.Where("request_id = ?", "req-qiniu-settle-fail").First(&app).Error)
	assert.Equal(t, model.QiniuRealtimeWalletApplicationStatusFailed, app.Status)
	assert.Equal(t, actualQuota, app.Quota)
	assert.Equal(t, 0, app.WalletFlowId)
	assert.Contains(t, app.LastError, "forced qiniu realtime settlement failure")
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestRepairQiniuRealtimeWalletApplicationRestoresUsageFactsAfterSettlementFailure(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4318
	const tokenID = 4318
	const channelID = 4318
	initialQuota := amountToQuota(20)
	preConsumed := 3000
	actualQuota := 3600
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-settle-repair-token", initialQuota)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-settle-repair-token", "req-qiniu-settle-repair")
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}

	callbackName := "test_fail_qiniu_realtime_repair_settlement_wallet_update"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "wallet_accounts" {
			tx.AddError(errors.New("forced qiniu realtime repair settlement failure"))
		}
	}))

	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
	}, []string{"请求成功"})
	require.NoError(t, model.DB.Callback().Update().Remove(callbackName))

	var app model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.Where("request_id = ?", "req-qiniu-settle-repair").First(&app).Error)
	require.Equal(t, model.QiniuRealtimeWalletApplicationStatusFailed, app.Status)
	require.False(t, app.SettlementApplied)
	assert.Equal(t, int64(0), countLogs(t))
	assert.Equal(t, 0, getUserUsedQuota(t, userID))

	repaired, err := RepairQiniuRealtimeWalletApplication(app.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, repaired.Status)
	require.True(t, repaired.SettlementApplied)
	require.True(t, repaired.UsageApplied)
	require.NotZero(t, repaired.ConsumeLogId)
	require.NotZero(t, repaired.WalletFlowId)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, actualQuota, log.Quota)
	assert.Equal(t, "req-qiniu-settle-repair", log.RequestId)
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-settle-repair"))

	repairedAgain, err := RepairQiniuRealtimeWalletApplication(app.Id)
	require.NoError(t, err)
	assert.Equal(t, repaired.WalletFlowId, repairedAgain.WalletFlowId)
	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-settle-repair"))
}

func TestRepairQiniuRealtimeWalletApplicationRollsBackReplayLogWhenUsageRestoreFails(t *testing.T) {
	truncate(t)

	const userID = 4320
	const tokenID = 4320
	const channelID = 4320
	actualQuota := amountToQuota(0.36)
	seedUser(t, userID, amountToQuota(10))
	seedWalletAccount(t, userID, 10, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-repair-usage-rollback-token", amountToQuota(10))
	seedChannel(t, channelID)

	app := &model.QiniuRealtimeWalletApplication{
		UserId:            userID,
		TokenId:           tokenID,
		RequestId:         "req-qiniu-repair-usage-rollback",
		IdempotencyKey:    "qiniu:realtime:request:req-qiniu-repair-usage-rollback",
		Quota:             actualQuota,
		PreConsumedQuota:  actualQuota,
		Amount:            quotaToWalletAmount(actualQuota),
		SettlementApplied: true,
		UsageApplied:      false,
		Status:            model.QiniuRealtimeWalletApplicationStatusFailed,
		CreatedTime:       common.GetTimestamp(),
	}
	require.NoError(t, app.SetConsumeLogParams(&model.RecordConsumeLogParams{
		ChannelId:        channelID,
		PromptTokens:     10,
		CompletionTokens: 20,
		ModelName:        "qiniu-test-model",
		TokenName:        "qiniu-test-token",
		Quota:            actualQuota,
		Content:          "七牛市场价实时扣费",
		TokenId:          tokenID,
		Group:            "default",
		Other: map[string]interface{}{
			"billing_source": QiniuMarketRealtimeBillingSource,
			"price_source":   QiniuMarketPriceSource,
		},
	}))
	require.NoError(t, model.DB.Create(app).Error)

	callbackName := "test_fail_qiniu_realtime_usage_restore_user_update"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "users" {
			tx.AddError(errors.New("forced qiniu realtime usage restore failure"))
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Update().Remove(callbackName)
	})

	_, err := RepairQiniuRealtimeWalletApplication(app.Id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced qiniu realtime usage restore failure")
	assert.Equal(t, int64(0), countLogs(t))
	assert.Equal(t, 0, getUserUsedQuota(t, userID))

	var reloaded model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.First(&reloaded, "id = ?", app.Id).Error)
	assert.False(t, reloaded.UsageApplied)
	assert.Equal(t, 0, reloaded.ConsumeLogId)
}

func TestRepairQiniuRealtimeWalletApplicationWithSeparateLogDBDoesNotCreateLogWhenStatsFail(t *testing.T) {
	truncate(t)

	originalLogDB := model.LOG_DB
	separateLogDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, separateLogDB.AutoMigrate(&model.Log{}))
	model.LOG_DB = separateLogDB
	oldDataExportEnabled := common.DataExportEnabled
	common.DataExportEnabled = false
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		common.DataExportEnabled = oldDataExportEnabled
	})

	const userID = 4321
	const tokenID = 4321
	const channelID = 4321
	actualQuota := amountToQuota(0.36)
	seedUser(t, userID, amountToQuota(10))
	seedWalletAccount(t, userID, 10, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-repair-separate-log-db-token", amountToQuota(10))
	seedChannel(t, channelID)

	app := &model.QiniuRealtimeWalletApplication{
		UserId:            userID,
		TokenId:           tokenID,
		RequestId:         "req-qiniu-repair-separate-log-db",
		IdempotencyKey:    "qiniu:realtime:request:req-qiniu-repair-separate-log-db",
		Quota:             actualQuota,
		PreConsumedQuota:  actualQuota,
		Amount:            quotaToWalletAmount(actualQuota),
		SettlementApplied: true,
		UsageApplied:      false,
		UsageStatsApplied: false,
		UsageLogApplied:   false,
		Status:            model.QiniuRealtimeWalletApplicationStatusFailed,
		CreatedTime:       common.GetTimestamp(),
	}
	require.NoError(t, app.SetConsumeLogParams(&model.RecordConsumeLogParams{
		ChannelId:        channelID,
		PromptTokens:     10,
		CompletionTokens: 20,
		ModelName:        "qiniu-test-model",
		TokenName:        "qiniu-test-token",
		Quota:            actualQuota,
		Content:          "七牛市场价实时扣费",
		TokenId:          tokenID,
		Group:            "default",
		Other: map[string]interface{}{
			"billing_source": QiniuMarketRealtimeBillingSource,
			"price_source":   QiniuMarketPriceSource,
		},
	}))
	require.NoError(t, model.DB.Create(app).Error)

	callbackName := "test_fail_qiniu_realtime_separate_log_db_stats_update"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "users" {
			tx.AddError(errors.New("forced qiniu realtime separate log db stats failure"))
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Update().Remove(callbackName)
	})

	_, err = RepairQiniuRealtimeWalletApplication(app.Id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced qiniu realtime separate log db stats failure")
	var logCount int64
	require.NoError(t, separateLogDB.Model(&model.Log{}).Count(&logCount).Error)
	assert.Equal(t, int64(0), logCount)
	assert.Equal(t, 0, getUserUsedQuota(t, userID))

	var reloaded model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.First(&reloaded, "id = ?", app.Id).Error)
	assert.False(t, reloaded.UsageStatsApplied)
	assert.False(t, reloaded.UsageLogApplied)
	assert.False(t, reloaded.UsageApplied)
	assert.Equal(t, 0, reloaded.ConsumeLogId)
}

func TestRepairQiniuRealtimeWalletApplicationWithSeparateLogDBIsIdempotentAcrossStages(t *testing.T) {
	truncate(t)

	originalLogDB := model.LOG_DB
	separateLogDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, separateLogDB.AutoMigrate(&model.Log{}))
	model.LOG_DB = separateLogDB
	oldDataExportEnabled := common.DataExportEnabled
	common.DataExportEnabled = false
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		common.DataExportEnabled = oldDataExportEnabled
	})

	const userID = 4322
	const tokenID = 4322
	const channelID = 4322
	actualQuota := amountToQuota(0.42)
	seedUser(t, userID, amountToQuota(10))
	seedWalletAccount(t, userID, 10, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-repair-separate-log-db-idempotent-token", amountToQuota(10))
	seedChannel(t, channelID)

	app := &model.QiniuRealtimeWalletApplication{
		UserId:            userID,
		TokenId:           tokenID,
		RequestId:         "req-qiniu-repair-separate-log-db-idempotent",
		IdempotencyKey:    "qiniu:realtime:request:req-qiniu-repair-separate-log-db-idempotent",
		Quota:             actualQuota,
		PreConsumedQuota:  actualQuota,
		Amount:            quotaToWalletAmount(actualQuota),
		SettlementApplied: true,
		UsageApplied:      false,
		UsageStatsApplied: false,
		UsageLogApplied:   false,
		Status:            model.QiniuRealtimeWalletApplicationStatusFailed,
		CreatedTime:       common.GetTimestamp(),
	}
	require.NoError(t, app.SetConsumeLogParams(&model.RecordConsumeLogParams{
		ChannelId:        channelID,
		PromptTokens:     11,
		CompletionTokens: 22,
		ModelName:        "qiniu-test-model",
		TokenName:        "qiniu-test-token",
		Quota:            actualQuota,
		Content:          "七牛市场价实时扣费",
		TokenId:          tokenID,
		Group:            "default",
		Other: map[string]interface{}{
			"billing_source": QiniuMarketRealtimeBillingSource,
			"price_source":   QiniuMarketPriceSource,
		},
	}))
	require.NoError(t, model.DB.Create(app).Error)

	repaired, err := RepairQiniuRealtimeWalletApplication(app.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, repaired.Status)
	require.True(t, repaired.UsageStatsApplied)
	require.True(t, repaired.UsageLogApplied)
	require.True(t, repaired.UsageApplied)
	require.NotZero(t, repaired.ConsumeLogId)
	require.NotZero(t, repaired.WalletFlowId)

	var logCount int64
	require.NoError(t, separateLogDB.Model(&model.Log{}).Count(&logCount).Error)
	assert.Equal(t, int64(1), logCount)
	var replayLog model.Log
	require.NoError(t, separateLogDB.First(&replayLog, "id = ?", repaired.ConsumeLogId).Error)
	assert.Equal(t, actualQuota, replayLog.Quota)
	assert.Equal(t, "req-qiniu-repair-separate-log-db-idempotent", replayLog.RequestId)
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-repair-separate-log-db-idempotent"))

	repairedAgain, err := RepairQiniuRealtimeWalletApplication(app.Id)
	require.NoError(t, err)
	assert.Equal(t, repaired.WalletFlowId, repairedAgain.WalletFlowId)
	assert.Equal(t, repaired.ConsumeLogId, repairedAgain.ConsumeLogId)
	require.NoError(t, separateLogDB.Model(&model.Log{}).Count(&logCount).Error)
	assert.Equal(t, int64(1), logCount)
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-repair-separate-log-db-idempotent"))
}

func TestScanFailedQiniuRealtimeWalletApplicationsRepairsDueApplications(t *testing.T) {
	truncate(t)

	const userID = 4319
	const tokenID = 4319
	const channelID = 4319
	flowQuota := amountToQuota(0.35)
	seedUser(t, userID, amountToQuota(10))
	seedWalletAccount(t, userID, 10, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-repair-scan-token", amountToQuota(10))
	seedChannel(t, channelID)

	app := &model.QiniuRealtimeWalletApplication{
		UserId:            userID,
		TokenId:           tokenID,
		RequestId:         "req-qiniu-repair-scan",
		IdempotencyKey:    "qiniu:realtime:request:req-qiniu-repair-scan",
		Quota:             flowQuota,
		Amount:            quotaToWalletAmount(flowQuota),
		SettlementApplied: true,
		UsageApplied:      true,
		Status:            model.QiniuRealtimeWalletApplicationStatusFailed,
		LastError:         "wallet flow write failed",
	}
	require.NoError(t, model.DB.Create(app).Error)

	result, err := ScanFailedQiniuRealtimeWalletApplications(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ProcessedCount)
	assert.Equal(t, 1, result.AppliedCount)
	assert.Equal(t, 0, result.FailedCount)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-repair-scan"))
}

func TestScanFailedQiniuRealtimeWalletApplicationsRepairsPendingApplications(t *testing.T) {
	truncate(t)

	const userID = 4335
	const tokenID = 4335
	const channelID = 4335
	actualQuota := amountToQuota(0.44)
	seedUser(t, userID, amountToQuota(10))
	seedWalletAccount(t, userID, 10, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-repair-pending-scan-token", amountToQuota(10))
	seedChannel(t, channelID)

	app := &model.QiniuRealtimeWalletApplication{
		UserId:            userID,
		TokenId:           tokenID,
		RequestId:         "req-qiniu-repair-pending-scan",
		IdempotencyKey:    "qiniu:realtime:request:req-qiniu-repair-pending-scan",
		Quota:             actualQuota,
		PreConsumedQuota:  actualQuota,
		Amount:            quotaToWalletAmount(actualQuota),
		SettlementApplied: true,
		UsageApplied:      false,
		UsageStatsApplied: false,
		UsageLogApplied:   false,
		Status:            model.QiniuRealtimeWalletApplicationStatusPending,
		LastError:         "waiting for qiniu realtime wallet repair",
		CreatedTime:       common.GetTimestamp(),
	}
	require.NoError(t, app.SetConsumeLogParams(&model.RecordConsumeLogParams{
		ChannelId:        channelID,
		PromptTokens:     12,
		CompletionTokens: 32,
		ModelName:        "qiniu-test-model",
		TokenName:        "qiniu-test-token",
		Quota:            actualQuota,
		Content:          "七牛市场价实时扣费",
		TokenId:          tokenID,
		Group:            "default",
		Other: map[string]interface{}{
			"billing_source": QiniuMarketRealtimeBillingSource,
			"price_source":   QiniuMarketPriceSource,
		},
	}))
	require.NoError(t, model.DB.Create(app).Error)

	result, err := ScanFailedQiniuRealtimeWalletApplications(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ProcessedCount)
	assert.Equal(t, 1, result.AppliedCount)
	assert.Equal(t, 0, result.FailedCount)

	var repaired model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.Where("request_id = ?", "req-qiniu-repair-pending-scan").First(&repaired).Error)
	assert.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, repaired.Status)
	assert.True(t, repaired.UsageStatsApplied)
	assert.True(t, repaired.UsageLogApplied)
	assert.True(t, repaired.UsageApplied)
	require.NotZero(t, repaired.ConsumeLogId)
	require.NotZero(t, repaired.WalletFlowId)
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)
	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-repair-pending-scan"))

	secondResult, err := ScanFailedQiniuRealtimeWalletApplications(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 0, secondResult.ProcessedCount)
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)
	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-repair-pending-scan"))
}

func TestRepairQiniuRealtimeWalletApplicationRetriesUnappliedSettlementBeforeFinalFlow(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4317
	const tokenID = 4317
	preConsumed := 3000
	actualQuota := 3600
	initialQuota := preConsumed
	seedUser(t, userID, initialQuota-preConsumed)
	seedWalletAccount(t, userID, 0, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-repair-settlement-token", initialQuota-preConsumed)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", tokenID).Update("used_quota", preConsumed).Error)

	app := &model.QiniuRealtimeWalletApplication{
		UserId:            userID,
		TokenId:           tokenID,
		RequestId:         "req-qiniu-repair-settlement",
		IdempotencyKey:    "qiniu:realtime:request:req-qiniu-repair-settlement",
		Quota:             actualQuota,
		PreConsumedQuota:  preConsumed,
		Amount:            quotaToWalletAmount(actualQuota),
		Status:            model.QiniuRealtimeWalletApplicationStatusFailed,
		LastError:         "forced qiniu realtime settlement failure",
		SettlementApplied: false,
	}
	require.NoError(t, model.DB.Create(app).Error)

	repaired, err := RepairQiniuRealtimeWalletApplication(app.Id)
	require.NoError(t, err)
	require.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, repaired.Status)
	require.True(t, repaired.SettlementApplied)
	require.NotZero(t, repaired.WalletFlowId)
	assert.Equal(t, initialQuota-actualQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, -quotaToWalletAmount(actualQuota-initialQuota))
	assert.Equal(t, initialQuota-actualQuota, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, getTokenUsedQuota(t, tokenID))
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-repair-settlement"))

	repairedAgain, err := RepairQiniuRealtimeWalletApplication(app.Id)
	require.NoError(t, err)
	require.Equal(t, repaired.WalletFlowId, repairedAgain.WalletFlowId)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-repair-settlement"))
}

func TestQiniuMarketRealtimeTaskPostSuccessDeltaCanCreateDebt(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4315
	const tokenID = 4315
	const channelID = 4315
	preConsumed := 3000
	actualQuota := 3600
	initialQuota := preConsumed
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, quotaToWalletAmount(preConsumed), 0, 0)
	seedToken(t, tokenID, userID, "qiniu-task-debt-token", preConsumed)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-task-debt-token", "req-qiniu-task-debt")
	relayInfo.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	relayInfo.Action = "image"
	relayInfo.PriceData.Quota = actualQuota
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market task pre-consume to succeed, got %v", apiErr)
	}
	require.NoError(t, SettleBilling(ctx, relayInfo, actualQuota))

	LogTaskConsumption(ctx, relayInfo)

	log := getLastLog(t)
	require.NotNil(t, log)
	require.Equal(t, actualQuota, log.Quota)
	assert.Equal(t, initialQuota-actualQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, -quotaToWalletAmount(actualQuota-initialQuota))
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertQiniuRealtimeWalletApplication(t, userID, tokenID, log.Id, "req-qiniu-task-debt", actualQuota)
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestQiniuMarketRealtimeTaskStatsFailureKeepsApplicationRepairable(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4323
	const tokenID = 4323
	const channelID = 4323
	preConsumed := 3000
	actualQuota := 3600
	initialQuota := amountToQuota(20)
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-task-stats-fail-token", initialQuota)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-task-stats-fail-token", "req-qiniu-task-stats-fail")
	relayInfo.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	relayInfo.Action = "image"
	relayInfo.PriceData.Quota = actualQuota
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market task pre-consume to succeed, got %v", apiErr)
	}
	require.NoError(t, SettleBilling(ctx, relayInfo, actualQuota))

	callbackName := "test_fail_qiniu_task_usage_stats_user_update"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "users" {
			tx.AddError(errors.New("forced qiniu task usage stats failure"))
		}
	}))

	LogTaskConsumption(ctx, relayInfo)
	require.NoError(t, model.DB.Callback().Update().Remove(callbackName))

	var app model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.Where("request_id = ?", "req-qiniu-task-stats-fail").First(&app).Error)
	assert.Equal(t, model.QiniuRealtimeWalletApplicationStatusFailed, app.Status)
	assert.False(t, app.UsageStatsApplied)
	assert.False(t, app.UsageLogApplied)
	assert.False(t, app.UsageApplied)
	assert.Equal(t, 0, app.WalletFlowId)
	assert.Equal(t, int64(0), countLogs(t))
	assert.Equal(t, 0, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 0)
	assertChannelUsedQuota(t, channelID, 0)

	result, err := ScanFailedQiniuRealtimeWalletApplications(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ProcessedCount)
	assert.Equal(t, 1, result.AppliedCount)
	assert.Equal(t, 0, result.FailedCount)

	var repaired model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.Where("request_id = ?", "req-qiniu-task-stats-fail").First(&repaired).Error)
	assert.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, repaired.Status)
	assert.True(t, repaired.UsageStatsApplied)
	assert.True(t, repaired.UsageLogApplied)
	assert.True(t, repaired.UsageApplied)
	require.NotZero(t, repaired.ConsumeLogId)
	require.NotZero(t, repaired.WalletFlowId)
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)
	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-task-stats-fail"))

	secondResult, err := ScanFailedQiniuRealtimeWalletApplications(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 0, secondResult.ProcessedCount)
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)
	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-task-stats-fail"))
}

func TestQiniuMarketRealtimeTaskDoesNotCreateFinalFlowWhenBillingUnsettled(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4316
	const tokenID = 4316
	const channelID = 4316
	initialQuota := amountToQuota(20)
	preConsumed := 3000
	actualQuota := 3600
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-task-unsettled-token", initialQuota)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-task-unsettled-token", "req-qiniu-task-unsettled")
	relayInfo.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	relayInfo.Action = "image"
	relayInfo.PriceData.Quota = actualQuota
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market task pre-consume to succeed, got %v", apiErr)
	}

	LogTaskConsumption(ctx, relayInfo)

	assert.Equal(t, initialQuota-preConsumed, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 20-quotaToWalletAmount(preConsumed))
	assert.Equal(t, 0, getUserUsedQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))
	assert.Equal(t, int64(0), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-task-unsettled"))

	var app model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.Where("request_id = ?", "req-qiniu-task-unsettled").First(&app).Error)
	assert.Equal(t, model.QiniuRealtimeWalletApplicationStatusFailed, app.Status)
	assert.Equal(t, actualQuota, app.Quota)
	assert.Equal(t, 0, app.WalletFlowId)
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestQiniuMarketRealtimeCreatesFinalWalletFlowWhenTokenDeltaFailsAfterFunding(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4310
	const tokenID = 4310
	const channelID = 4310
	initialQuota := amountToQuota(20)
	preConsumed := 3000
	actualQuota := 3600
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-token-delta-fail-token", preConsumed)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-token-delta-fail-token", "req-qiniu-token-delta-fail")
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}
	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
	}, []string{"请求成功"})

	log := getLastLog(t)
	require.NotNil(t, log)
	require.Equal(t, actualQuota, log.Quota)
	assert.Equal(t, initialQuota-actualQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 20-quotaToWalletAmount(actualQuota))
	assertQiniuRealtimeWalletApplication(t, userID, tokenID, log.Id, "req-qiniu-token-delta-fail", actualQuota)
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestQiniuMarketRealtimeCreatesRequestWalletFlowWhenConsumeLogDisabled(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	oldLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() {
		common.LogConsumeEnabled = oldLogConsumeEnabled
	})

	const userID = 4311
	const tokenID = 4311
	const channelID = 4311
	initialQuota := amountToQuota(20)
	preConsumed := 5000
	actualQuota := 3600
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-log-disabled-token", initialQuota)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-log-disabled-token", "req-qiniu-log-disabled")
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}
	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
	}, []string{"请求成功"})

	var logCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logCount).Error)
	assert.Equal(t, int64(0), logCount)
	assert.Equal(t, initialQuota-actualQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 20-quotaToWalletAmount(actualQuota))

	var app model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.Where("request_id = ?", "req-qiniu-log-disabled").First(&app).Error)
	assert.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, app.Status)
	assert.Equal(t, 0, app.ConsumeLogId)
	assert.Equal(t, actualQuota, app.Quota)
	require.NotZero(t, app.WalletFlowId)
	assertNoQiniuInternalWalletFlows(t, userID)
}

func TestQiniuMarketRealtimeCreatesRequestWalletFlowWhenConsumeLogWriteFails(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	callbackName := "test_fail_qiniu_realtime_consume_log_create"
	require.NoError(t, model.LOG_DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "logs" {
			tx.AddError(errors.New("forced consume log failure"))
		}
	}))
	callbackRemoved := false
	t.Cleanup(func() {
		if !callbackRemoved {
			_ = model.LOG_DB.Callback().Create().Remove(callbackName)
		}
	})

	const userID = 4312
	const tokenID = 4312
	const channelID = 4312
	initialQuota := amountToQuota(20)
	preConsumed := 5000
	actualQuota := 3600
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-log-fail-token", initialQuota)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-log-fail-token", "req-qiniu-log-fail")
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}
	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
	}, []string{"请求成功"})

	var logCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logCount).Error)
	assert.Equal(t, int64(0), logCount)
	assert.Equal(t, initialQuota-actualQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 20-quotaToWalletAmount(actualQuota))
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)

	var app model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.Where("request_id = ?", "req-qiniu-log-fail").First(&app).Error)
	assert.Equal(t, model.QiniuRealtimeWalletApplicationStatusFailed, app.Status)
	assert.Equal(t, 0, app.ConsumeLogId)
	assert.Equal(t, actualQuota, app.Quota)
	assert.True(t, app.SettlementApplied)
	assert.True(t, app.UsageStatsApplied)
	assert.False(t, app.UsageLogApplied)
	assert.False(t, app.UsageApplied)
	require.NotZero(t, app.WalletFlowId)
	assertNoQiniuInternalWalletFlows(t, userID)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-log-fail"))

	require.NoError(t, model.LOG_DB.Callback().Create().Remove(callbackName))
	callbackRemoved = true

	result, err := ScanFailedQiniuRealtimeWalletApplications(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ProcessedCount)
	assert.Equal(t, 1, result.AppliedCount)
	assert.Equal(t, 0, result.FailedCount)

	var repaired model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.Where("request_id = ?", "req-qiniu-log-fail").First(&repaired).Error)
	assert.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, repaired.Status)
	assert.True(t, repaired.UsageStatsApplied)
	assert.True(t, repaired.UsageLogApplied)
	assert.True(t, repaired.UsageApplied)
	require.NotZero(t, repaired.ConsumeLogId)
	assert.Equal(t, app.WalletFlowId, repaired.WalletFlowId)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, actualQuota, log.Quota)
	assert.Equal(t, "req-qiniu-log-fail", log.RequestId)
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-log-fail"))

	secondResult, err := ScanFailedQiniuRealtimeWalletApplications(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 0, secondResult.ProcessedCount)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logCount).Error)
	assert.Equal(t, int64(1), logCount)
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assertUserRequestCount(t, userID, 1)
	assertChannelUsedQuota(t, channelID, actualQuota)
	assert.Equal(t, int64(1), countWalletFlows(t, userID, model.WalletFlowTypeBalanceConsume, "qiniu:realtime:request:req-qiniu-log-fail"))
}

func TestQiniuMarketRealtimeFailureRefundsSilentlyWithoutWalletFlow(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4309
	const tokenID = 4309
	const channelID = 4309
	initialQuota := amountToQuota(20)
	preConsumed := amountToQuota(1)
	seedUser(t, userID, initialQuota)
	seedWalletAccount(t, userID, 20, 0, 0)
	seedToken(t, tokenID, userID, "qiniu-failure-token", initialQuota)
	seedChannel(t, channelID)

	ctx, relayInfo := qiniuRealtimeRelayContext(t, userID, tokenID, channelID, "qiniu-failure-token", "req-qiniu-failure")
	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}
	require.NoError(t, relayInfo.Billing.Settle(0))

	assert.Equal(t, initialQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, 20)
	assert.Equal(t, initialQuota, getTokenRemainQuota(t, tokenID))
	assertNoQiniuInternalWalletFlows(t, userID)
	var flowCount int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).Where("user_id = ?", userID).Count(&flowCount).Error)
	assert.Equal(t, int64(0), flowCount)
}

func seedQiniuConsumeLog(t *testing.T, userID int, tokenID int, channelID int, requestID string, quota int) int {
	t.Helper()
	other := map[string]interface{}{
		"billing_source": QiniuMarketRealtimeBillingSource,
		"price_source":   QiniuMarketPriceSource,
	}
	log := &model.Log{
		UserId:    userID,
		CreatedAt: common.GetTimestamp(),
		Type:      model.LogTypeConsume,
		Content:   "市场价实时扣费",
		ModelName: "qiniu-test-model",
		TokenName: "qiniu-test-token",
		Quota:     quota,
		TokenId:   tokenID,
		ChannelId: channelID,
		Group:     "default",
		RequestId: requestID,
		Other:     common.MapToJsonStr(other),
	}
	require.NoError(t, model.LOG_DB.Create(log).Error)
	return log.Id
}

func qiniuRealtimeRelayContext(t *testing.T, userID int, tokenID int, channelID int, tokenKey string, requestID string) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("token_quota", amountToQuota(20))
	ctx.Set("token_name", "qiniu-test-token")
	ctx.Set(common.RequestIdKey, requestID)
	return ctx, &relaycommon.RelayInfo{
		UserId:            userID,
		TokenId:           tokenID,
		TokenKey:          tokenKey,
		TokenUnlimited:    false,
		QiniuManagedToken: true,
		OriginModelName:   "deepseek/deepseek-v4-flash",
		UsingGroup:        "default",
		StartTime:         time.Now(),
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: channelID},
		RequestId:         requestID,
		PriceData: types.PriceData{
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
			QiniuMarket:    qiniuMarketTextPriceSnapshotForTest(1),
		},
	}
}

func assertQiniuRealtimeWalletApplication(t *testing.T, userID int, tokenID int, logID int, requestID string, quota int) {
	t.Helper()
	var app model.QiniuRealtimeWalletApplication
	require.NoError(t, model.DB.Where("request_id = ?", requestID).First(&app).Error)
	assert.Equal(t, model.QiniuRealtimeWalletApplicationStatusApplied, app.Status)
	assert.Equal(t, userID, app.UserId)
	assert.Equal(t, tokenID, app.TokenId)
	assert.Equal(t, logID, app.ConsumeLogId)
	assert.Equal(t, quota, app.Quota)
	assert.InDelta(t, quotaToWalletAmount(quota), app.Amount, 0.000001)
	require.NotZero(t, app.WalletFlowId)

	var flow model.WalletFlow
	require.NoError(t, model.DB.First(&flow, "id = ?", app.WalletFlowId).Error)
	assert.Equal(t, "qiniu:realtime:request:"+requestID, flow.BizNo)
	assert.Equal(t, model.WalletFlowTypeBalanceConsume, flow.FlowType)
	assert.Equal(t, model.WalletFlowDirectionOut, flow.Direction)
	assert.InDelta(t, quotaToWalletAmount(quota), flow.Amount, 0.000001)
}

func assertWalletBalance(t *testing.T, userID int, expected float64) {
	t.Helper()
	account, err := model.GetWalletAccountByUserId(userID)
	require.NoError(t, err)
	assert.InDelta(t, expected, account.BalanceAmount, 0.000001)
}

func assertNoQiniuInternalWalletFlows(t *testing.T, userID int) {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).
		Where("user_id = ? AND biz_no IN ?", userID, []string{"wallet-preconsume", "wallet-reserve", "wallet-settle", "wallet-refund", "wallet-reserve-rollback"}).
		Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func assertUserRequestCount(t *testing.T, userID int, expected int) {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("request_count").Where("id = ?", userID).First(&user).Error)
	assert.Equal(t, expected, user.RequestCount)
}

func assertChannelUsedQuota(t *testing.T, channelID int, expected int) {
	t.Helper()
	var channel model.Channel
	require.NoError(t, model.DB.Select("used_quota").Where("id = ?", channelID).First(&channel).Error)
	assert.Equal(t, int64(expected), channel.UsedQuota)
}
