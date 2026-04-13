export interface PresetPhotoSpecItem {
  id: string;
  name: string;
  width: number;
  height: number;
  desc: string;
  size_text: string;
}

const PHOTO_SPEC_PRESETS = [
  { id: 'one-inch', name: '一寸', width: 25, height: 35, desc: '常用证件照尺寸' },
  { id: 'two-inch', name: '二寸', width: 35, height: 49, desc: '常用报名照尺寸' },
  { id: 'small-two-inch', name: '小二寸', width: 35, height: 45, desc: '签证报名常用' },
  { id: 'cet', name: '英语四六级考试', width: 33, height: 48, desc: 'CET 常用尺寸' },
  { id: 'computer-exam', name: '全国计算机等级考试', width: 33, height: 48, desc: 'NCRE 常用尺寸' },
  { id: 'college-image', name: '大学生图像信息采集', width: 41, height: 54, desc: '学籍照片常用尺寸' },
  { id: 'teacher', name: '教师资格证', width: 35, height: 45, desc: '证件报名常用尺寸' },
  { id: 'accounting', name: '初级会计考试', width: 25, height: 35, desc: '报名照常用尺寸' }
];

export function normalizePhotoSpec(spec: any) {
  return {
    name: String((spec && spec.name) || '').trim(),
    width: Number((spec && spec.width) || 0),
    height: Number((spec && spec.height) || 0),
    max_size_kb: Number((spec && spec.max_size_kb) || 0),
    background_color: String((spec && spec.background_color) || '').trim()
  };
}

export function isSamePhotoSpec(left: any, right: any): boolean {
  const leftSpec = normalizePhotoSpec(left);
  const rightSpec = normalizePhotoSpec(right);
  return !!leftSpec.name
    && !!rightSpec.name
    && leftSpec.name === rightSpec.name
    && leftSpec.width === rightSpec.width
    && leftSpec.height === rightSpec.height;
}

export function buildPhotoSpecOptions(currentSpec?: any): {
  options: PresetPhotoSpecItem[];
  selectedPhotoSpecId: string;
} {
  const normalizedCurrentSpec = normalizePhotoSpec(currentSpec);
  const options: PresetPhotoSpecItem[] = PHOTO_SPEC_PRESETS.map((item) => ({
    ...item,
    size_text: `${item.width}x${item.height}mm`
  }));

  let selectedPhotoSpecId = '';
  for (let i = 0; i < options.length; i += 1) {
    if (isSamePhotoSpec(options[i], normalizedCurrentSpec)) {
      selectedPhotoSpecId = options[i].id;
      break;
    }
  }

  if (!selectedPhotoSpecId && normalizedCurrentSpec.name && normalizedCurrentSpec.width > 0 && normalizedCurrentSpec.height > 0) {
    options.unshift({
      id: 'current-task-spec',
      name: normalizedCurrentSpec.name,
      width: normalizedCurrentSpec.width,
      height: normalizedCurrentSpec.height,
      desc: '当前任务规格',
      size_text: `${normalizedCurrentSpec.width}x${normalizedCurrentSpec.height}mm`
    });
    selectedPhotoSpecId = 'current-task-spec';
  }

  return {
    options,
    selectedPhotoSpecId
  };
}
