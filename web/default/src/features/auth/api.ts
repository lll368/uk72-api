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
import { api } from '@/lib/api'
import type {
  LoginPayload,
  LoginResponse,
  Login2FAResponse,
  TwoFAPayload,
  RegisterPayload,
  PhoneRegisterPayload,
  PhoneLoginPayload,
  EmailLoginPayload,
  PhoneVerificationPayload,
  PasswordResetVerifyPayload,
  PasswordResetConfirmPayload,
  PasswordResetSession,
  ApiResponse,
} from './types'
import { normalizeAuthAccount } from './lib/account'

// ============================================================================
// Authentication APIs
// ============================================================================

// ----------------------------------------------------------------------------
// Login & Logout
// ----------------------------------------------------------------------------

// User login with username and password
export async function login(payload: LoginPayload) {
  const turnstile = payload.turnstile ?? ''
  const res = await api.post<LoginResponse>(
    `/api/user/login?turnstile=${turnstile}`,
    {
      username: payload.username,
      password: payload.password,
    }
  )
  return res.data
}

// User login with phone verification code
export async function loginByPhone(payload: PhoneLoginPayload) {
  const turnstile = payload.turnstile ?? ''
  const res = await api.post<LoginResponse>(
    `/api/user/login/phone?turnstile=${turnstile}`,
    {
      phone_number: payload.phone_number,
      verification_code: payload.verification_code,
    }
  )
  return res.data
}

// User login with email verification code
export async function loginByEmail(payload: EmailLoginPayload) {
  const turnstile = payload.turnstile ?? ''
  const res = await api.post<LoginResponse>(
    `/api/user/login/email?turnstile=${turnstile}`,
    {
      email: payload.email,
      verification_code: payload.verification_code,
    }
  )
  return res.data
}

// Two-factor authentication login
export async function login2fa(payload: TwoFAPayload) {
  const res = await api.post<Login2FAResponse>('/api/user/login/2fa', payload)
  return res.data
}

// User logout
export async function logout(): Promise<ApiResponse> {
  const res = await api.get('/api/user/logout')
  return res.data
}

// ----------------------------------------------------------------------------
// Password Management
// ----------------------------------------------------------------------------

// Send password reset email
export async function sendPasswordResetEmail(
  email: string,
  turnstile?: string,
  mode?: 'link' | 'code'
): Promise<ApiResponse> {
  const res = await api.get('/api/reset_password', {
    params: { email, turnstile, mode },
  })
  return res.data
}

// Reset password by email verification token/code
export async function resetPasswordByEmail(
  email: string,
  token: string,
  password?: string,
  turnstile?: string
): Promise<ApiResponse> {
  const res = await api.post(
    '/api/user/reset',
    {
      email,
      token,
      password,
    },
    { params: { turnstile: turnstile ?? '' } }
  )
  return res.data
}

// Reset password by phone verification code
export async function resetPasswordByPhone(
  phoneNumber: string,
  verificationCode: string,
  password?: string,
  turnstile?: string
): Promise<ApiResponse> {
  const res = await api.post(
    '/api/user/reset_password/phone',
    {
      phone_number: phoneNumber,
      verification_code: verificationCode,
      password,
    },
    { params: { turnstile: turnstile ?? '' } }
  )
  return res.data
}

export async function verifyPasswordResetCode(
  payload: PasswordResetVerifyPayload
): Promise<ApiResponse & { data?: PasswordResetSession }> {
  const res = await api.post(
    '/api/user/reset/verify',
    {
      account: payload.account,
      verification_code: payload.verification_code,
    },
    { params: { turnstile: payload.turnstile ?? '' } }
  )
  return res.data
}

export async function confirmPasswordReset(
  payload: PasswordResetConfirmPayload
): Promise<ApiResponse> {
  const res = await api.post(
    '/api/user/reset/confirm',
    {
      account_type: payload.account_type,
      account: payload.account,
      reset_token: payload.reset_token,
      password: payload.password,
    },
    { params: { turnstile: payload.turnstile ?? '' } }
  )
  return res.data
}

// ----------------------------------------------------------------------------
// OAuth
// ----------------------------------------------------------------------------

// Start GitHub OAuth flow
export async function githubOAuthStart(clientId: string, state: string) {
  const url = `https://github.com/login/oauth/authorize?client_id=${clientId}&state=${state}&scope=user:email`
  window.open(url)
}

// Get OAuth state for CSRF protection
export async function getOAuthState(): Promise<string> {
  const aff =
    typeof window !== 'undefined' ? (localStorage.getItem('aff') ?? '') : ''
  const res = await api.get('/api/oauth/state', { params: { aff } })
  if (res.data?.success) return res.data.data
  return ''
}

// WeChat login by authorization code
export async function wechatLoginByCode(code: string): Promise<ApiResponse> {
  const res = await api.get('/api/oauth/wechat', { params: { code } })
  return res.data
}

// ----------------------------------------------------------------------------
// Registration
// ----------------------------------------------------------------------------

// User registration
export async function register(payload: RegisterPayload): Promise<ApiResponse> {
  const res = await api.post(`/api/user/register`, payload, {
    params: { turnstile: payload.turnstile ?? '' },
  })
  return res.data
}

// User registration by phone verification code
export async function registerByPhone(
  payload: PhoneRegisterPayload
): Promise<ApiResponse> {
  const res = await api.post(
    '/api/user/register/phone',
    {
      username: payload.username,
      password: payload.password,
      phone_number: payload.phone_number,
      verification_code: payload.verification_code,
      aff_code: payload.aff_code,
    },
    {
      params: { turnstile: payload.turnstile ?? '' },
    }
  )
  return res.data
}

// Send email verification code
export async function sendEmailVerification(
  email: string,
  turnstile?: string,
  purpose?: 'register' | 'login'
): Promise<ApiResponse> {
  const res = await api.get('/api/verification', {
    params: { email, turnstile, purpose },
  })
  return res.data
}

// Send phone verification code
export async function sendPhoneVerification(
  payload: PhoneVerificationPayload
): Promise<ApiResponse> {
  const res = await api.post(
    '/api/phone/verification',
    {
      phone_number: payload.phone_number,
      purpose: payload.purpose,
    },
    {
      params: { turnstile: payload.turnstile ?? '' },
    }
  )
  return res.data
}

export async function sendAccountVerificationCode(
  account: string,
  purpose: 'register' | 'login' | 'reset_password',
  turnstile?: string
): Promise<ApiResponse> {
  const parsed = normalizeAuthAccount(account)
  if (!parsed) {
    return {
      success: false,
      message: 'Please enter a valid email or phone number',
    }
  }
  if (parsed.type === 'email') {
    if (purpose === 'reset_password') {
      return sendPasswordResetEmail(parsed.value, turnstile, 'code')
    }
    return sendEmailVerification(parsed.value, turnstile, purpose)
  }
  return sendPhoneVerification({
    phone_number: parsed.value,
    purpose,
    turnstile,
  })
}

// Bind email to OAuth account
export async function bindEmail(
  email: string,
  code: string
): Promise<ApiResponse> {
  const res = await api.post('/api/oauth/email/bind', {
    email,
    code,
  })
  return res.data
}
