import { authorizeTaskExportLink, getTask, listTaskExports } from '../../services/task';
import { showError, showLoading, hideLoading } from '../../utils/request';
import { formatTime, isEffectiveTime } from '../../utils/time';

const PAGE_SIZE = 20;

function getExportStatusText(status: string): string {
  if (status === 'success') return '已完成';
  if (status === 'failed') return '导出失败';
  if (status === 'processing') return '处理中';
  return '排队中';
}

function normalizeExportRecord(record: any) {
  const status = String(record && record.status || (record && record.file_name ? 'processing' : 'pending'));
  const availableUntil = String(record && record.available_until || '');
  const canDownload = status === 'success'
    && (!isEffectiveTime(availableUntil) || new Date(availableUntil).getTime() > Date.now());
  return {
    ...record,
    status,
    statusText: getExportStatusText(status),
    createdAtFormatted: formatTime(String(record && record.created_at || '')),
    exportedAtFormatted: formatTime(String(record && record.exported_at || '')),
    availableUntilFormatted: formatTime(availableUntil),
    canDownload
  };
}

function canStartTaskExport(task: any): boolean {
  return !!(task
    && isEffectiveTime(task.end_time)
    && Date.now() > new Date(task.end_time).getTime());
}

Page({
  data: {
    taskId: '',
    task: null as any,
    taskTitle: '',
    records: [] as any[],
    page: 1,
    total: 0,
    hasMore: false,
    pageLoading: true,
    loadingMore: false,
    canStartExport: false
  },

  onLoad(options: any) {
    const taskId = String(options && options.taskId || '');
    this.setData({ taskId });
    if (!taskId) {
      this.setData({ pageLoading: false });
      showError('任务参数无效');
      return;
    }
    this.loadData();
  },

  onShow() {
    if (this.data.taskId && this.data.task) {
      this.loadHistory(true);
    }
  },

  onHide() {
    this.clearRefreshTimer();
  },

  onUnload() {
    this.clearRefreshTimer();
  },

  onPullDownRefresh() {
    this.loadHistory(true).finally(() => wx.stopPullDownRefresh());
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loadingMore) {
      this.loadMore();
    }
  },

  async loadData() {
    this.setData({ pageLoading: true });
    try {
      const task = await getTask(this.data.taskId);
      const currentUserId = String(wx.getStorageSync('openid') || '');
      if (!task || task.user_id !== currentUserId) {
        throw new Error('只有创建者可查看导出记录');
      }
      this.setData({
        task,
        taskTitle: task.title || '',
        canStartExport: canStartTaskExport(task)
      });
      await this.loadHistory(false);
    } catch (err: any) {
      this.setData({ pageLoading: false });
      showError(err.message || '加载导出记录失败');
    }
  },

  async loadHistory(silent: boolean = false) {
    this.clearRefreshTimer();
    try {
      const result = await listTaskExports(this.data.taskId, 1, PAGE_SIZE);
      const records = ((result && result.list) || []).map(normalizeExportRecord);
      this.setData({
        records,
        page: 1,
        total: Number(result && result.total || 0),
        hasMore: !!(result && result.has_more),
        pageLoading: false
      });
      this.scheduleRefresh(records);
    } catch (err: any) {
      this.setData({ pageLoading: false });
      if (!silent) {
        showError(err.message || '加载导出记录失败');
      }
    }
  },

  async loadMore() {
    const nextPage = this.data.page + 1;
    this.setData({ loadingMore: true });
    try {
      const result = await listTaskExports(this.data.taskId, nextPage, PAGE_SIZE);
      const records = ((result && result.list) || []).map(normalizeExportRecord);
      this.setData({
        records: [...this.data.records, ...records],
        page: nextPage,
        hasMore: !!(result && result.has_more),
        loadingMore: false
      });
    } catch (err: any) {
      this.setData({ loadingMore: false });
      showError(err.message || '加载更多失败');
    }
  },

  scheduleRefresh(records: any[]) {
    const hasRunning = (records || []).some((record: any) => record.status === 'pending' || record.status === 'processing');
    if (!hasRunning) {
      return;
    }
    (this as any).exportRefreshTimer = setTimeout(() => {
      this.loadHistory(true);
    }, 3000);
  },

  clearRefreshTimer() {
    const timer = (this as any).exportRefreshTimer;
    if (timer) {
      clearTimeout(timer);
      (this as any).exportRefreshTimer = null;
    }
  },

  goToEditor() {
    const canStartExport = canStartTaskExport(this.data.task);
    if (!canStartExport) {
      showError('任务结束后才能导出');
      return;
    }
    this.setData({ canStartExport: true });
    wx.navigateTo({ url: `/pages/task-export-edit/task-export-edit?taskId=${this.data.taskId}` });
  },

  async copyDownloadLink(e: any) {
    const exportId = String(e.currentTarget.dataset.id || '');
    if (!exportId) {
      showError('导出记录无效');
      return;
    }
    try {
      showLoading('生成链接中...');
      const result = await authorizeTaskExportLink(this.data.taskId, exportId);
      hideLoading();
      if (!result.download_url) {
        showError('下载链接生成失败');
        return;
      }
      wx.setClipboardData({
        data: result.download_url,
        success: () => wx.showToast({ title: '链接已复制', icon: 'success' })
      });
    } catch (err: any) {
      hideLoading();
      showError(err.message || '生成下载链接失败');
    }
  }
});
