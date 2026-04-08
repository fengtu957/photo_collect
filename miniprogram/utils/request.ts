const BASE_URL = 'http://localhost:8000/api/v1';

export async function request<T = any>(
  url: string,
  options: Partial<WechatMiniprogram.RequestOption> = {}
): Promise<T> {
  const token = wx.getStorageSync('token');

  const res = await wx.request({
    url: BASE_URL + url,
    method: options.method || 'GET',
    data: options.data,
    header: {
      'Authorization': token ? `Bearer ${token}` : '',
      'Content-Type': 'application/json',
      ...options.header
    }
  });

  console.log('请求响应:', res);

  if (!res.data) {
    throw new Error('请求失败: 无响应数据');
  }

  return res.data as T;
}

export function showError(message: string) {
  wx.showToast({ title: message, icon: 'none', duration: 2000 });
}

export function showSuccess(message: string) {
  wx.showToast({ title: message, icon: 'success', duration: 2000 });
}

export function showLoading(title: string = '加载中...') {
  wx.showLoading({ title, mask: true });
}

export function hideLoading() {
  wx.hideLoading();
}
