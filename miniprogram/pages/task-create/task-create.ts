import { createTask } from '../../services/task';
import { showError, showSuccess } from '../../utils/request';
import { toRFC3339 } from '../../utils/time';

Page({
  data: {
    startDate: '', startTime: '',
    endDate: '', endTime: '',
    form: {
      title: '',
      description: '',
      photo_spec: { name: '', width: 0, height: 0 },
      start_time: '',
      end_time: '',
      custom_fields: [] as any[]
    }
  },

  onShow() {
    const appInstance = getApp<any>();
    const fields = (appInstance.globalData && appInstance.globalData.customFields) || [];
    this.setData({ 'form.custom_fields': fields });
  },

  onInput(e: any) {
    this.setData({ [`form.${e.currentTarget.dataset.field}`]: e.detail.value });
  },

  onSpecInput(e: any) {
    const field = e.currentTarget.dataset.field;
    const value = e.detail.value;
    this.setData({
      [`form.photo_spec.${field}`]: field === 'width' || field === 'height'
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
    appInstance.globalData.customFields = this.data.form.custom_fields;
    wx.navigateTo({ url: '/pages/custom-fields/custom-fields' });
  },

  async onSubmit() {
    const appInstance = getApp<any>();
    const { form } = this.data;
    if (!form.title || !form.end_time) {
      showError('请填写标题和截止时间');
      return;
    }
    try {
      await createTask(form);
      appInstance.globalData.customFields = [];
      showSuccess('创建成功');
      setTimeout(() => wx.navigateBack(), 1500);
    } catch (err: any) {
      showError(err.message || '创建失败');
    }
  }
});
