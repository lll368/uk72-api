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
import type { QiniuKeyListItem } from '../types'
import { renderQiniuKeyAccountOwnership } from './qiniu-keys-table'

const t = ((key: string) => key) as TFunction

function createQiniuKey(
  overrides: Partial<QiniuKeyListItem> = {}
): QiniuKeyListItem {
  return {
    token_id: 2001,
    user_id: 1001,
    name: 'Qiniu Managed Key',
    key: 'ak***abcd',
    status: 1,
    group: 'default',
    qiniu_child_account_id: 0,
    created_time: 1_710_000_000,
    accessed_time: 1_710_000_100,
    deleted: false,
    deleted_time: 0,
    user: {
      id: 1001,
      username: 'owner',
      display_name: 'Owner',
      email: 'owner@example.com',
    },
    quota: {
      applied_limit_amount: 10,
      pending_limit_amount: 0,
      failed_limit_amount: 0,
      latest_grant_error: '',
    },
    latest_task: null,
    ...overrides,
  }
}

describe('QiniuKeysTable account ownership display', () => {
  test('renders parent account ownership for historical parent keys', () => {
    const html = renderToStaticMarkup(
      renderQiniuKeyAccountOwnership(createQiniuKey(), t)
    )

    expect(html).toContain('Parent Account')
    expect(html.includes('Child Account')).toBe(false)
  })

  test('renders child account ownership with account id and email', () => {
    const html = renderToStaticMarkup(
      renderQiniuKeyAccountOwnership(
        createQiniuKey({
          qiniu_child_account_id: 3003,
          qiniu_child_account: {
            id: 3003,
            email: 'child3003@uk72.cn',
            uid: 'uid-3003',
            status: 'enabled',
          },
        }),
        t
      )
    )

    expect(html).toContain('Child Account')
    expect(html).toContain('#3003')
    expect(html).toContain('child3003@uk72.cn')
  })
})
