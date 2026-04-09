import { getTask } from '../../services/task';
import { listSubmissions } from '../../services/submission';
import { showError } from '../../utils/request';
import { formatTime } from '../../utils/time';

Page({
  data: {
    taskId: '',
    task: null as any,
    submissions: [] as any[],
    startTime: '',
    endTime: '',
    isCreator: false,
    currentUserId: '',
    mySubmissionId: ''  // 当前用户在该任务中的提交ID
  },

  onLoad(options: any) {
    this.setData({ taskId: options.id });
    this.loadData();
  },

  onShow() {
    if (this.data.taskId) {
      this.loadData();
    }
  },

  async loadData() {
    try {
      const [task, submissions] = await Promise.all([
        getTask(this.data.taskId),
        listSubmissions(this.data.taskId)
      ]);

      const startTime = task.start_time ? formatTime(String(task.start_time)) : '';
      const endTime = task.end_time ? formatTime(String(task.end_time)) : '';

      const currentOpenid = wx.getStorageSync('openid') || '';
      const isCreator = task.user_id === currentOpenid;

      const formattedSubmissions = (submissions || []).map((s: any) => ({
        ...s,
        createdAtFormatted: s.created_at ? formatTime(String(s.created_at)) : ''
      }));

      // 找出当前用户自己的提交ID
      const mySubmission = (submissions || []).find((s: any) => s.user_id === currentOpenid);

      this.setData({
        task,
        submissions: formattedSubmissions,
        startTime,
        endTime,
        isCreator,
        currentUserId: currentOpenid,
        mySubmissionId: (mySubmission && mySubmission.id) || ''
      });
    } catch (err: any) {
      showError(err.message || '加载失败');
    }
  },

  goToUpload() {
    // 创建者：始终新建；非创建者：有提交则编辑，否则新建
    if (!this.data.isCreator && this.data.mySubmissionId) {
      wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${this.data.taskId}&submissionId=${this.data.mySubmissionId}` });
    } else {
      wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${this.data.taskId}` });
    }
  },

  editSubmission(e: any) {
    const submissionId = e.currentTarget.dataset.id;
    wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${this.data.taskId}&submissionId=${submissionId}` });
  }
});
