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
import { Eye } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { getPiggyWithdrawCallbackLogs } from '../api'
import type { PiggyWithdrawCallbackLog } from '../types'
import {
  ADMIN_FINANCE_PAGE_SIZE,
  FilterInput,
  PaginationBar,
  StatusBadge,
  StatusFilter,
  TableShell,
  formatTime,
  getStatusLabel,
} from './shared'

export function PiggyWithdrawCallbackLogsTable() {
  const { t } = useTranslation()
  const [items, setItems] = useState<PiggyWithdrawCallbackLog[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [callbackType, setCallbackType] = useState('')
  const [orderNo, setOrderNo] = useState('')
  const [processStatus, setProcessStatus] = useState('')
  const [detail, setDetail] = useState<PiggyWithdrawCallbackLog | null>(null)

  const fetchItems = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getPiggyWithdrawCallbackLogs({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        callbackType,
        orderNo,
        processStatus,
      })
      if (response.success && response.data) {
        setItems(response.data.items || [])
        setTotal(response.data.total || 0)
      } else {
        toast.error(response.message || t('Failed to load records'))
      }
    } finally {
      setLoading(false)
    }
  }, [callbackType, orderNo, page, processStatus, t])

  useEffect(() => {
    fetchItems()
  }, [fetchItems])

  return (
    <>
      <TableShell
        title={t('Piggy withdrawal callback logs')}
        description={t(
          'Review Piggy contract and payment callback audit records'
        )}
        loading={loading}
        onRefresh={fetchItems}
        refreshLabel={t('Refresh')}
        filters={
          <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap'>
            <StatusFilter
              value={callbackType}
              onChange={(value) => {
                setCallbackType(value)
                setPage(1)
              }}
              options={['contract', 'payment']}
              allLabel={t('All Callback Types')}
              getOptionLabel={(value) =>
                value === 'contract' ? t('Contract') : t('Payment')
              }
            />
            <FilterInput
              value={orderNo}
              onChange={(value) => {
                setOrderNo(value)
                setPage(1)
              }}
              placeholder={t('Withdraw No.')}
            />
            <StatusFilter
              value={processStatus}
              onChange={(value) => {
                setProcessStatus(value)
                setPage(1)
              }}
              options={['pending', 'success', 'failed', 'duplicate']}
              allLabel={t('All Status')}
              getOptionLabel={(value) => getStatusLabel(t, value)}
            />
          </div>
        }
      >
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Callback Type')}</TableHead>
                <TableHead>{t('Withdraw No.')}</TableHead>
                <TableHead>{t('User ID')}</TableHead>
                <TableHead>{t('Piggy status')}</TableHead>
                <TableHead>{t('Process Status')}</TableHead>
                <TableHead>{t('Created At')}</TableHead>
                <TableHead className='text-right'>{t('Details')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={7}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No callback logs found')}
                  </TableCell>
                </TableRow>
              ) : (
                items.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>
                      {item.callback_type === 'contract'
                        ? t('Contract')
                        : t('Payment')}
                    </TableCell>
                    <TableCell className='max-w-[220px] truncate font-mono text-xs'>
                      {item.order_no || '-'}
                    </TableCell>
                    <TableCell>{item.user_id || '-'}</TableCell>
                    <TableCell>
                      <div>{item.notify_type || '-'}</div>
                      <div className='text-muted-foreground text-xs'>
                        {item.trade_status || '-'}
                      </div>
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        label={getStatusLabel(t, item.process_status)}
                      />
                    </TableCell>
                    <TableCell>{formatTime(item.created_at)}</TableCell>
                    <TableCell className='text-right'>
                      <Button
                        size='sm'
                        variant='outline'
                        onClick={() => setDetail(item)}
                      >
                        <Eye data-icon='inline-start' />
                        {t('View')}
                      </Button>
                    </TableCell>
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

      <Dialog
        open={Boolean(detail)}
        onOpenChange={(open) => !open && setDetail(null)}
      >
        <DialogContent className='sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle>{t('Callback Detail')}</DialogTitle>
            <DialogDescription>{detail?.order_no || '-'}</DialogDescription>
          </DialogHeader>
          <div className='grid gap-3'>
            <Textarea
              value={detail?.payload_digest || ''}
              readOnly
              className='min-h-24 font-mono text-xs'
              placeholder={t('Payload digest')}
            />
            <Textarea
              value={detail?.decrypted_digest || ''}
              readOnly
              className='min-h-24 font-mono text-xs'
              placeholder={t('Decrypted digest')}
            />
            <Textarea
              value={detail?.error_message || ''}
              readOnly
              className='min-h-24 font-mono text-xs'
              placeholder={t('No error message')}
            />
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
