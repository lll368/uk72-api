package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripeVipActivationUnitAmountRoundsToCents(t *testing.T) {
	unitAmount, err := stripeVipActivationUnitAmount(19.99)

	require.NoError(t, err)
	assert.Equal(t, int64(1999), unitAmount)
}

func TestStripeVipActivationUnitAmountRejectsNonPositiveAmount(t *testing.T) {
	_, err := stripeVipActivationUnitAmount(0)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Stripe支付金额")
}
