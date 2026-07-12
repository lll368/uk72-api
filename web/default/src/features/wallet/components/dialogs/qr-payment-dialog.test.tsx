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
import { QRCodeSVG } from 'qrcode.react'
import { renderToStaticMarkup } from 'react-dom/server'
import {
  formatQrPaymentExpiresAt,
  getQrPaymentStatusHelpTextKey,
  getQrPaymentStatusLabelKey,
  getQrPaymentDialogHelpTextKey,
  getQrPaymentDialogTitleKey,
  getQrPaymentSuccessCloseLabelKey,
  shouldShowQrPaymentCode,
  shouldShowQrPaymentRefresh,
} from './qr-payment-dialog'

describe('QrPaymentDialog helpers', () => {
  test('selects purpose-specific title keys', () => {
    expect(getQrPaymentDialogTitleKey({ purpose: 'topup' })).toBe(
      'WeChat Pay top-up'
    )
    expect(getQrPaymentDialogTitleKey({ purpose: 'vvip_activation' })).toBe(
      'WeChat Pay VVIP activation'
    )
  })

  test('selects purpose-specific help text keys', () => {
    expect(getQrPaymentDialogHelpTextKey({ purpose: 'topup' })).toBe(
      'After completing payment in WeChat, refresh wallet or open order history to check the result.'
    )
    expect(getQrPaymentDialogHelpTextKey({ purpose: 'vvip_activation' })).toBe(
      'After completing payment in WeChat, refresh compute partners to check the activation result.'
    )
  })

  test('selects status-specific label keys for top-up QR payments', () => {
    expect(getQrPaymentStatusLabelKey('pending', { purpose: 'topup' })).toBe(
      'Payment pending'
    )
    expect(getQrPaymentStatusLabelKey('success', { purpose: 'topup' })).toBe(
      'Top-up successful'
    )
    expect(getQrPaymentStatusLabelKey('failed', { purpose: 'topup' })).toBe(
      'Payment failed'
    )
    expect(getQrPaymentStatusLabelKey('expired', { purpose: 'topup' })).toBe(
      'Payment expired'
    )
  })

  test('keeps VVIP QR payments on the shared status model', () => {
    expect(
      getQrPaymentStatusLabelKey('success', { purpose: 'vvip_activation' })
    ).toBe('Activation successful')
    expect(
      getQrPaymentStatusHelpTextKey('success', { purpose: 'vvip_activation' })
    ).toBe('Activation successful. Refresh compute partners to view the result.')
  })

  test('hides QR payment code and refresh action after success', () => {
    expect(shouldShowQrPaymentCode('success')).toBe(false)
    expect(shouldShowQrPaymentRefresh('success')).toBe(false)
  })

  test('keeps QR payment code and refresh action while pending', () => {
    expect(shouldShowQrPaymentCode('pending')).toBe(true)
    expect(shouldShowQrPaymentRefresh('pending')).toBe(true)
  })

  test('uses a dedicated close action for successful QR payments', () => {
    expect(getQrPaymentSuccessCloseLabelKey()).toBe('Close dialog')
  })

  test('formats second-based expiry timestamps', () => {
    const formatted = formatQrPaymentExpiresAt(1_900_000_000)

    expect(formatted).toContain('2030')
  })

  test('renders a WeChat Pay QR SVG from code_url', () => {
    const html = renderToStaticMarkup(
      <QRCodeSVG value='weixin://wxpay/bizpayurl?pr=test' size={128} />
    )

    expect(html).toContain('<svg')
    expect(html).toContain('height="128"')
  })
})
