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
  buildAlipaySettingsUpdates,
  type AlipaySettingsValues,
} from './alipay-settings'

const baseSettings: AlipaySettingsValues = {
  AlipayEnabled: false,
  AlipaySandbox: false,
  AlipayAppId: '',
  AlipayPrivateKey: '',
  AlipayPublicKey: '',
  AlipayUnitPrice: 7.3,
  AlipayMinTopUp: 1,
  AlipayReturnUrl: '',
  AlipayNotifyUrl: '',
}

describe('buildAlipaySettingsUpdates', () => {
  test('builds direct Alipay payload without epay option keys', () => {
    const updates = buildAlipaySettingsUpdates(baseSettings, {
      ...baseSettings,
      AlipayEnabled: true,
      AlipaySandbox: true,
      AlipayAppId: ' 2021000000000000 ',
      AlipayPrivateKey: ' merchant-private-key ',
      AlipayPublicKey: ' alipay-public-key ',
      AlipayUnitPrice: 7.5,
      AlipayMinTopUp: 5,
      AlipayReturnUrl: 'https://app.example.com/wallet/',
      AlipayNotifyUrl: 'https://api.example.com/api/alipay/notify/',
    })

    expect(updates).toEqual([
      { key: 'AlipayEnabled', value: true },
      { key: 'AlipaySandbox', value: true },
      { key: 'AlipayAppId', value: '2021000000000000' },
      { key: 'AlipayPrivateKey', value: 'merchant-private-key' },
      { key: 'AlipayPublicKey', value: 'alipay-public-key' },
      { key: 'AlipayUnitPrice', value: 7.5 },
      { key: 'AlipayMinTopUp', value: 5 },
      { key: 'AlipayReturnUrl', value: 'https://app.example.com/wallet' },
      {
        key: 'AlipayNotifyUrl',
        value: 'https://api.example.com/api/alipay/notify',
      },
    ])
  })

  test('does not send blank key rotations or epay settings', () => {
    const updates = buildAlipaySettingsUpdates(
      {
        ...baseSettings,
        AlipayEnabled: true,
        AlipayAppId: '2021000000000000',
        AlipayPrivateKey: 'stored-private-key',
        AlipayPublicKey: 'stored-public-key',
      },
      {
        ...baseSettings,
        AlipayEnabled: true,
        AlipayAppId: '2021000000000000',
        AlipayPrivateKey: '',
        AlipayPublicKey: '',
      }
    )

    expect(updates).toEqual([])
  })
})
