import { request } from '../utils/request';

export async function login() {
  const { code } = await wx.login();
  const res = await request<{ token: string; openid: string }>('/auth/login', {
    method: 'POST',
    data: { code }
  });
  wx.setStorageSync('token', res.token);
  wx.setStorageSync('openid', res.openid);
  return res;
}

export function checkLogin() {
  return !!wx.getStorageSync('token');
}
