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
import { Code as CodeIcon, Server } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'
import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'

interface FeaturesProps {
  className?: string
}

const SAMPLE_CODE = `export BASE_URL="https://www.uk72.cn"
export API_KEY="sk-xxx"

# Compatible with all standard SDKs
client = OpenAI(base_url=BASE_URL,
                api_key=API_KEY)
`

const ENDPOINT_URL = 'https://www.uk72.cn'

export function Features(_props: FeaturesProps) {
  const { t } = useTranslation()

   return (
    <section className='relative z-10 bg-[#F1F6FE] px-4 py-16 sm:px-6 md:py-32'>
      <div className='mx-auto max-w-6xl'>
        {/* Section title */}
        <AnimateInView className='mb-10 text-center md:mb-20'>
          <h2 className='text-2xl leading-tight font-bold tracking-tight sm:text-3xl md:text-4xl'>
            {t('Full-performance API, one-click direct connect')}
          </h2>
        </AnimateInView>

        <div className='grid gap-5 sm:gap-6 md:grid-cols-2 md:gap-8'>
          {/* Left: Quick Integration card */}
          <AnimateInView animation='scale-in' className='min-w-0'>
            <div className='border-border/60 bg-background relative w-full min-w-0 overflow-hidden rounded-2xl border p-5 shadow-sm sm:p-6 md:p-8'>
              {/* Top accent gradient line */}
              <div className='absolute inset-x-5 top-0 h-[3px] rounded-b-full bg-gradient-to-r from-sky-400 via-violet-400 to-fuchsia-400 sm:inset-x-6' />

              <div className='text-muted-foreground mb-4 flex items-center gap-1.5 text-xs font-semibold tracking-widest uppercase sm:mb-5'>
                <CodeIcon className='size-3.5' strokeWidth={2.5} />
                {t('Quick Integration')}
              </div>
              <h3 className='mb-4 text-lg font-bold tracking-tight sm:mb-5 sm:text-xl md:text-2xl'>
                {t('One line of code, globally compliant access')}
              </h3>

              <CodeBlock
                code={SAMPLE_CODE}
                language='bash'
                className='!overflow-x-auto bg-zinc-950 text-zinc-100 [&>div>div>pre]:!bg-zinc-950 [&>div>div>pre]:!pr-12 [&_code]:text-xs sm:[&_code]:text-[13px]'
              >
                <CodeBlockCopyButton className='text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100' />
              </CodeBlock>
            </div>
          </AnimateInView>

          {/* Right: Enterprise Endpoint card */}
          <AnimateInView animation='scale-in' delay={100} className='min-w-0'>
            <div className='border-border/60 bg-background relative w-full min-w-0 overflow-hidden rounded-2xl border p-5 shadow-sm sm:p-6 md:p-8'>
              <div className='text-muted-foreground mb-4 flex items-center gap-1.5 text-xs font-semibold tracking-widest uppercase sm:mb-5'>
                <Server
                  className='size-3.5 text-emerald-500'
                  strokeWidth={2.5}
                />
                {t('Enterprise Endpoint')}
              </div>
              <h3 className='mb-4 text-lg font-bold tracking-tight sm:mb-5 sm:text-xl md:text-2xl'>
                {t('Elastic high-availability endpoint')}
              </h3>

              <p className='text-muted-foreground mb-5 text-sm leading-relaxed sm:mb-6'>
                {t(
                  'We recommend using the following elastic endpoint in production; it will automatically route to the best region globally.'
                )}
              </p>

              {/* Endpoint URL bar */}
              <div className='border-border/60 bg-muted/40 mb-5 flex items-center gap-2 rounded-lg border px-3 py-2.5 sm:mb-6 sm:gap-3 sm:px-4 sm:py-3'>
                <span className='text-foreground/90 min-w-0 flex-1 truncate text-sm'>
                  {ENDPOINT_URL}
                </span>
                <button
                  type='button'
                  onClick={() => {
                    if (navigator?.clipboard) {
                      navigator.clipboard.writeText(ENDPOINT_URL)
                    }
                  }}
                  className='hover:bg-background flex size-7 shrink-0 items-center justify-center rounded-md transition-colors'
                  aria-label={t('Copy')}
                >
                  <svg
                    viewBox='0 0 24 24'
                    fill='none'
                    stroke='currentColor'
                    strokeWidth={2}
                    strokeLinecap='round'
                    strokeLinejoin='round'
                    className='text-foreground/70 size-3.5'
                  >
                    <rect x='9' y='9' width='13' height='13' rx='2' ry='2' />
                    <path d='M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1' />
                  </svg>
                </button>
              </div>

              {/* Multi-region indicator */}
              <div className='flex items-center gap-2'>
                <div className='flex -space-x-1.5'>
                  <span className='border-background size-4 rounded-full border-2 bg-indigo-500' />
                  <span className='border-background size-4 rounded-full border-2 bg-rose-400' />
                  <span className='border-background size-4 rounded-full border-2 bg-amber-400' />
                </div>
                <span className='text-muted-foreground text-xs'>
                  {t('Multi-region redundancy ready')}
                </span>
              </div>
            </div>
          </AnimateInView>
        </div>
      </div>
    </section>
  )
}
