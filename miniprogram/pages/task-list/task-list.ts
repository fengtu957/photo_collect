import { listTasks } from '../../services/task';
import { showError } from '../../utils/request';
import { formatTime } from '../../utils/time';

Page({
  data: { tasks: [] as any[] },

  onLoad() { this.loadTasks(); },
  onShow() { this.loadTasks(); },

  async loadTasks() {
    try {
      const tasks = await listTasks();
      const formatted = (tasks || []).map((t: any) => ({
        ...t,
        created_at_formatted: formatTime(String(t.created_at)),
        end_time_formatted: formatTime(String(t.end_time)),
        start_time_formatted: formatTime(String(t.start_time)),
      }));
      this.setData({ tasks: formatted });
    } catch (err: any) {
      showError(err.message || '加载失败');
    }
  },

  goToCreate() {
    wx.navigateTo({ url: '/pages/task-create/task-create' });
  },

  goToDetail(e: any) {
    wx.navigateTo({ url: `/pages/task-detail/task-detail?id=${e.currentTarget.dataset.id}` });
  },

  viewList(e: any) {
    wx.navigateTo({ url: `/pages/task-detail/task-detail?id=${e.currentTarget.dataset.id}` });
  },

  goToUpload(e: any) {
    wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${e.currentTarget.dataset.id}` });
  }
});

