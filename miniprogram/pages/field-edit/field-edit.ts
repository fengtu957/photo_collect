const TYPES = [
  { value: 'text', label: '文本' },
  { value: 'number', label: '数字' },
  { value: 'select', label: '单选' },
  { value: 'multiselect', label: '多选' }
];

Page({
  data: {
    mode: 'add',
    index: -1,
    typeLabel: '文本',
    field: { id: '', type: 'text', label: '', placeholder: '', required: false, options: [] as string[] }
  },

  onLoad(options: any) {
    const appInstance = getApp<any>();
    this.setData({ mode: options.mode });
    if (options.mode === 'edit') {
      const index = parseInt(options.index);
      const field = { ...appInstance.globalData.customFields[index], options: appInstance.globalData.customFields[index].options || [] };
      const typeObj = TYPES.find(t => t.value === field.type);
      const typeLabel = typeObj ? typeObj.label : '文本';
      this.setData({ index, field, typeLabel });
    }
  },

  onInput(e: any) {
    const field = e.currentTarget.dataset.field;
    this.setData({ [`field.${field}`]: e.detail.value });
  },

  onRequiredChange(e: any) {
    this.setData({ 'field.required': e.detail.value });
  },

  showTypeMenu() {
    wx.showActionSheet({
      itemList: TYPES.map(t => t.label),
      success: (res) => {
        const t = TYPES[res.tapIndex];
        const options = (t.value === 'select' || t.value === 'multiselect') ? ['', ''] : [];
        this.setData({ 'field.type': t.value, typeLabel: t.label, 'field.options': options });
      }
    });
  },

  addOption() {
    const options = [...this.data.field.options, ''];
    this.setData({ 'field.options': options });
  },

  removeOption(e: any) {
    const index = e.currentTarget.dataset.index;
    const options = this.data.field.options.filter((_: any, i: number) => i !== index);
    this.setData({ 'field.options': options });
  },

  onOptionInput(e: any) {
    const index = e.currentTarget.dataset.index;
    this.setData({ [`field.options[${index}]`]: e.detail.value });
  },

  save() {
    const appInstance = getApp<any>();
    const { field, mode, index } = this.data;
    if (!field.label) {
      wx.showToast({ title: '请输入名称', icon: 'none' });
      return;
    }
    if (!appInstance.globalData.customFields) appInstance.globalData.customFields = [];
    if (mode === 'add') {
      field.id = `field_${Date.now()}`;
      appInstance.globalData.customFields.push(field);
    } else {
      appInstance.globalData.customFields[index] = field;
    }
    console.log('保存后 globalData.customFields:', JSON.stringify(appInstance.globalData.customFields));
    wx.navigateBack();
  },

  cancel() {
    wx.navigateBack();
  }
});
