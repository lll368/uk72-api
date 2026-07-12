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
import type { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useCountdown } from '@/hooks/use-countdown'
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
  confirmPasswordReset,
  sendAccountVerificationCode,
  verifyPasswordResetCode,
} from '@/features/auth/api'
import {
  forgotPasswordFormSchema,
  PASSWORD_MAX_LENGTH,
  PASSWORD_MIN_LENGTH,
  PHONE_VERIFICATION_COUNTDOWN,
} from '@/features/auth/constants'
import { useTurnstile } from '@/features/auth/hooks/use-turnstile'
import { hasLetterAndNumber } from '@/features/auth/lib/account'
import { getVerificationAccountValidationMessage } from '@/features/auth/lib/verification-code'
import type { PasswordResetSession } from '@/features/auth/types'

export function ForgotPasswordForm({
  className,
  ...props
}: React.HTMLAttributes<HTMLFormElement>) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [step, setStep] = useState<'verify' | 'password'>('verify')
  const [isLoading, setIsLoading] = useState(false)
  const [isSendingCode, setIsSendingCode] = useState(false)
  const [resetSession, setResetSession] = useState<PasswordResetSession | null>(
    null
  )

  const {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  } = useTurnstile()
  const {
    secondsLeft,
    isActive,
    start: startCountdown,
  } = useCountdown({ initialSeconds: PHONE_VERIFICATION_COUNTDOWN })

  const form = useForm<z.infer<typeof forgotPasswordFormSchema>>({
    resolver: zodResolver(forgotPasswordFormSchema),
    defaultValues: {
      account: '',
      email: '',
      phoneNumber: '',
      verificationCode: '',
      password: '',
      confirmPassword: '',
    },
  })

  const accountValue = form.watch('account') || ''
  const codeValue = form.watch('verificationCode') || ''

  async function onSubmit(data: z.infer<typeof forgotPasswordFormSchema>) {
    if (step === 'verify') {
      await handleVerifyStep(data)
      return
    }
    await handlePasswordStep(data)
  }

  async function handleVerifyStep(
    data: z.infer<typeof forgotPasswordFormSchema>
  ) {
    const account = data.account?.trim() || ''
    const code = data.verificationCode?.trim() || ''
    if (!account) {
      toast.error(t('Please enter your email or phone number'))
      return
    }
    if (!code) {
      toast.error(t('Please enter the verification code'))
      return
    }
    if (!validateTurnstile()) return

    setIsLoading(true)
    try {
      const res = await verifyPasswordResetCode({
        account,
        verification_code: code,
        turnstile: turnstileToken,
      })
      if (res?.success && res.data) {
        setResetSession(res.data)
        setStep('password')
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsLoading(false)
    }
  }

  async function handlePasswordStep(
    data: z.infer<typeof forgotPasswordFormSchema>
  ) {
    const password = data.password || ''
    const confirmPassword = data.confirmPassword || ''

    if (!resetSession) {
      toast.error(t('Please complete verification first'))
      setStep('verify')
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

    setIsLoading(true)
    try {
      const res = await confirmPasswordReset({
        ...resetSession,
        password,
        turnstile: turnstileToken,
      })

      if (res?.success) {
        toast.success(t('Password updated successfully'))
        navigate({ to: '/sign-in', replace: true })
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsLoading(false)
    }
  }

  async function handleSendResetCode() {
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
        'reset_password',
        turnstileToken
      )
      if (res?.success) {
        startCountdown()
        toast.success(t('Verification code sent'))
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsSendingCode(false)
    }
  }

  function handleBack() {
    if (step === 'password') {
      setResetSession(null)
      setStep('verify')
      return
    }
    navigate({ to: '/sign-in', replace: true })
  }

  const codeSecondsLeft = secondsLeft
  const isCodeActive = isActive

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-5', className)}
        {...props}
      >
        <div className='border-b border-slate-100 pb-3'>
          <h2 className='text-sm font-semibold text-blue-600'>
            {step === 'verify' ? t('Forgot password') : t('Enter new password')}
          </h2>
        </div>

        {step === 'verify' ? (
          <div className='grid gap-3'>
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
                        placeholder={t('Please enter the verification code')}
                        autoComplete='one-time-code'
                        className='text-sm'
                        {...field}
                      />
                      <InputGroupAddon align='inline-end'>
                        <InputGroupButton
                          type='button'
                          disabled={
                            isLoading ||
                            isSendingCode ||
                            isCodeActive ||
                            !accountValue.trim()
                          }
                          onClick={handleSendResetCode}
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
          </div>
        ) : (
          <div className='grid gap-3'>
            <FormField
              control={form.control}
              name='password'
              render={({ field }) => (
                <FormItem>
                  <FormControl>
                    <PasswordInput
                      placeholder={t('New password')}
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

            <p className='text-xs leading-5 text-slate-600'>
              <span className='text-red-500'>*</span>{' '}
              {t(
                'Password must be 8-20 characters and contain letters and numbers'
              )}
            </p>
          </div>
        )}

        {isTurnstileEnabled && (
          <Turnstile siteKey={turnstileSiteKey} onVerify={setTurnstileToken} />
        )}

        <div className='grid grid-cols-2 gap-3 pt-1'>
          <Button
            type='button'
            variant='outline'
            className='h-9 rounded-sm border-blue-600 text-sm text-blue-600 hover:text-blue-700'
            onClick={handleBack}
            disabled={isLoading}
          >
            {t('Back')}
          </Button>
          <Button
            type='submit'
            className='h-9 rounded-sm bg-blue-600 text-sm text-white hover:bg-blue-700'
            disabled={
              isLoading ||
              (step === 'verify' && (!accountValue.trim() || !codeValue.trim()))
            }
          >
            {isLoading ? <Loader2 className='h-4 w-4 animate-spin' /> : null}
            {step === 'verify' ? t('Next') : t('Change password')}
          </Button>
        </div>
      </form>
    </Form>
  )
}
