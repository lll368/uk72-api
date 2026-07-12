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
import {
  DASHBOARD_SECTION_IDS,
  getDashboardSectionNavItems,
  getVisibleDashboardSectionIds,
  isDashboardSectionAdminOnly,
} from './section-registry'

const t = ((key: string) => key) as TFunction

describe('dashboard section registry', () => {
  test('exposes rankings as an admin-only dashboard section', () => {
    expect(DASHBOARD_SECTION_IDS).toContain('rankings')
    expect(isDashboardSectionAdminOnly('users')).toBe(true)
    expect(isDashboardSectionAdminOnly('rankings')).toBe(true)
    expect(isDashboardSectionAdminOnly('models')).toBe(false)
  })

  test('hides admin-only dashboard sections from non-admin navigation', () => {
    const navTitles = getDashboardSectionNavItems(t).map((item) => item.title)

    expect(navTitles).toContain('Model Call Analytics')
    expect(navTitles.includes('User Analytics')).toBe(false)
    expect(navTitles.includes('Rankings')).toBe(false)
  })

  test('returns the correct visible dashboard sections for admins', () => {
    expect(getVisibleDashboardSectionIds(false)).toEqual(['overview', 'models'])
    expect(getVisibleDashboardSectionIds(true)).toEqual([
      'overview',
      'models',
      'users',
      'rankings',
    ])
  })
})
