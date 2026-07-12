/*
Copyright (C) 2025 QuantumNous

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

import { describe, expect, test } from 'bun:test';
import {
  buildEmailLoginPayload,
  buildEmailRegisterPayload,
  buildPasswordResetConfirmPayload,
  buildPasswordResetVerifyPayload,
  buildPhoneLoginPayload,
  buildPhoneRegisterPayload,
  buildPhoneVerificationPayload,
  getPhoneCodeButtonText,
  getPhoneResetPassword,
  hasLetterAndNumber,
  normalizeAuthAccount,
} from './phoneAuth.js';

describe('phoneAuth helper', () => {
  test('builds backend payloads with trimmed phone verification fields', () => {
    expect(
      buildPhoneVerificationPayload(' 13800138000 ', 'login'),
    ).toEqual({
      phone_number: '13800138000',
      purpose: 'login',
    });

    expect(buildPhoneLoginPayload(' 13800138000 ', ' 123456 ')).toEqual({
      phone_number: '13800138000',
      verification_code: '123456',
    });
  });

  test('builds phone register payload without empty affiliate code', () => {
    expect(
      buildPhoneRegisterPayload({
        username: ' user1 ',
        password: 'password123',
        phoneNumber: ' 13800138000 ',
        verificationCode: ' 123456 ',
        affCode: '',
      }),
    ).toEqual({
      username: 'user1',
      password: 'password123',
      phone_number: '13800138000',
      verification_code: '123456',
    });
  });

  test('builds email register payload from email account and verification code', () => {
    expect(
      buildEmailRegisterPayload({
        email: ' Person@Example.COM ',
        password: 'password123',
        verificationCode: ' 123456 ',
        affCode: ' aff1 ',
      }),
    ).toEqual({
      username: 'person@example.com',
      email: 'person@example.com',
      password: 'password123',
      verification_code: '123456',
      aff_code: 'aff1',
    });
  });

  test('normalizes auth account and builds email login payload', () => {
    expect(normalizeAuthAccount(' Person@Example.COM ')).toEqual({
      type: 'email',
      value: 'person@example.com',
    });
    expect(normalizeAuthAccount(' 13800138000 ')).toEqual({
      type: 'phone',
      value: '13800138000',
    });
    expect(normalizeAuthAccount('legacy_user')).toBeNull();

    expect(buildEmailLoginPayload(' Person@Example.COM ', ' 123456 ')).toEqual({
      email: 'person@example.com',
      verification_code: '123456',
    });
  });

  test('builds password reset verify and confirm payloads', () => {
    expect(
      buildPasswordResetVerifyPayload(' Person@Example.COM ', ' 123456 '),
    ).toEqual({
      account: 'person@example.com',
      verification_code: '123456',
    });

    expect(
      buildPasswordResetConfirmPayload({
        accountType: 'phone',
        account: ' 13800138000 ',
        resetToken: ' token ',
        password: 'password123',
      }),
    ).toEqual({
      account_type: 'phone',
      account: '13800138000',
      reset_token: 'token',
      password: 'password123',
    });
  });

  test('validates password letter and number requirement', () => {
    expect(hasLetterAndNumber('password123')).toBe(true);
    expect(hasLetterAndNumber('abcdefgh')).toBe(false);
    expect(hasLetterAndNumber('12345678')).toBe(false);
  });

  test('returns countdown button text while code is cooling down', () => {
    expect(getPhoneCodeButtonText(false, 0, (key) => key)).toBe(
      '发送验证码',
    );
    expect(getPhoneCodeButtonText(true, 42, (key, vars) => `${vars.seconds}s`))
      .toBe('42s');
  });

  test('extracts generated password from phone reset response body', () => {
    expect(getPhoneResetPassword({ success: true, data: 'new-pass-123' })).toBe(
      'new-pass-123',
    );
    expect(getPhoneResetPassword({ success: true, data: { password: 'x' } }))
      .toBe('');
  });
});
