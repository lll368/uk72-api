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
import { CheckCircle2, MonitorUp } from 'lucide-react'

export function PiggyContractSuccess() {
  return (
    <main className='from-background via-background to-emerald-50/70 dark:to-emerald-950/20 relative flex min-h-svh items-center justify-center overflow-hidden bg-gradient-to-br px-4 py-16'>
      <div className='pointer-events-none absolute -top-28 -left-28 size-72 rounded-full bg-emerald-300/20 blur-3xl' />
      <div className='pointer-events-none absolute right-0 -bottom-32 size-96 rounded-full bg-cyan-300/20 blur-3xl' />

      <section className='border-border/70 bg-card/90 relative z-10 w-full max-w-2xl rounded-[2rem] border p-8 text-center shadow-2xl shadow-emerald-950/10 backdrop-blur md:p-12'>
        <div className='mx-auto flex size-20 items-center justify-center rounded-full bg-emerald-500 text-white shadow-lg shadow-emerald-500/25'>
          <CheckCircle2 className='size-10' aria-hidden='true' />
        </div>

        <div className='mt-8 space-y-4'>
          <p className='text-sm font-medium tracking-[0.28em] text-emerald-600 uppercase dark:text-emerald-400'>
            Piggy Contract Signed
          </p>
          <h1 className='text-foreground text-3xl leading-tight font-semibold md:text-4xl'>
            电子合同签约成功，请在电脑网页上发起提现操作
          </h1>
          <p className='text-muted-foreground mx-auto max-w-xl text-base leading-7'>
            签约状态将以小猪回调结果为准。如电脑网页仍未解锁提现，请返回电脑端刷新签约状态后再操作。
          </p>
        </div>

        <div className='bg-muted/60 text-muted-foreground mt-8 inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm'>
          <MonitorUp className='size-4' aria-hidden='true' />
          请回到电脑网页继续提现流程
        </div>
      </section>
    </main>
  )
}
