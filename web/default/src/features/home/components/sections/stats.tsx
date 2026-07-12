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
import deepseekLogo from '@/assets/home/models/deepseek.png'
import doubaoLogo from '@/assets/home/models/doubao.png'
import ernieLogo from '@/assets/home/models/ernie.png'
import iflytekLogo from '@/assets/home/models/iflytek.png'
import jimengLogo from '@/assets/home/models/jimeng.png'
import klingLogo from '@/assets/home/models/kling.png'
import minimaxLogo from '@/assets/home/models/minimax.png'
import qwenLogo from '@/assets/home/models/qwen.png'
import volcengineLogo from '@/assets/home/models/volcengine.png'
import zhipuLogo from '@/assets/home/models/Zhipu.png'
import zhipuQingyanLogo from '@/assets/home/models/zhipuqingyan.png'
import zhipuaiLogo from '@/assets/home/models/zhipuai.png'

interface StatsProps {
  className?: string
}

interface ModelItem {
  name: string
  logo: string
}

const models: ModelItem[] = [
  { name: 'Zhipu AI', logo: zhipuLogo },
  { name: '豆包', logo: doubaoLogo },
  { name: '即梦AI', logo: jimengLogo },
  { name: 'KLING AI', logo: klingLogo },
  { name: 'DeepSeek', logo: deepseekLogo },
  { name: 'Qwen', logo: qwenLogo },
  { name: 'VolcEngine', logo: volcengineLogo },
  { name: 'MiniMax', logo: minimaxLogo },
  { name: 'ERNIE Bot', logo: ernieLogo },
  { name: 'iFLYTEK Spark', logo: iflytekLogo },
  { name: 'Zhipu Qingyan', logo: zhipuQingyanLogo },
  { name: '智谱AI', logo: zhipuaiLogo },
]

export function Stats(_props: StatsProps) {
  const { t } = useTranslation()

  return (
    <div
      id='stats'
      className='border-border/40 relative z-10 scroll-mt-20 border-y bg-[#F1F6FE]'
    >
      <div className='mx-auto max-w-6xl px-6 py-12 md:py-16'>
        {/* Section title with decorative dots/lines */}
        <div className='text-muted-foreground mb-10 flex items-center justify-center gap-3'>
          <span className='bg-border h-px w-10 sm:w-16' />
          <span className='border-border h-1.5 w-1.5 rounded-full border' />
          <span className='text-xs tracking-wide sm:text-sm'>
            {t('China AI Model Compute Cluster')}
          </span>
          <span className='border-border h-1.5 w-1.5 rounded-full border' />
          <span className='bg-border h-px w-10 sm:w-16' />
        </div>

        {/* Models grid: 3 cols on mobile, 4 on sm, 6 on md+ (12 items -> 2 rows on md+) */}
        <div className='grid grid-cols-3 gap-x-4 gap-y-8 sm:grid-cols-4 sm:gap-x-6 md:grid-cols-6'>
          {models.map((m) => (
            <div
              key={m.name}
              className='flex flex-col items-center gap-2.5 text-center'
            >
              <img
                src={m.logo}
                alt={m.name}
                loading='lazy'
                draggable={false}
                className='h-12 w-12 object-contain select-none'
              />
              <span className='text-foreground/80 text-xs leading-none'>
                {m.name}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
