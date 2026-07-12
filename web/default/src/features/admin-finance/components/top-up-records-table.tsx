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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getAdminTopUpRecords } from '../api'
import type { AdminTopUpRecord, AdminTopUpRecordFilters } from '../types'
import {
  createEmptyAdminRechargeFilterDraft,
  datetimeLocalToUnixSeconds,
  trimToUndefined,
  type AdminRechargeFilterDraft,
} from './admin-recharge-filter-utils'
import { AdminRechargeFiltersBar } from './admin-recharge-filters'
import {
  ADMIN_FINANCE_PAGE_SIZE,
  PaginationBar,
  StatusBadge,
  TableShell,
  formatMoney,
  formatTime,
  getStatusLabel,
} from './shared'

const topUpStatusOptions = ['pending', 'success', 'failed', 'reversed']

function normalizeTopUpFilters(
  draft: AdminRechargeFilterDraft
): AdminTopUpRecordFilters {
  return {
    userId: trimToUndefined(draft.userId),
    email: trimToUndefined(draft.email),
    phoneNumber: trimToUndefined(draft.phoneNumber),
    tradeNo: trimToUndefined(draft.tradeNo),
    status: trimToUndefined(draft.status),
    paymentProvider: trimToUndefined(draft.paymentProvider),
    paymentMethod: trimToUndefined(draft.paymentMethod),
    createdFrom: datetimeLocalToUnixSeconds(draft.createdFrom),
    createdTo: datetimeLocalToUnixSeconds(draft.createdTo),
    completedFrom: datetimeLocalToUnixSeconds(draft.eventFrom),
    completedTo: datetimeLocalToUnixSeconds(draft.eventTo),
  }
}

function getDisplayRechargeAmount(record: AdminTopUpRecord) {
  return record.recharge_amount > 0 ? record.recharge_amount : record.amount
}

function getDisplayPaidAmount(record: AdminTopUpRecord) {
  return record.paid_amount > 0 ? record.paid_amount : record.money
}

function formatDiscountValue(value: number) {
  return Number.isFinite(value) ? value : '-'
}

export function TopUpRecordsTable() {
  const { t } = useTranslation()
  const [records, setRecords] = useState<AdminTopUpRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [draftFilters, setDraftFilters] = useState(
    createEmptyAdminRechargeFilterDraft
  )
  const [filters, setFilters] = useState<AdminTopUpRecordFilters>({})

  const fetchRecords = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getAdminTopUpRecords(
        page,
        ADMIN_FINANCE_PAGE_SIZE,
        filters
      )
      if (response.success && response.data) {
        setRecords(response.data.items || [])
        setTotal(response.data.total || 0)
      } else {
        toast.error(response.message || t('Failed to load records'))
      }
    } finally {
      setLoading(false)
    }
  }, [filters, page, t])

  useEffect(() => {
    fetchRecords()
  }, [fetchRecords])

  const applyFilters = () => {
    setPage(1)
    setFilters(normalizeTopUpFilters(draftFilters))
  }

  const resetFilters = () => {
    setPage(1)
    setDraftFilters(createEmptyAdminRechargeFilterDraft())
    setFilters({})
  }

  return (
    <TableShell
      title={t('Ordinary Top-ups')}
      description={t('Ordinary user recharge orders')}
      loading={loading}
      onRefresh={fetchRecords}
      refreshLabel={t('Refresh')}
      filters={
        <AdminRechargeFiltersBar
          draft={draftFilters}
          statusOptions={topUpStatusOptions}
          eventFromPlaceholder={t('Completed From')}
          eventToPlaceholder={t('Completed To')}
          onChange={setDraftFilters}
          onApply={applyFilters}
          onReset={resetFilters}
          t={t}
        />
      }
    >
      <div className='overflow-x-auto'>
          <Table>
            <TableHeader className='bg-card'>
            <TableRow>
              <TableHead>{t('User')}</TableHead>
              <TableHead>{t('Email')}</TableHead>
              <TableHead>{t('Phone Number')}</TableHead>
              <TableHead>{t('Trade No.')}</TableHead>
              <TableHead>{t('Provider')}</TableHead>
              <TableHead>{t('Amount')}</TableHead>
              <TableHead>{t('Paid Amount')}</TableHead>
              <TableHead>{t('Discount')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Created At')}</TableHead>
              <TableHead>{t('Completed At')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {records.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={11}
                  className='text-muted-foreground h-24 text-center'
                >
                  {t('No recharge records found')}
                </TableCell>
              </TableRow>
            ) : (
              records.map((record) => (
                <TableRow key={record.id}>
                  <TableCell>
                    <div className='font-medium'>{record.user_id}</div>
                    <div className='text-muted-foreground max-w-[180px] truncate text-xs'>
                      {record.display_name || record.username || '-'}
                    </div>
                  </TableCell>
                  <TableCell className='max-w-[220px] truncate'>
                    {record.email || '-'}
                  </TableCell>
                  <TableCell>{record.phone_number || '-'}</TableCell>
                  <TableCell className='max-w-[220px] truncate font-mono text-xs'>
                    {record.trade_no}
                  </TableCell>
                  <TableCell>
                    <div className='text-sm'>
                      {record.payment_provider || '-'}
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      {record.payment_method || '-'}
                    </div>
                  </TableCell>
                  <TableCell className='font-mono'>
                    {formatMoney(getDisplayRechargeAmount(record))}
                  </TableCell>
                  <TableCell className='font-mono'>
                    {formatMoney(getDisplayPaidAmount(record))}
                  </TableCell>
                  <TableCell>{formatDiscountValue(record.discount)}</TableCell>
                  <TableCell>
                    <StatusBadge label={getStatusLabel(t, record.status)} />
                  </TableCell>
                  <TableCell>{formatTime(record.create_time)}</TableCell>
                  <TableCell>{formatTime(record.complete_time)}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
          </Table>
      </div>
      <PaginationBar
        page={page}
        pageSize={ADMIN_FINANCE_PAGE_SIZE}
        total={total}
        loading={loading}
        onPageChange={setPage}
        t={t}
      />
    </TableShell>
  )
}
