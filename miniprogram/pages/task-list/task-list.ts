import { listTasks } from '../../services/task';
import { listSubmissions } from '../../services/submission';
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

  async goToUpload(e: any) {
    const taskId = e.currentTarget.dataset.id;
    wx.showLoading({ title: '检查中...' });
    try {
      const subs = await listSubmissions(taskId);
      wx.hideLoading();
      if (subs && subs.length > 0) {
        // 已有提交，直接进入编辑模式
        wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${taskId}&submissionId=${(subs[0] as any).id}` });
      } else {
        wx.navigateTo({ url: `/pages/photo-upload/photo-upload?taskId=${taskId}` });
      }
    } catch (err: any) {
      wx.hideLoading();
      showError(err.message || '加载失败');
    }
  }
});

