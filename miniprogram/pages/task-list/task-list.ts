import { listTasks } from '../../services/task';
import { getUserEntitlements } from '../../services/entitlement';
import { showError } from '../../utils/request';
import { formatTime, isEffectiveTime } from '../../utils/time';
import {
  buildActiveTaskTip,
  buildOpenDurationTip,
  buildRetentionTip,
  buildSubmissionLimitTip
} from '../../utils/display-limit';

function extractTaskIdFromScanResult(result: string): string {
  const value = String(result || '').trim();
  if (!value) return '';

  const directMatch = value.match(/^[a-fA-F0-9]{24}$/);
  if (directMatch) {
    return directMatch[0];
  }

  const customSchemeMatch = value.match(/photo-task:([a-fA-F0-9]{24})/i);
  if (customSchemeMatch && customSchemeMatch[1]) {
    return customSchemeMatch[1];
  }

  const queryMatch = value.match(/[?&]id=([a-fA-F0-9]{24})/i);
  if (queryMatch && queryMatch[1]) {
    return queryMatch[1];
  }

  const pathMatch = value.match(/\/pages\/task-detail\/task-detail\?id=([a-fA-F0-9]{24})/i);
  if (pathMatch && pathMatch[1]) {
    return pathMatch[1];
  }

  return '';
}

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
  data: {
    tasks: [] as any[],
    createTip: buildActiveTaskTip(null),
    submissionTip: buildSubmissionLimitTip(null),
    durationTip: buildOpenDurationTip(null),
    retentionTip: buildRetentionTip(null)
  },

  onLoad() { this.loadTasks(); },
  onShow() { this.loadTasks(); },

  async loadTasks() {
    try {
      const tasks = await listTasks();
      let entitlements = null;
      try {
        entitlements = await getUserEntitlements();
        const appInstance = getApp<any>();
        appInstance.globalData.entitlements = entitlements;
      } catch (err) {}
      const formatted = (tasks || []).map((t: any) => ({
        ...t,
        status: getTaskStatus(t),
        spec_text: (t && t.photo_spec && t.photo_spec.name)
          ? `${t.photo_spec.name}${(t.photo_spec && t.photo_spec.background_color) ? '，' + t.photo_spec.background_color : ''}${t.description ? '，' + t.description : ''}`
          : (t.description || '未设置'),
        created_at_formatted: formatTime(String(t.created_at || '')),
        end_time_formatted: formatTime(String(t.end_time || '')),
        start_time_formatted: formatTime(String(t.start_time || '')),
      }));
      this.setData({
        tasks: formatted,
        createTip: buildActiveTaskTip(entitlements),
        submissionTip: buildSubmissionLimitTip(entitlements),
        durationTip: buildOpenDurationTip(entitlements),
        retentionTip: buildRetentionTip(entitlements)
      });
    } catch (err: any) {
      showError(err.message || '加载失败');
    }
  },

  goToCreate() {
    wx.navigateTo({ url: '/pages/task-create/task-create' });
  },

  scanTask() {
    wx.scanCode({
      onlyFromCamera: false,
      scanType: ['qrCode'],
      success: (res) => {
        const taskId = extractTaskIdFromScanResult(res.result || '');
        if (!taskId) {
          showError('未识别到有效任务二维码');
          return;
        }

        wx.navigateTo({ url: `/pages/task-detail/task-detail?id=${taskId}&fromShare=1` });
      },
      fail: (err) => {
        if (err && err.errMsg && err.errMsg.indexOf('cancel') >= 0) {
          return;
        }
        showError('扫码失败，请重试');
      }
    });
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
