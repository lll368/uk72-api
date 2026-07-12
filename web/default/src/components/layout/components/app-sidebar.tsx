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
import { useMemo } from 'react'
import { useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { useLayout } from '@/context/layout-provider'
import { useSidebarConfig } from '@/hooks/use-sidebar-config'
import { useSidebarData } from '@/hooks/use-sidebar-data'
import { Sidebar, SidebarContent, SidebarRail } from '@/components/ui/sidebar'
import { getNavGroupsForPath } from '../lib/workspace-registry'
import { NavGroup } from './nav-group'

/**
 * Application sidebar component
 * Fetches corresponding navigation menu from workspace registry based on current path
 * Dynamically filters navigation items based on backend SidebarModulesAdmin configuration
 *
 * Automatically matches workspace configuration for current path through workspace registry system
 * Adding new workspaces only requires registration in workspace-registry.ts
 */
export function AppSidebar() {
  const { t } = useTranslation()
  const { collapsible, variant } = useLayout()
  const { pathname } = useLocation()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const sidebarData = useSidebarData()

  // Get navigation group configuration corresponding to current path from workspace registry
  const allNavGroups = getNavGroupsForPath(pathname, t) || sidebarData.navGroups

  // Filter sidebar navigation items based on backend configuration
  const configFilteredNavGroups = useSidebarConfig(allNavGroups)

  // Filter navigation groups based on user role
  // Non-Admin users cannot see Admin navigation group
  const currentNavGroups = useMemo(() => {
    const isAdmin = userRole && userRole >= ROLE.ADMIN
    return configFilteredNavGroups.filter((group) => {
      if (group.id === 'admin') {
        return isAdmin
      }
      return true
    })
  }, [configFilteredNavGroups, userRole])

  // Inline CSS variable overrides so the gradient sidebar uses light text
  // and a translucent accent for the active item. This is wrapped around
  // <Sidebar /> so the variables cascade to all sidebar descendants
  // (including the outer wrapper that consumes `text-sidebar-foreground`).
  const sidebarVarOverrides = {
    '--sidebar-foreground': 'oklch(1 0 0)',
    '--sidebar-accent': 'oklch(1 0 0 / 0.18)',
    '--sidebar-accent-foreground': 'oklch(1 0 0)',
    '--sidebar-border': 'oklch(1 0 0 / 0.16)',
    '--sidebar-ring': 'oklch(1 0 0 / 0.5)',
  } as React.CSSProperties

  return (
    <div style={sidebarVarOverrides} className='contents'>
      <Sidebar
        collapsible={collapsible}
        variant={variant}
        className={
          // Apply a vertical gradient to the inner sidebar surface.
          // bg-linear-to-b sets background-image which visually overrides
          // the underlying bg-sidebar background-color.
          [
            '[&_[data-sidebar=sidebar]]:bg-linear-to-b',
            '[&_[data-sidebar=sidebar]]:from-violet-600',
            '[&_[data-sidebar=sidebar]]:via-indigo-600',
            '[&_[data-sidebar=sidebar]]:to-blue-600',
            'dark:[&_[data-sidebar=sidebar]]:from-violet-950',
            'dark:[&_[data-sidebar=sidebar]]:via-indigo-950',
            'dark:[&_[data-sidebar=sidebar]]:to-blue-950',
          ].join(' ')
        }
      >
        <SidebarContent className='py-2'>
          {currentNavGroups.map((props) => {
            const key = props.id || props.title
            return <NavGroup key={key} {...props} />
          })}
        </SidebarContent>
        <SidebarRail />
      </Sidebar>
    </div>
  )
}
