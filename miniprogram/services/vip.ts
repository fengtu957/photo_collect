import { request } from '../utils/request';
import { UserEntitlements } from '../types/vip';

export async function getUserEntitlements() {
  return request<UserEntitlements>('/user/entitlements', { method: 'GET' });
}

export async function redeemVIPCode(code: string) {
  return request<UserEntitlements>('/vip/redeem', {
    method: 'POST',
    data: { code }
  });
}
