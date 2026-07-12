package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func isPaymentComplianceConfirmed() bool {
	return operation_setting.IsPaymentComplianceConfirmed()
}

func isStripeTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	return strings.TrimSpace(setting.StripeApiSecret) != "" &&
		strings.TrimSpace(setting.StripeWebhookSecret) != "" &&
		strings.TrimSpace(setting.StripePriceId) != ""
}

func isStripeWebhookConfigured() bool {
	return strings.TrimSpace(setting.StripeWebhookSecret) != ""
}

func isStripeWebhookEnabled() bool {
	return isStripeTopUpEnabled()
}

func isAlipayTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	if !setting.AlipayEnabled {
		return false
	}
	return strings.TrimSpace(setting.AlipayAppId) != "" &&
		strings.TrimSpace(setting.AlipayPrivateKey) != "" &&
		strings.TrimSpace(setting.AlipayPublicKey) != "" &&
		setting.AlipayUnitPrice > 0 &&
		setting.AlipayMinTopUp >= 0
}

func isAlipayWebhookConfigured() bool {
	return strings.TrimSpace(setting.AlipayAppId) != "" &&
		strings.TrimSpace(setting.AlipayPublicKey) != ""
}

func isAlipayWebhookEnabled() bool {
	return isAlipayWebhookConfigured()
}

func isWechatPayTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	if !setting.WechatPayEnabled {
		return false
	}
	return strings.TrimSpace(setting.WechatPayAppId) != "" &&
		strings.TrimSpace(setting.WechatPayMchId) != "" &&
		strings.TrimSpace(setting.WechatPayMerchantSerialNo) != "" &&
		strings.TrimSpace(setting.WechatPayMerchantPrivateKey) != "" &&
		len([]byte(strings.TrimSpace(setting.WechatPayAPIv3Key))) == 32 &&
		strings.TrimSpace(setting.WechatPayPlatformPublicKey) != "" &&
		setting.WechatPayUnitPrice > 0 &&
		setting.WechatPayMinTopUp >= 0
}

func isWechatPayWebhookConfigured() bool {
	return strings.TrimSpace(setting.WechatPayAppId) != "" &&
		strings.TrimSpace(setting.WechatPayMchId) != "" &&
		len([]byte(strings.TrimSpace(setting.WechatPayAPIv3Key))) == 32 &&
		strings.TrimSpace(setting.WechatPayPlatformPublicKey) != ""
}

func isWechatPayWebhookEnabled() bool {
	return isWechatPayWebhookConfigured()
}

func isCreemTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	products := strings.TrimSpace(setting.CreemProducts)
	return strings.TrimSpace(setting.CreemApiKey) != "" &&
		products != "" &&
		products != "[]"
}

func isCreemWebhookConfigured() bool {
	return strings.TrimSpace(setting.CreemWebhookSecret) != ""
}

func isCreemWebhookEnabled() bool {
	return isCreemTopUpEnabled() && isCreemWebhookConfigured()
}

func isWaffoTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	if !setting.WaffoEnabled {
		return false
	}

	return isWaffoWebhookConfigured()
}

func isWaffoWebhookConfigured() bool {
	if setting.WaffoSandbox {
		return strings.TrimSpace(setting.WaffoSandboxApiKey) != "" &&
			strings.TrimSpace(setting.WaffoSandboxPrivateKey) != "" &&
			strings.TrimSpace(setting.WaffoSandboxPublicCert) != ""
	}

	return strings.TrimSpace(setting.WaffoApiKey) != "" &&
		strings.TrimSpace(setting.WaffoPrivateKey) != "" &&
		strings.TrimSpace(setting.WaffoPublicCert) != ""
}

func isWaffoWebhookEnabled() bool {
	return isWaffoTopUpEnabled()
}

func isWaffoPancakeTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	if !setting.WaffoPancakeEnabled {
		return false
	}

	return isWaffoPancakeWebhookConfigured() &&
		strings.TrimSpace(setting.WaffoPancakeMerchantID) != "" &&
		strings.TrimSpace(setting.WaffoPancakePrivateKey) != "" &&
		strings.TrimSpace(setting.WaffoPancakeStoreID) != "" &&
		strings.TrimSpace(setting.WaffoPancakeProductID) != ""
}

func isWaffoPancakeWebhookConfigured() bool {
	currentWebhookKey := strings.TrimSpace(setting.WaffoPancakeWebhookPublicKey)
	if setting.WaffoPancakeSandbox {
		currentWebhookKey = strings.TrimSpace(setting.WaffoPancakeWebhookTestKey)
	}

	return currentWebhookKey != ""
}

func isWaffoPancakeWebhookEnabled() bool {
	return isWaffoPancakeTopUpEnabled()
}

func isEpayTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	return isEpayWebhookConfigured() && len(operation_setting.GetPayMethods()) > 0
}

func isEpayWebhookConfigured() bool {
	return strings.TrimSpace(operation_setting.PayAddress) != "" &&
		strings.TrimSpace(operation_setting.EpayId) != "" &&
		strings.TrimSpace(operation_setting.EpayKey) != ""
}

func isEpayWebhookEnabled() bool {
	return isEpayTopUpEnabled()
}
