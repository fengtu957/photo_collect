import { exportTask, getTask } from '../../services/task';
import { showError, showLoading, hideLoading } from '../../utils/request';
import { isEffectiveTime } from '../../utils/time';

function getDefaultExportTemplate(task: any): string {
  const customFields = (task && task.custom_fields) || [];
  if (customFields.length > 0 && customFields[0].label) {
    return `{index}_{field:${customFields[0].label}}_{nick_name}`;
  }
  return '{index}_{nick_name}';
}

function getExportTemplateHint(task: any): string {
  const tokens = ['{index}', '{nick_name}', '{created_at}', '{task_title}'];
  const customFields = ((task && task.custom_fields) || []).slice(0, 3);
  customFields.forEach((field: any) => {
    if (field && field.label) {
      tokens.push(`{field:${field.label}}`);
    }
  });
  return `可用变量：${tokens.join(' ')}，不要写扩展名，系统会自动补原图后缀`;
}

Page({
  data: {
    taskId: '',
    task: null as any,
    taskTitle: '',
    exportTemplate: '',
    exportTemplateHint: '',
    pageLoading: true,
    submitting: false
  },

  onLoad(options: any) {
    const taskId = String(options && options.taskId || '');
    this.setData({ taskId });
    if (!taskId) {
      this.setData({ pageLoading: false });
      showError('任务参数无效');
      return;
    }
    this.loadTask();
  },

  async loadTask() {
    try {
      const task = await getTask(this.data.taskId);
      const currentUserId = String(wx.getStorageSync('openid') || '');
      if (!task || task.user_id !== currentUserId) {
        throw new Error('只有创建者可导出');
      }
      if (!isEffectiveTime(task.end_time) || Date.now() <= new Date(task.end_time).getTime()) {
        throw new Error('任务结束后才能导出');
      }
      this.setData({
        task,
        taskTitle: task.title || '',
        exportTemplate: String(task.export_info && task.export_info.filename_template || '').trim() || getDefaultExportTemplate(task),
        exportTemplateHint: getExportTemplateHint(task),
        pageLoading: false
      });
    } catch (err: any) {
      this.setData({ pageLoading: false });
      showError(err.message || '加载任务失败');
    }
  },

  onExportTemplateInput(e: any) {
    this.setData({ exportTemplate: e.detail.value || '' });
  },

  async submitExport() {
    if (this.data.submitting || !this.data.task) {
      return;
    }
    const template = String(this.data.exportTemplate || '').trim() || getDefaultExportTemplate(this.data.task);
    this.setData({ submitting: true });
    try {
      showLoading('创建导出任务...');
      await exportTask(this.data.taskId, { filename_template: template });
      hideLoading();
      wx.showToast({ title: '已开始导出', icon: 'success' });
      setTimeout(() => wx.navigateBack(), 800);
    } catch (err: any) {
      hideLoading();
      this.setData({ submitting: false });
      showError('导出失败');
    }
  }
});
