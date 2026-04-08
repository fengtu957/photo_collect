const BASE_URL = 'http://localhost:8000/api/v1';

export async function request<T = any>(
  url: string,
  options: Partial<WechatMiniprogram.RequestOption> = {}
): Promise<T> {
  const token = wx.getStorageSync('token');

  try {
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

    if (res.statusCode !== 200) {
      throw new Error('请求失败');
    }

    return res.data as T;
  } catch (err: any) {
    console.error(`请求失败:`, err);
    throw err;
  }
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
