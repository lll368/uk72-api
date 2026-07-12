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

export type ContactMessageStatus = 'pending' | 'contacted' | 'unreachable'

export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

export interface PageResponse<T> {
  page?: number
  page_size?: number
  total: number
  items: T[]
}

export interface ContactMessage {
  id: number
  name: string
  phone: string
  message: string
  status: ContactMessageStatus
  remark: string
  processed_at: number
  processed_by: number
  client_ip: string
  created_at: number
  updated_at: number
}

export interface UpdateContactMessageRequest {
  status: ContactMessageStatus
  remark: string
}
