package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhoneVerificationCodeLatestPendingAndConsume(t *testing.T) {
	truncateTables(t)

	now := common.GetTimestamp()
	oldCode := &PhoneVerificationCode{
		PhoneNumber: "+8613800138000",
		CodeHash:    "old",
		Purpose:     PhoneVerificationPurposeRegister,
		ExpiresAt:   now + 600,
		Status:      PhoneVerificationStatusPending,
		CreatedAt:   now - 60,
	}
	require.NoError(t, DB.Create(oldCode).Error)

	latestCode := &PhoneVerificationCode{
		PhoneNumber: "+8613800138000",
		CodeHash:    "latest",
		Purpose:     PhoneVerificationPurposeRegister,
		ExpiresAt:   now + 600,
		Status:      PhoneVerificationStatusPending,
		CreatedAt:   now,
	}
	require.NoError(t, DB.Create(latestCode).Error)

	found, err := GetLatestPendingPhoneVerificationCode("+8613800138000", PhoneVerificationPurposeRegister)
	require.NoError(t, err)
	assert.Equal(t, latestCode.Id, found.Id)

	require.NoError(t, MarkPhoneVerificationCodeUsed(found.Id))
	require.Error(t, MarkPhoneVerificationCodeUsed(found.Id))

	used, err := GetLatestPendingPhoneVerificationCode("+8613800138000", PhoneVerificationPurposeRegister)
	require.NoError(t, err)
	assert.Equal(t, oldCode.Id, used.Id)
}

func TestPhoneVerificationCodeExpirePendingBeforeCreate(t *testing.T) {
	truncateTables(t)

	now := common.GetTimestamp()
	require.NoError(t, DB.Create(&PhoneVerificationCode{
		PhoneNumber: "+8613800138001",
		CodeHash:    "old",
		Purpose:     PhoneVerificationPurposeLogin,
		ExpiresAt:   now + 600,
		Status:      PhoneVerificationStatusPending,
		CreatedAt:   now - 30,
	}).Error)

	require.NoError(t, ExpirePendingPhoneVerificationCodes("+8613800138001", PhoneVerificationPurposeLogin))

	_, err := GetLatestPendingPhoneVerificationCode("+8613800138001", PhoneVerificationPurposeLogin)
	require.Error(t, err)
}

func TestPhoneUserProfileLookupAndResetPassword(t *testing.T) {
	truncateTables(t)

	user := &User{
		Username:    "phone_user",
		Password:    "old-password",
		DisplayName: "phone_user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, user.Insert(0))

	phone := "+8613800138002"
	require.NoError(t, DB.Create(&UserProfile{
		UserId:          user.Id,
		PhoneNumber:     &phone,
		PhoneVerifiedAt: time.Now().Unix(),
	}).Error)

	assert.True(t, IsPhoneNumberAlreadyTaken(phone))

	found, err := GetUserByPhoneNumber(phone)
	require.NoError(t, err)
	assert.Equal(t, user.Id, found.Id)

	require.NoError(t, ResetUserPasswordByPhoneNumber(phone, "new-password"))

	reloaded := &User{Username: "phone_user", Password: "new-password"}
	require.NoError(t, reloaded.ValidateAndFill())
}
