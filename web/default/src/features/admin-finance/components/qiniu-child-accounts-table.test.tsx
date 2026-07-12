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
import { renderToStaticMarkup } from 'react-dom/server'
import type { QiniuChildAccountDetail } from '../types'
import {
  ChildAccountDetail,
  getQiniuChildAccountImpactSummary,
} from './qiniu-child-accounts-table'

const t = ((key: string) => key) as TFunction

function createQiniuChildAccountDetail(): QiniuChildAccountDetail {
  return {
    id: 3003,
    sequence_no: 3,
    email: 'child3003@uk72.cn',
    remote_user_id: 'remote-3003',
    uid: 'uid-3003',
    parent_uid: 'parent-uid',
    access_key: 'ak***3003',
    backup_access_key: '',
    key_state: 'enabled',
    backup_key_state: '',
    status: 'enabled',
    last_error: '',
    user_count: 1,
    latest_task: null,
    created_by: 1,
    disabled_by: 0,
    disabled_reason: '',
    impact: {
      associated_user_count: 1,
      associated_token_count: 2,
      enabled_token_count: 1,
    },
    created_time: 1_710_000_000,
    updated_time: 1_710_000_100,
    disabled_time: 0,
    tasks: [],
    users: [
      {
        id: 1001,
        username: 'owner',
        display_name: 'Owner',
        email: 'owner@example.com',
      },
    ],
    tokens: [
      {
        id: 2001,
        user_id: 1001,
        username: 'owner',
        display_name: 'Owner',
        email: 'owner@example.com',
        name: 'Qiniu Managed Key',
        status: 1,
        qiniu_child_account_id: 3003,
        key_fingerprint: 'FP-3003',
        remote_cleanup_result: 'success',
        created_time: 1_710_000_000,
        accessed_time: 1_710_000_100,
        expired_time: 0,
        deleted: true,
        deleted_time: 1_710_000_200,
      },
    ],
  }
}

describe('QiniuChildAccountsTable detail behavior', () => {
  test('formats disable impact with associated users and tokens', () => {
    expect(
      getQiniuChildAccountImpactSummary(createQiniuChildAccountDetail(), t)
    ).toBe('Users: 1 · Tokens: 2 · Enabled Tokens: 1')
  })

  test('renders child account detail users, tokens, and cleanup result', () => {
    const html = renderToStaticMarkup(
      <ChildAccountDetail detail={createQiniuChildAccountDetail()} t={t} />
    )

    expect(html).toContain('owner@example.com')
    expect(html).toContain('Enabled Tokens')
    expect(html).toContain('Qiniu Managed Key')
    expect(html).toContain('FP-3003')
    expect(html).toContain('Enabled')
    expect(html).toContain('success')
  })

  test('renders empty users and tokens messages without crashing', () => {
    const detail = createQiniuChildAccountDetail()
    detail.users = []
    detail.tokens = []
    detail.user_count = 0
    detail.impact = {
      associated_user_count: 0,
      associated_token_count: 0,
      enabled_token_count: 0,
    }

    const html = renderToStaticMarkup(
      <ChildAccountDetail detail={detail} t={t} />
    )

    expect(html).toContain('No users are bound')
    expect(html).toContain('No tokens found')
  })
})
