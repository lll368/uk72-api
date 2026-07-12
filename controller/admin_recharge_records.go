package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func buildAdminTopUpRecordFilter(c *gin.Context) *model.AdminTopUpRecordFilter {
	return &model.AdminTopUpRecordFilter{
		UserId:          parseAdminRechargeIntQuery(c, "user_id"),
		Email:           c.Query("email"),
		PhoneNumber:     c.Query("phone_number"),
		TradeNo:         firstAdminRechargeQuery(c, "trade_no", "keyword"),
		Status:          c.Query("status"),
		PaymentProvider: c.Query("payment_provider"),
		PaymentMethod:   c.Query("payment_method"),
		CreatedFrom:     parseAdminRechargeInt64Query(c, "created_from", "create_from"),
		CreatedTo:       parseAdminRechargeInt64Query(c, "created_to", "create_to"),
		CompletedFrom:   parseAdminRechargeInt64Query(c, "completed_from", "complete_from"),
		CompletedTo:     parseAdminRechargeInt64Query(c, "completed_to", "complete_to"),
	}
}

func buildAdminVipActivationRecordFilter(c *gin.Context) *model.AdminVipActivationRecordFilter {
	return &model.AdminVipActivationRecordFilter{
		UserId:          parseAdminRechargeIntQuery(c, "user_id"),
		Email:           c.Query("email"),
		PhoneNumber:     c.Query("phone_number"),
		TradeNo:         firstAdminRechargeQuery(c, "trade_no", "keyword"),
		Status:          c.Query("status"),
		PaymentProvider: c.Query("payment_provider"),
		PaymentMethod:   c.Query("payment_method"),
		CreatedFrom:     parseAdminRechargeInt64Query(c, "created_from", "create_from"),
		CreatedTo:       parseAdminRechargeInt64Query(c, "created_to", "create_to"),
		ActivatedFrom:   parseAdminRechargeInt64Query(c, "activated_from", "activate_from"),
		ActivatedTo:     parseAdminRechargeInt64Query(c, "activated_to", "activate_to"),
	}
}

func firstAdminRechargeQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return ""
}

func parseAdminRechargeIntQuery(c *gin.Context, key string) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func parseAdminRechargeInt64Query(c *gin.Context, keys ...string) int64 {
	value := firstAdminRechargeQuery(c, keys...)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}
