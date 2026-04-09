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
    isCreator: false
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

      // 获取当前用户openid判断是否为创建者（用于前端显示）
      const currentOpenid = wx.getStorageSync('openid') || '';
      const isCreator = task.user_id === currentOpenid;

      const formattedSubmissions = (submissions || []).map((s: any) => ({
        ...s,
        createdAtFormatted: s.created_at ? formatTime(String(s.created_at)) : ''
      }));

      this.setData({
        task,
        submissions: formattedSubmissions,
        startTime,
        endTime,
        isCreator
      });
    } catch (err: any) {
      showError(err.message || '加载失败');
    }
  },

  goToUpload() {
    wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${this.data.taskId}` });
  }
});
