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
import { cn } from '@/lib/utils'
import { useSystemConfig } from '@/hooks/use-system-config'
import { PublicSourceCodeLink } from './public-source-code-link'

interface FooterProps {
  logo?: string
  name?: string
  copyright?: string
  className?: string
}

// Email addresses surfaced in the footer support column. Replace with your own as needed.
const BUSINESS_EMAIL = 'obtpzw7buk@gmail.com'
const SUPPORT_EMAIL = 'obtpzw7buk@gmail.com'

interface FooterNavLink {
  label: string
  to: string
  hash?: string
}

interface FooterMailLink {
  label: string
  href: string
}

const NAV_LINKS: FooterNavLink[] = [
  { label: 'Home', to: '/' },
  { label: 'API Service', to: '/pricing' },
  { label: 'AI Workstation', to: '/playground' },
  { label: 'Compute Cluster', to: '/', hash: 'stats' },
  { label: 'Top up Center', to: '/wallet' },
]

const SUPPORT_LINKS: FooterMailLink[] = [
  { label: 'Business Cooperation', href: `mailto:${BUSINESS_EMAIL}` },
  { label: 'Technical Support', href: `mailto:${SUPPORT_EMAIL}` },
]

const NEW_API_UPSTREAM_URL = 'https://github.com/QuantumNous/new-api'

export function Footer(props: FooterProps) {
  const { t } = useTranslation()
  const { systemName, logo: systemLogo } = useSystemConfig()

  const displayLogo = systemLogo || props.logo || '/logo.png'
  const displayName = systemName || props.name || 'New API'
  const currentYear = new Date().getFullYear()

  return (
    <footer
      className={cn(
        'relative z-10 bg-[#0F1729] text-slate-300',
        props.className
      )}
    >
      <div className='mx-auto max-w-6xl px-6 py-14 md:py-20'>
        <div className='flex flex-col gap-12 md:flex-row md:items-start md:justify-between'>
          {/* Brand column */}
          <div className='max-w-xs'>
            <Link to='/' className='group inline-flex items-center gap-2.5'>
              <img
                src={displayLogo}
                alt={displayName}
                className='h-7 w-auto rounded-md object-contain'
              />
              <span className='text-base font-semibold tracking-tight text-white'>
                {displayName}
              </span>
            </Link>
            <p className='mt-4 text-sm leading-relaxed text-slate-400'>
              {t(
                'One-stop AI API distribution and management system, supports multiple LLMs, fast and stable, empowers your AI applications.'
              )}
            </p>
          </div>

          {/* Link columns */}
          <div className='flex gap-12 md:gap-20'>
            <div>
              <h3 className='mb-4 text-sm font-semibold text-white'>
                {t('Navigation')}
              </h3>
              <ul className='space-y-3'>
                {NAV_LINKS.map((link) => (
                  <li key={link.label}>
                    <Link
                      to={link.to}
                      hash={link.hash}
                      className='text-sm text-slate-400 transition-colors duration-200 hover:text-white'
                    >
                      {t(link.label)}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
            <div>
              <h3 className='mb-4 text-sm font-semibold text-white'>
                {t('Support')}
              </h3>
              <ul className='space-y-3'>
                {SUPPORT_LINKS.map((link) => (
                  <li key={link.label}>
                    <a
                      href={link.href}
                      className='text-sm text-slate-400 transition-colors duration-200 hover:text-white'
                    >
                      {t(link.label)}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>

        {/* Bottom bar */}
        <div className='mt-12 border-t border-slate-700/40 pt-6'>
          <div className='flex flex-col gap-5 text-xs text-slate-500 lg:flex-row lg:items-end lg:justify-between'>
            <div className='max-w-2xl space-y-1 text-center lg:text-left'>
              <p>
                &copy; {currentYear} {displayName}.{' '}
                {props.copyright ?? t('All rights reserved.')}
              </p>
              <p>本项目基于 New API 开源项目改造，遵循 AGPL-3.0 协议。</p>
            </div>
            <div className='flex flex-wrap items-center justify-center gap-x-5 gap-y-2 lg:justify-end'>
              <PublicSourceCodeLink className='transition-colors hover:text-slate-300'>
                开源代码
              </PublicSourceCodeLink>
              <a
                href={NEW_API_UPSTREAM_URL}
                target='_blank'
                rel='noreferrer'
                className='transition-colors hover:text-slate-300'
              >
                New API 上游项目
              </a>
              <Link
                to='/user-agreement'
                className='transition-colors hover:text-slate-300'
              >
                {t('User Agreement')}
              </Link>
              <Link
                to='/privacy-policy'
                className='transition-colors hover:text-slate-300'
              >
                {t('Privacy Policy')}
              </Link>
            </div>
          </div>
        </div>
      </div>
    </footer>
  )
}
