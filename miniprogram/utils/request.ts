interface CloudFunctionResult<T = any> {
  success: boolean;
  data?: T;
  error?: string;
}

export async function callFunction<T = any>(
  name: string,
  data: any = {}
): Promise<CloudFunctionResult<T>> {
  try {
    const res = await wx.cloud.callFunction({ name, data });
    return res.result as CloudFunctionResult<T>;
  } catch (err: any) {
    console.error(`云函数 ${name} 调用失败:`, err);
    return { success: false, error: err.errMsg || '网络错误' };
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
