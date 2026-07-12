package service

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePhoneNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "mainland bare number", input: "13800138000", want: "+8613800138000"},
		{name: "mainland with country code", input: "+8613800138000", want: "+8613800138000"},
		{name: "trim spaces", input: " 13800138000 ", want: "+8613800138000"},
		{name: "invalid letters", input: "+861380013800a", wantErr: true},
		{name: "invalid length", input: "1380013800", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePhoneNumber(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSendPhoneVerificationCodePersistsLatestAfterSmsSuccess(t *testing.T) {
	truncate(t)

	sender := NewFakeSmsSender()
	SetSmsSender(sender)
	t.Cleanup(func() { SetSmsSender(nil) })

	phone, err := NormalizePhoneNumber("13800138000")
	require.NoError(t, err)

	require.NoError(t, SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1"))

	assert.Len(t, sender.Messages(), 1)
	assert.Equal(t, phone, sender.Messages()[0].PhoneNumber)
	assert.Regexp(t, regexp.MustCompile(`^[0-9]{6}$`), sender.Messages()[0].Code)

	latest, err := model.GetLatestPendingPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister)
	require.NoError(t, err)
	assert.Equal(t, phone, latest.PhoneNumber)
	assert.NotEmpty(t, latest.CodeHash)
}

func TestSendPhoneVerificationCodeDoesNotPersistWhenSmsFails(t *testing.T) {
	truncate(t)

	sender := NewFakeSmsSender()
	sender.SendErr = errors.New("provider unavailable")
	SetSmsSender(sender)
	t.Cleanup(func() { SetSmsSender(nil) })

	phone, err := NormalizePhoneNumber("13800138001")
	require.NoError(t, err)

	err = SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1")
	require.Error(t, err)

	_, err = model.GetLatestPendingPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister)
	require.Error(t, err)
}

func TestSendPhoneVerificationCodeRateLimit(t *testing.T) {
	truncate(t)

	sender := NewFakeSmsSender()
	SetSmsSender(sender)
	t.Cleanup(func() { SetSmsSender(nil) })

	phone, err := NormalizePhoneNumber("13800138002")
	require.NoError(t, err)

	require.NoError(t, SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1"))

	err = SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1")
	require.ErrorIs(t, err, ErrPhoneVerificationSendTooFrequent)
	assert.Len(t, sender.Messages(), 1)
}

func TestSendPhoneVerificationCodeRateLimitsRecentlyUsedCode(t *testing.T) {
	truncate(t)

	sender := NewFakeSmsSender()
	SetSmsSender(sender)
	t.Cleanup(func() { SetSmsSender(nil) })

	phone, err := NormalizePhoneNumber("13800138012")
	require.NoError(t, err)

	require.NoError(t, SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1"))
	code := sender.Messages()[0].Code
	require.NoError(t, ConsumePhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, code))

	err = SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1")
	require.ErrorIs(t, err, ErrPhoneVerificationSendTooFrequent)
	assert.Len(t, sender.Messages(), 1)
}

type slowCountingSmsSender struct {
	count atomic.Int32
}

func (s *slowCountingSmsSender) Send(_ context.Context, _ SmsSendRequest) error {
	s.count.Add(1)
	time.Sleep(50 * time.Millisecond)
	return nil
}

func TestSendPhoneVerificationCodeConcurrentRequestsSendOnce(t *testing.T) {
	truncate(t)

	sender := &slowCountingSmsSender{}
	SetSmsSender(sender)
	t.Cleanup(func() { SetSmsSender(nil) })

	phone, err := NormalizePhoneNumber("13800138013")
	require.NoError(t, err)

	const workers = 5
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1")
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		require.ErrorIs(t, err, ErrPhoneVerificationSendTooFrequent)
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, int32(1), sender.count.Load())
}

func TestConsumePhoneVerificationCodeStateMachine(t *testing.T) {
	truncate(t)

	sender := NewFakeSmsSender()
	SetSmsSender(sender)
	t.Cleanup(func() { SetSmsSender(nil) })

	phone, err := NormalizePhoneNumber("13800138003")
	require.NoError(t, err)
	require.NoError(t, SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1"))
	code := sender.Messages()[0].Code

	require.ErrorIs(t, ConsumePhoneVerificationCode(phone, model.PhoneVerificationPurposeLogin, code), ErrPhoneVerificationCodeInvalid)
	require.ErrorIs(t, ConsumePhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "000000"), ErrPhoneVerificationCodeInvalid)

	require.NoError(t, ConsumePhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, code))
	require.ErrorIs(t, ConsumePhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, code), ErrPhoneVerificationCodeInvalid)
}

func TestConsumePhoneVerificationCodeConcurrentUseSucceedsOnce(t *testing.T) {
	truncate(t)

	sender := NewFakeSmsSender()
	SetSmsSender(sender)
	t.Cleanup(func() { SetSmsSender(nil) })

	phone, err := NormalizePhoneNumber("13800138014")
	require.NoError(t, err)
	require.NoError(t, SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1"))
	code := sender.Messages()[0].Code

	const workers = 5
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- ConsumePhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, code)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		require.ErrorIs(t, err, ErrPhoneVerificationCodeInvalid)
	}
	assert.Equal(t, 1, successes)
}

func TestConsumePhoneVerificationCodeExpiresAfterMaxAttempts(t *testing.T) {
	truncate(t)

	sender := NewFakeSmsSender()
	SetSmsSender(sender)
	t.Cleanup(func() { SetSmsSender(nil) })

	phone, err := NormalizePhoneNumber("13800138004")
	require.NoError(t, err)
	require.NoError(t, SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1"))

	for i := 0; i < PhoneVerificationMaxAttempts; i++ {
		require.ErrorIs(t, ConsumePhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "000000"), ErrPhoneVerificationCodeInvalid)
	}

	latest, err := model.GetLatestPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister)
	require.NoError(t, err)
	assert.Equal(t, model.PhoneVerificationStatusExpired, latest.Status)
	assert.Equal(t, PhoneVerificationMaxAttempts, latest.AttemptCount)
}

func TestSendPhoneVerificationCodePurposeAccountChecks(t *testing.T) {
	truncate(t)

	sender := NewFakeSmsSender()
	SetSmsSender(sender)
	t.Cleanup(func() { SetSmsSender(nil) })

	phone, err := NormalizePhoneNumber("13800138005")
	require.NoError(t, err)

	require.ErrorIs(t, SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeLogin, "127.0.0.1"), ErrPhoneNumberNotRegistered)

	user := &model.User{
		Username:    "registered_phone_user",
		Password:    "password123",
		DisplayName: "registered_phone_user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, user.Insert(0))
	require.NoError(t, model.DB.Create(&model.UserProfile{
		UserId:          user.Id,
		PhoneNumber:     &phone,
		PhoneVerifiedAt: common.GetTimestamp(),
	}).Error)

	require.ErrorIs(t, SendPhoneVerificationCode(phone, model.PhoneVerificationPurposeRegister, "127.0.0.1"), ErrPhoneNumberAlreadyRegistered)
}
