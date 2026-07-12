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
import type { TFunction } from 'i18next'
import ar from '../i18n/locales/ar.json'
import en from '../i18n/locales/en.json'
import esES from '../i18n/locales/es-ES.json'
import fr from '../i18n/locales/fr.json'
import ja from '../i18n/locales/ja.json'
import ko from '../i18n/locales/ko.json'
import ptBR from '../i18n/locales/pt-BR.json'
import ru from '../i18n/locales/ru.json'
import vi from '../i18n/locales/vi.json'
import zhCN from '../i18n/locales/zh-CN.json'
import zhTW from '../i18n/locales/zh-TW.json'
import { buildSidebarData } from './use-sidebar-data'

const translations: Record<string, string> = {
  admin_qiniu_child_accounts_title: '七牛子账户管理',
  admin_qiniu_keys_title: '七牛密钥管理',
  admin_recharge_records_title: '充值记录',
  admin_withdrawals_title: '提现管理',
  admin_withdrawals_description: '查看、审核、驳回和登记佣金提现',
}

const t = ((key: string) =>
  translations[key] ?? `__MISSING__:${key}`) as TFunction

const locales = {
  ar,
  en,
  'es-ES': esES,
  fr,
  ja,
  ko,
  'pt-BR': ptBR,
  ru,
  vi,
  'zh-CN': zhCN,
  'zh-TW': zhTW,
} as const

describe('sidebar data', () => {
  test('exposes admin qiniu key management as a translated direct navigation entry', () => {
    const sidebarData = buildSidebarData(t)
    const adminGroup = sidebarData.navGroups.find(
      (group) => group.id === 'admin'
    )
    const titles = adminGroup?.items.map((item) => item.title) ?? []

    expect(titles).toContain('七牛密钥管理')
    expect(
      adminGroup?.items.some(
        (item) => 'url' in item && item.url === '/qiniu-keys'
      )
    ).toBe(true)
  })

  test('exposes qiniu child accounts as a translated direct navigation entry', () => {
    const sidebarData = buildSidebarData(t)
    const adminGroup = sidebarData.navGroups.find(
      (group) => group.id === 'admin'
    )
    const titles = adminGroup?.items.map((item) => item.title) ?? []

    expect(titles).toContain('七牛子账户管理')
    expect(
      adminGroup?.items.some(
        (item) => 'url' in item && item.url === '/qiniu-child-accounts'
      )
    ).toBe(true)
  })

  test('exposes withdrawal management above contact messages in admin navigation', () => {
    const sidebarData = buildSidebarData(t)
    const adminGroup = sidebarData.navGroups.find(
      (group) => group.id === 'admin'
    )
    const items = adminGroup?.items ?? []
    const withdrawalIndex = items.findIndex(
      (item) => 'url' in item && item.url === '/finance-withdrawals'
    )
    const contactMessagesIndex = items.findIndex(
      (item) => 'url' in item && item.url === '/contact-messages'
    )

    expect(withdrawalIndex >= 0).toBe(true)
    expect(items[withdrawalIndex]?.title).toBe('提现管理')
    expect(contactMessagesIndex >= 0).toBe(true)
    expect(withdrawalIndex < contactMessagesIndex).toBe(true)
  })

  test('exposes recharge records above withdrawal management in admin navigation', () => {
    const sidebarData = buildSidebarData(t)
    const adminGroup = sidebarData.navGroups.find(
      (group) => group.id === 'admin'
    )
    const items = adminGroup?.items ?? []
    const rechargeIndex = items.findIndex(
      (item) => 'url' in item && item.url === '/recharge-records'
    )
    const withdrawalIndex = items.findIndex(
      (item) => 'url' in item && item.url === '/finance-withdrawals'
    )

    expect(rechargeIndex >= 0).toBe(true)
    expect(items[rechargeIndex]?.title).toBe('充值记录')
    expect(withdrawalIndex >= 0).toBe(true)
    expect(rechargeIndex < withdrawalIndex).toBe(true)
  })

  test('defines recharge and withdrawal management translations in every locale', () => {
    Object.entries(locales).forEach(([locale, messages]) => {
      expect(messages.translation['admin_recharge_records_title']).toBeTruthy()
      expect(messages.translation['admin_withdrawals_title']).toBeTruthy()
      expect(messages.translation['admin_withdrawals_description']).toBeTruthy()
      expect(typeof messages.translation['admin_recharge_records_title']).toBe(
        'string'
      )
      expect(typeof messages.translation['admin_withdrawals_title']).toBe(
        'string'
      )
      expect(typeof messages.translation['admin_withdrawals_description']).toBe(
        'string'
      )
      expect(
        messages.translation['admin_recharge_records_title'].length > 0
      ).toBe(true)
      expect(messages.translation['admin_withdrawals_title'].length > 0).toBe(
        true
      )
      expect(
        messages.translation['admin_withdrawals_description'].length > 0
      ).toBe(true)
      expect(
        messages.translation['admin_recharge_records_title'] ===
          `__MISSING__:${locale}`
      ).toBe(false)
      expect(
        messages.translation['admin_withdrawals_title'] ===
          `__MISSING__:${locale}`
      ).toBe(false)
    })
  })
})
