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
import * as z from 'zod'
import type { OptionUpdate } from './payment-settings-core'
import { removeTrailingSlash } from './utils'

export interface QiniuSettingsValues {
  Enabled: boolean
  BaseURL: string
  ChildAccountBaseURL: string
  AccessKey: string
  SecretKey: string
  RequestTimeout: number
  RetryIntervalSeconds: number
  OfficialLedgerEnabled: boolean
  OfficialLedgerCutoverTime: number
  OfficialLedgerSyncIntervalSeconds: number
  OfficialLedgerWindowHours: number
  OfficialLedgerWindowDays: number
  OfficialLedgerBatchSize: number
  OfficialLedgerRateLimitPerSecond: number
  OfficialLedgerRetryIntervalSeconds: number
  CostDetailCutoverTime: number
  CostDetailLookbackDays: number
  CostDetailAutoApplyEnabled: boolean
  MarketCatalogEnabled: boolean
  MarketCatalogBaseURL: string
  MarketCatalogTTLSeconds: number
  MarketCatalogOverseas: boolean
  MarketCatalogFallbackEnabled: boolean
  ChildAccountEmailDomain: string
  ChildAccountEmailPrefix: string
  ChildAccountPasswordLength: number
  ChildAccountRequestTimeout: number
  ChildAccountRetryIntervalSeconds: number
  ChildAccountBindingEnabled: boolean
  ChildAccountAssignmentMode: QiniuChildAccountAssignmentMode
  ChildAccountBindingCutoverTime: number
}

export const QINIU_DEFAULT_BASE_URL = 'https://api.qnaigc.com'
export const QINIU_CHILD_ACCOUNT_DEFAULT_BASE_URL = 'https://api.qiniu.com'
export const QINIU_MARKET_DEFAULT_BASE_URL = 'https://openai.qiniu.com'
export const QINIU_COST_DETAIL_MAX_LOOKBACK_DAYS = 30
export const QINIU_CHILD_ACCOUNT_DEFAULT_DOMAIN = 'uk72.cn'
export const QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_PARENT_ONLY = 'parent_only'
export const QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD =
  'one_key_one_child'

export type QiniuChildAccountAssignmentMode =
  | typeof QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_PARENT_ONLY
  | typeof QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD

function normalizeQiniuChildAccountAssignmentMode(
  value: string
): QiniuChildAccountAssignmentMode {
  const mode = value.trim().toLowerCase()
  if (mode === QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD) {
    return QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD
  }
  return QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_PARENT_ONLY
}

function isOptionalHttpUrl(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return true
  return /^https?:\/\//.test(trimmed)
}

export const qiniuSettingsSchema = z.object({
  Enabled: z.boolean(),
  BaseURL: z
    .string()
    .refine(
      isOptionalHttpUrl,
      'Provide a valid URL starting with http:// or https://'
    ),
  ChildAccountBaseURL: z
    .string()
    .refine(
      isOptionalHttpUrl,
      'Provide a valid URL starting with http:// or https://'
    ),
  AccessKey: z.string(),
  SecretKey: z.string(),
  RequestTimeout: z.number().int().gt(0),
  RetryIntervalSeconds: z.number().int().gt(0),
  OfficialLedgerEnabled: z.boolean(),
  OfficialLedgerCutoverTime: z.number().int().min(0),
  OfficialLedgerSyncIntervalSeconds: z.number().int().gt(0),
  OfficialLedgerWindowHours: z.number().int().gt(0),
  OfficialLedgerWindowDays: z.number().int().gt(0),
  OfficialLedgerBatchSize: z.number().int().gt(0),
  OfficialLedgerRateLimitPerSecond: z.number().int().gt(0),
  OfficialLedgerRetryIntervalSeconds: z.number().int().gt(0),
  CostDetailCutoverTime: z.number().int().min(0),
  CostDetailLookbackDays: z
    .number()
    .int()
    .gt(0)
    .lte(QINIU_COST_DETAIL_MAX_LOOKBACK_DAYS),
  CostDetailAutoApplyEnabled: z.boolean(),
  MarketCatalogEnabled: z.boolean(),
  MarketCatalogBaseURL: z
    .string()
    .refine(
      isOptionalHttpUrl,
      'Provide a valid URL starting with http:// or https://'
    ),
  MarketCatalogTTLSeconds: z.number().int().gt(0),
  MarketCatalogOverseas: z.boolean(),
  MarketCatalogFallbackEnabled: z.boolean(),
  ChildAccountEmailDomain: z
    .string()
    .trim()
    .min(1)
    .refine(
      (value) =>
        /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/i.test(
          value.replace(/^@/, '')
        ),
      'Enter a valid email domain'
    ),
  ChildAccountEmailPrefix: z
    .string()
    .trim()
    .min(1)
    .regex(
      /^[A-Za-z0-9._-]+$/,
      'Use only letters, numbers, dots, underscores, and hyphens'
    ),
  ChildAccountPasswordLength: z.number().int().min(12).max(64),
  ChildAccountRequestTimeout: z.number().int().gt(0),
  ChildAccountRetryIntervalSeconds: z.number().int().gt(0),
  ChildAccountBindingEnabled: z.boolean(),
  ChildAccountAssignmentMode: z.enum([
    QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_PARENT_ONLY,
    QINIU_CHILD_ACCOUNT_ASSIGNMENT_MODE_ONE_KEY_ONE_CHILD,
  ]),
  ChildAccountBindingCutoverTime: z.number().int().min(0),
})

export function normalizeQiniuSettings(
  values: QiniuSettingsValues
): QiniuSettingsValues {
  return {
    Enabled: !!values.Enabled,
    BaseURL: removeTrailingSlash(values.BaseURL),
    ChildAccountBaseURL:
      removeTrailingSlash(values.ChildAccountBaseURL) ||
      QINIU_CHILD_ACCOUNT_DEFAULT_BASE_URL,
    AccessKey: values.AccessKey.trim(),
    SecretKey: values.SecretKey.trim(),
    RequestTimeout: Number(values.RequestTimeout),
    RetryIntervalSeconds: Number(values.RetryIntervalSeconds),
    OfficialLedgerEnabled: !!values.OfficialLedgerEnabled,
    OfficialLedgerCutoverTime: Number(values.OfficialLedgerCutoverTime),
    OfficialLedgerSyncIntervalSeconds: Number(
      values.OfficialLedgerSyncIntervalSeconds
    ),
    OfficialLedgerWindowHours: Number(values.OfficialLedgerWindowHours),
    OfficialLedgerWindowDays: Number(values.OfficialLedgerWindowDays),
    OfficialLedgerBatchSize: Number(values.OfficialLedgerBatchSize),
    OfficialLedgerRateLimitPerSecond: Number(
      values.OfficialLedgerRateLimitPerSecond
    ),
    OfficialLedgerRetryIntervalSeconds: Number(
      values.OfficialLedgerRetryIntervalSeconds
    ),
    CostDetailCutoverTime: Number(values.CostDetailCutoverTime),
    CostDetailLookbackDays: Number(values.CostDetailLookbackDays),
    CostDetailAutoApplyEnabled: !!values.CostDetailAutoApplyEnabled,
    MarketCatalogEnabled: !!values.MarketCatalogEnabled,
    MarketCatalogBaseURL: removeTrailingSlash(values.MarketCatalogBaseURL),
    MarketCatalogTTLSeconds: Number(values.MarketCatalogTTLSeconds),
    MarketCatalogOverseas: !!values.MarketCatalogOverseas,
    MarketCatalogFallbackEnabled: !!values.MarketCatalogFallbackEnabled,
    ChildAccountEmailDomain:
      values.ChildAccountEmailDomain.trim().toLowerCase().replace(/^@/, '') ||
      QINIU_CHILD_ACCOUNT_DEFAULT_DOMAIN,
    ChildAccountEmailPrefix: values.ChildAccountEmailPrefix.trim() || 'child',
    ChildAccountPasswordLength: Number(values.ChildAccountPasswordLength),
    ChildAccountRequestTimeout: Number(values.ChildAccountRequestTimeout),
    ChildAccountRetryIntervalSeconds: Number(
      values.ChildAccountRetryIntervalSeconds
    ),
    ChildAccountBindingEnabled: !!values.ChildAccountBindingEnabled,
    ChildAccountAssignmentMode: normalizeQiniuChildAccountAssignmentMode(
      values.ChildAccountAssignmentMode
    ),
    ChildAccountBindingCutoverTime: Number(
      values.ChildAccountBindingCutoverTime
    ),
  }
}

export function buildQiniuSettingsUpdates(
  initialValues: QiniuSettingsValues,
  nextValues: QiniuSettingsValues
): OptionUpdate[] {
  const initial = normalizeQiniuSettings(initialValues)
  const next = normalizeQiniuSettings(nextValues)
  const updates: OptionUpdate[] = []

  pushChanged(
    updates,
    'qiniu_key_setting.enabled',
    initial.Enabled,
    next.Enabled
  )
  pushChanged(
    updates,
    'qiniu_key_setting.base_url',
    initial.BaseURL,
    next.BaseURL
  )
  pushChanged(
    updates,
    'qiniu_key_setting.child_account_base_url',
    initial.ChildAccountBaseURL,
    next.ChildAccountBaseURL
  )
  pushSecretChanged(
    updates,
    'qiniu_key_setting.access_key',
    initial.AccessKey,
    next.AccessKey
  )
  pushSecretChanged(
    updates,
    'qiniu_key_setting.secret_key',
    initial.SecretKey,
    next.SecretKey
  )
  pushChanged(
    updates,
    'qiniu_key_setting.request_timeout',
    initial.RequestTimeout,
    next.RequestTimeout
  )
  pushChanged(
    updates,
    'qiniu_key_setting.retry_interval_seconds',
    initial.RetryIntervalSeconds,
    next.RetryIntervalSeconds
  )
  pushChanged(
    updates,
    'qiniu_key_setting.official_ledger_enabled',
    initial.OfficialLedgerEnabled,
    next.OfficialLedgerEnabled
  )
  pushChanged(
    updates,
    'qiniu_key_setting.official_ledger_cutover_time',
    initial.OfficialLedgerCutoverTime,
    next.OfficialLedgerCutoverTime
  )
  pushChanged(
    updates,
    'qiniu_key_setting.official_ledger_sync_interval_seconds',
    initial.OfficialLedgerSyncIntervalSeconds,
    next.OfficialLedgerSyncIntervalSeconds
  )
  pushChanged(
    updates,
    'qiniu_key_setting.official_ledger_window_hours',
    initial.OfficialLedgerWindowHours,
    next.OfficialLedgerWindowHours
  )
  pushChanged(
    updates,
    'qiniu_key_setting.official_ledger_window_days',
    initial.OfficialLedgerWindowDays,
    next.OfficialLedgerWindowDays
  )
  pushChanged(
    updates,
    'qiniu_key_setting.official_ledger_batch_size',
    initial.OfficialLedgerBatchSize,
    next.OfficialLedgerBatchSize
  )
  pushChanged(
    updates,
    'qiniu_key_setting.official_ledger_rate_limit_per_second',
    initial.OfficialLedgerRateLimitPerSecond,
    next.OfficialLedgerRateLimitPerSecond
  )
  pushChanged(
    updates,
    'qiniu_key_setting.official_ledger_retry_interval_seconds',
    initial.OfficialLedgerRetryIntervalSeconds,
    next.OfficialLedgerRetryIntervalSeconds
  )
  pushChanged(
    updates,
    'qiniu_key_setting.cost_detail_cutover_time',
    initial.CostDetailCutoverTime,
    next.CostDetailCutoverTime
  )
  pushChanged(
    updates,
    'qiniu_key_setting.cost_detail_lookback_days',
    initial.CostDetailLookbackDays,
    next.CostDetailLookbackDays
  )
  pushChanged(
    updates,
    'qiniu_key_setting.cost_detail_auto_apply_enabled',
    initial.CostDetailAutoApplyEnabled,
    next.CostDetailAutoApplyEnabled
  )
  pushChanged(
    updates,
    'qiniu_key_setting.market_catalog_enabled',
    initial.MarketCatalogEnabled,
    next.MarketCatalogEnabled
  )
  pushChanged(
    updates,
    'qiniu_key_setting.market_catalog_base_url',
    initial.MarketCatalogBaseURL,
    next.MarketCatalogBaseURL
  )
  pushChanged(
    updates,
    'qiniu_key_setting.market_catalog_ttl_seconds',
    initial.MarketCatalogTTLSeconds,
    next.MarketCatalogTTLSeconds
  )
  pushChanged(
    updates,
    'qiniu_key_setting.market_catalog_overseas',
    initial.MarketCatalogOverseas,
    next.MarketCatalogOverseas
  )
  pushChanged(
    updates,
    'qiniu_key_setting.market_catalog_fallback_enabled',
    initial.MarketCatalogFallbackEnabled,
    next.MarketCatalogFallbackEnabled
  )
  pushChanged(
    updates,
    'qiniu_key_setting.child_account_email_domain',
    initial.ChildAccountEmailDomain,
    next.ChildAccountEmailDomain
  )
  pushChanged(
    updates,
    'qiniu_key_setting.child_account_email_prefix',
    initial.ChildAccountEmailPrefix,
    next.ChildAccountEmailPrefix
  )
  pushChanged(
    updates,
    'qiniu_key_setting.child_account_password_length',
    initial.ChildAccountPasswordLength,
    next.ChildAccountPasswordLength
  )
  pushChanged(
    updates,
    'qiniu_key_setting.child_account_request_timeout',
    initial.ChildAccountRequestTimeout,
    next.ChildAccountRequestTimeout
  )
  pushChanged(
    updates,
    'qiniu_key_setting.child_account_retry_interval_seconds',
    initial.ChildAccountRetryIntervalSeconds,
    next.ChildAccountRetryIntervalSeconds
  )
  pushChanged(
    updates,
    'qiniu_key_setting.child_account_binding_enabled',
    initial.ChildAccountBindingEnabled,
    next.ChildAccountBindingEnabled
  )
  pushChanged(
    updates,
    'qiniu_key_setting.child_account_assignment_mode',
    initial.ChildAccountAssignmentMode,
    next.ChildAccountAssignmentMode
  )
  pushChanged(
    updates,
    'qiniu_key_setting.child_account_binding_cutover_time',
    initial.ChildAccountBindingCutoverTime,
    next.ChildAccountBindingCutoverTime
  )

  return updates
}

function pushChanged(
  updates: OptionUpdate[],
  key: string,
  initial: string | number | boolean,
  next: string | number | boolean
) {
  if (next !== initial) {
    updates.push({ key, value: next })
  }
}

function pushSecretChanged(
  updates: OptionUpdate[],
  key: string,
  initial: string,
  next: string
) {
  // 后端不会回显敏感配置；空值只表示保留已有 AK/SK，避免误清空生产密钥。
  if (next && next !== initial) {
    updates.push({ key, value: next })
  }
}
