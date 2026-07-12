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
import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { resetPasswordByEmail } from '@/features/auth/api'
import {
  PASSWORD_MAX_LENGTH,
  PASSWORD_MIN_LENGTH,
} from '@/features/auth/constants'
import { useTurnstile } from '@/features/auth/hooks/use-turnstile'
import { hasLetterAndNumber } from '@/features/auth/lib/account'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'
import { Turnstile } from '@/components/turnstile'
import { AuthLayout } from '../auth-layout'

export type ResetPasswordSearchParams = {
  email?: string
  token?: string
}

type ResetPasswordConfirmProps = ResetPasswordSearchParams

export function ResetPasswordConfirm({
  email,
  token,
}: ResetPasswordConfirmProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  } = useTurnstile()

  const isValidResetLink = Boolean(email && token)

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!isValidResetLink || !email || !token) {
      toast.error(t('Invalid reset link, please request a new password reset'))
      return
    }
    if (
      password.length < PASSWORD_MIN_LENGTH ||
      password.length > PASSWORD_MAX_LENGTH
    ) {
      toast.error(t('Password must be 8-20 characters long'))
      return
    }
    if (!hasLetterAndNumber(password)) {
      toast.error(t('Password must contain letters and numbers'))
      return
    }
    if (password !== confirmPassword) {
      toast.error(t("Passwords don't match."))
      return
    }
    if (!validateTurnstile()) return

    setLoading(true)
    try {
      const res = await resetPasswordByEmail(
        email,
        token,
        password,
        turnstileToken
      )
      if (res?.success) {
        toast.success(t('Password updated successfully'))
        navigate({ to: '/sign-in', replace: true })
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setLoading(false)
    }
  }

  return (
    <AuthLayout>
      <form onSubmit={handleSubmit} className='grid gap-5'>
        <div className='border-b border-slate-100 pb-3'>
          <h2 className='text-sm font-semibold text-blue-600'>
            {t('Enter new password')}
          </h2>
        </div>

        {!isValidResetLink && (
          <Alert variant='destructive'>
            <AlertDescription>
              {t('Invalid reset link, please request a new password reset.')}
            </AlertDescription>
          </Alert>
        )}

        <Input
          type='email'
          value={email || ''}
          disabled
          placeholder={t('Waiting for email...')}
          className='h-9 rounded-sm border-slate-200 bg-white text-sm'
        />

        <PasswordInput
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          placeholder={t('New password')}
          autoComplete='new-password'
          disabled={!isValidResetLink || loading}
          className='[&_input]:h-9 [&_input]:rounded-sm [&_input]:border-slate-200 [&_input]:bg-white [&_input]:text-sm'
        />

        <PasswordInput
          value={confirmPassword}
          onChange={(event) => setConfirmPassword(event.target.value)}
          placeholder={t('Confirm password')}
          autoComplete='new-password'
          disabled={!isValidResetLink || loading}
          className='[&_input]:h-9 [&_input]:rounded-sm [&_input]:border-slate-200 [&_input]:bg-white [&_input]:text-sm'
        />

        <p className='text-xs leading-5 text-slate-600'>
          <span className='text-red-500'>*</span>{' '}
          {t('Password must be 8-20 characters and contain letters and numbers')}
        </p>

        {isTurnstileEnabled && (
          <Turnstile siteKey={turnstileSiteKey} onVerify={setTurnstileToken} />
        )}

        <div className='grid grid-cols-2 gap-3 pt-1'>
          <Button
            type='button'
            variant='outline'
            className='h-9 rounded-sm border-blue-600 text-sm text-blue-600 hover:text-blue-700'
            onClick={() => navigate({ to: '/sign-in', replace: true })}
            disabled={loading}
          >
            {t('Back')}
          </Button>
          <Button
            type='submit'
            className='h-9 rounded-sm bg-blue-600 text-sm text-white hover:bg-blue-700'
            disabled={!isValidResetLink || loading}
          >
            {loading ? <Loader2 className='h-4 w-4 animate-spin' /> : null}
            {t('Change password')}
          </Button>
        </div>
      </form>
    </AuthLayout>
  )
}
