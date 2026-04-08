Page({
  data: {
    fields: [] as any[]
  },

  onLoad() {
    const appInstance = getApp<any>();
    const fields = (appInstance.globalData && appInstance.globalData.customFields) || [];
    this.setData({ fields });
  },

  onShow() {
    const appInstance = getApp<any>();
    const fields = (appInstance.globalData && appInstance.globalData.customFields) || [];
    console.log('custom-fields onShow, fields:', JSON.stringify(fields));
    this.setData({ fields: [...fields] });
  },

  addField() {
    if (this.data.fields.length >= 5) {
      wx.showToast({ title: '最多5条', icon: 'none' });
      return;
    }
    wx.navigateTo({ url: '/pages/field-edit/field-edit?mode=add' });
  },

  editField(e: any) {
    const index = e.currentTarget.dataset.index;
    wx.navigateTo({ url: `/pages/field-edit/field-edit?mode=edit&index=${index}` });
  },

  moveUp(e: any) {
    const appInstance = getApp<any>();
    const index = e.currentTarget.dataset.index;
    const fields = [...this.data.fields];
    [fields[index - 1], fields[index]] = [fields[index], fields[index - 1]];
    appInstance.globalData.customFields = fields;
    this.setData({ fields });
  },

  moveDown(e: any) {
    const appInstance = getApp<any>();
    const index = e.currentTarget.dataset.index;
    const fields = [...this.data.fields];
    [fields[index], fields[index + 1]] = [fields[index + 1], fields[index]];
    appInstance.globalData.customFields = fields;
    this.setData({ fields });
  },

  removeField(e: any) {
    const appInstance = getApp<any>();
    const index = e.currentTarget.dataset.index;
    const fields = this.data.fields;
    fields.splice(index, 1);
    appInstance.globalData.customFields = fields;
    this.setData({ fields });
  },

  confirm() {
    const pages = getCurrentPages();
    const prevPage = pages[pages.length - 2] as any;
    prevPage.setData({ 'form.custom_fields': this.data.fields });
    wx.navigateBack();
  }
});
