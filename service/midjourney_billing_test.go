package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestRefundMidjourneyFailureQuotaQiniuMarketSubscription(t *testing.T) {
	truncate(t)

	userID := 9211
	tokenID := 9212
	subscriptionID := 9213
	preConsumed := 5000
	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "qiniu-mj-subscription-token", 15000)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", tokenID).Update("used_quota", preConsumed).Error)
	seedSubscription(t, subscriptionID, userID, 50000, int64(preConsumed))

	task := &model.Midjourney{
		UserId:         userID,
		MjId:           "mj-qiniu-market-subscription-failed",
		Action:         constant.MjActionImagine,
		ChannelId:      9101,
		Quota:          preConsumed,
		TokenId:        tokenID,
		BillingSource:  QiniuMarketRealtimeBillingSource,
		FundingSource:  BillingSourceSubscription,
		SubscriptionId: subscriptionID,
	}

	RefundMidjourneyFailureQuota(context.Background(), task, "构图失败")

	require.Equal(t, int64(0), getSubscriptionUsed(t, subscriptionID))
	var token model.Token
	require.NoError(t, model.DB.Select("remain_quota", "used_quota").First(&token, "id = ?", tokenID).Error)
	require.Equal(t, 20000, token.RemainQuota)
	require.Equal(t, 0, token.UsedQuota)

	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log, "type = ?", model.LogTypeRefund).Error)
	require.Equal(t, preConsumed, log.Quota)
	require.Equal(t, tokenID, log.TokenId)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	require.Equal(t, QiniuMarketRealtimeBillingSource, other["billing_source"])
	require.Equal(t, BillingSourceSubscription, other["funding_source"])
	require.Equal(t, float64(subscriptionID), other["subscription_id"])
}

func TestRefundMidjourneyFailureQuotaQiniuMarketWalletDoesNotCreateRefundFlow(t *testing.T) {
	truncate(t)

	userID := 9221
	tokenID := 9222
	preConsumed := 5000
	initialQuota := amountToQuota(20)
	seedUser(t, userID, initialQuota-preConsumed)
	seedWalletAccount(t, userID, quotaToWalletAmount(initialQuota-preConsumed), 0, 0)
	seedToken(t, tokenID, userID, "qiniu-mj-wallet-token", initialQuota-preConsumed)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", tokenID).Update("used_quota", preConsumed).Error)

	task := &model.Midjourney{
		UserId:        userID,
		MjId:          "mj-qiniu-market-wallet-failed",
		Action:        constant.MjActionImagine,
		ChannelId:     9102,
		Quota:         preConsumed,
		TokenId:       tokenID,
		BillingSource: QiniuMarketRealtimeBillingSource,
		FundingSource: BillingSourceWallet,
	}

	RefundMidjourneyFailureQuota(context.Background(), task, "构图失败")

	require.Equal(t, initialQuota, getUserQuota(t, userID))
	assertWalletBalance(t, userID, quotaToWalletAmount(initialQuota))
	require.Equal(t, initialQuota, getTokenRemainQuota(t, tokenID))
	require.Equal(t, 0, getTokenUsedQuota(t, tokenID))
	require.Equal(t, int64(0), countWalletFlows(t, userID, model.WalletFlowTypeBalanceRefund, midjourneyWalletBizNo(task)))

	var flowCount int64
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).Where("user_id = ? AND biz_no LIKE ?", userID, "midjourney:%").Count(&flowCount).Error)
	require.Equal(t, int64(0), flowCount)
}
