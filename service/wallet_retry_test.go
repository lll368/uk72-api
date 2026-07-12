package service

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestWalletTransactionRetriesMySQLDeadlock(t *testing.T) {
	attempts := 0

	err := runWalletTransactionWithRetryUsingExecutor(func(tx *gorm.DB) error {
		attempts++
		if attempts == 1 {
			return errors.New("Error 1213 (40001): Deadlock found when trying to get lock; try restarting transaction")
		}
		return nil
	}, func(operation func(tx *gorm.DB) error) error {
		return operation(nil)
	})

	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestWalletTransactionDoesNotRetryBusinessError(t *testing.T) {
	businessErr := errors.New("消费余额不足")
	attempts := 0

	err := runWalletTransactionWithRetryUsingExecutor(func(tx *gorm.DB) error {
		attempts++
		return businessErr
	}, func(operation func(tx *gorm.DB) error) error {
		return operation(nil)
	})

	require.ErrorIs(t, err, businessErr)
	assert.Equal(t, 1, attempts)
}

func TestWalletRetryableTransactionErrorRecognizesMySQLDriverErrors(t *testing.T) {
	assert.True(t, isWalletRetryableTransactionError(&mysql.MySQLError{Number: 1213}))
	assert.True(t, isWalletRetryableTransactionError(&mysql.MySQLError{Number: 1205}))
	assert.False(t, isWalletRetryableTransactionError(&mysql.MySQLError{Number: 1062}))
}
