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

export interface AlipaySettingsValues {
  AlipayEnabled: boolean
  AlipaySandbox: boolean
  AlipayAppId: string
  AlipayPrivateKey: string
  AlipayPublicKey: string
  AlipayUnitPrice: number
  AlipayMinTopUp: number
  AlipayReturnUrl: string
  AlipayNotifyUrl: string
}

export type AlipayOptionUpdate = {
  key: string
  value: string | number | boolean
}

export function normalizeAlipaySettings(
  values: AlipaySettingsValues
): AlipaySettingsValues {
  return {
    AlipayEnabled: !!values.AlipayEnabled,
    AlipaySandbox: !!values.AlipaySandbox,
    AlipayAppId: values.AlipayAppId.trim(),
    AlipayPrivateKey: values.AlipayPrivateKey.trim(),
    AlipayPublicKey: values.AlipayPublicKey.trim(),
    AlipayUnitPrice: Number(values.AlipayUnitPrice),
    AlipayMinTopUp: Number(values.AlipayMinTopUp),
    AlipayReturnUrl: removeTrailingSlash(values.AlipayReturnUrl),
    AlipayNotifyUrl: removeTrailingSlash(values.AlipayNotifyUrl),
  }
}

export function buildAlipaySettingsUpdates(
  initialValues: AlipaySettingsValues,
  nextValues: AlipaySettingsValues
): AlipayOptionUpdate[] {
  const initial = normalizeAlipaySettings(initialValues)
  const next = normalizeAlipaySettings(nextValues)
  const updates: AlipayOptionUpdate[] = []

  if (next.AlipayEnabled !== initial.AlipayEnabled) {
    updates.push({ key: 'AlipayEnabled', value: next.AlipayEnabled })
  }
  if (next.AlipaySandbox !== initial.AlipaySandbox) {
    updates.push({ key: 'AlipaySandbox', value: next.AlipaySandbox })
  }
  if (next.AlipayAppId !== initial.AlipayAppId) {
    updates.push({ key: 'AlipayAppId', value: next.AlipayAppId })
  }
  if (
    next.AlipayPrivateKey &&
    next.AlipayPrivateKey !== initial.AlipayPrivateKey
  ) {
    updates.push({ key: 'AlipayPrivateKey', value: next.AlipayPrivateKey })
  }
  if (
    next.AlipayPublicKey &&
    next.AlipayPublicKey !== initial.AlipayPublicKey
  ) {
    updates.push({ key: 'AlipayPublicKey', value: next.AlipayPublicKey })
  }
  if (next.AlipayUnitPrice !== initial.AlipayUnitPrice) {
    updates.push({ key: 'AlipayUnitPrice', value: next.AlipayUnitPrice })
  }
  if (next.AlipayMinTopUp !== initial.AlipayMinTopUp) {
    updates.push({ key: 'AlipayMinTopUp', value: next.AlipayMinTopUp })
  }
  if (next.AlipayReturnUrl !== initial.AlipayReturnUrl) {
    updates.push({ key: 'AlipayReturnUrl', value: next.AlipayReturnUrl })
  }
  if (next.AlipayNotifyUrl !== initial.AlipayNotifyUrl) {
    updates.push({ key: 'AlipayNotifyUrl', value: next.AlipayNotifyUrl })
  }

  return updates
}
