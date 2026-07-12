package setting

const (
	WechatPayGatewayProduction = "https://api.mch.weixin.qq.com"
)

var WechatPayEnabled = false
var WechatPaySandbox = false
var WechatPayAppId = ""
var WechatPayMchId = ""
var WechatPayMerchantSerialNo = ""
var WechatPayMerchantPrivateKey = ""
var WechatPayAPIv3Key = ""
var WechatPayPlatformSerialNo = ""
var WechatPayPlatformPublicKey = ""
var WechatPayUnitPrice = 7.3
var WechatPayMinTopUp = 1
var WechatPayNotifyUrl = ""

func GetWechatPayGatewayURL() string {
	return WechatPayGatewayProduction
}
