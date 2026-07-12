package service

import (
	"context"
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
)

func TestQiniuOfficialLedgerSkipsLocalRealtimeBilling(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4201
	const tokenID = 4201
	const initialQuota = 10000
	const tokenRemain = 5000
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "qiniu-managed-token", tokenRemain)

	relayInfo := &relaycommon.RelayInfo{
		UserId:            userID,
		TokenId:           tokenID,
		TokenKey:          "qiniu-managed-token",
		TokenUnlimited:    false,
		QiniuManagedToken: true,
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	if apiErr := PreConsumeBilling(ctx, 1000, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu pre-consume to be skipped, got %v", apiErr)
	}
	requireNoError(t, PreConsumeTokenQuota(relayInfo, 1000))
	requireNoError(t, PostConsumeQuota(relayInfo, 1000, 0, true))
	requireNoError(t, SettleBilling(ctx, relayInfo, 1000))

	if got := getUserQuota(t, userID); got != initialQuota {
		t.Fatalf("expected user quota unchanged, got %d", got)
	}
	if got := getTokenRemainQuota(t, tokenID); got != tokenRemain {
		t.Fatalf("expected token quota unchanged, got %d", got)
	}
	var flowCount int64
	requireNoError(t, model.DB.Model(&model.WalletFlow{}).Count(&flowCount).Error)
	if flowCount != 0 {
		t.Fatalf("expected no wallet flow for qiniu realtime request, got %d", flowCount)
	}
	if relayInfo.BillingSource != qiniuOfficialLedgerBillingSource || relayInfo.FinalPreConsumedQuota != 0 {
		t.Fatalf("expected qiniu billing marker and zero pre-consume, got source=%s pre=%d", relayInfo.BillingSource, relayInfo.FinalPreConsumedQuota)
	}
}

func TestQiniuMarketRealtimeBypassesOfficialLedgerSkip(t *testing.T) {
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	relayInfo := &relaycommon.RelayInfo{
		QiniuManagedToken: true,
		PriceData: types.PriceData{
			QiniuMarket: qiniuMarketTextPriceSnapshotForTest(1),
		},
	}

	if ShouldUseQiniuOfficialLedger(relayInfo) {
		t.Fatalf("expected qiniu market realtime request to bypass official ledger skip")
	}
	if got := QiniuOfficialLedgerLogQuota(relayInfo, 12345); got != 12345 {
		t.Fatalf("expected qiniu market realtime log quota to keep actual quota, got %d", got)
	}
}

func TestQiniuMarketRealtimePreConsumeAndSettleDebitDelta(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4211
	const tokenID = 4211
	const initialQuota = 10000
	const tokenRemain = 10000
	const preConsumed = 3600
	const actualQuota = 4600
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "qiniu-market-token-debit", tokenRemain)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("token_quota", tokenRemain)
	relayInfo := &relaycommon.RelayInfo{
		UserId:            userID,
		TokenId:           tokenID,
		TokenKey:          "qiniu-market-token-debit",
		TokenUnlimited:    false,
		QiniuManagedToken: true,
		ForcePreConsume:   true,
		PriceData: types.PriceData{
			QiniuMarket: qiniuMarketTextPriceSnapshotForTest(1),
		},
	}

	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}
	if got := getUserQuota(t, userID); got != initialQuota-preConsumed {
		t.Fatalf("expected user quota after pre-consume %d, got %d", initialQuota-preConsumed, got)
	}
	if got := getTokenRemainQuota(t, tokenID); got != tokenRemain-preConsumed {
		t.Fatalf("expected token quota after pre-consume %d, got %d", tokenRemain-preConsumed, got)
	}

	requireNoError(t, SettleBilling(ctx, relayInfo, actualQuota))

	if got := getUserQuota(t, userID); got != initialQuota-actualQuota {
		t.Fatalf("expected user quota after settle %d, got %d", initialQuota-actualQuota, got)
	}
	if got := getTokenRemainQuota(t, tokenID); got != tokenRemain-actualQuota {
		t.Fatalf("expected token quota after settle %d, got %d", tokenRemain-actualQuota, got)
	}
}

func TestQiniuMarketRealtimePreConsumeRejectsInsufficientBalance(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4219
	const tokenID = 4219
	const initialQuota = 100
	const tokenRemain = 100
	const preConsumed = 200
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "qiniu-market-token-insufficient", tokenRemain)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("token_quota", tokenRemain)
	relayInfo := &relaycommon.RelayInfo{
		UserId:            userID,
		TokenId:           tokenID,
		TokenKey:          "qiniu-market-token-insufficient",
		TokenUnlimited:    false,
		QiniuManagedToken: true,
		ForcePreConsume:   true,
		PriceData: types.PriceData{
			QiniuMarket: qiniuMarketTextPriceSnapshotForTest(1),
		},
	}

	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr == nil {
		t.Fatalf("expected qiniu market realtime pre-consume to reject insufficient balance")
	}
	if got := getUserQuota(t, userID); got != initialQuota {
		t.Fatalf("expected user quota unchanged after rejected realtime consume, got %d", got)
	}
}

func TestQiniuMarketRealtimePreConsumeAndSettleRefundDelta(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4212
	const tokenID = 4212
	const initialQuota = 10000
	const tokenRemain = 10000
	const preConsumed = 3600
	const actualQuota = 2400
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "qiniu-market-token-refund", tokenRemain)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("token_quota", tokenRemain)
	relayInfo := &relaycommon.RelayInfo{
		UserId:            userID,
		TokenId:           tokenID,
		TokenKey:          "qiniu-market-token-refund",
		TokenUnlimited:    false,
		QiniuManagedToken: true,
		ForcePreConsume:   true,
		PriceData: types.PriceData{
			QiniuMarket: qiniuMarketTextPriceSnapshotForTest(1),
		},
	}

	if apiErr := PreConsumeBilling(ctx, preConsumed, relayInfo); apiErr != nil {
		t.Fatalf("expected qiniu market pre-consume to succeed, got %v", apiErr)
	}
	requireNoError(t, SettleBilling(ctx, relayInfo, actualQuota))

	if got := getUserQuota(t, userID); got != initialQuota-actualQuota {
		t.Fatalf("expected user quota after refund settle %d, got %d", initialQuota-actualQuota, got)
	}
	if got := getTokenRemainQuota(t, tokenID); got != tokenRemain-actualQuota {
		t.Fatalf("expected token quota after refund settle %d, got %d", tokenRemain-actualQuota, got)
	}
}

func TestQiniuMarketRealtimeTextConsumeLogUsesActualQuota(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4213
	const tokenID = 4213
	const channelID = 4213
	const initialQuota = 10000
	const tokenRemain = 10000
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "qiniu-market-token-log", tokenRemain)
	seedChannel(t, channelID)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("token_name", "qiniu-market-token")
	relayInfo := &relaycommon.RelayInfo{
		UserId:            userID,
		TokenId:           tokenID,
		TokenKey:          "qiniu-market-token-log",
		TokenUnlimited:    false,
		QiniuManagedToken: true,
		OriginModelName:   "kimi-k2",
		UsingGroup:        "default",
		StartTime:         time.Now(),
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: channelID},
		PriceData: types.PriceData{
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
			QiniuMarket:    qiniuMarketTextPriceSnapshotForTest(1),
		},
	}

	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
	}, []string{"请求成功"})

	log := getLastLog(t)
	if log == nil {
		t.Fatalf("expected qiniu market consume log")
	}
	if log.Quota != 3600 {
		t.Fatalf("expected qiniu market log actual quota 3600, got %d", log.Quota)
	}
	other, err := common.StrToMap(log.Other)
	requireNoError(t, err)
	if other["billing_source"] != QiniuMarketRealtimeBillingSource || other["price_source"] != QiniuMarketPriceSource {
		t.Fatalf("expected qiniu market realtime source in other, got %#v", other)
	}
	if other["token_provider"] != "qiniu" {
		t.Fatalf("expected qiniu token provider, got %#v", other["token_provider"])
	}
	if other["qiniu_realtime_billing_skipped"] != nil {
		t.Fatalf("expected no qiniu realtime skipped marker, got %#v", other)
	}
	if other["qiniu_market_converted_quota"] != float64(3600) {
		t.Fatalf("expected converted quota 3600, got %#v", other["qiniu_market_converted_quota"])
	}
	assertQiniuQuotaSyncTaskCount(t, userID, tokenID, 0)
}

func TestLogTaskConsumptionRecordsQiniuMarketRealtimeCharge(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4202
	const tokenID = 4202
	const initialQuota = 10000
	seedUser(t, userID, initialQuota)
	seedChannel(t, 4202)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	ctx.Set("token_name", "qiniu-token")
	info := &relaycommon.RelayInfo{
		UserId:            userID,
		TokenId:           tokenID,
		TokenKey:          "qiniu-managed-token",
		QiniuManagedToken: true,
		OriginModelName:   "test-model",
		UsingGroup:        "default",
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 4202},
		TaskRelayInfo:     &relaycommon.TaskRelayInfo{Action: "image"},
		PriceData: types.PriceData{
			Quota:       1200,
			ModelPrice:  0.0024,
			QiniuMarket: qiniuMarketUnitPriceSnapshotForTest(1),
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
	}

	LogTaskConsumption(ctx, info)

	if got := getUserQuota(t, userID); got != initialQuota {
		t.Fatalf("expected user quota unchanged, got %d", got)
	}
	if got := getUserUsedQuota(t, userID); got != 1200 {
		t.Fatalf("expected qiniu market task used quota 1200, got %d", got)
	}
	log := getLastLog(t)
	if log == nil {
		t.Fatalf("expected qiniu request log to be recorded")
	}
	if log.Quota != 1200 {
		t.Fatalf("expected qiniu request log quota 1200, got %d", log.Quota)
	}
	other, err := common.StrToMap(log.Other)
	requireNoError(t, err)
	if other["billing_source"] != QiniuMarketRealtimeBillingSource || other["price_source"] != QiniuMarketPriceSource {
		t.Fatalf("expected qiniu market realtime marker in log other, got %#v", other)
	}
	if other["qiniu_official_ledger_pending"] != nil || other["qiniu_realtime_billing_skipped"] != nil {
		t.Fatalf("expected no official ledger pending marker, got %#v", other)
	}
	if other["qiniu_market_unit_name"] != "request" {
		t.Fatalf("expected qiniu market unit snapshot, got %#v", other)
	}
}

func TestLogTaskConsumptionRecordsQiniuMarketOtherRatiosSnapshot(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	const userID = 4204
	const tokenID = 4204
	seedUser(t, userID, 10000)
	seedChannel(t, 4204)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
	ctx.Set("token_name", "qiniu-token")
	info := &relaycommon.RelayInfo{
		UserId:            userID,
		TokenId:           tokenID,
		TokenKey:          "qiniu-managed-token-ratio",
		QiniuManagedToken: true,
		OriginModelName:   "test-video-model",
		UsingGroup:        "default",
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 4204},
		TaskRelayInfo:     &relaycommon.TaskRelayInfo{Action: "video"},
		PriceData: types.PriceData{
			Quota:       3600,
			ModelPrice:  0.0024,
			QiniuMarket: qiniuMarketUnitPriceSnapshotForTest(1),
			OtherRatios: map[string]float64{
				"seconds": 2,
				"size":    1.5,
			},
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
	}

	LogTaskConsumption(ctx, info)

	log := getLastLog(t)
	if log == nil {
		t.Fatalf("expected qiniu request log to be recorded")
	}
	other, err := common.StrToMap(log.Other)
	requireNoError(t, err)
	if other["qiniu_market_base_amount"] != float64(0.0024) {
		t.Fatalf("expected base amount 0.0024, got %#v", other["qiniu_market_base_amount"])
	}
	if other["qiniu_market_other_multiplier"] != float64(3) {
		t.Fatalf("expected other multiplier 3, got %#v", other["qiniu_market_other_multiplier"])
	}
	if other["qiniu_market_final_amount"] != float64(0.0072) {
		t.Fatalf("expected final amount 0.0072, got %#v", other["qiniu_market_final_amount"])
	}
	if other["qiniu_market_converted_quota"] != float64(3600) {
		t.Fatalf("expected converted quota 3600, got %#v", other["qiniu_market_converted_quota"])
	}
	ratios, ok := other["qiniu_market_other_ratios"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected qiniu market other ratios, got %#v", other["qiniu_market_other_ratios"])
	}
	if ratios["seconds"] != float64(2) || ratios["size"] != float64(1.5) {
		t.Fatalf("expected seconds and size ratios, got %#v", ratios)
	}
}

func qiniuMarketUnitPriceSnapshotForTest(groupRatio float64) *types.QiniuMarketPriceSnapshot {
	return &types.QiniuMarketPriceSnapshot{
		PriceSource:       QiniuMarketPriceSource,
		BillingSource:     QiniuMarketRealtimeBillingSource,
		BillingMode:       QiniuMarketBillingModeUnit,
		MarketModelID:     "qiniu-task-market-only",
		RuleIndex:         0,
		UnitDetailKey:     "request",
		UnitName:          "request",
		UnitSize:          1,
		UnitPrice:         0.0024,
		UnitCurrency:      "CNY",
		UnitQuantity:      1,
		AmountToQuotaRate: 500000,
		GroupRatio:        groupRatio,
		RoundingMode:      qiniuMarketRoundingMode,
		CatalogStatus:     QiniuMarketCatalogStatusFresh,
		ConvertedQuota:    1200,
	}
}

func TestChargeViolationFeeSkipsQiniuOfficialLedger(t *testing.T) {
	truncate(t)
	configureQiniuOfficialLedgerSettingForTest(t, "http://127.0.0.1")

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{UserId: 4203, QiniuManagedToken: true}
	charged := ChargeViolationFeeIfNeeded(ctx, relayInfo, types.NewErrorWithStatusCode(context.Canceled, types.ErrorCodeViolationFeeGrokCSAM, http.StatusBadRequest))
	if charged {
		t.Fatalf("expected qiniu official ledger request to skip violation fee")
	}
}
