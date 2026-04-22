import { getTask, deleteTask, exportTask as requestExportTask, authorizeExportLink, syncExportStatus as requestSyncExportStatus } from '../../services/task';
import { listSubmissions, deleteSubmission } from '../../services/submission';
import { showError, showLoading, hideLoading } from '../../utils/request';
import { formatTime, isEffectiveTime } from '../../utils/time';
import { getTimeRemaining, isTaskActive } from '../../utils/format';
import { drawQrCode } from '../../utils/qrcode';
import { isTaskAIAnalysisEnabled } from '../../utils/task';
const PAGE_SIZE = 20;
const QR_CANVAS_ID = 'taskQrCanvas';
const QR_CANVAS_SIZE = 360;
let exportStatusTimer = 0;

function getTaskStatus(task: any): string {
  if (!task) return '';
  if (task.enabled === false) return '已停用';

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
  if (hasEndTime && isTaskActive(task.start_time, task.end_time)) {
    return getTimeRemaining(task.end_time);
  }

  return '';
}

function canExportTask(task: any): boolean {
  if (!task || !isEffectiveTime(task.end_time)) {
    return false;
  }

  return new Date().getTime() > new Date(task.end_time).getTime();
}

function formatSubmissions(list: any[], customFields: any[]) {
  const fieldLabelMap: Record<string, string> = {};
  (customFields || []).forEach((f: any) => {
    fieldLabelMap[f.id] = f.label;
  });
  return list.map((s: any) => {
    const customDataList = Object.keys(s.custom_data || {}).map((key: string) => ({
      label: fieldLabelMap[key] || key,
      value: Array.isArray(s.custom_data[key]) ? s.custom_data[key].join('、') : s.custom_data[key]
    }));
    return {
      ...s,
      createdAtFormatted: s.created_at ? formatTime(String(s.created_at)) : '',
      customDataList
    };
  });
}

function getCustomFieldSummary(customFields: any[]): string {
  const labels = (customFields || []).map((field: any) => String(field.label || '').trim()).filter(Boolean);
  return labels.join('、');
}

function getDefaultExportTemplate(task: any): string {
  const customFields = (task && task.custom_fields) || [];
  if (customFields.length > 0 && customFields[0].label) {
    return `{index}_{field:${customFields[0].label}}_{nick_name}`;
  }
  return '{index}_{nick_name}';
}

function getExportTemplateHint(task: any): string {
  const tokens = ['{index}', '{nick_name}', '{created_at}', '{task_title}'];
  const customFields = ((task && task.custom_fields) || []).slice(0, 3);

  customFields.forEach((field: any) => {
    if (field && field.label) {
      tokens.push(`{field:${field.label}}`);
    }
  });

  return `可用变量：${tokens.join(' ')}，不要写扩展名，系统会自动补原图后缀`;
}

function getTaskExportInfo(task: any) {
  return (task && task.export_info) || {};
}

function normalizeExportStatus(exportInfo: any): string {
  if (!exportInfo) return '';
  if (exportInfo.status) return String(exportInfo.status);
  if (exportInfo.file_name) return 'processing';
  return '';
}

function getExportStatusText(status: string): string {
  if (status === 'success') return '已完成';
  if (status === 'failed') return '导出失败';
  if (status === 'pending') return '排队中';
  if (status === 'processing') return '处理中';
  return '';
}

function canAuthorizeExportLink(availableUntil: string): boolean {
  if (!isEffectiveTime(availableUntil)) {
    return true;
  }

  return new Date(availableUntil).getTime() > Date.now();
}

function buildExportState(task: any) {
  const exportInfo = getTaskExportInfo(task);
  const exportStatus = normalizeExportStatus(exportInfo);
  const availableUntil = String(exportInfo.available_until || '');
  return {
    exportStatus,
    exportStatusText: getExportStatusText(exportStatus),
    exportTemplate: exportInfo.filename_template || getDefaultExportTemplate(task),
    exportTemplateHint: getExportTemplateHint(task),
    exportFileName: exportInfo.file_name || '',
    exportCount: Number(exportInfo.count || 0),
    exportAvailableUntil: isEffectiveTime(availableUntil) ? formatTime(availableUntil) : '',
    canAuthorizeExportLink: canAuthorizeExportLink(availableUntil),
    exportErrorMessage: exportInfo.error_message || ''
  };
}

function mergeTaskExportInfo(task: any, exportInfo: any) {
  if (!task) return task;
  return {
    ...task,
    export_info: {
      ...getTaskExportInfo(task),
      ...exportInfo
    }
  };
}

function buildTaskQrContent(taskId: string): string {
  return `photo-task:${taskId}`;
}

function ensureAlbumPermission(): Promise<boolean> {
  return new Promise((resolve) => {
    wx.getSetting({
      success: (res) => {
        const authSetting = res.authSetting || {};
        const hasPermission = authSetting['scope.writePhotosAlbum'];

        if (hasPermission === true) {
          resolve(true);
          return;
        }

        const openSettingModal = () => {
          wx.showModal({
            title: '需要相册权限',
            content: '保存二维码到本地需要相册权限，请前往设置开启。',
            confirmText: '去设置',
            success: (modalRes) => {
              if (!modalRes.confirm) {
                resolve(false);
                return;
              }

              wx.openSetting({
                success: (settingRes) => {
                  const opened = settingRes.authSetting && settingRes.authSetting['scope.writePhotosAlbum'];
                  resolve(!!opened);
                },
                fail: () => resolve(false)
              });
            },
            fail: () => resolve(false)
          });
        };

        if (hasPermission === false) {
          openSettingModal();
          return;
        }

        wx.authorize({
          scope: 'scope.writePhotosAlbum',
          success: () => resolve(true),
          fail: () => openSettingModal()
        });
      },
      fail: () => resolve(false)
    });
  });
}

function canvasToTempFilePath(page: any): Promise<string> {
  return new Promise((resolve, reject) => {
    wx.canvasToTempFilePath({
      canvasId: QR_CANVAS_ID,
      width: QR_CANVAS_SIZE,
      height: QR_CANVAS_SIZE,
      destWidth: QR_CANVAS_SIZE * 3,
      destHeight: QR_CANVAS_SIZE * 3,
      fileType: 'png',
      success: (res) => resolve(res.tempFilePath),
      fail: (err) => reject(err)
    }, page);
  });
}

function saveImageToAlbum(filePath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    wx.saveImageToPhotosAlbum({
      filePath,
      success: () => resolve(),
      fail: (err) => reject(err)
    });
  });
}

Page({
  data: {
    taskId: '',
    task: null as any,
    taskStatusText: '',
    customFieldSummary: '',
    taskQrContent: '',
    exportTemplate: '',
    exportTemplateHint: '',
    exportStatus: '',
    exportStatusText: '',
    exportFileName: '',
    exportDownloadUrl: '',
    exportExpiresAt: '',
    exportAvailableUntil: '',
    canAuthorizeExportLink: true,
    exportCount: 0,
    exportErrorMessage: '',
    aiAnalysisEnabled: true,
    submissions: [] as any[],
    startTime: '',
    endTime: '',
    isCreator: false,
    canExportTask: false,
    fromShare: false,
    currentUserId: '',
    mySubmissionId: '',
    // 分页
    page: 1,
    hasMore: true,
    loadingMore: false,
    total: 0
  },

  onLoad(options: any) {
    this.setData({
      taskId: options.id,
      fromShare: options.fromShare === '1'
    });

    this.loadData();
  },

  onShow() {
    if (this.data.taskId) {
      // 刷新时重置到第一页
      this.setData({ page: 1, submissions: [], hasMore: true });
      this.loadData();
    }
  },

  onHide() {
    this.stopExportStatusPolling();
  },

  onUnload() {
    this.stopExportStatusPolling();
  },

  // 滚动到底部自动加载更多
  onReachBottom() {
    if (this.data.hasMore && !this.data.loadingMore) {
      this.loadMoreSubmissions();
    }
  },

  async loadData() {
    try {
      const [task, result] = await Promise.all([
        getTask(this.data.taskId),
        listSubmissions(this.data.taskId, 1, PAGE_SIZE)
      ]);

      const startTime = formatTime(String(task.start_time || ''));
      const endTime = formatTime(String(task.end_time || ''));

      const currentOpenid = wx.getStorageSync('openid') || '';
      const isCreator = task.user_id === currentOpenid;

      const list = (result && result.list) || [];
      const customFields: any[] = (task && task.custom_fields) || [];
      const formattedSubmissions = formatSubmissions(list, customFields);
      const customFieldSummary = getCustomFieldSummary(customFields);
      const total = (result && result.total) || 0;
      const exportState = buildExportState(task);

      const mySubmission = list.find((s: any) => s.user_id === currentOpenid);
      this.setData({
        task,
        taskStatusText: getTaskStatus(task),
        customFieldSummary,
        taskQrContent: buildTaskQrContent(this.data.taskId),
        ...exportState,
        exportDownloadUrl: '',
        exportExpiresAt: '',
        submissions: formattedSubmissions,
        startTime,
        endTime,
        isCreator,
        aiAnalysisEnabled: isTaskAIAnalysisEnabled(task),
        canExportTask: canExportTask(task),
        currentUserId: currentOpenid,
        mySubmissionId: (mySubmission && mySubmission.id) || '',
        page: 1,
        total,
        hasMore: (result && result.has_more) || false
      });

      if (exportState.exportStatus === 'processing' || exportState.exportStatus === 'pending') {
        this.startExportStatusPolling();
      } else {
        this.stopExportStatusPolling();
      }
      this.clearAuthorizedExportLink();
      if (exportState.exportStatus === 'success' && exportState.canAuthorizeExportLink) {
        this.fetchAuthorizedExportLink(true);
      }
    } catch (err: any) {
      showError(err.message || '加载失败');
    }
  },

  async loadMoreSubmissions() {
    if (!this.data.hasMore || this.data.loadingMore) return;

    const nextPage = this.data.page + 1;
    this.setData({ loadingMore: true });

    try {
      const result = await listSubmissions(this.data.taskId, nextPage, PAGE_SIZE);
      const list = (result && result.list) || [];
      const more = formatSubmissions(list, (this.data.task && this.data.task.custom_fields) || []);

      this.setData({
        submissions: [...this.data.submissions, ...more],
        page: nextPage,
        hasMore: (result && result.has_more) || false,
        loadingMore: false
      });
    } catch (err: any) {
      this.setData({ loadingMore: false });
      showError(err.message || '加载失败');
    }
  },

  onExportTemplateInput(e: any) {
    this.setData({
      exportTemplate: e.detail.value
    });
  },

  stopExportStatusPolling() {
    if (exportStatusTimer) {
      clearTimeout(exportStatusTimer);
      exportStatusTimer = 0;
    }
  },

  startExportStatusPolling() {
    this.stopExportStatusPolling();
    exportStatusTimer = setTimeout(() => {
      exportStatusTimer = 0;
      this.syncExportStatus(true);
    }, 3000) as unknown as number;
  },

  clearAuthorizedExportLink() {
    this.setData({
      exportDownloadUrl: '',
      exportExpiresAt: ''
    });
  },

  applyExportResult(result: any, extra: any = {}) {
    const nextTask = mergeTaskExportInfo(this.data.task, {
      status: result.status || '',
      filename_template: extra.filename_template || getTaskExportInfo(this.data.task).filename_template || this.data.exportTemplate,
      file_name: result.file_name,
      count: Number(result.count || 0),
      available_until: result.available_until || getTaskExportInfo(this.data.task).available_until || '',
      error_message: result.error_message || '',
      exported_at: extra.exported_at || getTaskExportInfo(this.data.task).exported_at || ''
    });

    this.setData({
      task: nextTask,
      ...buildExportState(nextTask)
    });

    const status = String(result.status || '');
    if (status === 'processing' || status === 'pending') {
      this.clearAuthorizedExportLink();
      this.startExportStatusPolling();
    } else {
      this.stopExportStatusPolling();
      if (status !== 'success') {
        this.clearAuthorizedExportLink();
      }
    }
  },

  applyAuthorizedExportLink(result: any) {
    const nextTask = mergeTaskExportInfo(this.data.task, {
      available_until: result.available_until || getTaskExportInfo(this.data.task).available_until || ''
    });
    this.setData({
      task: nextTask,
      exportDownloadUrl: result.download_url || '',
      exportExpiresAt: isEffectiveTime(String(result.expires_at || '')) ? formatTime(String(result.expires_at || '')) : '',
      exportAvailableUntil: isEffectiveTime(String(result.available_until || '')) ? formatTime(String(result.available_until || '')) : this.data.exportAvailableUntil,
      canAuthorizeExportLink: canAuthorizeExportLink(String(result.available_until || ''))
    });
  },

  async fetchAuthorizedExportLink(silent: boolean = false) {
    if (!this.data.isCreator || !this.data.canExportTask || !this.data.exportFileName || this.data.exportStatus !== 'success' || !this.data.canAuthorizeExportLink) {
      return;
    }

    try {
      const result = await authorizeExportLink(this.data.taskId);
      this.applyAuthorizedExportLink(result);
      if (!silent) {
        wx.showToast({
          title: '链接已更新',
          icon: 'success'
        });
      }
    } catch (err: any) {
      this.clearAuthorizedExportLink();
      if (!silent) {
        showError(err.message || '生成链接失败');
      }
    }
  },

  async syncExportStatus(silent: boolean = false) {
    if (!this.data.isCreator || !this.data.canExportTask || !this.data.exportFileName) {
      return;
    }

    try {
      const result = await requestSyncExportStatus(this.data.taskId);
      const prevStatus = this.data.exportStatus;
      this.applyExportResult(result);

      if (result.status === 'success') {
        await this.fetchAuthorizedExportLink(true);
        if (prevStatus !== 'success' && !silent) {
          wx.showToast({
            title: '导出已完成',
            icon: 'success'
          });
        }
      }

      if (result.status === 'failed' && result.error_message && !silent) {
        showError(result.error_message);
      }
    } catch (err: any) {
      this.stopExportStatusPolling();
      if (!silent) {
        showError(err.message || '刷新导出状态失败');
      }
    }
  },

  refreshExportStatus() {
    if (!this.data.canExportTask) {
      showError('活动结束后才能导出');
      return;
    }
    this.syncExportStatus(false);
  },

  async saveTaskQr() {
    if (!this.data.taskQrContent) {
      showError('任务二维码内容为空');
      return;
    }

    const hasPermission = await ensureAlbumPermission();
    if (!hasPermission) {
      showError('未获得保存到相册权限');
      return;
    }

    wx.showLoading({
      title: '生成中...',
      mask: true
    });

    try {
      await drawQrCode(QR_CANVAS_ID, this.data.taskQrContent, QR_CANVAS_SIZE, this);
      const tempFilePath = await canvasToTempFilePath(this);
      await saveImageToAlbum(tempFilePath);
      wx.showToast({
        title: '二维码已保存',
        icon: 'success'
      });
    } catch (err: any) {
      showError((err && err.errMsg) || '保存二维码失败');
    } finally {
      wx.hideLoading();
    }
  },

  async exportTask() {
    if (!this.data.isCreator) {
      showError('只有创建者可导出');
      return;
    }
    if (!this.data.canExportTask) {
      showError('活动结束后才能导出');
      return;
    }

    const template = String(this.data.exportTemplate || '').trim() || getDefaultExportTemplate(this.data.task);

    try {
      showLoading('导出中...');
      const result = await requestExportTask(this.data.taskId, {
        filename_template: template
      });
      hideLoading();

      this.applyExportResult(result, {
        filename_template: template,
        exported_at: result.status === 'success' ? new Date().toISOString() : ''
      });
      if (result.status === 'success') {
        await this.fetchAuthorizedExportLink(true);
      }

      wx.showToast({
        title: result.status === 'success' ? '导出完成' : '已开始导出',
        icon: 'success'
      });
    } catch (err: any) {
      hideLoading();
      showError(err.message || '导出失败');
    }
  },

  copyExportLink() {
    if (!this.data.canExportTask) {
      showError('活动结束后才能导出');
      return;
    }
    if (this.data.exportStatus !== 'success') {
      showError('导出未完成，请先刷新状态');
      return;
    }
    if (!this.data.exportDownloadUrl) {
      showError(this.data.exportFileName ? '链接已失效，请先重新授权' : '暂无可复制的下载链接');
      return;
    }

    wx.setClipboardData({
      data: this.data.exportDownloadUrl,
      success: () => {
        wx.showToast({
          title: '链接已复制',
          icon: 'success'
        });
      }
    });
  },

  async authorizeTaskExport() {
    if (!this.data.isCreator) {
      showError('只有创建者可操作');
      return;
    }
    if (!this.data.canExportTask) {
      showError('活动结束后才能导出');
      return;
    }
    if (!this.data.exportFileName) {
      showError('请先完成导出');
      return;
    }
    if (this.data.exportStatus !== 'success') {
      showError('导出未完成，请先刷新状态');
      return;
    }

    try {
      showLoading('生成链接中...');
      const result = await authorizeExportLink(this.data.taskId);
      hideLoading();

      this.applyAuthorizedExportLink(result);

      wx.showToast({
        title: '链接已更新',
        icon: 'success'
      });
    } catch (err: any) {
      hideLoading();
      showError(err.message || '生成链接失败');
    }
  },

  goToUpload() {
    if (!this.data.isCreator && this.data.mySubmissionId) {
      wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${this.data.taskId}&submissionId=${this.data.mySubmissionId}` });
    } else {
      wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${this.data.taskId}` });
    }
  },

  editSubmission(e: any) {
    const submissionId = e.currentTarget.dataset.id;
    wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${this.data.taskId}&submissionId=${submissionId}` });
  },

  editTask() {
    wx.navigateTo({ url: `/pages/task-create/task-create?id=${this.data.taskId}` });
  },

  deleteActivity() {
    wx.showModal({
      title: '确认删除',
      content: '删除后活动及所有提交记录将无法恢复，确认删除？',
      confirmText: '删除',
      confirmColor: '#ff4444',
      success: async (res) => {
        if (!res.confirm) return;
        try {
          await deleteTask(this.data.taskId);
          wx.showToast({ title: '删除成功', icon: 'success' });
          setTimeout(() => wx.navigateBack(), 1500);
        } catch (err: any) {
          wx.showToast({ title: err.message || '删除失败', icon: 'none' });
        }
      }
    });
  },

  deleteSubmissionRecord(e: any) {
    const submissionId = e.currentTarget.dataset.id;
    if (!submissionId || !this.data.isCreator) {
      return;
    }

    wx.showModal({
      title: '确认删除',
      content: '删除后该提交记录将无法恢复，确认删除？',
      confirmText: '删除',
      confirmColor: '#ff4444',
      success: async (res) => {
        if (!res.confirm) return;

        try {
          showLoading('删除中...');
          await deleteSubmission(submissionId);
          hideLoading();
          wx.showToast({ title: '删除成功', icon: 'success' });
          this.setData({ page: 1, submissions: [], hasMore: true });
          this.loadData();
        } catch (err: any) {
          hideLoading();
          showError(err.message || '删除失败');
        }
      }
    });
  }
});
