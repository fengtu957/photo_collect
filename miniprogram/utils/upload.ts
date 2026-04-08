export interface UploadResult {
  fileID: string;
  statusCode: number;
}

export async function uploadToCloud(
  filePath: string,
  cloudPath: string
): Promise<UploadResult | null> {
  try {
    const res = await wx.cloud.uploadFile({ cloudPath, filePath });
    return res;
  } catch (err) {
    console.error('上传失败:', err);
    return null;
  }
}

export function generateCloudPath(taskId: string, openid: string, ext: string = 'jpg'): string {
  const timestamp = Date.now();
  return `submissions/${taskId}/${openid}_${timestamp}.${ext}`;
}

export async function getImageInfo(filePath: string): Promise<wx.GetImageInfoSuccessCallbackResult | null> {
  try {
    const res = await wx.getImageInfo({ src: filePath });
    return res;
  } catch (err) {
    console.error('获取图片信息失败:', err);
    return null;
  }
}
