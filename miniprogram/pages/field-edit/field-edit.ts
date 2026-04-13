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
    field: { id: '', type: 'text', label: '', placeholder: '', required: false, unique: false, options: [] as string[] }
  },

  onLoad(options: any) {
    const appInstance = getApp<any>();
    this.setData({ mode: options.mode });
    if (options.mode === 'edit') {
      const index = parseInt(options.index);
      const currentField = appInstance.globalData.customFields[index];
      const field = {
        ...currentField,
        unique: !!(currentField && currentField.unique),
        options: (currentField && currentField.options) || []
      };
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

  onUniqueChange(e: any) {
    this.setData({ 'field.unique': e.detail.value });
  },

  showTypeMenu() {
    wx.showActionSheet({
      itemList: TYPES.map(t => t.label),
      success: (res) => {
        const t = TYPES[res.tapIndex];
        const options = (t.value === 'select' || t.value === 'multiselect') ? ['', ''] : [];
        const nextData: any = {
          'field.type': t.value,
          typeLabel: t.label,
          'field.options': options
        };
        if (t.value === 'multiselect') {
          nextData['field.unique'] = false;
        }
        this.setData(nextData);
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
