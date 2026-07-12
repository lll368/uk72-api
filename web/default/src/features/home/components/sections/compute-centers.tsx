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
import center1 from '@/assets/home/center/center1.png'
import center2 from '@/assets/home/center/center2.png'
import center3 from '@/assets/home/center/center3.png'
import center4 from '@/assets/home/center/center4.png'

export function ComputeCenters() {
  const { t } = useTranslation()

  const cards = [
    {
      key: 'nvidia',
      title: t('NVIDIA GPU'),
      desc: t(
        'Powered by superior parallel compute and the CUDA ecosystem, with integrated Tensor and ray-tracing cores, leading AI training and high-performance computing'
      ),
      image: center1,
    },
    {
      key: 'ascend',
      title: t('Huawei Ascend GPU'),
      desc: t(
        'Built on the in-house Da Vinci architecture with powerful 3D Cube matrix compute, fully self-controllable across the stack, suited for large-model training and inference'
      ),
      image: center2,
    },
    {
      key: 'green-power',
      title: t('China Green Power'),
      desc: t(
        'Backed by domestic clean energy supply with low-carbon, stable power delivery, fully safeguarding the long-term healthy growth of the compute industry'
      ),
      image: center3,
    },
    {
      key: 'global-gpu',
      title: t('Global GPU Direct Sourcing'),
      desc: t(
        'Direct manufacturer quota access, shorter supply chain, locked-in high-end compute, with assured supply and technical support to power AI compute infrastructure'
      ),
      image: center4,
    },
  ]

  return (
    <section className='border-border/40 relative z-10 border-t bg-[#F1F6FE] px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='mb-14 text-center md:mb-20'>
          <h2 className='mb-3 text-3xl font-bold tracking-tight md:text-4xl'>
            {t('China Computing Power Centers Onboarded')}
          </h2>
          <p className='text-muted-foreground mx-auto max-w-3xl text-sm md:text-base'>
            {t(
              'Bringing top-tier compute resources together to strengthen the digital industry foundation and empower regional economic intelligent transformation'
            )}
          </p>
        </AnimateInView>

        <div className='grid gap-5 sm:grid-cols-2 md:gap-6 lg:grid-cols-4'>
          {cards.map((card, i) => (
            <AnimateInView
              key={card.key}
              delay={i * 100}
              animation='fade-up'
              className='group border-border/60 bg-background relative aspect-[7/8] overflow-hidden rounded-2xl border shadow-sm transition-all duration-300 hover:-translate-y-1 hover:shadow-xl'
            >
              {/* Full-bleed image background */}
              <img
                src={card.image}
                alt={card.title}
                loading='lazy'
                draggable={false}
                className='absolute inset-0 h-full w-full object-cover transition-transform duration-500 group-hover:scale-105'
              />

              {/* Text panel overlaying the lower portion of the image */}
              <div className='bg-background absolute inset-x-0 bottom-0 p-5 md:p-6'>
                <h3 className='mb-2.5 text-lg font-bold tracking-tight'>
                  {card.title}
                </h3>
                <p className='text-muted-foreground line-clamp-3 text-sm leading-relaxed'>
                  {card.desc}
                </p>
              </div>
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
