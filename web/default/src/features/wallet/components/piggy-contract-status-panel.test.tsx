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
import { renderToStaticMarkup } from 'react-dom/server'
import type { WithdrawalEligibility, WithdrawalProfile } from '../types'
import { PiggyContractStatusPanel } from './piggy-contract-status-panel'
import { resolvePiggyContractRefreshAction } from './piggy-contract-status-panel-logic'

const signedProfile: WithdrawalProfile = {
  id: 1,
  user_id: 1001,
  account_type: 'bankcard',
  real_name: 'Alice Chen',
  bank_name: 'Test Bank',
  masked_id_card_no: '110************1234',
  masked_mobile: '138****5678',
  masked_bank_card_no: '6222 **** **** 1234',
  piggy_sign_status: 'signed',
  piggy_signed_at: 1710000000,
  piggy_contract_document_id: 'DOC-2102',
  created_at: 1710000000,
  updated_at: 1710000000,
}

function eligibility(
  override: Partial<WithdrawalEligibility> = {}
): WithdrawalEligibility {
  return {
    enabled: true,
    can_withdraw: false,
    need_profile: false,
    need_sign: false,
    profile: signedProfile,
    withdrawable_commission: 0,
    frozen_commission: 0,
    commission_min_withdraw_amount: 0,
    cooldown_remaining_seconds: 3600,
    disabled_reason: 'Insufficient commission',
    blocking_reasons: ['Insufficient commission'],
    ...override,
  }
}

const noop = async () => true

describe('PiggyContractStatusPanel', () => {
  test('routes refresh actions by contract profile state', async () => {
    const calls: string[] = []
    const refresh = async () => {
      calls.push('refresh')
      return true
    }
    const refreshContractStatus = async () => {
      calls.push('contract-status')
      return true
    }

    await resolvePiggyContractRefreshAction(
      'profile',
      refresh,
      refreshContractStatus
    )?.()
    await resolvePiggyContractRefreshAction(
      'sign',
      refresh,
      refreshContractStatus
    )?.()
    await resolvePiggyContractRefreshAction(
      'signed',
      refresh,
      refreshContractStatus
    )?.()

    expect(calls).toEqual(['refresh', 'contract-status', 'refresh'])
  })

  test('renders profile completion without relying on the withdrawal dialog', () => {
    const html = renderToStaticMarkup(
      <PiggyContractStatusPanel
        profile={null}
        eligibility={eligibility({
          need_profile: true,
          need_sign: true,
          profile: null,
        })}
        onSaveProfile={noop}
        onRequestSign={noop}
        onRefresh={noop}
        onRefreshContractStatus={noop}
      />
    )

    expect(html).toContain('Withdrawal profile')
    expect(html).toContain('Complete bank card profile before signing.')
    expect(html).toContain('Save withdrawal profile')
    expect(html.includes('Withdraw Commission')).toBe(false)
  })

  test('does not expose editable profile fields while eligibility is loading', () => {
    const html = renderToStaticMarkup(
      <PiggyContractStatusPanel
        loading
        profile={null}
        eligibility={null}
        onSaveProfile={noop}
        onRequestSign={noop}
        onRefresh={noop}
        onRefreshContractStatus={noop}
      />
    )

    expect(html).toContain('Loading withdrawal profile')
    expect(html.includes('Save withdrawal profile')).toBe(false)
    expect(html.includes('Complete bank card profile before signing.')).toBe(false)
  })

  test('does not expose editable profile fields when eligibility fails to load', () => {
    const html = renderToStaticMarkup(
      <PiggyContractStatusPanel
        eligibilityLoadFailed
        profile={null}
        eligibility={null}
        onSaveProfile={noop}
        onRequestSign={noop}
        onRefresh={noop}
        onRefreshContractStatus={noop}
      />
    )

    expect(html).toContain('Failed to load withdrawal profile')
    expect(html).toContain('Refresh')
    expect(html.includes('Save withdrawal profile')).toBe(false)
    expect(html.includes('Complete bank card profile before signing.')).toBe(false)
  })

  test('discards dirty profile draft back to the current saved profile', async () => {
    const logic = (await import('./piggy-contract-status-panel-logic')) as {
      discardPiggyContractProfileDraft?: (
        draft: {
          account_type: 'bankcard'
          real_name: string
          id_card_no: string
          mobile: string
          bank_card_no: string
          bank_name: string
        },
        profile?: WithdrawalProfile | null
      ) => {
        account_type: 'bankcard'
        real_name: string
        id_card_no: string
        mobile: string
        bank_card_no: string
        bank_name: string
      }
    }
    const dirtyDraft = {
      account_type: 'bankcard' as const,
      real_name: 'Changed Name',
      id_card_no: 'changed-id',
      mobile: '13900000000',
      bank_card_no: 'changed-card',
      bank_name: 'Changed Bank',
    }

    expect(typeof logic.discardPiggyContractProfileDraft).toBe('function')
    expect(
      logic.discardPiggyContractProfileDraft?.(dirtyDraft, signedProfile)
    ).toEqual({
      account_type: 'bankcard',
      real_name: 'Alice Chen',
      id_card_no: '',
      mobile: '',
      bank_card_no: '',
      bank_name: 'Test Bank',
    })
  })

  test('shows signing QR action without refresh before signing has started', () => {
    const unsignedProfile = {
      ...signedProfile,
      piggy_sign_status: 'unsigned',
      piggy_contract_document_id: '',
    }
    const html = renderToStaticMarkup(
      <PiggyContractStatusPanel
        profile={unsignedProfile}
        eligibility={eligibility({
          need_profile: false,
          need_sign: true,
          profile: unsignedProfile,
        })}
        onSaveProfile={noop}
        onRequestSign={noop}
        onRefresh={noop}
        onRefreshContractStatus={noop}
      />
    )

    expect(html).toContain('Sign Piggy electronic contract')
    expect(html).toContain('手机扫描签约二维码')
    expect(html.includes('Refresh contract status')).toBe(false)
    expect(html.includes('Amount')).toBe(false)
  })

  test('keeps contract status refresh after signing URL is generated', () => {
    const unsignedProfile = {
      ...signedProfile,
      piggy_sign_status: 'unsigned',
      piggy_contract_document_id: '',
    }
    const html = renderToStaticMarkup(
      <PiggyContractStatusPanel
        profile={unsignedProfile}
        eligibility={eligibility({
          need_profile: false,
          need_sign: true,
          profile: unsignedProfile,
        })}
        piggySignUrl='https://test.xzsz.ltd/sign?ticket=abc'
        onSaveProfile={noop}
        onRequestSign={noop}
        onRefresh={noop}
        onRefreshContractStatus={noop}
      />
    )

    expect(html).toContain('手机扫描签约二维码')
    expect(html).toContain('Refresh contract status')
    expect(html).toContain('<svg')
  })

  test('keeps contract status refresh after a signing URL request was stored', () => {
    const unsignedProfile = {
      ...signedProfile,
      piggy_sign_status: 'unsigned',
      piggy_contract_document_id: '',
      piggy_sign_url_digest: 'digest-value',
    }
    const html = renderToStaticMarkup(
      <PiggyContractStatusPanel
        profile={unsignedProfile}
        eligibility={eligibility({
          need_profile: false,
          need_sign: true,
          profile: unsignedProfile,
        })}
        onSaveProfile={noop}
        onRequestSign={noop}
        onRefresh={noop}
        onRefreshContractStatus={noop}
      />
    )

    expect(html).toContain('手机扫描签约二维码')
    expect(html).toContain('Refresh contract status')
  })

  test('hides signed contract preview action when withdrawal is blocked', () => {
    const html = renderToStaticMarkup(
      <PiggyContractStatusPanel
        profile={signedProfile}
        eligibility={eligibility()}
        contractPreviewing={false}
        onOpenContract={noop}
        onSaveProfile={noop}
        onRequestSign={noop}
        onRefresh={noop}
        onRefreshContractStatus={noop}
      />
    )

    expect(html).toContain('DOC-2102')
    expect(html.includes('Open contract')).toBe(false)
    expect(html).toContain('Insufficient commission')
    expect(html.includes('/contract/sign/viewContract')).toBe(false)
    expect(html.includes('Refresh contract status')).toBe(false)
  })
})
