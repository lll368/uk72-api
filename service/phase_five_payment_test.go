package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyTopUpSnapshotUsesUnifiedMoneySemantics(t *testing.T) {
	epayTopUp := &model.TopUp{
		Amount:          100,
		Money:           90,
		PaymentProvider: model.PaymentProviderEpay,
	}
	ApplyTopUpSnapshot(epayTopUp)
	assert.InDelta(t, 100.0, epayTopUp.RechargeAmount, 0.000001)
	assert.InDelta(t, 90.0, epayTopUp.PaidAmount, 0.000001)
	assert.InDelta(t, 0.9, epayTopUp.Discount, 0.000001)

	creemTopUp := &model.TopUp{
		Amount:          int64(20 * common.QuotaPerUnit),
		Money:           16,
		PaymentProvider: model.PaymentProviderCreem,
	}
	ApplyTopUpSnapshot(creemTopUp)
	assert.InDelta(t, 20.0, creemTopUp.RechargeAmount, 0.000001)
	assert.InDelta(t, 16.0, creemTopUp.PaidAmount, 0.000001)
	assert.InDelta(t, 0.8, creemTopUp.Discount, 0.000001)
}

func TestPaymentCallbackAuditLifecycleStoresDigestAndStatus(t *testing.T) {
	truncate(t)

	payload := []byte(`{"trade_no":"audit-501","amount":"10"}`)
	log, err := CreatePaymentCallbackAudit(PaymentCallbackAuditInput{
		Provider:  model.PaymentProviderEpay,
		EventType: "notify",
		Payload:   payload,
	})
	require.NoError(t, err)
	require.NotNil(t, log)
	require.NotZero(t, log.Id)
	assert.Equal(t, model.PaymentProcessStatusPending, log.ProcessStatus)
	assert.NotEmpty(t, log.PayloadDigest)
	assert.NotEqual(t, string(payload), log.PayloadDigest)

	require.NoError(t, MarkPaymentCallbackAuditVerified(log, "audit-501", "notify", PaymentBizTypeTopUp))
	require.NoError(t, FinishPaymentCallbackAudit(log, model.PaymentProcessStatusSuccess, ""))

	var stored model.PaymentCallbackLog
	require.NoError(t, model.DB.First(&stored, log.Id).Error)
	assert.True(t, stored.VerifyStatus)
	assert.Equal(t, "audit-501", stored.TradeNo)
	assert.Equal(t, PaymentBizTypeTopUp, stored.BizType)
	assert.Equal(t, model.PaymentProcessStatusSuccess, stored.ProcessStatus)
	assert.Empty(t, stored.ErrorMessage)
}

func TestReverseTopUpOrderDebitsBalanceAndReversesCommissionIdempotently(t *testing.T) {
	truncate(t)

	parentDiscount := 0.8
	seedVipActivationUser(t, 701, "reverse_parent", "rp701")
	seedVipActivationUser(t, 702, "reverse_child", "rc702")
	seedActiveVvip(t, 701, "active-vvip-reverse-701")
	require.NoError(t, model.UpdateUserTopupDiscount(701, &parentDiscount))
	seedWalletRelation(t, 701, 702)

	topUp := &model.TopUp{
		UserId:          702,
		Amount:          100,
		Money:           85,
		RechargeAmount:  100,
		PaidAmount:      85,
		Discount:        0.85,
		TradeNo:         "topup-reverse-702",
		PaymentMethod:   "alipay",
		PaymentProvider: model.PaymentProviderEpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, model.DB.Create(topUp).Error)
	require.NoError(t, CompleteTopUpOrder(CompleteTopUpOrderRequest{
		TradeNo:                 topUp.TradeNo,
		ExpectedPaymentProvider: model.PaymentProviderEpay,
		ActualPaymentMethod:     "alipay",
	}))

	require.NoError(t, ReverseTopUpOrder(topUp.TradeNo, model.PaymentProviderEpay, "refund"))
	require.NoError(t, ReverseTopUpOrder(topUp.TradeNo, model.PaymentProviderEpay, "duplicate refund"))

	childAccount, err := model.GetWalletAccountByUserId(702)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, childAccount.BalanceAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 702, model.WalletFlowTypeRefundReverse, topUp.TradeNo))

	parentAccount, err := model.GetWalletAccountByUserId(701)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, parentAccount.CommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 701, model.WalletFlowTypeRefundReverse, topUp.TradeNo))

	var refreshed model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", topUp.TradeNo).First(&refreshed).Error)
	assert.Equal(t, common.TopUpStatusReversed, refreshed.Status)
}

func TestReverseVipActivationOrderDisablesVvipAndReversesCommissions(t *testing.T) {
	truncate(t)
	setVipActivationCommissionRatesForTest(t, 1000.0/model.DefaultVipActivationPaid, 0)

	seedVipActivationUser(t, 711, "reverse_vip_parent", "rvp711")
	seedVipActivationUser(t, 712, "reverse_vip_child", "rvc712")
	seedActiveVvip(t, 711, "active-vvip-reverse-711")
	seedWalletRelation(t, 711, 712)

	order, err := CreateVipActivationOrder(712, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)
	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderStripe, ""))

	require.NoError(t, ReverseVipActivationOrder(order.TradeNo, model.PaymentProviderStripe, "chargeback"))
	require.NoError(t, ReverseVipActivationOrder(order.TradeNo, model.PaymentProviderStripe, "duplicate chargeback"))

	active, err := model.IsUserActiveVvip(712)
	require.NoError(t, err)
	assert.False(t, active)

	parentAccount, err := model.GetWalletAccountByUserId(711)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, parentAccount.CommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 711, model.WalletFlowTypeRefundReverse, order.TradeNo))

	var record model.VipActivationRecord
	require.NoError(t, model.DB.Where("trade_no = ?", order.TradeNo).First(&record).Error)
	assert.Equal(t, model.VipActivationStatusDisabled, record.Status)
	assert.Contains(t, record.DisableReason, "chargeback")
}

func TestReverseVipActivationOrderReversesCommissionsAfterAdminDisable(t *testing.T) {
	truncate(t)
	setVipActivationCommissionRatesForTest(t, 1000.0/model.DefaultVipActivationPaid, 0)

	seedVipActivationUser(t, 713, "reverse_vip_disabled_parent", "rvdp713")
	seedVipActivationUser(t, 714, "reverse_vip_disabled_child", "rvdc714")
	seedActiveVvip(t, 713, "active-vvip-reverse-disabled-713")
	seedWalletRelation(t, 713, 714)

	order, err := CreateVipActivationOrder(714, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)
	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderStripe, ""))

	parentAccount, err := model.GetWalletAccountByUserId(713)
	require.NoError(t, err)
	assert.InDelta(t, 1000.0, parentAccount.CommissionAmount, 0.000001)

	require.NoError(t, DisableVipActivation(714, 1, "manual disable", "127.0.0.1"))
	require.NoError(t, ReverseVipActivationOrder(order.TradeNo, model.PaymentProviderStripe, "refund after manual disable"))

	parentAccount, err = model.GetWalletAccountByUserId(713)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, parentAccount.CommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 713, model.WalletFlowTypeRefundReverse, order.TradeNo))

	var record model.VipActivationRecord
	require.NoError(t, model.DB.Where("trade_no = ?", order.TradeNo).First(&record).Error)
	assert.Equal(t, model.VipActivationStatusDisabled, record.Status)
	assert.Contains(t, record.DisableReason, "refund after manual disable")
}

func TestReconcilePaymentOrdersDetectsLocalProviderAmountStatusAndDuplicateDiffs(t *testing.T) {
	truncate(t)
	now := time.Now().Unix()

	seedUser(t, 721, 0)
	localOrders := []model.TopUp{
		{UserId: 721, Amount: 10, Money: 10, RechargeAmount: 10, PaidAmount: 10, Discount: 1, TradeNo: "rec-amount", PaymentProvider: model.PaymentProviderEpay, CreateTime: now, Status: common.TopUpStatusSuccess},
		{UserId: 721, Amount: 20, Money: 20, RechargeAmount: 20, PaidAmount: 20, Discount: 1, TradeNo: "rec-status", PaymentProvider: model.PaymentProviderEpay, CreateTime: now, Status: common.TopUpStatusPending},
		{UserId: 721, Amount: 30, Money: 30, RechargeAmount: 30, PaidAmount: 30, Discount: 1, TradeNo: "rec-local-only", PaymentProvider: model.PaymentProviderEpay, CreateTime: now, Status: common.TopUpStatusSuccess},
		{UserId: 721, Amount: 40, Money: 40, RechargeAmount: 40, PaidAmount: 40, Discount: 1, TradeNo: "rec-duplicate", PaymentProvider: model.PaymentProviderEpay, CreateTime: now, Status: common.TopUpStatusSuccess},
	}
	for i := range localOrders {
		require.NoError(t, model.DB.Create(&localOrders[i]).Error)
	}
	for i := 0; i < 2; i++ {
		_, err := CreatePaymentCallbackAudit(PaymentCallbackAuditInput{
			Provider:  model.PaymentProviderEpay,
			EventType: "notify",
			TradeNo:   "rec-duplicate",
			BizType:   PaymentBizTypeTopUp,
			Payload:   []byte(`{"trade_no":"rec-duplicate"}`),
		})
		require.NoError(t, err)
	}
	require.NoError(t, model.DB.Model(&model.PaymentCallbackLog{}).
		Where("trade_no = ?", "rec-duplicate").
		Updates(map[string]interface{}{"verify_status": true, "process_status": model.PaymentProcessStatusSuccess}).Error)

	task, diffs, err := ReconcilePaymentOrders(ReconcilePaymentOrdersRequest{
		Provider: model.PaymentProviderEpay,
		DateFrom: now - 10,
		DateTo:   now + 10,
		Orders: []ProviderPaymentOrder{
			{TradeNo: "rec-amount", BizType: PaymentBizTypeTopUp, PaidAmount: 9, Status: common.TopUpStatusSuccess},
			{TradeNo: "rec-status", BizType: PaymentBizTypeTopUp, PaidAmount: 20, Status: common.TopUpStatusSuccess},
			{TradeNo: "rec-duplicate", BizType: PaymentBizTypeTopUp, PaidAmount: 40, Status: common.TopUpStatusSuccess},
			{TradeNo: "rec-provider-only", BizType: PaymentBizTypeTopUp, PaidAmount: 50, Status: common.TopUpStatusSuccess},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, task)

	diffTypes := make(map[string]bool)
	for _, diff := range diffs {
		diffTypes[diff.DiffType] = true
	}
	assert.True(t, diffTypes[PaymentReconcileDiffAmountMismatch])
	assert.True(t, diffTypes[PaymentReconcileDiffStatusMismatch])
	assert.True(t, diffTypes[PaymentReconcileDiffProviderMissing])
	assert.True(t, diffTypes[PaymentReconcileDiffLocalMissing])
	assert.True(t, diffTypes[PaymentReconcileDiffDuplicateCallback])
	assert.Equal(t, len(diffs), task.DiffCount)
	assert.Equal(t, model.PaymentProcessStatusSuccess, task.Status)
}

func TestReconcilePaymentOrdersUsesCompletionTimeAndIncludesSubscriptions(t *testing.T) {
	truncate(t)
	now := time.Now().Unix()
	oldCreateTime := now - 2*86400

	seedUser(t, 731, 0)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          731,
		Amount:          10,
		Money:           10,
		RechargeAmount:  10,
		PaidAmount:      10,
		Discount:        1,
		TradeNo:         "rec-complete-topup",
		PaymentProvider: model.PaymentProviderEpay,
		CreateTime:      oldCreateTime,
		CompleteTime:    now,
		Status:          common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:           731,
		TradeNo:          "rec-complete-vip",
		ActivationAmount: model.DefaultVipActivationAmount,
		PaidAmount:       model.DefaultVipActivationPaid,
		Discount:         model.DefaultVipActivationDiscount,
		PaymentProvider:  model.PaymentProviderEpay,
		Status:           model.VipActivationStatusSuccess,
		ActivatedAt:      now,
		CreatedAt:        oldCreateTime,
		UpdatedAt:        oldCreateTime,
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
		UserId:          731,
		PlanId:          1,
		Money:           30,
		TradeNo:         "rec-complete-sub",
		PaymentProvider: model.PaymentProviderEpay,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      oldCreateTime,
		CompleteTime:    now,
	}).Error)

	task, diffs, err := ReconcilePaymentOrders(ReconcilePaymentOrdersRequest{
		Provider: model.PaymentProviderEpay,
		DateFrom: now - 10,
		DateTo:   now + 10,
		Orders: []ProviderPaymentOrder{
			{TradeNo: "rec-complete-topup", BizType: PaymentBizTypeTopUp, PaidAmount: 10, Status: common.TopUpStatusSuccess},
			{TradeNo: "rec-complete-vip", BizType: PaymentBizTypeVipActivation, PaidAmount: model.DefaultVipActivationPaid, Status: model.VipActivationStatusSuccess},
			{TradeNo: "rec-complete-sub", BizType: PaymentBizTypeSubscription, PaidAmount: 30, Status: common.TopUpStatusSuccess},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Empty(t, diffs)
	assert.Zero(t, task.DiffCount)
}

func TestReconcilePaymentOrdersUsesReverseTimeForRefundPeriod(t *testing.T) {
	truncate(t)
	now := time.Now().Unix()
	oldTime := now - 2*86400

	seedUser(t, 732, 0)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          732,
		Amount:          10,
		Money:           10,
		RechargeAmount:  10,
		PaidAmount:      10,
		Discount:        1,
		TradeNo:         "rec-reversed-topup",
		PaymentProvider: model.PaymentProviderEpay,
		CreateTime:      oldTime,
		CompleteTime:    oldTime,
		ReversedAt:      now,
		Status:          common.TopUpStatusReversed,
	}).Error)
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:           732,
		TradeNo:          "rec-disabled-vip",
		ActivationAmount: model.DefaultVipActivationAmount,
		PaidAmount:       model.DefaultVipActivationPaid,
		Discount:         model.DefaultVipActivationDiscount,
		PaymentProvider:  model.PaymentProviderEpay,
		Status:           model.VipActivationStatusDisabled,
		ActivatedAt:      oldTime,
		DisabledAt:       now,
		CreatedAt:        oldTime,
		UpdatedAt:        oldTime,
	}).Error)

	task, diffs, err := ReconcilePaymentOrders(ReconcilePaymentOrdersRequest{
		Provider: model.PaymentProviderEpay,
		DateFrom: now - 10,
		DateTo:   now + 10,
		Orders: []ProviderPaymentOrder{
			{TradeNo: "rec-reversed-topup", BizType: PaymentBizTypeTopUp, PaidAmount: 10, Status: common.TopUpStatusReversed},
			{TradeNo: "rec-disabled-vip", BizType: PaymentBizTypeVipActivation, PaidAmount: model.DefaultVipActivationPaid, Status: model.VipActivationStatusDisabled},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Empty(t, diffs)
	assert.Zero(t, task.DiffCount)
}

func TestReconcilePaymentOrdersDetectsDuplicateCallbacksForProviderMissingOrders(t *testing.T) {
	truncate(t)
	now := time.Now().Unix()

	seedUser(t, 733, 0)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          733,
		Amount:          15,
		Money:           15,
		RechargeAmount:  15,
		PaidAmount:      15,
		Discount:        1,
		TradeNo:         "rec-provider-missing-duplicate",
		PaymentProvider: model.PaymentProviderEpay,
		CreateTime:      now,
		Status:          common.TopUpStatusSuccess,
	}).Error)
	for i := 0; i < 2; i++ {
		log, err := CreatePaymentCallbackAudit(PaymentCallbackAuditInput{
			Provider:  model.PaymentProviderEpay,
			EventType: "notify",
			TradeNo:   "rec-provider-missing-duplicate",
			BizType:   PaymentBizTypeTopUp,
			Payload:   []byte(`{"trade_no":"rec-provider-missing-duplicate"}`),
		})
		require.NoError(t, err)
		require.NoError(t, MarkPaymentCallbackAuditVerified(log, "rec-provider-missing-duplicate", "notify", PaymentBizTypeTopUp))
		require.NoError(t, FinishPaymentCallbackAudit(log, model.PaymentProcessStatusSuccess, ""))
	}

	_, diffs, err := ReconcilePaymentOrders(ReconcilePaymentOrdersRequest{
		Provider: model.PaymentProviderEpay,
		DateFrom: now - 10,
		DateTo:   now + 10,
		Orders:   nil,
	})
	require.NoError(t, err)

	diffTypes := make(map[string]bool)
	for _, diff := range diffs {
		diffTypes[diff.DiffType] = true
	}
	assert.True(t, diffTypes[PaymentReconcileDiffProviderMissing])
	assert.True(t, diffTypes[PaymentReconcileDiffDuplicateCallback])
}
