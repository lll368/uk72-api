package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestListAdminTopUpRecordsIncludesCurrentUserContactAndFilters(t *testing.T) {
	truncateTables(t)
	clearAdminRechargeRecordTables(t)

	require.NoError(t, DB.Create(&User{
		Id:          4101,
		Username:    "topup_user",
		DisplayName: "Topup User",
		Email:       "before-topup@example.com",
		AffCode:     "topup4101",
		Status:      common.UserStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:          4102,
		Username:    "other_topup_user",
		DisplayName: "Other Topup User",
		Email:       "other@example.com",
		AffCode:     "topup4102",
		Status:      common.UserStatusEnabled,
	}).Error)
	phone := "13800138001"
	require.NoError(t, DB.Create(&UserProfile{
		UserId:      4101,
		PhoneNumber: &phone,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId:          4101,
		Amount:          100,
		Money:           100,
		RechargeAmount:  100,
		PaidAmount:      95,
		Discount:        0.95,
		TradeNo:         "TOPUP-CURRENT-USER",
		PaymentProvider: PaymentProviderStripe,
		PaymentMethod:   PaymentMethodStripe,
		CreateTime:      1710000000,
		CompleteTime:    1710000300,
		Status:          common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId:          4102,
		Amount:          200,
		Money:           200,
		TradeNo:         "TOPUP-OTHER-USER",
		PaymentProvider: PaymentProviderWechat,
		PaymentMethod:   PaymentMethodWechatDirect,
		CreateTime:      1710001000,
		CompleteTime:    0,
		Status:          common.TopUpStatusPending,
	}).Error)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", 4101).Update("email", "current-topup@example.com").Error)
	updatedPhone := "13800138099"
	require.NoError(t, DB.Model(&UserProfile{}).Where("user_id = ?", 4101).Update("phone_number", updatedPhone).Error)

	records, total, err := ListAdminTopUpRecords(&AdminTopUpRecordFilter{
		Email:           "current-topup@example.com",
		PhoneNumber:     updatedPhone,
		TradeNo:         "CURRENT",
		Status:          common.TopUpStatusSuccess,
		PaymentProvider: PaymentProviderStripe,
		PaymentMethod:   PaymentMethodStripe,
		CreatedFrom:     1709999999,
		CreatedTo:       1710000001,
		CompletedFrom:   1710000200,
		CompletedTo:     1710000400,
	}, &common.PageInfo{Page: 1, PageSize: 10})

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, records, 1)
	require.Equal(t, "TOPUP-CURRENT-USER", records[0].TradeNo)
	require.Equal(t, 4101, records[0].UserId)
	require.Equal(t, "topup_user", records[0].Username)
	require.Equal(t, "Topup User", records[0].DisplayName)
	require.Equal(t, "current-topup@example.com", records[0].Email)
	require.Equal(t, updatedPhone, records[0].PhoneNumber)
}

func TestListAdminTopUpRecordsReturnsEmptyPhoneAndPagedTotal(t *testing.T) {
	truncateTables(t)
	clearAdminRechargeRecordTables(t)

	require.NoError(t, DB.Create(&User{
		Id:          4201,
		Username:    "empty_phone_topup_user",
		DisplayName: "Empty Phone User",
		Email:       "empty-phone@example.com",
		AffCode:     "topup4201",
		Status:      common.UserStatusEnabled,
	}).Error)
	for i := 0; i < 3; i++ {
		require.NoError(t, DB.Create(&TopUp{
			UserId:          4201,
			Amount:          int64(10 + i),
			Money:           float64(10 + i),
			TradeNo:         "TOPUP-PAGED-" + string(rune('A'+i)),
			PaymentProvider: PaymentProviderAlipay,
			PaymentMethod:   PaymentMethodAlipayDirect,
			CreateTime:      1710010000 + int64(i),
			Status:          common.TopUpStatusSuccess,
		}).Error)
	}

	records, total, err := ListAdminTopUpRecords(&AdminTopUpRecordFilter{
		UserId: 4201,
	}, &common.PageInfo{Page: 1, PageSize: 2})

	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, records, 2)
	require.Equal(t, "", records[0].PhoneNumber)
	require.Equal(t, "empty-phone@example.com", records[0].Email)
}

func TestListAdminVipActivationRecordsIncludesCurrentUserContactAndFilters(t *testing.T) {
	truncateTables(t)
	clearAdminRechargeRecordTables(t)

	require.NoError(t, DB.Create(&User{
		Id:          4301,
		Username:    "vvip_user",
		DisplayName: "VVIP User",
		Email:       "before-vvip@example.com",
		AffCode:     "vvip4301",
		Status:      common.UserStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:          4302,
		Username:    "other_vvip_user",
		DisplayName: "Other VVIP User",
		Email:       "other-vvip@example.com",
		AffCode:     "vvip4302",
		Status:      common.UserStatusEnabled,
	}).Error)
	phone := "13900139001"
	require.NoError(t, DB.Create(&UserProfile{
		UserId:      4301,
		PhoneNumber: &phone,
	}).Error)
	require.NoError(t, DB.Create(&VipActivationRecord{
		UserId:           4301,
		TradeNo:          "VVIP-CURRENT-USER",
		ActivationAmount: 1680,
		PaidAmount:       1580,
		Discount:         0.94,
		PaymentProvider:  PaymentProviderStripe,
		PaymentMethod:    PaymentMethodStripe,
		Status:           VipActivationStatusSuccess,
		ActivatedAt:      1710100300,
		CreatedAt:        1710100000,
	}).Error)
	require.NoError(t, DB.Create(&VipActivationRecord{
		UserId:          4302,
		TradeNo:         "VVIP-OTHER-USER",
		PaymentProvider: PaymentProviderWechat,
		PaymentMethod:   PaymentMethodWechatDirect,
		Status:          VipActivationStatusPending,
		CreatedAt:       1710101000,
	}).Error)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", 4301).Update("email", "current-vvip@example.com").Error)
	updatedPhone := "13900139099"
	require.NoError(t, DB.Model(&UserProfile{}).Where("user_id = ?", 4301).Update("phone_number", updatedPhone).Error)

	records, total, err := ListAdminVipActivationRecords(&AdminVipActivationRecordFilter{
		Email:           "current-vvip@example.com",
		PhoneNumber:     updatedPhone,
		TradeNo:         "CURRENT",
		Status:          VipActivationStatusSuccess,
		PaymentProvider: PaymentProviderStripe,
		PaymentMethod:   PaymentMethodStripe,
		CreatedFrom:     1710099999,
		CreatedTo:       1710100001,
		ActivatedFrom:   1710100200,
		ActivatedTo:     1710100400,
	}, &common.PageInfo{Page: 1, PageSize: 10})

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, records, 1)
	require.Equal(t, "VVIP-CURRENT-USER", records[0].TradeNo)
	require.Equal(t, 4301, records[0].UserId)
	require.Equal(t, "vvip_user", records[0].Username)
	require.Equal(t, "VVIP User", records[0].DisplayName)
	require.Equal(t, "current-vvip@example.com", records[0].Email)
	require.Equal(t, updatedPhone, records[0].PhoneNumber)
}

func TestListAdminVipActivationRecordsReturnsEmptyPhoneAndPagedTotal(t *testing.T) {
	truncateTables(t)
	clearAdminRechargeRecordTables(t)

	require.NoError(t, DB.Create(&User{
		Id:          4401,
		Username:    "empty_phone_vvip_user",
		DisplayName: "Empty VVIP Phone User",
		Email:       "empty-vvip-phone@example.com",
		AffCode:     "vvip4401",
		Status:      common.UserStatusEnabled,
	}).Error)
	for i := 0; i < 3; i++ {
		require.NoError(t, DB.Create(&VipActivationRecord{
			UserId:          4401,
			TradeNo:         "VVIP-PAGED-" + string(rune('A'+i)),
			PaymentProvider: PaymentProviderAlipay,
			PaymentMethod:   PaymentMethodAlipayDirect,
			Status:          VipActivationStatusSuccess,
			ActivatedAt:     1710110000 + int64(i),
			CreatedAt:       1710109000 + int64(i),
		}).Error)
	}

	records, total, err := ListAdminVipActivationRecords(&AdminVipActivationRecordFilter{
		UserId: 4401,
	}, &common.PageInfo{Page: 1, PageSize: 2})

	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, records, 2)
	require.Equal(t, "", records[0].PhoneNumber)
	require.Equal(t, "empty-vvip-phone@example.com", records[0].Email)
}

func clearAdminRechargeRecordTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM top_ups").Error)
	require.NoError(t, DB.Exec("DELETE FROM vip_activation_records").Error)
	require.NoError(t, DB.Exec("DELETE FROM user_profiles").Error)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
}
