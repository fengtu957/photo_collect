import { createTask, getTask, updateTask } from '../../services/task';
import { getUserEntitlements } from '../../services/entitlement';
import { showError, showSuccess } from '../../utils/request';
import { isEffectiveTime, toRFC3339 } from '../../utils/time';
import { formatDate } from '../../utils/format';
import { normalizePhotoSpec } from '../../constants/photo-spec';
import { canUseAIAnalysisFeature, isTaskAIAnalysisEnabled } from '../../utils/task';
import {
  buildActiveTaskTip,
  buildOpenDurationDetailTip,
  buildOpenDurationTip,
  buildRetentionTip,
  buildSubmissionLimitTip
} from '../../utils/display-limit';

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

function cloneCustomFields(fields: any[]): any[] {
  return (fields || []).map((field: any) => ({
    ...field,
    options: Array.isArray(field.options) ? [...field.options] : []
  }));
}

function getCopyTaskTimeValue(value: string): string {
  if (!isEffectiveTime(value)) {
    return '';
  }

  return new Date(value).getTime() > Date.now() ? value : '';
}

Page({
  data: {
    taskId: '',
    isEditMode: false,
    isCopyMode: false,
    taskLoaded: false,
    initialAIAnalysisEnabled: false,
    maxSizeKBInput: '',
    entitlements: null as any,
    aiSwitchDisabled: false,
    aiLimitTip: '开启后上传照片会进行 AI 分析。',
    createLimitTip: buildActiveTaskTip(null),
    submissionLimitTip: buildSubmissionLimitTip(null),
    durationLimitTip: buildOpenDurationTip(null),
    retentionTip: buildRetentionTip(null),
    openDurationTip: buildOpenDurationDetailTip(null),
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
    const copySourceTaskId = options.copyFrom || '';
    const isCopyMode = !!copySourceTaskId;
    const isEditMode = !!taskId && !isCopyMode;
    this.setData({
      taskId: isEditMode ? taskId : '',
      isEditMode,
      isCopyMode,
      taskLoaded: !isEditMode && !isCopyMode,
      maxSizeKBInput: isEditMode || isCopyMode ? '' : '128',
      'form.photo_spec.max_size_kb': isEditMode || isCopyMode ? 0 : 128
    });

    if (isEditMode) {
      wx.setNavigationBarTitle({ title: '编辑任务' });
      this.loadTask(taskId);
    }
    if (isCopyMode) {
      wx.setNavigationBarTitle({ title: '复制任务' });
      this.loadTask(copySourceTaskId, true);
    }
    this.loadEntitlements();
  },

  onShow() {
    if ((this.data.isEditMode || this.data.isCopyMode) && !this.data.taskLoaded) {
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
      this.syncEntitlementState(entitlements);
    } catch (err: any) {
      showError(err.message || '加载功能限制失败');
    }
  },

  syncEntitlementState(entitlements: any) {
    const canUseAI = canUseAIAnalysisFeature(entitlements);
    const allowLegacyAI = !!this.data.isEditMode && !!this.data.initialAIAnalysisEnabled;
    let currentAIEnabled = !!(this.data.form && this.data.form.ai_analysis_enabled);
    const nextData: any = {
      entitlements,
      aiLimitTip: canUseAI ? '开启后上传照片会进行 AI 分析。' : '当前版本暂不支持开启 AI 分析。',
      createLimitTip: buildActiveTaskTip(entitlements),
      submissionLimitTip: buildSubmissionLimitTip(entitlements),
      durationLimitTip: buildOpenDurationTip(entitlements),
      retentionTip: buildRetentionTip(entitlements),
      openDurationTip: buildOpenDurationDetailTip(entitlements)
    };

    if (!canUseAI && !allowLegacyAI && currentAIEnabled) {
      currentAIEnabled = false;
      nextData['form.ai_analysis_enabled'] = false;
    }

    nextData.aiSwitchDisabled = !canUseAI && !currentAIEnabled;
    this.setData(nextData);
  },

  async loadTask(taskId: string, isCopyMode: boolean = false) {
    try {
      const task = await getTask(taskId);
      const appInstance = getApp<any>();
      const customFields = cloneCustomFields((task && task.custom_fields) || []);
      const photoSpec = normalizePhotoSpec((task && task.photo_spec) || {});
      appInstance.globalData.customFields = cloneCustomFields(customFields);
      const aiAnalysisEnabled = isTaskAIAnalysisEnabled(task);
      const copiedStartTime = isCopyMode ? getCopyTaskTimeValue(task.start_time) : task.start_time;
      const copiedEndTime = isCopyMode ? getCopyTaskTimeValue(task.end_time) : task.end_time;

      this.setData({
        taskLoaded: true,
        initialAIAnalysisEnabled: isCopyMode ? false : aiAnalysisEnabled,
        maxSizeKBInput: String(photoSpec.max_size_kb),
        startDate: isEffectiveTime(copiedStartTime) ? formatDate(copiedStartTime) : '',
        startTime: isEffectiveTime(copiedStartTime) ? formatPickerTime(copiedStartTime) : '',
        endDate: isEffectiveTime(copiedEndTime) ? formatDate(copiedEndTime) : '',
        endTime: isEffectiveTime(copiedEndTime) ? formatPickerTime(copiedEndTime) : '',
        form: {
          title: task.title || '',
          description: task.description || '',
          photo_spec: photoSpec,
          ai_analysis_enabled: aiAnalysisEnabled,
          start_time: isEffectiveTime(copiedStartTime) ? copiedStartTime : '',
          end_time: isEffectiveTime(copiedEndTime) ? copiedEndTime : '',
          custom_fields: customFields
        },
        aiSwitchDisabled: false
      }, () => {
        if (this.data.entitlements) {
          this.syncEntitlementState(this.data.entitlements);
        }
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
    const canUseAI = canUseAIAnalysisFeature(this.data.entitlements);
    if (nextValue && !canUseAI) {
      showError('当前版本暂不支持开启AI分析');
      this.setData({ 'form.ai_analysis_enabled': false });
      return;
    }
    this.setData({
      'form.ai_analysis_enabled': nextValue,
      aiSwitchDisabled: !canUseAI && !nextValue
    });
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
    const canUseAI = canUseAIAnalysisFeature(this.data.entitlements);
    const nextAIEnabled = !!(form && form.ai_analysis_enabled);
    const initialAIEnabled = !!this.data.initialAIAnalysisEnabled;
    const validationMessage = validateTaskForm(form);
    if (validationMessage) {
      showError(validationMessage);
      return;
    }
    if (nextAIEnabled && !canUseAI && !initialAIEnabled) {
      this.setData({
        'form.ai_analysis_enabled': false,
        aiSwitchDisabled: true
      });
      showError('当前版本暂不支持开启AI分析');
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
