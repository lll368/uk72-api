package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

// RefundMidjourneyFailureQuota 处理 Midjourney 最终失败后的退款。
// 官方账单只记录观测日志；市场价实时扣费需要按提交时的真实资金源退款。
func RefundMidjourneyFailureQuota(ctx context.Context, task *model.Midjourney, reason string) {
	if task == nil {
		return
	}
	if task.BillingSource == qiniuOfficialLedgerBillingSource {
		recordQiniuOfficialLedgerMidjourneyRefund(ctx, task, reason)
		return
	}
	if task.Quota == 0 {
		return
	}
	if task.BillingSource == QiniuMarketRealtimeBillingSource {
		refundQiniuMarketMidjourneyQuota(ctx, task, reason)
		return
	}
	refundLegacyMidjourneyQuota(ctx, task, reason)
}

func recordQiniuOfficialLedgerMidjourneyRefund(ctx context.Context, task *model.Midjourney, reason string) {
	other := map[string]interface{}{
		"task_id": task.MjId,
		"reason":  reason,
	}
	MarkQiniuOfficialLedgerObservation(other, task.Quota)
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		Content:   reason,
		ChannelId: task.ChannelId,
		ModelName: CovertMjpActionToModelName(task.Action),
		Quota:     0,
		TokenId:   task.TokenId,
		Other:     other,
	})
	logger.LogInfo(ctx, fmt.Sprintf("Midjourney 任务 %s 使用官方账单，失败退款跳过本地余额调整", task.MjId))
}

func refundQiniuMarketMidjourneyQuota(ctx context.Context, task *model.Midjourney, reason string) {
	if err := refundMidjourneyFunding(task); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("Midjourney 任务 %s 退还资金来源失败: %s", task.MjId, err.Error()))
		return
	}
	refundMidjourneyTokenQuota(ctx, task)
	recordQiniuMarketMidjourneyRefundLog(task, reason)
}

func refundMidjourneyFunding(task *model.Midjourney) error {
	fundingSource := strings.TrimSpace(task.FundingSource)
	if fundingSource == "" {
		// 历史测试数据没有 funding_source；市场价 MJ 当前默认按钱包路径兼容退款。
		fundingSource = BillingSourceWallet
	}
	switch fundingSource {
	case BillingSourceSubscription:
		if task.SubscriptionId <= 0 {
			return fmt.Errorf("subscription id is missing")
		}
		return model.PostConsumeUserSubscriptionDelta(task.SubscriptionId, -int64(task.Quota))
	case BillingSourceWallet:
		// 七牛 market realtime 的 MJ 失败退款只是内部预扣 quota 回滚，不应生成用户可见的钱包退款流水。
		return refundQiniuWalletQuotaSilently(task.UserId, task.Quota)
	default:
		return fmt.Errorf("unsupported funding source: %s", fundingSource)
	}
}

func refundMidjourneyTokenQuota(ctx context.Context, task *model.Midjourney) {
	if task.TokenId <= 0 {
		return
	}
	tokenKey := resolveTokenKey(ctx, task.TokenId, task.MjId)
	if tokenKey == "" {
		return
	}
	if err := model.IncreaseTokenQuota(task.TokenId, tokenKey, task.Quota); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("Midjourney 任务 %s 退还令牌额度失败: %s", task.MjId, err.Error()))
	}
}

func recordQiniuMarketMidjourneyRefundLog(task *model.Midjourney, reason string) {
	fundingSource := strings.TrimSpace(task.FundingSource)
	if fundingSource == "" {
		fundingSource = BillingSourceWallet
	}
	other := map[string]interface{}{
		"task_id":        task.MjId,
		"reason":         reason,
		"billing_source": QiniuMarketRealtimeBillingSource,
		"funding_source": fundingSource,
		"price_source":   QiniuMarketPriceSource,
	}
	if task.SubscriptionId > 0 {
		other["subscription_id"] = task.SubscriptionId
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		Content:   "",
		ChannelId: task.ChannelId,
		ModelName: CovertMjpActionToModelName(task.Action),
		Quota:     task.Quota,
		TokenId:   task.TokenId,
		Other:     other,
	})
}

func refundLegacyMidjourneyQuota(ctx context.Context, task *model.Midjourney, reason string) {
	err := model.IncreaseUserQuota(task.UserId, task.Quota, false)
	if err != nil {
		logger.LogError(ctx, "fail to increase user quota: "+err.Error())
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		Content:   "",
		ChannelId: task.ChannelId,
		ModelName: CovertMjpActionToModelName(task.Action),
		Quota:     task.Quota,
		Other: map[string]interface{}{
			"task_id": task.MjId,
			"reason":  reason,
		},
	})
}

func midjourneyWalletBizNo(task *model.Midjourney) string {
	if task == nil || strings.TrimSpace(task.MjId) == "" {
		return "midjourney"
	}
	return "midjourney:" + strings.TrimSpace(task.MjId)
}
