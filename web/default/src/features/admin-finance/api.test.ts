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
import { describe, expect, test } from 'bun:test'
import {
  buildAdminTopUpRecordsPath,
  buildAdminVipActivationRecordsPath,
  buildQiniuKeysPath,
  buildQiniuBillingSummaryPath,
} from './api'
import type {
  AdminTopUpRecord,
  AdminTopUpRecordFilters,
  AdminVipActivationRecordFilters,
  VipActivationRecord,
} from './types'

describe('admin finance recharge records API', () => {
  test('builds ordinary top-up list query with all server-side filters', () => {
    const filters: AdminTopUpRecordFilters = {
      userId: '1001',
      email: 'current@example.com',
      phoneNumber: '13800138000',
      tradeNo: 'TOPUP-001',
      status: 'success',
      paymentProvider: 'stripe',
      paymentMethod: 'stripe',
      createdFrom: 1710000000,
      createdTo: 1710003600,
      completedFrom: 1710000300,
      completedTo: 1710003900,
    }

    expect(buildAdminTopUpRecordsPath(2, 20, filters)).toBe(
      '/api/user/topup?p=2&page_size=20&user_id=1001&email=current%40example.com&phone_number=13800138000&trade_no=TOPUP-001&status=success&payment_provider=stripe&payment_method=stripe&created_from=1710000000&created_to=1710003600&completed_from=1710000300&completed_to=1710003900'
    )
  })

  test('builds VVIP activation list query with activation time filters', () => {
    const filters: AdminVipActivationRecordFilters = {
      userId: '1002',
      email: 'partner@example.com',
      phoneNumber: '13900139000',
      tradeNo: 'VVIP-001',
      status: 'pending',
      paymentProvider: 'wechat',
      paymentMethod: 'wechat_direct',
      createdFrom: 1710100000,
      createdTo: 1710103600,
      activatedFrom: 1710100300,
      activatedTo: 1710103900,
    }

    expect(buildAdminVipActivationRecordsPath(1, 20, filters)).toBe(
      '/api/vip/admin/records?p=1&page_size=20&user_id=1002&email=partner%40example.com&phone_number=13900139000&trade_no=VVIP-001&status=pending&payment_provider=wechat&payment_method=wechat_direct&created_from=1710100000&created_to=1710103600&activated_from=1710100300&activated_to=1710103900'
    )
  })

  test('record types include current user email and phone number fields', () => {
    const topUpRecord = {
      id: 1,
      user_id: 1001,
      username: 'topup_user',
      display_name: 'Topup User',
      email: 'current@example.com',
      phone_number: '13800138000',
      amount: 100,
      money: 100,
      recharge_amount: 100,
      paid_amount: 100,
      discount: 1,
      trade_no: 'TOPUP-001',
      payment_provider: 'stripe',
      payment_method: 'stripe',
      create_time: 1710000000,
      complete_time: 1710000300,
      reversed_at: 0,
      status: 'success',
    } satisfies AdminTopUpRecord
    const vipRecord = {
      id: 2,
      user_id: 1002,
      username: 'vvip_user',
      display_name: 'VVIP User',
      email: 'partner@example.com',
      phone_number: '13900139000',
      trade_no: 'VVIP-001',
      activation_amount: 1680,
      paid_amount: 1680,
      discount: 1,
      payment_provider: 'wechat',
      payment_method: 'wechat_direct',
      status: 'success',
      activated_at: 1710100300,
      created_at: 1710100000,
      updated_at: 1710100400,
    } satisfies VipActivationRecord

    expect(topUpRecord.email).toBe('current@example.com')
    expect(topUpRecord.phone_number).toBe('13800138000')
    expect(vipRecord.email).toBe('partner@example.com')
    expect(vipRecord.phone_number).toBe('13900139000')
  })

  test('builds Qiniu billing summary query path for alert dashboard', () => {
    expect(buildQiniuBillingSummaryPath()).toBe(
      '/api/payment/admin/qiniu-billing-summary'
    )
  })

  test('builds Qiniu keys query with child account filter', () => {
    expect(
      buildQiniuKeysPath({
        page: 3,
        pageSize: 20,
        userId: '1001',
        tokenId: '2002',
        childAccountId: '3003',
        status: '1',
        keyFragment: 'abcd',
        includeDeleted: true,
      })
    ).toBe(
      '/api/payment/admin/qiniu-keys?p=3&page_size=20&user_id=1001&token_id=2002&qiniu_child_account_id=3003&status=1&qiniu_key=abcd&include_deleted=true'
    )
  })
})
