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
import { useState } from 'react'
import type { TFunction } from 'i18next'
import { ChevronDown, Search, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import type { AdminRechargeFilterDraft } from './admin-recharge-filter-utils'
import { FilterInput, StatusFilter, getStatusLabel } from './shared'

const paymentProviderOptions = [
  'stripe',
  'alipay',
  'wechat',
  'epay',
  'creem',
  'waffo',
  'waffo_pancake',
]

const paymentMethodOptions = [
  'stripe',
  'alipay_direct',
  'wechat_direct',
  'alipay',
  'wxpay',
  'creem',
  'waffo',
  'waffo_pancake',
]

const inputClass = 'h-9 w-full sm:w-[140px] lg:w-[160px]'
const selectClass = 'w-full sm:w-[140px] lg:w-[160px]'

export function AdminRechargeFiltersBar(props: {
  draft: AdminRechargeFilterDraft
  statusOptions: string[]
  eventFromPlaceholder: string
  eventToPlaceholder: string
  onChange: (draft: AdminRechargeFilterDraft) => void
  onApply: () => void
  onReset: () => void
  t: TFunction
}) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)

  const update = (key: keyof AdminRechargeFilterDraft, value: string) => {
    props.onChange({ ...props.draft, [key]: value })
  }

  const hasExpandedFilters =
    !!props.draft.phoneNumber ||
    !!props.draft.tradeNo ||
    !!props.draft.paymentProvider ||
    !!props.draft.paymentMethod ||
    !!props.draft.createdFrom ||
    !!props.draft.createdTo ||
    !!props.draft.eventFrom ||
    !!props.draft.eventTo

  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <FilterInput
          value={props.draft.userId}
          onChange={(value) => update('userId', value)}
          placeholder={props.t('User ID')}
          className={inputClass}
        />
        <FilterInput
          value={props.draft.email}
          onChange={(value) => update('email', value)}
          placeholder={props.t('Email')}
          className={inputClass}
        />
        <StatusFilter
          value={props.draft.status}
          onChange={(value) => update('status', value)}
          options={props.statusOptions}
          allLabel={props.t('All Statuses')}
          getOptionLabel={(value) => getStatusLabel(props.t, value)}
          className={selectClass}
        />

        <div className='ms-auto flex items-center gap-2'>
          <Button
            variant='ghost'
            onClick={() => setExpanded((p) => !p)}
            aria-expanded={expanded}
            className={cn(
              'text-muted-foreground hover:text-foreground gap-1 px-2',
              hasExpandedFilters &&
                !expanded &&
                'text-primary hover:text-primary'
            )}
          >
            {expanded ? t('Collapse') : t('Expand')}
            <ChevronDown
              className={cn(
                'size-3.5 transition-transform duration-200',
                expanded && 'rotate-180'
              )}
            />
          </Button>
          <Button size='sm' onClick={props.onApply}>
            <Search data-icon='inline-start' />
            {props.t('Search')}
          </Button>
          <Button size='sm' variant='outline' onClick={props.onReset}>
            <X data-icon='inline-start' />
            {props.t('Reset')}
          </Button>
        </div>
      </div>

      {expanded && (
        <div className='flex flex-wrap items-center gap-2'>
          <FilterInput
            value={props.draft.phoneNumber}
            onChange={(value) => update('phoneNumber', value)}
            placeholder={props.t('Phone Number')}
            className={inputClass}
          />
          <FilterInput
            value={props.draft.tradeNo}
            onChange={(value) => update('tradeNo', value)}
            placeholder={props.t('Trade No.')}
            className={inputClass}
          />
          <StatusFilter
            value={props.draft.paymentProvider}
            onChange={(value) => update('paymentProvider', value)}
            options={paymentProviderOptions}
            allLabel={props.t('All Providers')}
            className={selectClass}
          />
          <StatusFilter
            value={props.draft.paymentMethod}
            onChange={(value) => update('paymentMethod', value)}
            options={paymentMethodOptions}
            allLabel={props.t('All Methods')}
            className={selectClass}
          />
        </div>
      )}

      {expanded && (
        <div className='flex flex-wrap items-center gap-2'>
          <DateTimeFilterInput
            label={props.t('Created From')}
            value={props.draft.createdFrom}
            onChange={(value) => update('createdFrom', value)}
            className={inputClass}
          />
          <DateTimeFilterInput
            label={props.t('Created To')}
            value={props.draft.createdTo}
            onChange={(value) => update('createdTo', value)}
            className={inputClass}
          />
          <DateTimeFilterInput
            label={props.eventFromPlaceholder}
            value={props.draft.eventFrom}
            onChange={(value) => update('eventFrom', value)}
            className={inputClass}
          />
          <DateTimeFilterInput
            label={props.eventToPlaceholder}
            value={props.draft.eventTo}
            onChange={(value) => update('eventTo', value)}
            className={inputClass}
          />
        </div>
      )}
    </div>
  )
}

function DateTimeFilterInput(props: {
  label: string
  value: string
  onChange: (value: string) => void
  className?: string
}) {
  return (
    <label className='text-muted-foreground grid gap-1 text-xs'>
      <span>{props.label}</span>
      <Input
        type='datetime-local'
        value={props.value}
        onChange={(event) => props.onChange(event.target.value)}
        aria-label={props.label}
        className={cn('text-foreground h-9', props.className)}
      />
    </label>
  )
}
