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
import { RefreshCw, Wallet } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SectionPageLayout } from '@/components/layout'
import { getWalletWithdraws } from '@/features/wallet/api'
import { WithdrawDialog } from '@/features/wallet/components/dialogs/withdraw-dialog'
import { PiggyContractStatusPanel } from '@/features/wallet/components/piggy-contract-status-panel'
import { useWalletAccount } from '@/features/wallet/hooks'
import { formatCurrency } from '@/features/wallet/lib/format'
import {
  getPiggyActualReceivedAmount,
  getPiggyPlatformFeeAmount,
  getPiggyProviderFeeAmount,
  getPiggyTaxBeforeAmount,
  PIGGY_WITHDRAW_PROVIDER,
} from '@/features/wallet/lib/piggy-withdraw-amounts'
import type {
  PiggyWithdrawSubmitRequest,
  WithdrawOrder,
  WithdrawalProfileInput,
} from '@/features/wallet/types'

const PAGE_SIZE = 10

function formatTime(timestamp?: number) {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function formatOptionalCurrency(value?: number) {
  return typeof value === 'number' && Number.isFinite(value)
    ? formatCurrency(value)
    : '-'
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

export function Withdrawal() {
  const { t } = useTranslation()
  const [withdrawDialogOpen, setWithdrawDialogOpen] = useState(false)
  const [withdraws, setWithdraws] = useState<WithdrawOrder[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [recordsLoading, setRecordsLoading] = useState(false)

  const {
    account,
    withdrawalEligibility,
    withdrawalEligibilityLoadFailed,
    withdrawalProfile,
    minWithdrawAmount,
    loading: walletAccountLoading,
    withdrawing,
    profileSaving,
    signing,
    contractPreviewing,
    withdrawTaxTrial,
    withdrawTaxTrialLoading,
    withdrawTaxTrialError,
    piggySignUrl,
    refetch: refetchWalletAccount,
    submitWithdraw,
    trialWithdrawTax,
    clearWithdrawTaxTrial,
    saveWithdrawalProfile,
    requestPiggySign,
    refreshPiggyContractStatus,
    openPiggyContractPreview,
  } = useWalletAccount()

  const fetchWithdraws = useCallback(async () => {
    try {
      setRecordsLoading(true)
      const response = await getWalletWithdraws(page, PAGE_SIZE)
      if (response.success && response.data) {
        setWithdraws(response.data.items || [])
        setTotal(response.data.total || 0)
      }
    } finally {
      setRecordsLoading(false)
    }
  }, [page])

  useEffect(() => {
    fetchWithdraws()
  }, [fetchWithdraws])

  const refreshAll = useCallback(async () => {
    await Promise.all([refetchWalletAccount(), fetchWithdraws()])
  }, [refetchWalletAccount, fetchWithdraws])

  const handleWithdraw = async (request: PiggyWithdrawSubmitRequest) => {
    const success = await submitWithdraw(request)
    if (success) {
      await refreshAll()
    }
    return success
  }

  const handleSaveWithdrawalProfile = async (
    request: WithdrawalProfileInput
  ) => {
    const success = await saveWithdrawalProfile(request)
    if (success) {
      await refreshAll()
    }
    return success
  }

  const handleRequestPiggySign = async () => {
    const success = await requestPiggySign()
    if (success) {
      await refreshAll()
    }
    return success
  }

  const handleRefreshPiggyContractStatus = async () => {
    const success = await refreshPiggyContractStatus()
    if (success) {
      await refreshAll()
    }
    return success
  }

  const commissionAmount = account?.commission_amount ?? 0
  const frozenCommission = account?.frozen_commission_amount ?? 0
  const totalCommission = account?.total_commission_amount ?? 0
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Commission withdrawal')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t('Bank card commission withdrawal entry')}
        </SectionPageLayout.Description>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <Card className='border-purple-100 bg-gradient-to-br from-purple-50 via-blue-50 to-indigo-50'>
              <CardHeader className='flex flex-row items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <CardTitle className='text-base'>
                    {t('Commission withdrawal')}
                  </CardTitle>
                  <CardDescription>
                    {t(
                      'Only withdrawable commission can be withdrawn. Balance cannot be withdrawn.'
                    )}
                  </CardDescription>
                </div>
                <Button
                  onClick={() => setWithdrawDialogOpen(true)}
                  disabled={walletAccountLoading || commissionAmount <= 0}
                >
                  <Wallet data-icon='inline-start' />
                  {t('Withdraw Commission')}
                </Button>
              </CardHeader>
              <CardContent>
                <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
                  <div className='rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs font-medium uppercase'>
                      {t('Available')}
                    </div>
                    <div className='mt-1 font-mono text-lg font-semibold'>
                      {formatCurrency(commissionAmount)}
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs font-medium uppercase'>
                      {t('Frozen commission after')}
                    </div>
                    <div className='mt-1 font-mono text-lg font-semibold'>
                      {formatCurrency(frozenCommission)}
                    </div>
                  </div>
                  <div className='rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs font-medium uppercase'>
                      {t('Commissions')}
                    </div>
                    <div className='mt-1 font-mono text-lg font-semibold'>
                      {formatCurrency(totalCommission)}
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            <PiggyContractStatusPanel
              profile={withdrawalProfile}
              eligibility={withdrawalEligibility}
              loading={walletAccountLoading}
              eligibilityLoadFailed={withdrawalEligibilityLoadFailed}
              profileSaving={profileSaving}
              signing={signing}
              contractPreviewing={contractPreviewing}
              piggySignUrl={piggySignUrl}
              onRefresh={refreshAll}
              onRefreshContractStatus={handleRefreshPiggyContractStatus}
              onOpenContract={openPiggyContractPreview}
              onSaveProfile={handleSaveWithdrawalProfile}
              onRequestSign={handleRequestPiggySign}
            />

            <Card className='border-purple-100 bg-gradient-to-br from-purple-50 via-blue-50 to-indigo-50 py-0'>
              <CardHeader className='flex flex-row items-center justify-between gap-3 p-4'>
                <CardTitle className='text-base'>{t('Withdrawals')}</CardTitle>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={refreshAll}
                  disabled={recordsLoading || walletAccountLoading}
                >
                  <RefreshCw
                    className={
                      recordsLoading ? 'size-4 animate-spin' : 'size-4'
                    }
                  />
                  {t('Refresh')}
                </Button>
              </CardHeader>
              <CardContent className='p-0'>
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
                            <TableCell>
                              {getStatusLabel(t, order.status)}
                            </TableCell>
                            <TableCell>
                              {formatTime(order.created_at)}
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </div>
                <div className='flex flex-col gap-2 border-t p-3 text-sm sm:flex-row sm:items-center sm:justify-between'>
                  <div className='text-muted-foreground'>
                    {t('Total {{count}} records', { count: total })}
                  </div>
                  <div className='flex items-center gap-2'>
                    <Button
                      variant='outline'
                      size='sm'
                      disabled={recordsLoading || page <= 1}
                      onClick={() => setPage((value) => Math.max(1, value - 1))}
                    >
                      {t('Previous page')}
                    </Button>
                    <span className='text-muted-foreground min-w-20 text-center'>
                      {page} / {pageCount}
                    </span>
                    <Button
                      variant='outline'
                      size='sm'
                      disabled={recordsLoading || page >= pageCount}
                      onClick={() => setPage((value) => value + 1)}
                    >
                      {t('Next page')}
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <WithdrawDialog
        open={withdrawDialogOpen}
        onOpenChange={setWithdrawDialogOpen}
        onSubmit={handleWithdraw}
        availableAmount={commissionAmount}
        minAmount={minWithdrawAmount}
        submitting={withdrawing}
        eligibility={withdrawalEligibility}
        taxTrial={withdrawTaxTrial}
        taxTrialLoading={withdrawTaxTrialLoading}
        taxTrialError={withdrawTaxTrialError}
        onTaxTrial={trialWithdrawTax}
        onTaxTrialReset={clearWithdrawTaxTrial}
      />
    </>
  )
}
