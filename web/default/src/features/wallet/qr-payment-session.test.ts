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
import { describe, expect, test } from 'bun:test'
import {
  closeQrPaymentSession,
  createQrPaymentSession,
  getQrPaymentPrimaryTradeNo,
  getQrPaymentTradeNo,
  handleQrPaymentSuccessOnce,
  isCurrentQrPaymentSession,
} from './qr-payment-session'
import type { QrPaymentResult } from './types'

function payment(value: Partial<QrPaymentResult>): QrPaymentResult {
  return {
    type: 'qr',
    payment_method: 'wechat_direct',
    purpose: 'topup',
    code_url: 'weixin://wxpay/bizpayurl?pr=test',
    trade_no: 'ORDER-1',
    ...value,
  }
}

describe('qr payment session guard', () => {
  test('keeps an in-flight status response from mutating after dialog close', () => {
    const activeSession = createQrPaymentSession(0, payment({}))
    const pendingRequest = {
      sessionId: activeSession.sessionId,
      tradeNo: activeSession.tradeNo,
    }

    const closedSession = closeQrPaymentSession(activeSession)

    expect(isCurrentQrPaymentSession(closedSession, pendingRequest)).toBe(false)
  })

  test('rejects stale responses after a new QR payment session starts', () => {
    const firstSession = createQrPaymentSession(
      0,
      payment({ trade_no: 'ORDER-1' })
    )
    const pendingRequest = {
      sessionId: firstSession.sessionId,
      tradeNo: firstSession.tradeNo,
    }

    const secondSession = createQrPaymentSession(
      firstSession.sessionId,
      payment({ trade_no: 'ORDER-2' })
    )

    expect(isCurrentQrPaymentSession(secondSession, pendingRequest)).toBe(false)
  })

  test('normalizes trade number from trade_no before falling back to order_id', () => {
    expect(
      getQrPaymentTradeNo(payment({ trade_no: ' ORDER-1 ', order_id: 'ORDER-2' }))
    ).toBe('ORDER-1')
    expect(
      getQrPaymentTradeNo(payment({ trade_no: '', order_id: ' ORDER-2 ' }))
    ).toBe('ORDER-2')
  })

  test('keeps primary trade number strict for top-up status polling', () => {
    expect(
      getQrPaymentPrimaryTradeNo(
        payment({ trade_no: ' ORDER-1 ', order_id: 'ORDER-2' })
      )
    ).toBe('ORDER-1')
    expect(
      getQrPaymentPrimaryTradeNo(payment({ trade_no: '', order_id: 'ORDER-2' }))
    ).toBe('')
  })

  test('does not mark success as handled when the success refresh fails', async () => {
    const handledTradeNoRef = { current: null as string | null }
    let refreshAttempts = 0
    let notifyCount = 0

    let thrownError: unknown = null
    try {
      await handleQrPaymentSuccessOnce({
        tradeNo: 'ORDER-1',
        handledTradeNoRef,
        onSuccess: async () => {
          refreshAttempts += 1
          throw new Error('refresh failed')
        },
        notifySuccess: () => {
          notifyCount += 1
        },
      })
    } catch (error) {
      thrownError = error
    }

    expect(thrownError instanceof Error).toBe(true)
    expect((thrownError as Error).message).toBe('refresh failed')
    expect(handledTradeNoRef.current).toBeNull()

    const handled = await handleQrPaymentSuccessOnce({
      tradeNo: 'ORDER-1',
      handledTradeNoRef,
      onSuccess: async () => {
        refreshAttempts += 1
      },
      notifySuccess: () => {
        notifyCount += 1
      },
    })

    expect(handled).toBe(true)
    expect(handledTradeNoRef.current).toBe('ORDER-1')
    expect(refreshAttempts).toBe(2)
    expect(notifyCount).toBe(1)
  })

  test('does not mark success as handled when the success refresh reports false', async () => {
    const handledTradeNoRef = { current: null as string | null }
    let notifyCount = 0

    const handled = await handleQrPaymentSuccessOnce({
      tradeNo: 'ORDER-1',
      handledTradeNoRef,
      onSuccess: async () => false,
      notifySuccess: () => {
        notifyCount += 1
      },
    })

    expect(handled).toBe(false)
    expect(handledTradeNoRef.current).toBeNull()
    expect(notifyCount).toBe(0)
  })

  test('treats duplicate handled success as already successful', async () => {
    const handledTradeNoRef = { current: 'ORDER-1' }
    let refreshAttempts = 0
    let notifyCount = 0

    const handled = await handleQrPaymentSuccessOnce({
      tradeNo: 'ORDER-1',
      handledTradeNoRef,
      onSuccess: async () => {
        refreshAttempts += 1
      },
      notifySuccess: () => {
        notifyCount += 1
      },
    })

    expect(handled).toBe(true)
    expect(handledTradeNoRef.current).toBe('ORDER-1')
    expect(refreshAttempts).toBe(0)
    expect(notifyCount).toBe(0)
  })
})
