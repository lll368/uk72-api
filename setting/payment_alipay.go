package setting

const (
	AlipayGatewayProduction = "https://openapi.alipay.com/gateway.do"
	AlipayGatewaySandbox    = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
)

var AlipayEnabled = false
var AlipaySandbox = false
var AlipayAppId = ""
var AlipayPrivateKey = ""
var AlipayPublicKey = ""
var AlipayUnitPrice = 7.3
var AlipayMinTopUp = 1
var AlipayReturnUrl = ""
var AlipayNotifyUrl = ""

func GetAlipayGatewayURL() string {
	if AlipaySandbox {
		return AlipayGatewaySandbox
	}
	return AlipayGatewayProduction
}
