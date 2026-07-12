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
  BILLING_SECTION_IDS,
  getBillingSectionNavItems,
} from './section-registry'

const t = ((key: string) => key) as TFunction

describe('billing section registry', () => {
  test('hides local model and group pricing sections from billing navigation', () => {
    const navTitles = getBillingSectionNavItems(t).map((item) => item.title)

    expect(navTitles.includes('Model Pricing')).toBe(false)
    expect(navTitles.includes('Group Pricing')).toBe(false)
    expect(navTitles).toContain('Qiniu Key & Ledger')
  })

  test('does not allow direct routing to hidden pricing sections', () => {
    expect(BILLING_SECTION_IDS.includes('model-pricing')).toBe(false)
    expect(BILLING_SECTION_IDS.includes('group-pricing')).toBe(false)
    expect(BILLING_SECTION_IDS).toContain('qiniu')
  })
})
