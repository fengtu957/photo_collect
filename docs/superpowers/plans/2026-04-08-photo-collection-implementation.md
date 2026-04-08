# 批量证件照采集系统 - 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标:** 构建一个 B2B2C 的批量证件照采集管理系统，支持任务创建、分享、拍摄上传、AI 质量评估和数据导出

**架构:** 微信小程序原生 + TypeScript + 微信云开发（云函数 + 云数据库 + 云存储），使用 Skyline 渲染引擎。管理员创建任务并分享，参与者通过分享卡片进入填写信息并拍摄上传，AI 异步评估照片质量，管理员可导出数据。

**技术栈:** 微信小程序、TypeScript、微信云开发、通义千问-VL API

---

## 文件结构规划

### 前端页面
```
miniprogram/pages/
├── index/                          # 首页（任务列表）
├── task-create/                    # 创建任务
├── task-detail/                    # 任务详情（管理视图）
├── task-view/                      # 任务详情（参与视图）
├── submission-form/                # 信息填写
├── camera/                         # 拍摄页面
├── photo-preview/                  # 照片预览确认
└── submission-success/             # 提交成功
```

### 组件
```
miniprogram/components/
├── task-card/                      # 任务卡片
├── photo-spec-selector/            # 规格选择器
├── custom-field-builder/           # 字段构建器
├── camera-capture/                 # 相机组件（复用 other/camera）
├── ai-evaluation-card/             # AI 评估结果卡片
└── submission-list/                # 提交列表
```

### 云函数
```
cloudfunctions/
├── createTask/                     # 创建任务
├── getTaskDetail/                  # 获取任务详情
├── getTaskSubmissions/             # 获取提交列表
├── submitPhoto/                    # 提交照片
├── evaluatePhoto/                  # AI 评估（异步）
├── updateSubmission/               # 修改提交
├── exportTask/                     # 导出任务
└── common/                         # 公共工具
    ├── db.js                       # 数据库工具
    ├── storage.js                  # 存储工具
    └── qwen.js                     # 通义千问 API 封装
```

### 工具类
```
miniprogram/
├── utils/
│   ├── request.ts                  # 云函数调用封装
│   ├── upload.ts                   # 上传工具
│   └── format.ts                   # 格式化工具
├── services/
│   ├── task.ts                     # 任务服务
│   └── submission.ts               # 提交服务
└── types/
    ├── task.ts                     # 任务类型定义
    └── submission.ts               # 提交类型定义
```

---

## 阶段 1: 环境搭建与基础配置

### Task 1: 开通微信云开发环境

**目标:** 在微信开发者工具中开通云开发，创建数据库集合和索引

- [ ] **Step 1: 打开微信开发者工具**

打开项目 `D:\code\latest\photo`，点击工具栏"云开发"按钮

- [ ] **Step 2: 开通云开发**

1. 点击"开通云开发"
2. 选择"基础版 1"（免费额度）
3. 环境名称：`photo-collection`
4. 记录环境 ID（格式：`photo-collection-xxxxx`）

- [ ] **Step 3: 配置云开发环境 ID**

修改 `miniprogram/app.ts`，添加云开发初始化：

```typescript
App({
  onLaunch() {
    if (!wx.cloud) {
      console.error('请使用 2.2.3 或以上的基础库以使用云能力');
    } else {
      wx.cloud.init({
        env: 'photo-collection-xxxxx', // 替换为你的环境 ID
        traceUser: true,
      });
    }
  },
});
```

- [ ] **Step 4: 创建数据库集合**

在云开发控制台 → 数据库 → 创建集合：
1. 集合名：`tasks`
2. 集合名：`submissions`
3. 集合名：`export_history`

- [ ] **Step 5: 配置数据库索引**

在 `tasks` 集合中添加索引：
- 索引字段：`_openid`，排序：升序
- 索引字段：`enabled, endTime`，排序：升序, 升序

在 `submissions` 集合中添加索引：
- 索引字段：`taskId, _openid`，排序：升序, 升序，唯一索引：是
- 索引字段：`taskId, createdAt`，排序：升序, 降序
- 索引字段：`_openid`，排序：升序

- [ ] **Step 6: 配置云存储**

在云开发控制台 → 云存储 → 设置：
- 确认存储空间可用（免费 5GB）

- [ ] **Step 7: 提交配置**

```bash
git add miniprogram/app.ts
git commit -m "chore: 初始化云开发环境配置"
```

---

### Task 2: 创建类型定义

**Files:**
- Create: `miniprogram/types/task.ts`
- Create: `miniprogram/types/submission.ts`

- [ ] **Step 1: 创建任务类型定义**

创建 `miniprogram/types/task.ts`：

```typescript
export interface PhotoSpec {
  name: string;
  width: number;
  height: number;
  dpi?: number;
}

export interface CustomField {
  id: string;
  type: 'text' | 'select';
  label: string;
  required: boolean;
  options?: string[];
  placeholder?: string;
}

export interface StorageConfig {
  retentionDays: number;
  expirationDate: Date;
  autoDeleteAfterExport: boolean;
}

export interface ExportConfig {
  nameTemplate: string;
  includeOriginal: boolean;
}

export interface TaskStats {
  totalSubmissions: number;
  lastSubmitTime?: Date;
}

export interface Task {
  _id: string;
  _openid: string;
  title: string;
  description: string;
  photoSpec: PhotoSpec;
  startTime: Date;
  endTime: Date;
  enabled: boolean;
  customFields: CustomField[];
  stats: TaskStats;
  storageConfig: StorageConfig;
  exportConfig: ExportConfig;
  createdAt: Date;
  updatedAt: Date;
}

export interface CreateTaskParams {
  title: string;
  description: string;
  photoSpec: PhotoSpec;
  startTime: Date;
  endTime: Date;
  customFields: CustomField[];
  storageConfig?: Partial<StorageConfig>;
}
```

- [ ] **Step 2: 创建提交类型定义**

创建 `miniprogram/types/submission.ts`：

```typescript
export interface PhotoInfo {
  originalUrl: string;
  originalFileId: string;
  fileSize: number;
  width: number;
  height: number;
  expiresAt: Date;
  deleted: boolean;
  deletedAt?: Date;
  deletedReason?: string;
}

export interface AIEvaluationBreakdown {
  clarity: number;
  lighting: number;
  angle: number;
  background: number;
  expression: number;
  composition: number;
}

export interface AIEvaluation {
  status: 'pending' | 'success' | 'failed';
  score: number;
  issues: string[];
  suggestions: string[];
  breakdown: AIEvaluationBreakdown;
  evaluatedAt?: Date;
  error?: string;
}

export interface UserInfo {
  nickName: string;
  avatarUrl: string;
}

export interface Submission {
  _id: string;
  _openid: string;
  taskId: string;
  userInfo: UserInfo;
  customData: Record<string, string | string[]>;
  photo: PhotoInfo;
  aiEvaluation: AIEvaluation;
  status: 'draft' | 'submitted' | 'rejected';
  createdAt: Date;
  updatedAt: Date;
}

export interface SubmitPhotoParams {
  taskId: string;
  customData: Record<string, string | string[]>;
  photo: {
    fileId: string;
    width: number;
    height: number;
    fileSize: number;
  };
}
```

- [ ] **Step 3: 提交类型定义**

```bash
git add miniprogram/types/
git commit -m "feat: 添加任务和提交的类型定义"
```

---

### Task 3: 创建工具类

**Files:**
- Create: `miniprogram/utils/request.ts`
- Create: `miniprogram/utils/upload.ts`
- Create: `miniprogram/utils/format.ts`

- [ ] **Step 1: 创建云函数调用封装**

创建 `miniprogram/utils/request.ts`：

```typescript
interface CloudFunctionResult<T = any> {
  success: boolean;
  data?: T;
  error?: string;
}

export async function callFunction<T = any>(
  name: string,
  data: any = {}
): Promise<CloudFunctionResult<T>> {
  try {
    const res = await wx.cloud.callFunction({
      name,
      data,
    });
    return res.result as CloudFunctionResult<T>;
  } catch (err: any) {
    console.error(`云函数 ${name} 调用失败:`, err);
    return {
      success: false,
      error: err.errMsg || '网络错误',
    };
  }
}

export function showError(message: string) {
  wx.showToast({
    title: message,
    icon: 'none',
    duration: 2000,
  });
}

export function showSuccess(message: string) {
  wx.showToast({
    title: message,
    icon: 'success',
    duration: 2000,
  });
}

export function showLoading(title: string = '加载中...') {
  wx.showLoading({ title, mask: true });
}

export function hideLoading() {
  wx.hideLoading();
}
```

- [ ] **Step 2: 创建上传工具**

创建 `miniprogram/utils/upload.ts`：

```typescript
export interface UploadResult {
  fileID: string;
  statusCode: number;
}

export async function uploadToCloud(
  filePath: string,
  cloudPath: string
): Promise<UploadResult | null> {
  try {
    const res = await wx.cloud.uploadFile({
      cloudPath,
      filePath,
    });
    return res;
  } catch (err) {
    console.error('上传失败:', err);
    return null;
  }
}

export function generateCloudPath(taskId: string, openid: string, ext: string = 'jpg'): string {
  const timestamp = Date.now();
  return `submissions/${taskId}/${openid}_${timestamp}.${ext}`;
}

export async function getImageInfo(filePath: string): Promise<wx.GetImageInfoSuccessCallbackResult | null> {
  try {
    const res = await wx.getImageInfo({ src: filePath });
    return res;
  } catch (err) {
    console.error('获取图片信息失败:', err);
    return null;
  }
}
```

- [ ] **Step 3: 创建格式化工具**

创建 `miniprogram/utils/format.ts`：

```typescript
export function formatDate(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date;
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

export function formatDateTime(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date;
  const dateStr = formatDate(d);
  const hour = String(d.getHours()).padStart(2, '0');
  const minute = String(d.getMinutes()).padStart(2, '0');
  return `${dateStr} ${hour}:${minute}`;
}

export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

export function getTimeRemaining(endTime: Date | string): string {
  const end = typeof endTime === 'string' ? new Date(endTime) : endTime;
  const now = new Date();
  const diff = end.getTime() - now.getTime();

  if (diff <= 0) return '已截止';

  const days = Math.floor(diff / (1000 * 60 * 60 * 24));
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));

  if (days > 0) return `剩余 ${days} 天`;
  if (hours > 0) return `剩余 ${hours} 小时`;
  return '即将截止';
}

export function isTaskActive(startTime: Date | string, endTime: Date | string): boolean {
  const now = new Date();
  const start = typeof startTime === 'string' ? new Date(startTime) : startTime;
  const end = typeof endTime === 'string' ? new Date(endTime) : endTime;
  return now >= start && now <= end;
}
```

- [ ] **Step 4: 提交工具类**

```bash
git add miniprogram/utils/
git commit -m "feat: 添加云函数调用、上传和格式化工具类"
```

---

## 阶段 2: 云函数开发

### Task 4: 创建云函数公共工具

**Files:**
- Create: `cloudfunctions/common/db.js`
- Create: `cloudfunctions/common/qwen.js`

- [ ] **Step 1: 创建数据库工具**

在微信开发者工具中，右键 `cloudfunctions` 目录 → 新建 Node.js 云函数 → 命名为 `common`

创建 `cloudfunctions/common/db.js`：

```javascript
const cloud = require('wx-server-sdk');
cloud.init({ env: cloud.DYNAMIC_CURRENT_ENV });
const db = cloud.database();

module.exports = {
  db,
  _ : db.command,

  async getTask(taskId) {
    const { data } = await db.collection('tasks').doc(taskId).get();
    return data;
  },

  async updateTaskStats(taskId) {
    const { total } = await db.collection('submissions')
      .where({ taskId })
      .count();

    await db.collection('tasks').doc(taskId).update({
      data: {
        'stats.totalSubmissions': total,
        'stats.lastSubmitTime': new Date(),
        updatedAt: new Date(),
      }
    });
  },
};
```

- [ ] **Step 2: 创建通义千问 API 封装**

创建 `cloudfunctions/common/qwen.js`（需要在云函数环境变量中配置 QWEN_API_KEY）

- [ ] **Step 3: 配置 package.json**

创建 `cloudfunctions/common/package.json`：

```json
{
  "name": "common",
  "version": "1.0.0",
  "dependencies": {
    "wx-server-sdk": "latest",
    "axios": "^1.6.0"
  }
}
```

- [ ] **Step 4: 上传并部署**

右键 `common` 云函数 → 上传并部署：云端安装依赖

---

### Task 5: 创建任务管理云函数

**Files:**
- Create: `cloudfunctions/createTask/index.js`
- Create: `cloudfunctions/getTaskDetail/index.js`

- [ ] **Step 1: 创建 createTask 云函数**

右键 `cloudfunctions` → 新建 Node.js 云函数 → 命名为 `createTask`

- [ ] **Step 2: 创建 getTaskDetail 云函数**

右键 `cloudfunctions` → 新建 Node.js 云函数 → 命名为 `getTaskDetail`

- [ ] **Step 3: 上传并部署云函数**

分别右键两个云函数 → 上传并部署：云端安装依赖

- [ ] **Step 4: 提交代码**

```bash
git add cloudfunctions/
git commit -m "feat: 添加任务管理云函数"
```

---

### Task 6: 创建提交相关云函数

**Files:**
- Create: `cloudfunctions/submitPhoto/index.js`
- Create: `cloudfunctions/evaluatePhoto/index.js`
- Create: `cloudfunctions/getTaskSubmissions/index.js`

- [ ] **Step 1: 创建 submitPhoto 云函数**

创建云函数，实现提交照片逻辑：
1. 校验任务状态（enabled、时间范围）
2. 检查是否已提交（防重复）
3. 创建提交记录
4. 异步触发 AI 评估
5. 更新任务统计

- [ ] **Step 2: 创建 evaluatePhoto 云函数**

创建云函数，实现 AI 评估逻辑：
1. 获取照片临时 URL
2. 调用通义千问-VL API
3. 解析评估结果
4. 更新 submission 的 aiEvaluation 字段

- [ ] **Step 3: 创建 getTaskSubmissions 云函数**

创建云函数，实现获取提交列表（仅任务创建者可调用）

- [ ] **Step 4: 上传并部署**

上传并部署三个云函数

- [ ] **Step 5: 提交代码**

```bash
git add cloudfunctions/
git commit -m "feat: 添加提交和评估云函数"
```

---

## 阶段 3: 管理端开发

### Task 7: 创建首页（任务列表）

**Files:**
- Modify: `miniprogram/pages/index/index.ts`
- Modify: `miniprogram/pages/index/index.wxml`
- Modify: `miniprogram/pages/index/index.wxss`

- [ ] **Step 1: 实现页面逻辑**

修改 `miniprogram/pages/index/index.ts`：
1. 获取用户创建的任务列表
2. 显示任务状态（进行中/已结束）
3. 点击跳转到任务详情

- [ ] **Step 2: 实现页面布局**

修改 `miniprogram/pages/index/index.wxml`：
1. 顶部标题 + 创建按钮
2. 任务列表（使用 task-card 组件）
3. 空状态提示

- [ ] **Step 3: 实现样式**

修改 `miniprogram/pages/index/index.wxss`，使用 WeUI 风格

- [ ] **Step 4: 测试页面**

在开发者工具中预览，确认列表显示正常

- [ ] **Step 5: 提交代码**

```bash
git add miniprogram/pages/index/
git commit -m "feat: 实现任务列表页面"
```

---

### Task 8: 创建任务卡片组件

**Files:**
- Create: `miniprogram/components/task-card/`

- [ ] **Step 1: 创建组件**

右键 `miniprogram/components` → 新建 Component → 命名为 `task-card`

- [ ] **Step 2: 实现组件逻辑**

显示任务基本信息：标题、规格、时间范围、状态、提交统计

- [ ] **Step 3: 实现组件样式**

卡片式布局，圆角 12px，使用主色调蓝色

- [ ] **Step 4: 提交代码**

```bash
git add miniprogram/components/task-card/
git commit -m "feat: 添加任务卡片组件"
```

---

### Task 9: 创建任务创建页面

**Files:**
- Create: `miniprogram/pages/task-create/`
- Create: `miniprogram/components/photo-spec-selector/`
- Create: `miniprogram/components/custom-field-builder/`

- [ ] **Step 1: 创建页面**

右键 `miniprogram/pages` → 新建 Page → 命名为 `task-create`

- [ ] **Step 2: 创建规格选择器组件**

实现预设规格选择（一寸、二寸等）和自定义输入

- [ ] **Step 3: 创建字段构建器组件**

实现添加/编辑/删除自定义字段，支持文本和多选类型

- [ ] **Step 4: 实现创建页面逻辑**

1. 表单输入（标题、描述、规格、时间、字段）
2. 调用 createTask 云函数
3. 创建成功后跳转到任务详情

- [ ] **Step 5: 更新 app.json**

在 `miniprogram/app.json` 的 pages 数组中添加 `pages/task-create/task-create`

- [ ] **Step 6: 提交代码**

```bash
git add miniprogram/pages/task-create/ miniprogram/components/
git commit -m "feat: 实现任务创建页面"
```

---

### Task 10: 创建任务详情页（管理视图）

**Files:**
- Create: `miniprogram/pages/task-detail/`
- Create: `miniprogram/components/submission-list/`

- [ ] **Step 1: 创建页面**

右键 `miniprogram/pages` → 新建 Page → 命名为 `task-detail`

- [ ] **Step 2: 实现页面逻辑**

1. 获取任务详情
2. 显示任务信息卡片
3. 显示统计信息（提交数、过期倒计时）
4. 显示提交列表
5. 分享功能
6. 导出按钮

- [ ] **Step 3: 创建提交列表组件**

显示提交记录：头像、昵称、时间、AI 评分

- [ ] **Step 4: 实现分享功能**

```typescript
onShareAppMessage() {
  return {
    title: `【证件照采集】${this.data.task.title}`,
    path: `/pages/task-view/index?taskId=${this.data.task._id}`,
  };
}
```

- [ ] **Step 5: 更新 app.json**

添加 `pages/task-detail/task-detail`

- [ ] **Step 6: 提交代码**

```bash
git add miniprogram/pages/task-detail/ miniprogram/components/submission-list/
git commit -m "feat: 实现任务详情页（管理视图）"
```

---

## 阶段 4: 参与端开发

### Task 11: 创建任务查看页（参与视图）

**Files:**
- Create: `miniprogram/pages/task-view/`

- [ ] **Step 1: 创建页面**

右键 `miniprogram/pages` → 新建 Page → 命名为 `task-view`

- [ ] **Step 2: 实现页面逻辑**

1. 从 URL 参数获取 taskId
2. 调用 getTaskDetail 获取任务信息
3. 显示任务要求（规格、自定义字段）
4. 检查是否已提交
5. 显示"开始采集"按钮

- [ ] **Step 3: 实现页面布局**

1. 任务信息卡片
2. 采集要求说明
3. 底部固定按钮

- [ ] **Step 4: 更新 app.json**

添加 `pages/task-view/task-view`

- [ ] **Step 5: 提交代码**

```bash
git add miniprogram/pages/task-view/
git commit -m "feat: 实现任务查看页（参与视图）"
```

---

### Task 12: 创建信息填写页

**Files:**
- Create: `miniprogram/pages/submission-form/`

- [ ] **Step 1: 创建页面**

右键 `miniprogram/pages` → 新建 Page → 命名为 `submission-form`

- [ ] **Step 2: 实现页面逻辑**

1. 接收 taskId 参数
2. 根据 customFields 动态渲染表单
3. 表单验证（必填项检查）
4. 保存数据到页面状态
5. 跳转到拍摄页面

- [ ] **Step 3: 实现动态表单渲染**

支持 text 和 select 两种字段类型

- [ ] **Step 4: 更新 app.json**

添加 `pages/submission-form/submission-form`

- [ ] **Step 5: 提交代码**

```bash
git add miniprogram/pages/submission-form/
git commit -m "feat: 实现信息填写页面"
```

---

### Task 13: 创建相机组件（简化实现）

**Files:**
- Create: `miniprogram/components/camera-capture/`

- [ ] **Step 1: 创建组件**

右键 `miniprogram/components` → 新建 Component → 命名为 `camera-capture`

- [ ] **Step 2: 实现简化的拍摄逻辑**

使用 `<camera>` 组件和 `wx.chooseMedia`：
1. camera 组件配置
2. 拍照功能
3. 前后摄像头切换
4. 相册选择

- [ ] **Step 3: 移除人脸检测**

不使用 VKSession（避免资质要求），依赖 AI 评估保证照片质量

- [ ] **Step 4: 实现组件接口**

```typescript
Component({
  properties: {
    photoSpec: Object,
  },
  methods: {
    onCapture(filePath) {
      this.triggerEvent('capture', { filePath });
    }
  }
});
```

- [ ] **Step 5: 提交代码**

```bash
git add miniprogram/components/camera-capture/
git commit -m "feat: 添加相机组件（简化实现）"
```

---

### Task 14: 创建拍摄页面

**Files:**
- Create: `miniprogram/pages/camera/`

- [ ] **Step 1: 创建页面**

右键 `miniprogram/pages` → 新建 Page → 命名为 `camera`

- [ ] **Step 2: 实现页面逻辑**

1. 接收 taskId 和 customData 参数
2. 使用 camera-capture 组件
3. 拍照成功后跳转到预览页

- [ ] **Step 3: 实现全屏相机布局**

1. 全屏 camera-capture 组件
2. 底部工具栏（相册、拍照、切换）
3. 简化界面，无人脸识别框

- [ ] **Step 4: 更新 app.json**

添加 `pages/camera/camera`

- [ ] **Step 5: 提交代码**

```bash
git add miniprogram/pages/camera/
git commit -m "feat: 实现拍摄页面"
```

---

### Task 15: 创建照片预览确认页

**Files:**
- Create: `miniprogram/pages/photo-preview/`

- [ ] **Step 1: 创建页面**

右键 `miniprogram/pages` → 新建 Page → 命名为 `photo-preview`

- [ ] **Step 2: 实现页面逻辑**

1. 接收照片路径、taskId、customData
2. 显示照片预览
3. 获取图片信息（宽高、大小）
4. 上传到云存储
5. 调用 submitPhoto 云函数
6. 跳转到提交成功页

- [ ] **Step 3: 实现上传流程**

```typescript
async submitPhoto() {
  wx.showLoading({ title: '上传中...' });
  
  // 1. 获取图片信息
  const imageInfo = await getImageInfo(this.data.filePath);
  
  // 2. 上传到云存储
  const cloudPath = generateCloudPath(taskId, openid);
  const uploadResult = await uploadToCloud(this.data.filePath, cloudPath);
  
  // 3. 提交记录
  const result = await callFunction('submitPhoto', {
    taskId,
    customData,
    photo: {
      fileId: uploadResult.fileID,
      width: imageInfo.width,
      height: imageInfo.height,
      fileSize: imageInfo.size,
    }
  });
  
  wx.hideLoading();
  
  if (result.success) {
    wx.redirectTo({ url: `/pages/submission-success/index?submissionId=${result.data.submissionId}` });
  }
}
```

- [ ] **Step 4: 更新 app.json**

添加 `pages/photo-preview/photo-preview`

- [ ] **Step 5: 提交代码**

```bash
git add miniprogram/pages/photo-preview/
git commit -m "feat: 实现照片预览确认页"
```

---

### Task 16: 创建提交成功页

**Files:**
- Create: `miniprogram/pages/submission-success/`
- Create: `miniprogram/components/ai-evaluation-card/`

- [ ] **Step 1: 创建页面**

右键 `miniprogram/pages` → 新建 Page → 命名为 `submission-success`

- [ ] **Step 2: 创建 AI 评估卡片组件**

右键 `miniprogram/components` → 新建 Component → 命名为 `ai-evaluation-card`

实现三种状态：
1. pending: 加载动画 + "AI 评估中..."
2. success: 分数（环形进度条）+ 问题 + 建议
3. failed: 错误提示

- [ ] **Step 3: 实现页面逻辑**

1. 显示"提交成功"提示
2. 使用实时数据库监听 AI 评估结果
3. 评分 < 70 时提示"建议重拍"
4. 提供"返回首页"和"修改提交"按钮

- [ ] **Step 4: 实现实时数据库监听**

```typescript
const watcher = db.collection('submissions').doc(submissionId).watch({
  onChange: (snapshot) => {
    const data = snapshot.docs[0];
    if (data.aiEvaluation.status !== 'pending') {
      this.setData({ aiEvaluation: data.aiEvaluation });
      watcher.close();
    }
  },
  onError: (err) => {
    console.error('监听失败:', err);
  }
});
```

- [ ] **Step 5: 更新 app.json**

添加 `pages/submission-success/submission-success`

- [ ] **Step 6: 提交代码**

```bash
git add miniprogram/pages/submission-success/ miniprogram/components/ai-evaluation-card/
git commit -m "feat: 实现提交成功页和AI评估卡片"
```

---

## 阶段 5: AI 评估集成

### Task 17: 配置通义千问 API

**Files:**
- Modify: `cloudfunctions/common/qwen.js`

- [ ] **Step 1: 获取 API 密钥**

访问阿里云控制台，获取通义千问-VL 的 API Key

- [ ] **Step 2: 配置环境变量**

在微信开发者工具 → 云开发控制台 → 设置 → 环境变量：
- 变量名：`QWEN_API_KEY`
- 变量值：你的 API Key

- [ ] **Step 3: 完善 qwen.js 实现**

补充完整的 API 调用逻辑和错误处理

- [ ] **Step 4: 测试 API 调用**

创建测试云函数验证 API 可用性

---

### Task 18: 实现异步评估流程

**Files:**
- Modify: `cloudfunctions/submitPhoto/index.js`
- Modify: `cloudfunctions/evaluatePhoto/index.js`

- [ ] **Step 1: 修改 submitPhoto 云函数**

在提交成功后异步触发评估：

```javascript
// 创建提交记录后
await db.collection('submissions').doc(submissionId).update({
  data: {
    'aiEvaluation.status': 'pending'
  }
});

// 异步触发评估（不等待结果）
cloud.callFunction({
  name: 'evaluatePhoto',
  data: { submissionId }
});
```

- [ ] **Step 2: 实现 evaluatePhoto 云函数**

1. 获取提交记录
2. 获取照片临时 URL
3. 调用通义千问 API
4. 更新评估结果

- [ ] **Step 3: 添加重试机制**

评估失败时最多重试 2 次

- [ ] **Step 4: 上传并部署**

上传修改后的云函数

- [ ] **Step 5: 提交代码**

```bash
git add cloudfunctions/
git commit -m "feat: 实现异步AI评估流程"
```

---

### Task 19: 测试 AI 评估功能

- [ ] **Step 1: 端到端测试**

1. 创建测试任务
2. 提交照片
3. 观察 AI 评估结果
4. 验证评分和建议是否合理

- [ ] **Step 2: 边界测试**

1. 测试模糊照片
2. 测试角度不正的照片
3. 测试背景杂乱的照片

- [ ] **Step 3: 性能测试**

验证评估时间在 2-5 秒内

---

## 阶段 6: 导出功能开发

### Task 20: 创建导出云函数

**Files:**
- Create: `cloudfunctions/exportTask/index.js`

- [ ] **Step 1: 创建云函数**

右键 `cloudfunctions` → 新建 Node.js 云函数 → 命名为 `exportTask`

- [ ] **Step 2: 安装依赖**

在 `cloudfunctions/exportTask/package.json` 中添加：

```json
{
  "dependencies": {
    "wx-server-sdk": "latest",
    "archiver": "^5.3.0",
    "xlsx": "^0.18.0"
  }
}
```

- [ ] **Step 3: 实现导出逻辑**

1. 权限校验（仅创建者）
2. 查询所有提交记录
3. 批量下载照片（每次10张）
4. 按模板重命名文件
5. 生成 Excel 数据表
6. 打包成 ZIP
7. 上传到云存储
8. 返回临时下载链接（7天有效）

- [ ] **Step 4: 实现文件命名模板**

支持变量替换：
- `{name}` - 姓名
- `{id}` - 提交 ID
- `{date}` - 提交日期
- `{index}` - 序号

- [ ] **Step 5: 上传并部署**

右键 `exportTask` → 上传并部署：云端安装依赖

- [ ] **Step 6: 提交代码**

```bash
git add cloudfunctions/exportTask/
git commit -m "feat: 实现导出功能云函数"
```

---

### Task 21: 创建导出配置页面

**Files:**
- Create: `miniprogram/pages/task-export/`

- [ ] **Step 1: 创建页面**

右键 `miniprogram/pages` → 新建 Page → 命名为 `task-export`

- [ ] **Step 2: 实现页面逻辑**

1. 接收 taskId 参数
2. 配置文件名模板
3. 选择导出选项（包含原图、图片格式等）
4. 调用 exportTask 云函数
5. 显示导出进度
6. 生成下载链接

- [ ] **Step 3: 实现模板预览**

实时显示文件命名效果

- [ ] **Step 4: 更新 app.json**

添加 `pages/task-export/task-export`

- [ ] **Step 5: 提交代码**

```bash
git add miniprogram/pages/task-export/
git commit -m "feat: 实现导出配置页面"
```

---

### Task 22: 在任务详情页添加导出按钮

**Files:**
- Modify: `miniprogram/pages/task-detail/index.ts`
- Modify: `miniprogram/pages/task-detail/index.wxml`

- [ ] **Step 1: 添加导出按钮**

在任务详情页底部添加"导出数据"按钮

- [ ] **Step 2: 实现跳转逻辑**

点击按钮跳转到导出配置页

- [ ] **Step 3: 提交代码**

```bash
git add miniprogram/pages/task-detail/
git commit -m "feat: 在任务详情页添加导出按钮"
```

---

## 阶段 7: 测试与优化

### Task 23: 端到端测试

- [ ] **Step 1: 管理员流程测试**

1. 创建任务（各种配置组合）
2. 查看任务列表
3. 进入任务详情
4. 分享任务
5. 查看提交列表
6. 导出数据

- [ ] **Step 2: 参与者流程测试**

1. 通过分享卡片进入
2. 查看任务要求
3. 填写信息
4. 拍摄照片
5. 预览确认
6. 提交成功
7. 查看 AI 评估

- [ ] **Step 3: 边界情况测试**

1. 任务已截止
2. 重复提交
3. 网络异常
4. 照片过大
5. AI 评估失败

---

### Task 24: 性能优化

- [ ] **Step 1: 图片压缩**

在上传前压缩图片（JPEG 质量 85%）

- [ ] **Step 2: 列表分页**

任务列表和提交列表实现分页加载

- [ ] **Step 3: 缓存优化**

任务详情使用本地缓存，减少请求

- [ ] **Step 4: 提交优化**

```bash
git add miniprogram/
git commit -m "perf: 性能优化（图片压缩、分页、缓存）"
```

---

### Task 25: UI 优化

- [ ] **Step 1: 添加加载状态**

所有异步操作添加 loading 提示

- [ ] **Step 2: 添加空状态**

列表为空时显示友好提示

- [ ] **Step 3: 添加错误提示**

网络错误、权限错误等显示明确提示

- [ ] **Step 4: 优化交互动画**

页面切换、按钮点击添加过渡动画

- [ ] **Step 5: 提交优化**

```bash
git add miniprogram/
git commit -m "ui: UI优化（加载状态、空状态、错误提示）"
```

---

## 总结

### 开发时间估算

| 阶段 | 任务数 | 预估工期 |
|------|--------|----------|
| 阶段 1: 环境搭建 | 3 个任务 | 1 天 |
| 阶段 2: 云函数开发 | 3 个任务 | 2 天 |
| 阶段 3: 管理端开发 | 4 个任务 | 2.5 天 |
| 阶段 4: 参与端开发 | 6 个任务 | 3 天 |
| 阶段 5: AI 评估集成 | 3 个任务 | 1 天 |
| 阶段 6: 导出功能 | 3 个任务 | 1.5 天 |
| 阶段 7: 测试与优化 | 3 个任务 | 1 天 |
| **总计** | **25 个任务** | **12 天** |

### 关键里程碑

1. **Day 3**: 完成云函数开发，可以通过 API 测试工具验证
2. **Day 5.5**: 完成管理端，可以创建和管理任务
3. **Day 8.5**: 完成参与端，可以完整走通采集流程
4. **Day 9.5**: 完成 AI 评估，可以看到质量评分
5. **Day 11**: 完成导出功能，可以下载数据
6. **Day 12**: 完成测试优化，可以上线

### 技术要点

1. **云开发环境**: 使用微信云开发，无需自建服务器
2. **简化拍摄**: 使用 camera 组件，移除人脸检测（避免资质要求）
3. **AI 评估**: 异步调用通义千问-VL，评估照片质量并给出建议
4. **实时推送**: 使用云数据库实时监听获取评估结果
5. **文件管理**: 30天过期策略，导出后可选删除

### 注意事项

1. **环境变量**: 记得在云开发控制台配置 QWEN_API_KEY
2. **权限控制**: 所有云函数都要校验 _openid
3. **防重复提交**: 数据库唯一索引 taskId + _openid
4. **错误处理**: 所有异步操作都要 try-catch
5. **成本控制**: 开发阶段使用免费额度，注意监控用量

### 后续增强（可选）

- [ ] 照片裁剪调整功能
- [ ] 修改重传功能
- [ ] 过期提醒推送
- [ ] 导出历史记录
- [ ] 批量操作功能
- [ ] 数据分析统计

---

**计划创建时间**: 2026-04-08  
**预计完成时间**: 2026-04-20  
**计划版本**: 1.0

