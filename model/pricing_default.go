package model

import (
	"strings"
)

type defaultVendorRule struct {
	Pattern    string
	VendorName string
}

// 简化的供应商映射规则，使用有序列表避免 map 遍历顺序导致同一模型命中不稳定
var defaultVendorRules = []defaultVendorRule{
	{Pattern: "gpt", VendorName: "OpenAI"},
	{Pattern: "dall-e", VendorName: "OpenAI"},
	{Pattern: "whisper", VendorName: "OpenAI"},
	{Pattern: "o1", VendorName: "OpenAI"},
	{Pattern: "o3", VendorName: "OpenAI"},
	{Pattern: "claude", VendorName: "Anthropic"},
	{Pattern: "gemini", VendorName: "Google"},
	{Pattern: "moonshot", VendorName: "Moonshot"},
	{Pattern: "kimi", VendorName: "Moonshot"},
	{Pattern: "chatglm", VendorName: "智谱"},
	{Pattern: "glm-", VendorName: "智谱"},
	{Pattern: "qwen", VendorName: "阿里巴巴"},
	{Pattern: "deepseek", VendorName: "DeepSeek"},
	{Pattern: "abab", VendorName: "MiniMax"},
	{Pattern: "ernie", VendorName: "百度"},
	{Pattern: "spark", VendorName: "讯飞"},
	{Pattern: "hunyuan", VendorName: "腾讯"},
	{Pattern: "command", VendorName: "Cohere"},
	{Pattern: "@cf/", VendorName: "Cloudflare"},
	{Pattern: "360", VendorName: "360"},
	{Pattern: "yi", VendorName: "零一万物"},
	{Pattern: "jina", VendorName: "Jina"},
	{Pattern: "mistral", VendorName: "Mistral"},
	{Pattern: "grok", VendorName: "xAI"},
	{Pattern: "llama", VendorName: "Meta"},
	{Pattern: "doubao", VendorName: "字节跳动"},
	{Pattern: "kling", VendorName: "快手"},
	{Pattern: "jimeng", VendorName: "即梦"},
	{Pattern: "vidu", VendorName: "Vidu"},
}

// 供应商默认图标映射
var defaultVendorIcons = map[string]string{
	"OpenAI":     "OpenAI",
	"Anthropic":  "Claude.Color",
	"Google":     "Gemini.Color",
	"Moonshot":   "Moonshot",
	"智谱":         "Zhipu.Color",
	"阿里巴巴":       "Qwen.Color",
	"DeepSeek":   "DeepSeek.Color",
	"MiniMax":    "Minimax.Color",
	"百度":         "Wenxin.Color",
	"讯飞":         "Spark.Color",
	"腾讯":         "Hunyuan.Color",
	"Cohere":     "Cohere.Color",
	"Cloudflare": "Cloudflare.Color",
	"360":        "Ai360.Color",
	"零一万物":       "Yi.Color",
	"Jina":       "Jina",
	"Mistral":    "Mistral.Color",
	"xAI":        "XAI",
	"Meta":       "Ollama",
	"字节跳动":       "Doubao.Color",
	"快手":         "Kling.Color",
	"即梦":         "Jimeng.Color",
	"Vidu":       "Vidu",
	"微软":         "AzureAI",
	"Microsoft":  "AzureAI",
	"Azure":      "AzureAI",
}

// initDefaultVendorMapping 简化的默认供应商映射
func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	for _, ability := range enableAbilities {
		modelName := ability.Model
		if _, exists := metaMap[modelName]; exists {
			continue
		}

		vendorID := inferDefaultVendorIDByModelName(modelName, vendorMap)

		// 创建模型元数据
		metaMap[modelName] = &Model{
			ModelName: modelName,
			VendorID:  vendorID,
			Status:    1,
			NameRule:  NameRuleExact,
		}
	}
}

// inferDefaultVendorIDByModelName 只根据模型名推断默认供应商；pricing 运行时复用它补齐 logo 时不会回写模型元数据
func inferDefaultVendorIDByModelName(modelName string, vendorMap map[int]*Vendor) int {
	modelLower := strings.ToLower(strings.TrimSpace(modelName))
	if modelLower == "" {
		return 0
	}
	for _, rule := range defaultVendorRules {
		if strings.Contains(modelLower, rule.Pattern) {
			return getOrCreateVendor(rule.VendorName, vendorMap)
		}
	}
	return 0
}

// 查找或创建供应商
func getOrCreateVendor(vendorName string, vendorMap map[int]*Vendor) int {
	// 查找现有供应商
	for id, vendor := range vendorMap {
		if vendor.Name == vendorName {
			return id
		}
	}

	// 创建新供应商
	newVendor := &Vendor{
		Name:   vendorName,
		Status: 1,
		Icon:   getDefaultVendorIcon(vendorName),
	}

	if err := newVendor.Insert(); err != nil {
		return 0
	}

	vendorMap[newVendor.Id] = newVendor
	return newVendor.Id
}

// 获取供应商默认图标
func getDefaultVendorIcon(vendorName string) string {
	if icon, exists := defaultVendorIcons[vendorName]; exists {
		return icon
	}
	return ""
}
