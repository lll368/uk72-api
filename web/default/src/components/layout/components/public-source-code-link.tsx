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
import { cn } from '@/lib/utils'

// AGPL 网络服务场景下需要给用户可见的源码入口，统一放在这里避免各布局写散。
export const PUBLIC_SOURCE_CODE_URL = 'https://github.com/lll368/uk72-api'

type PublicSourceCodeLinkProps = Omit<
  React.AnchorHTMLAttributes<HTMLAnchorElement>,
  'href' | 'rel' | 'target'
>

export function PublicSourceCodeLink({
  className,
  children,
  ...props
}: PublicSourceCodeLinkProps) {
  return (
    <a
      {...props}
      href={PUBLIC_SOURCE_CODE_URL}
      target='_blank'
      rel='noopener noreferrer'
      className={cn(className)}
    >
      {children ?? 'Source Code'}
    </a>
  )
}
