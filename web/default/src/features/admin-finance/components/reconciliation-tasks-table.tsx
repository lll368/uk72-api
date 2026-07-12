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
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  createPaymentReconciliationTask,
  getPaymentReconciliationTasks,
} from '../api'
import type {
  PaymentReconciliationDiff,
  PaymentReconciliationTask,
  ProviderPaymentOrder,
} from '../types'
import {
  ADMIN_FINANCE_PAGE_SIZE,
  FilterInput,
  PaginationBar,
  StatusBadge,
  StatusFilter,
  TableShell,
  formatMoney,
  formatTime,
  getPaymentDiffLabel,
  getStatusLabel,
} from './shared'

export function ReconciliationTasksTable() {
  const { t } = useTranslation()
  const [items, setItems] = useState<PaymentReconciliationTask[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [provider, setProvider] = useState('')
  const [status, setStatus] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [form, setForm] = useState({
    provider: '',
    dateFrom: '',
    dateTo: '',
    ordersJson: '[]',
  })
  const [diffs, setDiffs] = useState<PaymentReconciliationDiff[] | null>(null)

  const fetchItems = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getPaymentReconciliationTasks({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        provider,
        status,
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
  }, [page, provider, status, t])

  useEffect(() => {
    fetchItems()
  }, [fetchItems])

  const resetForm = () => {
    setForm({ provider: '', dateFrom: '', dateTo: '', ordersJson: '[]' })
  }

  const handleCreate = async () => {
    if (submitting) return
    let orders: ProviderPaymentOrder[]
    try {
      orders = JSON.parse(form.ordersJson) as ProviderPaymentOrder[]
      if (!Array.isArray(orders)) throw new Error('orders must be array')
    } catch (_error) {
      toast.error(t('Orders JSON must be an array'))
      return
    }
    setSubmitting(true)
    try {
      const response = await createPaymentReconciliationTask({
        provider: form.provider.trim(),
        date_from: Number(form.dateFrom),
        date_to: Number(form.dateTo),
        orders,
      })
      if (response.success) {
        toast.success(t('Operation successful'))
        setDiffs(response.data?.diffs || [])
        setCreateOpen(false)
        resetForm()
        fetchItems()
        return
      }
      toast.error(response.message || t('Operation failed'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <TableShell
        title={t('Reconciliation')}
        description={t('Create and review local payment reconciliation tasks')}
        loading={loading}
        onRefresh={fetchItems}
        refreshLabel={t('Refresh')}
        actions={
          <Button size='sm' onClick={() => setCreateOpen(true)}>
            <Plus data-icon='inline-start' />
            {t('Create Task')}
          </Button>
        }
        filters={
          <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap'>
            <FilterInput
              value={provider}
              onChange={(value) => {
                setProvider(value)
                setPage(1)
              }}
              placeholder={t('Provider')}
            />
            <StatusFilter
              value={status}
              onChange={(value) => {
                setStatus(value)
                setPage(1)
              }}
              options={['pending', 'success', 'failed']}
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
                <TableHead>{t('Provider')}</TableHead>
                <TableHead>{t('Date Range')}</TableHead>
                <TableHead>{t('Total Count')}</TableHead>
                <TableHead>{t('Diff Count')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Created At')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No reconciliation tasks found')}
                  </TableCell>
                </TableRow>
              ) : (
                items.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{item.provider}</TableCell>
                    <TableCell>
                      <div>{formatTime(item.date_from)}</div>
                      <div className='text-muted-foreground text-xs'>
                        {formatTime(item.date_to)}
                      </div>
                    </TableCell>
                    <TableCell>{item.total_count}</TableCell>
                    <TableCell>{item.diff_count}</TableCell>
                    <TableCell>
                      <StatusBadge label={getStatusLabel(t, item.status)} />
                    </TableCell>
                    <TableCell>{formatTime(item.created_at)}</TableCell>
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

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className='sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle>{t('Create Reconciliation Task')}</DialogTitle>
            <DialogDescription>
              {t('Paste provider payment orders as a JSON array.')}
            </DialogDescription>
          </DialogHeader>
          <div className='grid gap-3'>
            <Input
              value={form.provider}
              onChange={(event) =>
                setForm((value) => ({ ...value, provider: event.target.value }))
              }
              placeholder={t('Provider')}
            />
            <div className='grid gap-3 sm:grid-cols-2'>
              <Input
                value={form.dateFrom}
                onChange={(event) =>
                  setForm((value) => ({
                    ...value,
                    dateFrom: event.target.value,
                  }))
                }
                placeholder={t('Start timestamp')}
              />
              <Input
                value={form.dateTo}
                onChange={(event) =>
                  setForm((value) => ({ ...value, dateTo: event.target.value }))
                }
                placeholder={t('End timestamp')}
              />
            </div>
            <Textarea
              value={form.ordersJson}
              onChange={(event) =>
                setForm((value) => ({
                  ...value,
                  ordersJson: event.target.value,
                }))
              }
              className='min-h-44 font-mono text-xs'
            />
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setCreateOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={handleCreate} disabled={submitting}>
              {t('Create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={diffs !== null}
        onOpenChange={(open) => !open && setDiffs(null)}
      >
        <DialogContent className='sm:max-w-5xl'>
          <DialogHeader>
            <DialogTitle>{t('Reconciliation Diffs')}</DialogTitle>
            <DialogDescription>
              {t('Review detected payment order differences for this task.')}
            </DialogDescription>
          </DialogHeader>
          <div className='max-h-[60vh] overflow-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Trade No.')}</TableHead>
                  <TableHead>{t('Biz Type')}</TableHead>
                  <TableHead>{t('Diff Type')}</TableHead>
                  <TableHead>{t('Local Status')}</TableHead>
                  <TableHead>{t('Provider Status')}</TableHead>
                  <TableHead>{t('Local Amount')}</TableHead>
                  <TableHead>{t('Provider Amount')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {diffs && diffs.length > 0 ? (
                  diffs.map((diff, index) => (
                    <TableRow
                      key={`${diff.biz_type}-${diff.trade_no}-${diff.diff_type}-${index}`}
                    >
                      <TableCell className='max-w-[220px] truncate font-mono text-xs'>
                        {diff.trade_no || '-'}
                      </TableCell>
                      <TableCell>{diff.biz_type || '-'}</TableCell>
                      <TableCell>
                        <StatusBadge
                          label={getPaymentDiffLabel(t, diff.diff_type)}
                        />
                      </TableCell>
                      <TableCell>
                        <StatusBadge
                          label={getStatusLabel(t, diff.local_status)}
                        />
                      </TableCell>
                      <TableCell>
                        <StatusBadge
                          label={getStatusLabel(t, diff.provider_status)}
                        />
                      </TableCell>
                      <TableCell className='font-mono'>
                        {formatMoney(diff.local_paid_amount)}
                      </TableCell>
                      <TableCell className='font-mono'>
                        {formatMoney(diff.provider_paid_amount)}
                      </TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell
                      colSpan={7}
                      className='text-muted-foreground h-24 text-center'
                    >
                      {t('No reconciliation diffs found')}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setDiffs(null)}>
              {t('Close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
