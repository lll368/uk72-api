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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  approveAdminWalletWithdraw,
  failAdminWalletWithdraw,
  getAdminWalletCommissions,
  getAdminWalletWithdraws,
  payAdminWalletWithdraw,
  rejectAdminWalletWithdraw,
} from '../../api'
import { formatCurrency } from '../../lib/format'
import {
  getPiggyActualReceivedAmount,
  getPiggyPlatformFeeAmount,
  getPiggyProviderFeeAmount,
  getPiggyTaxBeforeAmount,
  PIGGY_WITHDRAW_PROVIDER,
} from '../../lib/piggy-withdraw-amounts'
import type { ApiResponse, CommissionRecord, WithdrawOrder } from '../../types'

interface AdminWalletDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

function formatOptionalCurrency(value?: number) {
  return typeof value === 'number' && Number.isFinite(value)
    ? formatCurrency(value)
    : '-'
}

export function AdminWalletDialog({
  open,
  onOpenChange,
}: AdminWalletDialogProps) {
  const { t } = useTranslation()
  const [commissions, setCommissions] = useState<CommissionRecord[]>([])
  const [withdraws, setWithdraws] = useState<WithdrawOrder[]>([])
  const [voucher, setVoucher] = useState('')
  const [failReason, setFailReason] = useState('')

  const fetchRecords = useCallback(async () => {
    const [commissionResp, withdrawResp] = await Promise.all([
      getAdminWalletCommissions(1, 20),
      getAdminWalletWithdraws(1, 20),
    ])
    if (commissionResp.success && commissionResp.data) {
      setCommissions(commissionResp.data.items || [])
    }
    if (withdrawResp.success && withdrawResp.data) {
      setWithdraws(withdrawResp.data.items || [])
    }
  }, [])

  useEffect(() => {
    if (open) {
      fetchRecords()
    }
  }, [fetchRecords, open])

  const runAction = async (action: () => Promise<ApiResponse>) => {
    try {
      const response = await action()
      if (response.success || response.message === 'success') {
        toast.success(t('Operation successful'))
        await fetchRecords()
        return
      }
      toast.error(response.message || t('Operation failed'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Operation failed')
      )
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[calc(100dvh-2rem)] flex-col max-sm:h-dvh max-sm:w-screen max-sm:max-w-none sm:max-w-5xl'>
        <DialogHeader>
          <DialogTitle>{t('Wallet Admin')}</DialogTitle>
          <DialogDescription>
            {t('Review commission records and process withdrawal orders')}
          </DialogDescription>
        </DialogHeader>

        <Tabs defaultValue='withdraws' className='min-h-0 flex-1'>
          <TabsList>
            <TabsTrigger value='withdraws'>{t('Withdrawals')}</TabsTrigger>
            <TabsTrigger value='commissions'>{t('Commissions')}</TabsTrigger>
          </TabsList>

          <TabsContent value='withdraws' className='min-h-0 flex-1'>
            <div className='mb-3 flex items-center gap-2'>
              <Input
                value={voucher}
                onChange={(event) => setVoucher(event.target.value)}
                placeholder={t('Payment voucher')}
                className='max-w-sm'
              />
              <Input
                value={failReason}
                onChange={(event) => setFailReason(event.target.value)}
                placeholder={t('Fail Reason')}
                className='max-w-sm'
              />
              <Button variant='outline' onClick={fetchRecords}>
                {t('Refresh')}
              </Button>
            </div>
            <ScrollArea className='h-[520px] pr-3'>
              <div className='space-y-2'>
                {withdraws.map((order) => (
                  <div key={order.id} className='rounded-lg border p-3'>
                    <div className='flex flex-wrap items-start justify-between gap-3'>
                      <div>
                        <div className='font-mono text-xs'>
                          {order.withdraw_no}
                        </div>
                        <div className='mt-1 text-sm'>
                          {t('User ID')}: {order.user_id} ·{' '}
                          {t('Requested amount')}:{' '}
                          {formatCurrency(order.amount)} · {order.status}
                        </div>
                        {order.provider === PIGGY_WITHDRAW_PROVIDER ? (
                          <div className='text-muted-foreground mt-1 grid gap-x-3 gap-y-0.5 text-xs sm:grid-cols-2'>
                            <div>
                              {t('Platform fee amount')}:{' '}
                              {formatOptionalCurrency(
                                getPiggyPlatformFeeAmount(order)
                              )}
                            </div>
                            <div>
                              {t('Piggy tax-before amount')}:{' '}
                              {formatOptionalCurrency(
                                getPiggyTaxBeforeAmount(order)
                              )}
                            </div>
                            <div>
                              {t('Provider fee')}:{' '}
                              {formatOptionalCurrency(
                                getPiggyProviderFeeAmount(order)
                              )}
                            </div>
                            <div>
                              {t('Actual received')}:{' '}
                              {formatOptionalCurrency(
                                getPiggyActualReceivedAmount(order)
                              )}
                            </div>
                          </div>
                        ) : null}
                        <div className='text-muted-foreground mt-1 text-xs'>
                          {order.provider === PIGGY_WITHDRAW_PROVIDER
                            ? t('Piggy Labor V3')
                            : order.receive_type}{' '}
                          · {order.receive_account || order.bank_name || '-'}
                        </div>
                      </div>
                      <div className='flex flex-wrap gap-2'>
                        {order.provider !== 'piggy_labor_v3' &&
                        order.status === 'pending' ? (
                          <Button
                            size='sm'
                            variant='outline'
                            onClick={() =>
                              runAction(() =>
                                approveAdminWalletWithdraw(order.id)
                              )
                            }
                          >
                            {t('Approve')}
                          </Button>
                        ) : null}
                        {order.provider !== 'piggy_labor_v3' &&
                        (order.status === 'approved' ||
                          order.status === 'failed') ? (
                          <Button
                            size='sm'
                            onClick={() =>
                              runAction(() =>
                                payAdminWalletWithdraw(order.id, voucher)
                              )
                            }
                          >
                            {t('Mark Paid')}
                          </Button>
                        ) : null}
                        {order.provider !== 'piggy_labor_v3' &&
                        order.status === 'approved' ? (
                          <Button
                            size='sm'
                            variant='outline'
                            onClick={() =>
                              runAction(() =>
                                failAdminWalletWithdraw(
                                  order.id,
                                  failReason.trim() || 'payment failed'
                                )
                              )
                            }
                          >
                            {t('Mark Failed')}
                          </Button>
                        ) : null}
                        {order.provider !== 'piggy_labor_v3' &&
                        order.status !== 'paid' &&
                        order.status !== 'rejected' ? (
                          <Button
                            size='sm'
                            variant='destructive'
                            onClick={() =>
                              runAction(() =>
                                rejectAdminWalletWithdraw(order.id, 'rejected')
                              )
                            }
                          >
                            {t('Reject')}
                          </Button>
                        ) : null}
                      </div>
                    </div>
                  </div>
                ))}
                {withdraws.length === 0 ? (
                  <div className='text-muted-foreground rounded-lg border p-8 text-center'>
                    {t('No withdraw records found')}
                  </div>
                ) : null}
              </div>
            </ScrollArea>
          </TabsContent>

          <TabsContent value='commissions' className='min-h-0 flex-1'>
            <ScrollArea className='h-[560px] pr-3'>
              <div className='space-y-2'>
                {commissions.map((record) => (
                  <div key={record.id} className='rounded-lg border p-3'>
                    <div className='flex flex-wrap items-center justify-between gap-3'>
                      <div>
                        <div className='font-mono text-xs'>
                          {record.source_type} · {record.source_order_no}
                        </div>
                        <div className='mt-1 text-sm'>
                          {t('User ID')}: {record.beneficiary_user_id} ·{' '}
                          {t('Level')}: {record.level}
                        </div>
                      </div>
                      <div className='text-right'>
                        <div className='font-mono font-semibold'>
                          {formatCurrency(record.amount)}
                        </div>
                        <div className='text-muted-foreground text-xs'>
                          {record.status}
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
                {commissions.length === 0 ? (
                  <div className='text-muted-foreground rounded-lg border p-8 text-center'>
                    {t('No commission records found')}
                  </div>
                ) : null}
              </div>
            </ScrollArea>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
