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
import { getAdminCommissions } from '../api'
import type { CommissionRecord } from '../types'
import {
  ADMIN_FINANCE_PAGE_SIZE,
  FilterInput,
  PaginationBar,
  StatusBadge,
  StatusFilter,
  TableShell,
  formatMoney,
  formatTime,
  getStatusLabel,
} from './shared'

export function CommissionRecordsTable() {
  const { t } = useTranslation()
  const [items, setItems] = useState<CommissionRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [userId, setUserId] = useState('')
  const [status, setStatus] = useState('')

  const fetchItems = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getAdminCommissions({
        page,
        pageSize: ADMIN_FINANCE_PAGE_SIZE,
        userId,
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
  }, [page, status, t, userId])

  useEffect(() => {
    fetchItems()
  }, [fetchItems])

  return (
    <TableShell
      title={t('Commissions')}
      description={t(
        'Review settlement status for top-up and VVIP commissions'
      )}
      loading={loading}
      onRefresh={fetchItems}
      refreshLabel={t('Refresh')}
      filters={
        <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap'>
          <FilterInput
            value={userId}
            onChange={(value) => {
              setUserId(value)
              setPage(1)
            }}
            placeholder={t('Beneficiary user ID')}
          />
          <StatusFilter
            value={status}
            onChange={(value) => {
              setStatus(value)
              setPage(1)
            }}
            options={['pending', 'settled', 'failed', 'reversed']}
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
              <TableHead>{t('Beneficiary')}</TableHead>
              <TableHead>{t('Source')}</TableHead>
              <TableHead>{t('Level')}</TableHead>
              <TableHead>{t('Base Amount')}</TableHead>
              <TableHead>{t('Amount')}</TableHead>
              <TableHead>{t('Qualification')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Error')}</TableHead>
              <TableHead>{t('Created At')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={9}
                  className='text-muted-foreground h-24 text-center'
                >
                  {t('No commission records found')}
                </TableCell>
              </TableRow>
            ) : (
              items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell>{item.beneficiary_user_id}</TableCell>
                  <TableCell>
                    <div>{item.source_type}</div>
                    <div className='text-muted-foreground max-w-[220px] truncate font-mono text-xs'>
                      {item.source_order_no}
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      {t('Source User')}: {item.source_user_id}
                    </div>
                  </TableCell>
                  <TableCell>{item.level}</TableCell>
                  <TableCell className='font-mono'>
                    {formatMoney(item.base_amount)}
                  </TableCell>
                  <TableCell className='font-mono'>
                    {formatMoney(item.amount)}
                  </TableCell>
                  <TableCell>
                    <StatusBadge
                      label={getStatusLabel(t, item.qualification_status)}
                    />
                  </TableCell>
                  <TableCell>
                    <StatusBadge label={getStatusLabel(t, item.status)} />
                  </TableCell>
                  <TableCell className='text-muted-foreground max-w-[220px] truncate text-xs'>
                    {item.error_message || '-'}
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
  )
}
