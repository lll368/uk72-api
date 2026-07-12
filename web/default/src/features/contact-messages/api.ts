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
import { api } from '@/lib/api'
import type {
  ApiResponse,
  ContactMessage,
  ContactMessageStatus,
  PageResponse,
  UpdateContactMessageRequest,
} from './types'

type QueryValue = string | number | undefined | null

function buildQuery(params: Record<string, QueryValue>) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    query.set(key, String(value))
  }
  const text = query.toString()
  return text ? `?${text}` : ''
}

export async function getContactMessages(params: {
  page: number
  pageSize: number
  status?: ContactMessageStatus | ''
}): Promise<ApiResponse<PageResponse<ContactMessage>>> {
  const res = await api.get(
    `/api/contact/admin/messages${buildQuery({
      p: params.page,
      page_size: params.pageSize,
      status: params.status,
    })}`
  )
  return res.data
}

export async function updateContactMessage(
  id: number,
  request: UpdateContactMessageRequest
): Promise<ApiResponse<ContactMessage>> {
  const res = await api.put(`/api/contact/admin/messages/${id}`, request)
  return res.data
}

export async function deleteContactMessage(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/contact/admin/messages/${id}`)
  return res.data
}
