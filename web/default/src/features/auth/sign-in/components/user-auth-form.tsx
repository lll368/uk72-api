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
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { Label } from '@/components/ui/label'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { PasswordInput } from '@/components/password-input'
import { Turnstile } from '@/components/turnstile'
import {
  login,
  loginByEmail,
  loginByPhone,
  sendAccountVerificationCode,
} from '@/features/auth/api'
import { LegalConsent } from '@/features/auth/components/legal-consent'
import {
  PHONE_VERIFICATION_COUNTDOWN,
  loginFormSchema,
} from '@/features/auth/constants'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import { useTurnstile } from '@/features/auth/hooks/use-turnstile'
import { normalizeAuthAccount } from '@/features/auth/lib/account'
import { getVerificationAccountValidationMessage } from '@/features/auth/lib/verification-code'
import type { AuthFormProps } from '@/features/auth/types'

const REMEMBERED_ACCOUNT_KEY = 'auth:remembered-account'

function getRememberedAccount() {
  if (typeof window === 'undefined') return ''
  return window.localStorage.getItem(REMEMBERED_ACCOUNT_KEY) ?? ''
}

export function resolveLoginRedirect(redirectTo?: string) {
  if (!redirectTo?.startsWith('/') || redirectTo.startsWith('//')) {
    return '/'
  }
  return redirectTo
}

export function UserAuthForm({
  className,
  redirectTo,
  ...props
}: AuthFormProps) {
  const { t } = useTranslation()
  const rememberedAccount = getRememberedAccount()
  const [isLoading, setIsLoading] = useState(false)
  const [agreedToLegal, setAgreedToLegal] = useState(false)
  const [rememberAccount, setRememberAccount] = useState(
    Boolean(rememberedAccount)
  )
  const [loginMode, setLoginMode] = useState<'password' | 'verification'>(
    'password'
  )
  const [isSendingVerificationCode, setIsSendingVerificationCode] =
    useState(false)

  const { status } = useStatus()
  const {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  } = useTurnstile()
  const { handleLoginSuccess, redirectTo2FA } = useAuthRedirect()
  const {
    secondsLeft: verificationCodeSecondsLeft,
    isActive: isVerificationCodeActive,
    start: startVerificationCodeCountdown,
  } = useCountdown({ initialSeconds: PHONE_VERIFICATION_COUNTDOWN })

  const hasUserAgreement = Boolean(status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(status?.privacy_policy_enabled)
  const requiresLegalConsent = hasUserAgreement || hasPrivacyPolicy
  const canRegister = status?.register_enabled !== false

  const form = useForm<z.infer<typeof loginFormSchema>>({
    resolver: zodResolver(loginFormSchema),
    defaultValues: {
      username: rememberedAccount,
      password: '',
      account: '',
      phoneNumber: '',
      verificationCode: '',
    },
  })

  const verificationAccountValue = form.watch('account') || ''

  useEffect(() => {
    setAgreedToLegal(!requiresLegalConsent)
  }, [requiresLegalConsent])

  async function onSubmit(data: z.infer<typeof loginFormSchema>) {
    if (requiresLegalConsent && !agreedToLegal) {
      toast.error(t('Please agree to the legal terms first'))
      return
    }
    if (!validateTurnstile()) return

    setIsLoading(true)
    try {
      const res =
        loginMode === 'verification'
          ? await submitVerificationLogin(data)
          : await submitPasswordLogin(data)

      if (res?.success) {
        if (res.data?.require_2fa) {
          redirectTo2FA()
          return
        }
        if (loginMode === 'password') {
          persistRememberedAccount(data.username || '')
        }
        await handleLoginSuccess(
          res.data as { id?: number } | null,
          resolveLoginRedirect(redirectTo)
        )
        toast.success(t('Welcome back!'))
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsLoading(false)
    }
  }

  async function submitPasswordLogin(data: z.infer<typeof loginFormSchema>) {
    if (!data.username) {
      toast.error(t('Please enter your email or phone number'))
      return null
    }
    if (!data.password) {
      toast.error(t('Please enter your password'))
      return null
    }
    return login({
      username: data.username,
      password: data.password,
      turnstile: turnstileToken,
    })
  }

  async function submitVerificationLogin(
    data: z.infer<typeof loginFormSchema>
  ) {
    const account = data.account?.trim() ?? ''
    if (!account) {
      toast.error(t('Please enter your email or phone number'))
      return null
    }
    if (!data.verificationCode) {
      toast.error(t('Please enter the verification code'))
      return null
    }
    const parsed = normalizeAuthAccount(account)
    if (!parsed) {
      toast.error(t('Please enter a valid email or phone number'))
      return null
    }
    if (parsed.type === 'email') {
      return loginByEmail({
        email: parsed.value,
        verification_code: data.verificationCode,
        turnstile: turnstileToken,
      })
    }
    return loginByPhone({
      phone_number: parsed.value,
      verification_code: data.verificationCode,
      turnstile: turnstileToken,
    })
  }

  function persistRememberedAccount(account: string) {
    if (typeof window === 'undefined') return
    if (rememberAccount && account.trim()) {
      window.localStorage.setItem(REMEMBERED_ACCOUNT_KEY, account.trim())
      return
    }
    window.localStorage.removeItem(REMEMBERED_ACCOUNT_KEY)
  }

  async function handleSendVerificationLoginCode() {
    const account = verificationAccountValue.trim()
    const validationMessage = getVerificationAccountValidationMessage(account)
    if (validationMessage) {
      toast.error(t(validationMessage))
      return
    }
    if (!validateTurnstile()) return

    setIsSendingVerificationCode(true)
    try {
      const res = await sendAccountVerificationCode(
        account,
        'login',
        turnstileToken
      )
      if (res?.success) {
        startVerificationCodeCountdown()
        toast.success(t('Verification code sent'))
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsSendingVerificationCode(false)
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-5', className)}
        {...props}
      >
        <Tabs
          value={loginMode}
          onValueChange={(value) => setLoginMode(value as typeof loginMode)}
          className='gap-5'
        >
          <TabsList
            variant='line'
            className='h-8 w-full justify-start gap-5 border-b border-slate-100 p-0'
          >
            <TabsTrigger
              value='password'
              className='h-8 flex-none rounded-none px-0 text-sm data-active:text-blue-600 group-data-[variant=line]/tabs-list:data-active:after:bg-blue-600'
            >
              {t('Account login')}
            </TabsTrigger>
            <TabsTrigger
              value='verification'
              className='h-8 flex-none rounded-none px-0 text-sm data-active:text-blue-600 group-data-[variant=line]/tabs-list:data-active:after:bg-blue-600'
            >
              {t('Verification code login')}
            </TabsTrigger>
          </TabsList>
        </Tabs>

        {loginMode === 'password' ? (
          <div className='grid gap-3'>
            <FormField
              control={form.control}
              name='username'
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
              name='password'
              render={({ field }) => (
                <FormItem>
                  <FormControl>
                    <PasswordInput
                      placeholder={t('Password')}
                      autoComplete='current-password'
                      className='[&_input]:h-9 [&_input]:rounded-sm [&_input]:border-slate-200 [&_input]:bg-white [&_input]:text-sm'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='flex items-center justify-between gap-3 text-xs'>
              <Label className='flex items-center gap-2 font-normal text-slate-600'>
                <Checkbox
                  checked={rememberAccount}
                  onCheckedChange={(checked) =>
                    setRememberAccount(checked === true)
                  }
                />
                {t('Remember me')}
              </Label>
              <Link to='/forgot-password' className='text-blue-600'>
                {t('Forgot password?')}
              </Link>
            </div>
          </div>
        ) : (
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
                            isSendingVerificationCode ||
                            isVerificationCodeActive ||
                            !verificationAccountValue
                          }
                          onClick={handleSendVerificationLoginCode}
                          className='text-blue-600 hover:text-blue-700'
                        >
                          {isVerificationCodeActive
                            ? t('Resend ({{seconds}}s)', {
                                seconds: verificationCodeSecondsLeft,
                              })
                            : isSendingVerificationCode
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
        )}

        {isTurnstileEnabled && (
          <Turnstile siteKey={turnstileSiteKey} onVerify={setTurnstileToken} />
        )}

        <LegalConsent
          status={status}
          checked={agreedToLegal}
          onCheckedChange={setAgreedToLegal}
          className='border-0 bg-transparent p-0 text-xs'
        />

        <Button
          type='submit'
          className='h-9 w-full rounded-sm bg-blue-600 text-sm text-white hover:bg-blue-700'
          disabled={isLoading || (requiresLegalConsent && !agreedToLegal)}
        >
          {isLoading ? <Loader2 className='h-4 w-4 animate-spin' /> : null}
          {t('Sign in')}
        </Button>

        {canRegister && (
          <p className='text-center text-xs text-slate-700'>
            {t("Don't have an account?")}{' '}
            <Link to='/sign-up' className='text-blue-600'>
              {t('Sign up now')}
            </Link>
          </p>
        )}
      </form>
    </Form>
  )
}
