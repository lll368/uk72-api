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
import { Skeleton } from '@/components/ui/skeleton'
import { VIEW_MODES, type ViewMode } from '../constants'

export interface LoadingSkeletonProps {
  viewMode?: ViewMode
}

export function LoadingSkeleton(props: LoadingSkeletonProps) {
  const viewMode = props.viewMode ?? VIEW_MODES.CARD

  return (
    <div className='space-y-8'>
      <div className='mx-auto max-w-4xl space-y-4 text-center'>
        <Skeleton className='mx-auto h-10 w-48' />
        <Skeleton className='mx-auto h-5 w-[min(100%,28rem)]' />
        <Skeleton className='mx-auto h-14 w-full max-w-3xl rounded-2xl' />
      </div>
      <FilterBarSkeleton />
      {viewMode === VIEW_MODES.TABLE ? (
        <TableContentSkeleton />
      ) : (
        <CardContentSkeleton />
      )}
    </div>
  )
}

function CardContentSkeleton() {
  return (
    <div className='grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3'>
      {Array.from({ length: 9 }).map((_, i) => (
        <div
          key={i}
          className='overflow-hidden rounded-[20px] shadow-[0_4px_12px_rgba(135,129,255,0.1)] bg-white dark:bg-card border-2 border-transparent'
        >
          <div className='p-5'>
            <div className='flex items-start gap-4'>
              <Skeleton className='size-16 shrink-0 rounded-xl' />
              <div className='min-w-0 flex-1 space-y-2'>
                <Skeleton className='h-5 w-40' />
                <div className='flex gap-1.5'>
                  <Skeleton className='h-5 w-16 rounded-full' />
                  <Skeleton className='h-5 w-16 rounded-full' />
                </div>
              </div>
            </div>
            <div className='mt-4 space-y-2'>
              <Skeleton className='h-4 w-full' />
              <Skeleton className='h-4 w-4/5' />
            </div>
            <div className='mt-3 flex gap-2'>
              <Skeleton className='h-7 w-16 rounded-md' />
              <Skeleton className='h-7 w-7 rounded-md' />
            </div>
          </div>
          <div className='border-t border-gray-100 dark:border-gray-800 px-5 py-3'>
            <Skeleton className='mx-auto h-4 w-3/4' />
          </div>
        </div>
      ))}
    </div>
  )
}

function FilterBarSkeleton() {
  return (
    <div className='flex items-center justify-between gap-3'>
      <Skeleton className='h-5 w-52' />
      <div className='flex items-center gap-2'>
        <Skeleton className='h-9 w-24 rounded-lg' />
        <Skeleton className='h-9 w-20 rounded-lg' />
        <Skeleton className='h-9 w-20 rounded-lg' />
      </div>
    </div>
  )
}

function TableContentSkeleton() {
  const columns = [
    { width: 200 },
    { width: 100 },
    { width: 100 },
    { width: 100 },
    { width: 80 },
    { width: 100 },
  ]

  return (
    <div className='space-y-4'>
      <div className='overflow-hidden rounded-lg border bg-card'>
        <div className='border-b px-4 py-3'>
          <div className='flex items-center gap-4'>
            {columns.map((col, i) => (
              <Skeleton
                key={i}
                className='h-4'
                style={{ width: `${col.width}px` }}
              />
            ))}
          </div>
        </div>
        {Array.from({ length: 10 }).map((_, i) => (
          <div
            key={i}
            className='flex items-center gap-4 border-b px-4 py-3 last:border-b-0'
          >
            {columns.map((col, j) => (
              <Skeleton
                key={j}
                className='h-5'
                style={{ width: `${col.width}px` }}
              />
            ))}
          </div>
        ))}
      </div>
      <div className='flex items-center justify-between'>
        <Skeleton className='h-5 w-32' />
        <div className='flex items-center gap-2'>
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className='size-8' />
          ))}
        </div>
      </div>
    </div>
  )
}
