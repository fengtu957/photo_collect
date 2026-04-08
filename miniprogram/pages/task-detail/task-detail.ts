import { getTask } from '../../services/task';
import { listSubmissions } from '../../services/submission';
import { showError } from '../../utils/request';

Page({
  data: {
    taskId: '',
    task: {} as any,
    submissions: [] as any[]
  },

  onLoad(options: any) {
    this.setData({ taskId: options.id });
    this.loadData();
  },

  async loadData() {
    try {
      const [task, submissions] = await Promise.all([
        getTask(this.data.taskId),
        listSubmissions(this.data.taskId)
      ]);
      this.setData({ task, submissions });
    } catch (err: any) {
      showError(err.message || '加载失败');
    }
  },

  goToUpload() {
    wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${this.data.taskId}` });
  }
});
