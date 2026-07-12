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
import { removeTrailingSlash } from './utils'

export interface WechatPaySettingsValues {
  WechatPayEnabled: boolean
  WechatPaySandbox: boolean
  WechatPayAppId: string
  WechatPayMchId: string
  WechatPayMerchantSerialNo: string
  WechatPayMerchantPrivateKey: string
  WechatPayAPIv3Key: string
  WechatPayPlatformSerialNo: string
  WechatPayPlatformPublicKey: string
  WechatPayUnitPrice: number
  WechatPayMinTopUp: number
  WechatPayNotifyUrl: string
}

export type WechatPayOptionUpdate = {
  key: string
  value: string | number | boolean
}

export function normalizeWechatPaySettings(
  values: WechatPaySettingsValues
): WechatPaySettingsValues {
  return {
    WechatPayEnabled: !!values.WechatPayEnabled,
    WechatPaySandbox: !!values.WechatPaySandbox,
    WechatPayAppId: values.WechatPayAppId.trim(),
    WechatPayMchId: values.WechatPayMchId.trim(),
    WechatPayMerchantSerialNo: values.WechatPayMerchantSerialNo.trim(),
    WechatPayMerchantPrivateKey: values.WechatPayMerchantPrivateKey.trim(),
    WechatPayAPIv3Key: values.WechatPayAPIv3Key.trim(),
    WechatPayPlatformSerialNo: values.WechatPayPlatformSerialNo.trim(),
    WechatPayPlatformPublicKey: values.WechatPayPlatformPublicKey.trim(),
    WechatPayUnitPrice: Number(values.WechatPayUnitPrice),
    WechatPayMinTopUp: Number(values.WechatPayMinTopUp),
    WechatPayNotifyUrl: removeTrailingSlash(values.WechatPayNotifyUrl),
  }
}

export function buildWechatPaySettingsUpdates(
  initialValues: WechatPaySettingsValues,
  nextValues: WechatPaySettingsValues
): WechatPayOptionUpdate[] {
  const initial = normalizeWechatPaySettings(initialValues)
  const next = normalizeWechatPaySettings(nextValues)
  const updates: WechatPayOptionUpdate[] = []

  if (next.WechatPayAppId !== initial.WechatPayAppId) {
    updates.push({ key: 'WechatPayAppId', value: next.WechatPayAppId })
  }
  if (next.WechatPayMchId !== initial.WechatPayMchId) {
    updates.push({ key: 'WechatPayMchId', value: next.WechatPayMchId })
  }
  if (next.WechatPayMerchantSerialNo !== initial.WechatPayMerchantSerialNo) {
    updates.push({
      key: 'WechatPayMerchantSerialNo',
      value: next.WechatPayMerchantSerialNo,
    })
  }
  if (
    next.WechatPayMerchantPrivateKey &&
    next.WechatPayMerchantPrivateKey !== initial.WechatPayMerchantPrivateKey
  ) {
    updates.push({
      key: 'WechatPayMerchantPrivateKey',
      value: next.WechatPayMerchantPrivateKey,
    })
  }
  if (
    next.WechatPayAPIv3Key &&
    next.WechatPayAPIv3Key !== initial.WechatPayAPIv3Key
  ) {
    updates.push({ key: 'WechatPayAPIv3Key', value: next.WechatPayAPIv3Key })
  }
  if (next.WechatPayPlatformSerialNo !== initial.WechatPayPlatformSerialNo) {
    updates.push({
      key: 'WechatPayPlatformSerialNo',
      value: next.WechatPayPlatformSerialNo,
    })
  }
  if (
    next.WechatPayPlatformPublicKey &&
    next.WechatPayPlatformPublicKey !== initial.WechatPayPlatformPublicKey
  ) {
    updates.push({
      key: 'WechatPayPlatformPublicKey',
      value: next.WechatPayPlatformPublicKey,
    })
  }
  if (next.WechatPayUnitPrice !== initial.WechatPayUnitPrice) {
    updates.push({ key: 'WechatPayUnitPrice', value: next.WechatPayUnitPrice })
  }
  if (next.WechatPayMinTopUp !== initial.WechatPayMinTopUp) {
    updates.push({ key: 'WechatPayMinTopUp', value: next.WechatPayMinTopUp })
  }
  if (next.WechatPayNotifyUrl !== initial.WechatPayNotifyUrl) {
    updates.push({ key: 'WechatPayNotifyUrl', value: next.WechatPayNotifyUrl })
  }

  // 后端启用校验依赖当前已保存的密钥与商户信息，所以启用开关必须最后提交。
  if (next.WechatPayEnabled !== initial.WechatPayEnabled) {
    updates.push({ key: 'WechatPayEnabled', value: next.WechatPayEnabled })
  }

  return updates
}
