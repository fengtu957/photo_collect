import { getTask } from '../../services/task';
import { listSubmissions, setSubmissionStarred } from '../../services/submission';
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
      aiScoreText: submission.ai_evaluation && submission.ai_evaluation.status === 'success'
        ? `AI ${submission.ai_evaluation.score}分` : '无分析结果',
      aiNeedsAttention: !!(submission.ai_evaluation && submission.ai_evaluation.status === 'success' && submission.ai_evaluation.score < 80),
      aiIssue: submission.ai_evaluation && submission.ai_evaluation.status === 'success'
        ? (submission.ai_evaluation.issues || []).join('；') : '',
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
    starredOnly: false,
    aiNeedsAttentionOnly: false,
    listLoading: false,
    starringId: '',
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
    this.setData({ listLoading: true });
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

      const result = await listSubmissions(this.data.taskId, 1, PAGE_SIZE, 'all', this.getFilters());
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
      }, () => this.ensureGridScrollable());
    } catch (err: any) {
      this.setData({ pageLoading: false });
      showError(err.message || '加载失败');
    } finally {
      this.setData({ listLoading: false }, () => this.ensureGridScrollable());
    }
  },

  async loadMoreSubmissions() {
    if (!this.data.hasMore || this.data.loadingMore || this.data.listLoading || this.data.starringId) return;

    const nextPage = this.data.page + 1;
    this.setData({ loadingMore: true });
    try {
      const result = await listSubmissions(this.data.taskId, nextPage, PAGE_SIZE, 'all', this.getFilters());
      const list = (result && result.list) || [];
      const more = formatSubmissions(list, (this.data.task && this.data.task.custom_fields) || []);
      this.setData({
        submissions: [...this.data.submissions, ...more],
        page: nextPage,
        hasMore: (result && result.has_more) || false,
        loadingMore: false
      }, () => this.ensureGridScrollable());
    } catch (err: any) {
      this.setData({ loadingMore: false });
      showError(err.message || '加载失败');
    }
  },

  toggleViewMode() {
    this.setData({
      viewMode: this.data.viewMode === 'list' ? 'grid' : 'list'
    }, () => this.ensureGridScrollable());
  },

  getFilters() {
    return { starred: this.data.starredOnly, ai_needs_attention: this.data.aiNeedsAttentionOnly };
  },

  toggleFilter(e: any) {
    if (this.data.listLoading || this.data.loadingMore || this.data.starringId) return;
    const filter = String(e.currentTarget.dataset.filter || '');
    if (filter === 'starred') {
      this.setData({ starredOnly: !this.data.starredOnly });
    } else if (filter === 'ai') {
      this.setData({ aiNeedsAttentionOnly: !this.data.aiNeedsAttentionOnly });
    } else {
      return;
    }
    this.setData({ submissions: [], total: 0, page: 1, hasMore: false });
    this.loadData();
  },

  async toggleStar(e: any) {
    if (this.data.starringId || this.data.listLoading || this.data.loadingMore) return;
    const id = String(e.currentTarget.dataset.id || '');
    const submission = this.data.submissions.find((item: any) => item.id === id);
    if (!submission) return;
    this.setData({ starringId: id });
    try {
      const result = await setSubmissionStarred(id, !submission.starred);
      this.setData({
        submissions: this.data.submissions.map((item: any) => item.id === id ? { ...item, starred: result.starred } : item)
      });
      if (this.data.starredOnly && !result.starred) {
        this.setData({ submissions: [], total: 0, hasMore: false });
        await this.loadData(false);
      }
    } catch (err: any) {
      showError(err.message || '重点标记失败');
    } finally {
      this.setData({ starringId: '' }, () => this.ensureGridScrollable());
    }
  },

  ensureGridScrollable() {
    if (this.data.viewMode !== 'grid' || !this.data.hasMore || this.data.loadingMore) return;

    wx.nextTick(() => {
      const query = wx.createSelectorQuery().in(this);
      query.select('.page').boundingClientRect();
      query.selectViewport().boundingClientRect();
      query.exec((rects: any[]) => {
        const pageRect = rects && rects[0];
        const viewportRect = rects && rects[1];
        if (
          this.data.viewMode === 'grid' &&
          this.data.hasMore &&
          !this.data.loadingMore &&
          pageRect &&
          viewportRect &&
          Number(pageRect.height || 0) <= Number(viewportRect.height || 0)
        ) {
          this.loadMoreSubmissions();
        }
      });
    });
  },

  editSubmission(e: any) {
    const submissionId = String(e.currentTarget.dataset.id || '');
    if (!submissionId) return;
    wx.navigateTo({
      url: `/pages/photo-upload/photo-upload?taskId=${this.data.taskId}&submissionId=${submissionId}`
    });
  }
});
