package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func seedQiniuMarketCatalogSnapshotForTest(t *testing.T, models []dto.QiniuMarketModel, expiresAt time.Time) {
	t.Helper()
	setting := operation_setting.GetQiniuKeySetting()
	qiniuMarketCatalogCache.mu.Lock()
	defer qiniuMarketCatalogCache.mu.Unlock()
	qiniuMarketCatalogCache.cacheKey = qiniuMarketCatalogCacheKey(setting)
	qiniuMarketCatalogCache.models = cloneQiniuMarketModels(models)
	qiniuMarketCatalogCache.lastSuccessTime = time.Now().Add(-time.Minute)
	qiniuMarketCatalogCache.expiresAt = expiresAt
	qiniuMarketCatalogCache.lastError = ""
}

func qiniuMarketTextModel(modelID string, rules ...dto.QiniuMarketPricingRuleV2) dto.QiniuMarketModel {
	return dto.QiniuMarketModel{
		ID:             modelID,
		PricingRulesV2: rules,
	}
}

func qiniuMarketTextRule(inputRange []int64, outputRange []int64, inputPrice float64, outputPrice float64) dto.QiniuMarketPricingRuleV2 {
	return dto.QiniuMarketPricingRuleV2{
		InputRange:  inputRange,
		OutputRange: outputRange,
		DetailsV2: map[string]dto.QiniuMarketPricingDetail{
			"input": {
				UnitName:  "token",
				UnitSize:  1000,
				UnitPrice: qiniuFloat64Ptr(inputPrice),
				Name:      "输入",
			},
			"output": {
				UnitName:  "token",
				UnitSize:  1000,
				UnitPrice: qiniuFloat64Ptr(outputPrice),
				Name:      "输出",
			},
		},
	}
}

func TestResolveQiniuMarketPriceDataUsesCurrentFreshSnapshot(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
		setting.MarketCatalogFallbackEnabled = false
	})
	seedQiniuMarketCatalogSnapshotForTest(t, []dto.QiniuMarketModel{
		qiniuMarketTextModel("kimi-k2", qiniuMarketTextRule([]int64{0, 999999}, []int64{0, 999999}, 0.004, 0.016)),
	}, time.Now().Add(time.Minute))

	priceData, err := ResolveQiniuMarketPriceData(context.Background(), "kimi-k2", 1000, 200, types.GroupRatioInfo{GroupRatio: 2})
	require.NoError(t, err)
	require.NotNil(t, priceData.QiniuMarket)
	require.Equal(t, QiniuMarketPriceSource, priceData.QiniuMarket.PriceSource)
	require.Equal(t, "kimi-k2", priceData.QiniuMarket.MarketModelID)
	require.Equal(t, QiniuMarketCatalogStatusFresh, priceData.QiniuMarket.CatalogStatus)
	require.Equal(t, float64(0.004), priceData.QiniuMarket.InputUnitPrice)
	require.Equal(t, float64(0.016), priceData.QiniuMarket.OutputUnitPrice)
	require.Equal(t, float64(500000), priceData.QiniuMarket.AmountToQuotaRate)
	require.Equal(t, 7200, priceData.QuotaToPreConsume)
	require.Equal(t, 7200, priceData.QiniuMarket.ConvertedQuota)
}

func TestResolveQiniuMarketPriceDataSelectsUniqueRange(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
	})
	seedQiniuMarketCatalogSnapshotForTest(t, []dto.QiniuMarketModel{
		qiniuMarketTextModel("range-model",
			qiniuMarketTextRule([]int64{0, 1000}, []int64{0, 500}, 0.001, 0.002),
			qiniuMarketTextRule([]int64{1001, 2000}, []int64{0, 500}, 0.003, 0.004),
		),
	}, time.Now().Add(time.Minute))

	priceData, err := ResolveQiniuMarketPriceData(context.Background(), "range-model", 1500, 200, types.GroupRatioInfo{GroupRatio: 1})
	require.NoError(t, err)
	require.Equal(t, 1, priceData.QiniuMarket.RuleIndex)
	require.Equal(t, float64(0.003), priceData.QiniuMarket.InputUnitPrice)
	require.Equal(t, 2650, priceData.QuotaToPreConsume)
}

func TestResolveQiniuMarketPriceDataRejectsAmbiguousRange(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
	})
	seedQiniuMarketCatalogSnapshotForTest(t, []dto.QiniuMarketModel{
		qiniuMarketTextModel("ambiguous-model",
			qiniuMarketTextRule(nil, nil, 0.001, 0.002),
			qiniuMarketTextRule(nil, nil, 0.003, 0.004),
		),
	}, time.Now().Add(time.Minute))

	_, err := ResolveQiniuMarketPriceData(context.Background(), "ambiguous-model", 100, 100, types.GroupRatioInfo{GroupRatio: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "price_missing")
	require.Contains(t, err.Error(), "无法唯一匹配")
}

func TestResolveQiniuMarketPriceDataRejectsUnknownOutputRangeWithFallbackRule(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
	})
	seedQiniuMarketCatalogSnapshotForTest(t, []dto.QiniuMarketModel{
		qiniuMarketTextModel("fallback-range-model",
			qiniuMarketTextRule([]int64{0, 999999}, nil, 0.001, 0.002),
			qiniuMarketTextRule([]int64{0, 999999}, []int64{101, 500}, 0.003, 0.004),
		),
	}, time.Now().Add(time.Minute))

	_, err := ResolveQiniuMarketPriceData(context.Background(), "fallback-range-model", 100, 0, types.GroupRatioInfo{GroupRatio: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "price_missing")
	require.Contains(t, err.Error(), "无法确定 output token 范围")
}

func TestResolveQiniuMarketPriceDataRejectsMissingPrice(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
	})
	rule := qiniuMarketTextRule([]int64{0, 999999}, []int64{0, 999999}, 0.001, 0.002)
	rule.DetailsV2["output"] = dto.QiniuMarketPricingDetail{
		UnitName: "token",
		UnitSize: 1000,
		Name:     "输出",
	}
	seedQiniuMarketCatalogSnapshotForTest(t, []dto.QiniuMarketModel{
		qiniuMarketTextModel("missing-price-model", rule),
	}, time.Now().Add(time.Minute))

	_, err := ResolveQiniuMarketPriceData(context.Background(), "missing-price-model", 100, 100, types.GroupRatioInfo{GroupRatio: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "缺少 output CNY unit_price")
}

func TestResolveQiniuMarketPriceDataRejectsUnsupportedUnit(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
	})
	rule := qiniuMarketTextRule([]int64{0, 999999}, []int64{0, 999999}, 0.001, 0.002)
	input := rule.DetailsV2["input"]
	input.UnitName = "request"
	rule.DetailsV2["input"] = input
	seedQiniuMarketCatalogSnapshotForTest(t, []dto.QiniuMarketModel{
		qiniuMarketTextModel("bad-unit-model", rule),
	}, time.Now().Add(time.Minute))

	_, err := ResolveQiniuMarketPriceData(context.Background(), "bad-unit-model", 100, 100, types.GroupRatioInfo{GroupRatio: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "input 单位不是 token")
}

func TestResolveQiniuMarketPriceDataRejectsDisabledCatalog(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = false
	})
	seedQiniuMarketCatalogSnapshotForTest(t, []dto.QiniuMarketModel{
		qiniuMarketTextModel("disabled-model", qiniuMarketTextRule(nil, nil, 0.001, 0.002)),
	}, time.Now().Add(time.Minute))

	_, err := ResolveQiniuMarketPriceData(context.Background(), "disabled-model", 100, 100, types.GroupRatioInfo{GroupRatio: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "catalog 未启用")
}

func TestResolveQiniuMarketPriceDataAllowsCurrentStaleSnapshot(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
	})
	seedQiniuMarketCatalogSnapshotForTest(t, []dto.QiniuMarketModel{
		qiniuMarketTextModel("stale-model", qiniuMarketTextRule([]int64{0, 999999}, []int64{0, 999999}, 0.002, 0.004)),
	}, time.Now().Add(-time.Minute))

	priceData, err := ResolveQiniuMarketPriceData(context.Background(), "stale-model", 1000, 500, types.GroupRatioInfo{GroupRatio: 1})
	require.NoError(t, err)
	require.Equal(t, QiniuMarketCatalogStatusStale, priceData.QiniuMarket.CatalogStatus)
	require.True(t, priceData.QiniuMarket.CatalogStale)
	require.Equal(t, 2000, priceData.QuotaToPreConsume)
}

func TestResolveQiniuMarketPriceDataRejectsMissingSnapshotAfterFallbackFailure(t *testing.T) {
	withQiniuMarketSetting(t, func(setting *operation_setting.QiniuKeySetting) {
		setting.MarketCatalogEnabled = true
	})
	resetQiniuMarketCatalogCacheForTest()
	setQiniuMarketCatalogFetcherForTest(func(context.Context) ([]dto.QiniuMarketModel, error) {
		return nil, errors.New("market unavailable")
	})

	_, err := ResolveQiniuMarketPriceData(context.Background(), "missing-snapshot-model", 100, 100, types.GroupRatioInfo{GroupRatio: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "无可用 market catalog snapshot")
}
