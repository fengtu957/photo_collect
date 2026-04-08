import { getUploadToken } from '../../services/upload';
import { createSubmission } from '../../services/submission';
import { showError, showSuccess, showLoading, hideLoading } from '../../utils/request';

Page({
  data: {
    taskId: '',
    photoPath: ''
  },

  onLoad(options: any) {
    this.setData({ taskId: options.taskId });
  },

  choosePhoto() {
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      success: (res) => {
        this.setData({ photoPath: res.tempFiles[0].tempFilePath });
      }
    });
  },

  async submitPhoto() {
    showLoading('上传中...');
    try {
      const { token } = await getUploadToken();
      const key = `photo_${Date.now()}.jpg`;

      await wx.uploadFile({
        url: 'https://test1-oss.starpix.cn',
        filePath: this.data.photoPath,
        name: 'file',
        formData: { token, key }
      });

      const photoUrl = `https://your-domain.com/${key}`;
      await createSubmission({ taskId: this.data.taskId, photoUrl });

      hideLoading();
      showSuccess('提交成功');
      setTimeout(() => wx.navigateBack(), 1500);
    } catch (err: any) {
      hideLoading();
      showError(err.message || '上传失败');
    }
  }
});
