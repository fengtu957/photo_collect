import { getTask, deleteTask } from '../../services/task';
import { listSubmissions } from '../../services/submission';
import { showError } from '../../utils/request';
import { formatTime, isEffectiveTime } from '../../utils/time';
import { getTimeRemaining, isTaskActive } from '../../utils/format';
import { drawQrCode } from '../../utils/qrcode';
const PAGE_SIZE = 2;
const QR_CANVAS_ID = 'taskQrCanvas';
const QR_CANVAS_SIZE = 360;

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

function formatSubmissions(list: any[], customFields: any[]) {
  const fieldLabelMap: Record<string, string> = {};
  (customFields || []).forEach((f: any) => {
    fieldLabelMap[f.id] = f.label;
  });
  return list.map((s: any) => {
    const customDataList = Object.keys(s.custom_data || {}).map((key: string) => ({
      label: fieldLabelMap[key] || key,
      value: s.custom_data[key]
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
    submissions: [] as any[],
    startTime: '',
    endTime: '',
    isCreator: false,
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

      const mySubmission = list.find((s: any) => s.user_id === currentOpenid);

      this.setData({
        task,
        taskStatusText: getTaskStatus(task),
        customFieldSummary,
        taskQrContent: buildTaskQrContent(this.data.taskId),
        submissions: formattedSubmissions,
        startTime,
        endTime,
        isCreator,
        currentUserId: currentOpenid,
        mySubmissionId: (mySubmission && mySubmission.id) || '',
        page: 1,
        total,
        hasMore: (result && result.has_more) || false
      });
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
  }
});
