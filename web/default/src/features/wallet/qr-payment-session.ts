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
import type { QrPaymentResult } from './types'

export interface QrPaymentSession {
  sessionId: number
  open: boolean
  tradeNo: string
}

export interface QrPaymentSessionRequest {
  sessionId: number
  tradeNo: string
}

interface QrPaymentHandledTradeNoRef {
  current: string | null
}

type QrPaymentSuccessResult = boolean | void

interface HandleQrPaymentSuccessOptions {
  tradeNo: string
  handledTradeNoRef: QrPaymentHandledTradeNoRef
  onSuccess?: () => Promise<QrPaymentSuccessResult> | QrPaymentSuccessResult
  notifySuccess?: () => void
}

export const initialQrPaymentSession: QrPaymentSession = {
  sessionId: 0,
  open: false,
  tradeNo: '',
}

export function getQrPaymentTradeNo(
  payment: Pick<QrPaymentResult, 'trade_no' | 'order_id'> | null
) {
  return (
    getQrPaymentPrimaryTradeNo(payment) || (payment?.order_id || '').trim()
  )
}

export function getQrPaymentPrimaryTradeNo(
  payment: Pick<QrPaymentResult, 'trade_no'> | null
) {
  return (payment?.trade_no || '').trim()
}

export function createQrPaymentSession(
  previousSessionId: number,
  payment: Pick<QrPaymentResult, 'trade_no' | 'order_id'> | null
): QrPaymentSession {
  return {
    sessionId: previousSessionId + 1,
    open: true,
    tradeNo: getQrPaymentTradeNo(payment),
  }
}

export function closeQrPaymentSession(
  session: QrPaymentSession
): QrPaymentSession {
  return {
    sessionId: session.sessionId + 1,
    open: false,
    tradeNo: '',
  }
}

export function isCurrentQrPaymentSession(
  session: QrPaymentSession,
  request: QrPaymentSessionRequest
) {
  return (
    session.open &&
    request.tradeNo !== '' &&
    session.sessionId === request.sessionId &&
    session.tradeNo === request.tradeNo
  )
}

export async function handleQrPaymentSuccessOnce({
  tradeNo,
  handledTradeNoRef,
  onSuccess,
  notifySuccess,
}: HandleQrPaymentSuccessOptions) {
  if (!tradeNo) {
    return false
  }
  if (handledTradeNoRef.current === tradeNo) {
    return true
  }

  const successResult = await onSuccess?.()
  if (successResult === false) {
    return false
  }

  handledTradeNoRef.current = tradeNo
  notifySuccess?.()
  return true
}
