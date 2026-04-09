import { getTask } from '../../services/task';
import { getUploadToken } from '../../services/upload';
import { createSubmission } from '../../services/submission';
import { showError, showSuccess, showLoading, hideLoading } from '../../utils/request';

Page({
  data: {
    taskId: '',
    task: null as any,
    photoPath: '',
    customData: {} as Record<string, any>
  },

  async onLoad(options: any) {
    this.setData({ taskId: options.taskId });
    await this.loadTask();
  },

  async loadTask() {
    try {
      const task = await getTask(this.data.taskId);
      this.setData({ task });
    } catch (err: any) {
      showError(err.message || '加载任务失败');
    }
  },

  choosePhoto() {
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sizeType: ['original'],
      sourceType: ['album', 'camera'],
      success: (res) => {
        this.setData({ photoPath: res.tempFiles[0].tempFilePath });
      }
    });
  },

  onCustomFieldInput(e: any) {
    const field = e.currentTarget.dataset.field;
    const value = e.detail.value;
    this.setData({ [`customData.${field}`]: value });
  },

  onCustomFieldChange(e: any) {
    const field = e.currentTarget.dataset.field;
    const fieldConfig = this.data.task.custom_fields.find((f: any) => f.id === field);

    if (fieldConfig.type === 'select') {
      const index = e.detail.value;
      const value = fieldConfig.options[index];
      this.setData({ [`customData.${field}`]: value });
    } else if (fieldConfig.type === 'multiselect') {
      const indices = e.detail.value;
      const values = indices.map((i: number) => fieldConfig.options[i]);
      this.setData({ [`customData.${field}`]: values });
    }
  },

  submitPhoto() {
    if (!this.data.photoPath) {
      showError('请先选择照片');
      return;
    }

    // 验证必填字段
    const task = this.data.task;
    if (task && task.custom_fields) {
      for (const field of task.custom_fields) {
        if (field.required && !this.data.customData[field.id]) {
          showError(`请填写${field.label}`);
          return;
        }
      }
    }

    showLoading('上传中...');

    // 1. 获取上传token
    getUploadToken().then(({ token }) => {
      const openid = wx.getStorageSync('openid') || 'unknown';
      const timestamp = Date.now();
      const random = Math.floor(Math.random() * 10000);
      const key = `photo_${openid}_${timestamp}_${random}.jpg`;

      // 2. 上传到七牛云
      wx.uploadFile({
        url: 'https://up-z2.qiniup.com',
        filePath: this.data.photoPath,
        name: 'file',
        formData: { token, key },
        success: (uploadRes) => {
          console.log('七牛云上传成功:', uploadRes);

          if (uploadRes.statusCode !== 200) {
            hideLoading();
            showError('上传失败');
            return;
          }

          // 3. 提交到后端（使用正确的数据结构）
          createSubmission({
            task_id: this.data.taskId,
            photo: {
              url: key
            },
            custom_data: this.data.customData
          }).then(() => {
            hideLoading();
            showSuccess('提交成功');
            setTimeout(() => wx.navigateBack(), 1500);
          }).catch((err: any) => {
            hideLoading();
            showError(err.message || '提交失败');
          });
        },
        fail: (err) => {
          console.error('七牛云上传失败:', err);
          hideLoading();
          showError('上传失败');
        }
      });
    }).catch((err: any) => {
      hideLoading();
      showError(err.message || '获取上传凭证失败');
    });
  }
});
