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
import { AnimatePresence, motion } from 'motion/react'
import deepseekIcon from '@/assets/home/models/deepseek.png'
import doubaoIcon from '@/assets/home/models/doubao.png'
import ernieIcon from '@/assets/home/models/ernie.png'
import iflytekIcon from '@/assets/home/models/iflytek.png'
import jimengIcon from '@/assets/home/models/jimeng.png'
import klingIcon from '@/assets/home/models/kling.png'
import minimaxIcon from '@/assets/home/models/minimax.png'
import qwenIcon from '@/assets/home/models/qwen.png'
import volcengineIcon from '@/assets/home/models/volcengine.png'
import zhipuIcon from '@/assets/home/models/Zhipu.png'
import zhipuaiIcon from '@/assets/home/models/zhipuai.png'
import zhipuQingyanIcon from '@/assets/home/models/zhipuqingyan.png'

interface ModelItem {
  icon: string
  name: string
}

// Order follows the design reference (中国AI大模型算力集群).
const MODELS: ModelItem[] = [
  { icon: zhipuaiIcon, name: 'Zhipu AI' },
  { icon: doubaoIcon, name: '豆包' },
  { icon: jimengIcon, name: '即梦AI' },
  { icon: klingIcon, name: 'KLING AI' },
  { icon: deepseekIcon, name: 'DeepSeek' },
  { icon: qwenIcon, name: 'Qwen' },
  { icon: volcengineIcon, name: 'VolcEngine' },
  { icon: minimaxIcon, name: 'MiniMax' },
  { icon: ernieIcon, name: 'ERNIE Bot' },
  { icon: iflytekIcon, name: 'iFLYTEK Spark' },
  { icon: zhipuQingyanIcon, name: 'Zhipu Qingyan' },
  { icon: zhipuIcon, name: '智谱AI' },
]

interface HeroModelsCarouselProps {
  /** Switching interval in ms. Defaults to 2500ms. */
  intervalMs?: number
  className?: string
}

/**
 * Vertical carousel that cycles through promotional model logos and names.
 * Each item slides up out and the next slides in from below.
 */
export function HeroModelsCarousel({
  intervalMs = 2500,
  className,
}: HeroModelsCarouselProps) {
  const [index, setIndex] = useState(0)

  useEffect(() => {
    const id = setInterval(() => {
      setIndex((prev) => (prev + 1) % MODELS.length)
    }, intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])

  const current = MODELS[index]

  return (
    <div
      className={[
        'inline-flex h-20 items-center justify-center overflow-hidden rounded-md px-10 dark:border-zinc-500/60',
        className ?? '',
      ].join(' ')}
    >
      <AnimatePresence mode='wait'>
        <motion.div
          key={index}
          initial={{ y: 30, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={{ y: -30, opacity: 0 }}
          transition={{ duration: 0.45, ease: [0.4, 0, 0.2, 1] }}
          className='flex items-center gap-3'
        >
          <img
            src={current.icon}
            alt={current.name}
            className='h-10 w-auto select-none md:h-12'
            draggable={false}
          />
          <span className='text-3xl font-bold tracking-tight md:text-4xl'>
            {current.name}
          </span>
        </motion.div>
      </AnimatePresence>
    </div>
  )
}
