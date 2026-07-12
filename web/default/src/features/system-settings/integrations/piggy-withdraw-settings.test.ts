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
import { buildPiggyWithdrawDefaults } from '../billing/payment-defaults'
import { savePiggyWithdrawSettingOptions } from './piggy-withdraw-settings'

describe('savePiggyWithdrawSettingOptions', () => {
  test('builds Piggy platform fee defaults while preserving explicit zero', () => {
    const defaults = buildPiggyWithdrawDefaults({} as never)
    expect(defaults.PlatformFeeRate).toBe(8)

    const zero = buildPiggyWithdrawDefaults({
      'piggy_withdraw_setting.platform_fee_rate': 0,
    } as never)
    expect(zero.PlatformFeeRate).toBe(0)
  })

  test('saves options with one batch request', async () => {
    const calls: string[][] = []

    await savePiggyWithdrawSettingOptions(
      [
        { key: 'piggy_withdraw_setting.domain', value: 'https://piggy.test' },
        { key: 'piggy_withdraw_setting.enabled', value: 'true' },
        { key: 'piggy_withdraw_setting.callback_lock_ttl', value: '300' },
        { key: 'piggy_withdraw_setting.platform_fee_rate', value: '0' },
      ],
      async (options) => {
        calls.push(options.map((option) => option.key))
        return { success: true, message: '' }
      }
    )

    expect(calls).toEqual([
      [
        'piggy_withdraw_setting.domain',
        'piggy_withdraw_setting.enabled',
        'piggy_withdraw_setting.callback_lock_ttl',
        'piggy_withdraw_setting.platform_fee_rate',
      ],
    ])
  })

  test('throws failed batch response message', async () => {
    let caught: Error | null = null
    try {
      await savePiggyWithdrawSettingOptions(
        [
          { key: 'piggy_withdraw_setting.domain', value: 'https://piggy.test' },
          { key: 'piggy_withdraw_setting.enabled', value: 'true' },
        ],
        async () => ({ success: false, message: '小猪 appSecret 必须配置' })
      )
    } catch (error) {
      caught = error as Error
    }

    expect(caught?.message).toBe('小猪 appSecret 必须配置')
  })
})
