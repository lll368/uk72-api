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
import { RefreshCw } from 'lucide-react'
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
import {
  getPiggyActualReceivedAmount,
  getPiggyPlatformFeeAmount,
  getPiggyProviderFeeAmount,
  getPiggyTaxBeforeAmount,
  getPiggyTaxDetails,
  hasPiggyPlatformFeeSnapshot,
  PIGGY_WITHDRAW_PROVIDER,
} from '@/features/wallet/lib/piggy-withdraw-amounts'
import {
  approveAdminWithdraw,
  cancelPiggyWithdraw,
  failAdminWithdraw,
  getAdminWithdraws,
  payAdminWithdraw,
  rejectAdminWithdraw,
  recordPiggyWithdrawManualResult,
  recoverPiggyWithdrawSubmit,
  retryPiggyWithdrawConfirm,
  scanPiggyWithdrawCompensations,
} from '../api'
import {
  shouldRefreshWithdrawOrdersAfterAction,
  type AdminWithdrawAction,
} from '../lib/withdraw-actions'
import type {
  ApiResponse,
  WithdrawApprovalResult,
  WithdrawOrder,
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
  getStatusLabel,
  getWithdrawStatusLabel,
} from './shared'

type WithdrawAction = AdminWithdrawAction

function formatOptionalMoney(value?: number) {
  return typeof value === 'number' && Number.isFinite(value)
    ? formatMoney(value)
    : '-'
}

export function WithdrawOrdersTable() {
  const { t } = useTranslation()
  const [items, setItems] = useState<WithdrawOrder[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [scanning, setScanning] = useState(false)
  const [userId, setUserId] = useState('')
  const [status, setStatus] = useState('')
  const [target, setTarget] = useState<WithdrawOrder | null>(null)
  const [action, setAction] = useState<WithdrawAction | null>(null)
  const [text, setText] = useState('')

  const fetchItems = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getAdminWithdraws({
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

  const openAction = (order: WithdrawOrder, nextAction: WithdrawAction) => {
    setTarget(order)
    setAction(nextAction)
    setText('')
  }

  const closeAction = () => {
    setTarget(null)
    setAction(null)
    setText('')
  }

  const actionTitle = action
    ? {
        approve: t('Approve Withdrawal'),
        reject: t('Reject Withdrawal'),
        pay: t('Mark Withdrawal Paid'),
        fail: t('Mark Withdrawal Failed'),
        retry_confirm: t('Retry Piggy confirmation'),
        recover_submit: t('Recover Piggy submission'),
        cancel_piggy: t('Cancel Piggy withdrawal'),
        manual_paid: t('Mark Withdrawal Paid'),
        manual_failed: t('Mark Withdrawal Failed'),
      }[action]
    : ''

  const handleAction = async () => {
    if (!target || !action) return
    let response: ApiResponse | undefined
    if (action === 'approve') response = await approveAdminWithdraw(target.id)
    if (action === 'reject') {
      response = await rejectAdminWithdraw(target.id, text.trim() || 'rejected')
    }
    if (action === 'pay') {
      response = await payAdminWithdraw(target.id, text.trim())
    }
    if (action === 'fail') {
      response = await failAdminWithdraw(
        target.id,
        text.trim() || 'payment failed'
      )
    }
    if (action === 'retry_confirm') {
      response = await retryPiggyWithdrawConfirm(target.id)
    }
    if (action === 'recover_submit') {
      response = await recoverPiggyWithdrawSubmit(target.id)
    }
    if (action === 'cancel_piggy') {
      response = await cancelPiggyWithdraw(
        target.id,
        text.trim() || 'admin cancelled'
      )
    }
    if (action === 'manual_paid') {
      response = await recordPiggyWithdrawManualResult(
        target.id,
        text.trim() || 'manual paid',
        'manual_paid'
      )
    }
    if (action === 'manual_failed') {
      response = await recordPiggyWithdrawManualResult(
        target.id,
        text.trim() || 'manual failed',
        'manual_failed'
      )
    }
    if (response?.success) {
      const approvalResult = response.data as WithdrawApprovalResult | undefined
      if (
        (action === 'approve' || action === 'recover_submit') &&
        approvalResult?.recoverable &&
        approvalResult.submitted === false
      ) {
        toast.warning(
          approvalResult.message ||
            t(
              'Piggy submission result unknown. Please recover submit later or check network'
            )
        )
      } else {
        toast.success(t('Operation successful'))
      }
      closeAction()
      fetchItems()
      return
    }
    if (shouldRefreshWithdrawOrdersAfterAction(action, false)) {
      fetchItems()
    }
    toast.error(response?.message || t('Operation failed'))
  }

  const handleScan = async () => {
    setScanning(true)
    try {
      const response = await scanPiggyWithdrawCompensations(50)
      if (response.success) {
        toast.success(
          t('Processed {{count}} Piggy withdrawal orders', {
            count: response.data?.processed || 0,
          })
        )
        fetchItems()
        return
      }
      toast.error(response.message || t('Operation failed'))
    } finally {
      setScanning(false)
    }
  }

  return (
    <>
      <TableShell
        title={t('Withdrawals')}
        description={t(
          'Review, approve, reject, and mark commission withdrawals'
        )}
        loading={loading}
        onRefresh={fetchItems}
        refreshLabel={t('Refresh')}
        actions={
          <Button
            size='sm'
            variant='outline'
            onClick={handleScan}
            disabled={scanning}
          >
            <RefreshCw
              data-icon='inline-start'
              className={scanning ? 'animate-spin' : undefined}
            />
            {t('Scan Piggy orders')}
          </Button>
        }
        filters={
          <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap'>
            <FilterInput
              value={userId}
              onChange={(value) => {
                setUserId(value)
                setPage(1)
              }}
              placeholder={t('User ID')}
            />
            <StatusFilter
              value={status}
              onChange={(value) => {
                setStatus(value)
                setPage(1)
              }}
              options={[
                'pending',
                'approved',
                'submitted',
                'await_confirm',
                'confirming',
                'cancelling',
                'confirmed',
                'manual_review',
                'paid',
                'rejected',
                'failed',
                'cancelled',
              ]}
              allLabel={t('All Status')}
              getOptionLabel={(value) => getStatusLabel(t, value)}
            />
          </div>
        }
      >
        <div className='overflow-x-auto'>
            <Table>
              <TableHeader className='bg-card'>
              <TableRow>
                <TableHead>{t('Withdraw No.')}</TableHead>
                <TableHead>{t('User ID')}</TableHead>
                <TableHead>{t('Provider')}</TableHead>
                <TableHead>{t('Amount')}</TableHead>
                <TableHead>{t('Receive Account')}</TableHead>
                <TableHead>{t('Piggy status')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Created At')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={9}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No withdraw records found')}
                  </TableCell>
                </TableRow>
              ) : (
                items.map((item) => {
                  const isPiggy = item.provider === PIGGY_WITHDRAW_PROVIDER
                  const taxDetails = getPiggyTaxDetails(item)
                  const isLocalPiggyReview =
                    isPiggy &&
                    item.status === 'pending' &&
                    !item.external_trade_no
                  const canRetryPiggyConfirm =
                    isPiggy && item.status === 'await_confirm'
                  const canRecoverPiggySubmit =
                    isPiggy &&
                    item.status === 'approved' &&
                    !item.external_trade_no
                  const canCancelPiggy =
                    isPiggy &&
                    ['submitted', 'await_confirm', 'manual_review'].includes(
                      item.status
                    )

                  return (
                    <TableRow key={item.id}>
                      <TableCell className='max-w-[220px] truncate font-mono text-xs'>
                        {item.withdraw_no}
                      </TableCell>
                      <TableCell>{item.user_id}</TableCell>
                      <TableCell>
                        {isPiggy ? t('Piggy Labor V3') : t('Manual')}
                      </TableCell>
                      <TableCell className='font-mono'>
                        <div>
                          {isPiggy ? t('Requested amount') : t('Amount')}:{' '}
                          {formatMoney(item.amount)}
                        </div>
                        {isPiggy ? (
                          <div className='text-muted-foreground mt-1 space-y-0.5 text-xs'>
                            <div>
                              {t('Platform fee rate')}:{' '}
                              {hasPiggyPlatformFeeSnapshot(item)
                                ? `${item.platform_fee_rate}%`
                                : '-'}
                            </div>
                            <div>
                              {t('Platform fee amount')}:{' '}
                              {formatOptionalMoney(
                                getPiggyPlatformFeeAmount(item)
                              )}
                            </div>
                            <div>
                              {t('Piggy tax-before amount')}:{' '}
                              {formatOptionalMoney(
                                getPiggyTaxBeforeAmount(item)
                              )}
                            </div>
                            <div>
                              {t('Provider fee')}:{' '}
                              {formatOptionalMoney(
                                getPiggyProviderFeeAmount(item)
                              )}
                            </div>
                            <div>
                              {t('Actual received')}:{' '}
                              {formatOptionalMoney(
                                getPiggyActualReceivedAmount(item)
                              )}
                            </div>
                          </div>
                        ) : null}
                      </TableCell>
                      <TableCell>
                        <div>{item.bank_name || item.receive_type || '-'}</div>
                        <div className='text-muted-foreground max-w-[180px] truncate text-xs'>
                          {item.receive_account || item.account_name || '-'}
                        </div>
                      </TableCell>
                      <TableCell>
                        {isPiggy ? (
                          <div className='space-y-1 text-xs'>
                            <StatusBadge
                              label={getWithdrawStatusLabel(
                                t,
                                item.piggy_status || item.status
                              )}
                            />
                            <div className='text-muted-foreground font-mono'>
                              {item.front_log_no || item.labor_order_no || '-'}
                            </div>
                            <div className='text-muted-foreground'>
                              {t('Individual tax')}:{' '}
                              {formatOptionalMoney(
                                taxDetails.individualTaxAmount
                              )}
                            </div>
                            <div className='text-muted-foreground'>
                              {t('VAT')}:{' '}
                              {formatOptionalMoney(taxDetails.addedTaxAmount)}
                            </div>
                            <div className='text-muted-foreground'>
                              {t('Tax')}:{' '}
                              {formatOptionalMoney(taxDetails.totalTaxAmount)}
                            </div>
                            {item.manual_review_reason ? (
                              <div className='text-muted-foreground max-w-[220px] truncate'>
                                {item.manual_review_reason}
                              </div>
                            ) : null}
                          </div>
                        ) : (
                          '-'
                        )}
                      </TableCell>
                      <TableCell>
                        <StatusBadge
                          label={getWithdrawStatusLabel(t, item.status)}
                        />
                        {item.fail_reason ? (
                          <div className='text-muted-foreground mt-1 max-w-[180px] truncate text-xs'>
                            {item.fail_reason}
                          </div>
                        ) : null}
                      </TableCell>
                      <TableCell>{formatTime(item.created_at)}</TableCell>
                      <TableCell className='text-right'>
                        <div className='flex flex-wrap justify-end gap-2'>
                          {(!isPiggy && item.status === 'pending') ||
                          isLocalPiggyReview ? (
                            <Button
                              size='sm'
                              variant='outline'
                              onClick={() => openAction(item, 'approve')}
                            >
                              {t('Approve')}
                            </Button>
                          ) : null}
                          {(!isPiggy &&
                            (item.status === 'pending' ||
                              item.status === 'approved' ||
                              item.status === 'failed')) ||
                          isLocalPiggyReview ? (
                            <Button
                              size='sm'
                              variant='destructive'
                              onClick={() => openAction(item, 'reject')}
                            >
                              {t('Reject')}
                            </Button>
                          ) : null}
                          {!isPiggy &&
                          (item.status === 'approved' ||
                            item.status === 'failed') ? (
                            <Button
                              size='sm'
                              onClick={() => openAction(item, 'pay')}
                            >
                              {t('Mark Paid')}
                            </Button>
                          ) : null}
                          {!isPiggy && item.status === 'approved' ? (
                            <Button
                              size='sm'
                              variant='outline'
                              onClick={() => openAction(item, 'fail')}
                            >
                              {t('Mark Failed')}
                            </Button>
                          ) : null}
                          {canRetryPiggyConfirm ? (
                            <Button
                              size='sm'
                              variant='outline'
                              onClick={() => openAction(item, 'retry_confirm')}
                            >
                              {t('Retry Confirm')}
                            </Button>
                          ) : null}
                          {canRecoverPiggySubmit ? (
                            <Button
                              size='sm'
                              variant='outline'
                              onClick={() => openAction(item, 'recover_submit')}
                            >
                              {t('Recover Submit')}
                            </Button>
                          ) : null}
                          {canCancelPiggy ? (
                            <Button
                              size='sm'
                              variant='destructive'
                              onClick={() => openAction(item, 'cancel_piggy')}
                            >
                              {t('Cancel')}
                            </Button>
                          ) : null}
                          {isPiggy && item.status === 'manual_review' ? (
                            <>
                              <Button
                                size='sm'
                                variant='outline'
                                onClick={() => openAction(item, 'manual_paid')}
                              >
                                {t('Mark Paid')}
                              </Button>
                              <Button
                                size='sm'
                                variant='outline'
                                onClick={() =>
                                  openAction(item, 'manual_failed')
                                }
                              >
                                {t('Mark Failed')}
                              </Button>
                            </>
                          ) : null}
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })
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

      <ConfirmDialog
        open={Boolean(target && action)}
        onOpenChange={(open) => !open && closeAction()}
        title={actionTitle}
        desc={t('Confirm this withdrawal operation.')}
        confirmText={t('Confirm')}
        destructive={
          action === 'reject' ||
          action === 'fail' ||
          action === 'cancel_piggy' ||
          action === 'manual_failed'
        }
        handleConfirm={handleAction}
      >
        {action === 'reject' ||
        action === 'fail' ||
        action === 'pay' ||
        action === 'cancel_piggy' ||
        action === 'manual_paid' ||
        action === 'manual_failed' ? (
          <Input
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder={
              action === 'pay' || action === 'manual_paid'
                ? t('Payment voucher')
                : t('Reason')
            }
          />
        ) : null}
      </ConfirmDialog>
    </>
  )
}
