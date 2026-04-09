import { getTask } from '../../services/task';
import { getUploadToken } from '../../services/upload';
import { createSubmission, getSubmission, updateSubmission } from '../../services/submission';
import { showError, showSuccess, showLoading, hideLoading } from '../../utils/request';
import { isEffectiveTime } from '../../utils/time';
import { getTimeRemaining, isTaskActive } from '../../utils/format';

function isRemoteUrl(path: string): boolean {
  return path.indexOf('http://') === 0 || path.indexOf('https://') === 0;
}

function extractFileKey(fileUrl: string): string {
  if (!fileUrl) return '';

  const queryIndex = fileUrl.indexOf('?');
  const urlWithoutQuery = queryIndex >= 0 ? fileUrl.slice(0, queryIndex) : fileUrl;
  const protocolIndex = urlWithoutQuery.indexOf('://');

  if (protocolIndex < 0) {
    return decodeURIComponent(urlWithoutQuery.replace(/^\/+/, ''));
  }

  const pathIndex = urlWithoutQuery.indexOf('/', protocolIndex + 3);
  if (pathIndex < 0) return '';

  return decodeURIComponent(urlWithoutQuery.slice(pathIndex + 1));
}

function isEmptyFieldValue(value: any): boolean {
  if (Array.isArray(value)) {
    return value.length === 0;
  }

  return value === undefined || value === null || value === '';
}

function getTaskUnavailableMessage(task: any): string {
  if (!task) return '任务不存在';
  if (task.enabled === false) return '任务已停用';

  const now = new Date();
  const hasStartTime = isEffectiveTime(task.start_time);
  const hasEndTime = isEffectiveTime(task.end_time);
  const start = hasStartTime ? new Date(task.start_time) : null;
  const end = hasEndTime ? new Date(task.end_time) : null;

  if (start && now < start) {
    return '任务尚未开始';
  }
  if (end && now > end) {
    return '任务已截止';
  }
  if (hasEndTime && isTaskActive(task.start_time, task.end_time)) return '';

  return '';
}

Page({
  data: {
    taskId: '',
    submissionId: '',
    task: null as any,
    taskStatusText: '',
    taskUnavailableMessage: '',
    photoPath: '',
    photoKey: '',
    customData: {} as Record<string, any>,
    isEditMode: false
  },

  async onLoad(options: any) {
    this.setData({
      taskId: options.taskId,
      submissionId: options.submissionId || '',
      isEditMode: !!options.submissionId
    });

    // 设置页面标题
    wx.setNavigationBarTitle({
      title: this.data.isEditMode ? '编辑提交' : '上传照片'
    });

    await this.loadTask();

    // 如果是编辑模式，加载已有的提交数据
    if (this.data.isEditMode) {
      await this.loadSubmission();
    }
  },

  async loadTask() {
    try {
      const task = await getTask(this.data.taskId);
      const unavailableMessage = getTaskUnavailableMessage(task);
      let taskStatusText = '';

      if (unavailableMessage) {
        taskStatusText = unavailableMessage;
      } else if (isEffectiveTime(task.end_time)) {
        taskStatusText = getTimeRemaining(task.end_time);
      }

      this.setData({
        task,
        taskStatusText,
        taskUnavailableMessage: unavailableMessage
      });
    } catch (err: any) {
      showError(err.message || '加载任务失败');
    }
  },

  async loadSubmission() {
    try {
      const submission = await getSubmission(this.data.submissionId);
      const photoUrl = (submission.photo && submission.photo.url) || '';

      this.setData({
        customData: submission.custom_data || {},
        photoPath: photoUrl,
        photoKey: extractFileKey(photoUrl)
      });
    } catch (err: any) {
      showError(err.message || '加载提交数据失败');
    }
  },

  choosePhoto() {
    if (this.data.taskUnavailableMessage) {
      showError(this.data.taskUnavailableMessage);
      return;
    }

    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sizeType: ['original'],
      sourceType: ['album', 'camera'],
      success: (res) => {
        this.setData({
          photoPath: res.tempFiles[0].tempFilePath,
          photoKey: ''
        });
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
    if (this.data.taskUnavailableMessage) {
      showError(this.data.taskUnavailableMessage);
      return;
    }

    if (!this.data.photoPath) {
      showError('请先选择照片');
      return;
    }

    // 验证必填字段
    const task = this.data.task;
    if (task && task.custom_fields) {
      for (const field of task.custom_fields) {
        if (field.required && isEmptyFieldValue(this.data.customData[field.id])) {
          showError(`请填写${field.label}`);
          return;
        }
      }
    }

    if (this.data.isEditMode && isRemoteUrl(this.data.photoPath)) {
      if (!this.data.photoKey) {
        showError('当前照片信息无效，请重新选择照片');
        return;
      }

      showLoading('提交中...');
      this.saveSubmission(this.data.photoKey);
      return;
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

          // 3. 提交到后端
          this.saveSubmission(key);
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
  },

  saveSubmission(photoKey: string) {
    const params = {
      task_id: this.data.taskId,
      photo: {
        url: photoKey
      },
      custom_data: this.data.customData
    };

    const submitPromise = this.data.isEditMode
      ? updateSubmission(this.data.submissionId, params)
      : createSubmission(params);

    submitPromise.then(() => {
      hideLoading();
      showSuccess(this.data.isEditMode ? '更新成功' : '提交成功');
      setTimeout(() => wx.navigateBack(), 1500);
    }).catch((err: any) => {
      hideLoading();
      showError(err.message || '提交失败');
    });
  }
});
