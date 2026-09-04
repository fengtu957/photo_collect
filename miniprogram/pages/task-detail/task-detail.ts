import { getTask, downloadTaskMiniProgramCode } from '../../services/task';
import { listSubmissions, deleteSubmission } from '../../services/submission';
import { showError, showLoading, hideLoading } from '../../utils/request';
import { formatTime, isEffectiveTime } from '../../utils/time';
import { getTimeRemaining, isTaskActive } from '../../utils/format';
import { isTaskAIAnalysisEnabled } from '../../utils/task';
import { buildTaskSubmissionLimitText } from '../../utils/display-limit';
const PAGE_SIZE = 20;
const TASK_SHARE_IMAGE_URL = '/imgs/invited_2.png';

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

function buildTaskSharePath(taskId: string): string {
  return `/pages/task-detail/task-detail?id=${taskId}&fromShare=1`;
}

function buildTaskShareQuery(taskId: string): string {
  return `id=${taskId}&fromShare=1`;
}

function buildTaskShareTitle(task: any): string {
  if (!task) {
    return '邀请你参与照片采集';
  }

  const title = String(task.title || '').trim();
  const taskCode = String(task.task_code || '').trim();

  if (title) {
    return `邀请你参与「${title}」照片采集`;
  }
  if (taskCode) {
    return `邀请你参与照片采集（任务码 ${taskCode}）`;
  }

  return '邀请你参与照片采集';
}

function normalizeTaskId(value: any): string {
  const taskId = String(value || '').trim();
  return /^[a-fA-F0-9]{24}$/.test(taskId) ? taskId : '';
}

function extractTaskIdFromScene(sceneValue: any): string {
  const rawScene = String(sceneValue || '').trim();
  let decodedScene = rawScene;

  for (let i = 0; i < 2; i++) {
    try {
      const nextValue = decodeURIComponent(decodedScene);
      if (!nextValue || nextValue === decodedScene) {
        break;
      }
      decodedScene = nextValue;
    } catch (err) {
      break;
    }
  }

  if (!decodedScene) {
    return '';
  }

  const directTaskId = normalizeTaskId(decodedScene);
  if (directTaskId) {
    return directTaskId;
  }

  const queryMatch = decodedScene.match(/(?:^|[?&])id=([a-fA-F0-9]{24})/i);
  if (queryMatch && queryMatch[1]) {
    return normalizeTaskId(queryMatch[1]);
  }

  const prefixedMatch = decodedScene.match(/^task:([a-fA-F0-9]{24})$/i);
  if (prefixedMatch && prefixedMatch[1]) {
    return normalizeTaskId(prefixedMatch[1]);
  }

  return '';
}

function getRuntimeEntryOptions(): WechatMiniprogram.LaunchOptionsApp | null {
  try {
    if (typeof wx.getEnterOptionsSync === 'function') {
      return wx.getEnterOptionsSync();
    }
  } catch (err) {}

  try {
    if (typeof wx.getLaunchOptionsSync === 'function') {
      return wx.getLaunchOptionsSync();
    }
  } catch (err) {}

  return null;
}

function getStoredEntryOptions() {
  try {
    const appInstance = getApp<any>();
    return (appInstance && appInstance.globalData && appInstance.globalData.entryOptions) || null;
  } catch (err) {
    return null;
  }
}

function getTaskIdFromQuery(query: any): string {
  if (!query) {
    return '';
  }

  return normalizeTaskId(query.id) || extractTaskIdFromScene(query.scene);
}

function resolveTaskDetailEntry(options: any) {
  const runtimeEntryOptions = getRuntimeEntryOptions();
  const storedEntryOptions = getStoredEntryOptions();
  const runtimeQuery = (runtimeEntryOptions && runtimeEntryOptions.query) || {};
  const storedQuery = (storedEntryOptions && storedEntryOptions.query) || {};
  const taskIdFromPage = normalizeTaskId(options && options.id);
  const taskIdFromPageScene = extractTaskIdFromScene(options && options.scene);
  const taskIdFromRuntimeQuery = getTaskIdFromQuery(runtimeQuery);
  const taskIdFromStoredQuery = getTaskIdFromQuery(storedQuery);
  const taskId = taskIdFromPage || taskIdFromPageScene || taskIdFromRuntimeQuery || taskIdFromStoredQuery;
  const fromShare = !!taskIdFromPageScene
    || !!extractTaskIdFromScene(runtimeQuery.scene)
    || !!extractTaskIdFromScene(storedQuery.scene)
    || !!(options && options.fromShare === '1')
    || !!(runtimeQuery && runtimeQuery.fromShare === '1')
    || !!(storedQuery && storedQuery.fromShare === '1');

  return {
    taskId,
    fromShare,
    debugInfo: {
      pageOptions: options || {},
      runtimePath: (runtimeEntryOptions && runtimeEntryOptions.path) || '',
      runtimeQuery,
      storedPath: (storedEntryOptions && storedEntryOptions.path) || '',
      storedQuery,
      taskIdFromPage,
      taskIdFromPageScene,
      taskIdFromRuntimeQuery,
      taskIdFromStoredQuery
    }
  };
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
            content: '保存小程序码到本地需要相册权限，请前往设置开启。',
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
    submissionLimitText: buildTaskSubmissionLimitText(null),
    aiAnalysisEnabled: true,
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
    total: 0,
    pageLoading: true,
    bootstrapping: false
  },

  async onLoad(options: any) {
    const entry = resolveTaskDetailEntry(options);
    const taskId = entry.taskId;

    console.log('task-detail entry:', JSON.stringify(entry.debugInfo));

    this.setData({
      taskId,
      fromShare: entry.fromShare,
      bootstrapping: true
    });

    wx.showShareMenu({
      withShareTicket: true,
      menus: ['shareAppMessage', 'shareTimeline']
    });

    if (!taskId) {
      this.setData({ pageLoading: false, bootstrapping: false });
      showError('任务参数无效');
      return;
    }

    try {
      const appInstance = getApp<any>();
      if (appInstance && typeof appInstance.autoLogin === 'function') {
        await appInstance.autoLogin();
      }
    } catch (err) {}

    this.setData({ bootstrapping: false });
    this.loadData();
  },

  onShow() {
    if (this.data.taskId && !this.data.bootstrapping) {
      // 刷新时重置到第一页
      this.setData({ page: 1, submissions: [], hasMore: true, pageLoading: true });
      this.loadData();
    }
  },

  onHide() {
  },

  onUnload() {
  },

  // 滚动到底部自动加载更多
  onReachBottom() {
    if (this.data.hasMore && !this.data.loadingMore) {
      this.loadMoreSubmissions();
    }
  },

  async loadData() {
    this.setData({ pageLoading: true });

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
        submissionLimitText: buildTaskSubmissionLimitText(task),
        submissions: formattedSubmissions,
        startTime,
        endTime,
        isCreator,
        aiAnalysisEnabled: isTaskAIAnalysisEnabled(task),
        currentUserId: currentOpenid,
        mySubmissionId: (mySubmission && mySubmission.id) || '',
        page: 1,
        total,
        hasMore: (result && result.has_more) || false,
        pageLoading: false
      });
    } catch (err: any) {
      this.setData({ pageLoading: false });
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

  async saveTaskMiniCode() {
    if (!this.data.taskId) {
      showError('任务参数无效');
      return;
    }

    const hasPermission = await ensureAlbumPermission();
    if (!hasPermission) {
      showError('未获得保存到相册权限');
      return;
    }

    wx.showLoading({
      title: '获取中...',
      mask: true
    });

    try {
      const tempFilePath = await downloadTaskMiniProgramCode(this.data.taskId);
      await saveImageToAlbum(tempFilePath);
      wx.showToast({
        title: '小程序码已保存',
        icon: 'success'
      });
    } catch (err: any) {
      showError((err && err.message) || '保存小程序码失败');
    } finally {
      wx.hideLoading();
    }
  },

  goToExports() {
    if (!this.data.isCreator) {
      showError('只有创建者可导出');
      return;
    }
    wx.navigateTo({ url: `/pages/task-export/task-export?taskId=${this.data.taskId}` });
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

  onShareAppMessage() {
    return {
      title: buildTaskShareTitle(this.data.task),
      path: buildTaskSharePath(this.data.taskId),
      imageUrl: TASK_SHARE_IMAGE_URL
    };
  },

  onShareTimeline() {
    return {
      title: buildTaskShareTitle(this.data.task),
      query: buildTaskShareQuery(this.data.taskId),
      imageUrl: TASK_SHARE_IMAGE_URL
    };
  },

  copyTask() {
    wx.navigateTo({ url: `/pages/task-create/task-create?copyFrom=${this.data.taskId}` });
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
