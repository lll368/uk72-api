package service

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedVipActivationUser(t *testing.T, id int, username string, affCode string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       id,
		Username: username,
		AffCode:  affCode,
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
	}).Error)
}

func seedActiveVvip(t *testing.T, userId int, tradeNo string) {
	t.Helper()
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:          userId,
		TradeNo:         tradeNo,
		PaymentProvider: model.PaymentProviderEpay,
		PaymentMethod:   "alipay",
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     now,
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserProfile{
		UserId:          userId,
		IsVvip:          true,
		VvipActivatedAt: now,
		VvipStatus:      model.VvipStatusActive,
	}).Error)
}

func countVipActivationSuccessRecords(t *testing.T, userId int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.VipActivationRecord{}).
		Where("user_id = ? AND status = ?", userId, model.VipActivationStatusSuccess).
		Count(&count).Error)
	return count
}

func TestCompleteVipActivationOrderActivatesUserIdempotently(t *testing.T) {
	truncate(t)

	operation_setting.GetPaymentSetting().AmountDiscount[1680] = 0.5
	t.Cleanup(func() { delete(operation_setting.GetPaymentSetting().AmountDiscount, 1680) })

	seedVipActivationUser(t, 101, "vip_user", "vip101")

	order, err := CreateVipActivationOrder(101, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)
	require.NotEmpty(t, order.TradeNo)
	assert.Equal(t, 1680.0, order.ActivationAmount)
	assert.Equal(t, 1680.0, order.PaidAmount)
	assert.Equal(t, 1.0, order.Discount)
	assert.Equal(t, model.VipActivationStatusPending, order.Status)

	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderStripe, ""))
	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid-again"}`, model.PaymentProviderStripe, ""))

	active, err := model.GetActiveVipActivationByUserId(101)
	require.NoError(t, err)
	assert.Equal(t, order.TradeNo, active.TradeNo)
	assert.Equal(t, 1680.0, active.ActivationAmount)
	assert.Equal(t, 1680.0, active.PaidAmount)
	assert.Equal(t, 1.0, active.Discount)
	assert.NotZero(t, active.ActivatedAt)
	assert.Equal(t, int64(1), countVipActivationSuccessRecords(t, 101))

	profile, err := model.GetUserProfileByUserId(101)
	require.NoError(t, err)
	assert.True(t, profile.IsVvip)
	assert.Equal(t, model.VvipStatusActive, profile.VvipStatus)
	assert.NotZero(t, profile.VvipActivatedAt)
}

func TestCreateVipActivationOrderUsesConfiguredActivationPrice(t *testing.T) {
	truncate(t)
	setVipActivationAmountConfigForTest(t, 1999, 1000, 400)

	seedVipActivationUser(t, 109, "vip_price_user", "vip109")

	order, err := CreateVipActivationOrder(109, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	assert.InDelta(t, 1999.0, order.ActivationAmount, 0.000001)
	assert.InDelta(t, 1999.0, order.PaidAmount, 0.000001)
	assert.InDelta(t, 1.0, order.Discount, 0.000001)
}

func TestCreateVipActivationOrderRoundsConfiguredActivationPriceToCents(t *testing.T) {
	truncate(t)
	setVipActivationAmountConfigForTest(t, 19.999, 10, 5)

	seedVipActivationUser(t, 112, "vip_rounded_price_user", "vip112")

	order, err := CreateVipActivationOrder(112, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	assert.InDelta(t, 20.0, order.ActivationAmount, 0.000001)
	assert.InDelta(t, 20.0, order.PaidAmount, 0.000001)
	assert.InDelta(t, 1.0, order.Discount, 0.000001)
}

func TestCreateVipActivationOrderRejectsPriceRoundedToZeroCents(t *testing.T) {
	truncate(t)
	setVipActivationAmountConfigForTest(t, 0.004, 0, 0)

	seedVipActivationUser(t, 113, "vip_zero_cent_price_user", "vip113")

	_, err := CreateVipActivationOrder(113, model.PaymentProviderStripe, model.PaymentMethodStripe)

	assert.Error(t, err)
}

func TestCompleteVipActivationOrderDoesNotOverwriteSuccessfulPaymentSnapshot(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 110, "vip_duplicate_payment_user", "vip110")
	order, err := CreateVipActivationOrder(110, model.PaymentProviderEpay, "alipay")
	require.NoError(t, err)

	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderEpay, "alipay"))
	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid-again"}`, model.PaymentProviderEpay, "wxpay"))

	created, err := model.GetVipActivationRecordByTradeNo(order.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, model.VipActivationStatusSuccess, created.Status)
	assert.Equal(t, `{"event":"paid"}`, created.ProviderPayload)
	assert.Equal(t, "alipay", created.PaymentMethod)
}

func TestCompleteVipActivationOrderAppliesDefaultVvipTopupDiscount(t *testing.T) {
	truncate(t)

	paymentSetting := operation_setting.GetPaymentSetting()
	oldDefaultVvipDiscount := paymentSetting.DefaultVvipTopupDiscount
	paymentSetting.DefaultVvipTopupDiscount = 0.7
	t.Cleanup(func() {
		paymentSetting.DefaultVvipTopupDiscount = oldDefaultVvipDiscount
	})

	seedVipActivationUser(t, 108, "vip_discount_user", "vip108")
	order, err := CreateVipActivationOrder(108, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderStripe, ""))

	user, err := model.GetUserById(108, false)
	require.NoError(t, err)
	require.NotNil(t, user.TopupDiscount)
	assert.InDelta(t, 0.7, *user.TopupDiscount, 0.000001)
	assert.InDelta(t, 0.7, user.EffectiveTopupDiscount, 0.000001)
	assert.Equal(t, model.UserTopupDiscountSourceUser, user.TopupDiscountSource)
}

func TestCompleteVipActivationOrderDoesNotOverwriteAdjustedVvipTopupDiscount(t *testing.T) {
	truncate(t)

	paymentSetting := operation_setting.GetPaymentSetting()
	oldDefaultVvipDiscount := paymentSetting.DefaultVvipTopupDiscount
	paymentSetting.DefaultVvipTopupDiscount = 0.7
	t.Cleanup(func() {
		paymentSetting.DefaultVvipTopupDiscount = oldDefaultVvipDiscount
	})

	seedVipActivationUser(t, 111, "vip_adjusted_discount_user", "vip111")
	order, err := CreateVipActivationOrder(111, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderStripe, ""))

	adjustedDiscount := 0.82
	require.NoError(t, model.UpdateUserTopupDiscount(111, &adjustedDiscount))

	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid-again"}`, model.PaymentProviderStripe, ""))

	user, err := model.GetUserById(111, false)
	require.NoError(t, err)
	require.NotNil(t, user.TopupDiscount)
	assert.InDelta(t, adjustedDiscount, *user.TopupDiscount, 0.000001)
}

func TestCompleteVipActivationOrderResetsDiscountWhenDefaultVvipDiscountIsOne(t *testing.T) {
	truncate(t)

	paymentSetting := operation_setting.GetPaymentSetting()
	oldDefaultVvipDiscount := paymentSetting.DefaultVvipTopupDiscount
	paymentSetting.DefaultVvipTopupDiscount = 1
	t.Cleanup(func() {
		paymentSetting.DefaultVvipTopupDiscount = oldDefaultVvipDiscount
	})

	existingDiscount := 0.85
	seedVipActivationUser(t, 109, "vip_keep_discount_user", "vip109")
	require.NoError(t, model.UpdateUserTopupDiscount(109, &existingDiscount))
	order, err := CreateVipActivationOrder(109, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	require.NoError(t, CompleteVipActivationOrder(order.TradeNo, `{"event":"paid"}`, model.PaymentProviderStripe, ""))

	user, err := model.GetUserById(109, false)
	require.NoError(t, err)
	require.NotNil(t, user.TopupDiscount)
	assert.InDelta(t, 1, *user.TopupDiscount, 0.000001)
}

func TestCompleteVipActivationOrderRejectsMismatchedProvider(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 102, "vip_mismatch_user", "vip102")
	order, err := CreateVipActivationOrder(102, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	err = CompleteVipActivationOrder(order.TradeNo, `{"provider":"epay"}`, model.PaymentProviderEpay, "alipay")
	require.ErrorIs(t, err, model.ErrPaymentMethodMismatch)

	created, err := model.GetVipActivationRecordByTradeNo(order.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, model.VipActivationStatusPending, created.Status)
	_, err = model.GetActiveVipActivationByUserId(102)
	require.Error(t, err)
}

func TestCreateVipActivationOrderRejectsActiveVvip(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 103, "active_vvip_user", "vip103")
	seedActiveVvip(t, 103, "active-vvip-103")

	_, err := CreateVipActivationOrder(103, model.PaymentProviderStripe, model.PaymentMethodStripe)

	require.ErrorIs(t, err, model.ErrVipActivationAlreadyActive)
}

func TestFailVipActivationOrderMarksPendingOrderFailed(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 104, "vip_failed_user", "vip104")
	order, err := CreateVipActivationOrder(104, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	require.NoError(t, FailVipActivationOrder(order.TradeNo, `{"event":"failed"}`, model.PaymentProviderStripe))
	require.NoError(t, FailVipActivationOrder(order.TradeNo, `{"event":"failed-again"}`, model.PaymentProviderStripe))

	created, err := model.GetVipActivationRecordByTradeNo(order.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, model.VipActivationStatusFailed, created.Status)
	assert.Contains(t, created.ProviderPayload, "failed")
	assert.Zero(t, created.ActivatedAt)
}

func TestFailVipActivationOrderRejectsMismatchedProvider(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 105, "vip_failed_mismatch_user", "vip105")
	order, err := CreateVipActivationOrder(105, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	err = FailVipActivationOrder(order.TradeNo, `{"provider":"waffo"}`, model.PaymentProviderWaffo)

	require.ErrorIs(t, err, model.ErrPaymentMethodMismatch)
	created, err := model.GetVipActivationRecordByTradeNo(order.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, model.VipActivationStatusPending, created.Status)
}

func TestDisableVipActivationDisablesAllActiveRecords(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 106, "vip_disable_user", "vip106")
	seedActiveVvip(t, 106, "active-vvip-106-a")
	require.NoError(t, model.DB.Create(&model.VipActivationRecord{
		UserId:          106,
		TradeNo:         "active-vvip-106-b",
		PaymentProvider: model.PaymentProviderStripe,
		PaymentMethod:   model.PaymentMethodStripe,
		Status:          model.VipActivationStatusSuccess,
		ActivatedAt:     time.Now().Unix() + 1,
	}).Error)

	require.NoError(t, DisableVipActivation(106, 1, "manual disable", "127.0.0.1"))

	isActive, err := model.IsUserActiveVvip(106)
	require.NoError(t, err)
	assert.False(t, isActive)

	var disabledCount int64
	require.NoError(t, model.DB.Model(&model.VipActivationRecord{}).
		Where("user_id = ? AND status = ?", 106, model.VipActivationStatusDisabled).
		Count(&disabledCount).Error)
	assert.Equal(t, int64(2), disabledCount)

	profile, err := model.GetUserProfileByUserId(106)
	require.NoError(t, err)
	assert.False(t, profile.IsVvip)
	assert.Equal(t, model.VvipStatusDisabled, profile.VvipStatus)
	assert.NotZero(t, profile.VvipDisabledAt)
}

func TestDisableVipActivationInvalidatesPendingOrders(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 107, "vip_disable_pending_user", "vip107")
	firstOrder, err := CreateVipActivationOrder(107, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)
	secondOrder, err := CreateVipActivationOrder(107, model.PaymentProviderStripe, model.PaymentMethodStripe)
	require.NoError(t, err)

	require.NoError(t, CompleteVipActivationOrder(firstOrder.TradeNo, `{"event":"paid"}`, model.PaymentProviderStripe, ""))
	require.NoError(t, DisableVipActivation(107, 1, "manual disable", "127.0.0.1"))
	require.NoError(t, CompleteVipActivationOrder(firstOrder.TradeNo, `{"event":"duplicate-paid"}`, model.PaymentProviderStripe, ""))

	err = CompleteVipActivationOrder(secondOrder.TradeNo, `{"event":"late-paid"}`, model.PaymentProviderStripe, "")
	require.ErrorIs(t, err, model.ErrVipActivationOrderStatusInvalid)

	created, err := model.GetVipActivationRecordByTradeNo(secondOrder.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, model.VipActivationStatusFailed, created.Status)
	assert.Equal(t, int64(0), countVipActivationSuccessRecords(t, 107))

	isActive, err := model.IsUserActiveVvip(107)
	require.NoError(t, err)
	assert.False(t, isActive)
}

func TestBindInvitationRelationAfterRegistrationRequiresActiveVvip(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 201, "normal_inviter", "normal201")
	seedVipActivationUser(t, 202, "child_without_vvip_parent", "child202")

	require.NoError(t, BindInvitationRelationAfterRegistrationTx(model.DB, 201, 202, model.UserRelationSourceRegister))

	_, err := model.GetActiveUserRelationByChildId(202)
	require.True(t, errors.Is(err, gorm.ErrRecordNotFound), "non-vvip inviter must not create relation")

	seedActiveVvip(t, 201, "active-vvip-201")
	seedVipActivationUser(t, 203, "child_with_vvip_parent", "child203")

	require.NoError(t, BindInvitationRelationAfterRegistrationTx(model.DB, 201, 203, model.UserRelationSourceRegister))

	relation, err := model.GetActiveUserRelationByChildId(203)
	require.NoError(t, err)
	assert.Equal(t, 201, relation.ParentUserId)
	assert.Equal(t, 203, relation.ChildUserId)
	assert.Equal(t, model.UserRelationSourceRegister, relation.Source)
}

func TestBindInvitationRelationAfterRegistrationSkipsInvalidRelation(t *testing.T) {
	truncate(t)

	seedVipActivationUser(t, 301, "vip_parent", "vip301")
	seedVipActivationUser(t, 302, "vip_child", "vip302")
	seedActiveVvip(t, 301, "active-vvip-301")
	seedActiveVvip(t, 302, "active-vvip-302")

	require.NoError(t, BindInvitationRelationAfterRegistrationTx(model.DB, 301, 301, model.UserRelationSourceRegister))
	_, err := model.GetActiveUserRelationByChildId(301)
	require.True(t, errors.Is(err, gorm.ErrRecordNotFound), "self binding must be skipped")

	require.NoError(t, BindInvitationRelationAfterRegistrationTx(model.DB, 301, 302, model.UserRelationSourceRegister))
	require.NoError(t, BindInvitationRelationAfterRegistrationTx(model.DB, 301, 302, model.UserRelationSourceRegister))

	var relationCount int64
	require.NoError(t, model.DB.Model(&model.UserRelation{}).
		Where("parent_user_id = ? AND child_user_id = ? AND status = ?", 301, 302, model.UserRelationStatusActive).
		Count(&relationCount).Error)
	assert.Equal(t, int64(1), relationCount)

	require.NoError(t, BindInvitationRelationAfterRegistrationTx(model.DB, 302, 301, model.UserRelationSourceRegister))

	var cycleCount int64
	require.NoError(t, model.DB.Model(&model.UserRelation{}).
		Where("parent_user_id = ? AND child_user_id = ? AND status = ?", 302, 301, model.UserRelationStatusActive).
		Count(&cycleCount).Error)
	assert.Zero(t, cycleCount, "cycle relation must be skipped")
}
