package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestResolveQiniuCostDetailOwnershipStatuses(t *testing.T) {
	truncate(t)

	resolvedToken := seedQiniuOwnershipToken(t, 5201, 5201, "abc-resolved-123456")
	requireNoError(t, model.DB.Delete(resolvedToken).Error)
	resolvedRecord := seedQiniuCostDetailRawRecord(t, "raw-resolved", "sk-abc***123456", "abc", "123456", "2026-06-04", 0.25)
	result, err := ResolveQiniuCostDetailRecordOwnership(context.Background(), resolvedRecord.Id)
	requireNoError(t, err)
	if result.OwnerStatus != model.QiniuBillingOwnerStatusResolved || result.TokenId != resolvedToken.Id || result.UserId != resolvedToken.UserId {
		t.Fatalf("expected soft-deleted token to resolve ownership, got %#v", result)
	}

	unmappedRecord := seedQiniuCostDetailRawRecord(t, "raw-unmapped", "sk-zzz***000000", "zzz", "000000", "2026-06-04", 0.25)
	result, err = ResolveQiniuCostDetailRecordOwnership(context.Background(), unmappedRecord.Id)
	requireNoError(t, err)
	if result.OwnerStatus != model.QiniuBillingOwnerStatusUnmapped || result.TokenId != 0 {
		t.Fatalf("expected unmapped ownership, got %#v", result)
	}
	var reloadedUnmapped model.QiniuCostDetailRecord
	requireNoError(t, model.DB.First(&reloadedUnmapped, "id = ?", unmappedRecord.Id).Error)
	if reloadedUnmapped.RetryCount != 1 || reloadedUnmapped.LastRetryTime == 0 || reloadedUnmapped.NextRetryTime == 0 || reloadedUnmapped.LastError == "" {
		t.Fatalf("expected unmapped raw record retry context, got %#v", reloadedUnmapped)
	}

	seedQiniuOwnershipToken(t, 5202, 5202, "dup-one-999999")
	seedQiniuOwnershipToken(t, 5203, 5203, "dup-two-999999")
	ambiguousRecord := seedQiniuCostDetailRawRecord(t, "raw-ambiguous", "sk-dup***999999", "dup", "999999", "2026-06-04", 0.25)
	result, err = ResolveQiniuCostDetailRecordOwnership(context.Background(), ambiguousRecord.Id)
	requireNoError(t, err)
	if result.OwnerStatus != model.QiniuBillingOwnerStatusAmbiguous || result.CandidateCount != 2 {
		t.Fatalf("expected ambiguous ownership, got %#v", result)
	}
	var reloadedAmbiguous model.QiniuCostDetailRecord
	requireNoError(t, model.DB.First(&reloadedAmbiguous, "id = ?", ambiguousRecord.Id).Error)
	if reloadedAmbiguous.RetryCount != 1 || reloadedAmbiguous.LastRetryTime == 0 || reloadedAmbiguous.NextRetryTime == 0 || reloadedAmbiguous.LastError == "" {
		t.Fatalf("expected ambiguous raw record retry context, got %#v", reloadedAmbiguous)
	}
}

func TestResolveQiniuCostDetailOwnershipDisambiguatesWithPerKeyCostDetail(t *testing.T) {
	truncate(t)

	first := seedQiniuOwnershipToken(t, 5211, 5211, "abc-first-123456")
	second := seedQiniuOwnershipToken(t, 5212, 5212, "abc-second-123456")
	record := seedQiniuCostDetailRawRecord(t, "raw-confirmed", "sk-abc***123456", "abc", "123456", "2026-06-04", 0.25)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != qiniuAPIKeyUsagePath {
			t.Fatalf("unexpected qiniu request %s %s", r.Method, r.URL.String())
		}
		fullKey := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		fee := 0.10
		if fullKey == "sk-"+second.Key {
			fee = 0.25
		}
		if fullKey != "sk-"+first.Key && fullKey != "sk-"+second.Key {
			t.Fatalf("unexpected per-key confirmation key %q", fullKey)
		}
		writeQiniuJSON(t, w, map[string]any{
			"status": true,
			"data": map[string]any{
				"api_key": fullKey,
				"bills": []map[string]any{
					{
						"date": r.URL.Query().Get("start_date"),
						"models": []map[string]any{
							{
								"model_id": "deepseek-v3",
								"items": []map[string]any{
									{
										"name":  "deepseek-v3输入",
										"key":   "input",
										"usage": map[string]any{"count": 1.0, "unit": "k/tokens"},
										"fee":   fee,
									},
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()
	configureQiniuOfficialLedgerSettingForTest(t, server.URL)

	result, err := ResolveQiniuCostDetailRecordOwnership(context.Background(), record.Id)
	requireNoError(t, err)
	if result.OwnerStatus != model.QiniuBillingOwnerStatusResolved || result.TokenId != second.Id || result.ConfirmedCount != 1 {
		t.Fatalf("expected per-key confirmation to resolve second token, got %#v", result)
	}
}

func TestManualResolveQiniuCostDetailOwnershipRecalculatesBucket(t *testing.T) {
	truncate(t)

	token := seedQiniuOwnershipToken(t, 5221, 5221, "manual-owner-123456")
	record := seedQiniuCostDetailRawRecord(t, "raw-manual", "sk-man***123456", "man", "123456", "2026-06-04", 0.35)

	bucket, err := ManuallyResolveQiniuCostDetailRawRecordOwnership(QiniuManualOwnershipInput{
		RawRecordId: record.Id,
		TokenId:     token.Id,
		AdminUserId: 1,
		Reason:      "manual bind in test",
	})
	requireNoError(t, err)
	if bucket == nil || bucket.TokenId != token.Id || bucket.BillingDate != record.BillingDate || bucket.OfficialQuota != amountToQuota(0.35) {
		t.Fatalf("expected manual resolve to recalculate bucket, got %#v", bucket)
	}
	var updated model.QiniuCostDetailRecord
	requireNoError(t, model.DB.First(&updated, "id = ?", record.Id).Error)
	if updated.OwnerStatus != model.QiniuBillingOwnerStatusManualResolved || updated.TokenId != token.Id || updated.UserId != token.UserId {
		t.Fatalf("expected manual resolved raw record, got %#v", updated)
	}
}

func TestResolveQiniuCostDetailOwnershipPersistsTokenChildAccount(t *testing.T) {
	truncate(t)

	token := seedQiniuOwnershipToken(t, 5222, 5222, "child-owner-123456")
	requireNoError(t, model.DB.Model(&model.User{}).Where("id = ?", token.UserId).Update("qiniu_child_account_id", 9901).Error)
	requireNoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Update("qiniu_child_account_id", 3301).Error)
	token.QiniuChildAccountId = 3301
	record := seedQiniuCostDetailRawRecord(t, "raw-child-account", "sk-chi***123456", "chi", "123456", "2026-06-04", 0.35)

	result, err := ResolveQiniuCostDetailRecordOwnership(context.Background(), record.Id)
	requireNoError(t, err)
	if result.QiniuChildAccountId != token.QiniuChildAccountId {
		t.Fatalf("expected ownership result to use token child account, got %#v", result)
	}
	var updated model.QiniuCostDetailRecord
	requireNoError(t, model.DB.First(&updated, "id = ?", record.Id).Error)
	if updated.QiniuChildAccountId != token.QiniuChildAccountId {
		t.Fatalf("expected raw record to use token child account, got %#v", updated)
	}
	var bucket model.QiniuBillingBucket
	requireNoError(t, model.DB.First(&bucket, "token_id = ? AND billing_date = ?", token.Id, record.BillingDate).Error)
	if bucket.QiniuChildAccountId != token.QiniuChildAccountId {
		t.Fatalf("expected bucket to use token child account, got %#v", bucket)
	}
}

func seedQiniuOwnershipToken(t *testing.T, userId int, tokenId int, key string) *model.Token {
	t.Helper()
	requireNoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: fmt.Sprintf("qiniu_owner_user_%d", userId),
		AffCode:  fmt.Sprintf("qiniu_owner_aff_%d", userId),
		Quota:    int(100 * common.QuotaPerUnit),
		Status:   common.UserStatusEnabled,
	}).Error)
	token := &model.Token{
		Id:             tokenId,
		UserId:         userId,
		Name:           "qiniu-owner-token",
		Key:            key,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	requireNoError(t, model.DB.Create(token).Error)
	return token
}

func seedQiniuCostDetailRawRecord(t *testing.T, hash string, maskedKey string, prefix string, suffix string, billingDate string, fee float64) *model.QiniuCostDetailRecord {
	t.Helper()
	record := &model.QiniuCostDetailRecord{
		QiniuMaskedKey: maskedKey,
		KeyPrefix:      prefix,
		KeySuffix:      suffix,
		BillingDate:    billingDate,
		ModelName:      "deepseek-v3",
		BillingItem:    "input",
		UsageCount:     1,
		UsageUnit:      "k/tokens",
		FeeAmount:      fee,
		Currency:       "CNY",
		RecordHash:     hash,
		RawResponse:    `{"fee":0.25}`,
		OwnerStatus:    model.QiniuBillingOwnerStatusUnmapped,
	}
	requireNoError(t, model.DB.Create(record).Error)
	return record
}
