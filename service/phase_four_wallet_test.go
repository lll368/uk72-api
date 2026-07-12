package service

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedWalletRelation(t *testing.T, parentUserId int, childUserId int) {
	t.Helper()
	_, err := model.CreateActiveUserRelationTx(model.DB, parentUserId, childUserId, model.UserRelationSourceRegister, "")
	require.NoError(t, err)
}

func seedWalletAccount(t *testing.T, userId int, balanceAmount float64, commissionAmount float64, frozenCommissionAmount float64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.WalletAccount{
		UserId:                 userId,
		BalanceAmount:          balanceAmount,
		CommissionAmount:       commissionAmount,
		FrozenCommissionAmount: frozenCommissionAmount,
		TotalCommissionAmount:  commissionAmount,
	}).Error)
}

func countWalletFlows(t *testing.T, userId int, flowType string, bizNo string) int64 {
	t.Helper()
	var count int64
	query := model.DB.Model(&model.WalletFlow{}).Where("user_id = ? AND flow_type = ?", userId, flowType)
	if bizNo != "" {
		query = query.Where("biz_no = ?", bizNo)
	}
	require.NoError(t, query.Count(&count).Error)
	return count
}

func getWalletFlow(t *testing.T, userId int, flowType string, bizNo string) model.WalletFlow {
	t.Helper()
	var flow model.WalletFlow
	require.NoError(t, model.DB.
		Where("user_id = ? AND flow_type = ? AND biz_no = ?", userId, flowType, bizNo).
		First(&flow).Error)
	return flow
}

func assertWalletFlowSnapshot(t *testing.T, flow model.WalletFlow, balanceAfter float64, commissionAfter float64, frozenCommissionAfter float64) {
	t.Helper()
	assert.InDelta(t, balanceAfter, flow.BalanceAfter, 0.000001)
	assert.InDelta(t, commissionAfter, flow.CommissionAfter, 0.000001)
	assert.InDelta(t, frozenCommissionAfter, flow.FrozenCommissionAfter, 0.000001)
}

func getWalletFlowRemarks(t *testing.T, userId int, flowType string, bizNo string) []string {
	t.Helper()
	var remarks []string
	require.NoError(t, model.DB.Model(&model.WalletFlow{}).
		Where("user_id = ? AND flow_type = ? AND biz_no = ?", userId, flowType, bizNo).
		Order("id asc").
		Pluck("remark", &remarks).Error)
	return remarks
}

func getCommissionSourceUserLabels(t *testing.T, sourceOrderNo string) []string {
	t.Helper()
	var labels []string
	require.NoError(t, model.DB.Table("commission_records").
		Where("source_order_no = ?", sourceOrderNo).
		Order("level asc").
		Pluck("source_user_label", &labels).Error)
	return labels
}

func setVipActivationCommissionRatesForTest(t *testing.T, level1Rate float64, level2Rate float64) {
	t.Helper()
	setting := operation_setting.GetPaymentSetting()
	oldPrice := setting.VipActivationPrice
	oldLevel1Amount := setting.VipActivationCommissionLevel1Amount
	oldLevel2Amount := setting.VipActivationCommissionLevel2Amount
	setting.VipActivationPrice = model.DefaultVipActivationPaid
	setting.VipActivationCommissionLevel1Amount = model.DefaultVipActivationPaid * level1Rate
	setting.VipActivationCommissionLevel2Amount = model.DefaultVipActivationPaid * level2Rate
	t.Cleanup(func() {
		setting.VipActivationPrice = oldPrice
		setting.VipActivationCommissionLevel1Amount = oldLevel1Amount
		setting.VipActivationCommissionLevel2Amount = oldLevel2Amount
	})
}

func setVipActivationAmountConfigForTest(t *testing.T, price float64, level1Amount float64, level2Amount float64) {
	t.Helper()
	setting := operation_setting.GetPaymentSetting()
	oldPrice := setting.VipActivationPrice
	oldLevel1Amount := setting.VipActivationCommissionLevel1Amount
	oldLevel2Amount := setting.VipActivationCommissionLevel2Amount
	setting.VipActivationPrice = price
	setting.VipActivationCommissionLevel1Amount = level1Amount
	setting.VipActivationCommissionLevel2Amount = level2Amount
	t.Cleanup(func() {
		setting.VipActivationPrice = oldPrice
		setting.VipActivationCommissionLevel1Amount = oldLevel1Amount
		setting.VipActivationCommissionLevel2Amount = oldLevel2Amount
	})
}

func TestCompleteTopUpOrderCreditsWalletAndQuotaIdempotently(t *testing.T) {
	truncate(t)

	seedUser(t, 401, 0)
	topUp := &model.TopUp{
		UserId:          401,
		Amount:          20,
		Money:           18,
		RechargeAmount:  20,
		PaidAmount:      18,
		Discount:        0.9,
		TradeNo:         "topup-wallet-401",
		PaymentMethod:   "alipay",
		PaymentProvider: model.PaymentProviderEpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, model.DB.Create(topUp).Error)

	req := CompleteTopUpOrderRequest{
		TradeNo:                 topUp.TradeNo,
		ExpectedPaymentProvider: model.PaymentProviderEpay,
		ActualPaymentMethod:     "alipay",
		CallerIP:                "127.0.0.1",
	}
	require.NoError(t, CompleteTopUpOrder(req))
	require.NoError(t, CompleteTopUpOrder(req))

	user, err := model.GetUserById(401, false)
	require.NoError(t, err)
	assert.Equal(t, int(20*common.QuotaPerUnit), user.Quota)

	account, err := model.GetWalletAccountByUserId(401)
	require.NoError(t, err)
	assert.InDelta(t, 20.0, account.BalanceAmount, 0.000001)
	assert.InDelta(t, 0.0, account.CommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 401, model.WalletFlowTypeRechargeBalance, topUp.TradeNo))
	rechargeFlow := getWalletFlow(t, 401, model.WalletFlowTypeRechargeBalance, topUp.TradeNo)
	assertWalletFlowSnapshot(t, rechargeFlow, 20.0, 0.0, 0.0)

	var refreshed model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", topUp.TradeNo).First(&refreshed).Error)
	assert.Equal(t, common.TopUpStatusSuccess, refreshed.Status)
	assert.NotZero(t, refreshed.CompleteTime)
}

func TestCompleteTopUpOrderCreatesQiniuQuotaGrant(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)
	configureQiniuKeySettingForTest(t, "http://127.0.0.1")

	keyBody := strings.Repeat("4", 64)
	seedUser(t, 404, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             404,
		UserId:         404,
		Name:           "qiniu-token",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	topUp := &model.TopUp{
		UserId:          404,
		Amount:          20,
		Money:           20,
		RechargeAmount:  20,
		PaidAmount:      20,
		Discount:        1,
		TradeNo:         "topup-qiniu-quota-sync-404",
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
		CallerIP:                "127.0.0.1",
	}))

	assertQiniuQuotaGrant(t, 404, 404, "recharge:topup-qiniu-quota-sync-404", 20, model.QiniuQuotaGrantStatusPending)
	assertQiniuQuotaSyncTaskCount(t, 404, 404, 0)
}

func TestCompleteTopUpOrderRollsBackWhenQiniuQuotaGrantRecordFails(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)
	configureQiniuKeySettingForTest(t, "http://127.0.0.1")

	keyBody := strings.Repeat("6", 64)
	seedUser(t, 405, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             405,
		UserId:         405,
		Name:           "qiniu-token",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	topUp := &model.TopUp{
		UserId:          405,
		Amount:          20,
		Money:           20,
		RechargeAmount:  20,
		PaidAmount:      20,
		Discount:        1,
		TradeNo:         "topup-qiniu-grant-fail-405",
		PaymentMethod:   "alipay",
		PaymentProvider: model.PaymentProviderEpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, model.DB.Create(topUp).Error)

	registerFailQiniuQuotaGrantCreateCallback(t)
	err := CompleteTopUpOrder(CompleteTopUpOrderRequest{
		TradeNo:                 topUp.TradeNo,
		ExpectedPaymentProvider: model.PaymentProviderEpay,
		ActualPaymentMethod:     "alipay",
		CallerIP:                "127.0.0.1",
	})
	require.Error(t, err)

	user, err := model.GetUserById(405, false)
	require.NoError(t, err)
	assert.Equal(t, 0, user.Quota)
	var accountCount int64
	require.NoError(t, model.DB.Model(&model.WalletAccount{}).Where("user_id = ?", 405).Count(&accountCount).Error)
	assert.Equal(t, int64(0), accountCount)
	assert.Equal(t, int64(0), countWalletFlows(t, 405, model.WalletFlowTypeRechargeBalance, topUp.TradeNo))
	var refreshed model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", topUp.TradeNo).First(&refreshed).Error)
	assert.Equal(t, common.TopUpStatusPending, refreshed.Status)
}

func TestCompleteTopUpOrderBackfillsLegacyQuotaBeforeCredit(t *testing.T) {
	truncate(t)

	seedUser(t, 402, int(100*common.QuotaPerUnit))
	topUp := &model.TopUp{
		UserId:          402,
		Amount:          20,
		Money:           20,
		RechargeAmount:  20,
		PaidAmount:      20,
		Discount:        1,
		TradeNo:         "topup-wallet-legacy-402",
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

	account, err := model.GetWalletAccountByUserId(402)
	require.NoError(t, err)
	assert.InDelta(t, 120.0, account.BalanceAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 402, model.WalletFlowTypeLegacyBalanceInit, "legacy-balance-init-402"))
	assert.Equal(t, int64(1), countWalletFlows(t, 402, model.WalletFlowTypeRechargeBalance, topUp.TradeNo))

	user, err := model.GetUserById(402, false)
	require.NoError(t, err)
	assert.Equal(t, int(120*common.QuotaPerUnit), user.Quota)
}

func TestTransferCommissionToBalanceBackfillsLegacyQuotaBeforeCredit(t *testing.T) {
	truncate(t)

	seedUser(t, 403, int(50*common.QuotaPerUnit))
	seedWalletAccount(t, 403, 0, 20, 0)

	require.NoError(t, TransferCommissionToBalance(403, 10))

	account, err := model.GetWalletAccountByUserId(403)
	require.NoError(t, err)
	assert.InDelta(t, 60.0, account.BalanceAmount, 0.000001)
	assert.InDelta(t, 10.0, account.CommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 403, model.WalletFlowTypeLegacyBalanceInit, "legacy-balance-init-403"))
	assert.Equal(t, int64(1), countWalletFlows(t, 403, model.WalletFlowTypeCommissionToBalance, "commission-transfer-403"))

	user, err := model.GetUserById(403, false)
	require.NoError(t, err)
	assert.Equal(t, int(60*common.QuotaPerUnit), user.Quota)
}

func TestTopUpCommissionStopsWhenDirectParentIsNotActiveVvip(t *testing.T) {
	truncate(t)

	setting := operation_setting.GetPaymentSetting()
	oldLevel1Rate := setting.TopupCommissionLevel1Rate
	oldLevel2Rate := setting.TopupCommissionLevel2Rate
	setting.TopupCommissionLevel1Rate = 0.10
	setting.TopupCommissionLevel2Rate = 0.05
	t.Cleanup(func() {
		setting.TopupCommissionLevel1Rate = oldLevel1Rate
		setting.TopupCommissionLevel2Rate = oldLevel2Rate
	})

	seedVipActivationUser(t, 411, "vvip_grand_parent_topup", "gp411")
	seedVipActivationUser(t, 412, "normal_parent_topup", "p412")
	seedVipActivationUser(t, 413, "topup_child", "c413")
	seedActiveVvip(t, 411, "active-vvip-411")
	seedWalletRelation(t, 411, 412)
	seedWalletRelation(t, 412, 413)

	topUp := &model.TopUp{
		UserId:          413,
		Amount:          100,
		Money:           100,
		RechargeAmount:  100,
		PaidAmount:      100,
		Discount:        1,
		TradeNo:         "topup-commission-vvip-filter",
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

	var records []model.CommissionRecord
	require.NoError(t, model.DB.Where("source_order_no = ?", topUp.TradeNo).Order("level asc").Find(&records).Error)
	require.Empty(t, records)

	_, err := model.GetWalletAccountByUserId(412)
	assert.Error(t, err)
	_, err = model.GetWalletAccountByUserId(411)
	assert.Error(t, err)
}

func TestCompleteTopUpOrderCreatesCommissionFromDiscountSpread(t *testing.T) {
	truncate(t)

	setting := operation_setting.GetPaymentSetting()
	oldLevel1Rate := setting.TopupCommissionLevel1Rate
	oldLevel2Rate := setting.TopupCommissionLevel2Rate
	setting.TopupCommissionLevel1Rate = 0
	setting.TopupCommissionLevel2Rate = 0
	t.Cleanup(func() {
		setting.TopupCommissionLevel1Rate = oldLevel1Rate
		setting.TopupCommissionLevel2Rate = oldLevel2Rate
	})

	parentDiscount := 0.8
	seedVipActivationUser(t, 421, "discount_parent_topup", "dp421")
	seedVipActivationUser(t, 422, "discount_child_topup", "dc422")
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 422).Update("display_name", "Topup Child").Error)
	seedActiveVvip(t, 421, "active-vvip-421")
	require.NoError(t, model.UpdateUserTopupDiscount(421, &parentDiscount))
	seedWalletRelation(t, 421, 422)

	topUp := &model.TopUp{
		UserId:          422,
		Amount:          100,
		Money:           85,
		RechargeAmount:  100,
		PaidAmount:      85,
		Discount:        0.85,
		TradeNo:         "topup-commission-discount-spread",
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
	require.NoError(t, CompleteTopUpOrder(CompleteTopUpOrderRequest{
		TradeNo:                 topUp.TradeNo,
		ExpectedPaymentProvider: model.PaymentProviderEpay,
		ActualPaymentMethod:     "alipay",
	}))

	parentAccount, err := model.GetWalletAccountByUserId(421)
	require.NoError(t, err)
	assert.InDelta(t, 4.25, parentAccount.CommissionAmount, 0.000001)
	assert.InDelta(t, 4.25, parentAccount.TotalCommissionAmount, 0.000001)

	var records []model.CommissionRecord
	require.NoError(t, model.DB.Where("source_order_no = ?", topUp.TradeNo).Order("level asc").Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, 421, records[0].BeneficiaryUserId)
	assert.Equal(t, 1, records[0].Level)
	assert.InDelta(t, 85.0, records[0].BaseAmount, 0.000001)
	assert.InDelta(t, 0.05, records[0].CommissionRate, 0.000001)
	assert.InDelta(t, 4.25, records[0].Amount, 0.000001)
	assert.Equal(t, model.CommissionStatusSettled, records[0].Status)
	assert.Equal(t, int64(1), countWalletFlows(t, 421, model.WalletFlowTypeCommissionIncome, topUp.TradeNo))
	assert.Equal(t, []string{"Topup Child/discount_child_topup (#422)"}, getCommissionSourceUserLabels(t, topUp.TradeNo))
	assert.Equal(t, []string{"充值分佣：下级用户 Topup Child/discount_child_topup (#422)"}, getWalletFlowRemarks(t, 421, model.WalletFlowTypeCommissionIncome, topUp.TradeNo))
}

func TestTopUpCommissionUsesUserDiscountWhenOrderSnapshotMissing(t *testing.T) {
	truncate(t)

	parentDiscount := 0.8
	childDiscount := 0.85
	seedVipActivationUser(t, 441, "fallback_parent_topup", "fp441")
	seedVipActivationUser(t, 442, "fallback_child_topup", "fc442")
	seedActiveVvip(t, 441, "active-vvip-441")
	require.NoError(t, model.UpdateUserTopupDiscount(441, &parentDiscount))
	require.NoError(t, model.UpdateUserTopupDiscount(442, &childDiscount))
	seedWalletRelation(t, 441, 442)

	rules, err := buildTopUpCommissionRulesTx(model.DB, &model.TopUp{
		UserId:         442,
		Amount:         100,
		Money:          85,
		RechargeAmount: 100,
		PaidAmount:     0,
		Discount:       0,
		TradeNo:        "topup-commission-user-discount-fallback",
	})

	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, 441, rules[0].beneficiary)
	assert.InDelta(t, 85.0, rules[0].baseAmount, 0.000001)
	assert.InDelta(t, 0.05, rules[0].rate, 0.000001)
	assert.InDelta(t, 4.25, rules[0].amount, 0.000001)
}

func TestTopUpCommissionDoesNotFallbackToConfiguredRates(t *testing.T) {
	truncate(t)

	setting := operation_setting.GetPaymentSetting()
	oldLevel1Rate := setting.TopupCommissionLevel1Rate
	oldLevel2Rate := setting.TopupCommissionLevel2Rate
	setting.TopupCommissionLevel1Rate = 0.10
	setting.TopupCommissionLevel2Rate = 0.05
	t.Cleanup(func() {
		setting.TopupCommissionLevel1Rate = oldLevel1Rate
		setting.TopupCommissionLevel2Rate = oldLevel2Rate
	})

	parentDiscount := 0.8
	childDiscount := 0.8
	seedVipActivationUser(t, 451, "no_fixed_parent_topup", "nfp451")
	seedVipActivationUser(t, 452, "no_fixed_child_topup", "nfc452")
	seedActiveVvip(t, 451, "active-vvip-451")
	require.NoError(t, model.UpdateUserTopupDiscount(451, &parentDiscount))
	require.NoError(t, model.UpdateUserTopupDiscount(452, &childDiscount))
	seedWalletRelation(t, 451, 452)

	rules, err := buildTopUpCommissionRulesTx(model.DB, &model.TopUp{
		UserId:         452,
		Amount:         100,
		Money:          80,
		RechargeAmount: 100,
		PaidAmount:     80,
		Discount:       0.8,
		TradeNo:        "topup-commission-no-fixed-rate-fallback",
	})

	require.NoError(t, err)
	require.Empty(t, rules)
}

func TestCompleteTopUpOrderRepairsMissingCommissionForSuccessfulOrder(t *testing.T) {
	truncate(t)

	setting := operation_setting.GetPaymentSetting()
	oldLevel1Rate := setting.TopupCommissionLevel1Rate
	oldLevel2Rate := setting.TopupCommissionLevel2Rate
	setting.TopupCommissionLevel1Rate = 0
	setting.TopupCommissionLevel2Rate = 0
	t.Cleanup(func() {
		setting.TopupCommissionLevel1Rate = oldLevel1Rate
		setting.TopupCommissionLevel2Rate = oldLevel2Rate
	})

	parentDiscount := 0.8
	seedVipActivationUser(t, 431, "repair_parent_topup", "rp431")
	seedVipActivationUser(t, 432, "repair_child_topup", "rc432")
	seedActiveVvip(t, 431, "active-vvip-431")
	require.NoError(t, model.UpdateUserTopupDiscount(431, &parentDiscount))
	seedWalletRelation(t, 431, 432)

	topUp := &model.TopUp{
		UserId:          432,
		Amount:          100,
		Money:           85,
		RechargeAmount:  100,
		PaidAmount:      85,
		Discount:        0.85,
		TradeNo:         "topup-commission-repair-success",
		PaymentMethod:   "alipay",
		PaymentProvider: model.PaymentProviderEpay,
		CreateTime:      time.Now().Unix(),
		CompleteTime:    time.Now().Unix(),
		Status:          common.TopUpStatusSuccess,
	}
	require.NoError(t, model.DB.Create(topUp).Error)

	req := CompleteTopUpOrderRequest{
		TradeNo:                 topUp.TradeNo,
		ExpectedPaymentProvider: model.PaymentProviderEpay,
		ActualPaymentMethod:     "alipay",
	}
	require.NoError(t, CompleteTopUpOrder(req))
	require.NoError(t, CompleteTopUpOrder(req))

	parentAccount, err := model.GetWalletAccountByUserId(431)
	require.NoError(t, err)
	assert.InDelta(t, 4.25, parentAccount.CommissionAmount, 0.000001)
	assert.InDelta(t, 4.25, parentAccount.TotalCommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 431, model.WalletFlowTypeCommissionIncome, topUp.TradeNo))

	var records []model.CommissionRecord
	require.NoError(t, model.DB.Where("source_order_no = ?", topUp.TradeNo).Find(&records).Error)
	require.Len(t, records, 1)
	assert.InDelta(t, 4.25, records[0].Amount, 0.000001)
}

func TestCompleteVipActivationOrderCreatesConfiguredCommissionsIdempotently(t *testing.T) {
	truncate(t)

	setVipActivationCommissionRatesForTest(t, 0.20, 0.10)

	seedVipActivationUser(t, 501, "vvip_grand_parent", "gp501")
	seedVipActivationUser(t, 502, "vvip_parent", "p502")
	seedVipActivationUser(t, 503, "vvip_child", "c503")
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 503).Update("display_name", "VVIP Child").Error)
	seedActiveVvip(t, 501, "active-vvip-501")
	seedActiveVvip(t, 502, "active-vvip-502")
	seedWalletRelation(t, 501, 502)
	seedWalletRelation(t, 502, 503)

	order, err := CreateVipActivationOrder(503, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderStripe, ""))
	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid-again"}`, model.PaymentProviderStripe, ""))

	parentAccount, err := model.GetWalletAccountByUserId(502)
	require.NoError(t, err)
	assert.InDelta(t, 336.0, parentAccount.CommissionAmount, 0.000001)
	assert.InDelta(t, 336.0, parentAccount.TotalCommissionAmount, 0.000001)

	grandAccount, err := model.GetWalletAccountByUserId(501)
	require.NoError(t, err)
	assert.InDelta(t, 168.0, grandAccount.CommissionAmount, 0.000001)
	assert.InDelta(t, 168.0, grandAccount.TotalCommissionAmount, 0.000001)

	childAccount, err := model.GetWalletAccountByUserId(503)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, childAccount.CommissionAmount, 0.000001)
	assert.Equal(t, int64(1), countWalletFlows(t, 503, model.WalletFlowTypeVipActivation, order.TradeNo))

	var records []model.CommissionRecord
	require.NoError(t, model.DB.Where("source_order_no = ?", order.TradeNo).Order("level asc").Find(&records).Error)
	require.Len(t, records, 2)
	assert.Equal(t, 502, records[0].BeneficiaryUserId)
	assert.Equal(t, 1, records[0].Level)
	assert.InDelta(t, model.DefaultVipActivationPaid, records[0].BaseAmount, 0.000001)
	assert.InDelta(t, 0.20, records[0].CommissionRate, 0.000001)
	assert.InDelta(t, 336.0, records[0].Amount, 0.000001)
	assert.Equal(t, model.CommissionStatusSettled, records[0].Status)
	assert.Equal(t, 501, records[1].BeneficiaryUserId)
	assert.Equal(t, 2, records[1].Level)
	assert.InDelta(t, model.DefaultVipActivationPaid, records[1].BaseAmount, 0.000001)
	assert.InDelta(t, 0.10, records[1].CommissionRate, 0.000001)
	assert.InDelta(t, 168.0, records[1].Amount, 0.000001)

	assert.Equal(t, int64(1), countWalletFlows(t, 502, model.WalletFlowTypeCommissionIncome, order.TradeNo))
	assert.Equal(t, int64(1), countWalletFlows(t, 501, model.WalletFlowTypeCommissionIncome, order.TradeNo))
	assert.Equal(t, []string{
		"VVIP Child/vvip_child (#503)",
		"VVIP Child/vvip_child (#503)",
	}, getCommissionSourceUserLabels(t, order.TradeNo))
	assert.Equal(t, []string{"算力伙伴 开通分佣：下级用户 VVIP Child/vvip_child (#503)"}, getWalletFlowRemarks(t, 502, model.WalletFlowTypeCommissionIncome, order.TradeNo))
	assert.Equal(t, []string{"算力伙伴 开通分佣：下级用户 VVIP Child/vvip_child (#503)"}, getWalletFlowRemarks(t, 501, model.WalletFlowTypeCommissionIncome, order.TradeNo))
}

func TestRepairVipActivationSettlementBackfillsSuccessfulOrder(t *testing.T) {
	truncate(t)

	setVipActivationCommissionRatesForTest(t, 0.20, 0.10)
	paymentSetting := operation_setting.GetPaymentSetting()
	oldDefaultVvipDiscount := paymentSetting.DefaultVvipTopupDiscount
	paymentSetting.DefaultVvipTopupDiscount = 0.7
	t.Cleanup(func() {
		paymentSetting.DefaultVvipTopupDiscount = oldDefaultVvipDiscount
	})

	seedVipActivationUser(t, 581, "repair_grand_parent", "rgp581")
	seedVipActivationUser(t, 582, "repair_parent", "rp582")
	seedVipActivationUser(t, 583, "repair_child", "rc583")
	seedActiveVvip(t, 581, "active-vvip-581")
	seedActiveVvip(t, 582, "active-vvip-582")
	seedWalletRelation(t, 581, 582)
	seedWalletRelation(t, 582, 583)

	order := &model.VipActivationRecord{
		UserId:          583,
		TradeNo:         "vip-repair-success-583",
		PaidAmount:      model.DefaultVipActivationPaid,
		PaymentProvider: model.PaymentProviderEpay,
		PaymentMethod:   "alipay",
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(order).Error)
	adjustedDiscount := 0.86
	require.NoError(t, model.UpdateUserTopupDiscount(583, &adjustedDiscount))

	require.NoError(t, RepairVipActivationSettlement(order.TradeNo, model.PaymentProviderEpay))
	require.NoError(t, RepairVipActivationSettlement(order.TradeNo, model.PaymentProviderEpay))

	assert.Equal(t, int64(1), countWalletFlows(t, 583, model.WalletFlowTypeVipActivation, order.TradeNo))
	assert.Equal(t, int64(1), countWalletFlows(t, 582, model.WalletFlowTypeCommissionIncome, order.TradeNo))
	assert.Equal(t, int64(1), countWalletFlows(t, 581, model.WalletFlowTypeCommissionIncome, order.TradeNo))

	parentAccount, err := model.GetWalletAccountByUserId(582)
	require.NoError(t, err)
	assert.InDelta(t, 336.0, parentAccount.CommissionAmount, 0.000001)
	grandAccount, err := model.GetWalletAccountByUserId(581)
	require.NoError(t, err)
	assert.InDelta(t, 168.0, grandAccount.CommissionAmount, 0.000001)

	var records []model.CommissionRecord
	require.NoError(t, model.DB.Where("source_order_no = ?", order.TradeNo).Order("level asc").Find(&records).Error)
	require.Len(t, records, 2)
	assert.Equal(t, 582, records[0].BeneficiaryUserId)
	assert.Equal(t, 1, records[0].Level)
	assert.Equal(t, 581, records[1].BeneficiaryUserId)
	assert.Equal(t, 2, records[1].Level)

	user, err := model.GetUserById(583, false)
	require.NoError(t, err)
	require.NotNil(t, user.TopupDiscount)
	assert.InDelta(t, adjustedDiscount, *user.TopupDiscount, 0.000001)
}

func TestCompleteVipActivationOrderUsesConfiguredFixedCommissionAmounts(t *testing.T) {
	truncate(t)
	setVipActivationAmountConfigForTest(t, 1680, 1000, 400)

	seedVipActivationUser(t, 521, "fixed_amount_grand_parent", "fagp521")
	seedVipActivationUser(t, 522, "fixed_amount_parent", "fap522")
	seedVipActivationUser(t, 523, "fixed_amount_child", "fac523")
	seedActiveVvip(t, 521, "active-vvip-521")
	seedActiveVvip(t, 522, "active-vvip-522")
	seedWalletRelation(t, 521, 522)
	seedWalletRelation(t, 522, 523)

	order, err := CreateVipActivationOrder(523, model.PaymentProviderEpay, "alipay")
	require.NoError(t, err)
	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderEpay, "alipay"))

	var records []model.CommissionRecord
	require.NoError(t, model.DB.Where("source_order_no = ?", order.TradeNo).Order("level asc").Find(&records).Error)
	require.Len(t, records, 2)
	assert.Equal(t, 522, records[0].BeneficiaryUserId)
	assert.Equal(t, 1, records[0].Level)
	assert.InDelta(t, 1680.0, records[0].BaseAmount, 0.000001)
	assert.InDelta(t, 1000.0/1680.0, records[0].CommissionRate, 0.000001)
	assert.InDelta(t, 1000.0, records[0].Amount, 0.000001)
	assert.Equal(t, 521, records[1].BeneficiaryUserId)
	assert.Equal(t, 2, records[1].Level)
	assert.InDelta(t, 1680.0, records[1].BaseAmount, 0.000001)
	assert.InDelta(t, 400.0/1680.0, records[1].CommissionRate, 0.000001)
	assert.InDelta(t, 400.0, records[1].Amount, 0.000001)
}

func TestCompleteVipActivationOrderUsesExistingPaidAmountForConfiguredCommissions(t *testing.T) {
	truncate(t)

	setVipActivationCommissionRatesForTest(t, 0.20, 0.10)

	seedVipActivationUser(t, 531, "paid_amount_grand_parent", "pagp531")
	seedVipActivationUser(t, 532, "paid_amount_parent", "pap532")
	seedVipActivationUser(t, 533, "paid_amount_child", "pac533")
	seedActiveVvip(t, 531, "active-vvip-531")
	seedActiveVvip(t, 532, "active-vvip-532")
	seedWalletRelation(t, 531, 532)
	seedWalletRelation(t, 532, 533)

	order, err := CreateVipActivationOrder(533, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.VipActivationRecord{}).
		Where("trade_no = ?", order.TradeNo).
		Updates(map[string]interface{}{
			"paid_amount": 840.0,
			"discount":    1.0,
		}).Error)

	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderStripe, ""))

	var refreshed model.VipActivationRecord
	require.NoError(t, model.DB.Where("trade_no = ?", order.TradeNo).First(&refreshed).Error)
	assert.InDelta(t, 840.0, refreshed.PaidAmount, 0.000001)
	assert.InDelta(t, 0.5, refreshed.Discount, 0.000001)

	var records []model.CommissionRecord
	require.NoError(t, model.DB.Where("source_order_no = ?", order.TradeNo).Order("level asc").Find(&records).Error)
	require.Len(t, records, 2)
	assert.InDelta(t, 840.0, records[0].BaseAmount, 0.000001)
	assert.InDelta(t, 0.4, records[0].CommissionRate, 0.000001)
	assert.InDelta(t, 336.0, records[0].Amount, 0.000001)
	assert.InDelta(t, 840.0, records[1].BaseAmount, 0.000001)
	assert.InDelta(t, 0.2, records[1].CommissionRate, 0.000001)
	assert.InDelta(t, 168.0, records[1].Amount, 0.000001)
}

func TestVipActivationCommissionSkipsUnqualifiedGrandParentOnly(t *testing.T) {
	truncate(t)

	setVipActivationCommissionRatesForTest(t, 0.20, 0.10)

	seedVipActivationUser(t, 571, "inactive_grand_parent", "igp571")
	seedVipActivationUser(t, 572, "active_parent_only", "apo572")
	seedVipActivationUser(t, 573, "vip_child_parent_only", "vcpo573")
	seedActiveVvip(t, 572, "active-vvip-572")
	seedWalletRelation(t, 571, 572)
	seedWalletRelation(t, 572, 573)

	order, err := CreateVipActivationOrder(573, model.PaymentProviderEpay, "alipay")
	require.NoError(t, err)
	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderEpay, "alipay"))

	var records []model.CommissionRecord
	require.NoError(t, model.DB.Where("source_order_no = ?", order.TradeNo).Order("level asc").Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, 572, records[0].BeneficiaryUserId)
	assert.Equal(t, 1, records[0].Level)
	assert.InDelta(t, 336.0, records[0].Amount, 0.000001)

	parentAccount, err := model.GetWalletAccountByUserId(572)
	require.NoError(t, err)
	assert.InDelta(t, 336.0, parentAccount.CommissionAmount, 0.000001)
	_, err = model.GetWalletAccountByUserId(571)
	assert.Error(t, err)
}

func TestReverseVipActivationOrderCreatesRefundReverseFlowIdempotently(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 591, "reverse_vvip_user", "rvu591")
	order, err := CreateVipActivationOrder(591, model.PaymentProviderEpay, "alipay")
	require.NoError(t, err)
	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderEpay, "alipay"))

	require.NoError(t, ReverseVipActivationOrder(order.TradeNo, model.PaymentProviderEpay, "退款冲正"))
	require.NoError(t, ReverseVipActivationOrder(order.TradeNo, model.PaymentProviderEpay, "退款冲正"))

	assert.Equal(t, int64(1), countWalletFlows(t, 591, model.WalletFlowTypeVipActivation, order.TradeNo))
	assert.Equal(t, int64(1), countWalletFlows(t, 591, model.WalletFlowTypeRefundReverse, order.TradeNo))

	var reverseFlow model.WalletFlow
	require.NoError(t, model.DB.Where("user_id = ? AND flow_type = ? AND biz_no = ?", 591, model.WalletFlowTypeRefundReverse, order.TradeNo).First(&reverseFlow).Error)
	assert.Equal(t, model.WalletFlowDirectionIn, reverseFlow.Direction)
	assert.InDelta(t, model.DefaultVipActivationPaid, reverseFlow.Amount, 0.000001)
}

func TestFailedCommissionRecordBlocksDuplicateSettlement(t *testing.T) {
	truncate(t)

	parentDiscount := 0.8
	seedVipActivationUser(t, 521, "failed_parent_topup", "fp521")
	seedVipActivationUser(t, 522, "failed_child_topup", "fc522")
	seedActiveVvip(t, 521, "active-vvip-521")
	require.NoError(t, model.UpdateUserTopupDiscount(521, &parentDiscount))
	seedWalletRelation(t, 521, 522)

	topUp := &model.TopUp{
		UserId:          522,
		Amount:          100,
		Money:           85,
		RechargeAmount:  100,
		PaidAmount:      85,
		Discount:        0.85,
		TradeNo:         "topup-commission-failed-idempotent",
		PaymentMethod:   "alipay",
		PaymentProvider: model.PaymentProviderEpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, model.DB.Create(topUp).Error)
	require.NoError(t, model.DB.Create(&model.CommissionRecord{
		BeneficiaryUserId:   521,
		SourceUserId:        522,
		SourceOrderNo:       topUp.TradeNo,
		SourceType:          model.CommissionSourceTypeTopUp,
		Level:               1,
		BaseAmount:          85,
		CommissionRate:      0.05,
		Amount:              4.25,
		QualificationStatus: model.CommissionQualificationQualified,
		Status:              model.CommissionStatusFailed,
		ErrorMessage:        "wallet credit failed",
	}).Error)

	require.NoError(t, CompleteTopUpOrder(CompleteTopUpOrderRequest{
		TradeNo:                 topUp.TradeNo,
		ExpectedPaymentProvider: model.PaymentProviderEpay,
		ActualPaymentMethod:     "alipay",
	}))

	_, err := model.GetWalletAccountByUserId(521)
	assert.Error(t, err)
	assert.Equal(t, int64(0), countWalletFlows(t, 521, model.WalletFlowTypeCommissionIncome, topUp.TradeNo))

	var records []model.CommissionRecord
	require.NoError(t, model.DB.Where("source_order_no = ?", topUp.TradeNo).Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, model.CommissionStatusFailed, records[0].Status)
	assert.Equal(t, "wallet credit failed", records[0].ErrorMessage)
}

func TestVipActivationCommissionRecordsFailedWhenQualificationQueryFails(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:vip_commission_failure_audit?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.CommissionRecord{}, &model.WalletFlow{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		_ = sqlDB.Close()
	})

	err = createVipActivationCommissionForBeneficiaryTx(db, 541, 542, "vip-qualification-query-failed", 1, 840, 0.2)
	require.NoError(t, err)
	err = createVipActivationCommissionForBeneficiaryTx(db, 541, 542, "vip-qualification-query-failed", 1, 840, 0.2)
	require.NoError(t, err)

	var records []model.CommissionRecord
	require.NoError(t, db.Where("source_order_no = ?", "vip-qualification-query-failed").Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, model.CommissionStatusFailed, records[0].Status)
	assert.NotEmpty(t, records[0].ErrorMessage)
	assert.Equal(t, int64(0), countWalletFlows(t, 541, model.WalletFlowTypeCommissionIncome, "vip-qualification-query-failed"))
}

func TestCompleteVipActivationOrderSkipsCommissionForNonVvipParent(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 511, "normal_parent", "normal511")
	seedVipActivationUser(t, 512, "child_without_vvip_parent", "child512")
	seedWalletRelation(t, 511, 512)

	order, err := CreateVipActivationOrder(512, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderStripe, ""))

	var count int64
	require.NoError(t, model.DB.Model(&model.CommissionRecord{}).Where("source_order_no = ?", order.TradeNo).Count(&count).Error)
	assert.Zero(t, count)
	_, err = model.GetWalletAccountByUserId(511)
	assert.Error(t, err)
}

func TestTransferCommissionToBalanceMovesCommissionIntoConsumableBalance(t *testing.T) {
	truncate(t)

	seedUser(t, 601, 0)
	seedWalletAccount(t, 601, 0, 25, 0)

	require.NoError(t, TransferCommissionToBalance(601, 10))

	account, err := model.GetWalletAccountByUserId(601)
	require.NoError(t, err)
	assert.InDelta(t, 10.0, account.BalanceAmount, 0.000001)
	assert.InDelta(t, 15.0, account.CommissionAmount, 0.000001)

	user, err := model.GetUserById(601, false)
	require.NoError(t, err)
	assert.Equal(t, int(10*common.QuotaPerUnit), user.Quota)
	assert.Equal(t, int64(1), countWalletFlows(t, 601, model.WalletFlowTypeCommissionToBalance, "commission-transfer-601"))
	transferFlow := getWalletFlow(t, 601, model.WalletFlowTypeCommissionToBalance, "commission-transfer-601")
	assertWalletFlowSnapshot(t, transferFlow, 10.0, 15.0, 0.0)
}

func TestTransferCommissionToBalanceCreatesQiniuQuotaGrant(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)
	configureQiniuKeySettingForTest(t, "http://127.0.0.1")

	keyBody := strings.Repeat("5", 64)
	seedUser(t, 602, 0)
	seedWalletAccount(t, 602, 0, 25, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             602,
		UserId:         602,
		Name:           "qiniu-token",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	require.NoError(t, TransferCommissionToBalance(602, 10))

	transferFlow := getWalletFlow(t, 602, model.WalletFlowTypeCommissionToBalance, "commission-transfer-602")
	assertQiniuQuotaGrant(t, 602, 602, "commission_transfer:"+strconv.Itoa(transferFlow.Id), 10, model.QiniuQuotaGrantStatusPending)
	assertQiniuQuotaSyncTaskCount(t, 602, 602, 0)
}

func TestTransferCommissionToBalanceRollsBackWhenQiniuQuotaGrantRecordFails(t *testing.T) {
	truncate(t)
	disableQiniuAsyncForTest(t)
	configureQiniuKeySettingForTest(t, "http://127.0.0.1")

	keyBody := strings.Repeat("7", 64)
	seedUser(t, 603, 0)
	seedWalletAccount(t, 603, 0, 25, 0)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             603,
		UserId:         603,
		Name:           "qiniu-token",
		Key:            keyBody,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	registerFailQiniuQuotaGrantCreateCallback(t)
	err := TransferCommissionToBalance(603, 10)
	require.Error(t, err)

	account, err := model.GetWalletAccountByUserId(603)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, account.BalanceAmount, 0.000001)
	assert.InDelta(t, 25.0, account.CommissionAmount, 0.000001)
	user, err := model.GetUserById(603, false)
	require.NoError(t, err)
	assert.Equal(t, 0, user.Quota)
	assert.Equal(t, int64(0), countWalletFlows(t, 603, model.WalletFlowTypeCommissionToBalance, "commission-transfer-603"))
}

func registerFailQiniuQuotaGrantCreateCallback(t *testing.T) {
	t.Helper()
	callbackName := "test_fail_qiniu_quota_grant_create_" + common.GetRandomString(8)
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(db *gorm.DB) {
		if db.Statement != nil && db.Statement.Table == "qiniu_quota_grants" {
			db.AddError(errors.New("test qiniu quota grant insert failure"))
		}
	}))
	t.Cleanup(func() {
		model.DB.Callback().Create().Remove(callbackName)
	})
}

func TestWithdrawWorkflowFreezesRejectsAndPaysCommission(t *testing.T) {
	truncate(t)

	seedUser(t, 701, 0)
	seedWalletAccount(t, 701, 0, 50, 0)
	operation_setting.GetPaymentSetting().CommissionMinWithdrawAmount = 10
	t.Cleanup(func() { operation_setting.GetPaymentSetting().CommissionMinWithdrawAmount = 0 })

	rejected, err := SubmitWithdrawOrder(SubmitWithdrawOrderRequest{
		UserId:         701,
		Amount:         20,
		ReceiveType:    "bank",
		ReceiveAccount: "bank-account",
	})
	require.NoError(t, err)
	account, err := model.GetWalletAccountByUserId(701)
	require.NoError(t, err)
	assert.InDelta(t, 30.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 20.0, account.FrozenCommissionAmount, 0.000001)
	assert.Equal(t, model.WithdrawStatusPending, rejected.Status)
	rejectedFreezeFlow := getWalletFlow(t, 701, model.WalletFlowTypeWithdrawFreeze, rejected.WithdrawNo)
	assertWalletFlowSnapshot(t, rejectedFreezeFlow, 0.0, 30.0, 20.0)

	require.NoError(t, RejectWithdrawOrder(rejected.Id, 1, "invalid account"))
	account, err = model.GetWalletAccountByUserId(701)
	require.NoError(t, err)
	assert.InDelta(t, 50.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
	rejectedReturnFlow := getWalletFlow(t, 701, model.WalletFlowTypeWithdrawReject, rejected.WithdrawNo)
	assertWalletFlowSnapshot(t, rejectedReturnFlow, 0.0, 50.0, 0.0)

	paid, err := SubmitWithdrawOrder(SubmitWithdrawOrderRequest{
		UserId:         701,
		Amount:         15,
		ReceiveType:    "bank",
		ReceiveAccount: "bank-account",
	})
	require.NoError(t, err)
	paidFreezeFlow := getWalletFlow(t, 701, model.WalletFlowTypeWithdrawFreeze, paid.WithdrawNo)
	assertWalletFlowSnapshot(t, paidFreezeFlow, 0.0, 35.0, 15.0)
	require.NoError(t, ApproveWithdrawOrder(paid.Id, 1, "approved"))
	require.NoError(t, MarkWithdrawOrderPaid(paid.Id, 1, "voucher-701", "paid"))

	account, err = model.GetWalletAccountByUserId(701)
	require.NoError(t, err)
	assert.InDelta(t, 35.0, account.CommissionAmount, 0.000001)
	assert.InDelta(t, 0.0, account.FrozenCommissionAmount, 0.000001)
	assert.InDelta(t, 15.0, account.TotalWithdrawAmount, 0.000001)

	var refreshed model.WithdrawOrder
	require.NoError(t, model.DB.First(&refreshed, paid.Id).Error)
	assert.Equal(t, model.WithdrawStatusPaid, refreshed.Status)
	assert.Equal(t, "voucher-701", refreshed.PaymentVoucher)
	assert.Equal(t, int64(1), countWalletFlows(t, 701, model.WalletFlowTypeWithdrawFreeze, paid.WithdrawNo))
	assert.Equal(t, int64(1), countWalletFlows(t, 701, model.WalletFlowTypeWithdrawSuccess, paid.WithdrawNo))
	paidSuccessFlow := getWalletFlow(t, 701, model.WalletFlowTypeWithdrawSuccess, paid.WithdrawNo)
	assertWalletFlowSnapshot(t, paidSuccessFlow, 0.0, 35.0, 0.0)
}

func TestWalletFundingSynchronizesWalletBalanceWithQuota(t *testing.T) {
	truncate(t)

	seedUser(t, 801, int(100*common.QuotaPerUnit))
	seedWalletAccount(t, 801, 100, 0, 0)

	funding := &WalletFunding{userId: 801}
	require.NoError(t, funding.PreConsume(int(25*common.QuotaPerUnit)))

	account, err := model.GetWalletAccountByUserId(801)
	require.NoError(t, err)
	assert.InDelta(t, 75.0, account.BalanceAmount, 0.000001)
	user, err := model.GetUserById(801, false)
	require.NoError(t, err)
	assert.Equal(t, int(75*common.QuotaPerUnit), user.Quota)

	require.NoError(t, funding.Refund())
	account, err = model.GetWalletAccountByUserId(801)
	require.NoError(t, err)
	assert.InDelta(t, 100.0, account.BalanceAmount, 0.000001)
	user, err = model.GetUserById(801, false)
	require.NoError(t, err)
	assert.Equal(t, int(100*common.QuotaPerUnit), user.Quota)
}

func TestTaskAdjustFundingSynchronizesWalletBalanceWithQuota(t *testing.T) {
	truncate(t)

	seedUser(t, 811, int(100*common.QuotaPerUnit))
	seedWalletAccount(t, 811, 100, 0, 0)
	task := makeTask(811, 0, 0, 0, BillingSourceWallet, 0)
	task.TaskID = "task-wallet-sync-811"

	require.NoError(t, taskAdjustFunding(task, int(30*common.QuotaPerUnit)))

	account, err := model.GetWalletAccountByUserId(811)
	require.NoError(t, err)
	assert.InDelta(t, 70.0, account.BalanceAmount, 0.000001)
	user, err := model.GetUserById(811, false)
	require.NoError(t, err)
	assert.Equal(t, int(70*common.QuotaPerUnit), user.Quota)

	require.NoError(t, taskAdjustFunding(task, -int(12*common.QuotaPerUnit)))

	account, err = model.GetWalletAccountByUserId(811)
	require.NoError(t, err)
	assert.InDelta(t, 82.0, account.BalanceAmount, 0.000001)
	user, err = model.GetUserById(811, false)
	require.NoError(t, err)
	assert.Equal(t, int(82*common.QuotaPerUnit), user.Quota)
}
