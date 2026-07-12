package common

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateNumericVerificationCodeReturnsFixedLengthDigits(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9]{6}$`)

	for i := 0; i < 100; i++ {
		code, err := GenerateNumericVerificationCode(VerificationCodeLength)
		require.NoError(t, err)
		require.Truef(t, pattern.MatchString(code), "code %q must be six numeric digits", code)
	}
}

func TestGenerateNumericVerificationCodeRejectsInvalidLength(t *testing.T) {
	code, err := GenerateNumericVerificationCode(0)

	require.Error(t, err)
	require.Empty(t, code)
}
