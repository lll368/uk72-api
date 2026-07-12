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
import { resolveQrPaymentPolledStatus } from './use-qr-payment-status-polling'

describe('resolveQrPaymentPolledStatus', () => {
  test('keeps pending when backend is successful but success refresh fails', async () => {
    const status = await resolveQrPaymentPolledStatus('success', async () => false)

    expect(status).toBe('pending')
  })

  test('returns success only after the success refresh is handled', async () => {
    const status = await resolveQrPaymentPolledStatus('success', async () => true)

    expect(status).toBe('success')
  })

  test('returns terminal failure states without running success refresh', async () => {
    let successRefreshCalled = false
    const status = await resolveQrPaymentPolledStatus('failed', async () => {
      successRefreshCalled = true
      return true
    })

    expect(status).toBe('failed')
    expect(successRefreshCalled).toBe(false)
  })
})
