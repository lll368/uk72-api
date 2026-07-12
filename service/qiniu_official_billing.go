package service

import relaycommon "github.com/QuantumNous/new-api/relay/common"

const qiniuOfficialLedgerBillingSource = "qiniu_official_ledger"

// QiniuOfficialLedgerSource 返回七牛官方账单的资金事实源标识。
func QiniuOfficialLedgerSource() string {
	return qiniuOfficialLedgerBillingSource
}

// ShouldUseQiniuOfficialLedger 判断当前请求是否应完全跳过本地实时扣费。
func ShouldUseQiniuOfficialLedger(relayInfo *relaycommon.RelayInfo) bool {
	return shouldUseQiniuOfficialLedger(relayInfo)
}

func shouldUseQiniuOfficialLedger(relayInfo *relaycommon.RelayInfo) bool {
	if relayInfo == nil || !relayInfo.QiniuManagedToken {
		return false
	}
	if relayInfo.PriceData.QiniuMarket != nil {
		return false
	}
	return IsQiniuOfficialLedgerEnabled()
}

// MarkQiniuOfficialLedgerObservation 标记本地日志仅为观测记录，真实账务等待七牛官方 ledger。
func MarkQiniuOfficialLedgerObservation(other map[string]interface{}, estimatedQuota int) {
	if other == nil {
		return
	}
	other["token_provider"] = "qiniu"
	other["billing_source"] = qiniuOfficialLedgerBillingSource
	other["qiniu_official_ledger_pending"] = true
	other["qiniu_realtime_billing_skipped"] = true
	other["local_estimated_quota"] = estimatedQuota
}

func markQiniuOfficialLedgerOther(other map[string]interface{}, relayInfo *relaycommon.RelayInfo, estimatedQuota int) {
	if other == nil || !shouldUseQiniuOfficialLedger(relayInfo) {
		return
	}
	MarkQiniuOfficialLedgerObservation(other, estimatedQuota)
}

// QiniuOfficialLedgerLogQuota 返回写入本地消费日志的额度。
// 七牛托管 Key 的真实费用以官方账单为准，本地请求日志只记录 0 额度和估算观测值。
func QiniuOfficialLedgerLogQuota(relayInfo *relaycommon.RelayInfo, estimatedQuota int) int {
	return qiniuOfficialLedgerLogQuota(relayInfo, estimatedQuota)
}

func qiniuOfficialLedgerLogQuota(relayInfo *relaycommon.RelayInfo, estimatedQuota int) int {
	if shouldUseQiniuOfficialLedger(relayInfo) {
		return 0
	}
	return estimatedQuota
}
