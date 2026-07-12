/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { StatusBadgeProps } from '@/components/status-badge'
import {
  BILLING_PRICING_VARS,
  normalizeTierLabel,
  parseTiersFromExpr,
  type ParsedTier,
} from '@/features/pricing/lib/billing-expr'
import {
  formatQiniuMarketPriceItems,
  isMarketPricingModel,
  sanitizeSupplierBrandText,
} from '@/features/pricing/lib/qiniu-market'
import type { PricingModel } from '@/features/pricing/types'
import type { UsageLog } from '../data/schema'
import type { LogOtherData } from '../types'

export { normalizeTierLabel }

const PARAM_OVERRIDE_ACTION_MAP: Record<string, string> = {
  set: 'Set',
  delete: 'Delete',
  copy: 'Copy',
  move: 'Move',
  append: 'Append',
  prepend: 'Prepend',
  trim_prefix: 'Trim Prefix',
  trim_suffix: 'Trim Suffix',
  ensure_prefix: 'Ensure Prefix',
  ensure_suffix: 'Ensure Suffix',
  trim_space: 'Trim Space',
  to_lower: 'To Lower',
  to_upper: 'To Upper',
  replace: 'Replace',
  regex_replace: 'Regex Replace',
  set_header: 'Set Header',
  delete_header: 'Delete Header',
  copy_header: 'Copy Header',
  move_header: 'Move Header',
  pass_headers: 'Pass Headers',
  sync_fields: 'Sync Fields',
  return_error: 'Return Error',
}

/**
 * Get localized label for a param override action
 */
export function getParamOverrideActionLabel(
  action: string,
  t: (key: string) => string
): string {
  const key = PARAM_OVERRIDE_ACTION_MAP[action.toLowerCase()]
  return key ? t(key) : action
}

/**
 * Parse a param override audit line into action and content
 */
export function parseAuditLine(
  line: string
): { action: string; content: string } | null {
  if (typeof line !== 'string') return null
  const firstSpace = line.indexOf(' ')
  if (firstSpace <= 0) return { action: line, content: line }
  return {
    action: line.slice(0, firstSpace),
    content: line.slice(firstSpace + 1),
  }
}

/**
 * Check if the log is a violation fee log
 */
export function isViolationFeeLog(other: LogOtherData | null): boolean {
  if (!other) return false
  return (
    other.violation_fee === true ||
    Boolean(other.violation_fee_code) ||
    Boolean(other.violation_fee_marker)
  )
}

/**
 * Check whether a usage log is a local request observation that waits for
 * Qiniu official ledger reconciliation instead of using local real-time quota.
 */
export function isQiniuLocalOfficialLedgerObservation(
  other: LogOtherData | null | undefined
): boolean {
  return (
    other?.billing_source === 'qiniu_official_ledger' &&
    other.qiniu_official_ledger_pending === true &&
    other.qiniu_official_ledger_log !== true
  )
}

/**
 * Check whether a usage log is a synthetic log generated from Qiniu official
 * ledger sync results.
 */
export function isQiniuOfficialLedgerLog(
  other: LogOtherData | null | undefined
): boolean {
  return (
    other?.billing_source === 'qiniu_official_ledger' &&
    other.qiniu_official_ledger_log === true
  )
}

export function isQiniuMarketRealtimeLog(
  other: LogOtherData | null | undefined
): boolean {
  return (
    other?.billing_source === 'qiniu_market_realtime' &&
    other.price_source === 'qiniu_market'
  )
}

export function isQiniuCostDetailBucketLog(
  other: LogOtherData | null | undefined
): boolean {
  return other?.billing_source === 'qiniu_cost_detail_bucket'
}

export function sanitizeQiniuOfficialLedgerContent(content: string): string {
  return content.replace(/^(?:七牛)?官方用量同步(?:消费|退款)：/, '')
}

export function getUsageLogDisplayContent(
  log: UsageLog,
  other: LogOtherData | null | undefined
): string {
  const content = log.content ?? ''
  if (isQiniuMarketRealtimeLog(other)) {
    const label = '市场价实时扣费'
    const normalizedContent = content.replace(/七牛市场价实时扣费/g, label)
    return normalizedContent.includes(label)
      ? normalizedContent
      : `${label} · ${normalizedContent}`
  }
  if (isQiniuCostDetailBucketLog(other)) {
    const label = '账单延迟对账调整'
    const normalizedContent = content.replace(
      /七牛 cost-detail 延迟对账/g,
      '账单延迟对账'
    )
    return normalizedContent.includes(label)
      ? normalizedContent
      : `${label} · ${normalizedContent}`
  }
  if (!isQiniuOfficialLedgerLog(other)) return content
  return sanitizeQiniuOfficialLedgerContent(content)
}

export type QiniuOfficialLedgerPriceItem = {
  detailKey: 'input' | 'output'
  formatted: string
}

export function getQiniuOfficialLedgerPriceItems(
  model: PricingModel | null | undefined
): QiniuOfficialLedgerPriceItem[] {
  if (!model) return []

  const prices: QiniuOfficialLedgerPriceItem[] = []
  const seen = new Set<string>()
  formatQiniuMarketPriceItems(model, { tokenUnit: 'M' }).forEach((item) => {
    if (item.detailKey !== 'input' && item.detailKey !== 'output') return
    if (seen.has(item.detailKey)) return
    seen.add(item.detailKey)
    prices.push({
      detailKey: item.detailKey,
      formatted: item.formatted,
    })
  })
  return prices
}

export type QiniuMarketRealtimePriceItem = {
  detailKey: 'input' | 'output' | 'unit'
  formatted: string
}

export function getQiniuMarketRealtimePriceItems(
  other: LogOtherData | null | undefined
): QiniuMarketRealtimePriceItem[] {
  if (!isQiniuMarketRealtimeLog(other)) return []
  const prices: QiniuMarketRealtimePriceItem[] = []
  const unit = formatQiniuMarketRealtimeCNYUnitPrice(
    other?.qiniu_market_unit_price,
    other?.qiniu_market_unit_size,
    other?.qiniu_market_unit_name
  )
  if (unit) {
    prices.push({ detailKey: 'unit', formatted: unit })
    return prices
  }
  const input = formatQiniuMarketRealtimeCNYTokenPrice(
    other?.qiniu_market_input_unit_price,
    other?.qiniu_market_input_unit_size
  )
  if (input) {
    prices.push({ detailKey: 'input', formatted: input })
  }
  const output = formatQiniuMarketRealtimeCNYTokenPrice(
    other?.qiniu_market_output_unit_price,
    other?.qiniu_market_output_unit_size
  )
  if (output) {
    prices.push({ detailKey: 'output', formatted: output })
  }
  return prices
}

export function getQiniuMarketRealtimeBillingModeLabel(
  other: LogOtherData | null | undefined
): 'Per-call' | 'Per-token' {
  return other?.qiniu_market_billing_mode === 'unit' ? 'Per-call' : 'Per-token'
}

export function findQiniuMarketPricingModel(
  log: UsageLog,
  other: LogOtherData | null | undefined,
  models: PricingModel[]
): PricingModel | null {
  const candidates = [other?.upstream_model_name, log.model_name]
    .map(normalizeQiniuModelLookupKey)
    .filter(Boolean)

  if (candidates.length === 0) return null

  return (
    models.find((model) => {
      if (!isMarketPricingModel(model)) return false
      const modelKeys = [
        model.market_pricing?.id,
        model.market_pricing?.name,
        model.qiniu_market?.id,
        model.qiniu_market?.name,
        model.model_name,
        model.key,
        model.id != null ? String(model.id) : undefined,
      ]
        .map(normalizeQiniuModelLookupKey)
        .filter(Boolean)

      return candidates.some((candidate) => modelKeys.includes(candidate))
    }) ?? null
  )
}

function normalizeQiniuModelLookupKey(value: unknown): string {
  if (typeof value !== 'string') return ''
  return sanitizeSupplierBrandText(value).trim().toLowerCase()
}

function formatQiniuMarketRealtimeCNYTokenPrice(
  unitPrice: number | null | undefined,
  unitSize: number | null | undefined
): string {
  if (unitPrice == null || unitSize == null || unitSize <= 0) return ''
  const value = Number(unitPrice)
  const size = Number(unitSize)
  if (!Number.isFinite(value) || !Number.isFinite(size)) return ''
  return `¥${stripQiniuRealtimeTrailingZeros((value * 1_000) / size)}/k`
}

function formatQiniuMarketRealtimeCNYUnitPrice(
  unitPrice: number | null | undefined,
  unitSize: number | null | undefined,
  unitName: string | null | undefined
): string {
  if (unitPrice == null || unitSize == null || unitSize <= 0) return ''
  const value = Number(unitPrice)
  const size = Number(unitSize)
  if (!Number.isFinite(value) || !Number.isFinite(size)) return ''
  const normalizedUnitName =
    typeof unitName === 'string' && unitName.trim() !== ''
      ? unitName.trim()
      : 'unit'
  const unitLabel =
    size === 1 ? normalizedUnitName : `${size} ${normalizedUnitName}`
  return `¥${stripQiniuRealtimeTrailingZeros(value)}/${unitLabel}`
}

function stripQiniuRealtimeTrailingZeros(value: number): string {
  if (value === 0) return '0'
  if (Math.abs(value) >= 1) {
    return value.toFixed(6).replace(/\.?0+$/, '')
  }
  return value.toPrecision(6).replace(/\.?0+$/, '')
}

/**
 * Parse the 'other' field from JSON string to object
 */
export function parseLogOther(other: string): LogOtherData | null {
  if (!other) return null
  try {
    return JSON.parse(other) as LogOtherData
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to parse log other field:', error)
    return null
  }
}

/**
 * Get time color based on duration (in seconds)
 */
export function getTimeColor(
  seconds: number
): 'success' | 'warning' | 'danger' {
  if (seconds < 10) return 'success'
  if (seconds < 30) return 'warning'
  return 'danger'
}

/**
 * Get first-response-token color based on latency (in seconds)
 */
export function getFirstResponseTimeColor(
  seconds: number
): 'success' | 'warning' | 'danger' {
  if (seconds < 5) return 'success'
  if (seconds < 10) return 'warning'
  return 'danger'
}

/**
 * Get throughput color based on generated tokens per second
 */
export function getThroughputColor(
  tokensPerSecond: number
): 'success' | 'warning' | 'danger' {
  if (tokensPerSecond >= 30) return 'success'
  if (tokensPerSecond >= 15) return 'warning'
  return 'danger'
}

/**
 * Get response color using throughput only when enough output tokens exist.
 */
export function getResponseTimeColor(
  seconds: number,
  completionTokens: number
): 'success' | 'warning' | 'danger' {
  if (completionTokens < 100 || seconds <= 0) return getTimeColor(seconds)
  return getThroughputColor(completionTokens / seconds)
}

/**
 * Format model name with mapping indicator
 */
export function formatModelName(log: UsageLog): {
  name: string
  isMapped: boolean
  actualModel?: string
} {
  const other = parseLogOther(log.other)
  const isMapped = !!(
    other?.is_model_mapped &&
    other?.upstream_model_name &&
    other.upstream_model_name !== ''
  )
  const displayModelName =
    log.model_name === 'qiniu_cost_detail_bucket'
      ? 'billing-settlement'
      : log.model_name

  return {
    name: displayModelName,
    isMapped,
    actualModel: isMapped ? other.upstream_model_name : undefined,
  }
}

/**
 * Decode a base64-encoded billing expression. Safely returns an empty string
 * when the input is missing or malformed (e.g. legacy logs without expr_b64).
 */
export function decodeBillingExprB64(exprB64: string | undefined): string {
  if (!exprB64) return ''
  try {
    const binaryString =
      typeof window !== 'undefined'
        ? window.atob(exprB64)
        : Buffer.from(exprB64, 'base64').toString('binary')
    const bytes = new Uint8Array(binaryString.length)

    for (let i = 0; i < binaryString.length; i++) {
      bytes[i] = binaryString.charCodeAt(i)
    }

    if (typeof TextDecoder !== 'undefined') {
      return new TextDecoder().decode(bytes)
    }

    return decodeURIComponent(
      Array.prototype.map
        .call(bytes, (byte: number) => '%' + byte.toString(16).padStart(2, '0'))
        .join('')
    )
  } catch {
    return ''
  }
}

/**
 * Resolve which parsed tier corresponds to the matched_tier label in a log
 * entry. Missing or unknown labels do not fall back to another tier because
 * that would display guessed unit prices.
 */
export function resolveMatchedTier(
  tiers: ParsedTier[],
  matchedLabel: string | undefined
): ParsedTier | null {
  if (tiers.length === 0) return null
  if (!matchedLabel) return null
  const found = tiers.find((tier) => {
    const l1 = normalizeTierLabel(tier.label)
    const l2 = normalizeTierLabel(matchedLabel)
    return l1 === l2 && l1 !== ''
  })
  return found || null
}

/**
 * Tiered pricing summary derived from an `other` log payload using the
 * billing-expression library. Returns null when the entry is not a tiered
 * billing log or the expression failed to parse.
 */
export interface TieredBillingSummary {
  tiers: ParsedTier[]
  tier: ParsedTier
  priceEntries: Array<{ field: string; shortLabel: string; price: number }>
}

/**
 * Whether the request payload reports any cache-related token usage. Used to
 * suppress cache pricing rows from the tiered breakdown when the request did
 * not exercise the cache path (mirrors the classic frontend behaviour).
 */
export function hasAnyCacheTokens(
  other: LogOtherData | null | undefined
): boolean {
  if (!other) return false
  return (
    (other.cache_tokens || 0) > 0 ||
    (other.cache_creation_tokens || 0) > 0 ||
    (other.cache_creation_tokens_5m || 0) > 0 ||
    (other.cache_creation_tokens_1h || 0) > 0
  )
}

export function getTieredBillingSummary(
  other: LogOtherData | null
): TieredBillingSummary | null {
  if (!other || other.billing_mode !== 'tiered_expr') return null
  const exprStr = decodeBillingExprB64(other.expr_b64)
  if (!exprStr) return null
  const tiers = parseTiersFromExpr(exprStr)
  const tier = resolveMatchedTier(tiers, other.matched_tier)
  if (!tier) return null

  const cacheTokensPresent = hasAnyCacheTokens(other)

  const priceEntries: TieredBillingSummary['priceEntries'] = []
  for (const v of BILLING_PRICING_VARS) {
    if (!v.field) continue
    if (v.group === 'cache' && !cacheTokensPresent) continue
    const raw = tier[v.field as keyof ParsedTier]
    const price = Number(raw)
    if (Number.isFinite(price) && price > 0) {
      priceEntries.push({
        field: v.field,
        shortLabel: v.shortLabel,
        price,
      })
    }
  }
  return { tiers, tier, priceEntries }
}

/**
 * Calculate duration and return formatted result with color variant
 * @param submitTime - Submit timestamp
 * @param finishTime - Finish timestamp
 * @param unit - Unit of the timestamps ('seconds' or 'milliseconds')
 */
export function formatDuration(
  submitTime?: number,
  finishTime?: number,
  unit: 'seconds' | 'milliseconds' = 'milliseconds'
): { durationSec: number; variant: StatusBadgeProps['variant'] } | null {
  if (!submitTime || !finishTime) return null

  const durationSec =
    unit === 'milliseconds'
      ? (finishTime - submitTime) / 1000
      : finishTime - submitTime

  return { durationSec, variant: durationSec > 60 ? 'red' : 'green' }
}
