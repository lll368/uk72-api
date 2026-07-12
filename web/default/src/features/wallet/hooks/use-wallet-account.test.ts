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
  getClearedWithdrawTaxTrialState,
  openPiggyContractPreviewWindow,
} from './use-wallet-account'

function fakePreviewWindow() {
  return {
    opener: { unsafe: true },
    location: { href: '' },
    closed: false,
    close() {
      this.closed = true
    },
  }
}

describe('openPiggyContractPreviewWindow', () => {
  test('opens a blank window first and navigates it to backend preview_url', async () => {
    const win = fakePreviewWindow()
    let opened = false

    const ok = await openPiggyContractPreviewWindow({
      openWindow: () => {
        opened = true
        return win
      },
      loadPreview: async () => ({
        success: true,
        data: {
          document_id: 'DOC-2102',
          preview_url: 'https://preview.example.com/contracts/DOC-2102',
        },
      }),
      onError: () => {
        throw new Error('must not toast on success')
      },
    })

    expect(ok).toBe(true)
    expect(opened).toBe(true)
    expect(win.location.href).toBe(
      'https://preview.example.com/contracts/DOC-2102'
    )
    expect(win.opener).toBeNull()
    expect(win.closed).toBe(false)
  })

  test('does not call backend when the blank window is blocked', async () => {
    let loadCalled = false
    const errors: string[] = []

    const ok = await openPiggyContractPreviewWindow({
      openWindow: () => null,
      loadPreview: async () => {
        loadCalled = true
        return { success: true }
      },
      onError: (message) => errors.push(message),
    })

    expect(ok).toBe(false)
    expect(loadCalled).toBe(false)
    expect(errors).toEqual(['Please allow pop-ups to open the contract'])
  })

  test('closes the blank window when backend returns no preview_url', async () => {
    const win = fakePreviewWindow()
    const errors: string[] = []

    const ok = await openPiggyContractPreviewWindow({
      openWindow: () => win,
      loadPreview: async () => ({
        success: true,
        message: 'success',
        data: { document_id: 'DOC-EMPTY', preview_url: '' },
      }),
      onError: (message) => errors.push(message),
    })

    expect(ok).toBe(false)
    expect(win.location.href).toBe('')
    expect(win.closed).toBe(true)
    expect(errors).toEqual(['Failed to open contract'])
  })
})

describe('getClearedWithdrawTaxTrialState', () => {
  test('invalidates the pending request and stops loading', () => {
    expect(getClearedWithdrawTaxTrialState(7)).toEqual({
      requestSeq: 8,
      taxTrial: null,
      loading: false,
      error: '',
    })
  })
})
