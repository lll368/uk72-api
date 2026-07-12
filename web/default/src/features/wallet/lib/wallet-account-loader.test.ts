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
import { loadWalletAccountSequentially } from './wallet-account-loader'
import type {
  ApiResponse,
  WalletAccountPayload,
  WithdrawalEligibility,
} from '../types'

describe('loadWalletAccountSequentially', () => {
  test('loads account before withdrawal eligibility', async () => {
    const calls: string[] = []
    const accountResponse: ApiResponse<WalletAccountPayload> = {
      success: true,
      data: {
        account: {
          id: 1,
          user_id: 10,
          balance_amount: 0,
          commission_amount: 0,
          frozen_commission_amount: 0,
          total_commission_amount: 0,
          total_withdraw_amount: 0,
          created_at: 0,
          updated_at: 0,
        },
        commission_min_withdraw_amount: 0,
      },
    }
    const eligibilityResponse: ApiResponse<WithdrawalEligibility> = {
      success: true,
      data: {
        enabled: true,
        can_withdraw: false,
        need_profile: false,
        need_sign: false,
        profile: null,
        withdrawable_commission: 0,
        frozen_commission: 0,
        commission_min_withdraw_amount: 0,
        cooldown_remaining_seconds: 0,
        disabled_reason: '',
        blocking_reasons: [],
      },
    }

    const result = await loadWalletAccountSequentially({
      getWalletAccount: async () => {
        calls.push('account:start')
        await Promise.resolve()
        calls.push('account:end')
        return accountResponse
      },
      getWalletWithdrawalEligibility: async () => {
        calls.push('eligibility:start')
        return eligibilityResponse
      },
    })

    expect(calls).toEqual([
      'account:start',
      'account:end',
      'eligibility:start',
    ])
    expect(result.accountResponse).toBe(accountResponse)
    expect(result.eligibilityResponse).toBe(eligibilityResponse)
  })
})
