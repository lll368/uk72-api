package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedUserVvipStatusTestUser(t *testing.T, id int, username string) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:          id,
		Username:    username,
		Password:    "password123",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     fmt.Sprintf("aff%d", id),
	}).Error)
}

func usersById(users []*User) map[int]*User {
	result := make(map[int]*User, len(users))
	for _, user := range users {
		result[user.Id] = user
	}
	return result
}

func TestGetAllUsersHydratesVvipStatus(t *testing.T) {
	truncateTables(t)
	seedUserVvipStatusTestUser(t, 2101, "active_vvip")
	seedUserVvipStatusTestUser(t, 2102, "disabled_vvip")
	seedUserVvipStatusTestUser(t, 2103, "plain_user")
	seedUserVvipStatusTestUser(t, 2104, "phone_only_profile")

	require.NoError(t, DB.Create(&UserProfile{
		UserId:          2101,
		IsVvip:          true,
		VvipStatus:      VvipStatusActive,
		VvipActivatedAt: 1700000001,
	}).Error)
	require.NoError(t, DB.Create(&UserProfile{
		UserId:          2102,
		IsVvip:          false,
		VvipStatus:      VvipStatusDisabled,
		VvipActivatedAt: 1700000002,
		VvipDisabledAt:  1700000003,
	}).Error)
	require.NoError(t, DB.Create(&UserProfile{
		UserId:     2104,
		IsVvip:     false,
		VvipStatus: VvipStatusDisabled,
	}).Error)

	users, total, err := GetAllUsers(&common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)

	byId := usersById(users)
	assert.True(t, byId[2101].IsVvip)
	assert.Equal(t, VvipStatusActive, byId[2101].VvipStatus)
	assert.Equal(t, int64(1700000001), byId[2101].VvipActivatedAt)

	assert.False(t, byId[2102].IsVvip)
	assert.Equal(t, VvipStatusDisabled, byId[2102].VvipStatus)
	assert.Equal(t, int64(1700000003), byId[2102].VvipDisabledAt)

	assert.False(t, byId[2103].IsVvip)
	assert.Equal(t, VvipStatusNone, byId[2103].VvipStatus)

	assert.False(t, byId[2104].IsVvip)
	assert.Equal(t, VvipStatusNone, byId[2104].VvipStatus)
}

func TestSearchUsersFiltersByVvipStatus(t *testing.T) {
	truncateTables(t)
	seedUserVvipStatusTestUser(t, 2201, "active_vvip")
	seedUserVvipStatusTestUser(t, 2202, "disabled_vvip")
	seedUserVvipStatusTestUser(t, 2203, "plain_user")
	seedUserVvipStatusTestUser(t, 2204, "phone_only_profile")

	require.NoError(t, DB.Create(&UserProfile{
		UserId:          2201,
		IsVvip:          true,
		VvipStatus:      VvipStatusActive,
		VvipActivatedAt: 1700000011,
	}).Error)
	require.NoError(t, DB.Create(&VipActivationRecord{
		UserId:      2201,
		TradeNo:     "active-vvip-2201",
		Status:      VipActivationStatusSuccess,
		ActivatedAt: 1700000011,
	}).Error)
	require.NoError(t, DB.Create(&VipActivationRecord{
		UserId:        2202,
		TradeNo:       "disabled-vvip-2202",
		Status:        VipActivationStatusDisabled,
		ActivatedAt:   1700000021,
		DisabledAt:    1700000022,
		DisableReason: "test disabled",
	}).Error)
	require.NoError(t, DB.Create(&UserProfile{
		UserId:     2204,
		IsVvip:     false,
		VvipStatus: VvipStatusDisabled,
	}).Error)

	activeUsers, activeTotal, err := SearchUsers("", "", VvipStatusActive, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), activeTotal)
	assert.Equal(t, 2201, activeUsers[0].Id)
	assert.True(t, activeUsers[0].IsVvip)

	disabledUsers, disabledTotal, err := SearchUsers("", "", VvipStatusDisabled, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), disabledTotal)
	assert.Equal(t, 2202, disabledUsers[0].Id)
	assert.Equal(t, VvipStatusDisabled, disabledUsers[0].VvipStatus)

	noneUsers, noneTotal, err := SearchUsers("", "", VvipStatusNone, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), noneTotal)
	assert.ElementsMatch(t, []int{2203, 2204}, []int{noneUsers[0].Id, noneUsers[1].Id})
}
