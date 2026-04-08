import { request } from '../utils/request';

export function getUploadToken() {
  return request<{ token: string }>('/upload/token', { method: 'GET' });
}
