package dto

// QiniuMarketModel 是七牛模型市场返回给模型广场的受控字段视图。
type QiniuMarketModel struct {
	ID                  string                      `json:"id"`
	Name                string                      `json:"name"`
	Description         string                      `json:"description,omitempty"`
	CreatedTime         string                      `json:"created_time,omitempty"`
	Avatar              string                      `json:"avatar,omitempty"`
	HotTags             []string                    `json:"hot_tags,omitempty"`
	Features            []string                    `json:"features,omitempty"`
	Private             bool                        `json:"private"`
	ModelConstraints    QiniuMarketModelConstraints `json:"model_constraints,omitempty"`
	Issuer              QiniuMarketIssuer           `json:"issuer,omitempty"`
	Architecture        QiniuMarketArchitecture     `json:"architecture,omitempty"`
	PricingRulesV2      []QiniuMarketPricingRuleV2  `json:"pricing_rules_v2,omitempty"`
	RateLimit           map[string]QiniuMarketLimit `json:"rate_limit,omitempty"`
	ModelFiling         QiniuMarketModelFiling      `json:"model_filing,omitempty"`
	SupportedParameters []string                    `json:"supported_parameters,omitempty"`
	SupportAPIProtocols []string                    `json:"support_api_protocols,omitempty"`
	Rank                int                         `json:"rank,omitempty"`
	RetirementAt        string                      `json:"retirement_at,omitempty"`
	ReleaseAt           string                      `json:"release_at,omitempty"`
	SuggestedModel      string                      `json:"suggested_model,omitempty"`
}

type QiniuMarketModelConstraints struct {
	ContextLength              int64 `json:"context_length,omitempty"`
	MaxCompletionTokens        int64 `json:"max_completion_tokens,omitempty"`
	MaxTokens                  int64 `json:"max_tokens,omitempty"`
	MaxDefaultCompletionTokens int64 `json:"max_default_completion_tokens,omitempty"`
	MaxChainOfThoughtLength    int64 `json:"max_chain_of_thought_length,omitempty"`
}

type QiniuMarketIssuer struct {
	Name      string `json:"name,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
	ModelPage string `json:"model_page,omitempty"`
}

type QiniuMarketArchitecture struct {
	InputModalities  []string                    `json:"input_modalities,omitempty"`
	OutputModalities []string                    `json:"output_modalities,omitempty"`
	SchemaOutput     QiniuMarketSupportedFeature `json:"schema_output,omitempty"`
	FunctionCalling  QiniuMarketSupportedFeature `json:"function_calling,omitempty"`
	Reasoning        QiniuMarketSupportedFeature `json:"reasoning,omitempty"`
	ContentCache     QiniuMarketSupportedFeature `json:"content_cache,omitempty"`
}

type QiniuMarketSupportedFeature struct {
	Supported   bool   `json:"supported"`
	Description string `json:"description,omitempty"`
}

type QiniuMarketPricingRuleV2 struct {
	InputRange     []int64                             `json:"input_range,omitempty"`
	OutputRange    []int64                             `json:"output_range,omitempty"`
	InputItemType  string                              `json:"input_item_type,omitempty"`
	OutputItemType string                              `json:"output_item_type,omitempty"`
	Details        map[string]any                      `json:"details,omitempty"`
	DetailsV2      map[string]QiniuMarketPricingDetail `json:"details_v2,omitempty"`
}

type QiniuMarketPricingDetail struct {
	UnitName     string   `json:"unit_name,omitempty"`
	UnitSize     int64    `json:"unit_size,omitempty"`
	UnitPrice    *float64 `json:"unit_price,omitempty"`
	UnitPriceUSD *float64 `json:"unit_price_usd,omitempty"`
	Name         string   `json:"name,omitempty"`
}

type QiniuMarketLimit struct {
	Name     string  `json:"name,omitempty"`
	Quantity float64 `json:"quantity,omitempty"`
	UnitName string  `json:"unit_name,omitempty"`
	UnitTime int64   `json:"unit_time,omitempty"`
}

type QiniuMarketModelFiling struct {
	FilingNo string `json:"filing_no,omitempty"`
}
