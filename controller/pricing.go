package controller

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

type publicPricingModel struct {
	ModelName              string                  `json:"model_name"`
	Description            string                  `json:"description,omitempty"`
	Icon                   string                  `json:"icon,omitempty"`
	Tags                   string                  `json:"tags,omitempty"`
	VendorID               int                     `json:"vendor_id,omitempty"`
	QuotaType              int                     `json:"quota_type"`
	ModelRatio             float64                 `json:"model_ratio"`
	ModelPrice             float64                 `json:"model_price"`
	OwnerBy                string                  `json:"owner_by"`
	CompletionRatio        float64                 `json:"completion_ratio"`
	CacheRatio             *float64                `json:"cache_ratio,omitempty"`
	CreateCacheRatio       *float64                `json:"create_cache_ratio,omitempty"`
	ImageRatio             *float64                `json:"image_ratio,omitempty"`
	AudioRatio             *float64                `json:"audio_ratio,omitempty"`
	AudioCompletionRatio   *float64                `json:"audio_completion_ratio,omitempty"`
	EnableGroup            []string                `json:"enable_groups"`
	SupportedEndpointTypes []constant.EndpointType `json:"supported_endpoint_types"`
	BillingMode            string                  `json:"billing_mode,omitempty"`
	BillingExpr            string                  `json:"billing_expr,omitempty"`
	PricingVersion         string                  `json:"pricing_version,omitempty"`
	Enabled                bool                    `json:"enabled,omitempty"`
	Routable               bool                    `json:"routable,omitempty"`
	PriceSourceLabel       string                  `json:"price_source_label,omitempty"`
	MarketPricing          *publicMarketPricing    `json:"market_pricing,omitempty"`
	ContextLength          int64                   `json:"context_length,omitempty"`
	MaxOutputTokens        int64                   `json:"max_output_tokens,omitempty"`
	ReleaseDate            string                  `json:"release_date,omitempty"`
	InputModalities        []string                `json:"input_modalities,omitempty"`
	OutputModalities       []string                `json:"output_modalities,omitempty"`
	Capabilities           []string                `json:"capabilities,omitempty"`
}

type publicMarketPricing struct {
	ID             string                         `json:"id,omitempty"`
	Name           string                         `json:"name,omitempty"`
	PricingRulesV2 []dto.QiniuMarketPricingRuleV2 `json:"pricing_rules_v2,omitempty"`
}

var (
	publicQiniuMarketSnakePattern = regexp.MustCompile(`(?i)qiniu_market`)
	publicQiniuMarketWordsPattern = regexp.MustCompile(`(?i)qiniu\s+market`)
	publicQiniuBrandPattern       = regexp.MustCompile(`(?i)qiniu`)
)

const publicPricingVersion = "a42d372ccf0b5dd13ecf71203521f9d2"

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			item.EnableGroup = []string{"all"}
			filtered = append(filtered, item)
			continue
		}
		allowedGroups := make([]string, 0, len(item.EnableGroup))
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				allowedGroups = append(allowedGroups, group)
			}
		}
		if len(allowedGroups) == 0 {
			continue
		}
		item.EnableGroup = allowedGroups
		filtered = append(filtered, item)
	}
	return filtered
}

func buildPublicPricingModels(pricing []model.Pricing) []publicPricingModel {
	if len(pricing) == 0 {
		return []publicPricingModel{}
	}
	publicModels := make([]publicPricingModel, 0, len(pricing))
	for _, item := range pricing {
		publicModel := publicPricingModel{
			ModelName:              item.ModelName,
			Description:            sanitizePublicSupplierText(item.Description),
			Icon:                   sanitizePublicSupplierURL(item.Icon),
			Tags:                   sanitizePublicSupplierText(item.Tags),
			VendorID:               item.VendorID,
			QuotaType:              item.QuotaType,
			ModelRatio:             item.ModelRatio,
			ModelPrice:             item.ModelPrice,
			OwnerBy:                item.OwnerBy,
			CompletionRatio:        item.CompletionRatio,
			CacheRatio:             item.CacheRatio,
			CreateCacheRatio:       item.CreateCacheRatio,
			ImageRatio:             item.ImageRatio,
			AudioRatio:             item.AudioRatio,
			AudioCompletionRatio:   item.AudioCompletionRatio,
			EnableGroup:            append([]string(nil), item.EnableGroup...),
			SupportedEndpointTypes: append([]constant.EndpointType(nil), item.SupportedEndpointTypes...),
			BillingMode:            item.BillingMode,
			BillingExpr:            item.BillingExpr,
			PricingVersion:         item.PricingVersion,
			Enabled:                item.Enabled,
			Routable:               item.Routable,
			PriceSourceLabel:       sanitizePublicSupplierText(item.PriceSourceLabel),
			ContextLength:          item.ContextLength,
			MaxOutputTokens:        item.MaxOutputTokens,
			ReleaseDate:            item.ReleaseDate,
			InputModalities:        append([]string(nil), item.InputModalities...),
			OutputModalities:       append([]string(nil), item.OutputModalities...),
			Capabilities:           append([]string(nil), item.Capabilities...),
		}
		if item.QiniuMarket != nil {
			// 普通用户端只需要通用价格规则；供应商字段和值继续留在内部结构用于账务和排障。
			publicModel.MarketPricing = &publicMarketPricing{
				ID:             sanitizePublicSupplierText(item.QiniuMarket.ID),
				Name:           sanitizePublicSupplierText(item.QiniuMarket.Name),
				PricingRulesV2: buildPublicMarketPricingRules(item.QiniuMarket.PricingRulesV2),
			}
		}
		publicModels = append(publicModels, publicModel)
	}
	return publicModels
}

func buildPublicPricingVendors(vendors []model.PricingVendor) []model.PricingVendor {
	if len(vendors) == 0 {
		return []model.PricingVendor{}
	}
	publicVendors := make([]model.PricingVendor, 0, len(vendors))
	for _, vendor := range vendors {
		vendor.Name = sanitizePublicSupplierText(vendor.Name)
		vendor.Description = sanitizePublicSupplierText(vendor.Description)
		vendor.Icon = sanitizePublicSupplierURL(vendor.Icon)
		publicVendors = append(publicVendors, vendor)
	}
	return publicVendors
}

func buildPublicMarketPricingRules(rules []dto.QiniuMarketPricingRuleV2) []dto.QiniuMarketPricingRuleV2 {
	if len(rules) == 0 {
		return nil
	}
	publicRules := make([]dto.QiniuMarketPricingRuleV2, 0, len(rules))
	for _, rule := range rules {
		publicRule := dto.QiniuMarketPricingRuleV2{
			InputRange:     append([]int64(nil), rule.InputRange...),
			OutputRange:    append([]int64(nil), rule.OutputRange...),
			InputItemType:  sanitizePublicSupplierText(rule.InputItemType),
			OutputItemType: sanitizePublicSupplierText(rule.OutputItemType),
		}
		if len(rule.DetailsV2) > 0 {
			publicRule.DetailsV2 = make(map[string]dto.QiniuMarketPricingDetail, len(rule.DetailsV2))
			index := 0
			for key, detail := range rule.DetailsV2 {
				detail.Name = sanitizePublicSupplierText(detail.Name)
				detail.UnitName = sanitizePublicSupplierText(detail.UnitName)
				publicRule.DetailsV2[sanitizePublicMarketDetailKey(key, index)] = detail
				index++
			}
		}
		publicRules = append(publicRules, publicRule)
	}
	return publicRules
}

func sanitizePublicMarketDetailKey(key string, index int) string {
	if containsSupplierBrand(key) {
		return "market_detail_" + strconv.Itoa(index+1)
	}
	return key
}

func sanitizePublicSupplierText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = publicQiniuMarketSnakePattern.ReplaceAllString(value, "market")
	value = publicQiniuMarketWordsPattern.ReplaceAllString(value, "Official Market")
	value = publicQiniuBrandPattern.ReplaceAllString(value, "Official")
	value = strings.ReplaceAll(value, "七牛", "官方")
	value = strings.ReplaceAll(value, "官方官方", "官方")
	value = strings.ReplaceAll(value, "Official Official", "Official")
	value = strings.Join(strings.Fields(value), " ")
	return strings.Trim(value, " ,;|")
}

func sanitizePublicSupplierURL(value string) string {
	// 隐藏供应商品牌只针对用户可见文案；图片和链接属于资源地址，不能因为包含供应商域名而清空，否则会导致模型 logo 丢失。
	return strings.TrimSpace(value)
}

func containsSupplierBrand(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "qiniu") || strings.Contains(value, "七牛")
}

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	for s, f := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[s] = f
	}
	var group string
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
			for g := range groupRatio {
				ratio, ok := ratio_setting.GetGroupGroupRatio(group, g)
				if ok {
					groupRatio[g] = ratio
				}
			}
		}
	}

	usableGroup = service.GetUserUsableGroups(group)
	pricing = filterPricingByUsableGroups(pricing, usableGroup)
	qiniuMarketSnapshot := service.GetQiniuMarketCatalogSnapshot(c.Request.Context())
	pricing = service.ApplyQiniuMarketCatalogToPricing(pricing, qiniuMarketSnapshot)
	// check groupRatio contains usableGroup
	for group := range ratio_setting.GetGroupRatioCopy() {
		if _, ok := usableGroup[group]; !ok {
			delete(groupRatio, group)
		}
	}

	publicPricing := buildPublicPricingModels(pricing)
	renderPublicPricingResponse(c, true, "", publicPricing, group, usableGroup, groupRatio, qiniuMarketSnapshot)
}

func renderPublicPricingResponse(c *gin.Context, success bool, message string, data []publicPricingModel, group string, usableGroup map[string]string, groupRatio map[string]float64, qiniuMarketSnapshot service.QiniuMarketCatalogSnapshot) {
	payload := gin.H{
		"success":             success,
		"data":                data,
		"vendors":             buildPublicPricingVendors(model.GetVendors()),
		"group_ratio":         groupRatio,
		"usable_group":        usableGroup,
		"supported_endpoint":  model.GetSupportedEndpointMap(),
		"auto_groups":         service.GetUserAutoGroup(group),
		"market_pricing_sync": service.QiniuMarketCatalogPublicState(qiniuMarketSnapshot),
		"pricing_version":     publicPricingVersion,
	}
	if message != "" {
		payload["message"] = message
	}
	c.JSON(200, payload)
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
