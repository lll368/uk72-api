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
import { runQrPaymentSuccessRefreshTasks } from './qr-payment-refresh'

async function captureConsoleError<T>(
  action: () => Promise<T>
): Promise<{ result: T; loggedArgs: unknown[][] }> {
  const originalConsoleError = console.error
  const loggedArgs: unknown[][] = []

  console.error = (...args: unknown[]) => {
    loggedArgs.push(args)
  }

  try {
    const result = await action()
    return { result, loggedArgs }
  } finally {
    console.error = originalConsoleError
  }
}

describe('runQrPaymentSuccessRefreshTasks', () => {
  test('returns true when every refresh task succeeds', async () => {
    const refreshed = await runQrPaymentSuccessRefreshTasks([
      async () => true,
      async () => undefined,
    ])

    expect(refreshed).toBe(true)
  })

  test('returns false when a refresh task reports failure', async () => {
    const refreshed = await runQrPaymentSuccessRefreshTasks([
      async () => true,
      async () => false,
    ])

    expect(refreshed).toBe(false)
  })

  test('returns false when a refresh task rejects', async () => {
    const { result: refreshed } = await captureConsoleError(() =>
      runQrPaymentSuccessRefreshTasks([
        async () => true,
        async () => {
          throw new Error('network failed')
        },
      ])
    )

    expect(refreshed).toBe(false)
  })

  test('logs rejected refresh task reason for diagnostics', async () => {
    const error = new Error('network failed')

    const { loggedArgs } = await captureConsoleError(() =>
      runQrPaymentSuccessRefreshTasks([
        async () => {
          throw error
        },
      ])
    )

    expect(loggedArgs).toEqual([
      [
        'QR payment success refresh task failed:',
        error,
      ],
    ])
  })
})
