package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitContactMessageTrimsInputAndDefaultsStatus(t *testing.T) {
	truncate(t)

	record, err := SubmitContactMessage(ContactMessageSubmitRequest{
		Name:     "  Alice  ",
		Phone:    "  +86 13800138000  ",
		Message:  "  Need enterprise plan details  ",
		ClientIP: "127.0.0.1",
	})

	require.NoError(t, err)
	assert.Equal(t, "Alice", record.Name)
	assert.Equal(t, "+86 13800138000", record.Phone)
	assert.Equal(t, "Need enterprise plan details", record.Message)
	assert.Equal(t, "127.0.0.1", record.ClientIp)
	assert.Equal(t, model.ContactMessageStatusPending, record.Status)
	assert.NotZero(t, record.CreatedAt)
}

func TestSubmitContactMessageValidatesRequiredFieldsAndPhone(t *testing.T) {
	truncate(t)

	tests := []struct {
		name string
		req  ContactMessageSubmitRequest
	}{
		{name: "missing name", req: ContactMessageSubmitRequest{Phone: "13800138000"}},
		{name: "missing phone", req: ContactMessageSubmitRequest{Name: "Alice"}},
		{name: "invalid phone", req: ContactMessageSubmitRequest{Name: "Alice", Phone: "abc"}},
		{name: "message too long", req: ContactMessageSubmitRequest{Name: "Alice", Phone: "13800138000", Message: strings.Repeat("a", ContactMessageMessageMaxLength+1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SubmitContactMessage(tt.req)
			assert.Error(t, err)
		})
	}
}

func TestUpdateContactMessageSetsProcessingFields(t *testing.T) {
	truncate(t)

	record, err := SubmitContactMessage(ContactMessageSubmitRequest{Name: "Alice", Phone: "13800138000"})
	require.NoError(t, err)

	updated, err := UpdateContactMessage(record.Id, 9001, ContactMessageUpdateRequest{
		Status: model.ContactMessageStatusUnreachable,
		Remark: "no answer",
	})

	require.NoError(t, err)
	assert.Equal(t, model.ContactMessageStatusUnreachable, updated.Status)
	assert.Equal(t, "no answer", updated.Remark)
	assert.Equal(t, 9001, updated.ProcessedBy)
	assert.NotZero(t, updated.ProcessedAt)
}

func TestUpdateContactMessageRejectsInvalidStatus(t *testing.T) {
	truncate(t)

	record, err := SubmitContactMessage(ContactMessageSubmitRequest{Name: "Alice", Phone: "13800138000"})
	require.NoError(t, err)

	_, err = UpdateContactMessage(record.Id, 9001, ContactMessageUpdateRequest{
		Status: "invalid",
		Remark: "bad status",
	})

	assert.Error(t, err)
}
