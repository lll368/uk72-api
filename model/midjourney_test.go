package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestMidjourneyJSONDoesNotExposeBillingInternals(t *testing.T) {
	task := Midjourney{
		TokenId:        9101,
		BillingSource:  "qiniu_market_realtime",
		FundingSource:  "wallet",
		SubscriptionId: 9201,
	}

	data, err := common.Marshal(task)
	require.NoError(t, err)
	body := string(data)
	require.Contains(t, body, "token_id")
	require.Contains(t, body, "billing_source")
	require.NotContains(t, body, "funding_source")
	require.NotContains(t, body, "subscription_id")
}
