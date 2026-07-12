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
import { memo } from 'react'
import { ChevronRight, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { DEFAULT_TOKEN_UNIT } from '../constants'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
} from '../lib/dynamic-price'
import { parseTags } from '../lib/filters'
import { isTokenBasedModel } from '../lib/model-helpers'
import { formatPrice, formatRequestPrice } from '../lib/price'
import { getPricingIconSource } from '../lib/pricing-icon'
import {
  formatQiniuMarketPriceItems,
  isMarketPricingModel,
} from '../lib/qiniu-market'
import type { PricingModel, TokenUnit } from '../types'
import type { ModelPerfBadgeData } from './model-perf-badge'
import { PricingIcon } from './pricing-icon'

export interface ModelCardProps {
  model: PricingModel
  onClick: () => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  perf?: ModelPerfBadgeData
}

type PriceColumn = {
  key: string
  label: string
  price: string
  unit: string
}

function buildPriceColumns(
  props: ModelCardProps,
  t: (key: string) => string
): PriceColumn[] {
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const priceRate = props.priceRate ?? 1
  const usdExchangeRate = props.usdExchangeRate ?? 1
  const showRechargePrice = props.showRechargePrice ?? false
  const isTokenBased = isTokenBasedModel(props.model)
  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'
  const isDynamicPricing =
    props.model.billing_mode === 'tiered_expr' &&
    Boolean(props.model.billing_expr)
  const dynamicSummary = isDynamicPricing
    ? getDynamicPricingSummary(props.model, {
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        groupRatioMultiplier: getDynamicDisplayGroupRatio(props.model),
      })
    : null
  const qiniuPriceItems = formatQiniuMarketPriceItems(props.model, {
    tokenUnit,
    showRechargePrice,
    priceRate,
    usdExchangeRate,
  })
  const isMarketPricing = isMarketPricingModel(props.model)

  if (qiniuPriceItems.length > 0) {
    return qiniuPriceItems.slice(0, 2).map((item) => ({
      key: item.key,
      label: item.label,
      price: item.formatted,
      unit: item.unitLabel || `/${tokenUnitLabel}`,
    }))
  }

  if (isMarketPricing) {
    return []
  }

  if (dynamicSummary) {
    if (dynamicSummary.isSpecialExpression) {
      return [
        {
          key: 'dynamic',
          label: t('Special billing expression'),
          price: '—',
          unit: '',
        },
      ]
    }

    if (dynamicSummary.primaryEntries.length > 0) {
      return dynamicSummary.primaryEntries.slice(0, 2).map((entry) => ({
        key: entry.key,
        label: t(entry.shortLabel),
        price: entry.formatted,
        unit: `/${tokenUnitLabel}`,
      }))
    }

    return [
      {
        key: 'dynamic',
        label: t('Dynamic Pricing'),
        price: '—',
        unit: '',
      },
    ]
  }

  if (isTokenBased) {
    return [
      {
        key: 'input',
        label: t('Input'),
        price: formatPrice(
          props.model,
          'input',
          tokenUnit,
          showRechargePrice,
          priceRate,
          usdExchangeRate
        ),
        unit: `/${tokenUnitLabel} ${t('Token').toLowerCase()}`,
      },
      {
        key: 'output',
        label: t('Output'),
        price: formatPrice(
          props.model,
          'output',
          tokenUnit,
          showRechargePrice,
          priceRate,
          usdExchangeRate
        ),
        unit: `/${tokenUnitLabel} ${t('Token').toLowerCase()}`,
      },
    ]
  }

  return [
    {
      key: 'request',
      label: t('Per Request'),
      price: formatRequestPrice(
        props.model,
        showRechargePrice,
        priceRate,
        usdExchangeRate
      ),
      unit: `/ ${t('request')}`,
    },
  ]
}

export const ModelCard = memo(function ModelCard(props: ModelCardProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const tags = parseTags(props.model.tags)
  const endpoints = props.model.supported_endpoint_types || []
  const displayTags = [...tags, ...endpoints].slice(0, 4)
  const iconSource = getPricingIconSource(
    props.model.icon,
    props.model.vendor_icon
  )
  const initial = props.model.model_name?.charAt(0).toUpperCase() || '?'
  const priceColumns = buildPriceColumns(props, t)

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation()
    copyToClipboard(props.model.model_name || '')
  }

  return (
    <div
      className={cn(
        'group relative flex flex-col overflow-hidden transition-all duration-300',
        'rounded-[20px]',
        'dark:bg-card border-2 border-transparent bg-white',
        'shadow-[0_4px_12px_rgba(135,129,255,0.1)]',
        'hover:-translate-y-0.5 hover:border-[#C2CEFF] hover:shadow-[0px_12px_25px_0px_#E3E4FF]',
        'dark:hover:bg-card'
      )}
    >
      {/* Hover gradient overlay */}
      <div
        aria-hidden
        className='pointer-events-none absolute inset-0 z-0 rounded-[18px] opacity-0 transition-opacity duration-300 group-hover:opacity-100'
        style={{
          background:
            'linear-gradient(138deg, #FAF4FF 0%, #F2F7FF 52.4%, #F1F6FF 86.54%)',
        }}
      />

      {/* Card content */}
      <div className='relative z-[1] flex flex-1 flex-col p-5'>
        {/* Top: Icon + Name/Tags */}
        <div className='flex items-start gap-4'>
          <div className='dark:bg-muted/40 flex size-16 shrink-0 items-center justify-center rounded-xl border border-gray-100 bg-[#F8F9FC] dark:border-gray-800'>
            {iconSource ? (
              <PricingIcon
                icon={props.model.icon}
                fallbackIcon={props.model.vendor_icon}
                size={36}
                className='rounded-sm'
              />
            ) : (
              <span className='text-foreground/80 text-lg font-bold'>
                {initial}
              </span>
            )}
          </div>
          <div className='min-w-0 flex-1'>
            <h3 className='text-foreground truncate text-lg leading-tight font-semibold'>
              {props.model.model_name}
            </h3>
            {displayTags.length > 0 && (
              <div className='mt-2 flex flex-wrap gap-1.5'>
                {displayTags.map((tag) => (
                  <span
                    key={tag}
                    className='dark:bg-muted/60 rounded-full border border-[#D0D7FF]/50 bg-[#F8F9FC] px-2.5 py-0.5 text-xs text-gray-600 dark:border-gray-700 dark:text-gray-400'
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Description */}
        <p className='text-muted-foreground mt-3 line-clamp-2 min-h-[2.5rem] text-sm leading-relaxed'>
          {props.model.description || t('No description available.')}
        </p>

        {/* Actions */}
        <div className='mt-3 flex items-center gap-2'>
          <button
            type='button'
            onClick={props.onClick}
            className='inline-flex items-center gap-0.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors'
            style={{ background: '#EEF2FE', color: '#4A6FF7' }}
          >
            {t('Details')}
            <ChevronRight className='size-3.5' />
          </button>
          <button
            type='button'
            onClick={handleCopy}
            className='rounded-md border border-gray-200 p-1.5 transition-colors dark:border-gray-700'
            style={{ color: '#4A6FF7' }}
            title={t('Copy')}
          >
            <Copy className='size-3.5' />
          </button>
        </div>
      </div>

      {/* Separator line (above overlay) */}
      {priceColumns.length > 0 && (
        <div
          className='relative z-[1] h-px bg-gray-200/80 dark:bg-gray-800'
          aria-hidden
        />
      )}

      {/* Price line */}
      {priceColumns.length > 0 && (
        <div className='relative z-[1] px-5 py-3'>
          <div className='flex flex-wrap items-center justify-center gap-1 text-sm'>
            {priceColumns.map((column, index) => (
              <span key={column.key} className='inline-flex items-center gap-1'>
                {index > 0 && (
                  <span className='mx-1.5 text-[#D0D7FF] dark:text-gray-600'>
                    |
                  </span>
                )}
                <span className='text-muted-foreground'>{column.label}</span>
                <span className='font-semibold' style={{ color: '#E65B2E' }}>
                  {column.price}
                  {column.unit ? (
                    <span className='font-normal'> {column.unit}</span>
                  ) : null}
                </span>
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  )
})
