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
import { useTranslation } from 'react-i18next'
import contactBack from '@/assets/home/contact-back.png'
import { AnimateInView } from '@/components/animate-in-view'
import { openContactMessageDialog } from '../contact-message-widget'

interface CTAProps {
  className?: string
  isAuthenticated?: boolean
}

// Business contact — replace with your own address
const CONTACT_EMAIL = 'obtpzw7buk@gmail.com'

export function CTA(props: CTAProps) {
  const { t } = useTranslation()

  if (props.isAuthenticated) {
    return null
  }

  return (
    <section className='relative z-10 overflow-hidden px-6 py-24 md:py-32'>
      {/* Background image (provided by design) */}
      <img
        aria-hidden
        src={contactBack}
        alt=''
        className='pointer-events-none absolute inset-0 -z-10 h-full w-full object-cover'
      />

      <AnimateInView
        className='mx-auto max-w-3xl text-center'
        animation='scale-in'
      >
        <h2 className='text-3xl font-bold tracking-tight text-slate-900 md:text-5xl'>
          {t('China Compute · Global Reach')}
        </h2>
        <p className='mt-5 text-sm text-slate-600 md:text-base'>
          {t('Business Cooperation Email: {{email}}', {
            email: CONTACT_EMAIL,
          })}
        </p>
        <div className='mt-8 flex items-center justify-center'>
          <button
            type='button'
            onClick={() => openContactMessageDialog()}
            className='group inline-flex items-center justify-center rounded-md bg-gradient-to-r from-blue-500 via-indigo-500 to-violet-500 px-8 py-2.5 text-sm font-medium text-white shadow-md shadow-indigo-500/30 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg hover:shadow-indigo-500/40 active:translate-y-0'
          >
            {t('Contact Us')}
          </button>
        </div>
      </AnimateInView>
    </section>
  )
}
