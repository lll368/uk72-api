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
import { DEFAULT_TOKEN_UNIT } from '../constants'
import type {
  MarketPricingModel,
  PricingModel,
  QiniuMarketPricingDetail,
  TokenUnit,
} from '../types'

export type QiniuMarketPriceDisplayItem = {
  key: string
  detailKey: string
  label: string
  formatted: string
  unitLabel: string
}

export type QiniuMarketPriceFormatOptions = {
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  priceRate?: number
  usdExchangeRate?: number
}

const DETAIL_ORDER = ['input', 'output', 'cache', 'image', 'audio']
const TOKEN_UNIT_SIZE: Record<TokenUnit, number> = {
  M: 1_000_000,
  K: 1_000,
}

export function hasQiniuMarketPricing(model: PricingModel): boolean {
  return formatQiniuMarketPriceItems(model).length > 0
}

export function isMarketPricingModel(model: PricingModel): boolean {
  return Boolean(
    model.market_pricing ||
    model.qiniu_market ||
    model.price_source === 'qiniu_market'
  )
}

export function formatQiniuMarketPriceItems(
  model: PricingModel,
  options: QiniuMarketPriceFormatOptions = {}
): QiniuMarketPriceDisplayItem[] {
  if (!isMarketPricingModel(model)) return []

  const rules = getMarketPricing(model)?.pricing_rules_v2 ?? []
  const items: QiniuMarketPriceDisplayItem[] = []
  rules.forEach((rule, ruleIndex) => {
    const details = rule.details_v2
    if (!details) return

    Object.entries(details)
      .sort(([left], [right]) => detailSortIndex(left) - detailSortIndex(right))
      .forEach(([key, detail]) => {
        const formatted = formatQiniuCNYPrice(detail, options)
        if (!formatted) return
        items.push({
          key: `${ruleIndex}-${key}`,
          detailKey: key,
          label: sanitizeSupplierBrandText(detail.name) || key,
          formatted,
          unitLabel: formatQiniuUnitLabel(detail, options),
        })
      })
  })
  return items
}

export function getQiniuMarketSortPrice(
  model: PricingModel,
  tokenUnit: TokenUnit = DEFAULT_TOKEN_UNIT
): number | null {
  if (!isMarketPricingModel(model)) return null

  let lowestPrice: number | null = null
  const rules = getMarketPricing(model)?.pricing_rules_v2 ?? []
  rules.forEach((rule) => {
    Object.values(rule.details_v2 ?? {}).forEach((detail) => {
      if (detail.unit_price === undefined || detail.unit_price === null) {
        return
      }
      const value = Number(detail.unit_price)
      if (!Number.isFinite(value)) return
      const scaledValue = value * getQiniuUnitScale(detail, tokenUnit)
      if (lowestPrice === null || scaledValue < lowestPrice) {
        lowestPrice = scaledValue
      }
    })
  })

  return lowestPrice
}

export function qiniuMarketVisibleText(model: PricingModel): string {
  const parts = [
    model.model_name,
    getMarketPricing(model)?.name,
    getMarketPricing(model)?.description,
    ...(getMarketPricing(model)?.hot_tags ?? []),
    ...(getMarketPricing(model)?.features ?? []),
    ...formatQiniuMarketPriceItems(model).flatMap((item) => [
      item.label,
      item.formatted,
      item.unitLabel,
    ]),
  ]
  return parts
    .map((part) => sanitizeSupplierBrandText(part))
    .filter(Boolean)
    .join(' ')
}

export function sanitizeSupplierBrandText(value?: string | null): string {
  const raw = value?.trim()
  if (!raw) return ''
  return raw
    .replace(/qiniu_market/gi, 'market')
    .replace(/qiniu/gi, 'official')
    .replace(/七牛/g, '官方')
    .replace(/官方官方/g, '官方')
    .replace(/official\s+official/gi, 'official')
    .replace(/\s+/g, ' ')
    .trim()
}

export function sanitizeSupplierBrandUrl(value?: string | null): string {
  const raw = value?.trim()
  if (!raw) return ''
  // 隐藏供应商品牌只针对用户可见文案；图片和链接属于资源地址，不能因为包含供应商域名而清空，否则会导致模型 logo 丢失。
  return raw
}

function getMarketPricing(model: PricingModel): MarketPricingModel | undefined {
  return model.market_pricing ?? model.qiniu_market
}

function detailSortIndex(key: string): number {
  const index = DETAIL_ORDER.indexOf(key)
  return index >= 0 ? index : DETAIL_ORDER.length
}

function formatQiniuCNYPrice(
  detail: QiniuMarketPricingDetail,
  options: QiniuMarketPriceFormatOptions
): string {
  if (detail.unit_price === undefined || detail.unit_price === null) return ''
  const value = Number(detail.unit_price)
  if (!Number.isFinite(value)) return ''

  const scaledValue = value * getQiniuUnitScale(detail, options.tokenUnit)
  const displayValue = applyQiniuRechargeRate(scaledValue, options)
  return `¥${stripQiniuTrailingZeros(displayValue)}`
}

function stripQiniuTrailingZeros(value: number): string {
  if (value === 0) return '0'
  const fixed = value.toFixed(8)
  return fixed.replace(/\.?0+$/, '')
}

function formatQiniuUnitLabel(
  detail: QiniuMarketPricingDetail,
  options: QiniuMarketPriceFormatOptions
): string {
  const unitName = sanitizeSupplierBrandText(detail.unit_name)
  const tokenUnit = normalizeTokenUnit(options.tokenUnit)
  if (isTokenUnitName(unitName)) {
    return `/ ${formatQiniuUnitSize(TOKEN_UNIT_SIZE[tokenUnit])} ${unitName}`
  }

  const unitSize = Number(detail.unit_size)
  if (unitName && Number.isFinite(unitSize) && unitSize > 1) {
    return `/ ${formatQiniuUnitSize(unitSize)} ${unitName}`
  }
  if (unitName) {
    return `/ ${unitName}`
  }
  return ''
}

function formatQiniuUnitSize(value: number): string {
  if (value === 1000) return '1K'
  if (value === 1000000) return '1M'
  return String(value)
}

function normalizeTokenUnit(tokenUnit?: TokenUnit): TokenUnit {
  return tokenUnit === 'K' ? 'K' : DEFAULT_TOKEN_UNIT
}

function getQiniuUnitScale(
  detail: QiniuMarketPricingDetail,
  tokenUnit?: TokenUnit
): number {
  if (!isTokenUnitName(detail.unit_name)) {
    return 1
  }

  const sourceUnitSize = Number(detail.unit_size)
  const normalizedSourceUnitSize =
    Number.isFinite(sourceUnitSize) && sourceUnitSize > 0 ? sourceUnitSize : 1

  return (
    TOKEN_UNIT_SIZE[normalizeTokenUnit(tokenUnit)] / normalizedSourceUnitSize
  )
}

function isTokenUnitName(unitName?: string): boolean {
  const normalized = unitName?.trim().toLowerCase()
  return normalized === 'token' || normalized === 'tokens'
}

function applyQiniuRechargeRate(
  price: number,
  _options: QiniuMarketPriceFormatOptions
): number {
  // 七牛模型市场返回的是官方人民币价格，不能被本地充值汇率覆盖。
  return price
}
