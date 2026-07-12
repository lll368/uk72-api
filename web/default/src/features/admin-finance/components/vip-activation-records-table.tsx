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
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { disableAdminVipActivation, getAdminVipActivationRecords } from '../api'
import type {
  AdminVipActivationRecordFilters,
  VipActivationRecord,
} from '../types'
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
  getVipStatusLabel,
} from './shared'

type VipActivationRecordsTableProps = {
  title?: string
  description?: string
  showUserContactColumns?: boolean
  enableFilters?: boolean
  showActions?: boolean
}

const vipActivationStatusOptions = ['pending', 'success', 'failed', 'disabled']

function normalizeVipActivationFilters(
  draft: AdminRechargeFilterDraft
): AdminVipActivationRecordFilters {
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
    activatedFrom: datetimeLocalToUnixSeconds(draft.eventFrom),
    activatedTo: datetimeLocalToUnixSeconds(draft.eventTo),
  }
}

export function VipActivationRecordsTable({
  title,
  description,
  showUserContactColumns = false,
  enableFilters = false,
  showActions = true,
}: VipActivationRecordsTableProps = {}) {
  const { t } = useTranslation()
  const [records, setRecords] = useState<VipActivationRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [target, setTarget] = useState<VipActivationRecord | null>(null)
  const [reason, setReason] = useState('')
  const [draftFilters, setDraftFilters] = useState(
    createEmptyAdminRechargeFilterDraft
  )
  const [filters, setFilters] = useState<AdminVipActivationRecordFilters>({})
  const colSpan = 9 + (showUserContactColumns ? 2 : 0) + (showActions ? 1 : 0)

  const fetchRecords = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getAdminVipActivationRecords(
        page,
        ADMIN_FINANCE_PAGE_SIZE,
        enableFilters ? filters : undefined
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
  }, [enableFilters, filters, page, t])

  useEffect(() => {
    fetchRecords()
  }, [fetchRecords])

  const handleDisable = async () => {
    if (!target) return
    const response = await disableAdminVipActivation(
      target.user_id,
      reason.trim() || 'Disabled by administrator'
    )
    if (response.success) {
      toast.success(t('Operation successful'))
      setTarget(null)
      setReason('')
      fetchRecords()
      return
    }
    toast.error(response.message || t('Operation failed'))
  }

  const applyFilters = () => {
    setPage(1)
    setFilters(normalizeVipActivationFilters(draftFilters))
  }

  const resetFilters = () => {
    setPage(1)
    setDraftFilters(createEmptyAdminRechargeFilterDraft())
    setFilters({})
  }

  return (
    <>
      <TableShell
        title={title ?? t('VVIP Activations')}
        description={
          description ?? t('Review one-time paid VVIP activation records')
        }
        loading={loading}
        onRefresh={fetchRecords}
        refreshLabel={t('Refresh')}
        filters={
          enableFilters ? (
            <AdminRechargeFiltersBar
              draft={draftFilters}
              statusOptions={vipActivationStatusOptions}
              eventFromPlaceholder={t('Activated From')}
              eventToPlaceholder={t('Activated To')}
              onChange={setDraftFilters}
              onApply={applyFilters}
              onReset={resetFilters}
              t={t}
            />
          ) : undefined
        }
      >
        <div className='overflow-x-auto'>
            <Table>
              <TableHeader className='bg-card'>
              <TableRow>
                <TableHead>{t('User ID')}</TableHead>
                {showUserContactColumns ? (
                  <>
                    <TableHead>{t('Email')}</TableHead>
                    <TableHead>{t('Phone Number')}</TableHead>
                  </>
                ) : null}
                <TableHead>{t('Trade No.')}</TableHead>
                <TableHead>{t('Provider')}</TableHead>
                <TableHead>{t('Activation Amount')}</TableHead>
                <TableHead>{t('Paid Amount')}</TableHead>
                <TableHead>{t('Discount')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Activated At')}</TableHead>
                <TableHead>{t('Disabled At')}</TableHead>
                {showActions ? (
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                ) : null}
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={colSpan}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No VVIP activation records found')}
                  </TableCell>
                </TableRow>
              ) : (
                records.map((record) => (
                  <TableRow key={record.id}>
                    <TableCell>
                      <div className='font-medium'>{record.user_id}</div>
                      {showUserContactColumns ? (
                        <div className='text-muted-foreground max-w-[180px] truncate text-xs'>
                          {record.display_name || record.username || '-'}
                        </div>
                      ) : null}
                    </TableCell>
                    {showUserContactColumns ? (
                      <>
                        <TableCell className='max-w-[220px] truncate'>
                          {record.email || '-'}
                        </TableCell>
                        <TableCell>{record.phone_number || '-'}</TableCell>
                      </>
                    ) : null}
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
                      {formatMoney(record.activation_amount)}
                    </TableCell>
                    <TableCell className='font-mono'>
                      {formatMoney(record.paid_amount)}
                    </TableCell>
                    <TableCell>{record.discount}</TableCell>
                    <TableCell>
                      <StatusBadge
                        label={getVipStatusLabel(t, record.status)}
                      />
                    </TableCell>
                    <TableCell>{formatTime(record.activated_at)}</TableCell>
                    <TableCell>
                      <div>{formatTime(record.disabled_at)}</div>
                      {record.disable_reason ? (
                        <div className='text-muted-foreground max-w-[180px] truncate text-xs'>
                          {t('Disable Reason')}: {record.disable_reason}
                        </div>
                      ) : null}
                    </TableCell>
                    {showActions ? (
                      <TableCell className='text-right'>
                        {record.status === 'success' ? (
                          <Button
                            size='sm'
                            variant='destructive'
                            onClick={() => setTarget(record)}
                          >
                            {t('Disable')}
                          </Button>
                        ) : (
                          <span className='text-muted-foreground text-xs'>
                            -
                          </span>
                        )}
                      </TableCell>
                    ) : null}
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

      {showActions ? (
        <ConfirmDialog
          open={Boolean(target)}
          onOpenChange={(open) => !open && setTarget(null)}
          title={t('Disable VVIP')}
          desc={t(
            'Disable this user VVIP status and invalidate pending activation orders.'
          )}
          confirmText={t('Disable')}
          destructive
          handleConfirm={handleDisable}
        >
          <Input
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder={t('Disable reason')}
          />
        </ConfirmDialog>
      ) : null}
    </>
  )
}
