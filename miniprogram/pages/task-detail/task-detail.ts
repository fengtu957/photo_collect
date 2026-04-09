import { getTask, deleteTask } from '../../services/task';
import { listSubmissions } from '../../services/submission';
import { showError } from '../../utils/request';
import { formatTime } from '../../utils/time';

const PAGE_SIZE = 20;

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

Page({
  data: {
    taskId: '',
    task: null as any,
    submissions: [] as any[],
    startTime: '',
    endTime: '',
    isCreator: false,
    currentUserId: '',
    mySubmissionId: '',
    // 分页
    page: 1,
    hasMore: true,
    loadingMore: false,
    total: 0
  },

  onLoad(options: any) {
    this.setData({ taskId: options.id });
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

      const startTime = task.start_time ? formatTime(String(task.start_time)) : '';
      const endTime = task.end_time ? formatTime(String(task.end_time)) : '';

      const currentOpenid = wx.getStorageSync('openid') || '';
      const isCreator = task.user_id === currentOpenid;

      const list = (result && result.list) || [];
      const customFields: any[] = (task && task.custom_fields) || [];
      const formattedSubmissions = formatSubmissions(list, customFields);

      const mySubmission = list.find((s: any) => s.user_id === currentOpenid);

      this.setData({
        task,
        submissions: formattedSubmissions,
        startTime,
        endTime,
        isCreator,
        currentUserId: currentOpenid,
        mySubmissionId: (mySubmission && mySubmission.id) || '',
        page: 1,
        total: (result && result.total) || 0,
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
