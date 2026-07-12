/**
此文件为旧版支付设置文件，如需增加新的参数、变量等，请在 payment_setting.go 中添加
This file is the old version of the payment settings file. If you need to add new parameters, variables, etc., please add them in payment_setting.go
*/

package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var PayAddress = ""
var CustomCallbackAddress = ""
var EpayId = ""
var EpayKey = ""
var Price = 7.3
var MinTopUp = 1
var USDExchangeRate = 7.3

var PayMethods = []map[string]string{
	{
		"name":  "支付宝",
		"color": "rgba(var(--semi-blue-5), 1)",
		"type":  "alipay",
	},
	{
		"name":  "微信",
		"color": "rgba(var(--semi-green-5), 1)",
		"type":  "wxpay",
	},
	{
		"name":      "自定义1",
		"color":     "black",
		"type":      "custom1",
		"min_topup": "50",
	},
}

const (
	retiredDebugPaymentMethodPrefix  = "mock" + "_debug_"
	retiredDebugPaymentMethodSuccess = retiredDebugPaymentMethodPrefix + "success"
	retiredDebugPaymentMethodFailed  = retiredDebugPaymentMethodPrefix + "failed"
)

func UpdatePayMethodsByJsonString(jsonString string) error {
	PayMethods = make([]map[string]string, 0)
	if err := common.Unmarshal([]byte(jsonString), &PayMethods); err != nil {
		return err
	}
	PayMethods = filterRetiredPaymentMethods(PayMethods)
	return nil
}

func PayMethods2JsonString() string {
	jsonBytes, err := common.Marshal(GetPayMethods())
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

// GetPayMethods 返回可用于真实支付链路的支付方式副本。
func GetPayMethods() []map[string]string {
	return filterRetiredPaymentMethods(PayMethods)
}

func ContainsPayMethod(method string) bool {
	if isRetiredPaymentMethod(method) {
		return false
	}
	for _, payMethod := range PayMethods {
		if isRetiredPaymentMethod(payMethod["type"]) {
			continue
		}
		if payMethod["type"] == method {
			return true
		}
	}
	return false
}

func filterRetiredPaymentMethods(payMethods []map[string]string) []map[string]string {
	filtered := make([]map[string]string, 0, len(payMethods))
	for _, payMethod := range payMethods {
		if isRetiredPaymentMethod(payMethod["type"]) {
			continue
		}
		copied := make(map[string]string, len(payMethod))
		for key, value := range payMethod {
			copied[key] = value
		}
		filtered = append(filtered, copied)
	}
	return filtered
}

func isRetiredPaymentMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case retiredDebugPaymentMethodSuccess, retiredDebugPaymentMethodFailed:
		return true
	default:
		return false
	}
}
