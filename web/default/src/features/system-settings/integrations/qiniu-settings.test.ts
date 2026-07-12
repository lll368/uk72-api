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
  buildQiniuSettingsUpdates,
  QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD,
  QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_PARENT_ONLY,
  normalizeQiniuSettings,
  QINIU_COST_DETAIL_MAX_LOOKBACK_DAYS,
  qiniuSettingsSchema,
  type QiniuChildAccountAssignmentMode,
  type QiniuSettingsValues,
} from './qiniu-settings'

const baseSettings: QiniuSettingsValues = {
  Enabled: false,
  BaseURL: 'https://api.qnaigc.com',
  ChildAccountBaseURL: 'https://api.qiniu.com',
  AccessKey: '',
  SecretKey: '',
  RequestTimeout: 15,
  RetryIntervalSeconds: 300,
  OfficialLedgerEnabled: false,
  OfficialLedgerCutoverTime: 0,
  OfficialLedgerSyncIntervalSeconds: 60,
  OfficialLedgerWindowHours: 6,
  OfficialLedgerWindowDays: 2,
  OfficialLedgerBatchSize: 100,
  OfficialLedgerRateLimitPerSecond: 4,
  OfficialLedgerRetryIntervalSeconds: 300,
  CostDetailCutoverTime: 0,
  CostDetailLookbackDays: 7,
  CostDetailAutoApplyEnabled: true,
  MarketCatalogEnabled: false,
  MarketCatalogBaseURL: 'https://openai.qiniu.com',
  MarketCatalogTTLSeconds: 3600,
  MarketCatalogOverseas: true,
  MarketCatalogFallbackEnabled: true,
  ChildAccountEmailDomain: 'uk72.cn',
  ChildAccountEmailPrefix: 'child',
  ChildAccountPasswordLength: 18,
  ChildAccountRequestTimeout: 15,
  ChildAccountRetryIntervalSeconds: 300,
  ChildAccountBindingEnabled: false,
  ChildAccountAssignmentMode: QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_PARENT_ONLY,
  ChildAccountBindingCutoverTime: 0,
}

describe('normalizeQiniuSettings', () => {
  test('trims text fields, removes URL trailing slashes, and coerces numbers', () => {
    const normalized = normalizeQiniuSettings({
      ...baseSettings,
      BaseURL: ' https://api.qnaigc.com/ ',
      ChildAccountBaseURL: ' https://api.qiniu.com/ ',
      AccessKey: ' ak-value ',
      SecretKey: ' sk-value ',
      RequestTimeout: '20' as unknown as number,
      OfficialLedgerCutoverTime: '1710000000' as unknown as number,
      CostDetailCutoverTime: '1710086400' as unknown as number,
      MarketCatalogBaseURL: ' https://openai.qiniu.com/ ',
      ChildAccountAssignmentMode:
        ' ONE_KEY_ONE_CHILD ' as unknown as QiniuChildAccountAssignmentMode,
      ChildAccountBindingCutoverTime: '1710200000' as unknown as number,
    })

    expect(normalized.BaseURL).toBe('https://api.qnaigc.com')
    expect(normalized.ChildAccountBaseURL).toBe('https://api.qiniu.com')
    expect(normalized.AccessKey).toBe('ak-value')
    expect(normalized.SecretKey).toBe('sk-value')
    expect(normalized.RequestTimeout).toBe(20)
    expect(normalized.OfficialLedgerCutoverTime).toBe(1710000000)
    expect(normalized.CostDetailCutoverTime).toBe(1710086400)
    expect(normalized.MarketCatalogBaseURL).toBe('https://openai.qiniu.com')
    expect(normalized.ChildAccountAssignmentMode).toBe(
      QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD
    )
    expect(normalized.ChildAccountBindingCutoverTime).toBe(1710200000)
  })
})

describe('qiniuSettingsSchema', () => {
  test('rejects cost-detail lookback days beyond the safety limit', () => {
    expect(
      qiniuSettingsSchema.safeParse({
        ...baseSettings,
        CostDetailLookbackDays: QINIU_COST_DETAIL_MAX_LOOKBACK_DAYS,
      }).success
    ).toBe(true)
    expect(
      qiniuSettingsSchema.safeParse({
        ...baseSettings,
        CostDetailLookbackDays: QINIU_COST_DETAIL_MAX_LOOKBACK_DAYS + 1,
      }).success
    ).toBe(false)
  })

  test('rejects child account email prefixes that cannot be used in generated addresses', () => {
    expect(
      qiniuSettingsSchema.safeParse({
        ...baseSettings,
        ChildAccountEmailPrefix: 'qn-child_01',
      }).success
    ).toBe(true)
    expect(
      qiniuSettingsSchema.safeParse({
        ...baseSettings,
        ChildAccountEmailPrefix: 'bad prefix',
      }).success
    ).toBe(false)
    expect(
      qiniuSettingsSchema.safeParse({
        ...baseSettings,
        ChildAccountEmailPrefix: 'bad@prefix',
      }).success
    ).toBe(false)
  })

  test('rejects unsupported child account assignment modes', () => {
    expect(
      qiniuSettingsSchema.safeParse({
        ...baseSettings,
        ChildAccountAssignmentMode:
          QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD,
      }).success
    ).toBe(true)
    expect(
      qiniuSettingsSchema.safeParse({
        ...baseSettings,
        ChildAccountAssignmentMode: 'shared_child',
      }).success
    ).toBe(false)
  })
})

describe('buildQiniuSettingsUpdates', () => {
  test('builds complete qiniu_key_setting payload for changed fields', () => {
    const updates = buildQiniuSettingsUpdates(baseSettings, {
      ...baseSettings,
      Enabled: true,
      BaseURL: ' https://api.qnaigc.com/v2/ ',
      ChildAccountBaseURL: ' https://api.qiniu.com/oem/ ',
      AccessKey: ' qiniu-ak ',
      SecretKey: ' qiniu-sk ',
      RequestTimeout: 20,
      RetryIntervalSeconds: 600,
      OfficialLedgerEnabled: true,
      OfficialLedgerCutoverTime: 1710000000,
      OfficialLedgerSyncIntervalSeconds: 120,
      OfficialLedgerWindowHours: 8,
      OfficialLedgerWindowDays: 3,
      OfficialLedgerBatchSize: 50,
      OfficialLedgerRateLimitPerSecond: 2,
      OfficialLedgerRetryIntervalSeconds: 900,
      CostDetailCutoverTime: 1710086400,
      CostDetailLookbackDays: 4,
      CostDetailAutoApplyEnabled: false,
      MarketCatalogEnabled: true,
      MarketCatalogBaseURL: ' https://openai.qiniu.com/api/ ',
      MarketCatalogTTLSeconds: 180,
      MarketCatalogOverseas: false,
      MarketCatalogFallbackEnabled: false,
      ChildAccountEmailDomain: ' @UK72.CN ',
      ChildAccountEmailPrefix: ' qn-child ',
      ChildAccountPasswordLength: 24,
      ChildAccountRequestTimeout: 20,
      ChildAccountRetryIntervalSeconds: 600,
      ChildAccountBindingEnabled: true,
      ChildAccountAssignmentMode:
        QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD,
      ChildAccountBindingCutoverTime: 1710200000,
    })

    expect(updates).toEqual([
      { key: 'qiniu_key_setting.enabled', value: true },
      { key: 'qiniu_key_setting.base_url', value: 'https://api.qnaigc.com/v2' },
      {
        key: 'qiniu_key_setting.child_account_base_url',
        value: 'https://api.qiniu.com/oem',
      },
      { key: 'qiniu_key_setting.access_key', value: 'qiniu-ak' },
      { key: 'qiniu_key_setting.secret_key', value: 'qiniu-sk' },
      { key: 'qiniu_key_setting.request_timeout', value: 20 },
      { key: 'qiniu_key_setting.retry_interval_seconds', value: 600 },
      { key: 'qiniu_key_setting.official_ledger_enabled', value: true },
      {
        key: 'qiniu_key_setting.official_ledger_cutover_time',
        value: 1710000000,
      },
      {
        key: 'qiniu_key_setting.official_ledger_sync_interval_seconds',
        value: 120,
      },
      { key: 'qiniu_key_setting.official_ledger_window_hours', value: 8 },
      { key: 'qiniu_key_setting.official_ledger_window_days', value: 3 },
      { key: 'qiniu_key_setting.official_ledger_batch_size', value: 50 },
      {
        key: 'qiniu_key_setting.official_ledger_rate_limit_per_second',
        value: 2,
      },
      {
        key: 'qiniu_key_setting.official_ledger_retry_interval_seconds',
        value: 900,
      },
      {
        key: 'qiniu_key_setting.cost_detail_cutover_time',
        value: 1710086400,
      },
      {
        key: 'qiniu_key_setting.cost_detail_lookback_days',
        value: 4,
      },
      {
        key: 'qiniu_key_setting.cost_detail_auto_apply_enabled',
        value: false,
      },
      { key: 'qiniu_key_setting.market_catalog_enabled', value: true },
      {
        key: 'qiniu_key_setting.market_catalog_base_url',
        value: 'https://openai.qiniu.com/api',
      },
      { key: 'qiniu_key_setting.market_catalog_ttl_seconds', value: 180 },
      { key: 'qiniu_key_setting.market_catalog_overseas', value: false },
      {
        key: 'qiniu_key_setting.market_catalog_fallback_enabled',
        value: false,
      },
      {
        key: 'qiniu_key_setting.child_account_email_prefix',
        value: 'qn-child',
      },
      { key: 'qiniu_key_setting.child_account_password_length', value: 24 },
      { key: 'qiniu_key_setting.child_account_request_timeout', value: 20 },
      {
        key: 'qiniu_key_setting.child_account_retry_interval_seconds',
        value: 600,
      },
      { key: 'qiniu_key_setting.child_account_binding_enabled', value: true },
      {
        key: 'qiniu_key_setting.child_account_assignment_mode',
        value: QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD,
      },
      {
        key: 'qiniu_key_setting.child_account_binding_cutover_time',
        value: 1710200000,
      },
    ])
  })

  test('does not clear stored qiniu access key or secret key when fields are blank', () => {
    const updates = buildQiniuSettingsUpdates(
      {
        ...baseSettings,
        AccessKey: 'stored-ak',
        SecretKey: 'stored-sk',
      },
      {
        ...baseSettings,
        AccessKey: '',
        SecretKey: '',
        RequestTimeout: 30,
      }
    )

    expect(updates).toEqual([
      { key: 'qiniu_key_setting.request_timeout', value: 30 },
    ])
  })
})
