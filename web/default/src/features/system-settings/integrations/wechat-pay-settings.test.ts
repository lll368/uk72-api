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
  buildWechatPaySettingsUpdates,
  type WechatPaySettingsValues,
} from './wechat-pay-settings'

const baseSettings: WechatPaySettingsValues = {
  WechatPayEnabled: false,
  WechatPaySandbox: false,
  WechatPayAppId: '',
  WechatPayMchId: '',
  WechatPayMerchantSerialNo: '',
  WechatPayMerchantPrivateKey: '',
  WechatPayAPIv3Key: '',
  WechatPayPlatformSerialNo: '',
  WechatPayPlatformPublicKey: '',
  WechatPayUnitPrice: 7.3,
  WechatPayMinTopUp: 1,
  WechatPayNotifyUrl: '',
}

describe('buildWechatPaySettingsUpdates', () => {
  test('builds direct WeChat Pay payload without epay option keys', () => {
    const updates = buildWechatPaySettingsUpdates(baseSettings, {
      ...baseSettings,
      WechatPayEnabled: true,
      WechatPaySandbox: true,
      WechatPayAppId: ' wx-app-id ',
      WechatPayMchId: ' 1900000001 ',
      WechatPayMerchantSerialNo: ' merchant-serial ',
      WechatPayMerchantPrivateKey: ' merchant-private-key ',
      WechatPayAPIv3Key: '12345678901234567890123456789012',
      WechatPayPlatformSerialNo: ' platform-serial ',
      WechatPayPlatformPublicKey: ' platform-public-key ',
      WechatPayUnitPrice: 7.5,
      WechatPayMinTopUp: 5,
      WechatPayNotifyUrl: 'https://api.example.com/api/wechat/notify/',
    })

    expect(updates).toEqual([
      { key: 'WechatPayAppId', value: 'wx-app-id' },
      { key: 'WechatPayMchId', value: '1900000001' },
      { key: 'WechatPayMerchantSerialNo', value: 'merchant-serial' },
      { key: 'WechatPayMerchantPrivateKey', value: 'merchant-private-key' },
      { key: 'WechatPayAPIv3Key', value: '12345678901234567890123456789012' },
      { key: 'WechatPayPlatformSerialNo', value: 'platform-serial' },
      { key: 'WechatPayPlatformPublicKey', value: 'platform-public-key' },
      { key: 'WechatPayUnitPrice', value: 7.5 },
      { key: 'WechatPayMinTopUp', value: 5 },
      {
        key: 'WechatPayNotifyUrl',
        value: 'https://api.example.com/api/wechat/notify',
      },
      { key: 'WechatPayEnabled', value: true },
    ])
    expect(updates.some((update) => update.key === 'PayMethods')).toBe(false)
    expect(updates.some((update) => update.key === 'WechatPaySandbox')).toBe(
      false
    )
    expect(updates.at(-1)).toEqual({ key: 'WechatPayEnabled', value: true })
  })

  test('does not send blank key rotations or epay settings', () => {
    const updates = buildWechatPaySettingsUpdates(
      {
        ...baseSettings,
        WechatPayEnabled: true,
        WechatPayMerchantPrivateKey: 'stored-private-key',
        WechatPayAPIv3Key: '12345678901234567890123456789012',
        WechatPayPlatformPublicKey: 'stored-platform-key',
      },
      {
        ...baseSettings,
        WechatPayEnabled: true,
        WechatPayMerchantPrivateKey: '',
        WechatPayAPIv3Key: '',
        WechatPayPlatformPublicKey: '',
      }
    )

    expect(updates).toEqual([])
  })
})
