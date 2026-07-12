package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSmsProviderDevIsUnsupported(t *testing.T) {
	oldDebugEnabled := common.DebugEnabled
	oldSmsEnabled := common.SmsEnabled
	oldSmsProvider := common.SmsProvider
	t.Cleanup(func() {
		common.DebugEnabled = oldDebugEnabled
		common.SmsEnabled = oldSmsEnabled
		common.SmsProvider = oldSmsProvider
		SetSmsSender(nil)
	})

	common.DebugEnabled = true
	common.SmsEnabled = true
	common.SmsProvider = "dev"
	SetSmsSender(nil)

	err := getSmsSender().Send(context.Background(), SmsSendRequest{
		PhoneNumber: "+8613800138000",
		Code:        "123456",
		Purpose:     "register",
	})

	require.ErrorContains(t, err, "unsupported SMS provider: dev")
}
