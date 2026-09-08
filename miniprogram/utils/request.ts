export const BASE_URL = 'https://photo-collect.starpix.cn/api/v1';

interface ApiResponse<T> {
  code: number;
  message: string;
  data?: T;
}

interface RequestOptions extends Partial<WechatMiniprogram.RequestOption> {
  skipLogin?: boolean;
  skipUnauthorizedRetry?: boolean;
}

function getAppInstance(): any {
  try {
    return getApp<any>();
  } catch (err) {
    return null;
  }
}

function hasToken(): boolean {
  return !!wx.getStorageSync('token');
}

export function clearAuthStorage() {
  try {
    wx.removeStorageSync('token');
  } catch (err) {}

  try {
    wx.removeStorageSync('openid');
  } catch (err) {}
}

export async function ensureAuthorizedSession(forceLogin: boolean = false): Promise<void> {
  const appInstance = getAppInstance();
  if (!appInstance || typeof appInstance.autoLogin !== 'function') {
    return;
  }

  if (forceLogin) {
    clearAuthStorage();
    await appInstance.autoLogin();
    return;
  }

  if (appInstance.loginPromise) {
    await appInstance.loginPromise;
    return;
  }

  if (!hasToken()) {
    await appInstance.autoLogin();
  }
}

function doRequest<T = any>(
  url: string,
  options: RequestOptions = {}
): Promise<T> {
  return new Promise((resolve, reject) => {
    wx.request({
      url: BASE_URL + url,
      method: options.method || 'GET',
      data: options.data,
      header: {
        ...getAuthHeader(),
        'Content-Type': 'application/json',
        ...options.header
      },
      success: (res) => {
        console.log('request response:', res);
        console.log('request data:', res.data);

        if (!res.data) {
          reject(new Error('empty response data'));
          return;
        }

        const apiRes = res.data as ApiResponse<T>;

        if (res.statusCode === 401 || res.statusCode === 403 || apiRes.message === 'unauthorized') {
          reject(new Error('unauthorized'));
          return;
        }

        if (apiRes.code !== 0) {
          reject(new Error(apiRes.message || 'request failed'));
          return;
        }

        resolve(apiRes.data as T);
      },
      fail: (err) => {
        reject(new Error(err.errMsg || 'request failed'));
      }
    });
  });
}

export async function request<T = any>(
  url: string,
  options: RequestOptions = {}
): Promise<T> {
  const skipLogin = !!options.skipLogin || url === '/auth/login';
  if (!skipLogin) {
    await ensureAuthorizedSession();
  }

  try {
    return await doRequest<T>(url, options);
  } catch (err: any) {
    if (skipLogin || options.skipUnauthorizedRetry || !err || err.message !== 'unauthorized') {
      throw err;
    }

    await ensureAuthorizedSession(true);
    return doRequest<T>(url, {
      ...options,
      skipUnauthorizedRetry: true
    });
  }
}

export function getAuthHeader(): Record<string, string> {
  const token = wx.getStorageSync('token');
  return {
    'Authorization': token ? `Bearer ${token}` : ''
  };
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
