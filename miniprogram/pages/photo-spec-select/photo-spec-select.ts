import { buildPhotoSpecOptions, normalizePhotoSpec } from '../../constants/photo-spec';

const BACKGROUND_COLOR_OPTIONS = [
  { value: '', label: '无要求', dot_class: 'dot-none' },
  { value: '白底', label: '白底', dot_class: 'dot-white' },
  { value: '蓝底', label: '蓝底', dot_class: 'dot-blue' },
  { value: '红底', label: '红底', dot_class: 'dot-red' },
  { value: '纯色', label: '纯色', dot_class: 'dot-solid' },
  { value: '__custom__', label: '其他', dot_class: 'dot-custom' }
];

function filterPhotoSpecs(options: any[], keyword: string) {
  const text = String(keyword || '').trim().toLowerCase();
  if (!text) {
    return options;
  }

  const result = [];
  for (let i = 0; i < options.length; i += 1) {
    const item = options[i];
    const searchText = `${item.name} ${item.desc} ${item.size_text}`.toLowerCase();
    if (searchText.indexOf(text) >= 0) {
      result.push(item);
    }
  }
  return result;
}

Page({
  data: {
    keyword: '',
    allOptions: [] as any[],
    filteredOptions: [] as any[],
    selectedPhotoSpecId: '',
    selectedPhotoSpec: null as any,
    backgroundColorOptions: BACKGROUND_COLOR_OPTIONS,
    selectedBackgroundColor: '',
    customBackgroundColor: ''
  },

  onLoad() {
    const pages = getCurrentPages();
    const prevPage = pages[pages.length - 2] as any;
    const currentSpec = normalizePhotoSpec(prevPage && prevPage.data && prevPage.data.form && prevPage.data.form.photo_spec);
    const photoSpecState = buildPhotoSpecOptions(currentSpec);
    const backgroundColor = String(currentSpec.background_color || '').trim();
    let selectedBackgroundColor = backgroundColor;
    let customBackgroundColor = '';

    if (backgroundColor && backgroundColor !== '白底' && backgroundColor !== '蓝底' && backgroundColor !== '红底' && backgroundColor !== '纯色') {
      selectedBackgroundColor = '__custom__';
      customBackgroundColor = backgroundColor;
    }

    this.setData({
      allOptions: photoSpecState.options,
      filteredOptions: photoSpecState.options,
      selectedPhotoSpecId: photoSpecState.selectedPhotoSpecId,
      selectedPhotoSpec: photoSpecState.selectedPhotoSpecId
        ? photoSpecState.options.filter((item: any) => item.id === photoSpecState.selectedPhotoSpecId)[0] || null
        : null,
      selectedBackgroundColor,
      customBackgroundColor
    });
  },

  onKeywordInput(e: any) {
    const keyword = e.detail.value;
    this.setData({
      keyword,
      filteredOptions: filterPhotoSpecs(this.data.allOptions, keyword)
    });
  },

  selectSpec(e: any) {
    const selectedId = e.currentTarget.dataset.id;
    const allOptions = this.data.allOptions || [];
    let selectedOption = null;

    for (let i = 0; i < allOptions.length; i += 1) {
      if (allOptions[i].id === selectedId) {
        selectedOption = allOptions[i];
        break;
      }
    }

    if (!selectedOption) {
      return;
    }

    this.setData({
      selectedPhotoSpecId: selectedId,
      selectedPhotoSpec: selectedOption
    });
  },

  confirmSelection() {
    const selectedOption = this.data.selectedPhotoSpec;

    if (!selectedOption) {
      wx.showToast({ title: '请选择照片规格', icon: 'none' });
      return;
    }

    const pages = getCurrentPages();
    const prevPage = pages[pages.length - 2] as any;
    const prevSpec = normalizePhotoSpec(prevPage && prevPage.data && prevPage.data.form && prevPage.data.form.photo_spec);
    const customBackgroundColor = String(this.data.customBackgroundColor || '').trim();
    const backgroundColor = this.data.selectedBackgroundColor === '__custom__'
      ? customBackgroundColor
      : String(this.data.selectedBackgroundColor || '');

    if (this.data.selectedBackgroundColor === '__custom__' && !customBackgroundColor) {
      wx.showToast({ title: '请输入其他背景色要求', icon: 'none' });
      return;
    }

    prevPage.setData({
      'form.photo_spec.name': selectedOption.name,
      'form.photo_spec.width': Number(selectedOption.width || 0),
      'form.photo_spec.height': Number(selectedOption.height || 0),
      'form.photo_spec.max_size_kb': Number(prevSpec.max_size_kb || 0),
      'form.photo_spec.background_color': backgroundColor
    });

    wx.navigateBack();
  },

  selectBackgroundColor(e: any) {
    const value = String(e.currentTarget.dataset.value || '');
    this.setData({
      selectedBackgroundColor: value,
      customBackgroundColor: value === '__custom__' ? this.data.customBackgroundColor : ''
    });
  },

  onCustomBackgroundInput(e: any) {
    this.setData({
      customBackgroundColor: e.detail.value
    });
  }
});
