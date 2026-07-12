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
import { AnimateInView } from '@/components/animate-in-view'
import { openAistudioLaunchDialog } from '@/components/aistudio-launch-dialog'
import workgroup1 from '@/assets/home/workgroup/workgroup1.png'
import workgroup2 from '@/assets/home/workgroup/workgroup2.png'
import workgroup3 from '@/assets/home/workgroup/workgroup3.png'

/** Custom external URL for the "Videos" card — overrides the default
 *  AI Studio target when the dialog is opened from that card. */
const VIDEOS_TARGET_URL = 'http://www.xingheyungu.cn/'

export function HowItWorks() {
  const { t } = useTranslation()

  const cards = [
    {
      key: 'copywriting',
      title: t('Copywriting'),
      desc: t(
        'Smart copywriting generation across formats, suited for various tasks, producing scripts efficiently and effortlessly meeting external promotion needs'
      ),
      image: workgroup1,
    },
    {
      key: 'images',
      title: t('Images'),
      desc: t(
        'One-click stunning visual creation; freely customize styles to produce premium promotional artwork'
      ),
      image: workgroup2,
    },
    {
      key: 'videos',
      title: t('Videos'),
      desc: t(
        'Rapidly edit and produce short videos with smooth, high-quality visuals; supports lifelike animation with multi-task concurrency'
      ),
      image: workgroup3,
    },
  ]

  return (
    <section className='border-border/40 relative z-10 border-t bg-[#F1F6FE] px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='mb-14 text-center md:mb-20'>
          <h2 className='mb-3 text-3xl font-bold tracking-tight md:text-4xl'>
            {t('OPC · AI Workstation')}
          </h2>
          <p className='text-muted-foreground text-sm md:text-base'>
            {t(
              'Copywriting, images, and video — one-stop creation with maximum efficiency'
            )}
          </p>
        </AnimateInView>

        <div className='grid gap-6 md:grid-cols-3 md:gap-8'>
          {cards.map((card, i) => (
            <AnimateInView
              key={card.key}
              delay={i * 120}
              animation='fade-up'
              className='group relative overflow-hidden rounded-2xl border border-white/40 shadow-sm transition-all duration-300 hover:-translate-y-1 hover:shadow-xl'
            >
              <div
                className='relative aspect-[397/480] w-full bg-cover bg-center'
                style={{ backgroundImage: `url(${card.image})` }}
              >
                {/* Soft top fade for better text contrast */}
                <div className='pointer-events-none absolute inset-x-0 top-0 h-2/5 bg-gradient-to-b from-white/35 to-transparent' />

                {/* Text overlay */}
                <div className='relative z-10 flex h-full flex-col px-6 pt-7 md:px-7 md:pt-8'>
                  <span className='border-border/40 mb-3 inline-flex w-fit items-center rounded-md border bg-white/70 px-2 py-0.5 text-[11px] font-medium tracking-wide text-violet-600 backdrop-blur-sm'>
                    {t('AI Apps')}
                  </span>
                  <h3 className='mb-3 text-2xl font-bold tracking-tight text-slate-900'>
                    {card.title}
                  </h3>
                  <p className='mb-5 max-w-[260px] text-sm leading-relaxed text-slate-700/85'>
                    {card.desc}
                  </p>
                  <button
                    type='button'
                    onClick={() =>
                      openAistudioLaunchDialog(
                        card.key === 'videos'
                          ? { targetUrl: VIDEOS_TARGET_URL }
                          : undefined
                      )
                    }
                    className='inline-flex w-fit items-center rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white shadow-sm transition-all hover:bg-slate-800 hover:shadow group-hover:translate-x-0.5'
                  >
                    {t('Try Now')}
                  </button>
                </div>
              </div>
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
