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
import { useAuthStore, type AuthUser } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { SectionPageLayout } from '@/components/layout'
import { getVipActivationQrPaymentStatusByTradeNo } from '@/features/wallet/api'
import { AffiliateRewardsCard } from '@/features/wallet/components/affiliate-rewards-card'
import { QrPaymentDialog } from '@/features/wallet/components/dialogs/qr-payment-dialog'
import { TransferDialog } from '@/features/wallet/components/dialogs/transfer-dialog'
import { VipActivationCard } from '@/features/wallet/components/vip-activation-card'
import {
  useAffiliate,
  useTopupInfo,
  useQrPaymentSuccessClose,
  useQrPaymentStatusPolling,
  useVipActivation,
  useWalletAccount,
} from '@/features/wallet/hooks'
import { runQrPaymentSuccessRefreshTasks } from '@/features/wallet/lib/qr-payment-refresh'
import type {
  PaymentMethod,
  QrPaymentResult,
  UserWalletData,
} from '@/features/wallet/types'

export function ComputePartners() {
  const { t } = useTranslation()
  const [user, setUser] = useState<UserWalletData | null>(null)
  const [userLoading, setUserLoading] = useState(true)
  const [transferDialogOpen, setTransferDialogOpen] = useState(false)
  const [qrDialogOpen, setQrDialogOpen] = useState(false)
  const [qrPayment, setQrPayment] = useState<QrPaymentResult | null>(null)

  const setAuthUser = useAuthStore((s) => s.auth.setUser)
  const { topupInfo } = useTopupInfo()
  const {
    affiliateLink,
    loading: affiliateLoading,
    refetch: refetchAffiliate,
  } = useAffiliate()
  const {
    account,
    loading: walletAccountLoading,
    transferring,
    refetch: refetchWalletAccount,
    transferCommission,
  } = useWalletAccount()
  const {
    vipInfo,
    loading: vipLoading,
    processing: vipProcessing,
    refreshVipInfo,
    processVipActivation,
  } = useVipActivation()

  const isQrPaymentResult = (result: unknown): result is QrPaymentResult => {
    const candidate = result as Partial<QrPaymentResult>
    return (
      !!result &&
      typeof result === 'object' &&
      candidate.type === 'qr' &&
      typeof candidate.code_url === 'string'
    )
  }

  const fetchUser = useCallback(async (): Promise<boolean> => {
    try {
      setUserLoading(true)
      const response = await getSelf()
      if (response.success && response.data) {
        setUser(response.data as UserWalletData)
        setAuthUser(response.data as AuthUser)
        return true
      }
      return false
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch user data:', error)
      return false
    } finally {
      setUserLoading(false)
    }
  }, [setAuthUser])

  const refreshComputePartnerData = useCallback(async () => {
    return runQrPaymentSuccessRefreshTasks([
      fetchUser,
      refetchAffiliate,
      refetchWalletAccount,
      refreshVipInfo,
    ])
  }, [fetchUser, refetchAffiliate, refetchWalletAccount, refreshVipInfo])

  const pollVipActivationQrPaymentStatus = useCallback(
    async (_payment: QrPaymentResult, tradeNo: string) =>
      getVipActivationQrPaymentStatusByTradeNo(tradeNo),
    []
  )

  const qrPaymentPolling = useQrPaymentStatusPolling({
    open: qrDialogOpen,
    payment: qrPayment,
    enabled: qrPayment?.purpose === 'vvip_activation',
    pollStatus: pollVipActivationQrPaymentStatus,
    onSuccess: refreshComputePartnerData,
    onFallbackRefresh: refreshComputePartnerData,
    successMessage: 'Activation successful',
  })

  useEffect(() => {
    void fetchUser()
  }, [fetchUser])

  const handleVipActivationPay = async (method: PaymentMethod) => {
    const result = await processVipActivation(method)
    if (isQrPaymentResult(result)) {
      qrPaymentPolling.activatePayment(result)
      setQrPayment(result)
      setQrDialogOpen(true)
      return
    }

    if (result) {
      await refreshComputePartnerData()
    }
  }

  const handleTransfer = async (amount: number) => {
    const success = await transferCommission(amount)
    if (success) {
      await refreshComputePartnerData()
    }
    return success
  }

  const {
    successClosing: qrSuccessClosing,
    closeAfterSuccess: handleQrSuccessClose,
    handleOpenChange: handleQrDialogOpenChange,
  } = useQrPaymentSuccessClose({
    status: qrPaymentPolling.status,
    refresh: refreshComputePartnerData,
    closeSession: qrPaymentPolling.closeSession,
    setOpen: setQrDialogOpen,
    failureMessageKey:
      'Activation succeeded but the latest compute partner data could not be refreshed. Please refresh the page to confirm your VVIP status.',
    failureLogMessage:
      'Failed to refresh compute partner data after QR payment:',
  })

  const handleQrRefresh = async () => {
    await qrPaymentPolling.refresh()
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Compute Partners')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t('Manage compute partner activation and referral rewards')}
        </SectionPageLayout.Description>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <VipActivationCard
              vipInfo={vipInfo}
              loading={vipLoading}
              processing={vipProcessing}
              onPay={handleVipActivationPay}
            />

            <AffiliateRewardsCard
              user={user}
              affiliateLink={affiliateLink}
              commissionAmount={account?.commission_amount ?? 0}
              totalCommissionAmount={account?.total_commission_amount ?? 0}
              onTransfer={() => setTransferDialogOpen(true)}
              complianceConfirmed={
                topupInfo?.payment_compliance_confirmed !== false
              }
              loading={userLoading || affiliateLoading || walletAccountLoading}
            />

          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <TransferDialog
        open={transferDialogOpen}
        onOpenChange={setTransferDialogOpen}
        onConfirm={handleTransfer}
        availableAmount={account?.commission_amount ?? 0}
        transferring={transferring}
      />

      <QrPaymentDialog
        open={qrDialogOpen}
        onOpenChange={handleQrDialogOpenChange}
        payment={qrPayment}
        status={qrPaymentPolling.status}
        refreshing={qrPaymentPolling.refreshing}
        onRefresh={handleQrRefresh}
        onSuccessClose={handleQrSuccessClose}
        successClosing={qrSuccessClosing}
      />
    </>
  )
}
