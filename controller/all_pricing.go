package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// GetAllPricing 返回七牛海外模型市场列表，并保持与 /api/pricing 一致的响应结构。
func GetAllPricing(c *gin.Context) {
	group := ""
	usableGroup := service.GetUserUsableGroups(group)
	groupRatio := buildPublicPricingGroupRatio(usableGroup)
	models, err := service.FetchQiniuOverseasMarketModels(c.Request.Context())
	if err != nil {
		renderPublicPricingResponse(c, false, err.Error(), []publicPricingModel{}, group, usableGroup, groupRatio, service.QiniuMarketCatalogSnapshot{
			Status: service.QiniuMarketCatalogStatusStale,
			Stale:  true,
		})
		return
	}

	renderPublicPricingResponse(c, true, "", buildPublicPricingModelsFromQiniuMarket(models), group, usableGroup, groupRatio, service.QiniuMarketCatalogSnapshot{
		Status: service.QiniuMarketCatalogStatusFresh,
		Models: models,
	})
}

func buildPublicPricingGroupRatio(usableGroup map[string]string) map[string]float64 {
	groupRatio := map[string]float64{}
	for group, ratio := range ratio_setting.GetGroupRatioCopy() {
		if _, ok := usableGroup[group]; ok {
			groupRatio[group] = ratio
		}
	}
	return groupRatio
}

func buildPublicPricingModelsFromQiniuMarket(models []dto.QiniuMarketModel) []publicPricingModel {
	if len(models) == 0 {
		return []publicPricingModel{}
	}
	pricing := make([]model.Pricing, 0, len(models))
	for _, item := range models {
		modelName := strings.TrimSpace(item.ID)
		if modelName == "" {
			continue
		}
		pricing = append(pricing, model.Pricing{
			ModelName:              modelName,
			OwnerBy:                strings.TrimSpace(item.Issuer.Name),
			EnableGroup:            []string{"all"},
			SupportedEndpointTypes: qiniuMarketSupportedEndpointTypes(item.SupportAPIProtocols),
		})
	}
	pricing = service.ApplyQiniuMarketCatalogToPricing(pricing, service.QiniuMarketCatalogSnapshot{
		Status: service.QiniuMarketCatalogStatusFresh,
		Models: models,
		Strict: true,
	})
	return buildPublicPricingModels(pricing)
}

func qiniuMarketSupportedEndpointTypes(protocols []string) []constant.EndpointType {
	endpoints := make([]constant.EndpointType, 0, len(protocols))
	seen := make(map[constant.EndpointType]struct{})
	for _, protocol := range protocols {
		endpoint := constant.EndpointType(strings.ToLower(strings.TrimSpace(protocol)))
		if endpoint == "" {
			continue
		}
		if _, ok := common.GetDefaultEndpointInfo(endpoint); !ok {
			continue
		}
		if _, ok := seen[endpoint]; ok {
			continue
		}
		seen[endpoint] = struct{}{}
		endpoints = append(endpoints, endpoint)
	}
	if len(endpoints) == 0 {
		return []constant.EndpointType{constant.EndpointTypeOpenAI}
	}
	return endpoints
}
