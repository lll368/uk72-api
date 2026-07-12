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
import type { WithdrawalProfile, WithdrawalProfileInput } from '../types'

export type PiggyContractPanelStep = 'profile' | 'sign' | 'signed'
type PiggyContractRefreshAction = () => Promise<boolean | void>

export function resolvePiggyContractRefreshAction(
  step: PiggyContractPanelStep,
  onRefresh?: PiggyContractRefreshAction,
  onRefreshContractStatus?: PiggyContractRefreshAction,
  signingStarted = true
) {
  if (step === 'sign') {
    if (!signingStarted) {
      return undefined
    }
    return onRefreshContractStatus || onRefresh
  }
  return onRefresh || onRefreshContractStatus
}

export function hasPiggyContractSigningStarted(
  profile?: WithdrawalProfile | null,
  piggySignUrl?: string
) {
  return (
    (piggySignUrl?.trim() || '') !== '' ||
    (profile?.piggy_sign_url_digest?.trim() || '') !== ''
  )
}

export function createPiggyContractProfileForm(
  profile?: WithdrawalProfile | null
): WithdrawalProfileInput {
  return {
    account_type: 'bankcard',
    real_name: profile?.real_name || '',
    id_card_no: '',
    mobile: '',
    bank_card_no: '',
    bank_name: profile?.bank_name || '',
  }
}

export function discardPiggyContractProfileDraft(
  _draft: WithdrawalProfileInput,
  profile?: WithdrawalProfile | null
): WithdrawalProfileInput {
  return createPiggyContractProfileForm(profile)
}
