package types

import "fmt"

type GroupRatioInfo struct {
	GroupRatio        float64
	GroupSpecialRatio float64
	HasSpecialRatio   bool
}

type PriceData struct {
	FreeModel            bool
	ModelPrice           float64
	ModelRatio           float64
	CompletionRatio      float64
	CacheRatio           float64
	CacheCreationRatio   float64
	CacheCreation5mRatio float64
	CacheCreation1hRatio float64
	ImageRatio           float64
	AudioRatio           float64
	AudioCompletionRatio float64
	OtherRatios          map[string]float64
	UsePrice             bool
	Quota                int // 按次计费的最终额度（MJ / Task）
	QuotaToPreConsume    int // 按量计费的预消耗额度
	GroupRatioInfo       GroupRatioInfo
	QiniuMarket          *QiniuMarketPriceSnapshot
}

func (p *PriceData) AddOtherRatio(key string, ratio float64) {
	if p.OtherRatios == nil {
		p.OtherRatios = make(map[string]float64)
	}
	if ratio <= 0 {
		return
	}
	p.OtherRatios[key] = ratio
}

func (p *PriceData) ToSetting() string {
	qiniuMarketModel := ""
	if p.QiniuMarket != nil {
		qiniuMarketModel = p.QiniuMarket.MarketModelID
	}
	return fmt.Sprintf("ModelPrice: %f, ModelRatio: %f, CompletionRatio: %f, CacheRatio: %f, GroupRatio: %f, UsePrice: %t, CacheCreationRatio: %f, CacheCreation5mRatio: %f, CacheCreation1hRatio: %f, QuotaToPreConsume: %d, ImageRatio: %f, AudioRatio: %f, AudioCompletionRatio: %f, QiniuMarketModel: %s", p.ModelPrice, p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.GroupRatioInfo.GroupRatio, p.UsePrice, p.CacheCreationRatio, p.CacheCreation5mRatio, p.CacheCreation1hRatio, p.QuotaToPreConsume, p.ImageRatio, p.AudioRatio, p.AudioCompletionRatio, qiniuMarketModel)
}

type QiniuMarketPriceSnapshot struct {
	PriceSource            string  `json:"price_source"`
	BillingSource          string  `json:"billing_source"`
	BillingMode            string  `json:"billing_mode,omitempty"`
	MarketModelID          string  `json:"market_model_id"`
	RuleIndex              int     `json:"rule_index"`
	InputRange             []int64 `json:"input_range,omitempty"`
	OutputRange            []int64 `json:"output_range,omitempty"`
	InputUnitName          string  `json:"input_unit_name"`
	InputUnitSize          int64   `json:"input_unit_size"`
	InputUnitPrice         float64 `json:"input_unit_price"`
	InputCurrency          string  `json:"input_currency"`
	OutputUnitName         string  `json:"output_unit_name"`
	OutputUnitSize         int64   `json:"output_unit_size"`
	OutputUnitPrice        float64 `json:"output_unit_price"`
	OutputCurrency         string  `json:"output_currency"`
	UnitDetailKey          string  `json:"unit_detail_key,omitempty"`
	UnitName               string  `json:"unit_name,omitempty"`
	UnitSize               int64   `json:"unit_size,omitempty"`
	UnitPrice              float64 `json:"unit_price,omitempty"`
	UnitCurrency           string  `json:"unit_currency,omitempty"`
	UnitQuantity           float64 `json:"unit_quantity,omitempty"`
	AmountToQuotaRate      float64 `json:"amount_to_quota_rate"`
	GroupRatio             float64 `json:"group_ratio"`
	RoundingMode           string  `json:"rounding_mode"`
	CatalogStatus          string  `json:"catalog_status"`
	CatalogStale           bool    `json:"catalog_stale,omitempty"`
	CatalogFromCache       bool    `json:"catalog_from_cache,omitempty"`
	CatalogLastSuccessUnix int64   `json:"catalog_last_success_unix,omitempty"`
	EstimatedInputTokens   int     `json:"estimated_input_tokens"`
	EstimatedOutputTokens  int     `json:"estimated_output_tokens"`
	EstimatedAmount        float64 `json:"estimated_amount"`
	ConvertedQuota         int     `json:"converted_quota"`
}
