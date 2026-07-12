package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContactMessageDefaultStatusAndTimestamps(t *testing.T) {
	truncateTables(t)

	message := &ContactMessage{
		Name:    "Alice",
		Phone:   "+86 13800138000",
		Message: "Need enterprise plan details",
	}

	require.NoError(t, CreateContactMessage(message))

	assert.NotZero(t, message.Id)
	assert.Equal(t, ContactMessageStatusPending, message.Status)
	assert.NotZero(t, message.CreatedAt)
	assert.NotZero(t, message.UpdatedAt)
	assert.Zero(t, message.ProcessedAt)
	assert.Zero(t, message.ProcessedBy)
}

func TestContactMessageListUpdateAndDelete(t *testing.T) {
	truncateTables(t)

	first := &ContactMessage{Name: "Alice", Phone: "13800138000"}
	second := &ContactMessage{Name: "Bob", Phone: "13900139000", Status: ContactMessageStatusContacted}
	require.NoError(t, CreateContactMessage(first))
	require.NoError(t, CreateContactMessage(second))

	pending, total, err := ListContactMessages(&common.PageInfo{Page: 1, PageSize: 10}, ContactMessageStatusPending)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, pending, 1)
	assert.Equal(t, first.Id, pending[0].Id)

	processedAt := int64(1700000000)
	updated, err := UpdateContactMessageProcessing(first.Id, ContactMessageStatusContacted, "called back", 9001, processedAt)
	require.NoError(t, err)
	assert.Equal(t, ContactMessageStatusContacted, updated.Status)
	assert.Equal(t, "called back", updated.Remark)
	assert.Equal(t, 9001, updated.ProcessedBy)
	assert.Equal(t, processedAt, updated.ProcessedAt)

	require.NoError(t, DeleteContactMessageById(first.Id))
	_, err = GetContactMessageById(first.Id)
	assert.ErrorIs(t, err, ErrContactMessageNotFound)
}
