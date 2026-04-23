import { request } from '../utils/request';
import { UserEntitlements } from '../types/entitlement';

export async function getUserEntitlements() {
  return request<UserEntitlements>('/user/entitlements', { method: 'GET' });
}
