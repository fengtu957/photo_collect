import { getTask } from '../../services/task';
import { getUploadToken } from '../../services/upload';
import { createSubmission, getSubmission, updateSubmission } from '../../services/submission';
import { showError, showSuccess, showLoading, hideLoading } from '../../utils/request';
import { isEffectiveTime } from '../../utils/time';
import { getTimeRemaining, isTaskActive } from '../../utils/format';

const COMPRESS_QUALITY_STEPS = [90, 80, 70, 60, 50, 40, 30, 20];

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

function normalizeCustomData(task: any, customData: Record<string, any>): Record<string, any> {
  const nextCustomData: Record<string, any> = { ...(customData || {}) };
  const fields = (task && task.custom_fields) || [];

  fields.forEach((field: any) => {
    if (field.type !== 'multiselect') return;

    const value = nextCustomData[field.id];
    if (Array.isArray(value)) {
      nextCustomData[field.id] = value.filter((item: any) => item !== undefined && item !== null && item !== '');
      return;
    }

    if (typeof value === 'string') {
      nextCustomData[field.id] = value
        .split(',')
        .map((item: string) => String(item || '').trim())
        .filter(Boolean);
      return;
    }

    nextCustomData[field.id] = [];
  });

  return nextCustomData;
}

function buildMultiSelectState(task: any, customData: Record<string, any>): Record<string, Record<string, boolean>> {
  const state: Record<string, Record<string, boolean>> = {};
  const fields = (task && task.custom_fields) || [];

  fields.forEach((field: any) => {
    if (field.type !== 'multiselect') return;

    const selectedValues = Array.isArray(customData[field.id]) ? customData[field.id] : [];
    const fieldState: Record<string, boolean> = {};

    (field.options || []).forEach((option: string) => {
      fieldState[option] = selectedValues.indexOf(option) >= 0;
    });

    state[field.id] = fieldState;
  });

  return state;
}

function getLocalFileInfo(filePath: string): Promise<WechatMiniprogram.GetFileInfoSuccessCallbackResult> {
  return new Promise((resolve, reject) => {
    wx.getFileInfo({
      filePath,
      success: resolve,
      fail: reject
    });
  });
}

function getLocalImageInfo(filePath: string): Promise<WechatMiniprogram.GetImageInfoSuccessCallbackResult> {
  return new Promise((resolve, reject) => {
    wx.getImageInfo({
      src: filePath,
      success: resolve,
      fail: reject
    });
  });
}

function compressImage(filePath: string, quality: number): Promise<string> {
  return new Promise((resolve, reject) => {
    wx.compressImage({
      src: filePath,
      quality,
      success: (res) => resolve(res.tempFilePath),
      fail: reject
    });
  });
}

async function getPhotoMeta(filePath: string) {
  const [fileInfo, imageInfo] = await Promise.all([
    getLocalFileInfo(filePath),
    getLocalImageInfo(filePath)
  ]);

  return {
    filePath,
    fileSize: Number(fileInfo.size || 0),
    width: Number(imageInfo.width || 0),
    height: Number(imageInfo.height || 0)
  };
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
    photoMeta: { fileSize: 0, width: 0, height: 0 },
    customData: {} as Record<string, any>,
    multiSelectState: {} as Record<string, Record<string, boolean>>,
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
        taskUnavailableMessage: unavailableMessage,
        multiSelectState: buildMultiSelectState(task, this.data.customData)
      });
    } catch (err: any) {
      showError(err.message || '加载任务失败');
    }
  },

  async loadSubmission() {
    try {
      const submission = await getSubmission(this.data.submissionId);
      const photoUrl = (submission.photo && submission.photo.url) || '';
      const normalizedCustomData = normalizeCustomData(this.data.task, submission.custom_data || {});

      this.setData({
        customData: normalizedCustomData,
        multiSelectState: buildMultiSelectState(this.data.task, normalizedCustomData),
        photoPath: photoUrl,
        photoKey: extractFileKey(photoUrl),
        photoMeta: {
          fileSize: Number((submission.photo && submission.photo.file_size) || 0),
          width: Number((submission.photo && submission.photo.width) || 0),
          height: Number((submission.photo && submission.photo.height) || 0)
        }
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

    wx.showActionSheet({
      itemList: ['拍照', '从相册选择'],
      success: (res) => {
        if (res.tapIndex === 0) {
          this.openCameraPage();
          return;
        }

        this.chooseFromAlbum();
      },
      fail: (err) => {
        if (err && err.errMsg && err.errMsg.indexOf('cancel') >= 0) {
          return;
        }
        showError('打开选择方式失败');
      }
    });
  },

  openCameraPage() {
    wx.navigateTo({
      url: '/pages/camera-shoot/camera-shoot',
      events: {
        photoSelected: (data: any) => {
          if (!data || !data.tempFilePath) return;
          this.setData({
            photoPath: data.tempFilePath,
            photoKey: '',
            photoMeta: { fileSize: 0, width: 0, height: 0 }
          });
        }
      }
    });
  },

  chooseFromAlbum() {
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sizeType: ['original'],
      sourceType: ['album'],
      success: (res) => {
        this.setData({
          photoPath: res.tempFiles[0].tempFilePath,
          photoKey: '',
          photoMeta: { fileSize: 0, width: 0, height: 0 }
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

    if (fieldConfig && fieldConfig.type === 'select') {
      const index = e.detail.value;
      const value = fieldConfig.options[index];
      this.setData({ [`customData.${field}`]: value });
    }
  },

  onMultiSelectChange(e: any) {
    const field = e.currentTarget.dataset.field;
    const values = (e.detail && e.detail.value) || [];
    const nextCustomData = {
      ...this.data.customData,
      [field]: values
    };
    this.setData({
      [`customData.${field}`]: values,
      multiSelectState: buildMultiSelectState(this.data.task, nextCustomData)
    });
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

    showLoading('处理照片中...');

    this.preparePhotoForUpload(this.data.photoPath).then((preparedPhoto) => {
      showLoading('上传中...');
      return getUploadToken().then(({ token }) => ({
        token,
        preparedPhoto
      }));
    }).then(({ token, preparedPhoto }) => {
      const openid = wx.getStorageSync('openid') || 'unknown';
      const timestamp = Date.now();
      const random = Math.floor(Math.random() * 10000);
      const key = `photo_${openid}_${timestamp}_${random}.jpg`;

      // 2. 上传到七牛云
      wx.uploadFile({
        url: 'https://up-z2.qiniup.com',
        filePath: preparedPhoto.filePath,
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
          this.saveSubmission(key, preparedPhoto);
        },
        fail: (err) => {
          console.error('七牛云上传失败:', err);
          hideLoading();
          showError('上传失败');
        }
      });
    }).catch((err: any) => {
      hideLoading();
      showError(err.message || '照片处理失败');
    });
  },

  async preparePhotoForUpload(filePath: string) {
    const limitKB = Number((this.data.task && this.data.task.photo_spec && this.data.task.photo_spec.max_size_kb) || 0);
    const limitBytes = limitKB > 0 ? limitKB * 1024 : 0;
    let photoMeta = await getPhotoMeta(filePath);

    if (limitBytes > 0 && photoMeta.fileSize > limitBytes) {
      for (const quality of COMPRESS_QUALITY_STEPS) {
        const compressedPath = await compressImage(filePath, quality);
        photoMeta = await getPhotoMeta(compressedPath);
        if (photoMeta.fileSize <= limitBytes) {
          break;
        }
      }

      if (photoMeta.fileSize > limitBytes) {
        throw new Error('自动压缩后仍超过大小限制');
      }
    }

    this.setData({
      photoPath: photoMeta.filePath,
      photoMeta: {
        fileSize: photoMeta.fileSize,
        width: photoMeta.width,
        height: photoMeta.height
      }
    });

    return photoMeta;
  },

  saveSubmission(photoKey: string, preparedPhoto?: any) {
    const customData = normalizeCustomData(this.data.task, this.data.customData);
    const photoMeta = preparedPhoto || this.data.photoMeta || {};
    const params = {
      task_id: this.data.taskId,
      photo: {
        url: photoKey,
        file_size: Number(photoMeta.fileSize || 0),
        width: Number(photoMeta.width || 0),
        height: Number(photoMeta.height || 0)
      },
      custom_data: customData
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
