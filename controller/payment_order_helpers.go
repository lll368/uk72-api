package controller

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

func createTopUpTradeNo(userId int) string {
	tradeNo := fmt.Sprintf("%s%d", common.GetRandomString(6), common.GetTimestamp())
	return fmt.Sprintf("USR%dNO%s", userId, tradeNo)
}

func createTopUpOrderSnapshot(userId int, requestAmount int64, payMoney float64, paymentMethod string, paymentProvider string, tradeNo string) *model.TopUp {
	amount := requestAmount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(requestAmount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}
	topUp := &model.TopUp{
		UserId:          userId,
		Amount:          amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   paymentMethod,
		PaymentProvider: paymentProvider,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}
	service.ApplyTopUpSnapshot(topUp)
	return topUp
}
