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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  getWalletPiggyContractPreview,
  getWalletAccount,
  getWalletPiggySignUrl,
  getWalletWithdrawalEligibility,
  refreshWalletPiggyContractStatus,
  saveWalletWithdrawalProfile,
  submitPiggyWalletWithdraw,
  trialPiggyWalletWithdrawTax,
  transferWalletCommission,
} from '../api'
import { loadWalletAccountSequentially } from '../lib/wallet-account-loader'
import type {
  ApiResponse,
  PiggyContractPreviewResponse,
  PiggyTaxTrialResult,
  PiggyWithdrawSubmitRequest,
  WalletAccount,
  WithdrawalEligibility,
  WithdrawalProfile,
  WithdrawalProfileInput,
} from '../types'

interface PiggyContractPreviewWindow {
  opener: unknown
  location: {
    href: string
  }
  close: () => void
}

interface OpenPiggyContractPreviewWindowOptions {
  openWindow: () => PiggyContractPreviewWindow | null
  loadPreview: () => Promise<ApiResponse<PiggyContractPreviewResponse>>
  onError: (message: string) => void
}

interface ClearedWithdrawTaxTrialState {
  requestSeq: number
  taxTrial: PiggyTaxTrialResult | null
  loading: boolean
  error: string
}

export function getClearedWithdrawTaxTrialState(
  currentRequestSeq: number
): ClearedWithdrawTaxTrialState {
  return {
    requestSeq: currentRequestSeq + 1,
    taxTrial: null,
    loading: false,
    error: '',
  }
}

export async function openPiggyContractPreviewWindow({
  openWindow,
  loadPreview,
  onError,
}: OpenPiggyContractPreviewWindowOptions): Promise<boolean> {
  const previewWindow = openWindow()
  if (!previewWindow) {
    onError('Please allow pop-ups to open the contract')
    return false
  }
  previewWindow.opener = null

  try {
    const response = await loadPreview()
    const previewURL = response.data?.preview_url?.trim() || ''
    if (
      (response.success === true || response.message === 'success') &&
      previewURL
    ) {
      previewWindow.location.href = previewURL
      return true
    }
    previewWindow.close()
    onError(
      response.message && response.message !== 'success'
        ? response.message
        : 'Failed to open contract'
    )
    return false
  } catch {
    previewWindow.close()
    onError('Failed to open contract')
    return false
  }
}

export function useWalletAccount() {
  const { t } = useTranslation()
  const [account, setAccount] = useState<WalletAccount | null>(null)
  const [withdrawalEligibility, setWithdrawalEligibility] =
    useState<WithdrawalEligibility | null>(null)
  const [withdrawalEligibilityLoadFailed, setWithdrawalEligibilityLoadFailed] =
    useState(false)
  const [withdrawalProfile, setWithdrawalProfile] =
    useState<WithdrawalProfile | null>(null)
  const [minWithdrawAmount, setMinWithdrawAmount] = useState(0)
  const [loading, setLoading] = useState(true)
  const [transferring, setTransferring] = useState(false)
  const [withdrawing, setWithdrawing] = useState(false)
  const [profileSaving, setProfileSaving] = useState(false)
  const [signing, setSigning] = useState(false)
  const [contractPreviewing, setContractPreviewing] = useState(false)
  const [withdrawTaxTrial, setWithdrawTaxTrial] =
    useState<PiggyTaxTrialResult | null>(null)
  const [withdrawTaxTrialLoading, setWithdrawTaxTrialLoading] = useState(false)
  const [withdrawTaxTrialError, setWithdrawTaxTrialError] = useState('')
  const [piggySignUrl, setPiggySignUrl] = useState('')
  const contractPreviewingRef = useRef(false)
  const taxTrialRequestSeqRef = useRef(0)

  const fetchWalletAccount = useCallback(async (): Promise<boolean> => {
    try {
      setLoading(true)
      const { accountResponse, eligibilityResponse } =
        await loadWalletAccountSequentially({
          getWalletAccount,
          getWalletWithdrawalEligibility,
        })

      if (accountResponse?.success && accountResponse.data) {
        setAccount(accountResponse.data.account)
        setMinWithdrawAmount(
          accountResponse.data.commission_min_withdraw_amount || 0
        )
      }

      const eligibilityData = eligibilityResponse?.data
      if (eligibilityResponse?.success && eligibilityData) {
        setWithdrawalEligibility(eligibilityData)
        setWithdrawalEligibilityLoadFailed(false)
        setWithdrawalProfile(eligibilityData.profile || null)
        if (eligibilityData.need_profile || !eligibilityData.need_sign) {
          setPiggySignUrl('')
        }
      } else {
        setWithdrawalEligibility(null)
        setWithdrawalEligibilityLoadFailed(true)
        setWithdrawalProfile(null)
        setPiggySignUrl('')
      }
      return !!(accountResponse?.success && accountResponse.data)
    } finally {
      setLoading(false)
    }
  }, [])

  const transferCommission = useCallback(
    async (amount: number): Promise<boolean> => {
      try {
        setTransferring(true)
        const response = await transferWalletCommission({ amount })
        if (response.success || response.message === 'success') {
          toast.success(t('Transfer successful'))
          await fetchWalletAccount()
          return true
        }
        toast.error(response.message || t('Transfer failed'))
        return false
      } catch {
        toast.error(t('Transfer failed'))
        return false
      } finally {
        setTransferring(false)
      }
    },
    [fetchWalletAccount, t]
  )

  const submitWithdraw = useCallback(
    async (request: PiggyWithdrawSubmitRequest): Promise<boolean> => {
      try {
        setWithdrawing(true)
        const response = await submitPiggyWalletWithdraw(request)
        if (response.success || response.message === 'success') {
          toast.success(t('Withdraw request submitted'))
          await fetchWalletAccount()
          return true
        }
        toast.error(response.message || t('Withdraw request failed'))
        return false
      } catch {
        toast.error(t('Withdraw request failed'))
        return false
      } finally {
        setWithdrawing(false)
      }
    },
    [fetchWalletAccount, t]
  )

  const clearWithdrawTaxTrial = useCallback(() => {
    const cleared = getClearedWithdrawTaxTrialState(
      taxTrialRequestSeqRef.current
    )
    taxTrialRequestSeqRef.current = cleared.requestSeq
    setWithdrawTaxTrial(cleared.taxTrial)
    setWithdrawTaxTrialLoading(cleared.loading)
    setWithdrawTaxTrialError(cleared.error)
  }, [])

  const trialWithdrawTax = useCallback(
    async (amount: number): Promise<PiggyTaxTrialResult | null> => {
      if (!Number.isFinite(amount) || amount <= 0) {
        clearWithdrawTaxTrial()
        return null
      }
      const requestSeq = taxTrialRequestSeqRef.current + 1
      taxTrialRequestSeqRef.current = requestSeq
      try {
        setWithdrawTaxTrialLoading(true)
        setWithdrawTaxTrialError('')
        const response = await trialPiggyWalletWithdrawTax({ amount })
        if (requestSeq !== taxTrialRequestSeqRef.current) {
          return null
        }
        if (response.success && response.data) {
          setWithdrawTaxTrial(response.data)
          return response.data
        }
        setWithdrawTaxTrial(null)
        setWithdrawTaxTrialError(
          response.message || t('Tax estimate unavailable')
        )
        return null
      } catch {
        if (requestSeq !== taxTrialRequestSeqRef.current) {
          return null
        }
        setWithdrawTaxTrial(null)
        setWithdrawTaxTrialError(t('Tax estimate unavailable'))
        return null
      } finally {
        if (requestSeq === taxTrialRequestSeqRef.current) {
          setWithdrawTaxTrialLoading(false)
        }
      }
    },
    [clearWithdrawTaxTrial, t]
  )

  const saveWithdrawalProfile = useCallback(
    async (request: WithdrawalProfileInput): Promise<boolean> => {
      try {
        setProfileSaving(true)
        const response = await saveWalletWithdrawalProfile(request)
        if (response.success && response.data) {
          toast.success(t('Withdrawal profile saved'))
          setWithdrawalProfile(response.data)
          setPiggySignUrl('')
          await fetchWalletAccount()
          return true
        }
        toast.error(response.message || t('Failed to save withdrawal profile'))
        return false
      } catch {
        toast.error(t('Failed to save withdrawal profile'))
        return false
      } finally {
        setProfileSaving(false)
      }
    },
    [fetchWalletAccount, t]
  )

  const requestPiggySign = useCallback(async (): Promise<boolean> => {
    try {
      setSigning(true)
      setPiggySignUrl('')
      const response = await getWalletPiggySignUrl()
      if (response.success && response.data) {
        if (response.data.signed) {
          toast.success(t('Contract already signed'))
          setPiggySignUrl('')
          await fetchWalletAccount()
          return true
        }
        if (response.data.sign_url) {
          // 小猪签约链接要求用微信打开，电脑浏览器直接打开不可用；
          // 因此这里只保存链接交给弹窗生成二维码，不再 window.open。
          setPiggySignUrl(response.data.sign_url)
          toast.success(t('Contract sign QR code generated'))
          return true
        }
      }
      toast.error(response.message || t('Failed to get contract sign URL'))
      return false
    } catch {
      toast.error(t('Failed to get contract sign URL'))
      return false
    } finally {
      setSigning(false)
    }
  }, [fetchWalletAccount, t])

  const refreshPiggyContractStatus = useCallback(async (): Promise<boolean> => {
    try {
      setSigning(true)
      const response = await refreshWalletPiggyContractStatus()
      if (response.success && response.data) {
        toast.success(t('Contract status refreshed'))
        setWithdrawalProfile(response.data)
        setPiggySignUrl('')
        await fetchWalletAccount()
        return true
      }
      toast.error(response.message || t('No signed contract found yet'))
      return false
    } catch {
      toast.error(t('No signed contract found yet'))
      return false
    } finally {
      setSigning(false)
    }
  }, [fetchWalletAccount, t])

  const openPiggyContractPreview = useCallback(async (): Promise<boolean> => {
    if (contractPreviewingRef.current) {
      return false
    }
    contractPreviewingRef.current = true
    setContractPreviewing(true)
    try {
      return await openPiggyContractPreviewWindow({
        openWindow: () => window.open('', '_blank'),
        loadPreview: getWalletPiggyContractPreview,
        onError: (message) => toast.error(t(message)),
      })
    } finally {
      contractPreviewingRef.current = false
      setContractPreviewing(false)
    }
  }, [t])

  useEffect(() => {
    void fetchWalletAccount()
  }, [fetchWalletAccount])

  return {
    account,
    withdrawalEligibility,
    withdrawalEligibilityLoadFailed,
    withdrawalProfile,
    minWithdrawAmount,
    loading,
    transferring,
    withdrawing,
    profileSaving,
    signing,
    contractPreviewing,
    withdrawTaxTrial,
    withdrawTaxTrialLoading,
    withdrawTaxTrialError,
    piggySignUrl,
    refetch: fetchWalletAccount,
    transferCommission,
    submitWithdraw,
    trialWithdrawTax,
    clearWithdrawTaxTrial,
    saveWithdrawalProfile,
    requestPiggySign,
    refreshPiggyContractStatus,
    openPiggyContractPreview,
  }
}
