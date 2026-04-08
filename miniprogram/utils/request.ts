const BASE_URL = 'http://192.168.31.154:8000/api/v1';

interface ApiResponse<T> {
  code: number;
  message: string;
  data?: T;
}

export function request<T = any>(
  url: string,
  options: Partial<WechatMiniprogram.RequestOption> = {}
): Promise<T> {
  const token = wx.getStorageSync('token');

  return new Promise((resolve, reject) => {
    wx.request({
      url: BASE_URL + url,
      method: options.method || 'GET',
      data: options.data,
      header: {
        'Authorization': token ? `Bearer ${token}` : '',
        'Content-Type': 'application/json',
        ...options.header
      },
      success: (res) => {
        console.log('完整响应:', res);
        console.log('响应数据:', res.data);

        if (!res.data) {
          reject(new Error('无响应数据'));
          return;
        }

        const apiRes = res.data as ApiResponse<T>;

        if (apiRes.code !== 0) {
          reject(new Error(apiRes.message || '请求失败'));
          return;
        }

        resolve(apiRes.data as T);
      },
      fail: (err) => {
        reject(new Error(err.errMsg || '网络请求失败'));
      }
    });
  });
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
