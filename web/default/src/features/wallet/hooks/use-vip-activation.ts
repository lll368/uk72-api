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
import i18next from 'i18next'
import { toast } from 'sonner'
import {
  getVipActivationInfo,
  isApiSuccess,
  requestVipActivationAlipayPayment,
  requestVipActivationCreemPayment,
  requestVipActivationEpayPayment,
  requestVipActivationStripePayment,
  requestVipActivationWechatPayPayment,
  requestVipActivationWaffoPayment,
} from '../api'
import {
  isCreemPayment,
  isAlipayDirectPayment,
  isWechatDirectPayment,
  isStripePayment,
  isWaffoPayment,
  filterVisiblePaymentMethods,
} from '../lib'
import { submitPaymentForm } from '../lib/payment'
import type {
  PaymentMethod,
  QrPaymentResult,
  VipActivationInfo,
} from '../types'

function getVipPaymentError(
  message: string | undefined,
  data: unknown
): string {
  if (typeof data === 'string' && data.trim()) {
    return data
  }
  return message || i18next.t('Payment request failed')
}

export function useVipActivation() {
  const [vipInfo, setVipInfo] = useState<VipActivationInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [processing, setProcessing] = useState<string | null>(null)

  const refreshVipInfo = useCallback(async (): Promise<boolean> => {
    try {
      setLoading(true)
      const response = await getVipActivationInfo()
      if (isApiSuccess(response) && response.data) {
        setVipInfo({
          ...response.data,
          payment_methods: filterVisiblePaymentMethods(
            response.data.payment_methods ?? []
          ),
        })
        return true
      }
      return false
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refreshVipInfo()
  }, [refreshVipInfo])

  const processVipActivation = useCallback(
    async (method: PaymentMethod) => {
      if (!method?.type) return false
      if (vipInfo?.is_vvip) {
        toast.error(i18next.t('You have already activated VVIP'))
        return false
      }

      setProcessing(method.type)
      try {
        if (isStripePayment(method.type)) {
          const response = await requestVipActivationStripePayment()
          if (isApiSuccess(response) && response.data?.pay_link) {
            window.open(response.data.pay_link, '_blank')
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
          toast.error(getVipPaymentError(response.message, response.data))
          return false
        }

        if (isCreemPayment(method.type)) {
          const productId = vipInfo?.creem_products?.[0]?.productId
          const response = await requestVipActivationCreemPayment({
            product_id: productId,
          })
          if (isApiSuccess(response) && response.data?.checkout_url) {
            window.open(response.data.checkout_url, '_blank')
            toast.success(i18next.t('Redirecting to Creem checkout...'))
            return true
          }
          toast.error(getVipPaymentError(response.message, response.data))
          return false
        }

        if (isWaffoPayment(method.type)) {
          const response = await requestVipActivationWaffoPayment({})
          if (isApiSuccess(response) && response.data?.payment_url) {
            window.open(response.data.payment_url, '_blank')
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
          toast.error(getVipPaymentError(response.message, response.data))
          return false
        }

        if (isAlipayDirectPayment(method.type)) {
          const response = await requestVipActivationAlipayPayment()
          if (isApiSuccess(response) && response.data?.url) {
            submitPaymentForm(response.data.url, response.data.params ?? {}, {
              target: '_self',
            })
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
          toast.error(getVipPaymentError(response.message, response.data))
          return false
        }

        if (isWechatDirectPayment(method.type)) {
          const response = await requestVipActivationWechatPayPayment({
            payment_method: 'wechat_direct',
          })
          if (isApiSuccess(response) && response.data?.code_url) {
            toast.success(i18next.t('WeChat Pay order created'))
            return {
              type: 'qr',
              payment_method: 'wechat_direct',
              purpose: 'vvip_activation',
              code_url: response.data.code_url,
              trade_no: response.data.trade_no,
              order_id: response.data.order_id,
              expires_at: response.data.expires_at,
            } satisfies QrPaymentResult
          }
          toast.error(getVipPaymentError(response.message, response.data))
          return false
        }

        const response = await requestVipActivationEpayPayment({
          payment_method: method.type,
        })
        if (isApiSuccess(response) && response.data) {
          const url = (response as unknown as { url?: string }).url
          if (url) {
            submitPaymentForm(url, response.data)
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
        }

        toast.error(getVipPaymentError(response.message, response.data))
        return false
      } catch (_error) {
        toast.error(i18next.t('Payment request failed'))
        return false
      } finally {
        setProcessing(null)
      }
    },
    [refreshVipInfo, vipInfo?.creem_products, vipInfo?.is_vvip]
  )

  return {
    vipInfo,
    loading,
    processing,
    refreshVipInfo,
    processVipActivation,
  }
}
