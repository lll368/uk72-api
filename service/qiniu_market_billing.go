package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
)

const (
	QiniuMarketRealtimeBillingSource = "qiniu_market_realtime"
	QiniuMarketBillingModeToken      = "token"
	QiniuMarketBillingModeUnit       = "unit"
	qiniuMarketCurrencyCNY           = "CNY"
	qiniuMarketRoundingMode          = "decimal_round_0"
)

// ResolveQiniuMarketPriceData 从当前进程内七牛 market catalog 快照解析请求级价格。
// 内存无快照时会兜底刷新一次；缺少确定 CNY token 单价时必须在上游调用前失败。
func ResolveQiniuMarketPriceData(ctx context.Context, modelName string, inputTokens int, outputTokens int, groupRatioInfo types.GroupRatioInfo) (types.PriceData, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return types.PriceData{}, err
		}
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return types.PriceData{}, qiniuMarketPriceMissing("模型名为空")
	}

	snapshot := GetCurrentQiniuMarketCatalogSnapshot(ctx)
	if snapshot.Status == QiniuMarketCatalogStatusDisabled {
		return types.PriceData{}, qiniuMarketPriceMissing("catalog 未启用")
	}
	if len(snapshot.Models) == 0 {
		return types.PriceData{}, qiniuMarketPriceMissing(fmt.Sprintf("无可用 market catalog snapshot status=%s", snapshot.Status))
	}

	marketModel, ok := findQiniuMarketModel(snapshot.Models, modelName)
	if !ok {
		return types.PriceData{}, qiniuMarketPriceMissing(fmt.Sprintf("模型 %s 未在 market catalog 中匹配", modelName))
	}

	resolvedOutputTokens, outputTokensKnown, err := resolveQiniuMarketEstimatedOutputTokens(marketModel, inputTokens, outputTokens)
	if err != nil {
		return types.PriceData{}, err
	}

	rule, ruleIndex, err := matchQiniuMarketPricingRule(marketModel, inputTokens, resolvedOutputTokens, outputTokensKnown)
	if err != nil {
		return types.PriceData{}, err
	}
	inputDetail, err := resolveQiniuMarketTokenPriceDetail(rule, "input")
	if err != nil {
		return types.PriceData{}, err
	}
	outputDetail, err := resolveQiniuMarketTokenPriceDetail(rule, "output")
	if err != nil {
		return types.PriceData{}, err
	}

	priceSnapshot := &types.QiniuMarketPriceSnapshot{
		PriceSource:           QiniuMarketPriceSource,
		BillingSource:         QiniuMarketRealtimeBillingSource,
		BillingMode:           QiniuMarketBillingModeToken,
		MarketModelID:         marketModel.ID,
		RuleIndex:             ruleIndex,
		InputRange:            cloneInt64Slice(rule.InputRange),
		OutputRange:           cloneInt64Slice(rule.OutputRange),
		InputUnitName:         inputDetail.UnitName,
		InputUnitSize:         inputDetail.UnitSize,
		InputUnitPrice:        *inputDetail.UnitPrice,
		InputCurrency:         qiniuMarketCurrencyCNY,
		OutputUnitName:        outputDetail.UnitName,
		OutputUnitSize:        outputDetail.UnitSize,
		OutputUnitPrice:       *outputDetail.UnitPrice,
		OutputCurrency:        qiniuMarketCurrencyCNY,
		AmountToQuotaRate:     common.QuotaPerUnit,
		GroupRatio:            groupRatioInfo.GroupRatio,
		RoundingMode:          qiniuMarketRoundingMode,
		CatalogStatus:         snapshot.Status,
		CatalogStale:          snapshot.Stale,
		CatalogFromCache:      snapshot.FromCache,
		EstimatedInputTokens:  inputTokens,
		EstimatedOutputTokens: resolvedOutputTokens,
	}
	if !snapshot.LastSuccessTime.IsZero() {
		priceSnapshot.CatalogLastSuccessUnix = snapshot.LastSuccessTime.Unix()
	}
	preConsumedQuota, estimatedAmount := CalculateQiniuMarketQuota(priceSnapshot, inputTokens, resolvedOutputTokens)
	priceSnapshot.EstimatedAmount, _ = estimatedAmount.Float64()
	priceSnapshot.ConvertedQuota = preConsumedQuota

	return types.PriceData{
		GroupRatioInfo:    groupRatioInfo,
		QuotaToPreConsume: preConsumedQuota,
		QiniuMarket:       priceSnapshot,
	}, nil
}

// ResolveQiniuMarketPerCallPriceData 为 MJ/task 等按次路径解析七牛市场价。
// 首批只支持 details_v2 中唯一的非 token CNY 单价项；多项组合价格留给后续独立设计。
func ResolveQiniuMarketPerCallPriceData(ctx context.Context, modelName string, groupRatioInfo types.GroupRatioInfo) (types.PriceData, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return types.PriceData{}, err
		}
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return types.PriceData{}, qiniuMarketPriceMissing("模型名为空")
	}

	snapshot := GetCurrentQiniuMarketCatalogSnapshot(ctx)
	if snapshot.Status == QiniuMarketCatalogStatusDisabled {
		return types.PriceData{}, qiniuMarketPriceMissing("catalog 未启用")
	}
	if len(snapshot.Models) == 0 {
		return types.PriceData{}, qiniuMarketPriceMissing(fmt.Sprintf("无可用 market catalog snapshot status=%s", snapshot.Status))
	}

	marketModel, ok := findQiniuMarketModel(snapshot.Models, modelName)
	if !ok {
		return types.PriceData{}, qiniuMarketPriceMissing(fmt.Sprintf("模型 %s 未在 market catalog 中匹配", modelName))
	}
	rule, ruleIndex, err := matchQiniuMarketPerCallPricingRule(marketModel)
	if err != nil {
		return types.PriceData{}, err
	}
	detailKey, detail, err := resolveQiniuMarketUnitPriceDetail(rule)
	if err != nil {
		return types.PriceData{}, err
	}

	priceSnapshot := &types.QiniuMarketPriceSnapshot{
		PriceSource:           QiniuMarketPriceSource,
		BillingSource:         QiniuMarketRealtimeBillingSource,
		BillingMode:           QiniuMarketBillingModeUnit,
		MarketModelID:         marketModel.ID,
		RuleIndex:             ruleIndex,
		InputRange:            cloneInt64Slice(rule.InputRange),
		OutputRange:           cloneInt64Slice(rule.OutputRange),
		UnitDetailKey:         detailKey,
		UnitName:              detail.UnitName,
		UnitSize:              detail.UnitSize,
		UnitPrice:             *detail.UnitPrice,
		UnitCurrency:          qiniuMarketCurrencyCNY,
		UnitQuantity:          1,
		AmountToQuotaRate:     common.QuotaPerUnit,
		GroupRatio:            groupRatioInfo.GroupRatio,
		RoundingMode:          qiniuMarketRoundingMode,
		CatalogStatus:         snapshot.Status,
		CatalogStale:          snapshot.Stale,
		CatalogFromCache:      snapshot.FromCache,
		EstimatedInputTokens:  0,
		EstimatedOutputTokens: 0,
	}
	if !snapshot.LastSuccessTime.IsZero() {
		priceSnapshot.CatalogLastSuccessUnix = snapshot.LastSuccessTime.Unix()
	}
	quota, amount := CalculateQiniuMarketQuota(priceSnapshot, 0, 0)
	priceSnapshot.EstimatedAmount, _ = amount.Float64()
	priceSnapshot.ConvertedQuota = quota

	return types.PriceData{
		UsePrice:          true,
		ModelPrice:        priceSnapshot.UnitPrice,
		GroupRatioInfo:    groupRatioInfo,
		Quota:             quota,
		QuotaToPreConsume: quota,
		QiniuMarket:       priceSnapshot,
	}, nil
}

func findQiniuMarketModel(models []dto.QiniuMarketModel, modelName string) (dto.QiniuMarketModel, bool) {
	for _, item := range models {
		if strings.TrimSpace(item.ID) == modelName {
			return item, true
		}
	}
	return dto.QiniuMarketModel{}, false
}

func resolveQiniuMarketEstimatedOutputTokens(model dto.QiniuMarketModel, inputTokens int, outputTokens int) (int, bool, error) {
	if outputTokens > 0 {
		return outputTokens, true, nil
	}
	constraints := model.ModelConstraints
	if constraints.MaxDefaultCompletionTokens > 0 {
		return int(clampQiniuMarketInt64ToInt(constraints.MaxDefaultCompletionTokens)), true, nil
	}
	if constraints.MaxCompletionTokens > 0 {
		return int(clampQiniuMarketInt64ToInt(constraints.MaxCompletionTokens)), true, nil
	}
	if constraints.MaxTokens > int64(inputTokens) {
		return int(clampQiniuMarketInt64ToInt(constraints.MaxTokens - int64(inputTokens))), true, nil
	}
	return 0, false, nil
}

func clampQiniuMarketInt64ToInt(value int64) int64 {
	maxInt := int64(^uint(0) >> 1)
	if value > maxInt {
		return maxInt
	}
	return value
}

func matchQiniuMarketPricingRule(model dto.QiniuMarketModel, inputTokens int, outputTokens int, outputTokensKnown bool) (dto.QiniuMarketPricingRuleV2, int, error) {
	if len(model.PricingRulesV2) == 0 {
		return dto.QiniuMarketPricingRuleV2{}, -1, qiniuMarketPriceMissing(fmt.Sprintf("模型 %s 缺少 pricing_rules_v2", model.ID))
	}
	matchedIndex := -1
	var matchedRule dto.QiniuMarketPricingRuleV2
	hasOutputRange := false
	for index, rule := range model.PricingRulesV2 {
		if !qiniuMarketRangeMatches(rule.InputRange, inputTokens) {
			continue
		}
		if len(rule.OutputRange) > 0 {
			hasOutputRange = true
		}
		if outputTokensKnown {
			if !qiniuMarketRangeMatches(rule.OutputRange, outputTokens) {
				continue
			}
		} else if len(rule.OutputRange) > 0 {
			continue
		}
		if matchedIndex >= 0 {
			return dto.QiniuMarketPricingRuleV2{}, -1, qiniuMarketPriceMissing(fmt.Sprintf("模型 %s pricing_rules_v2 无法唯一匹配", model.ID))
		}
		matchedIndex = index
		matchedRule = rule
	}
	if !outputTokensKnown && hasOutputRange {
		// 只要当前 input 档位存在 output range，就不能用未知输出长度命中无 range 兜底规则，避免错扣低价档。
		return dto.QiniuMarketPricingRuleV2{}, -1, qiniuMarketPriceMissing(fmt.Sprintf("模型 %s 无法确定 output token 范围", model.ID))
	}
	if matchedIndex < 0 {
		return dto.QiniuMarketPricingRuleV2{}, -1, qiniuMarketPriceMissing(fmt.Sprintf("模型 %s pricing_rules_v2 没有匹配当前 token 范围", model.ID))
	}
	return matchedRule, matchedIndex, nil
}

func matchQiniuMarketPerCallPricingRule(model dto.QiniuMarketModel) (dto.QiniuMarketPricingRuleV2, int, error) {
	if len(model.PricingRulesV2) == 0 {
		return dto.QiniuMarketPricingRuleV2{}, -1, qiniuMarketPriceMissing(fmt.Sprintf("模型 %s 缺少 pricing_rules_v2", model.ID))
	}
	matchedIndex := -1
	var matchedRule dto.QiniuMarketPricingRuleV2
	for index, rule := range model.PricingRulesV2 {
		if !qiniuMarketRangeMatches(rule.InputRange, 0) || !qiniuMarketRangeMatches(rule.OutputRange, 0) {
			continue
		}
		if matchedIndex >= 0 {
			return dto.QiniuMarketPricingRuleV2{}, -1, qiniuMarketPriceMissing(fmt.Sprintf("模型 %s per-call pricing_rules_v2 无法唯一匹配", model.ID))
		}
		matchedIndex = index
		matchedRule = rule
	}
	if matchedIndex < 0 {
		return dto.QiniuMarketPricingRuleV2{}, -1, qiniuMarketPriceMissing(fmt.Sprintf("模型 %s per-call pricing_rules_v2 没有匹配规则", model.ID))
	}
	return matchedRule, matchedIndex, nil
}

func qiniuMarketRangeMatches(valueRange []int64, tokens int) bool {
	if len(valueRange) == 0 {
		return true
	}
	if len(valueRange) < 2 {
		return false
	}
	tokenCount := int64(tokens)
	return tokenCount >= valueRange[0] && tokenCount <= valueRange[1]
}

func resolveQiniuMarketTokenPriceDetail(rule dto.QiniuMarketPricingRuleV2, key string) (dto.QiniuMarketPricingDetail, error) {
	detail, ok := findQiniuMarketPricingDetail(rule.DetailsV2, key)
	if !ok {
		return dto.QiniuMarketPricingDetail{}, qiniuMarketPriceMissing(fmt.Sprintf("缺少 details_v2.%s", key))
	}
	if !strings.EqualFold(strings.TrimSpace(detail.UnitName), "token") {
		return dto.QiniuMarketPricingDetail{}, qiniuMarketPriceMissing(fmt.Sprintf("%s 单位不是 token", key))
	}
	if detail.UnitSize <= 0 {
		return dto.QiniuMarketPricingDetail{}, qiniuMarketPriceMissing(fmt.Sprintf("%s unit_size 无效", key))
	}
	if detail.UnitPrice == nil {
		return dto.QiniuMarketPricingDetail{}, qiniuMarketPriceMissing(fmt.Sprintf("缺少 %s CNY unit_price", key))
	}
	return detail, nil
}

func resolveQiniuMarketUnitPriceDetail(rule dto.QiniuMarketPricingRuleV2) (string, dto.QiniuMarketPricingDetail, error) {
	if len(rule.DetailsV2) == 0 {
		return "", dto.QiniuMarketPricingDetail{}, qiniuMarketPriceMissing("缺少 per-call details_v2")
	}
	keys := make([]string, 0, len(rule.DetailsV2))
	for key := range rule.DetailsV2 {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	matchedKey := ""
	var matchedDetail dto.QiniuMarketPricingDetail
	for _, key := range keys {
		detail := rule.DetailsV2[key]
		unitName := strings.TrimSpace(detail.UnitName)
		if unitName == "" || strings.EqualFold(unitName, "token") {
			continue
		}
		if detail.UnitSize <= 0 {
			return "", dto.QiniuMarketPricingDetail{}, qiniuMarketPriceMissing(fmt.Sprintf("%s unit_size 无效", key))
		}
		if detail.UnitPrice == nil {
			return "", dto.QiniuMarketPricingDetail{}, qiniuMarketPriceMissing(fmt.Sprintf("缺少 %s CNY unit_price", key))
		}
		if matchedKey != "" {
			return "", dto.QiniuMarketPricingDetail{}, qiniuMarketPriceMissing("per-call details_v2 无法唯一匹配")
		}
		matchedKey = key
		matchedDetail = detail
	}
	if matchedKey == "" {
		return "", dto.QiniuMarketPricingDetail{}, qiniuMarketPriceMissing("缺少可用于 per-call 的非 token CNY 单价")
	}
	return matchedKey, matchedDetail, nil
}

func findQiniuMarketPricingDetail(details map[string]dto.QiniuMarketPricingDetail, key string) (dto.QiniuMarketPricingDetail, bool) {
	if len(details) == 0 {
		return dto.QiniuMarketPricingDetail{}, false
	}
	if detail, ok := details[key]; ok {
		return detail, true
	}
	for itemKey, detail := range details {
		if strings.EqualFold(strings.TrimSpace(itemKey), key) {
			return detail, true
		}
	}
	return dto.QiniuMarketPricingDetail{}, false
}

func qiniuMarketPriceMissing(reason string) error {
	return fmt.Errorf("price_missing: 市场价不可用：%s", reason)
}

// CalculateQiniuMarketQuota 用请求级七牛市场价快照计算 quota 和 CNY 金额。
func CalculateQiniuMarketQuota(snapshot *types.QiniuMarketPriceSnapshot, inputTokens int, outputTokens int) (int, decimal.Decimal) {
	if snapshot == nil {
		return 0, decimal.Zero
	}
	if snapshot.BillingMode == QiniuMarketBillingModeUnit {
		if snapshot.UnitSize <= 0 {
			return 0, decimal.Zero
		}
		quantity := snapshot.UnitQuantity
		if quantity <= 0 {
			quantity = 1
		}
		amount := decimal.NewFromFloat(snapshot.UnitPrice).
			Mul(decimal.NewFromFloat(quantity)).
			Div(decimal.NewFromInt(snapshot.UnitSize))
		return qiniuMarketQuotaFromAmount(snapshot, amount), amount
	}
	if inputTokens+outputTokens <= 0 {
		return 0, decimal.Zero
	}
	inputAmount := decimal.NewFromInt(int64(inputTokens)).
		Mul(decimal.NewFromFloat(snapshot.InputUnitPrice)).
		Div(decimal.NewFromInt(snapshot.InputUnitSize))
	outputAmount := decimal.NewFromInt(int64(outputTokens)).
		Mul(decimal.NewFromFloat(snapshot.OutputUnitPrice)).
		Div(decimal.NewFromInt(snapshot.OutputUnitSize))
	amount := inputAmount.Add(outputAmount)
	return qiniuMarketQuotaFromAmount(snapshot, amount), amount
}

func qiniuMarketQuotaFromAmount(snapshot *types.QiniuMarketPriceSnapshot, amount decimal.Decimal) int {
	quotaDecimal := amount.
		Mul(decimal.NewFromFloat(snapshot.AmountToQuotaRate)).
		Mul(decimal.NewFromFloat(snapshot.GroupRatio))
	quota := int(quotaDecimal.Round(0).IntPart())
	if amount.IsPositive() && snapshot.GroupRatio > 0 && snapshot.AmountToQuotaRate > 0 && quota == 0 {
		quota = 1
	}
	return quota
}

func InjectQiniuMarketRealtimeInfo(other map[string]interface{}, snapshot *types.QiniuMarketPriceSnapshot, inputTokens int, outputTokens int, actualQuota int, usageMissing bool) {
	if other == nil || snapshot == nil {
		return
	}
	_, amount := CalculateQiniuMarketQuota(snapshot, inputTokens, outputTokens)
	amountValue, _ := amount.Float64()
	other["token_provider"] = "qiniu"
	other["billing_source"] = QiniuMarketRealtimeBillingSource
	other["price_source"] = QiniuMarketPriceSource
	other["qiniu_market_billing_mode"] = snapshot.BillingMode
	other["qiniu_market_model_id"] = snapshot.MarketModelID
	other["qiniu_market_rule_index"] = snapshot.RuleIndex
	other["qiniu_market_input_unit_name"] = snapshot.InputUnitName
	other["qiniu_market_input_unit_size"] = snapshot.InputUnitSize
	other["qiniu_market_input_unit_price"] = snapshot.InputUnitPrice
	other["qiniu_market_input_currency"] = snapshot.InputCurrency
	other["qiniu_market_output_unit_name"] = snapshot.OutputUnitName
	other["qiniu_market_output_unit_size"] = snapshot.OutputUnitSize
	other["qiniu_market_output_unit_price"] = snapshot.OutputUnitPrice
	other["qiniu_market_output_currency"] = snapshot.OutputCurrency
	if snapshot.BillingMode == QiniuMarketBillingModeUnit {
		other["qiniu_market_unit_detail_key"] = snapshot.UnitDetailKey
		other["qiniu_market_unit_name"] = snapshot.UnitName
		other["qiniu_market_unit_size"] = snapshot.UnitSize
		other["qiniu_market_unit_price"] = snapshot.UnitPrice
		other["qiniu_market_unit_currency"] = snapshot.UnitCurrency
		other["qiniu_market_unit_quantity"] = snapshot.UnitQuantity
	}
	other["qiniu_market_amount_to_quota_rate"] = snapshot.AmountToQuotaRate
	other["qiniu_market_group_ratio"] = snapshot.GroupRatio
	other["qiniu_market_rounding_mode"] = snapshot.RoundingMode
	other["qiniu_market_catalog_state"] = snapshot.CatalogStatus
	other["qiniu_market_catalog_stale"] = snapshot.CatalogStale
	other["qiniu_market_catalog_from_cache"] = snapshot.CatalogFromCache
	other["qiniu_market_input_tokens"] = inputTokens
	other["qiniu_market_output_tokens"] = outputTokens
	other["qiniu_market_amount"] = amountValue
	other["qiniu_market_converted_quota"] = actualQuota
	if len(snapshot.InputRange) > 0 {
		other["qiniu_market_input_range"] = cloneInt64Slice(snapshot.InputRange)
	}
	if len(snapshot.OutputRange) > 0 {
		other["qiniu_market_output_range"] = cloneInt64Slice(snapshot.OutputRange)
	}
	if snapshot.CatalogLastSuccessUnix > 0 {
		other["qiniu_market_catalog_last_success"] = snapshot.CatalogLastSuccessUnix
	}
	if usageMissing {
		other["qiniu_market_usage_missing"] = true
	}
}

func InjectQiniuMarketTaskRealtimeInfo(other map[string]interface{}, snapshot *types.QiniuMarketPriceSnapshot, otherRatios map[string]float64, actualQuota int) {
	if other == nil || snapshot == nil {
		return
	}
	InjectQiniuMarketRealtimeInfo(other, snapshot, 0, 0, actualQuota, false)
	if snapshot.BillingMode != QiniuMarketBillingModeUnit {
		return
	}
	_, baseAmount := CalculateQiniuMarketQuota(snapshot, 0, 0)
	multiplier := qiniuMarketOtherMultiplier(otherRatios)
	finalAmount := baseAmount.Mul(decimal.NewFromFloat(multiplier))
	baseAmountValue, _ := baseAmount.Float64()
	finalAmountValue, _ := finalAmount.Float64()
	other["qiniu_market_base_amount"] = baseAmountValue
	other["qiniu_market_other_multiplier"] = multiplier
	other["qiniu_market_final_amount"] = finalAmountValue
	if len(otherRatios) > 0 {
		other["qiniu_market_other_ratios"] = cloneQiniuMarketOtherRatios(otherRatios)
	}
}

func qiniuMarketOtherMultiplier(otherRatios map[string]float64) float64 {
	multiplier := 1.0
	for _, ratio := range otherRatios {
		if ratio != 1.0 && ratio > 0 {
			multiplier *= ratio
		}
	}
	return multiplier
}

func cloneQiniuMarketOtherRatios(otherRatios map[string]float64) map[string]float64 {
	if len(otherRatios) == 0 {
		return nil
	}
	cloned := make(map[string]float64, len(otherRatios))
	for key, value := range otherRatios {
		cloned[key] = value
	}
	return cloned
}

func cloneInt64Slice(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]int64, len(values))
	copy(cloned, values)
	return cloned
}
