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
import type { ReactNode } from 'react'
import type { TFunction } from 'i18next'
import { RefreshCw } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { cn } from '@/lib/utils'

export const ADMIN_FINANCE_PAGE_SIZE = 20

export function formatTime(timestamp?: number) {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

export function formatMoney(value?: number) {
  const amount = Number(value ?? 0)
  return amount.toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

export function getStatusLabel(t: TFunction, status?: string) {
  const map: Record<string, string> = {
    active: 'Active',
    approved: 'Approved',
    disabled: 'Disabled',
    failed: 'Failed',
    ignored: 'Ignored',
    paid: 'Paid',
    pending: 'Pending',
    rejected: 'Rejected',
    reversed: 'Reversed',
    settled: 'Settled',
    submitted: 'Submitted',
    success: 'Success',
    await_confirm: 'Awaiting confirmation',
    confirming: 'Confirming payment',
    cancelling: 'Cancelling',
    confirmed: 'Confirmed',
    cancelled: 'Cancelled',
    manual_review: 'Manual review',
    needs_review: 'Needs review',
    applied: 'Applied',
    skipped: 'Skipped',
    reconciled: 'Reconciled',
    unmapped: 'Unmapped',
    ambiguous: 'Ambiguous',
    resolved: 'Resolved',
    manual_resolved: 'Manual resolved',
  }
  return t(map[status || ''] || 'Unknown')
}

export function getVipStatusLabel(t: TFunction, status?: string) {
  if (status === 'success') return t('Active')
  if (status === 'pending') return t('Pending payment')
  return getStatusLabel(t, status)
}

export function getWithdrawStatusLabel(t: TFunction, status?: string) {
  if (status === 'pending') return t('Pending review')
  if (status === 'await_confirm') return t('Awaiting confirmation')
  if (status === 'confirming') return t('Confirming payment')
  if (status === 'cancelling') return t('Cancelling')
  if (status === 'manual_review') return t('Manual review')
  return getStatusLabel(t, status)
}

export function getPaymentDiffLabel(t: TFunction, diffType?: string) {
  const map: Record<string, string> = {
    amount_mismatch: 'Amount mismatch',
    duplicate_callback: 'Duplicate callback',
    local_missing: 'Local missing',
    provider_missing: 'Provider missing',
    status_mismatch: 'Status mismatch',
  }
  return t(map[diffType || ''] || diffType || 'Unknown')
}

export function getWalletFlowLabel(t: TFunction, value?: string) {
  const map: Record<string, string> = {
    recharge_balance: 'Recharge to balance',
    vip_activation: 'VVIP activation',
    topup: 'Top-up',
    commission_income: 'Commission income',
    commission_to_balance: 'Commission to balance',
    balance_consume: 'Balance consume',
    balance_refund: 'Balance refund',
    withdraw_freeze: 'Withdrawal freeze',
    withdraw_success: 'Withdrawal paid',
    withdraw_reject: 'Withdrawal rejected',
    refund_reverse: 'Refund reversal',
  }
  return t(map[value || ''] || value || 'Unknown')
}

export function StatusBadge({ label }: { label: string }) {
  return (
    <Badge variant='outline' className='max-w-full truncate'>
      {label}
    </Badge>
  )
}

export function TableShell(props: {
  title: string
  description?: string
  loading?: boolean
  onRefresh: () => void
  refreshLabel: string
  actions?: ReactNode
  filters?: ReactNode
  children: ReactNode
}) {
  return (
    <Card className='py-0 bg-gradient-to-br from-purple-50 via-blue-50 to-indigo-50 border-purple-100'>
      <CardHeader className='flex flex-col gap-3 p-4 lg:flex-row lg:items-start lg:justify-between'>
        <div className='min-w-0'>
          <CardTitle className='text-base'>{props.title}</CardTitle>
          {props.description ? (
            <p className='text-muted-foreground mt-1 text-sm'>
              {props.description}
            </p>
          ) : null}
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          {props.actions}
          <Button
            variant='outline'
            size='sm'
            onClick={props.onRefresh}
            disabled={props.loading}
          >
            <RefreshCw
              data-icon='inline-start'
              className={props.loading ? 'animate-spin' : undefined}
            />
            {props.refreshLabel}
          </Button>
        </div>
      </CardHeader>
      {props.filters ? (
        <div className='border-t px-4 py-3'>{props.filters}</div>
      ) : null}
      <CardContent className='p-0'>{props.children}</CardContent>
    </Card>
  )
}

export function FilterInput(props: {
  value: string
  onChange: (value: string) => void
  placeholder: string
  className?: string
}) {
  return (
    <Input
      value={props.value}
      onChange={(event) => props.onChange(event.target.value)}
      placeholder={props.placeholder}
      className={cn('h-9', props.className ?? 'w-full sm:w-56')}
    />
  )
}

export function StatusFilter(props: {
  value: string
  onChange: (value: string) => void
  options: string[]
  allLabel: string
  getOptionLabel?: (value: string) => string
  className?: string
}) {
  return (
    <NativeSelect
      value={props.value}
      onChange={(event) => props.onChange(event.target.value)}
      className={props.className ?? 'w-full sm:w-44'}
    >
      <NativeSelectOption value=''>{props.allLabel}</NativeSelectOption>
      {props.options.map((option) => (
        <NativeSelectOption key={option} value={option}>
          {props.getOptionLabel ? props.getOptionLabel(option) : option}
        </NativeSelectOption>
      ))}
    </NativeSelect>
  )
}

export function PaginationBar(props: {
  page: number
  pageSize: number
  total: number
  loading?: boolean
  onPageChange: (page: number) => void
  t: TFunction
}) {
  const pageCount = Math.max(1, Math.ceil(props.total / props.pageSize))
  return (
    <div className='flex flex-col gap-2 border-t p-3 text-sm sm:flex-row sm:items-center sm:justify-between'>
      <div className='text-muted-foreground'>
        {props.t('Total {{count}} records', { count: props.total })}
      </div>
      <div className='flex items-center gap-2'>
        <Button
          variant='outline'
          size='sm'
          disabled={props.loading || props.page <= 1}
          onClick={() => props.onPageChange(props.page - 1)}
        >
          {props.t('Previous')}
        </Button>
        <span className='text-muted-foreground min-w-20 text-center'>
          {props.page} / {pageCount}
        </span>
        <Button
          variant='outline'
          size='sm'
          disabled={props.loading || props.page >= pageCount}
          onClick={() => props.onPageChange(props.page + 1)}
        >
          {props.t('Next')}
        </Button>
      </div>
    </div>
  )
}
