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
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, test } from 'node:test'

const currentDir = fileURLToPath(new URL('.', import.meta.url))

function readComponent(fileName: string): string {
  return readFileSync(new URL(fileName, `file://${currentDir}`), 'utf8')
}

describe('pricing frontend pagination', () => {
  test('table and card views render all filtered models without local pagination', () => {
    const table = readComponent('pricing-table.tsx')
    const cardGrid = readComponent('model-card-grid.tsx')

    assert.equal(table.includes('DataTablePagination'), false)
    assert.equal(table.includes('getPaginationRowModel'), false)
    assert.equal(table.includes('PaginationState'), false)

    assert.equal(cardGrid.includes('DEFAULT_PRICING_PAGE_SIZE'), false)
    assert.equal(cardGrid.includes('Previous page'), false)
    assert.equal(cardGrid.includes('Next page'), false)
  })
})

describe('pricing toolbar controls', () => {
  test('does not render the standard/recharge price mode switch', () => {
    const toolbar = readComponent('pricing-toolbar.tsx')

    assert.equal(toolbar.includes("t('Standard')"), false)
    assert.equal(toolbar.includes("t('Recharge')"), false)
    assert.equal(toolbar.includes('onRechargePriceChange'), false)
    assert.equal(toolbar.includes('Price display mode'), false)
  })
})
