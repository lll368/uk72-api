package controller

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/waffo-com/waffo-go/core"
)

func parseMajorPaymentAmount(value string) float64 {
	amount, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || amount <= 0 {
		return 0
	}
	return amount
}

func parseMinorPaymentAmount(amount int, currency string) float64 {
	if amount <= 0 {
		return 0
	}
	if zeroDecimalCurrencies[strings.ToUpper(strings.TrimSpace(currency))] {
		return float64(amount)
	}
	return decimal.NewFromInt(int64(amount)).Div(decimal.NewFromInt(100)).InexactFloat64()
}

func waffoActualPaidAmount(result *core.PaymentNotificationResult) float64 {
	if result == nil {
		return 0
	}
	if amount := parseMajorPaymentAmount(result.FinalDealAmount); amount > 0 {
		return amount
	}
	return parseMajorPaymentAmount(result.OrderAmount)
}
