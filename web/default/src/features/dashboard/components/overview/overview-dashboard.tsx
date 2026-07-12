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
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import {
  CardStaggerContainer,
  CardStaggerItem,
} from '@/components/page-transition'
import { AnnouncementsPanel } from './announcements-panel'
import { ApiInfoPanel } from './api-info-panel'
import { FAQPanel } from './faq-panel'
import { PerformanceHealthPanel } from './performance-health-panel'
import { SummaryCards } from './summary-cards'
import { UptimePanel } from './uptime-panel'

// Soft pastel gradient palettes per panel, designed to match the dashboard
// reference design. Each gradient is light enough for both light and dark modes.
const PANEL_GRADIENT = {
  apiInfo:
    'bg-linear-to-br from-violet-100 via-fuchsia-50 to-pink-100 dark:from-violet-950/40 dark:via-fuchsia-950/30 dark:to-pink-950/40',
  announcements:
    'bg-linear-to-br from-sky-100 via-indigo-50 to-violet-100 dark:from-sky-950/40 dark:via-indigo-950/30 dark:to-violet-950/40',
  uptime:
    'bg-linear-to-br from-amber-50 via-rose-50 to-pink-100 dark:from-amber-950/30 dark:via-rose-950/30 dark:to-pink-950/40',
  faq:
    'bg-linear-to-br from-emerald-50 via-teal-50 to-cyan-100 dark:from-emerald-950/30 dark:via-teal-950/30 dark:to-cyan-950/40',
}

export function OverviewDashboard() {
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = Boolean(user?.role && user.role >= ROLE.ADMIN)

  return (
    <div className='flex flex-col gap-4'>
      <SummaryCards />

      <CardStaggerContainer className='grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_22rem]'>
        <div className='grid min-w-0 grid-cols-1 gap-4 lg:grid-cols-2'>
          {isAdmin && (
            <CardStaggerItem className='lg:col-span-2'>
              <PerformanceHealthPanel />
            </CardStaggerItem>
          )}
          <CardStaggerItem>
            <ApiInfoPanel className={PANEL_GRADIENT.apiInfo} />
          </CardStaggerItem>
          <CardStaggerItem>
            <AnnouncementsPanel className={PANEL_GRADIENT.announcements} />
          </CardStaggerItem>
          <CardStaggerItem>
            <FAQPanel className={PANEL_GRADIENT.faq} />
          </CardStaggerItem>
        </div>
        <CardStaggerItem>
          <UptimePanel className={PANEL_GRADIENT.uptime} />
        </CardStaggerItem>
      </CardStaggerContainer>
    </div>
  )
}
