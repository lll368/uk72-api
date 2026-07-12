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
import { Pencil, RefreshCw, RotateCcw, UsersRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  getVipSubordinates,
  resetVipSubordinateDiscount,
  updateVipSubordinateDiscount,
} from '../api'
import type { VipSubordinate } from '../types'

interface SubordinateDiscountsCardProps {
  enabled: boolean
}

const PAGE_SIZE = 10

function formatTime(timestamp?: number) {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function formatDiscount(value?: number) {
  const discount = Number(value || 1)
  if (discount <= 0 || discount >= 1) return '-'
  const percent = discount * 100
  return `${Number.isInteger(percent) ? percent.toFixed(0) : percent.toFixed(1)}%`
}

function getUserStatusLabel(t: (key: string) => string, status: number) {
  if (status === 1) return t('Enabled')
  if (status === 2) return t('Disabled')
  return t('Unknown')
}

export function SubordinateDiscountsCard(props: SubordinateDiscountsCardProps) {
  const { t } = useTranslation()
  const [items, setItems] = useState<VipSubordinate[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [canSetDiscount, setCanSetDiscount] = useState(false)
  const [minDiscount, setMinDiscount] = useState(1)
  const [target, setTarget] = useState<VipSubordinate | null>(null)
  const [discountInput, setDiscountInput] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const fetchItems = useCallback(async () => {
    if (!props.enabled) return
    setLoading(true)
    try {
      const response = await getVipSubordinates(page, PAGE_SIZE)
      if (response.success && response.data) {
        const nextParentDiscount = response.data.parent_topup_discount || 1
        setItems(response.data.items || [])
        setTotal(response.data.total || 0)
        setCanSetDiscount(Boolean(response.data.can_set_subordinate_discount))
        setMinDiscount(
          response.data.min_subordinate_topup_discount || nextParentDiscount
        )
        return
      }
      toast.error(response.message || t('Failed to load subordinates'))
    } finally {
      setLoading(false)
    }
  }, [page, props.enabled, t])

  useEffect(() => {
    fetchItems()
  }, [fetchItems])

  if (!props.enabled) return null

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const openEditDialog = (item: VipSubordinate) => {
    setTarget(item)
    setDiscountInput(
      item.topup_discount > 0 && item.topup_discount < 1
        ? String(item.topup_discount)
        : ''
    )
  }

  const validateDiscountInput = () => {
    const value = Number(discountInput)
    if (!Number.isFinite(value)) {
      return t('Please enter a valid discount')
    }
    if (value < minDiscount) {
      return t('Discount must be at least {{min}}', {
        min: minDiscount,
      })
    }
    if (value > 1) {
      return t('Discount must be no more than 1')
    }
    return ''
  }

  const handleSave = async () => {
    if (!target) return
    const error = validateDiscountInput()
    if (error) {
      toast.error(error)
      return
    }
    setSubmitting(true)
    try {
      const response = await updateVipSubordinateDiscount(target.child_user_id, {
        topup_discount: Number(discountInput),
      })
      if (response.success) {
        toast.success(t('Discount updated successfully'))
        setTarget(null)
        setDiscountInput('')
        fetchItems()
        return
      }
      toast.error(response.message || t('Operation failed'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleReset = async (item: VipSubordinate) => {
    setSubmitting(true)
    try {
      const response = await resetVipSubordinateDiscount(item.child_user_id)
      if (response.success) {
        toast.success(t('Discount reset successfully'))
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
      <Card className='py-0 bg-gradient-to-br from-purple-50 via-blue-50 to-indigo-50 border-purple-100'>
        <CardHeader className='flex flex-col gap-3 p-4 lg:flex-row lg:items-start lg:justify-between'>
          <div className='flex min-w-0 items-start gap-3'>
            <div className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-lg border'>
              <UsersRound className='text-muted-foreground size-4' />
            </div>
            <div className='min-w-0'>
              <CardTitle className='text-base'>{t('Direct Subordinates')}</CardTitle>
              <p className='text-muted-foreground mt-1 text-sm'>
                {t('Manage recharge discounts for your direct subordinates')}
              </p>
            </div>
          </div>
          <Button
            size='sm'
            variant='outline'
            onClick={fetchItems}
            disabled={loading}
          >
            <RefreshCw data-icon='inline-start' />
            {t('Refresh')}
          </Button>
        </CardHeader>
        <CardContent className='p-0'>
          {!canSetDiscount ? (
            <div className='border-t px-4 py-3 text-sm text-muted-foreground'>
              {t('Your current discount does not allow assigning subordinate discounts')}
            </div>
          ) : null}
          <div className='overflow-x-auto border-t'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('User')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Group')}</TableHead>
                  <TableHead>{t('Balance')}</TableHead>
                  <TableHead>{t('Used')}</TableHead>
                  <TableHead>{t('Bind Time')}</TableHead>
                  <TableHead>{t('Discount')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell colSpan={8} className='h-24'>
                      <Skeleton className='mx-auto h-6 w-48' />
                    </TableCell>
                  </TableRow>
                ) : items.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className='text-muted-foreground h-24 text-center'
                    >
                      {t('No direct subordinates found')}
                    </TableCell>
                  </TableRow>
                ) : (
                  items.map((item) => (
                    <TableRow key={item.relation_id}>
                      <TableCell>
                        <div className='font-medium'>{item.username}</div>
                        <div className='text-muted-foreground text-xs'>
                          ID {item.child_user_id}
                          {item.display_name
                            ? ` / ${item.display_name}`
                            : ''}
                        </div>
                      </TableCell>
                      <TableCell>{getUserStatusLabel(t, item.status)}</TableCell>
                      <TableCell>{item.group || '-'}</TableCell>
                      <TableCell className='font-mono'>
                        {formatQuota(item.quota)}
                      </TableCell>
                      <TableCell className='font-mono'>
                        {formatQuota(item.used_quota)}
                      </TableCell>
                      <TableCell>{formatTime(item.bind_time)}</TableCell>
                      <TableCell className='font-mono'>
                        {formatDiscount(item.topup_discount)}
                      </TableCell>
                      <TableCell className='text-right'>
                        <div className='flex justify-end gap-2'>
                          <Button
                            size='sm'
                            variant='outline'
                            disabled={!canSetDiscount || submitting}
                            onClick={() => openEditDialog(item)}
                          >
                            <Pencil data-icon='inline-start' />
                            {t('Set')}
                          </Button>
                          {item.topup_discount > 0 && item.topup_discount < 1 ? (
                            <Button
                              size='sm'
                              variant='ghost'
                              disabled={submitting}
                              onClick={() => handleReset(item)}
                            >
                              <RotateCcw data-icon='inline-start' />
                              {t('Reset')}
                            </Button>
                          ) : null}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
          <div className='flex items-center justify-between gap-3 border-t px-4 py-3'>
            <div className='text-muted-foreground text-sm'>
              {t('Page')} {page} / {totalPages}
            </div>
            <div className='flex gap-2'>
              <Button
                size='sm'
                variant='outline'
                disabled={page <= 1 || loading}
                onClick={() => setPage((value) => Math.max(1, value - 1))}
              >
                {t('Previous')}
              </Button>
              <Button
                size='sm'
                variant='outline'
                disabled={page >= totalPages || loading}
                onClick={() => setPage((value) => value + 1)}
              >
                {t('Next')}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Dialog open={Boolean(target)} onOpenChange={(open) => !open && setTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Set Subordinate Discount')}</DialogTitle>
            <DialogDescription>
              {t('Discount must be at least {{min}} and no more than 1', {
                min: minDiscount,
              })}
            </DialogDescription>
          </DialogHeader>
          <Input
            type='number'
            min={minDiscount}
            max={1}
            step={0.01}
            value={discountInput}
            onChange={(event) => setDiscountInput(event.target.value)}
            placeholder={t('Discount')}
          />
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setTarget(null)}
              disabled={submitting}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleSave} disabled={submitting}>
              {t('Confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
