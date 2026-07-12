package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type qiniuCostDetailRecordView struct {
	Id                  int     `json:"id"`
	QiniuMaskedKey      string  `json:"qiniu_masked_key"`
	KeyPrefix           string  `json:"key_prefix"`
	KeySuffix           string  `json:"key_suffix"`
	BillingDate         string  `json:"billing_date"`
	ModelName           string  `json:"model_name"`
	BillingItem         string  `json:"billing_item"`
	UsageCount          float64 `json:"usage_count"`
	UsageUnit           string  `json:"usage_unit"`
	FeeAmount           float64 `json:"fee_amount"`
	Currency            string  `json:"currency"`
	RecordHash          string  `json:"record_hash"`
	RawResponse         string  `json:"raw_response"`
	OwnerStatus         string  `json:"owner_status"`
	UserId              int     `json:"user_id"`
	TokenId             int     `json:"token_id"`
	QiniuChildAccountId int     `json:"qiniu_child_account_id"`
	RetryCount          int     `json:"retry_count"`
	LastRetryTime       int64   `json:"last_retry_time"`
	NextRetryTime       int64   `json:"next_retry_time"`
	LastError           string  `json:"last_error"`
	CreatedTime         int64   `json:"created_time"`
	UpdatedTime         int64   `json:"updated_time"`
}

type qiniuBillingBucketView struct {
	Id                     int     `json:"id"`
	UserId                 int     `json:"user_id"`
	TokenId                int     `json:"token_id"`
	QiniuChildAccountId    int     `json:"qiniu_child_account_id"`
	BillingDate            string  `json:"billing_date"`
	QiniuMaskedKey         string  `json:"qiniu_masked_key"`
	KeyFingerprint         string  `json:"key_fingerprint"`
	OwnerStatus            string  `json:"owner_status"`
	OfficialAmount         float64 `json:"official_amount"`
	OfficialQuota          int     `json:"official_quota"`
	PreviousOfficialAmount float64 `json:"previous_official_amount"`
	PreviousOfficialQuota  int     `json:"previous_official_quota"`
	LocalRealtimeQuota     int     `json:"local_realtime_quota"`
	LocalRealtimeStatus    string  `json:"local_realtime_status"`
	AppliedDeltaQuota      int     `json:"applied_delta_quota"`
	PendingDeltaQuota      int     `json:"pending_delta_quota"`
	ApplyVersion           int     `json:"apply_version"`
	Status                 string  `json:"status"`
	LastError              string  `json:"last_error"`
	RetryCount             int     `json:"retry_count"`
	LastRetryTime          int64   `json:"last_retry_time"`
	NextRetryTime          int64   `json:"next_retry_time"`
	CreatedTime            int64   `json:"created_time"`
	UpdatedTime            int64   `json:"updated_time"`
}

type qiniuBillingBucketItemView struct {
	Id           int     `json:"id"`
	BucketId     int     `json:"bucket_id"`
	ModelName    string  `json:"model_name"`
	BillingItem  string  `json:"billing_item"`
	UsageCount   float64 `json:"usage_count"`
	FeeAmount    float64 `json:"fee_amount"`
	Currency     string  `json:"currency"`
	RawRecordIds string  `json:"raw_record_ids"`
	CreatedTime  int64   `json:"created_time"`
	UpdatedTime  int64   `json:"updated_time"`
}

type qiniuBillingBucketApplicationView struct {
	Id                 int     `json:"id"`
	BucketId           int     `json:"bucket_id"`
	ApplyVersion       int     `json:"apply_version"`
	DeltaQuota         int     `json:"delta_quota"`
	DeltaAmount        float64 `json:"delta_amount"`
	WalletFlowId       int     `json:"wallet_flow_id"`
	ConsumeLogId       int     `json:"consume_log_id"`
	IdempotencyKey     string  `json:"idempotency_key"`
	BalanceBeforeQuota int     `json:"balance_before_quota"`
	BalanceAfterQuota  int     `json:"balance_after_quota"`
	DebtQuota          int     `json:"debt_quota"`
	Status             string  `json:"status"`
	LastError          string  `json:"last_error"`
	RetryCount         int     `json:"retry_count"`
	LastRetryTime      int64   `json:"last_retry_time"`
	NextRetryTime      int64   `json:"next_retry_time"`
	OperationSource    string  `json:"operation_source"`
	CreatedTime        int64   `json:"created_time"`
	UpdatedTime        int64   `json:"updated_time"`
}

type qiniuBillingSummaryView struct {
	UnmappedCount            int     `json:"unmapped_count"`
	AmbiguousCount           int     `json:"ambiguous_count"`
	FailedApplicationCount   int     `json:"failed_application_count"`
	AffectedQuota            int     `json:"affected_quota"`
	AffectedAmount           float64 `json:"affected_amount"`
	LatestError              string  `json:"latest_error"`
	LatestSuccessfulSyncTime int64   `json:"latest_successful_sync_time"`
	LatestRetryResult        string  `json:"latest_retry_result"`
}

type qiniuBillingBucketAdminRequest struct {
	Reason string `json:"reason"`
}

type qiniuCostDetailResolveRequest struct {
	TokenId int    `json:"token_id"`
	Reason  string `json:"reason"`
}

func AdminGetQiniuBillingSummary(c *gin.Context) {
	summary := service.GetQiniuCostDetailAlertSummary()
	common.ApiSuccess(c, qiniuBillingSummaryView{
		UnmappedCount:            summary.UnmappedCount,
		AmbiguousCount:           summary.AmbiguousCount,
		FailedApplicationCount:   summary.FailedApplicationCount,
		AffectedQuota:            summary.AffectedQuota,
		AffectedAmount:           summary.AffectedAmount,
		LatestError:              service.SanitizeQiniuOfficialAdminText(summary.LatestError),
		LatestSuccessfulSyncTime: summary.LatestSuccessfulSyncTime,
		LatestRetryResult:        service.SanitizeQiniuOfficialAdminText(summary.LatestRetryResult),
	})
}

func AdminListQiniuBillingBuckets(c *gin.Context) {
	filter, ok := parseQiniuBillingBucketFilter(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	buckets, total, err := model.ListQiniuBillingBuckets(filter, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]qiniuBillingBucketView, 0, len(buckets))
	for _, bucket := range buckets {
		items = append(items, toQiniuBillingBucketView(bucket))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminListQiniuCostDetailRecords(c *gin.Context) {
	filter, ok := parseQiniuCostDetailRecordFilter(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListQiniuCostDetailRecords(filter, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]qiniuCostDetailRecordView, 0, len(records))
	for _, record := range records {
		items = append(items, toQiniuCostDetailRecordView(record))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminListQiniuBillingBucketItems(c *gin.Context) {
	filter, ok := parseQiniuBillingBucketItemFilter(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListQiniuBillingBucketItems(filter, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]qiniuBillingBucketItemView, 0, len(items))
	for _, item := range items {
		views = append(views, toQiniuBillingBucketItemView(item))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(views)
	common.ApiSuccess(c, pageInfo)
}

func AdminListQiniuBillingBucketApplications(c *gin.Context) {
	filter, ok := parseQiniuBillingBucketApplicationFilter(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	apps, total, err := model.ListQiniuBillingBucketApplications(filter, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]qiniuBillingBucketApplicationView, 0, len(apps))
	for _, app := range apps {
		items = append(items, toQiniuBillingBucketApplicationView(app))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminRecalculateQiniuBillingBucket(c *gin.Context) {
	bucketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "账单 bucket ID 无效")
		return
	}
	req, ok := readQiniuBillingBucketAdminRequest(c)
	if !ok {
		return
	}
	result, err := service.AdminRecalculateQiniuBillingBucket(c.Request.Context(), bucketId, c.GetInt("id"), req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminRetryQiniuBillingBucketApplication(c *gin.Context) {
	applicationId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "账单 bucket application ID 无效")
		return
	}
	req, ok := readQiniuBillingBucketAdminRequest(c)
	if !ok {
		return
	}
	result, err := service.AdminRetryQiniuBillingBucketApplication(c.Request.Context(), applicationId, c.GetInt("id"), req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminSkipQiniuBillingBucket(c *gin.Context) {
	bucketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "账单 bucket ID 无效")
		return
	}
	req, ok := readQiniuBillingBucketAdminRequest(c)
	if !ok {
		return
	}
	result, err := service.AdminSkipQiniuBillingBucket(c.Request.Context(), bucketId, c.GetInt("id"), req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminResolveQiniuBillingBucket(c *gin.Context) {
	bucketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "账单 bucket ID 无效")
		return
	}
	var req qiniuCostDetailResolveRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	result, err := service.AdminResolveQiniuBillingBucket(c.Request.Context(), service.QiniuManualBucketOwnershipInput{
		BucketId:    bucketId,
		TokenId:     req.TokenId,
		AdminUserId: c.GetInt("id"),
		Reason:      req.Reason,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminResolveQiniuCostDetailRecord(c *gin.Context) {
	rawRecordId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "cost-detail 原始记录 ID 无效")
		return
	}
	var req qiniuCostDetailResolveRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	result, err := service.AdminResolveQiniuCostDetailRecord(c.Request.Context(), service.QiniuManualOwnershipInput{
		RawRecordId: rawRecordId,
		TokenId:     req.TokenId,
		AdminUserId: c.GetInt("id"),
		Reason:      req.Reason,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func parseQiniuBillingBucketFilter(c *gin.Context) (model.QiniuBillingBucketQuery, bool) {
	filter := model.QiniuBillingBucketQuery{
		Status:         c.Query("status"),
		OwnerStatus:    c.Query("owner_status"),
		BillingDate:    c.Query("billing_date"),
		QiniuMaskedKey: firstNonEmptyQuery(c, "qiniu_masked_key", "masked_key"),
	}
	var ok bool
	if filter.UserId, ok = parseOptionalIntQuery(c, "user_id"); !ok {
		return filter, false
	}
	if filter.TokenId, ok = parseOptionalIntQuery(c, "token_id"); !ok {
		return filter, false
	}
	if filter.QiniuChildAccountId, ok = parseOptionalIntQuery(c, "qiniu_child_account_id"); !ok {
		return filter, false
	}
	if filter.CreatedFrom, ok = parseOptionalInt64Query(c, "created_from"); !ok {
		return filter, false
	}
	if filter.CreatedTo, ok = parseOptionalInt64Query(c, "created_to"); !ok {
		return filter, false
	}
	return filter, true
}

func parseQiniuCostDetailRecordFilter(c *gin.Context) (model.QiniuCostDetailRecordQuery, bool) {
	filter := model.QiniuCostDetailRecordQuery{
		OwnerStatus:    c.Query("owner_status"),
		BillingDate:    c.Query("billing_date"),
		QiniuMaskedKey: firstNonEmptyQuery(c, "qiniu_masked_key", "masked_key"),
		ModelName:      c.Query("model"),
		BillingItem:    c.Query("billing_item"),
	}
	var ok bool
	if filter.UserId, ok = parseOptionalIntQuery(c, "user_id"); !ok {
		return filter, false
	}
	if filter.TokenId, ok = parseOptionalIntQuery(c, "token_id"); !ok {
		return filter, false
	}
	if filter.QiniuChildAccountId, ok = parseOptionalIntQuery(c, "qiniu_child_account_id"); !ok {
		return filter, false
	}
	if filter.CreatedFrom, ok = parseOptionalInt64Query(c, "created_from"); !ok {
		return filter, false
	}
	if filter.CreatedTo, ok = parseOptionalInt64Query(c, "created_to"); !ok {
		return filter, false
	}
	return filter, true
}

func parseQiniuBillingBucketItemFilter(c *gin.Context) (model.QiniuBillingBucketItemQuery, bool) {
	filter := model.QiniuBillingBucketItemQuery{
		ModelName:   c.Query("model"),
		BillingItem: c.Query("billing_item"),
	}
	var ok bool
	if filter.BucketId, ok = parseOptionalIntQuery(c, "bucket_id"); !ok {
		return filter, false
	}
	if filter.CreatedFrom, ok = parseOptionalInt64Query(c, "created_from"); !ok {
		return filter, false
	}
	if filter.CreatedTo, ok = parseOptionalInt64Query(c, "created_to"); !ok {
		return filter, false
	}
	return filter, true
}

func parseQiniuBillingBucketApplicationFilter(c *gin.Context) (model.QiniuBillingBucketApplicationQuery, bool) {
	filter := model.QiniuBillingBucketApplicationQuery{
		Status: c.Query("status"),
	}
	var ok bool
	if filter.BucketId, ok = parseOptionalIntQuery(c, "bucket_id"); !ok {
		return filter, false
	}
	if filter.CreatedFrom, ok = parseOptionalInt64Query(c, "created_from"); !ok {
		return filter, false
	}
	if filter.CreatedTo, ok = parseOptionalInt64Query(c, "created_to"); !ok {
		return filter, false
	}
	return filter, true
}

func readQiniuBillingBucketAdminRequest(c *gin.Context) (qiniuBillingBucketAdminRequest, bool) {
	var req qiniuBillingBucketAdminRequest
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return req, true
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return req, false
	}
	return req, true
}

func toQiniuCostDetailRecordView(record model.QiniuCostDetailRecord) qiniuCostDetailRecordView {
	return qiniuCostDetailRecordView{
		Id:                  record.Id,
		QiniuMaskedKey:      record.QiniuMaskedKey,
		KeyPrefix:           record.KeyPrefix,
		KeySuffix:           record.KeySuffix,
		BillingDate:         record.BillingDate,
		ModelName:           record.ModelName,
		BillingItem:         record.BillingItem,
		UsageCount:          record.UsageCount,
		UsageUnit:           record.UsageUnit,
		FeeAmount:           record.FeeAmount,
		Currency:            record.Currency,
		RecordHash:          record.RecordHash,
		RawResponse:         service.SanitizeQiniuOfficialAdminText(record.RawResponse),
		OwnerStatus:         record.OwnerStatus,
		UserId:              record.UserId,
		TokenId:             record.TokenId,
		QiniuChildAccountId: record.QiniuChildAccountId,
		RetryCount:          record.RetryCount,
		LastRetryTime:       record.LastRetryTime,
		NextRetryTime:       record.NextRetryTime,
		LastError:           service.SanitizeQiniuOfficialAdminText(record.LastError),
		CreatedTime:         record.CreatedTime,
		UpdatedTime:         record.UpdatedTime,
	}
}

func toQiniuBillingBucketView(bucket model.QiniuBillingBucket) qiniuBillingBucketView {
	return qiniuBillingBucketView{
		Id:                     bucket.Id,
		UserId:                 bucket.UserId,
		TokenId:                bucket.TokenId,
		QiniuChildAccountId:    bucket.QiniuChildAccountId,
		BillingDate:            bucket.BillingDate,
		QiniuMaskedKey:         bucket.QiniuMaskedKey,
		KeyFingerprint:         bucket.KeyFingerprint,
		OwnerStatus:            bucket.OwnerStatus,
		OfficialAmount:         bucket.OfficialAmount,
		OfficialQuota:          bucket.OfficialQuota,
		PreviousOfficialAmount: bucket.PreviousOfficialAmount,
		PreviousOfficialQuota:  bucket.PreviousOfficialQuota,
		LocalRealtimeQuota:     bucket.LocalRealtimeQuota,
		LocalRealtimeStatus:    bucket.LocalRealtimeStatus,
		AppliedDeltaQuota:      bucket.AppliedDeltaQuota,
		PendingDeltaQuota:      bucket.PendingDeltaQuota,
		ApplyVersion:           bucket.ApplyVersion,
		Status:                 bucket.Status,
		LastError:              service.SanitizeQiniuOfficialAdminText(bucket.LastError),
		RetryCount:             bucket.RetryCount,
		LastRetryTime:          bucket.LastRetryTime,
		NextRetryTime:          bucket.NextRetryTime,
		CreatedTime:            bucket.CreatedTime,
		UpdatedTime:            bucket.UpdatedTime,
	}
}

func toQiniuBillingBucketItemView(item model.QiniuBillingBucketItem) qiniuBillingBucketItemView {
	return qiniuBillingBucketItemView{
		Id:           item.Id,
		BucketId:     item.BucketId,
		ModelName:    item.ModelName,
		BillingItem:  item.BillingItem,
		UsageCount:   item.UsageCount,
		FeeAmount:    item.FeeAmount,
		Currency:     item.Currency,
		RawRecordIds: item.RawRecordIds,
		CreatedTime:  item.CreatedTime,
		UpdatedTime:  item.UpdatedTime,
	}
}

func toQiniuBillingBucketApplicationView(app model.QiniuBillingBucketApplication) qiniuBillingBucketApplicationView {
	return qiniuBillingBucketApplicationView{
		Id:                 app.Id,
		BucketId:           app.BucketId,
		ApplyVersion:       app.ApplyVersion,
		DeltaQuota:         app.DeltaQuota,
		DeltaAmount:        app.DeltaAmount,
		WalletFlowId:       app.WalletFlowId,
		ConsumeLogId:       app.ConsumeLogId,
		IdempotencyKey:     app.IdempotencyKey,
		BalanceBeforeQuota: app.BalanceBeforeQuota,
		BalanceAfterQuota:  app.BalanceAfterQuota,
		DebtQuota:          app.DebtQuota,
		Status:             app.Status,
		LastError:          service.SanitizeQiniuOfficialAdminText(app.LastError),
		RetryCount:         app.RetryCount,
		LastRetryTime:      app.LastRetryTime,
		NextRetryTime:      app.NextRetryTime,
		OperationSource:    app.OperationSource,
		CreatedTime:        app.CreatedTime,
		UpdatedTime:        app.UpdatedTime,
	}
}
