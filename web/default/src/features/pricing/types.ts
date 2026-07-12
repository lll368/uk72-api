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
// ----------------------------------------------------------------------------
// Pricing Types
// ----------------------------------------------------------------------------

export type PricingVendor = {
  id: number
  name: string
  icon?: string
  description?: string
}

export type PricingModel = {
  id: number
  model_name: string
  description?: string
  icon?: string
  vendor_id?: number
  vendor_name?: string
  vendor_icon?: string
  vendor_description?: string
  quota_type: number
  model_ratio: number
  completion_ratio: number
  model_price?: number
  cache_ratio?: number | null
  create_cache_ratio?: number | null
  image_ratio?: number | null
  audio_ratio?: number | null
  audio_completion_ratio?: number | null
  enable_groups: string[]
  tags?: string
  supported_endpoint_types?: string[]
  key?: string
  group_ratio?: Record<string, number>
  /** Billing mode (e.g. "tiered_expr") used to flag dynamic pricing */
  billing_mode?: string
  /** Raw expression describing dynamic / tiered billing */
  billing_expr?: string
  /** Pricing version returned by backend, useful for cache busting */
  pricing_version?: string
  enabled?: boolean
  routable?: boolean
  price_source?: 'qiniu_market' | string
  price_source_label?: string
  market_pricing?: MarketPricingModel
  qiniu_market?: QiniuMarketModel
  /**
   * Optional model metadata fields. These are not yet returned by the backend
   * and are populated client-side from {@link inferModelMetadata}.
   * When the backend ships these fields, the inference layer becomes a
   * fallback rather than the source of truth.
   */
  context_length?: number
  max_output_tokens?: number
  knowledge_cutoff?: string
  release_date?: string
  parameter_count?: string
  input_modalities?: Modality[]
  output_modalities?: Modality[]
  capabilities?: ModelCapability[]
}

export type QiniuMarketModel = {
  id: string
  name?: string
  description?: string
  created_time?: string
  avatar?: string
  hot_tags?: string[] | null
  features?: string[] | null
  private?: boolean
  model_constraints?: QiniuMarketModelConstraints
  issuer?: QiniuMarketIssuer
  architecture?: QiniuMarketArchitecture
  pricing_rules_v2?: QiniuMarketPricingRuleV2[]
  rate_limit?: Record<string, QiniuMarketLimit>
  model_filing?: { filing_no?: string }
  supported_parameters?: string[]
  support_api_protocols?: string[]
  rank?: number
  retirement_at?: string
  release_at?: string
  suggested_model?: string
}

export type MarketPricingModel = {
  id?: string
  name?: string
  description?: string
  hot_tags?: string[] | null
  features?: string[] | null
  pricing_rules_v2?: QiniuMarketPricingRuleV2[]
}

export type QiniuMarketModelConstraints = {
  context_length?: number
  max_completion_tokens?: number
  max_tokens?: number
  max_default_completion_tokens?: number
  max_chain_of_thought_length?: number
}

export type QiniuMarketIssuer = {
  name?: string
  avatar?: string
  model_page?: string
}

export type QiniuMarketArchitecture = {
  input_modalities?: string[]
  output_modalities?: string[]
  schema_output?: QiniuMarketSupportedFeature
  function_calling?: QiniuMarketSupportedFeature
  reasoning?: QiniuMarketSupportedFeature
  content_cache?: QiniuMarketSupportedFeature
}

export type QiniuMarketSupportedFeature = {
  supported?: boolean
  description?: string
}

export type QiniuMarketPricingRuleV2 = {
  input_range?: number[]
  output_range?: number[]
  input_item_type?: string
  output_item_type?: string
  details?: Record<string, unknown>
  details_v2?: Record<string, QiniuMarketPricingDetail>
}

export type QiniuMarketPricingDetail = {
  unit_name?: string
  unit_size?: number
  unit_price?: number | null
  unit_price_usd?: number | null
  name?: string
}

export type QiniuMarketLimit = {
  name?: string
  quantity?: number
  unit_name?: string
  unit_time?: number
}

/** Input/output modalities supported by a model. */
export type Modality = 'text' | 'image' | 'audio' | 'video' | 'file'

/** Functional capabilities a model exposes. */
export type ModelCapability =
  | 'function_calling'
  | 'streaming'
  | 'vision'
  | 'json_mode'
  | 'structured_output'
  | 'reasoning'
  | 'tools'
  | 'system_prompt'
  | 'web_search'
  | 'code_interpreter'
  | 'caching'
  | 'embeddings'

export type PricingData = {
  success: boolean
  message?: string
  data: PricingModel[]
  vendors: PricingVendor[]
  group_ratio: Record<string, number>
  usable_group: Record<string, { desc: string; ratio: number }>
  supported_endpoint: Record<string, string>
  auto_groups: string[]
  market_pricing_sync?: {
    status?: string
    last_success_time?: number
    stale?: boolean
    from_cache?: boolean
    fallback_local?: boolean
  }
  qiniu_market_sync?: {
    status?: string
    last_success_time?: number
    stale?: boolean
    from_cache?: boolean
    fallback_local?: boolean
  }
}

export type TokenUnit = 'M' | 'K'
export type PriceType =
  | 'input'
  | 'output'
  | 'cache'
  | 'create_cache'
  | 'image'
  | 'audio_input'
  | 'audio_output'
export type QuotaType = 0 | 1 // 0: token-based, 1: per-request
