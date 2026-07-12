package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type qiniuOfficialUsageRecordView struct {
	Id                  int     `json:"id"`
	RecordKey           string  `json:"record_key"`
	RecordType          string  `json:"record_type"`
	SourceAPI           string  `json:"source_api"`
	RecordHash          string  `json:"record_hash"`
	QiniuKey            string  `json:"qiniu_key"`
	UserId              int     `json:"user_id"`
	TokenId             int     `json:"token_id"`
	QiniuChildAccountId int     `json:"qiniu_child_account_id"`
	PeriodStart         int64   `json:"period_start"`
	PeriodEnd           int64   `json:"period_end"`
	Granularity         string  `json:"granularity"`
	ModelName           string  `json:"model_name"`
	BillingItem         string  `json:"billing_item"`
	PromptTokens        int64   `json:"prompt_tokens"`
	CompletionTokens    int64   `json:"completion_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	FeeAmount           float64 `json:"fee_amount"`
	Currency            string  `json:"currency"`
	OfficialQuota       int     `json:"official_quota"`
	AppliedQuota        int     `json:"applied_quota"`
	ApplyVersion        int     `json:"apply_version"`
	Status              string  `json:"status"`
	LastError           string  `json:"last_error"`
	RawResponse         string  `json:"raw_response"`
	FetchedAt           int64   `json:"fetched_at"`
	CreatedTime         int64   `json:"created_time"`
	UpdatedTime         int64   `json:"updated_time"`
}

type qiniuOfficialLedgerApplicationView struct {
	Id             int     `json:"id"`
	UsageRecordId  int     `json:"usage_record_id"`
	ApplyVersion   int     `json:"apply_version"`
	UserId         int     `json:"user_id"`
	TokenId        int     `json:"token_id"`
	DeltaQuota     int     `json:"delta_quota"`
	DeltaAmount    float64 `json:"delta_amount"`
	WalletFlowId   int     `json:"wallet_flow_id"`
	ConsumeLogId   int     `json:"consume_log_id"`
	IdempotencyKey string  `json:"idempotency_key"`
	Status         string  `json:"status"`
	LastError      string  `json:"last_error"`
	CreatedTime    int64   `json:"created_time"`
	UpdatedTime    int64   `json:"updated_time"`
}

func AdminListQiniuOfficialUsageRecords(c *gin.Context) {
	filter, ok := parseQiniuOfficialUsageRecordFilter(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListQiniuOfficialUsageRecords(filter, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]qiniuOfficialUsageRecordView, 0, len(records))
	for _, record := range records {
		items = append(items, toQiniuOfficialUsageRecordView(record))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminListQiniuOfficialLedgerApplications(c *gin.Context) {
	filter, ok := parseQiniuOfficialLedgerApplicationFilter(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	apps, total, err := model.ListQiniuOfficialLedgerApplications(filter, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]qiniuOfficialLedgerApplicationView, 0, len(apps))
	for _, app := range apps {
		items = append(items, toQiniuOfficialLedgerApplicationView(app))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminRetryQiniuOfficialUsageRecord(c *gin.Context) {
	recordId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "官方记录 ID 无效")
		return
	}
	result, err := service.RetryQiniuOfficialUsageRecord(c.Request.Context(), recordId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminRetryQiniuOfficialLedgerApplication(c *gin.Context) {
	applicationId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "官方 ledger 应用 ID 无效")
		return
	}
	result, err := service.RetryQiniuOfficialLedgerApplication(c.Request.Context(), applicationId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func parseQiniuOfficialUsageRecordFilter(c *gin.Context) (model.QiniuOfficialUsageRecordQuery, bool) {
	filter := model.QiniuOfficialUsageRecordQuery{
		RecordType:  c.Query("record_type"),
		Status:      c.Query("status"),
		QiniuKey:    firstNonEmptyQuery(c, "key", "qiniu_key"),
		ModelName:   c.Query("model"),
		BillingItem: c.Query("billing_item"),
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
	if filter.PeriodStart, ok = parseOptionalInt64Query(c, "period_start"); !ok {
		return filter, false
	}
	if filter.PeriodEnd, ok = parseOptionalInt64Query(c, "period_end"); !ok {
		return filter, false
	}
	if filter.PeriodStart == 0 {
		if period, ok := parseOptionalInt64Query(c, "period"); !ok {
			return filter, false
		} else {
			filter.PeriodStart = period
		}
	}
	if filter.CreatedFrom, ok = parseOptionalInt64Query(c, "created_from"); !ok {
		return filter, false
	}
	if filter.CreatedTo, ok = parseOptionalInt64Query(c, "created_to"); !ok {
		return filter, false
	}
	return filter, true
}

func parseQiniuOfficialLedgerApplicationFilter(c *gin.Context) (model.QiniuOfficialLedgerApplicationQuery, bool) {
	filter := model.QiniuOfficialLedgerApplicationQuery{
		Status: c.Query("status"),
	}
	var ok bool
	if filter.UserId, ok = parseOptionalIntQuery(c, "user_id"); !ok {
		return filter, false
	}
	if filter.TokenId, ok = parseOptionalIntQuery(c, "token_id"); !ok {
		return filter, false
	}
	if filter.UsageRecordId, ok = parseOptionalIntQuery(c, "usage_record_id"); !ok {
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

func parseOptionalIntQuery(c *gin.Context, key string) (int, bool) {
	value := c.Query(key)
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		common.ApiErrorMsg(c, key+" 参数无效")
		return 0, false
	}
	return parsed, true
}

func parseOptionalInt64Query(c *gin.Context, key string) (int64, bool) {
	value := c.Query(key)
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		common.ApiErrorMsg(c, key+" 参数无效")
		return 0, false
	}
	return parsed, true
}

func firstNonEmptyQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := c.Query(key); value != "" {
			return value
		}
	}
	return ""
}

func toQiniuOfficialUsageRecordView(record model.QiniuOfficialUsageRecord) qiniuOfficialUsageRecordView {
	return qiniuOfficialUsageRecordView{
		Id:                  record.Id,
		RecordKey:           record.RecordKey,
		RecordType:          record.RecordType,
		SourceAPI:           record.SourceAPI,
		RecordHash:          record.RecordHash,
		QiniuKey:            model.MaskTokenKey(record.QiniuKey),
		UserId:              record.UserId,
		TokenId:             record.TokenId,
		QiniuChildAccountId: record.QiniuChildAccountId,
		PeriodStart:         record.PeriodStart,
		PeriodEnd:           record.PeriodEnd,
		Granularity:         record.Granularity,
		ModelName:           record.ModelName,
		BillingItem:         record.BillingItem,
		PromptTokens:        record.PromptTokens,
		CompletionTokens:    record.CompletionTokens,
		TotalTokens:         record.TotalTokens,
		FeeAmount:           record.FeeAmount,
		Currency:            record.Currency,
		OfficialQuota:       record.OfficialQuota,
		AppliedQuota:        record.AppliedQuota,
		ApplyVersion:        record.ApplyVersion,
		Status:              record.Status,
		LastError:           service.SanitizeQiniuOfficialAdminText(record.LastError),
		RawResponse:         service.SanitizeQiniuOfficialAdminText(record.RawResponse),
		FetchedAt:           record.FetchedAt,
		CreatedTime:         record.CreatedTime,
		UpdatedTime:         record.UpdatedTime,
	}
}

func toQiniuOfficialLedgerApplicationView(app model.QiniuOfficialLedgerApplication) qiniuOfficialLedgerApplicationView {
	return qiniuOfficialLedgerApplicationView{
		Id:             app.Id,
		UsageRecordId:  app.UsageRecordId,
		ApplyVersion:   app.ApplyVersion,
		UserId:         app.UserId,
		TokenId:        app.TokenId,
		DeltaQuota:     app.DeltaQuota,
		DeltaAmount:    app.DeltaAmount,
		WalletFlowId:   app.WalletFlowId,
		ConsumeLogId:   app.ConsumeLogId,
		IdempotencyKey: app.IdempotencyKey,
		Status:         app.Status,
		LastError:      service.SanitizeQiniuOfficialAdminText(app.LastError),
		CreatedTime:    app.CreatedTime,
		UpdatedTime:    app.UpdatedTime,
	}
}
