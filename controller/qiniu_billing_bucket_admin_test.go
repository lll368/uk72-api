package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

type qiniuBillingBucketListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int                      `json:"total"`
		Items []qiniuBillingBucketView `json:"items"`
	} `json:"data"`
}

type qiniuCostDetailRecordListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int                         `json:"total"`
		Items []qiniuCostDetailRecordView `json:"items"`
	} `json:"data"`
}

type qiniuBillingBucketItemListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int                          `json:"total"`
		Items []qiniuBillingBucketItemView `json:"items"`
	} `json:"data"`
}

type qiniuBillingBucketApplicationListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int                                 `json:"total"`
		Items []qiniuBillingBucketApplicationView `json:"items"`
	} `json:"data"`
}

type qiniuBillingSummaryResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Data    qiniuBillingSummaryView `json:"data"`
}

type qiniuBillingBucketOperationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Action        string `json:"action"`
		BucketId      int    `json:"bucket_id"`
		RawRecordId   int    `json:"raw_record_id"`
		ApplicationId int    `json:"application_id"`
		TokenId       int    `json:"token_id"`
		BillingDate   string `json:"billing_date"`
		Status        string `json:"status"`
		Applied       bool   `json:"applied"`
		Skipped       bool   `json:"skipped"`
		Message       string `json:"message"`
	} `json:"data"`
}

func TestAdminListQiniuBillingBucketResourcesFiltersAndMasks(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	fullKey := "sk-" + strings.Repeat("c", 64)
	maskedKey := model.MaskTokenKey(fullKey)
	require.NoError(t, model.DB.Create(&model.QiniuCostDetailRecord{
		QiniuMaskedKey:      maskedKey,
		KeyPrefix:           "sk-cc",
		KeySuffix:           "cccc",
		BillingDate:         "2026-06-01",
		ModelName:           "deepseek-v3",
		BillingItem:         "input",
		UsageCount:          12,
		UsageUnit:           "tokens",
		FeeAmount:           1.25,
		Currency:            "CNY",
		RecordHash:          "cost-detail-admin-mask",
		RawResponse:         `{"api_key":"` + fullKey + `","ak":"qiniu-ak-secret","sk":"qiniu-sk-secret"}`,
		OwnerStatus:         model.QiniuBillingOwnerStatusResolved,
		UserId:              9401,
		TokenId:             9402,
		QiniuChildAccountId: 901,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QiniuCostDetailRecord{
		QiniuMaskedKey: maskedKey,
		KeyPrefix:      "sk-cc",
		KeySuffix:      "cccc",
		BillingDate:    "2026-06-01",
		ModelName:      "deepseek-v3",
		BillingItem:    "input",
		UsageCount:     12,
		UsageUnit:      "tokens",
		FeeAmount:      1.25,
		Currency:       "CNY",
		RecordHash:     "cost-detail-admin-summary",
		RawResponse:    `{"fee":1.25}`,
		OwnerStatus:    model.QiniuBillingOwnerStatusUnmapped,
		RetryCount:     2,
		LastRetryTime:  common.GetTimestamp(),
		NextRetryTime:  common.GetTimestamp() + 300,
		LastError:      "unmapped key " + fullKey,
	}).Error)
	bucket := &model.QiniuBillingBucket{
		UserId:              9401,
		TokenId:             9402,
		QiniuChildAccountId: 901,
		BillingDate:         "2026-06-01",
		QiniuMaskedKey:      maskedKey,
		KeyFingerprint:      "fp-admin-mask",
		OwnerStatus:         model.QiniuBillingOwnerStatusResolved,
		OfficialAmount:      1.25,
		OfficialQuota:       625000,
		LocalRealtimeQuota:  100000,
		PendingDeltaQuota:   525000,
		Status:              model.QiniuBillingBucketStatusFailed,
		LastError:           "upstream key " + fullKey + " failed",
	}
	require.NoError(t, model.DB.Create(bucket).Error)
	require.NoError(t, model.DB.Create(&model.QiniuBillingBucketItem{
		BucketId:     bucket.Id,
		ModelName:    "deepseek-v3",
		BillingItem:  "input",
		UsageCount:   12,
		FeeAmount:    1.25,
		Currency:     "CNY",
		RawRecordIds: "1",
	}).Error)
	require.NoError(t, model.DB.Create(&model.QiniuBillingBucketApplication{
		BucketId:        bucket.Id,
		ApplyVersion:    1,
		DeltaQuota:      525000,
		DeltaAmount:     1.05,
		IdempotencyKey:  "qiniu:billing_bucket:admin-mask:v1",
		Status:          model.QiniuBillingApplicationStatusFailed,
		LastError:       "application key " + fullKey + " failed",
		OperationSource: model.QiniuBillingOperationSourceAdmin,
	}).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	bucketRecorder := performQiniuAdminRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-billing-buckets?billing_date=2026-06-01&user_id=9401&token_id=9402&qiniu_child_account_id=901&status=failed&owner_status=resolved&qiniu_masked_key="+maskedKey, "", cookies)
	var bucketResp qiniuBillingBucketListResponse
	require.NoError(t, common.Unmarshal(bucketRecorder.Body.Bytes(), &bucketResp))
	require.True(t, bucketResp.Success, bucketResp.Message)
	require.Equal(t, 1, bucketResp.Data.Total)
	require.Equal(t, 901, bucketResp.Data.Items[0].QiniuChildAccountId)
	require.NotContains(t, bucketResp.Data.Items[0].LastError, fullKey)

	rawRecorder := performQiniuAdminRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-cost-detail-records?billing_date=2026-06-01&qiniu_child_account_id=901&owner_status=resolved&qiniu_masked_key="+maskedKey+"&model=deepseek-v3&billing_item=input", "", cookies)
	var rawResp qiniuCostDetailRecordListResponse
	require.NoError(t, common.Unmarshal(rawRecorder.Body.Bytes(), &rawResp))
	require.True(t, rawResp.Success, rawResp.Message)
	require.Equal(t, 1, rawResp.Data.Total)
	require.Equal(t, 901, rawResp.Data.Items[0].QiniuChildAccountId)
	require.NotContains(t, rawResp.Data.Items[0].RawResponse, fullKey)
	require.NotContains(t, rawResp.Data.Items[0].RawResponse, "qiniu-ak-secret")
	require.NotContains(t, rawResp.Data.Items[0].RawResponse, "qiniu-sk-secret")

	itemRecorder := performQiniuAdminRequest(t, router, http.MethodGet, fmt.Sprintf("/api/payment/admin/qiniu-billing-bucket-items?bucket_id=%d&model=deepseek-v3&billing_item=input", bucket.Id), "", cookies)
	var itemResp qiniuBillingBucketItemListResponse
	require.NoError(t, common.Unmarshal(itemRecorder.Body.Bytes(), &itemResp))
	require.True(t, itemResp.Success, itemResp.Message)
	require.Equal(t, 1, itemResp.Data.Total)
	require.Equal(t, bucket.Id, itemResp.Data.Items[0].BucketId)

	appRecorder := performQiniuAdminRequest(t, router, http.MethodGet, fmt.Sprintf("/api/payment/admin/qiniu-billing-bucket-applications?bucket_id=%d&status=failed", bucket.Id), "", cookies)
	var appResp qiniuBillingBucketApplicationListResponse
	require.NoError(t, common.Unmarshal(appRecorder.Body.Bytes(), &appResp))
	require.True(t, appResp.Success, appResp.Message)
	require.Equal(t, 1, appResp.Data.Total)
	require.NotContains(t, appResp.Data.Items[0].LastError, fullKey)
	require.Equal(t, model.QiniuBillingOperationSourceAdmin, appResp.Data.Items[0].OperationSource)

	summaryRecorder := performQiniuAdminRequest(t, router, http.MethodGet, "/api/payment/admin/qiniu-billing-summary", "", cookies)
	var summaryResp qiniuBillingSummaryResponse
	require.NoError(t, common.Unmarshal(summaryRecorder.Body.Bytes(), &summaryResp))
	require.True(t, summaryResp.Success, summaryResp.Message)
	require.Equal(t, 1, summaryResp.Data.UnmappedCount)
	require.Equal(t, 1, summaryResp.Data.FailedApplicationCount)
	require.Equal(t, bucket.PendingDeltaQuota, summaryResp.Data.AffectedQuota)
	require.Greater(t, summaryResp.Data.AffectedAmount, 0.0)
	require.Greater(t, summaryResp.Data.LatestSuccessfulSyncTime, int64(0))
	require.NotContains(t, summaryResp.Data.LatestError, fullKey)
	require.Contains(t, summaryResp.Data.LatestRetryResult, "unmapped=1")
}

func TestAdminOperateQiniuBillingBuckets(t *testing.T) {
	router := setupQiniuOfficialAdminControllerTest(t)
	userId := 9501
	tokenId := 9502
	tokenKey := strings.Repeat("d", 64)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          userId,
		Username:    "qiniu-bucket-user",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       int(10 * common.QuotaPerUnit),
		DisplayName: "qiniu-bucket-user",
		Group:       "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             tokenId,
		UserId:         userId,
		Name:           "qiniu-bucket-token",
		Key:            tokenKey,
		Provider:       model.TokenProviderQiniu,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	raw := &model.QiniuCostDetailRecord{
		QiniuMaskedKey: model.MaskTokenKey("sk-" + tokenKey),
		KeyPrefix:      "sk-dd",
		KeySuffix:      "dddd",
		BillingDate:    "2026-06-02",
		ModelName:      "deepseek-v3",
		BillingItem:    "output",
		UsageCount:     20,
		UsageUnit:      "tokens",
		FeeAmount:      1,
		Currency:       "CNY",
		RecordHash:     "cost-detail-admin-resolve",
		RawResponse:    `{"fee":1}`,
		OwnerStatus:    model.QiniuBillingOwnerStatusAmbiguous,
	}
	require.NoError(t, model.DB.Create(raw).Error)

	cookies := loginQiniuOfficialAdminTestUser(t, router, "admin", "8601")
	resolveRecorder := performQiniuAdminRequest(t, router, http.MethodPost, fmt.Sprintf("/api/payment/admin/qiniu-cost-detail-records/%d/resolve", raw.Id), fmt.Sprintf(`{"token_id":%d,"reason":"manual owner"}`, tokenId), cookies)
	var resolveResp qiniuBillingBucketOperationResponse
	require.NoError(t, common.Unmarshal(resolveRecorder.Body.Bytes(), &resolveResp))
	require.True(t, resolveResp.Success, resolveResp.Message)
	require.Equal(t, "manual_resolve", resolveResp.Data.Action)
	require.Equal(t, raw.Id, resolveResp.Data.RawRecordId)
	require.Greater(t, resolveResp.Data.BucketId, 0)

	var reloadedRaw model.QiniuCostDetailRecord
	require.NoError(t, model.DB.First(&reloadedRaw, "id = ?", raw.Id).Error)
	require.Equal(t, model.QiniuBillingOwnerStatusManualResolved, reloadedRaw.OwnerStatus)
	require.Equal(t, tokenId, reloadedRaw.TokenId)

	recalculateRecorder := performQiniuAdminRequest(t, router, http.MethodPost, fmt.Sprintf("/api/payment/admin/qiniu-billing-buckets/%d/recalculate", resolveResp.Data.BucketId), `{"reason":"verify recalc"}`, cookies)
	var recalculateResp qiniuBillingBucketOperationResponse
	require.NoError(t, common.Unmarshal(recalculateRecorder.Body.Bytes(), &recalculateResp))
	require.True(t, recalculateResp.Success, recalculateResp.Message)
	require.Equal(t, "recalculate", recalculateResp.Data.Action)
	require.Contains(t, recalculateResp.Data.Message, "pending_delta_quota")

	unresolvedBucket := &model.QiniuBillingBucket{
		UserId:            0,
		TokenId:           0,
		BillingDate:       "2026-06-05",
		QiniuMaskedKey:    model.MaskTokenKey("sk-" + tokenKey),
		OwnerStatus:       model.QiniuBillingOwnerStatusAmbiguous,
		OfficialQuota:     100,
		PendingDeltaQuota: 100,
		Status:            model.QiniuBillingBucketStatusNeedsReview,
	}
	require.NoError(t, model.DB.Create(unresolvedBucket).Error)
	bucketRaw := &model.QiniuCostDetailRecord{
		QiniuMaskedKey: unresolvedBucket.QiniuMaskedKey,
		KeyPrefix:      "dd",
		KeySuffix:      "dddddd",
		BillingDate:    unresolvedBucket.BillingDate,
		ModelName:      "deepseek-v3",
		BillingItem:    "input",
		UsageCount:     10,
		UsageUnit:      "tokens",
		FeeAmount:      0.5,
		Currency:       "CNY",
		RecordHash:     "cost-detail-admin-bucket-resolve",
		RawResponse:    `{"fee":0.5}`,
		OwnerStatus:    model.QiniuBillingOwnerStatusAmbiguous,
	}
	require.NoError(t, model.DB.Create(bucketRaw).Error)
	bucketResolveRecorder := performQiniuAdminRequest(t, router, http.MethodPost, fmt.Sprintf("/api/payment/admin/qiniu-billing-buckets/%d/resolve", unresolvedBucket.Id), fmt.Sprintf(`{"token_id":%d,"reason":"bucket owner"}`, tokenId), cookies)
	var bucketResolveResp qiniuBillingBucketOperationResponse
	require.NoError(t, common.Unmarshal(bucketResolveRecorder.Body.Bytes(), &bucketResolveResp))
	require.True(t, bucketResolveResp.Success, bucketResolveResp.Message)
	require.Equal(t, "manual_resolve_bucket", bucketResolveResp.Data.Action)
	require.Equal(t, tokenId, bucketResolveResp.Data.TokenId)
	var reloadedBucketRaw model.QiniuCostDetailRecord
	require.NoError(t, model.DB.First(&reloadedBucketRaw, "id = ?", bucketRaw.Id).Error)
	require.Equal(t, model.QiniuBillingOwnerStatusManualResolved, reloadedBucketRaw.OwnerStatus)
	require.Equal(t, tokenId, reloadedBucketRaw.TokenId)

	var retryBucket model.QiniuBillingBucket
	require.NoError(t, model.DB.First(&retryBucket, "id = ?", resolveResp.Data.BucketId).Error)
	require.NoError(t, model.DB.Model(&model.QiniuBillingBucket{}).Where("id = ?", retryBucket.Id).Updates(map[string]interface{}{
		"pending_delta_quota": 1,
		"status":              model.QiniuBillingBucketStatusFailed,
	}).Error)
	retryBucket.PendingDeltaQuota = 1
	retryBucket.Status = model.QiniuBillingBucketStatusFailed
	retryApplyVersion := retryBucket.ApplyVersion + 1
	failedApp := &model.QiniuBillingBucketApplication{
		BucketId:        retryBucket.Id,
		ApplyVersion:    retryApplyVersion,
		DeltaQuota:      1,
		DeltaAmount:     0.01,
		IdempotencyKey:  model.QiniuBillingBucketIdempotencyKey(retryBucket.Id, retryApplyVersion),
		Status:          model.QiniuBillingApplicationStatusFailed,
		OperationSource: model.QiniuBillingOperationSourceAdmin,
	}
	require.NoError(t, model.DB.Create(failedApp).Error)
	retryRecorder := performQiniuAdminRequest(t, router, http.MethodPost, fmt.Sprintf("/api/payment/admin/qiniu-billing-bucket-applications/%d/retry", failedApp.Id), `{"reason":"retry failed app"}`, cookies)
	var retryResp qiniuBillingBucketOperationResponse
	require.NoError(t, common.Unmarshal(retryRecorder.Body.Bytes(), &retryResp))
	require.True(t, retryResp.Success, retryResp.Message)
	require.Equal(t, "retry_application", retryResp.Data.Action)
	require.Equal(t, failedApp.Id, retryResp.Data.ApplicationId)
	require.True(t, retryResp.Data.Applied)

	skipBucket := &model.QiniuBillingBucket{
		UserId:            userId,
		TokenId:           tokenId,
		BillingDate:       "2026-06-03",
		QiniuMaskedKey:    model.MaskTokenKey("sk-" + tokenKey),
		OwnerStatus:       model.QiniuBillingOwnerStatusResolved,
		OfficialQuota:     100,
		PendingDeltaQuota: 100,
		Status:            model.QiniuBillingBucketStatusNeedsReview,
	}
	require.NoError(t, model.DB.Create(skipBucket).Error)
	skipRecorder := performQiniuAdminRequest(t, router, http.MethodPost, fmt.Sprintf("/api/payment/admin/qiniu-billing-buckets/%d/skip", skipBucket.Id), `{"reason":"accepted manual difference"}`, cookies)
	var skipResp qiniuBillingBucketOperationResponse
	require.NoError(t, common.Unmarshal(skipRecorder.Body.Bytes(), &skipResp))
	require.True(t, skipResp.Success, skipResp.Message)
	require.Equal(t, "skip", skipResp.Data.Action)
	require.True(t, skipResp.Data.Skipped)

	var reloadedBucket model.QiniuBillingBucket
	require.NoError(t, model.DB.First(&reloadedBucket, "id = ?", skipBucket.Id).Error)
	require.Equal(t, model.QiniuBillingBucketStatusSkipped, reloadedBucket.Status)
	require.Contains(t, reloadedBucket.LastError, "accepted manual difference")
}

func performQiniuAdminRequest(t *testing.T, router http.Handler, method string, path string, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	var requestBody *bytes.Buffer
	if body == "" {
		requestBody = bytes.NewBuffer(nil)
	} else {
		requestBody = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, requestBody)
	req.Header.Set("New-Api-User", "8601")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder
}
