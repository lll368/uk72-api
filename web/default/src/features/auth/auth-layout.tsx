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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Skeleton } from '@/components/ui/skeleton'
import { LanguageSwitcher } from '@/components/language-switcher'

type AuthLayoutProps = {
  children: React.ReactNode
}

function AuthDynamicBackground() {
  return (
    <div className='auth-dynamic-background' aria-hidden='true'>
      <div className='auth-dynamic-background__base' />
      <div className='auth-dynamic-background__mesh' />
      <div className='auth-dynamic-background__flow' />
      <div className='auth-dynamic-background__sheen' />
    </div>
  )
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()
  const { status } = useStatus()
  const currentYear = new Date().getFullYear()
  const hasUserAgreement = Boolean(status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(status?.privacy_policy_enabled)

  return (
    <div className='auth-shell relative min-h-svh overflow-hidden text-slate-900'>
      <AuthDynamicBackground />

      <div className='absolute top-4 right-5 z-20 sm:top-6 sm:right-8'>
        <LanguageSwitcher />
      </div>

      <main className='relative z-10 flex min-h-svh flex-col items-center px-5 py-10 sm:px-8'>
        <div className='flex w-full flex-1 flex-col items-center justify-center gap-5 pt-8 pb-10 sm:gap-6 sm:pt-10'>
          <Link
            to='/'
            className='flex flex-col items-center gap-2 text-center transition-opacity hover:opacity-85'
          >
            <div className='flex items-center justify-center gap-2'>
              <div className='relative h-9 w-9'>
                {loading ? (
                  <Skeleton className='absolute inset-0 rounded-full bg-white/70' />
                ) : (
                  <img
                    src={logo}
                    alt={t('Logo')}
                    className='h-9 w-9 rounded-full object-cover shadow-sm'
                  />
                )}
              </div>
              {loading ? (
                <Skeleton className='h-7 w-28 bg-white/70' />
              ) : (
                <h1 className='text-2xl leading-none font-semibold tracking-normal text-slate-900'>
                  {systemName}
                </h1>
              )}
            </div>
            <p className='text-xs leading-5 text-slate-600'>
              {t('Sign in to {{name}} and start your AI exploration journey', {
                name: systemName,
              })}
            </p>
          </Link>

          <section className='auth-card w-full max-w-[360px] rounded-md px-7 py-7 sm:px-8'>
            {children}
          </section>
        </div>

        <footer className='relative z-10 pb-4 text-center text-[11px] leading-5 text-slate-600'>
          <span>
            {systemName} © {currentYear}
          </span>
          {(hasUserAgreement || hasPrivacyPolicy) && (
            <span className='ml-1 inline-flex items-center gap-1'>
              {hasUserAgreement && (
                <a
                  href='/user-agreement'
                  target='_blank'
                  rel='noopener noreferrer'
                  className='hover:text-primary'
                >
                  {t('User Agreement')}
                </a>
              )}
              {hasUserAgreement && hasPrivacyPolicy && <span>/</span>}
              {hasPrivacyPolicy && (
                <a
                  href='/privacy-policy'
                  target='_blank'
                  rel='noopener noreferrer'
                  className='hover:text-primary'
                >
                  {t('Privacy Policy')}
                </a>
              )}
            </span>
          )}
        </footer>
      </main>
    </div>
  )
}
