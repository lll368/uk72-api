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
import { useState, useCallback } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import {
  calculateAlipayAmount,
  calculateAmount,
  calculateStripeAmount,
  calculateWechatPayAmount,
  calculateWaffoPancakeAmount,
  requestAlipayPayment,
  requestPayment,
  requestStripePayment,
  requestWechatPayPayment,
  isApiSuccess,
} from '../api'
import { getWalletPaymentDispatch, submitPaymentForm } from '../lib'
import type { QrPaymentResult } from '../types'

// ============================================================================
// Payment Hook
// ============================================================================

export function usePayment() {
  const [amount, setAmount] = useState<number>(0)
  const [calculating, setCalculating] = useState(false)
  const [processing, setProcessing] = useState(false)

  // Calculate payment amount
  const calculatePaymentAmount = useCallback(
    async (topupAmount: number, paymentType: string) => {
      try {
        setCalculating(true)

        const dispatch = getWalletPaymentDispatch(paymentType)
        const response =
          dispatch === 'stripe'
            ? await calculateStripeAmount({ amount: topupAmount })
            : dispatch === 'alipay_direct'
              ? await calculateAlipayAmount({ amount: topupAmount })
              : dispatch === 'wechat_direct'
                ? await calculateWechatPayAmount({ amount: topupAmount })
                : dispatch === 'waffo_pancake'
                  ? await calculateWaffoPancakeAmount({ amount: topupAmount })
                  : await calculateAmount({ amount: topupAmount })

        if (isApiSuccess(response) && response.data) {
          const calculatedAmount = parseFloat(response.data)
          setAmount(calculatedAmount)
          return calculatedAmount
        }

        // Don't show error for calculation, just set to 0
        setAmount(0)
        return 0
      } catch (_error) {
        setAmount(0)
        return 0
      } finally {
        setCalculating(false)
      }
    },
    []
  )

  // Process payment
  const processPayment = useCallback(
    async (topupAmount: number, paymentType: string) => {
      try {
        setProcessing(true)

        const dispatch = getWalletPaymentDispatch(paymentType)
        const amount = Math.floor(topupAmount)

        if (dispatch === 'stripe') {
          const response = await requestStripePayment({
            amount,
            payment_method: 'stripe',
          })
          if (!isApiSuccess(response)) {
            toast.error(response.message || i18next.t('Payment request failed'))
            return false
          }
          if (response.data?.pay_link) {
            window.open(response.data.pay_link, '_blank')
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
          return false
        }

        if (dispatch === 'alipay_direct') {
          const response = await requestAlipayPayment({
            amount,
            payment_method: paymentType,
          })
          if (!isApiSuccess(response)) {
            toast.error(response.message || i18next.t('Payment request failed'))
            return false
          }
          if (response.data?.url) {
            submitPaymentForm(response.data.url, response.data.params ?? {}, {
              target: '_self',
            })
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
          return false
        }

        if (dispatch === 'wechat_direct') {
          const response = await requestWechatPayPayment({
            amount,
            payment_method: paymentType,
          })
          if (!isApiSuccess(response)) {
            toast.error(response.message || i18next.t('Payment request failed'))
            return false
          }
          if (response.data?.code_url) {
            toast.success(i18next.t('WeChat Pay order created'))
            return {
              type: 'qr',
              payment_method: 'wechat_direct',
              purpose: 'topup',
              amount,
              code_url: response.data.code_url,
              trade_no: response.data.trade_no,
              order_id: response.data.order_id,
              expires_at: response.data.expires_at,
            } satisfies QrPaymentResult
          }
          return false
        }

        const response = await requestPayment({
          amount,
          payment_method: paymentType,
        })

        if (!isApiSuccess(response)) {
          toast.error(response.message || i18next.t('Payment request failed'))
          return false
        }

        if (response.data) {
          const url = (response as unknown as { url?: string }).url
          if (url) {
            submitPaymentForm(url, response.data)
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
        }

        return false
      } catch (_error) {
        toast.error(i18next.t('Payment request failed'))
        return false
      } finally {
        setProcessing(false)
      }
    },
    []
  )

  return {
    amount,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
    setAmount,
  }
}
