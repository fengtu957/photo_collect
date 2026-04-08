import { listTasks } from '../../services/task';
import { showError } from '../../utils/request';

function formatTime(iso: string) {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

Page({
  data: { tasks: [] as any[] },

  onLoad() { this.loadTasks(); },
  onShow() { this.loadTasks(); },

  async loadTasks() {
    try {
      const tasks = await listTasks();
      const formatted = (tasks || []).map((t: any) => ({
        ...t,
        created_at: formatTime(t.created_at),
        end_time: formatTime(t.end_time),
        start_time: formatTime(t.start_time),
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

