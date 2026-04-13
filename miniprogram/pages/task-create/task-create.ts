import { createTask, getTask, updateTask } from '../../services/task';
import { getUserEntitlements } from '../../services/vip';
import { showError, showSuccess } from '../../utils/request';
import { isEffectiveTime, toRFC3339 } from '../../utils/time';
import { formatDate } from '../../utils/format';
import { normalizePhotoSpec } from '../../constants/photo-spec';
import { isTaskAIAnalysisEnabled } from '../../utils/task';

function isValidDateTime(value: string): boolean {
  if (!value) return false;
  return !isNaN(new Date(value).getTime());
}

function validateTaskForm(form: any): string {
  const title = String(form.title || '').trim();
  const photoSpec = normalizePhotoSpec(form.photo_spec || {});
  const photoSpecName = photoSpec.name;
  const width = photoSpec.width;
  const height = photoSpec.height;
  const maxSizeKB = photoSpec.max_size_kb;

  if (!title) {
    return '请填写任务标题';
  }
  if (!photoSpecName || width <= 0 || height <= 0) {
    return '请选择照片规格';
  }
  if (!form.end_time) {
    return '请填写截止时间';
  }
  if (!isValidDateTime(form.end_time)) {
    return '截止时间格式不正确';
  }
  if (form.start_time && !isValidDateTime(form.start_time)) {
    return '开始时间格式不正确';
  }
  if (form.start_time && new Date(form.start_time).getTime() > new Date(form.end_time).getTime()) {
    return '开始时间不能晚于截止时间';
  }
  if (maxSizeKB < 0) {
    return '文件大小限制不能小于 0';
  }

  return '';
}

function formatPickerTime(value: string): string {
  const date = new Date(value);
  if (isNaN(date.getTime())) return '';
  const hour = String(date.getHours()).padStart(2, '0');
  const minute = String(date.getMinutes()).padStart(2, '0');
  return `${hour}:${minute}`;
}

function formatDateOnly(value: Date): string {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, '0');
  const day = String(value.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function cloneCustomFields(fields: any[]): any[] {
  return (fields || []).map((field: any) => ({
    ...field,
    options: Array.isArray(field.options) ? [...field.options] : []
  }));
}

Page({
  data: {
    taskId: '',
    isEditMode: false,
    taskLoaded: false,
    maxSizeKBInput: '',
    entitlements: null as any,
    aiSwitchDisabled: false,
    aiLimitTip: '',
    createLimitTip: '',
    startLimitTip: '',
    startDateMax: '',
    startDate: '', startTime: '',
    endDate: '', endTime: '',
    form: {
      title: '',
      description: '',
      photo_spec: { name: '', width: 0, height: 0, max_size_kb: 0, background_color: '' },
      ai_analysis_enabled: true,
      start_time: '',
      end_time: '',
      custom_fields: [] as any[]
    }
  },

  onLoad(options: any) {
    const taskId = options.id || '';
    const isEditMode = !!taskId;
    this.setData({
      taskId,
      isEditMode,
      taskLoaded: !isEditMode,
      maxSizeKBInput: isEditMode ? '' : '128',
      'form.photo_spec.max_size_kb': isEditMode ? 0 : 128
    });

    if (isEditMode) {
      wx.setNavigationBarTitle({ title: '编辑任务' });
      this.loadTask(taskId);
    }
    this.loadEntitlements();
  },

  onShow() {
    if (this.data.isEditMode && !this.data.taskLoaded) {
      return;
    }

    const appInstance = getApp<any>();
    const fields = cloneCustomFields((appInstance.globalData && appInstance.globalData.customFields) || []);
    this.setData({ 'form.custom_fields': fields });
    this.loadEntitlements();
  },

  async loadEntitlements() {
    try {
      const entitlements = await getUserEntitlements();
      const appInstance = getApp<any>();
      appInstance.globalData.entitlements = entitlements;
      const canUseAI = !!(entitlements && entitlements.limits && entitlements.limits.can_use_ai_analysis);
      let currentAIEnabled = !!(this.data.form && this.data.form.ai_analysis_enabled);
      const maxActiveTasks = (entitlements && entitlements.limits && entitlements.limits.max_active_tasks) || 0;
      const activeTaskCount = (entitlements && entitlements.usage && entitlements.usage.active_task_count) || 0;
      const maxStartDelayDays = (entitlements && entitlements.limits && entitlements.limits.max_start_delay_days) || 0;
      const startDateMax = maxStartDelayDays > 0 ? formatDateOnly(new Date(Date.now() + maxStartDelayDays * 24 * 60 * 60 * 1000)) : '';
      const nextData: any = {
        entitlements,
        aiLimitTip: canUseAI ? '开启后上传照片会进行 AI 分析。' : 'AI 分析仅限 VIP 使用，开通后即可开启。',
        createLimitTip: entitlements && entitlements.is_vip
          ? 'VIP 会员创建任务和收集人数不受限制。'
          : `普通用户最多创建 ${maxActiveTasks} 个未结束任务，当前已创建 ${activeTaskCount} 个。`,
        startLimitTip: maxStartDelayDays > 0 ? `开始时间最多可选未来 ${maxStartDelayDays} 天内` : '',
        startDateMax
      };

      if (!canUseAI && !this.data.isEditMode && currentAIEnabled) {
        currentAIEnabled = false;
        nextData['form.ai_analysis_enabled'] = false;
      }

      this.setData({
        ...nextData,
        aiSwitchDisabled: !canUseAI && !currentAIEnabled,
      });
    } catch (err: any) {
      showError(err.message || '加载会员权益失败');
    }
  },

  async loadTask(taskId: string) {
    try {
      const task = await getTask(taskId);
      const appInstance = getApp<any>();
      const customFields = cloneCustomFields((task && task.custom_fields) || []);
      const photoSpec = normalizePhotoSpec((task && task.photo_spec) || {});
      appInstance.globalData.customFields = cloneCustomFields(customFields);
      const canUseAI = !!(this.data.entitlements && this.data.entitlements.limits && this.data.entitlements.limits.can_use_ai_analysis);
      const aiAnalysisEnabled = isTaskAIAnalysisEnabled(task);

      this.setData({
        taskLoaded: true,
        maxSizeKBInput: String(photoSpec.max_size_kb),
        startDate: isEffectiveTime(task.start_time) ? formatDate(task.start_time) : '',
        startTime: isEffectiveTime(task.start_time) ? formatPickerTime(task.start_time) : '',
        endDate: isEffectiveTime(task.end_time) ? formatDate(task.end_time) : '',
        endTime: isEffectiveTime(task.end_time) ? formatPickerTime(task.end_time) : '',
        form: {
          title: task.title || '',
          description: task.description || '',
          photo_spec: photoSpec,
          ai_analysis_enabled: aiAnalysisEnabled,
          start_time: isEffectiveTime(task.start_time) ? task.start_time : '',
          end_time: isEffectiveTime(task.end_time) ? task.end_time : '',
          custom_fields: customFields
        },
        aiSwitchDisabled: !canUseAI && !aiAnalysisEnabled
      });
    } catch (err: any) {
      showError(err.message || '加载任务失败');
    }
  },

  onInput(e: any) {
    this.setData({ [`form.${e.currentTarget.dataset.field}`]: e.detail.value });
  },

  onSpecInput(e: any) {
    const field = e.currentTarget.dataset.field;
    const value = e.detail.value;
    if (field === 'max_size_kb') {
      this.setData({
        maxSizeKBInput: value,
        'form.photo_spec.max_size_kb': Number(value || 0)
      });
      return;
    }

    this.setData({
      [`form.photo_spec.${field}`]: Number(value || 0)
    });
  },

  onAIAnalysisChange(e: any) {
    const nextValue = !!(e.detail && e.detail.value);
    const canUseAI = !!(this.data.entitlements && this.data.entitlements.limits && this.data.entitlements.limits.can_use_ai_analysis);
    if (nextValue && !canUseAI) {
      showError('AI分析仅限VIP开启');
      this.setData({ 'form.ai_analysis_enabled': false });
      return;
    }
    this.setData({
      'form.ai_analysis_enabled': nextValue,
      aiSwitchDisabled: !canUseAI && !nextValue
    });
  },

  goToVIPCenter() {
    wx.navigateTo({ url: '/pages/vip-center/vip-center' });
  },

  goToPhotoSpecSelect() {
    wx.navigateTo({ url: '/pages/photo-spec-select/photo-spec-select' });
  },

  onStartDateChange(e: any) {
    if (this.data.isEditMode) return;
    this.setData({ startDate: e.detail.value, 'form.start_time': toRFC3339(e.detail.value, this.data.startTime) });
  },

  onStartTimeChange(e: any) {
    if (this.data.isEditMode) return;
    this.setData({ startTime: e.detail.value, 'form.start_time': toRFC3339(this.data.startDate, e.detail.value) });
  },

  onEndDateChange(e: any) {
    this.setData({ endDate: e.detail.value, 'form.end_time': toRFC3339(e.detail.value, this.data.endTime) });
  },

  onEndTimeChange(e: any) {
    this.setData({ endTime: e.detail.value, 'form.end_time': toRFC3339(this.data.endDate, e.detail.value) });
  },

  goToCustomFields() {
    const appInstance = getApp<any>();
    appInstance.globalData.customFields = cloneCustomFields(this.data.form.custom_fields);
    wx.navigateTo({ url: '/pages/custom-fields/custom-fields' });
  },

  async onSubmit() {
    const appInstance = getApp<any>();
    const { form } = this.data;
    const validationMessage = validateTaskForm(form);
    if (validationMessage) {
      showError(validationMessage);
      return;
    }
    try {
      if (this.data.isEditMode) {
        await updateTask(this.data.taskId, form);
      } else {
        await createTask(form);
      }
      appInstance.globalData.customFields = [];
      showSuccess(this.data.isEditMode ? '更新成功' : '创建成功');
      setTimeout(() => wx.navigateBack(), 1500);
    } catch (err: any) {
      showError(err.message || (this.data.isEditMode ? '更新失败' : '创建失败'));
    }
  }
});
