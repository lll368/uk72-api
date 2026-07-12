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
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  getWalletCommissions,
  getWalletFlows,
  getWalletWithdraws,
} from '../api'
import { formatCurrency } from '../lib/format'
import {
  getPiggyActualReceivedAmount,
  getPiggyPlatformFeeAmount,
  getPiggyProviderFeeAmount,
  getPiggyTaxBeforeAmount,
  PIGGY_WITHDRAW_PROVIDER,
} from '../lib/piggy-withdraw-amounts'
import type { CommissionRecord, WalletFlow, WithdrawOrder } from '../types'

interface WalletRecordsCardProps {
  refreshKey: number
}

type WalletRecordTab = 'flows' | 'commissions' | 'withdraws'

const PAGE_SIZE = 10
const WALLET_FLOW_SMALL_AMOUNT_FRACTION_DIGITS = 6

export const walletFlowTableHeaders = [
  'Type',
  'Amount',
  'Balance after',
  'Remark',
  'Time',
] as const

function formatTime(timestamp?: number) {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function isQiniuCostDetailBucketFlow(flow: WalletFlow) {
  return (
    flow.biz_no?.startsWith('qiniu:billing_bucket:') ||
    flow.remark?.includes('账单延迟对账')
  )
}

function isQiniuRealtimeConsumptionFlow(flow: WalletFlow) {
  return flow.biz_no?.startsWith('qiniu:realtime:')
}

function isQiniuOfficialSyncConsumptionFlow(flow: WalletFlow) {
  return (
    flow.biz_no?.startsWith('qiniu:official_ledger:') ||
    flow.biz_no?.startsWith('qiniu:usage_apply:') ||
    flow.remark?.includes('官方同步') ||
    flow.remark?.includes('官方用量同步') ||
    flow.remark?.includes('official sync') ||
    flow.remark?.includes('source=official_sync') ||
    flow.remark?.includes('local_realtime_status=missing')
  )
}

export function getWalletFlowLabel(
  t: (key: string) => string,
  flow: WalletFlow
) {
  if (isQiniuRealtimeConsumptionFlow(flow)) {
    return t('Token/model consumption')
  }
  if (isQiniuOfficialSyncConsumptionFlow(flow)) {
    return t('Official synchronized consumption')
  }
  if (isQiniuCostDetailBucketFlow(flow)) {
    return t('Delayed billing settlement')
  }
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
  const value = flow.flow_type
  return t(map[value || ''] || value || 'Unknown')
}

function getStatusLabel(t: (key: string) => string, value?: string) {
  const map: Record<string, string> = {
    active: 'Active',
    approved: 'Approved',
    disabled: 'Disabled',
    failed: 'Failed',
    paid: 'Paid',
    pending: 'Pending',
    rejected: 'Rejected',
    reversed: 'Reversed',
    submitted: 'Submitted',
    settled: 'Settled',
    success: 'Success',
    await_confirm: 'Awaiting confirmation',
    confirming: 'Confirming payment',
    cancelling: 'Cancelling',
    confirmed: 'Confirmed',
    cancelled: 'Cancelled',
    manual_review: 'Manual review',
  }
  return t(map[value || ''] || value || 'Unknown')
}

function getCommissionSourceLabel(
  t: (key: string, options?: Record<string, unknown>) => string,
  record: CommissionRecord
) {
  const userLabel =
    record.source_user_label?.trim() ||
    t('User #{{id}}', { id: record.source_user_id })
  if (record.source_type === 'topup') {
    return t('{{user}} top-up commission', { user: userLabel })
  }
  if (record.source_type === 'vip_activation') {
    return t('{{user}} VVIP activation commission', { user: userLabel })
  }
  return t('{{user}} commission', { user: userLabel })
}

function formatOptionalCurrency(value?: number) {
  return typeof value === 'number' && Number.isFinite(value)
    ? formatCurrency(value)
    : '-'
}

export function WalletRecordsCard({ refreshKey }: WalletRecordsCardProps) {
  const { t } = useTranslation()
  const [flows, setFlows] = useState<WalletFlow[]>([])
  const [commissions, setCommissions] = useState<CommissionRecord[]>([])
  const [withdraws, setWithdraws] = useState<WithdrawOrder[]>([])
  const [activeTab, setActiveTab] = useState<WalletRecordTab>('flows')
  const [pages, setPages] = useState<Record<WalletRecordTab, number>>({
    flows: 1,
    commissions: 1,
    withdraws: 1,
  })
  const [totals, setTotals] = useState<Record<WalletRecordTab, number>>({
    flows: 0,
    commissions: 0,
    withdraws: 0,
  })
  const [loading, setLoading] = useState(false)

  const fetchRecords = useCallback(async () => {
    try {
      setLoading(true)
      const [flowResp, commissionResp, withdrawResp] = await Promise.all([
        getWalletFlows(pages.flows, PAGE_SIZE),
        getWalletCommissions(pages.commissions, PAGE_SIZE),
        getWalletWithdraws(pages.withdraws, PAGE_SIZE),
      ])
      if (flowResp.success && flowResp.data) {
        setFlows(flowResp.data.items || [])
        setTotals((value) => ({ ...value, flows: flowResp.data?.total || 0 }))
      }
      if (commissionResp.success && commissionResp.data) {
        setCommissions(commissionResp.data.items || [])
        setTotals((value) => ({
          ...value,
          commissions: commissionResp.data?.total || 0,
        }))
      }
      if (withdrawResp.success && withdrawResp.data) {
        setWithdraws(withdrawResp.data.items || [])
        setTotals((value) => ({
          ...value,
          withdraws: withdrawResp.data?.total || 0,
        }))
      }
    } finally {
      setLoading(false)
    }
  }, [pages])

  useEffect(() => {
    fetchRecords()
  }, [fetchRecords, refreshKey])

  const changePage = (tab: WalletRecordTab, nextPage: number) => {
    setPages((value) => ({
      ...value,
      [tab]: Math.max(1, nextPage),
    }))
  }

  const currentPage = pages[activeTab]
  const currentTotal = totals[activeTab]
  const pageCount = Math.max(1, Math.ceil(currentTotal / PAGE_SIZE))

  return (
    <Card className='border-purple-100 bg-gradient-to-br from-purple-50 via-blue-50 to-indigo-50 py-0'>
      <CardHeader className='flex flex-row items-center justify-between gap-3 p-4'>
        <CardTitle className='text-base'>{t('Wallet Records')}</CardTitle>
        <Button
          variant='outline'
          size='sm'
          onClick={fetchRecords}
          disabled={loading}
        >
          <RefreshCw className={loading ? 'size-4 animate-spin' : 'size-4'} />
          {t('Refresh')}
        </Button>
      </CardHeader>
      <CardContent className='p-0'>
        <Tabs
          value={activeTab}
          onValueChange={(value) => setActiveTab(value as WalletRecordTab)}
        >
          <div className='px-4 pb-3'>
            <TabsList>
              <TabsTrigger value='flows'>{t('Flows')}</TabsTrigger>
              <TabsTrigger value='commissions'>{t('Commissions')}</TabsTrigger>
              <TabsTrigger value='withdraws'>{t('Withdrawals')}</TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value='flows' className='m-0'>
            <div className='overflow-x-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    {walletFlowTableHeaders.map((header) => (
                      <TableHead
                        key={header}
                        className={
                          header === 'Amount' || header === 'Balance after'
                            ? 'whitespace-nowrap'
                            : undefined
                        }
                      >
                        {t(header)}
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {flows.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={walletFlowTableHeaders.length}
                        className='text-muted-foreground h-24 text-center'
                      >
                        {t('No wallet records found')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    flows.map((flow) => (
                      <TableRow key={flow.id}>
                        <TableCell>{getWalletFlowLabel(t, flow)}</TableCell>
                        <TableCell className='font-mono'>
                          {flow.direction === 'out' ? '-' : '+'}
                          {formatCurrency(flow.amount, {
                            smallAmountFractionDigits:
                              WALLET_FLOW_SMALL_AMOUNT_FRACTION_DIGITS,
                          })}
                        </TableCell>
                        <TableCell className='font-mono whitespace-nowrap'>
                          {formatCurrency(flow.balance_after, {
                            smallAmountFractionDigits:
                              WALLET_FLOW_SMALL_AMOUNT_FRACTION_DIGITS,
                          })}
                        </TableCell>
                        <TableCell className='text-muted-foreground max-w-[260px] truncate text-xs'>
                          <div className='truncate'>{flow.remark || '-'}</div>
                          {isQiniuCostDetailBucketFlow(flow) &&
                          flow.balance_after < 0 ? (
                            <div className='mt-1 inline-flex items-center gap-1 text-red-600 dark:text-red-400'>
                              <AlertTriangle className='size-3' />
                              {t('Debt after delayed settlement')}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell>{formatTime(flow.created_at)}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </TabsContent>

          <TabsContent value='commissions' className='m-0'>
            <div className='overflow-x-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Source')}</TableHead>
                    <TableHead>{t('Amount')}</TableHead>
                    <TableHead>{t('Time')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {commissions.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={3}
                        className='text-muted-foreground h-24 text-center'
                      >
                        {t('No commission records found')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    commissions.map((record) => (
                      <TableRow key={record.id}>
                        <TableCell>
                          <div>{getCommissionSourceLabel(t, record)}</div>
                          <div className='text-muted-foreground max-w-[260px] truncate font-mono text-xs'>
                            {record.source_order_no}
                          </div>
                        </TableCell>
                        <TableCell className='font-mono'>
                          {formatCurrency(record.amount)}
                        </TableCell>
                        <TableCell>{formatTime(record.created_at)}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </TabsContent>

          <TabsContent value='withdraws' className='m-0'>
            <div className='overflow-x-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Withdraw No.')}</TableHead>
                    <TableHead>{t('Amount')}</TableHead>
                    <TableHead>{t('Bank card')}</TableHead>
                    <TableHead>{t('Status')}</TableHead>
                    <TableHead>{t('Time')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {withdraws.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={5}
                        className='text-muted-foreground h-24 text-center'
                      >
                        {t('No withdraw records found')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    withdraws.map((order) => (
                      <TableRow key={order.id}>
                        <TableCell className='max-w-[220px] truncate font-mono text-xs'>
                          {order.withdraw_no}
                        </TableCell>
                        <TableCell className='font-mono'>
                          {formatCurrency(order.amount)}
                          {order.provider === PIGGY_WITHDRAW_PROVIDER ? (
                            <div className='text-muted-foreground mt-1 space-y-0.5 text-xs'>
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
                          ) : order.actual_amount > 0 ? (
                            <div className='text-muted-foreground text-xs'>
                              {t('Actual received')}:{' '}
                              {formatCurrency(order.actual_amount)}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell>
                          <div>
                            {order.bank_name || order.receive_type || '-'}
                          </div>
                          <div className='text-muted-foreground max-w-[180px] truncate text-xs'>
                            {order.receive_account || '-'}
                          </div>
                        </TableCell>
                        <TableCell>{getStatusLabel(t, order.status)}</TableCell>
                        <TableCell>{formatTime(order.created_at)}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </TabsContent>
        </Tabs>
        <div className='flex flex-col gap-2 border-t p-3 text-sm sm:flex-row sm:items-center sm:justify-between'>
          <div className='text-muted-foreground'>
            {t('Total {{count}} records', { count: currentTotal })}
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              disabled={loading || currentPage <= 1}
              onClick={() => changePage(activeTab, currentPage - 1)}
            >
              {t('Previous page')}
            </Button>
            <span className='text-muted-foreground min-w-20 text-center'>
              {currentPage} / {pageCount}
            </span>
            <Button
              variant='outline'
              size='sm'
              disabled={loading || currentPage >= pageCount}
              onClick={() => changePage(activeTab, currentPage + 1)}
            >
              {t('Next page')}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
