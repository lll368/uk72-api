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
import { useEffect, useState } from 'react'
import type { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useCountdown } from '@/hooks/use-countdown'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import { PasswordInput } from '@/components/password-input'
import { Turnstile } from '@/components/turnstile'
import {
  register,
  registerByPhone,
  sendAccountVerificationCode,
} from '@/features/auth/api'
import { LegalConsent } from '@/features/auth/components/legal-consent'
import {
  PHONE_VERIFICATION_COUNTDOWN,
  registerFormSchema,
} from '@/features/auth/constants'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import { useTurnstile } from '@/features/auth/hooks/use-turnstile'
import { normalizeAuthAccount } from '@/features/auth/lib/account'
import { getAffiliateCode } from '@/features/auth/lib/storage'
import { getVerificationAccountValidationMessage } from '@/features/auth/lib/verification-code'

export function SignUpForm({
  className,
  ...props
}: React.HTMLAttributes<HTMLFormElement>) {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(false)
  const [isSendingCode, setIsSendingCode] = useState(false)
  const [agreedToLegal, setAgreedToLegal] = useState(false)

  const { systemName } = useSystemConfig()
  const { status } = useStatus()
  const {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  } = useTurnstile()
  const { redirectToLogin } = useAuthRedirect()
  const {
    secondsLeft: codeSecondsLeft,
    isActive: isCodeActive,
    start: startCodeCountdown,
  } = useCountdown({ initialSeconds: PHONE_VERIFICATION_COUNTDOWN })

  const form = useForm<z.infer<typeof registerFormSchema>>({
    resolver: zodResolver(registerFormSchema),
    defaultValues: {
      account: '',
      verificationCode: '',
      username: '',
      email: '',
      phoneNumber: '',
      password: '',
      confirmPassword: '',
    },
  })

  const accountValue = form.watch('account') || ''
  const hasUserAgreement = Boolean(status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(status?.privacy_policy_enabled)
  const requiresLegalConsent = hasUserAgreement || hasPrivacyPolicy

  useEffect(() => {
    setAgreedToLegal(!requiresLegalConsent)
  }, [requiresLegalConsent])

  async function onSubmit(data: z.infer<typeof registerFormSchema>) {
    if (requiresLegalConsent && !agreedToLegal) {
      toast.error(t('Please agree to the legal terms first'))
      return
    }
    if (!validateTurnstile()) return

    const account = data.account.trim()
    const parsed = normalizeAuthAccount(account)
    if (!parsed) {
      toast.error(t('Please enter a valid email or phone number'))
      return
    }
    if (!data.verificationCode) {
      toast.error(t('Please enter the verification code'))
      return
    }

    setIsLoading(true)
    try {
      const res =
        parsed.type === 'email'
          ? await register({
              username: parsed.value,
              email: parsed.value,
              password: data.password,
              verification_code: data.verificationCode,
              aff_code: getAffiliateCode(),
              turnstile: turnstileToken,
            })
          : await registerByPhone({
              username: parsed.value,
              password: data.password,
              phone_number: parsed.value,
              verification_code: data.verificationCode,
              aff_code: getAffiliateCode(),
              turnstile: turnstileToken,
            })

      if (res?.success) {
        toast.success(t('Account created! Please sign in'))
        redirectToLogin()
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsLoading(false)
    }
  }

  async function handleSendVerificationCode() {
    const account = accountValue.trim()
    const validationMessage = getVerificationAccountValidationMessage(account)
    if (validationMessage) {
      toast.error(t(validationMessage))
      return
    }
    if (!validateTurnstile()) return
    setIsSendingCode(true)
    try {
      const res = await sendAccountVerificationCode(
        account,
        'register',
        turnstileToken
      )
      if (res?.success) {
        startCodeCountdown()
        toast.success(t('Verification code sent'))
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsSendingCode(false)
    }
  }

  const isCodeDisabled =
    isLoading || isSendingCode || isCodeActive || !accountValue.trim()

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        <div className='border-b border-slate-100 pb-3'>
          <p className='text-sm font-semibold text-slate-900'>
            {t('Welcome to {{name}}', {
              name: systemName,
            })}
          </p>
          <h2 className='mt-1 text-base font-semibold text-slate-900'>
            {t('Create an account')}
          </h2>
        </div>

        <FormField
          control={form.control}
          name='account'
          render={({ field }) => (
            <FormItem>
              <FormControl>
                <Input
                  placeholder={t('Email or phone number')}
                  autoComplete='username'
                  className='h-9 rounded-sm border-slate-200 bg-white text-sm'
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='verificationCode'
          render={({ field }) => (
            <FormItem>
              <FormControl>
                <InputGroup className='h-9 rounded-sm border-slate-200 bg-white'>
                  <InputGroupInput
                    placeholder={t('Verification code')}
                    autoComplete='one-time-code'
                    className='text-sm'
                    {...field}
                  />
                  <InputGroupAddon align='inline-end'>
                    <InputGroupButton
                      type='button'
                      disabled={isCodeDisabled}
                      onClick={handleSendVerificationCode}
                      className='text-blue-600 hover:text-blue-700'
                    >
                      {isCodeActive
                        ? t('Resend ({{seconds}}s)', {
                            seconds: codeSecondsLeft,
                          })
                        : isSendingCode
                          ? t('Sending...')
                          : t('Get code')}
                    </InputGroupButton>
                  </InputGroupAddon>
                </InputGroup>
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormControl>
                <PasswordInput
                  placeholder={t('Set password')}
                  autoComplete='new-password'
                  className='[&_input]:h-9 [&_input]:rounded-sm [&_input]:border-slate-200 [&_input]:bg-white [&_input]:text-sm'
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='confirmPassword'
          render={({ field }) => (
            <FormItem>
              <FormControl>
                <PasswordInput
                  placeholder={t('Confirm password')}
                  autoComplete='new-password'
                  className='[&_input]:h-9 [&_input]:rounded-sm [&_input]:border-slate-200 [&_input]:bg-white [&_input]:text-sm'
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <p className='text-xs leading-5 text-slate-400'>
          {t(
            'Password must be 8-20 characters and contain letters and numbers'
          )}
        </p>

        {isTurnstileEnabled && (
          <Turnstile siteKey={turnstileSiteKey} onVerify={setTurnstileToken} />
        )}

        <LegalConsent
          status={status}
          checked={agreedToLegal}
          onCheckedChange={setAgreedToLegal}
          className='border-0 bg-transparent p-0'
        />

        <Button
          type='submit'
          className='h-9 w-full rounded-sm bg-blue-600 text-sm text-white hover:bg-blue-700'
          disabled={isLoading || (requiresLegalConsent && !agreedToLegal)}
        >
          {isLoading ? <Loader2 className='h-4 w-4 animate-spin' /> : null}
          {t('Complete registration')}
        </Button>

        <p className='text-center text-xs text-slate-700'>
          {t('Already have an account?')}{' '}
          <Link to='/sign-in' className='text-blue-600'>
            {t('Sign in')}
          </Link>
        </p>
      </form>
    </Form>
  )
}
