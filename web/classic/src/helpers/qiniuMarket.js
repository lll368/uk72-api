/*
Copyright (C) 2025 QuantumNous

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

const DETAIL_ORDER = ['input', 'output', 'cache', 'image', 'audio'];

export function isQiniuMarketModel(model) {
  return Boolean(
    model?.market_pricing ||
      model?.qiniu_market ||
      model?.price_source === 'qiniu_market',
  );
}

export function hasQiniuMarketPricing(model) {
  return formatQiniuMarketPriceItems(model).length > 0;
}

export function formatQiniuMarketPriceItems(model) {
  if (!isQiniuMarketModel(model)) return [];

  const rules = getMarketPricing(model)?.pricing_rules_v2 || [];
  const items = [];
  rules.forEach((rule, ruleIndex) => {
    const details = rule?.details_v2;
    if (!details) return;

    Object.entries(details)
      .sort(([left], [right]) => detailSortIndex(left) - detailSortIndex(right))
      .forEach(([key, detail]) => {
        const formatted = formatQiniuCNYPrice(detail);
        if (!formatted) return;
        items.push({
          key: `${ruleIndex}-${key}`,
          label: sanitizeSupplierBrandText(detail?.name) || key,
          formatted,
          unitLabel: formatQiniuUnitLabel(detail),
        });
      });
  });
  return items;
}

export function getQiniuMarketPriceDisplayItems(model, t = (key) => key) {
  const items = formatQiniuMarketPriceItems(model);
  if (items.length > 0) {
    return items.map((item) => ({
      key: item.key,
      label: item.label,
      value: item.formatted,
      suffix: item.unitLabel,
    }));
  }
  if (isQiniuMarketModel(model)) {
    return [
      {
        key: 'qiniu-market-unavailable',
        label: t('价格'),
        value: '—',
        suffix: '',
      },
    ];
  }
  return [];
}

export function getQiniuMarketRatioDisplayItems(model, t = (key) => key) {
  if (!isQiniuMarketModel(model)) return null;
  return [
    { key: 'model-ratio', label: t('模型倍率'), value: t('无') },
    { key: 'completion-ratio', label: t('补全倍率'), value: t('无') },
    { key: 'group-ratio', label: t('分组倍率'), value: t('无') },
  ];
}

export function qiniuMarketVisibleText(model) {
  const marketPricing = getMarketPricing(model);
  const parts = [
    model?.model_name,
    marketPricing?.name,
    marketPricing?.description,
    ...(marketPricing?.hot_tags || []),
    ...(marketPricing?.features || []),
    ...formatQiniuMarketPriceItems(model).flatMap((item) => [
      item.label,
      item.formatted,
      item.unitLabel,
    ]),
  ];
  return parts
    .map((part) => sanitizeSupplierBrandText(part))
    .filter(Boolean)
    .join(' ');
}

export function sanitizeSupplierBrandText(value) {
  const raw = value?.trim();
  if (!raw) return '';
  return raw
    .replace(/qiniu_market/gi, 'market')
    .replace(/qiniu/gi, 'official')
    .replace(/七牛/g, '官方')
    .replace(/官方官方/g, '官方')
    .replace(/official\s+official/gi, 'official')
    .replace(/\s+/g, ' ')
    .trim();
}

export function sanitizeSupplierBrandUrl(value) {
  const raw = value?.trim();
  if (!raw) return '';
  // 隐藏供应商品牌只针对用户可见文案；图片和链接属于资源地址，不能因为包含供应商域名而清空，否则会导致模型 logo 丢失。
  return raw;
}

export function sanitizePricingModelForDisplay(model, vendor) {
  const sanitized = {
    ...(model || {}),
    description: sanitizeSupplierBrandText(model?.description),
    tags: sanitizeSupplierBrandText(model?.tags),
    icon: sanitizeSupplierBrandUrl(model?.icon),
    vendor_name: sanitizeSupplierBrandText(model?.vendor_name),
    vendor_icon: sanitizeSupplierBrandUrl(model?.vendor_icon),
    vendor_description: sanitizeSupplierBrandText(model?.vendor_description),
  };

  if (vendor) {
    sanitized.vendor_name = sanitizeSupplierBrandText(vendor.name);
    sanitized.vendor_icon = sanitizeSupplierBrandUrl(vendor.icon);
    sanitized.vendor_description = sanitizeSupplierBrandText(
      vendor.description,
    );
  }

  return sanitized;
}

function getMarketPricing(model) {
  return model?.market_pricing || model?.qiniu_market;
}

function detailSortIndex(key) {
  const index = DETAIL_ORDER.indexOf(key);
  return index >= 0 ? index : DETAIL_ORDER.length;
}

function formatQiniuCNYPrice(detail) {
  if (
    !detail ||
    detail.unit_price === undefined ||
    detail.unit_price === null
  ) {
    return '';
  }
  const value = Number(detail.unit_price);
  if (!Number.isFinite(value)) return '';
  return `¥${stripQiniuTrailingZeros(value)}`;
}

function stripQiniuTrailingZeros(value) {
  if (value === 0) return '0';
  return value.toFixed(8).replace(/\.?0+$/, '');
}

function formatQiniuUnitLabel(detail) {
  const unitName = sanitizeSupplierBrandText(detail?.unit_name);
  const unitSize = Number(detail?.unit_size);
  if (unitName && Number.isFinite(unitSize) && unitSize > 1) {
    return `/ ${formatQiniuUnitSize(unitSize)} ${unitName}`;
  }
  if (unitName) {
    return `/ ${unitName}`;
  }
  return '';
}

function formatQiniuUnitSize(value) {
  if (value === 1000) return '1K';
  if (value === 1000000) return '1M';
  return String(value);
}
