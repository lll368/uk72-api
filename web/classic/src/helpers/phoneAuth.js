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

const trimValue = (value) => String(value || '').trim();

export const PHONE_VERIFICATION_COUNTDOWN = 60;
export const PASSWORD_MIN_LENGTH = 8;
export const PASSWORD_MAX_LENGTH = 20;

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const E164_PHONE_REGEX = /^\+[1-9]\d{7,14}$/;
const MAINLAND_PHONE_REGEX = /^1\d{10}$/;

export function normalizeAuthAccount(account) {
  const value = trimValue(account);
  if (!value) return null;

  const lowerValue = value.toLowerCase();
  if (EMAIL_REGEX.test(lowerValue)) {
    return { type: 'email', value: lowerValue };
  }
  if (E164_PHONE_REGEX.test(value) || MAINLAND_PHONE_REGEX.test(value)) {
    return { type: 'phone', value };
  }
  return null;
}

export function isValidAuthAccount(account) {
  return normalizeAuthAccount(account) !== null;
}

export function hasLetterAndNumber(password) {
  return /[A-Za-z]/.test(String(password || '')) && /\d/.test(String(password || ''));
}

export function buildPhoneVerificationPayload(phoneNumber, purpose) {
  return {
    phone_number: trimValue(phoneNumber),
    purpose,
  };
}

export function buildPhoneLoginPayload(phoneNumber, verificationCode) {
  return {
    phone_number: trimValue(phoneNumber),
    verification_code: trimValue(verificationCode),
  };
}

export function buildEmailLoginPayload(email, verificationCode) {
  return {
    email: trimValue(email).toLowerCase(),
    verification_code: trimValue(verificationCode),
  };
}

export function buildPhoneRegisterPayload({
  username,
  password,
  phoneNumber,
  verificationCode,
  affCode,
}) {
  const payload = {
    username: trimValue(username),
    password,
    phone_number: trimValue(phoneNumber),
    verification_code: trimValue(verificationCode),
  };

  if (trimValue(affCode)) {
    payload.aff_code = trimValue(affCode);
  }

  return payload;
}

export function buildEmailRegisterPayload({
  email,
  password,
  verificationCode,
  affCode,
}) {
  const normalizedEmail = trimValue(email).toLowerCase();
  const payload = {
    username: normalizedEmail,
    email: normalizedEmail,
    password,
    verification_code: trimValue(verificationCode),
  };

  if (trimValue(affCode)) {
    payload.aff_code = trimValue(affCode);
  }

  return payload;
}

export function buildPasswordResetVerifyPayload(account, verificationCode) {
  const parsed = normalizeAuthAccount(account);
  return {
    account: parsed ? parsed.value : trimValue(account),
    verification_code: trimValue(verificationCode),
  };
}

export function buildPasswordResetConfirmPayload({
  accountType,
  account,
  resetToken,
  password,
}) {
  return {
    account_type: trimValue(accountType),
    account: trimValue(account),
    reset_token: trimValue(resetToken),
    password,
  };
}

export function getPhoneCodeButtonText(isCountingDown, secondsLeft, t) {
  if (isCountingDown) {
    return t('重试（{{seconds}}秒）', { seconds: secondsLeft });
  }
  return t('发送验证码');
}

export function getPhoneResetPassword(responseBody) {
  return typeof responseBody?.data === 'string' ? responseBody.data : '';
}

async function getAPI() {
  const { API } = await import('./api.js');
  return API;
}

export async function sendPhoneVerificationCode({
  phoneNumber,
  purpose,
  turnstile,
}) {
  const API = await getAPI();
  return API.post('/api/phone/verification', buildPhoneVerificationPayload(
    phoneNumber,
    purpose,
  ), {
    params: { turnstile: turnstile || '' },
  });
}

export async function sendAccountVerificationCode({
  account,
  purpose,
  turnstile,
}) {
  const parsed = normalizeAuthAccount(account);
  if (!parsed) {
    return {
      data: {
        success: false,
        message: 'Please enter a valid email or phone number',
      },
    };
  }

  const API = await getAPI();
  if (parsed.type === 'email') {
    if (purpose === 'reset_password') {
      return API.get('/api/reset_password', {
        params: {
          email: parsed.value,
          turnstile: turnstile || '',
          mode: 'code',
        },
      });
    }
    return API.get('/api/verification', {
      params: {
        email: parsed.value,
        turnstile: turnstile || '',
        purpose,
      },
    });
  }

  return sendPhoneVerificationCode({
    phoneNumber: parsed.value,
    purpose,
    turnstile,
  });
}

export async function loginByEmail({ email, verificationCode, turnstile }) {
  const API = await getAPI();
  return API.post(
    `/api/user/login/email?turnstile=${turnstile || ''}`,
    buildEmailLoginPayload(email, verificationCode),
  );
}

export async function loginByPhone({ phoneNumber, verificationCode, turnstile }) {
  const API = await getAPI();
  return API.post(
    `/api/user/login/phone?turnstile=${turnstile || ''}`,
    buildPhoneLoginPayload(phoneNumber, verificationCode),
  );
}

export async function registerByPhone({
  username,
  password,
  phoneNumber,
  verificationCode,
  affCode,
  turnstile,
}) {
  const API = await getAPI();
  return API.post('/api/user/register/phone', buildPhoneRegisterPayload({
    username,
    password,
    phoneNumber,
    verificationCode,
    affCode,
  }), {
    params: { turnstile: turnstile || '' },
  });
}

export async function resetPasswordByPhone({
  phoneNumber,
  verificationCode,
  password,
  turnstile,
}) {
  const API = await getAPI();
  return API.post('/api/user/reset_password/phone', {
    ...buildPhoneLoginPayload(phoneNumber, verificationCode),
    password,
  }, {
    params: { turnstile: turnstile || '' },
  });
}

export async function verifyPasswordResetCode({
  account,
  verificationCode,
  turnstile,
}) {
  const API = await getAPI();
  return API.post('/api/user/reset/verify', buildPasswordResetVerifyPayload(
    account,
    verificationCode,
  ), {
    params: { turnstile: turnstile || '' },
  });
}

export async function confirmPasswordReset({
  accountType,
  account,
  resetToken,
  password,
  turnstile,
}) {
  const API = await getAPI();
  return API.post('/api/user/reset/confirm', buildPasswordResetConfirmPayload({
    accountType,
    account,
    resetToken,
    password,
  }), {
    params: { turnstile: turnstile || '' },
  });
}

export async function resetPasswordByEmail({
  email,
  token,
  password,
  turnstile,
}) {
  const API = await getAPI();
  return API.post('/api/user/reset', {
    email: trimValue(email).toLowerCase(),
    token: trimValue(token),
    password,
  }, {
    params: { turnstile: turnstile || '' },
  });
}
