import { getTaskByCode, listTasks } from '../../services/task';
import { getUserEntitlements } from '../../services/entitlement';
import { showError } from '../../utils/request';
import { formatTime, isEffectiveTime } from '../../utils/time';
import {
  buildActiveTaskTip,
  buildOpenDurationTip,
  buildRetentionTip,
  buildSubmissionLimitTip
} from '../../utils/display-limit';

const STATUS_FILTER_OPTIONS = ['全部', '进行中', '未开始', '已截止'];
const STATUS_SORT_WEIGHT: Record<string, number> = {
  '进行中': 0,
  '未开始': 1,
  '已截止': 2,
  '已关闭': 3
};

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

function getTaskSortTime(task: any): number {
  const endTime = new Date(String(task && task.end_time || '')).getTime();
  if (!isNaN(endTime) && endTime > 0) {
    return endTime;
  }

  const startTime = new Date(String(task && task.start_time || '')).getTime();
  if (!isNaN(startTime) && startTime > 0) {
    return startTime;
  }

  const createdTime = new Date(String(task && task.created_at || '')).getTime();
  if (!isNaN(createdTime) && createdTime > 0) {
    return createdTime;
  }

  return 0;
}

function buildTaskSearchText(task: any): string {
  const taskCode = String(task && task.task_code || '');
  const title = String(task && task.title || '');
  const description = String(task && task.description || '');
  const specText = String(task && task.spec_text || '');
  return `${taskCode} ${title} ${description} ${specText}`.toLowerCase();
}

function normalizeTaskCodeInput(value: any): string {
  return String(value || '').replace(/\D/g, '').slice(0, 5);
}

Page({
  data: {
    tasks: [] as any[],
    allTasks: [] as any[],
    filterOptions: STATUS_FILTER_OPTIONS,
    activeFilter: '进行中',
    searchKeyword: '',
    joinTaskCode: '',
    joiningByCode: false,
    showJoinActions: false,
    showTaskCodePanel: false,
    emptyText: '暂无任务',
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
        allTasks: formatted,
        createTip: buildActiveTaskTip(entitlements),
        submissionTip: buildSubmissionLimitTip(entitlements),
        durationTip: buildOpenDurationTip(entitlements),
        retentionTip: buildRetentionTip(entitlements)
      });
      this.applyTaskFilters();
    } catch (err: any) {
      showError(err.message || '加载失败');
    }
  },

  applyTaskFilters() {
    const activeFilter = String(this.data.activeFilter || '全部');
    const keyword = String(this.data.searchKeyword || '').trim().toLowerCase();
    const tasks = (this.data.allTasks || []).filter((task: any) => {
      const statusText = String(task && task.status && task.status.text || '');
      const searchText = buildTaskSearchText(task);
      const matchesFilter = activeFilter === '全部' || statusText === activeFilter;
      const matchesKeyword = !keyword || searchText.indexOf(keyword) >= 0;
      return matchesFilter && matchesKeyword;
    }).sort((a: any, b: any) => {
      const aStatus = String(a && a.status && a.status.text || '');
      const bStatus = String(b && b.status && b.status.text || '');
      const aWeight = STATUS_SORT_WEIGHT[aStatus];
      const bWeight = STATUS_SORT_WEIGHT[bStatus];
      const statusDiff = (typeof aWeight === 'number' ? aWeight : 99) - (typeof bWeight === 'number' ? bWeight : 99);

      if (statusDiff !== 0) {
        return statusDiff;
      }

      const aTime = getTaskSortTime(a);
      const bTime = getTaskSortTime(b);
      if (aStatus === '已截止' || aStatus === '已关闭') {
        return bTime - aTime;
      }

      return aTime - bTime;
    });

    let emptyText = '暂无任务';
    if ((this.data.allTasks || []).length > 0 && tasks.length === 0) {
      emptyText = '当前筛选下暂无任务';
    }

    this.setData({
      tasks,
      emptyText
    });
  },

  onSearchInput(e: any) {
    this.setData({
      searchKeyword: e.detail.value || ''
    });
    this.applyTaskFilters();
  },

  onJoinTaskCodeInput(e: any) {
    this.setData({
      joinTaskCode: normalizeTaskCodeInput(e.detail.value)
    });
  },

  noop() {},

  closeJoinActions() {
    this.setData({
      showJoinActions: false,
      showTaskCodePanel: false
    });
  },

  toggleJoinActions() {
    const nextValue = !this.data.showJoinActions;
    this.setData({
      showJoinActions: nextValue,
      showTaskCodePanel: nextValue ? this.data.showTaskCodePanel : false
    });
  },

  openTaskCodePanel() {
    this.setData({
      showJoinActions: true,
      showTaskCodePanel: true
    });
  },

  async joinTaskByCode() {
    const taskCode = normalizeTaskCodeInput(this.data.joinTaskCode);
    if (taskCode.length !== 5) {
      showError('请输入 5 位任务码');
      return;
    }
    if (this.data.joiningByCode) {
      return;
    }

    this.setData({ joiningByCode: true });
    try {
      const task = await getTaskByCode(taskCode);
      this.setData({
        joinTaskCode: '',
        joiningByCode: false,
        showJoinActions: false,
        showTaskCodePanel: false
      });
      wx.navigateTo({ url: `/pages/task-detail/task-detail?id=${task.id}&fromShare=1` });
    } catch (err: any) {
      this.setData({ joiningByCode: false });
      showError(err.message || '未找到对应任务');
    }
  },

  onFilterChange(e: any) {
    const value = String(e.currentTarget.dataset.value || '');
    if (!value || value === this.data.activeFilter) {
      return;
    }

    this.setData({
      activeFilter: value
    });
    this.applyTaskFilters();
  },

  goToCreate() {
    this.closeJoinActions();
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

  scanTaskFromJoin() {
    this.closeJoinActions();
    this.scanTask();
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
