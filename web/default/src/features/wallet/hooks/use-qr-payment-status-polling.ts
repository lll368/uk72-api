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
  closeQrPaymentSession,
  getQrPaymentTradeNo,
  handleQrPaymentSuccessOnce,
  initialQrPaymentSession,
  isCurrentQrPaymentSession,
} from '../qr-payment-session'
import type { QrPaymentResult, QrPaymentStatus } from '../types'

interface UseQrPaymentStatusPollingOptions {
  open: boolean
  payment: QrPaymentResult | null
  enabled?: boolean
  intervalMs?: number
  successMessage?: string
  pollStatus: (
    payment: QrPaymentResult,
    tradeNo: string
  ) => Promise<QrPaymentStatus | null>
  onSuccess?: (
    payment: QrPaymentResult,
    tradeNo: string
  ) => Promise<boolean | void> | boolean | void
  onFallbackRefresh?: () => Promise<boolean | void> | boolean | void
  getPollingTradeNo?: (payment: QrPaymentResult) => string
}

type QrPaymentSuccessHandler = () => Promise<boolean> | boolean

export async function resolveQrPaymentPolledStatus(
  nextStatus: QrPaymentStatus,
  handleSuccess: QrPaymentSuccessHandler
): Promise<QrPaymentStatus> {
  if (nextStatus !== 'success') {
    return nextStatus
  }

  const handled = await handleSuccess()
  return handled ? 'success' : 'pending'
}

export function useQrPaymentStatusPolling({
  open,
  payment,
  enabled = true,
  intervalMs = 3000,
  successMessage,
  pollStatus,
  onSuccess,
  onFallbackRefresh,
  getPollingTradeNo = getQrPaymentTradeNo,
}: UseQrPaymentStatusPollingOptions) {
  const { t } = useTranslation()
  const [status, setStatus] = useState<QrPaymentStatus>('pending')
  const [refreshing, setRefreshing] = useState(false)
  const pollingInFlightRef = useRef(false)
  const pollingRequestIdRef = useRef(0)
  const handledTradeNoRef = useRef<string | null>(null)
  const sessionRef = useRef(initialQrPaymentSession)

  const activatePayment = useCallback((nextPayment: QrPaymentResult) => {
    sessionRef.current = {
      sessionId: sessionRef.current.sessionId + 1,
      open: true,
      tradeNo: getPollingTradeNo(nextPayment),
    }
    pollingRequestIdRef.current += 1
    pollingInFlightRef.current = false
    handledTradeNoRef.current = null
    setStatus('pending')
    setRefreshing(false)
  }, [getPollingTradeNo])

  const closeSession = useCallback(() => {
    sessionRef.current = closeQrPaymentSession(sessionRef.current)
    pollingRequestIdRef.current += 1
    handledTradeNoRef.current = null
    pollingInFlightRef.current = false
    setRefreshing(false)
    setStatus('pending')
  }, [])

  useEffect(() => {
    if (!open) {
      if (sessionRef.current.open) {
        closeSession()
      }
      return
    }
    if (!enabled || !payment) {
      return
    }
    const tradeNo = getPollingTradeNo(payment)
    if (!tradeNo) {
      return
    }
    if (
      !sessionRef.current.open ||
      sessionRef.current.tradeNo !== tradeNo
    ) {
      activatePayment(payment)
    }
  }, [activatePayment, closeSession, enabled, getPollingTradeNo, open, payment])

  const refreshStatus = useCallback(async (): Promise<
    QrPaymentStatus | null | undefined
  > => {
    if (!enabled || !payment) {
      await onFallbackRefresh?.()
      return undefined
    }

    const tradeNo = getPollingTradeNo(payment)
    if (!tradeNo) {
      await onFallbackRefresh?.()
      return undefined
    }

    if (pollingInFlightRef.current) {
      return undefined
    }

    const requestSession = {
      sessionId: sessionRef.current.sessionId,
      tradeNo,
    }
    if (!isCurrentQrPaymentSession(sessionRef.current, requestSession)) {
      return undefined
    }

    const requestId = pollingRequestIdRef.current + 1
    pollingRequestIdRef.current = requestId
    pollingInFlightRef.current = true
    setRefreshing(true)
    try {
      const nextStatus = await pollStatus(payment, tradeNo)
      if (!isCurrentQrPaymentSession(sessionRef.current, requestSession)) {
        return undefined
      }
      if (!nextStatus) {
        return null
      }

      const resolvedStatus = await resolveQrPaymentPolledStatus(
        nextStatus,
        () =>
          handleQrPaymentSuccessOnce({
            tradeNo,
            handledTradeNoRef,
            onSuccess: () => onSuccess?.(payment, tradeNo),
            notifySuccess: () => {
              if (successMessage) {
                toast.success(t(successMessage))
              }
            },
          })
      )
      setStatus(resolvedStatus)
      return resolvedStatus
    } catch (error) {
      if (!isCurrentQrPaymentSession(sessionRef.current, requestSession)) {
        return undefined
      }
      // eslint-disable-next-line no-console
      console.error('Failed to refresh QR payment order status:', error)
      return null
    } finally {
      if (pollingRequestIdRef.current === requestId) {
        pollingInFlightRef.current = false
        if (isCurrentQrPaymentSession(sessionRef.current, requestSession)) {
          setRefreshing(false)
        }
      }
    }
  }, [
    enabled,
    getPollingTradeNo,
    onFallbackRefresh,
    onSuccess,
    payment,
    pollStatus,
    successMessage,
    t,
  ])

  const refresh = useCallback(async () => {
    const nextStatus = await refreshStatus()
    if (nextStatus === null) {
      await onFallbackRefresh?.()
    }
    return nextStatus
  }, [onFallbackRefresh, refreshStatus])

  useEffect(() => {
    if (
      !open ||
      !enabled ||
      !payment ||
      status !== 'pending' ||
      !getPollingTradeNo(payment)
    ) {
      return
    }

    void refreshStatus()
    const intervalId = window.setInterval(() => {
      void refreshStatus()
    }, intervalMs)

    return () => {
      window.clearInterval(intervalId)
    }
  }, [
    enabled,
    getPollingTradeNo,
    intervalMs,
    open,
    payment,
    refreshStatus,
    status,
  ])

  return {
    status,
    refreshing,
    refresh,
    activatePayment,
    closeSession,
  }
}
