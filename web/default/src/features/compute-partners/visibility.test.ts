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
import * as bunTest from 'bun:test'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'

const mock = (
  bunTest as unknown as {
    mock: { module: (specifier: string, factory: () => unknown) => void }
  }
).mock

mock.module('@/stores/auth-store', () => ({
  useAuthStore: () => () => undefined,
}))

mock.module('@/features/wallet/api', () => ({
  getVipActivationQrPaymentStatusByTradeNo: async () => ({
    status: 'pending',
  }),
}))

mock.module('@/features/wallet/hooks', () => ({
  useAffiliate: () => ({
    affiliateLink: 'https://example.test/invite/abc',
    loading: false,
    refetch: async () => true,
  }),
  useTopupInfo: () => ({
    topupInfo: {
      payment_compliance_confirmed: true,
    },
  }),
  useQrPaymentSuccessClose: () => ({
    successClosing: false,
    closeAfterSuccess: async () => undefined,
    handleOpenChange: () => undefined,
  }),
  useQrPaymentStatusPolling: () => ({
    status: 'idle',
    refreshing: false,
    activatePayment: () => undefined,
    closeSession: () => undefined,
    refresh: async () => undefined,
  }),
  useVipActivation: () => ({
    vipInfo: {
      is_vvip: true,
      status: 'success',
      activated_at: 1710000000,
      activation_amount: 100,
      paid_amount: 100,
      discount: 1,
      payment_methods: [],
      aff_code: 'ABC123',
      invite_link: 'https://example.test/invite/abc',
    },
    loading: false,
    processing: false,
    refreshVipInfo: async () => true,
    processVipActivation: async () => true,
  }),
  useWalletAccount: () => ({
    account: {
      commission_amount: 100,
      total_commission_amount: 300,
    },
    loading: false,
    transferring: false,
    refetch: async () => true,
    transferCommission: async () => true,
  }),
}))

describe('compute partners visibility policy', () => {
  test('compute partners page does not render withdrawal profile or withdrawal entry', async () => {
    const { ComputePartners } = await import('./index')
    const html = renderToStaticMarkup(createElement(ComputePartners))

    expect(html).toContain('Compute Partners')
    expect(html.includes('Withdrawal profile')).toBe(false)
    expect(html.includes('Real name')).toBe(false)
    expect(html.includes('Mobile')).toBe(false)
    expect(html.includes('ID card number')).toBe(false)
    expect(html.includes('Bank card number')).toBe(false)
    expect(html.includes('Bank name')).toBe(false)
    expect(html.includes('Withdraw')).toBe(false)
  })

  test('referral rewards card keeps transfer action while withdrawal action is omitted', async () => {
    const { AffiliateRewardsCard } = await import(
      '@/features/wallet/components/affiliate-rewards-card'
    )
    const html = renderToStaticMarkup(
      createElement(AffiliateRewardsCard, {
        user: {
          id: 1,
          username: 'alice',
          quota: 0,
          used_quota: 0,
          request_count: 0,
          aff_quota: 0,
          aff_history_quota: 0,
          aff_count: 3,
          group: 'default',
        },
        affiliateLink: 'https://example.test/invite/abc',
        commissionAmount: 100,
        totalCommissionAmount: 300,
        onTransfer: () => undefined,
        complianceConfirmed: true,
        loading: false,
      })
    )

    expect(html).toContain('Referral Program')
    expect(html).toContain('Transfer to Balance')
    expect(html.includes('Withdraw')).toBe(false)
  })
})
