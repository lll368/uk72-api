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

/**
 * Common usage-log row returned by the logs API.
 * 表格列、详情弹窗和格式化工具共享这份行类型，避免各处重复声明字段。
 */
export interface UsageLog {
  id: number
  user_id: number
  created_at: number
  type: number
  content: string
  username: string
  token_id: number
  token_name: string
  model_name: string
  quota: number
  prompt_tokens: number
  completion_tokens: number
  use_time: number
  is_stream: boolean
  channel: number
  channel_name: string
  group: string
  ip: string
  other: string
  request_id: string
  upstream_request_id: string
}
