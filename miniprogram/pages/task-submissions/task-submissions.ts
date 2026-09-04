import { getTask } from '../../services/task';
import { listSubmissions } from '../../services/submission';
import { showError } from '../../utils/request';
import { formatTime } from '../../utils/time';

const PAGE_SIZE = 20;

function formatSubmissions(list: any[], customFields: any[]) {
  const visibleFields = (customFields || [])
    .filter((field: any) => String(field.label || '').trim())
    .slice(0, 5);
  return (list || []).map((submission: any) => {
    const customFieldItems = visibleFields.map((field: any, index: number) => {
      const rawValue = submission.custom_data ? submission.custom_data[field.id] : '';
      const value = Array.isArray(rawValue) ? rawValue.join('、') : String(rawValue || '').trim();
      return {
        id: String(field.id || index),
        label: String(field.label || '').trim(),
        value: value || '未填写'
      };
    });
    return {
      ...submission,
      createdAtFormatted: submission.created_at ? formatTime(String(submission.created_at)) : '',
      customFieldItems
    };
  });
}

function preserveSubmissionPhotoURLs(nextList: any[], previousList: any[]) {
  const previousById: Record<string, any> = {};
  (previousList || []).forEach((item: any) => {
    previousById[String(item.id || '')] = item;
  });

  return (nextList || []).map((item: any) => {
    const previous = previousById[String(item.id || '')];
    if (!previous || String(previous.updated_at || '') !== String(item.updated_at || '')) {
      return item;
    }

    const previousPhoto = previous.photo || {};
    const nextPhoto = item.photo || {};
    return {
      ...item,
      photo: {
        ...nextPhoto,
        url: previousPhoto.url || nextPhoto.url || '',
        thumbnail_url: previousPhoto.thumbnail_url || nextPhoto.thumbnail_url || ''
      }
    };
  });
}

Page({
  data: {
    taskId: '',
    task: null as any,
    submissions: [] as any[],
    page: 1,
    total: 0,
    hasMore: true,
    loadingMore: false,
    pageLoading: true,
    initialized: false,
    needsRefresh: false,
    viewMode: 'list' as 'list' | 'grid'
  },

  async onLoad(options: any) {
    const taskId = String((options && options.taskId) || '').trim();
    this.setData({ taskId });
    if (!taskId) {
      this.setData({ pageLoading: false });
      showError('任务参数无效');
      return;
    }

    try {
      const appInstance = getApp<any>();
      if (appInstance && typeof appInstance.autoLogin === 'function') {
        await appInstance.autoLogin();
      }
    } catch (err) {}

    await this.loadData();
    this.setData({ initialized: true });
  },

  onShow() {
    if (!this.data.initialized || !this.data.needsRefresh) return;

    this.setData({ page: 1, hasMore: true, needsRefresh: false });
    this.loadData(false);
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loadingMore) {
      this.loadMoreSubmissions();
    }
  },

  async loadData(showPageLoading: boolean = true) {
    if (showPageLoading) {
      this.setData({ pageLoading: true });
    }

    try {
      const task = await getTask(this.data.taskId);
      const currentOpenid = String(wx.getStorageSync('openid') || '');
      if (!task || task.user_id !== currentOpenid) {
        this.setData({ pageLoading: false });
        showError('只有创建者可查看全部提交');
        setTimeout(() => wx.navigateBack(), 800);
        return;
      }

      const result = await listSubmissions(this.data.taskId, 1, PAGE_SIZE, 'all');
      const list = (result && result.list) || [];
      this.setData({
        task,
        submissions: preserveSubmissionPhotoURLs(
          formatSubmissions(list, (task && task.custom_fields) || []),
          this.data.submissions
        ),
        page: 1,
        total: (result && result.total) || 0,
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
      const result = await listSubmissions(this.data.taskId, nextPage, PAGE_SIZE, 'all');
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

  toggleViewMode() {
    this.setData({
      viewMode: this.data.viewMode === 'list' ? 'grid' : 'list'
    });
  },

  editSubmission(e: any) {
    const submissionId = String(e.currentTarget.dataset.id || '');
    if (!submissionId) return;
    this.setData({ needsRefresh: true });
    wx.navigateTo({
      url: `/pages/photo-upload/photo-upload?taskId=${this.data.taskId}&submissionId=${submissionId}`
    });
  }
});
