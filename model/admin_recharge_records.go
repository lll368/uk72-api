package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// AdminTopUpRecordFilter 定义管理员普通充值记录筛选条件。
type AdminTopUpRecordFilter struct {
	UserId          int
	Email           string
	PhoneNumber     string
	TradeNo         string
	Status          string
	PaymentProvider string
	PaymentMethod   string
	CreatedFrom     int64
	CreatedTo       int64
	CompletedFrom   int64
	CompletedTo     int64
}

// AdminTopUpRecord 是管理员普通充值记录列表返回给前端的只读视图。
type AdminTopUpRecord struct {
	Id              int     `json:"id"`
	UserId          int     `json:"user_id"`
	Username        string  `json:"username"`
	DisplayName     string  `json:"display_name"`
	Email           string  `json:"email"`
	PhoneNumber     string  `json:"phone_number"`
	Amount          int64   `json:"amount"`
	Money           float64 `json:"money"`
	RechargeAmount  float64 `json:"recharge_amount"`
	PaidAmount      float64 `json:"paid_amount"`
	Discount        float64 `json:"discount"`
	TradeNo         string  `json:"trade_no"`
	PaymentMethod   string  `json:"payment_method"`
	PaymentProvider string  `json:"payment_provider"`
	CreateTime      int64   `json:"create_time"`
	CompleteTime    int64   `json:"complete_time"`
	ReversedAt      int64   `json:"reversed_at"`
	Status          string  `json:"status"`
}

// AdminVipActivationRecordFilter 定义管理员算力伙伴开通记录筛选条件。
type AdminVipActivationRecordFilter struct {
	UserId          int
	Email           string
	PhoneNumber     string
	TradeNo         string
	Status          string
	PaymentProvider string
	PaymentMethod   string
	CreatedFrom     int64
	CreatedTo       int64
	ActivatedFrom   int64
	ActivatedTo     int64
}

// AdminVipActivationRecord 是管理员算力伙伴开通记录列表返回给前端的只读视图。
type AdminVipActivationRecord struct {
	Id               int     `json:"id"`
	UserId           int     `json:"user_id"`
	Username         string  `json:"username"`
	DisplayName      string  `json:"display_name"`
	Email            string  `json:"email"`
	PhoneNumber      string  `json:"phone_number"`
	TradeNo          string  `json:"trade_no"`
	ActivationAmount float64 `json:"activation_amount"`
	PaidAmount       float64 `json:"paid_amount"`
	Discount         float64 `json:"discount"`
	PaymentProvider  string  `json:"payment_provider"`
	PaymentMethod    string  `json:"payment_method"`
	Status           string  `json:"status"`
	ProviderPayload  string  `json:"provider_payload"`
	ActivatedAt      int64   `json:"activated_at"`
	DisabledAt       int64   `json:"disabled_at"`
	DisabledBy       int     `json:"disabled_by"`
	DisableReason    string  `json:"disable_reason"`
	ActivatedBy      int     `json:"activated_by"`
	ActivationRemark string  `json:"activation_remark"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
}

// ListAdminTopUpRecords 查询管理员普通充值记录，并返回当前用户资料字段。
func ListAdminTopUpRecords(filter *AdminTopUpRecordFilter, pageInfo *common.PageInfo) (records []*AdminTopUpRecord, total int64, err error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := applyAdminTopUpRecordFilter(adminTopUpRecordBaseQuery(), filter)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err = applyAdminTopUpRecordFilter(adminTopUpRecordBaseQuery(), filter).
		Select(adminTopUpRecordSelectSQL()).
		Order("top_ups.id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&records).Error
	return records, total, err
}

// ListAdminVipActivationRecords 查询管理员算力伙伴开通记录，并返回当前用户资料字段。
func ListAdminVipActivationRecords(filter *AdminVipActivationRecordFilter, pageInfo *common.PageInfo) (records []*AdminVipActivationRecord, total int64, err error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := applyAdminVipActivationRecordFilter(adminVipActivationRecordBaseQuery(), filter)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err = applyAdminVipActivationRecordFilter(adminVipActivationRecordBaseQuery(), filter).
		Select(adminVipActivationRecordSelectSQL()).
		Order("vip_activation_records.id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&records).Error
	return records, total, err
}

func adminTopUpRecordBaseQuery() *gorm.DB {
	return DB.Table("top_ups").
		Joins("LEFT JOIN users ON users.id = top_ups.user_id").
		Joins("LEFT JOIN user_profiles ON user_profiles.user_id = top_ups.user_id")
}

func adminVipActivationRecordBaseQuery() *gorm.DB {
	return DB.Table("vip_activation_records").
		Joins("LEFT JOIN users ON users.id = vip_activation_records.user_id").
		Joins("LEFT JOIN user_profiles ON user_profiles.user_id = vip_activation_records.user_id")
}

func applyAdminTopUpRecordFilter(query *gorm.DB, filter *AdminTopUpRecordFilter) *gorm.DB {
	if filter == nil {
		return query
	}
	if filter.UserId > 0 {
		query = query.Where("top_ups.user_id = ?", filter.UserId)
	}
	query = applyAdminRechargeUserFilter(query, filter.Email, filter.PhoneNumber)
	query = applyAdminRechargeTextFilter(query, "top_ups.trade_no", filter.TradeNo)
	query = applyAdminRechargeExactFilter(query, "top_ups.status", filter.Status)
	query = applyAdminRechargeExactFilter(query, "top_ups.payment_provider", filter.PaymentProvider)
	query = applyAdminRechargeExactFilter(query, "top_ups.payment_method", filter.PaymentMethod)
	query = applyAdminRechargeTimeRange(query, "top_ups.create_time", filter.CreatedFrom, filter.CreatedTo)
	query = applyAdminRechargeTimeRange(query, "top_ups.complete_time", filter.CompletedFrom, filter.CompletedTo)
	return query
}

func applyAdminVipActivationRecordFilter(query *gorm.DB, filter *AdminVipActivationRecordFilter) *gorm.DB {
	if filter == nil {
		return query
	}
	if filter.UserId > 0 {
		query = query.Where("vip_activation_records.user_id = ?", filter.UserId)
	}
	query = applyAdminRechargeUserFilter(query, filter.Email, filter.PhoneNumber)
	query = applyAdminRechargeTextFilter(query, "vip_activation_records.trade_no", filter.TradeNo)
	query = applyAdminRechargeExactFilter(query, "vip_activation_records.status", filter.Status)
	query = applyAdminRechargeExactFilter(query, "vip_activation_records.payment_provider", filter.PaymentProvider)
	query = applyAdminRechargeExactFilter(query, "vip_activation_records.payment_method", filter.PaymentMethod)
	query = applyAdminRechargeTimeRange(query, "vip_activation_records.created_at", filter.CreatedFrom, filter.CreatedTo)
	query = applyAdminRechargeTimeRange(query, "vip_activation_records.activated_at", filter.ActivatedFrom, filter.ActivatedTo)
	return query
}

func applyAdminRechargeUserFilter(query *gorm.DB, email string, phoneNumber string) *gorm.DB {
	if email = strings.TrimSpace(email); email != "" {
		query = query.Where("users.email = ?", email)
	}
	if phoneNumber = strings.TrimSpace(phoneNumber); phoneNumber != "" {
		query = query.Where("user_profiles.phone_number = ?", phoneNumber)
	}
	return query
}

func applyAdminRechargeTextFilter(query *gorm.DB, column string, value string) *gorm.DB {
	if value = strings.TrimSpace(value); value == "" {
		return query
	}
	// 订单号按包含匹配，便于运营用部分订单号定位；输入按字面量转义，避免 LIKE 通配符被滥用。
	return query.Where(column+" LIKE ? ESCAPE '!'", "%"+escapeLikeLiteral(value)+"%")
}

func applyAdminRechargeExactFilter(query *gorm.DB, column string, value string) *gorm.DB {
	if value = strings.TrimSpace(value); value != "" {
		query = query.Where(column+" = ?", value)
	}
	return query
}

func applyAdminRechargeTimeRange(query *gorm.DB, column string, from int64, to int64) *gorm.DB {
	if from > 0 {
		query = query.Where(column+" >= ?", from)
	}
	if to > 0 {
		query = query.Where(column+" <= ?", to)
	}
	return query
}

func adminTopUpRecordSelectSQL() string {
	return strings.Join([]string{
		"top_ups.id",
		"top_ups.user_id",
		"COALESCE(users.username, '') AS username",
		"COALESCE(users.display_name, '') AS display_name",
		"COALESCE(users.email, '') AS email",
		"COALESCE(user_profiles.phone_number, '') AS phone_number",
		"top_ups.amount",
		"top_ups.money",
		"top_ups.recharge_amount",
		"top_ups.paid_amount",
		"top_ups.discount",
		"top_ups.trade_no",
		"top_ups.payment_method",
		"top_ups.payment_provider",
		"top_ups.create_time",
		"top_ups.complete_time",
		"top_ups.reversed_at",
		"top_ups.status",
	}, ", ")
}

func adminVipActivationRecordSelectSQL() string {
	return strings.Join([]string{
		"vip_activation_records.id",
		"vip_activation_records.user_id",
		"COALESCE(users.username, '') AS username",
		"COALESCE(users.display_name, '') AS display_name",
		"COALESCE(users.email, '') AS email",
		"COALESCE(user_profiles.phone_number, '') AS phone_number",
		"vip_activation_records.trade_no",
		"vip_activation_records.activation_amount",
		"vip_activation_records.paid_amount",
		"vip_activation_records.discount",
		"vip_activation_records.payment_provider",
		"vip_activation_records.payment_method",
		"vip_activation_records.status",
		"vip_activation_records.provider_payload",
		"vip_activation_records.activated_at",
		"vip_activation_records.disabled_at",
		"vip_activation_records.disabled_by",
		"vip_activation_records.disable_reason",
		"vip_activation_records.activated_by",
		"vip_activation_records.activation_remark",
		"vip_activation_records.created_at",
		"vip_activation_records.updated_at",
	}, ", ")
}
