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
import z from 'zod'
import {
  DEFAULT_RANKING_PERIOD,
  RANKING_PERIODS,
  type RankingPeriod,
} from '../types'

export { DEFAULT_RANKING_PERIOD, RANKING_PERIODS } from '../types'

const RANKING_PERIOD_SET = new Set<RankingPeriod>(RANKING_PERIODS)

export const rankingsSearchSchema = z.object({
  period: z.enum(RANKING_PERIODS).optional().catch(undefined),
})

export function resolveRankingPeriod(
  period: string | null | undefined
): RankingPeriod {
  if (period && RANKING_PERIOD_SET.has(period as RankingPeriod)) {
    return period as RankingPeriod
  }
  return DEFAULT_RANKING_PERIOD
}
