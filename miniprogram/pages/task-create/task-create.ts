import { createTask, getTask, updateTask } from '../../services/task';
import { showError, showSuccess } from '../../utils/request';
import { isEffectiveTime, toRFC3339 } from '../../utils/time';
import { formatDate } from '../../utils/format';

function isValidDateTime(value: string): boolean {
  if (!value) return false;
  return !isNaN(new Date(value).getTime());
}

function validateTaskForm(form: any): string {
  const title = String(form.title || '').trim();
  const photoSpec = form.photo_spec || {};
  const photoSpecName = String(photoSpec.name || '').trim();
  const width = Number(photoSpec.width || 0);
  const height = Number(photoSpec.height || 0);
  const maxSizeKB = Number(photoSpec.max_size_kb || 0);
  const hasPhotoSpecInput = !!photoSpecName || width > 0 || height > 0;

  if (!title) {
    return '请填写任务标题';
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
  if (hasPhotoSpecInput) {
    if (!photoSpecName) {
      return '请填写规格名称';
    }
    if (width <= 0 || height <= 0) {
      return '照片规格宽高必须大于 0';
    }
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

Page({
  data: {
    taskId: '',
    isEditMode: false,
    taskLoaded: false,
    startDate: '', startTime: '',
    endDate: '', endTime: '',
    form: {
      title: '',
      description: '',
      photo_spec: { name: '', width: 0, height: 0, max_size_kb: 0 },
      start_time: '',
      end_time: '',
      custom_fields: [] as any[]
    }
  },

  onLoad(options: any) {
    const taskId = options.id || '';
    const isEditMode = !!taskId;
    this.setData({ taskId, isEditMode, taskLoaded: !isEditMode });

    if (isEditMode) {
      wx.setNavigationBarTitle({ title: '编辑任务' });
      this.loadTask(taskId);
    }
  },

  onShow() {
    if (this.data.isEditMode && !this.data.taskLoaded) {
      return;
    }

    const appInstance = getApp<any>();
    const fields = cloneCustomFields((appInstance.globalData && appInstance.globalData.customFields) || []);
    this.setData({ 'form.custom_fields': fields });
  },

  async loadTask(taskId: string) {
    try {
      const task = await getTask(taskId);
      const appInstance = getApp<any>();
      const customFields = cloneCustomFields((task && task.custom_fields) || []);
      appInstance.globalData.customFields = cloneCustomFields(customFields);

      this.setData({
        taskLoaded: true,
        startDate: isEffectiveTime(task.start_time) ? formatDate(task.start_time) : '',
        startTime: isEffectiveTime(task.start_time) ? formatPickerTime(task.start_time) : '',
        endDate: isEffectiveTime(task.end_time) ? formatDate(task.end_time) : '',
        endTime: isEffectiveTime(task.end_time) ? formatPickerTime(task.end_time) : '',
        form: {
          title: task.title || '',
          description: task.description || '',
          photo_spec: {
            name: (task.photo_spec && task.photo_spec.name) || '',
            width: Number((task.photo_spec && task.photo_spec.width) || 0),
            height: Number((task.photo_spec && task.photo_spec.height) || 0),
            max_size_kb: Number((task.photo_spec && task.photo_spec.max_size_kb) || 0)
          },
          start_time: isEffectiveTime(task.start_time) ? task.start_time : '',
          end_time: isEffectiveTime(task.end_time) ? task.end_time : '',
          custom_fields: customFields
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
    this.setData({
      [`form.photo_spec.${field}`]: field === 'width' || field === 'height' || field === 'max_size_kb'
        ? Number(value || 0)
        : value
    });
  },

  onStartDateChange(e: any) {
    this.setData({ startDate: e.detail.value, 'form.start_time': toRFC3339(e.detail.value, this.data.startTime) });
  },

  onStartTimeChange(e: any) {
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
