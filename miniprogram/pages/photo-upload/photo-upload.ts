import { getTask } from '../../services/task';
import { finalizePhoto, getUploadPolicy, OSSUploadPolicy } from '../../services/upload';
import { analyzePhotoPreview, createSubmission, deleteSubmission, getSubmission, segmentPhoto, updateSubmission } from '../../services/submission';
import { SubmissionAnalysisResult } from '../../types/submission';
import { showError, showLoading, hideLoading } from '../../utils/request';
import { isEffectiveTime } from '../../utils/time';
import { getTimeRemaining, isTaskActive } from '../../utils/format';
import { isTaskAIAnalysisEnabled, isTaskBackgroundReplacementEnabled } from '../../utils/task';

const COMPRESS_QUALITY_STEPS = [90, 80, 70, 60, 50, 40, 30, 20];

function normalizeDigitText(value: any): string {
  return String(value || '').replace(/\D/g, '');
}

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

function getLocalFileInfo(filePath: string): Promise<WechatMiniprogram.WxGetFileInfoSuccessCallbackResult> {
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

function downloadFile(fileUrl: string): Promise<string> {
  return new Promise((resolve, reject) => {
    wx.downloadFile({
      url: fileUrl,
      success: (res) => {
        if (res.statusCode !== 200 || !res.tempFilePath) {
          reject(new Error('下载分割结果失败'));
          return;
        }
        resolve(res.tempFilePath);
      },
      fail: reject
    });
  });
}

function uploadPhotoToOSS(filePath: string, policy: OSSUploadPolicy): Promise<void> {
  return new Promise((resolve, reject) => {
    wx.uploadFile({
      url: policy.upload_url,
      filePath,
      name: 'file',
      formData: policy.fields,
      success: (uploadRes) => {
        if (uploadRes.statusCode !== 200 && uploadRes.statusCode !== 204) {
          const responseText = String(uploadRes.data || '');
          const codeMatch = responseText.match(/<Code>([^<]+)<\/Code>/);
          const errorCode = codeMatch && codeMatch[1] ? `：${codeMatch[1]}` : '';
          console.error('OSS upload failed:', uploadRes.statusCode, responseText);
          reject(new Error(`上传阿里云 OSS 失败 (${uploadRes.statusCode})${errorCode}`));
          return;
        }
        resolve();
      },
      fail: () => reject(new Error('上传阿里云 OSS 失败'))
    });
  });
}

function getBackgroundColor(value: string): string {
  const normalized = String(value || '').trim();
  if (normalized === '白底') return '#FFFFFF';
  if (normalized === '蓝底') return '#438EDB';
  if (normalized === '红底') return '#D7373F';
  if (normalized === '纯色') return '#FFFFFF';
  if (/^#[0-9a-fA-F]{6}$/.test(normalized)) return normalized;
  return '';
}

Page({
  data: {
    taskId: '',
    submissionId: '',
    task: null as any,
    taskStatusText: '',
    taskUnavailableMessage: '',
    isTaskCreator: false,
    requiresVerificationCode: false,
    verificationCodeInput: '',
    photoPath: '',
    sourcePhotoPath: '',
    temporaryPhotoPath: '',
    temporaryPhotoKey: '',
    temporaryPhotoMeta: { fileSize: 0, width: 0, height: 0 },
    photoKey: '',
    photoMeta: { fileSize: 0, width: 0, height: 0 },
    analysisState: '' as '' | 'existing' | 'analyzing' | 'success' | 'fallback' | 'error',
    analysisResult: null as SubmissionAnalysisResult | null,
    analysisPassed: false,
    analysisError: '',
    analysisMessage: '',
    verificationToken: '',
    sourceVerificationToken: '',
    backgroundReplacementEnabled: false,
    backgroundColorText: '',
    backgroundState: '' as '' | 'pending' | 'processing' | 'success' | 'error',
    backgroundError: '',
    backgroundProcessing: false,
    photoProcessing: false,
    canSubmit: false,
    aiAnalysisEnabled: true,
    customData: {} as Record<string, any>,
    multiSelectState: {} as Record<string, Record<string, boolean>>,
    isEditMode: false,
    canvasWidth: 1,
    canvasHeight: 1,
    keyboardSpacerHeight: 0,
    focusedInputTarget: '',
    pageScrollTopBeforeKeyboard: 0
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
      const currentOpenid = wx.getStorageSync('openid') || '';
      const isTaskCreator = task.user_id === currentOpenid;
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
        isTaskCreator,
        requiresVerificationCode: !!(task && task.verification_code_enabled && !isTaskCreator),
        aiAnalysisEnabled: isTaskAIAnalysisEnabled(task),
        backgroundReplacementEnabled: isTaskBackgroundReplacementEnabled(task),
        backgroundColorText: String((task && task.photo_spec && task.photo_spec.background_color) || '').trim(),
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
      const aiAnalysisEnabled = isTaskAIAnalysisEnabled(this.data.task);

      this.setData({
        customData: normalizedCustomData,
        multiSelectState: buildMultiSelectState(this.data.task, normalizedCustomData),
        photoPath: photoUrl,
        photoKey: extractFileKey(photoUrl),
        photoMeta: {
          fileSize: Number((submission.photo && submission.photo.file_size) || 0),
          width: Number((submission.photo && submission.photo.width) || 0),
          height: Number((submission.photo && submission.photo.height) || 0)
        },
        analysisState: aiAnalysisEnabled && photoUrl ? 'existing' : '',
        analysisResult: null,
        analysisPassed: !!photoUrl,
        analysisError: '',
        analysisMessage: aiAnalysisEnabled
          ? (photoUrl ? '已保存照片，可直接提交；重新选图后会再次检查。' : '')
          : '',
        verificationToken: '',
        backgroundState: photoUrl && this.data.backgroundReplacementEnabled ? 'success' : '',
        backgroundError: '',
        backgroundProcessing: false,
        canSubmit: !!photoUrl
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

    if (!this.data.task) {
      showError('任务加载中，请稍候');
      return;
    }

    if (this.data.task.disallow_album_photos) {
      this.openCameraPage();
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
    this.openPhotoCropPage('camera');
  },

  openPhotoCropPage(sourceType: 'camera' | 'album') {
    if (sourceType === 'album' && this.data.task && this.data.task.disallow_album_photos) {
      showError('当前任务不允许使用相册照片，请直接拍照');
      return;
    }
    wx.navigateTo({
      url: `/pages/camera-shoot/camera-shoot?source=${sourceType}&disallowAlbumPhotos=${this.data.task && this.data.task.disallow_album_photos ? '1' : '0'}`,
      events: {
        photoSelected: (data: any) => {
          if (!data || !data.tempFilePath) return;
          this.handleSelectedPhoto(data.tempFilePath);
        }
      }
    });
  },

  chooseFromAlbum() {
    if (this.data.task && this.data.task.disallow_album_photos) {
      showError('当前任务不允许使用相册照片，请直接拍照');
      return;
    }
    this.openPhotoCropPage('album');
  },

  handleSelectedPhoto(filePath: string) {
    if (this.data.photoProcessing) return;
    const shouldReplaceBackground = !!this.data.backgroundReplacementEnabled;
    this.setData({
      sourcePhotoPath: filePath,
      temporaryPhotoPath: '',
      temporaryPhotoKey: '',
      temporaryPhotoMeta: { fileSize: 0, width: 0, height: 0 },
      photoPath: filePath,
      photoKey: '',
      photoMeta: { fileSize: 0, width: 0, height: 0 },
      analysisState: this.data.aiAnalysisEnabled ? 'analyzing' : '',
      analysisResult: null,
      analysisPassed: false,
      analysisError: '',
      analysisMessage: this.data.aiAnalysisEnabled ? '正在上传照片，随后进行 AI 检查…' : '',
      verificationToken: '',
      sourceVerificationToken: '',
      backgroundState: shouldReplaceBackground ? 'pending' : '',
      backgroundError: '',
      backgroundProcessing: false,
      photoProcessing: true,
      canSubmit: false
    });
    this.processSelectedPhoto(filePath);
  },

  async processSelectedPhoto(filePath: string) {
    try {
      showLoading('处理照片中...');
      const preparedPhoto = await this.preparePhotoForUpload(filePath);
      if (!this.data.aiAnalysisEnabled && !this.data.backgroundReplacementEnabled) {
        await this.finalizeDirectPhoto(preparedPhoto);
        return;
      }

      const policy = await getUploadPolicy(this.data.taskId, 'temporary');
      await uploadPhotoToOSS(preparedPhoto.filePath, policy);
      this.setData({
        temporaryPhotoPath: preparedPhoto.filePath,
        temporaryPhotoKey: policy.key,
        temporaryPhotoMeta: {
          fileSize: preparedPhoto.fileSize,
          width: preparedPhoto.width,
          height: preparedPhoto.height
        },
        photoPath: preparedPhoto.filePath,
        photoMeta: {
          fileSize: preparedPhoto.fileSize,
          width: preparedPhoto.width,
          height: preparedPhoto.height
        }
      });
      await this.processTemporaryPhoto(preparedPhoto, policy.key);
    } catch (err: any) {
      const errorMessage = (err && err.message) || '处理照片失败';
      this.handlePhotoProcessingError(errorMessage);
      showError(errorMessage);
    } finally {
      this.setData({ photoProcessing: false, backgroundProcessing: false });
      hideLoading();
    }
  },

  async finalizeDirectPhoto(preparedPhoto: any) {
    showLoading('上传照片中...');
    const finalPolicy = await getUploadPolicy(this.data.taskId, 'final');
    await uploadPhotoToOSS(preparedPhoto.filePath, finalPolicy);
    const finalized = await finalizePhoto(this.data.taskId, '', finalPolicy.key, '');
    this.applyFinalizedPhoto(preparedPhoto, finalized);
  },

  async processTemporaryPhoto(preparedPhoto: any, temporaryKey: string) {
    let sourceVerificationToken = '';
    if (this.data.aiAnalysisEnabled) {
      showLoading('AI检查中...');
      const result = await analyzePhotoPreview({
        task_id: this.data.taskId,
        photo: { url: temporaryKey }
      });
      const aiUnavailable = !result
        || result.analysis_status === 'unavailable'
        || result.available === false;
      sourceVerificationToken = (result && result.verification_token) || '';
      const analysisApproved = !aiUnavailable
        && !!(result && (result.can_submit || result.passed))
        && !!sourceVerificationToken;

      this.setData({
        analysisState: aiUnavailable ? 'error' : 'success',
        analysisResult: result || null,
        analysisPassed: analysisApproved,
        analysisError: aiUnavailable ? 'AI 检查暂不可用，请稍后重试。' : '',
        analysisMessage: aiUnavailable
          ? 'AI 检查暂不可用，暂不能提交。'
          : (analysisApproved ? '照片检查通过，正在处理最终照片。' : '照片未通过检查，请重新选择。'),
        sourceVerificationToken,
        canSubmit: false
      });
      if (!analysisApproved) {
        return;
      }
    }
    await this.finalizeTemporaryPhoto(preparedPhoto, temporaryKey, sourceVerificationToken);
  },

  async finalizeTemporaryPhoto(preparedPhoto: any, temporaryKey: string, sourceVerificationToken: string) {
    const requestedBackgroundColor = String((this.data.task && this.data.task.photo_spec && this.data.task.photo_spec.background_color) || '').trim();
    const backgroundColor = getBackgroundColor(requestedBackgroundColor);
    if (this.data.backgroundReplacementEnabled && !backgroundColor) {
      throw new Error('任务配置的背景颜色无效，请联系任务创建者');
    }

    let finalKey = '';
    let finalPhoto = preparedPhoto;
    if (this.data.backgroundReplacementEnabled) {
      this.setData({
        backgroundState: 'processing',
        backgroundError: '',
        backgroundProcessing: true
      });
      showLoading('生成背景中...');
      const segmentResult = await segmentPhoto(this.data.taskId, temporaryKey);
      const transparentPath = await downloadFile(segmentResult.result_url);
      const compositedPath = await this.composeBackground(transparentPath, backgroundColor, preparedPhoto.width, preparedPhoto.height);
      finalPhoto = await this.preparePhotoForUpload(compositedPath);
      const finalPolicy = await getUploadPolicy(
        this.data.taskId,
        'final',
        temporaryKey,
        sourceVerificationToken
      );
      await uploadPhotoToOSS(finalPhoto.filePath, finalPolicy);
      finalKey = finalPolicy.key;
    }

    const finalized = await finalizePhoto(
      this.data.taskId,
      temporaryKey,
      finalKey,
      sourceVerificationToken
    );
    this.applyFinalizedPhoto(finalPhoto, finalized);
  },

  applyFinalizedPhoto(finalPhoto: any, finalized: any) {
    this.setData({
      photoPath: finalPhoto.filePath,
      photoKey: finalized && finalized.photo_key ? finalized.photo_key : '',
      photoMeta: {
        fileSize: Number((finalized && finalized.file_size) || finalPhoto.fileSize),
        width: Number(finalPhoto.width || 0),
        height: Number(finalPhoto.height || 0)
      },
      verificationToken: (finalized && finalized.verification_token) || '',
      analysisMessage: this.data.aiAnalysisEnabled ? '照片检查通过，可以提交。' : '',
      backgroundState: this.data.backgroundReplacementEnabled ? 'success' : '',
      backgroundError: '',
      backgroundProcessing: false,
      canSubmit: true
    });
  },

  handlePhotoProcessingError(errorMessage: string) {
    const backgroundFailed = this.data.backgroundReplacementEnabled
      && this.data.backgroundState === 'processing';
    if (backgroundFailed) {
      this.setData({
        photoKey: '',
        verificationToken: '',
        canSubmit: false,
        backgroundState: 'error',
        backgroundError: errorMessage,
        backgroundProcessing: false
      });
      return;
    }
    this.setData({
      photoKey: '',
      verificationToken: '',
      analysisState: this.data.aiAnalysisEnabled ? 'error' : '',
      analysisResult: null,
      analysisPassed: false,
      analysisError: errorMessage,
      analysisMessage: this.data.temporaryPhotoKey
        ? '照片已上传，但处理尚未完成。'
        : '照片上传失败，请重试。',
      canSubmit: false
    });
  },

  async composeBackground(transparentPath: string, color: string, width: number, height: number): Promise<string> {
    const transparentImage = await getLocalImageInfo(transparentPath);
    const decodedPath = String(transparentImage.path || transparentPath);
    const sourceWidth = Math.max(1, Number(transparentImage.width || width || 1));
    const sourceHeight = Math.max(1, Number(transparentImage.height || height || 1));
    const canvasWidth = Math.max(1, Number(width || sourceWidth));
    const canvasHeight = Math.max(1, Number(height || sourceHeight));
    await new Promise<void>((resolve) => {
      this.setData({ canvasWidth, canvasHeight }, resolve);
    });

    const canvas = await new Promise<any>((resolve, reject) => {
      wx.createSelectorQuery()
        .in(this)
        .select('#background-canvas')
        .node((result: any) => {
          if (!result || !result.node) {
            reject(new Error('背景画布初始化失败'));
            return;
          }
          resolve(result.node);
        })
        .exec();
    });
    canvas.width = canvasWidth;
    canvas.height = canvasHeight;

    const canvasImage = canvas.createImage();
    await new Promise<void>((resolve, reject) => {
      canvasImage.onload = () => resolve();
      canvasImage.onerror = (err: any) => reject(new Error((err && err.errMsg) || '透明抠图加载失败'));
      canvasImage.src = decodedPath;
    });

    const context = canvas.getContext('2d');
    context.clearRect(0, 0, canvasWidth, canvasHeight);
    context.fillStyle = color;
    context.fillRect(0, 0, canvasWidth, canvasHeight);
    context.drawImage(
      canvasImage,
      0,
      0,
      sourceWidth,
      sourceHeight,
      0,
      0,
      canvasWidth,
      canvasHeight
    );

    return new Promise<string>((resolve, reject) => {
      wx.canvasToTempFilePath({
        canvas,
        x: 0,
        y: 0,
        width: canvasWidth,
        height: canvasHeight,
        destWidth: canvasWidth,
        destHeight: canvasHeight,
        fileType: 'jpg',
        quality: 0.92,
        success: (res) => resolve(res.tempFilePath),
        fail: reject
      }, this);
    });
  },

  async retryAnalyzePhoto() {
    if (this.data.photoProcessing || this.data.backgroundProcessing) return;
    if (!this.data.aiAnalysisEnabled) {
      return;
    }
    if (!this.data.sourcePhotoPath) {
      showError('请先选择照片');
      return;
    }
    if (!this.data.temporaryPhotoKey || !this.data.temporaryPhotoPath) {
      this.handleSelectedPhoto(this.data.sourcePhotoPath);
      return;
    }

    this.setData({
      photoKey: '',
      verificationToken: '',
      sourceVerificationToken: '',
      analysisState: 'analyzing',
      analysisResult: null,
      analysisPassed: false,
      analysisError: '',
      analysisMessage: '正在重新检查照片，请稍候…',
      backgroundState: this.data.backgroundReplacementEnabled ? 'pending' : '',
      backgroundError: '',
      photoProcessing: true,
      canSubmit: false
    });
    try {
      showLoading('AI检查中...');
      await this.processTemporaryPhoto({
        filePath: this.data.temporaryPhotoPath,
        fileSize: this.data.temporaryPhotoMeta.fileSize,
        width: this.data.temporaryPhotoMeta.width,
        height: this.data.temporaryPhotoMeta.height
      }, this.data.temporaryPhotoKey);
    } catch (err: any) {
      const errorMessage = (err && err.message) || '照片处理失败，请重试';
      this.handlePhotoProcessingError(errorMessage);
      showError(errorMessage);
    } finally {
      this.setData({ photoProcessing: false, backgroundProcessing: false });
      hideLoading();
    }
  },

  async retryBackgroundReplacement() {
    if (this.data.photoProcessing || this.data.backgroundProcessing) return;
    if (!this.data.temporaryPhotoKey || !this.data.temporaryPhotoPath) {
      showError('临时照片已失效，请重新选择照片');
      return;
    }

    this.setData({
      photoPath: this.data.temporaryPhotoPath,
      photoKey: '',
      verificationToken: '',
      backgroundState: 'processing',
      backgroundError: '',
      backgroundProcessing: true,
      photoProcessing: true,
      canSubmit: false
    });

    try {
      await this.finalizeTemporaryPhoto({
        filePath: this.data.temporaryPhotoPath,
        fileSize: this.data.temporaryPhotoMeta.fileSize,
        width: this.data.temporaryPhotoMeta.width,
        height: this.data.temporaryPhotoMeta.height
      }, this.data.temporaryPhotoKey, this.data.sourceVerificationToken);
    } catch (err: any) {
      const errorMessage = (err && err.message) || '背景生成失败，请重试';
      this.handlePhotoProcessingError(errorMessage);
      showError(errorMessage);
    } finally {
      this.setData({ photoProcessing: false, backgroundProcessing: false });
      hideLoading();
    }
  },

  onCustomFieldInput(e: any) {
    const field = e.currentTarget.dataset.field;
    const value = e.detail.value;
    this.setData({ [`customData.${field}`]: value });
  },

  onInputFocus(e: any) {
    const target = String(e.currentTarget.dataset.target || '');
    if (!target) return;

    const shouldRememberScrollTop = this.data.keyboardSpacerHeight <= 0;
    this.setData({ focusedInputTarget: target });

    if (shouldRememberScrollTop) {
      const query = wx.createSelectorQuery();
      query.selectViewport().scrollOffset();
      query.exec((result: any[]) => {
        const viewport = result && result[0];
        if (!viewport) return;
        this.setData({
          pageScrollTopBeforeKeyboard: Math.max(0, Number(viewport.scrollTop || 0))
        });
      });
    }

    setTimeout(() => this.scrollInputIntoView(target), 120);
  },

  onKeyboardHeightChange(e: any) {
    const height = Math.max(0, Number((e.detail && e.detail.height) || 0));
    const target = this.data.focusedInputTarget;
    const keyboardWasOpen = this.data.keyboardSpacerHeight > 0;
    const restoreScrollTop = this.data.pageScrollTopBeforeKeyboard;
    this.setData({
      keyboardSpacerHeight: height > 0 ? height + 24 : 0,
      focusedInputTarget: height > 0 ? target : ''
    }, () => {
      if (height > 0 && target) {
        setTimeout(() => this.scrollInputIntoView(target), 60);
      } else if (height === 0 && keyboardWasOpen) {
        setTimeout(() => {
          wx.pageScrollTo({
            scrollTop: Math.max(0, Number(restoreScrollTop || 0)),
            duration: 200
          });
        }, 60);
      }
    });
  },

  scrollInputIntoView(target: string) {
    if (!target) return;

    const query = wx.createSelectorQuery();
    query.select(`#${target}`).boundingClientRect();
    query.selectViewport().scrollOffset();
    query.exec((result: any[]) => {
      const fieldRect = result && result[0];
      const viewport = result && result[1];
      if (!fieldRect || !viewport) return;

      const scrollTop = Number(viewport.scrollTop || 0) + Number(fieldRect.top || 0) - 20;
      wx.pageScrollTo({
        scrollTop: Math.max(0, scrollTop),
        duration: 200
      });
    });
  },

  onVerificationCodeInput(e: any) {
    this.setData({
      verificationCodeInput: normalizeDigitText(e.detail.value)
    });
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
    const verificationCode = normalizeDigitText(this.data.verificationCodeInput);

    if (this.data.taskUnavailableMessage) {
      showError(this.data.taskUnavailableMessage);
      return;
    }

    if (!this.data.photoPath) {
      showError('请先选择照片');
      return;
    }

    if (!this.data.canSubmit) {
      showError(this.data.aiAnalysisEnabled ? '请先选择一张通过 AI 检查的照片' : '请先选择照片');
      return;
    }
    if (this.data.requiresVerificationCode && !verificationCode) {
      showError('请输入数字校验码');
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

    if (!this.data.aiAnalysisEnabled) {
      if (!this.data.photoKey) {
        showError('照片尚未处理完成，请重新选择');
        return;
      }
      showLoading('提交中...');
      this.saveSubmission(this.data.photoKey, this.data.photoMeta);
      return;
    }

    if (!this.data.photoKey) {
      showError('当前照片尚未完成 AI 检查，请重新选择');
      return;
    }

    showLoading('提交中...');
    this.saveSubmission(this.data.photoKey, this.data.photoMeta);
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

  deleteSubmissionRecord() {
    if (!this.data.isEditMode || !this.data.submissionId) return;

    wx.showModal({
      title: '确认删除',
      content: '删除后该提交记录将无法恢复，确认删除？',
      confirmText: '删除',
      confirmColor: '#ff4444',
      success: async (res) => {
        if (!res.confirm) return;
        try {
          showLoading('删除中...');
          await deleteSubmission(this.data.submissionId);
          hideLoading();
          wx.showToast({ title: '删除成功', icon: 'success' });
          setTimeout(() => wx.navigateBack(), 1000);
        } catch (err: any) {
          hideLoading();
          showError(err.message || '删除失败');
        }
      }
    });
  },

  saveSubmission(photoKey: string, preparedPhoto?: any) {
    const customData = normalizeCustomData(this.data.task, this.data.customData);
    const photoMeta = preparedPhoto || this.data.photoMeta || {};
    const params = {
      task_id: this.data.taskId,
      verification_code: normalizeDigitText(this.data.verificationCodeInput),
      verification_token: this.data.verificationToken || undefined,
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
      wx.navigateBack();
    }).catch((err: any) => {
      hideLoading();
      showError(err.message || '提交失败');
    });
  }
});
