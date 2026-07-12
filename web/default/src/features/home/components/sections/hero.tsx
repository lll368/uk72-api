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
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import backImage from '@/assets/home/back-image.png'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { HeroModelsCarousel } from './hero-models-carousel'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
}

export function Hero({ className }: HeroProps) {
  const { t } = useTranslation()

  // Authenticated users see the China Compute Power Network promotional layout.
  return (
    <section
      className={cn(
        'relative z-10 flex flex-col items-center overflow-hidden px-6 pt-28 pb-24 md:pt-36 md:pb-32',
        className
      )}
    >
      {/* Background image (provided by design) */}
      <img
        aria-hidden
        src={backImage}
        alt=''
        className='pointer-events-none absolute inset-0 -z-10 h-full w-full object-cover'
      />

      <div className='flex w-full max-w-4xl flex-col items-center text-center'>
        <h1
          className='landing-animate-fade-up text-[clamp(2.5rem,7.5vw,5rem)] leading-[1.05] font-extrabold tracking-tight'
          style={{ animationDelay: '0ms' }}
        >
          <span className='bg-gradient-to-r from-pink-400 via-fuchsia-500 to-blue-600 bg-clip-text text-transparent'>
            {t('China Compute Power Network')}
          </span>
        </h1>

        <p
          className='landing-animate-fade-up mt-6 text-xl font-medium opacity-0 md:text-2xl'
          style={{ animationDelay: '80ms' }}
        >
          {t('China Compute, Globally Connected')}
        </p>

        <div
          className='landing-animate-fade-up mt-8 opacity-0'
          style={{ animationDelay: '160ms' }}
        >
          <HeroModelsCarousel />
        </div>

        <p
          className='landing-animate-fade-up mt-5 text-2xl font-semibold opacity-0 md:text-3xl'
          style={{ animationDelay: '220ms' }}
        >
          {t('Site-wide 15% off')}
          <span className='ml-1 align-baseline text-sm font-normal md:text-base'>
            {t('and up')}
          </span>
        </p>

        <p
          className='landing-animate-fade-up mt-3 text-xl opacity-0 md:text-2xl'
          style={{ animationDelay: '280ms' }}
        >
          <span className='text-muted-foreground'>Seedance 2.0</span>{' '}
          <span className='font-semibold text-blue-600'>
            {t('Limited time 75% off')}
          </span>
        </p>

        <div
          className='landing-animate-fade-up mt-10 opacity-0'
          style={{ animationDelay: '360ms' }}
        >
          <Button
            className='group h-12 rounded-md border-0 px-8 text-base font-semibold text-white shadow-lg shadow-indigo-500/30 hover:opacity-95'
            style={{
              background:
                'linear-gradient(135deg, #5b5bf0 0%, #7c3aed 100%)',
            }}
            render={<Link to='/wallet' />}
          >
            {t('Top up Now')}
            <ArrowRight className='ml-1 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
          </Button>
        </div>
      </div>
    </section>
  )
}
