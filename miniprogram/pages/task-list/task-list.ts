import { listTasks } from '../../services/task';
import { showError } from '../../utils/request';
import { formatTime, isEffectiveTime } from '../../utils/time';

function getTaskStatus(task: any) {
  const now = Date.now();
  const hasStartTime = isEffectiveTime(task && task.start_time);
  const hasEndTime = isEffectiveTime(task && task.end_time);
  const startTime = hasStartTime ? new Date(task.start_time).getTime() : 0;
  const endTime = hasEndTime ? new Date(task.end_time).getTime() : 0;

  if (task && task.enabled === false) {
    return {
      text: '已关闭',
      icon: '../../imgs/已取消.png'
    };
  }

  if (hasStartTime && now < startTime) {
    return {
      text: '未开始',
      icon: '../../imgs/未开始.png'
    };
  }

  if (hasEndTime && now > endTime) {
    return {
      text: '已截止',
      icon: '../../imgs/已截止.png'
    };
  }

  return {
    text: '进行中',
    icon: '../../imgs/进行中.png'
  };
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
        status: getTaskStatus(t),
        spec_text: (t && t.photo_spec && t.photo_spec.name)
          ? `${t.photo_spec.name}${t.description ? '，' + t.description : ''}`
          : (t.description || '未设置'),
        created_at_formatted: formatTime(String(t.created_at || '')),
        end_time_formatted: formatTime(String(t.end_time || '')),
        start_time_formatted: formatTime(String(t.start_time || '')),
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
