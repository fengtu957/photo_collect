# 批量证件照采集系统 - 设计文档

**日期**: 2026-03-27
**版本**: 1.0
**状态**: 待审核

---

## 1. 项目概述

### 1.1 项目背景

基于参考项目（other/ 目录中的证件照小程序），开发一个批量证件照采集管理系统。该系统不是简单的证件照拍摄工具，而是一个 B2B2C 的采集管理平台。

### 1.2 核心需求

- **管理端（B）**: 创建采集任务、配置采集要求、查看统计、导出数据
- **参与端（C）**: 查看任务、填写信息、拍摄上传证件照

### 1.3 典型场景

- 学校批量采集学生证件照
- 企业批量采集员工工牌照片
- 活动组织者收集参与者照片
- 考试报名批量采集考生照片

---

## 2. 系统架构

### 2.1 技术选型

| 层级 | 技术方案 | 说明 |
|------|---------|------|
| **前端** | 微信小程序原生 + TypeScript | 使用 Skyline 渲染引擎 |
| **后端** | 微信云开发 | 云函数 + 云数据库 + 云存储 |
| **AI 能力** | 通义千问-VL | 照片质量评估 |
| **用户系统** | 微信授权登录 | 管理员和参与者都需登录 |
| **状态管理** | 简单全局状态 | 或 mobx-miniprogram |

**选择微信云开发的理由**:
- 快速开发，无需自建服务器
- 小规模使用成本低（甚至免费）
- 深度集成微信生态
- 自动扩容，无需运维

### 2.2 系统角色

**管理员（任务创建者）**:
- 创建采集任务
- 配置证件照规格、自定义字段
- 查看提交统计
- 导出数据（ZIP + Excel）

**参与者（提交者）**:
- 通过分享卡片进入任务
- 填写自定义字段
- 拍摄/上传证件照
- 查看 AI 评估结果
- 修改重传（截止时间前）

### 2.3 核心页面结构

```
管理员端:
├── 首页（任务列表）
├── 创建任务页
├── 任务详情页（管理视图）
└── 导出配置页

参与者端:
├── 任务详情页（参与视图）
├── 信息填写页
├── 拍摄页
├── 预览确认页
└── 提交成功页
```

---

## 3. 数据库设计

### 3.1 tasks（采集任务表）

```typescript
{
  _id: string,                    // 任务 ID
  _openid: string,                // 创建者 openid
  title: string,                  // 任务标题
  description: string,            // 备注说明

  // 证件照规格
  photoSpec: {
    name: string,                 // 规格名称（一寸、二寸等）
    width: number,                // 宽度（像素）
    height: number,               // 高度（像素）
    dpi: number,                  // 分辨率（可选）
  },

  // 时间范围
  startTime: Date,                // 开始时间
  endTime: Date,                  // 截止时间

  // 状态控制
  enabled: boolean,               // 是否开启

  // 额外字段配置
  customFields: [
    {
      id: string,                 // 字段 ID
      type: 'text' | 'select',    // 字段类型
      label: string,              // 字段名称
      required: boolean,          // 是否必填
      options?: string[],         // 多选选项（仅 select 类型）
      placeholder?: string,       // 提示文字
    }
  ],

  // 统计信息
  stats: {
    totalSubmissions: number,     // 总提交数
    lastSubmitTime: Date,         // 最后提交时间
  },

  // 存储配置
  storageConfig: {
    retentionDays: number,        // 照片保存天数（默认 30）
    expirationDate: Date,         // 过期时间
    autoDeleteAfterExport: boolean,  // 导出后是否自动删除
  },

  // 导出配置
  exportConfig: {
    nameTemplate: string,         // 文件名模板
    includeOriginal: boolean,     // 是否包含原图
  },

  createdAt: Date,
  updatedAt: Date,
}
```

**索引**:
- `_openid` - 查询我的任务
- `enabled, endTime` - 查询有效任务

### 3.2 submissions（提交记录表）

```typescript
{
  _id: string,
  _openid: string,                // 提交者 openid
  taskId: string,                 // 关联的任务 ID

  // 用户信息
  userInfo: {
    nickName: string,
    avatarUrl: string,
  },

  // 自定义字段数据
  customData: {
    [fieldId: string]: string | string[],
  },

  // 照片信息
  photo: {
    originalUrl: string,          // 原图云存储路径
    originalFileId: string,       // 原图云文件 ID
    fileSize: number,
    width: number,
    height: number,
    expiresAt: Date,              // 过期时间
    deleted: boolean,             // 是否已删除
    deletedAt: Date,              // 删除时间
    deletedReason: string,        // 删除原因
  },

  // AI 评估结果
  aiEvaluation: {
    status: 'pending' | 'success' | 'failed',
    score: number,                // 质量分数 0-100
    issues: string[],             // 问题列表
    suggestions: string[],        // 改进建议
    breakdown: {                  // 详细评分
      clarity: number,
      lighting: number,
      angle: number,
      background: number,
      expression: number,
      composition: number,
    },
    evaluatedAt: Date,
    error?: string,
  },

  status: 'draft' | 'submitted' | 'rejected',

  createdAt: Date,
  updatedAt: Date,
}
```

**索引**:
- `taskId, _openid` - 唯一索引，防重复提交
- `taskId, createdAt` - 按时间查询提交
- `_openid` - 查询我的提交历史

### 3.3 users（用户表 - 可选）

```typescript
{
  _id: string,
  _openid: string,
  nickName: string,
  avatarUrl: string,

  stats: {
    createdTasks: number,
    submissions: number,
  },

  createdAt: Date,
  lastLoginAt: Date,
}
```

### 3.4 export_history（导出历史表）

```typescript
{
  _id: string,
  _openid: string,
  taskId: string,
  fileName: string,
  fileSize: number,
  submissionCount: number,
  zipUrl: string,               // 下载链接
  exportedAt: Date,
  expiresAt: Date,              // 链接过期时间（7天）
}
```

---

## 4. 云函数设计

### 4.1 createTask（创建任务）

**输入**:
```typescript
{
  title: string,
  description: string,
  photoSpec: PhotoSpec,
  startTime: Date,
  endTime: Date,
  customFields: CustomField[],
  storageConfig: StorageConfig,
}
```

**输出**:
```typescript
{
  success: boolean,
  data: { taskId: string },
}
```

**逻辑**:
1. 校验参数
2. 计算过期时间
3. 写入 tasks 集合
4. 初始化统计信息

### 4.2 getTaskDetail（获取任务详情）

**输入**: `{ taskId: string }`

**输出**: 任务完整信息

**权限**:
- 创建者：返回完整信息 + 统计
- 参与者：返回基本信息（不含统计）

### 4.3 submitPhoto（提交照片）

**输入**:
```typescript
{
  taskId: string,
  customData: object,
  photo: {
    fileId: string,
    width: number,
    height: number,
    fileSize: number,
  }
}
```

**输出**:
```typescript
{
  success: boolean,
  data: { submissionId: string }
}
```

**逻辑**:
1. 校验任务状态（enabled、时间范围）
2. 检查是否已提交
3. 创建提交记录
4. 异步触发 AI 评估
5. 更新任务统计

### 4.4 evaluatePhoto（AI 评估 - 异步）

**输入**: `{ submissionId: string }`

**处理流程**:
1. 获取照片临时 URL
2. 调用通义千问-VL API
3. 解析评估结果
4. 更新 submission 的 aiEvaluation 字段

**Prompt 示例**:
```
你是一个专业的证件照质量评估专家。请评估这张证件照的质量。

证件照规格要求：
- 尺寸：一寸照（295x413像素）
- 用途：证件照

请从以下维度进行评估（每项 0-100 分）：
1. 人脸清晰度
2. 光线质量
3. 人脸角度
4. 背景干净度
5. 表情规范性
6. 构图合理性

返回严格的 JSON 格式：
{
  "score": 85,
  "breakdown": { ... },
  "issues": ["光线略显不足"],
  "suggestions": ["建议在自然光充足的环境重拍"]
}
```

### 4.5 updateSubmission（修改提交）

**输入**:
```typescript
{
  submissionId: string,
  newPhoto?: PhotoInfo,
  newCustomData?: object,
}
```

**权限**: 仅提交者本人，且任务在时间范围内

**逻辑**:
1. 校验权限
2. 更新字段
3. 如果照片变更，重新触发 AI 评估

### 4.6 exportTask（导出任务）

**输入**:
```typescript
{
  taskId: string,
  nameTemplate: string,
  exportOptions: {
    includeOriginal: boolean,
    includeData: boolean,
    imageFormat: 'jpg' | 'png',
    deleteAfterExport: boolean,
  }
}
```

**输出**:
```typescript
{
  success: boolean,
  data: {
    zipUrl: string,
    fileName: string,
    fileSize: number,
    expiresAt: Date,
  }
}
```

**处理流程**:
1. 权限校验（仅创建者）
2. 查询所有提交记录
3. 下载照片并按模板重命名
4. 生成 Excel 数据表
5. 打包成 ZIP
6. 上传到云存储（7天过期）
7. 返回临时下载链接
8. 可选：导出后删除照片

**文件命名模板变量**:
- `{name}` - 姓名（来自自定义字段）
- `{id}` - 提交 ID
- `{date}` - 提交日期
- `{time}` - 提交时间
- `{index}` - 序号
- `{自定义字段ID}` - 任何自定义字段

### 4.7 cleanExpiredPhotos（清理过期照片 - 定时）

**触发**: 每天凌晨 2:00

**逻辑**:
1. 查询已过期的提交记录
2. 批量删除云存储文件
3. 更新数据库标记

---

## 5. 核心功能设计

### 5.1 拍摄功能

**参考实现**: other/pages/camera/index.js

**技术方案**:
- 使用 `<camera>` 组件
- 使用 `wx.createVKSession` 进行人脸检测
- 实时显示人脸识别框
- 检测到人脸时拍照按钮高亮

**组件设计**: camera-capture

```typescript
props: {
  spec: PhotoSpec,
  onCapture: (filePath: string) => void,
}

功能:
- 前后摄像头切换
- 相册选择
- 实时人脸检测
- 拍照预览
```

**优化**:
- 降低检测频率（500ms）以节省性能
- 拍照成功后立即停止检测

### 5.2 AI 质量评估

**模型选择**: 通义千问-VL（推荐）

**理由**:
- 国内访问快速，无需代理
- 成本低（¥0.004/张）
- 视觉理解能力强

**评估维度**:
1. 人脸清晰度（五官是否清晰）
2. 光线质量（充足、均匀）
3. 人脸角度（正面、端正）
4. 背景干净度（无杂物）
5. 表情规范性（自然、闭嘴）
6. 构图合理性（居中、合适）

**交互流程**:
1. 参与者提交照片
2. 显示"提交成功"
3. 异步 AI 评估（2-5秒）
4. 实时数据库推送结果
5. 如果评分 < 70：提示"建议重拍"
6. 如果评分 >= 70：鼓励完成

### 5.3 图片生命周期管理

**策略**: 30天过期（可配置）

**实现**:
1. 创建任务时配置 retentionDays
2. 上传照片时记录 expiresAt
3. 定时云函数每天清理
4. 任务详情页显示倒计时
5. 过期前 3 天推送提醒

**导出后删除**（可选）:
- 配置 `autoDeleteAfterExport: true`
- 导出成功后立即删除照片
- 释放存储空间

**成本优化**:
- 永久保存：100GB → ¥65/月
- 30天过期：50GB → ¥32.5/月（节省 50%）
- 导出后删除：23GB → ¥15/月（节省 77%）

### 5.4 分享机制

**方式**: 微信分享卡片

**实现**:
```typescript
onShareAppMessage() {
  return {
    title: `【证件照采集】${task.title}`,
    path: `/pages/task-view/index?taskId=${task._id}`,
    imageUrl: task.shareImage || '/static/share-default.png',
  };
}
```

**流程**:
1. 管理员创建任务
2. 点击"分享"按钮
3. 生成微信分享卡片（携带 taskId）
4. 参与者点击卡片
5. 小程序打开任务详情页
6. 开始采集流程

### 5.5 权限控制

**原则**: 仅创建者可管理

**实现**:
- 云函数中校验 `_openid`
- 查询提交列表：仅创建者
- 导出数据：仅创建者
- 修改任务：仅创建者

**防重复提交**:
- 数据库唯一索引：`taskId + _openid`
- 提交前检查是否已存在
- 返回友好提示："请使用修改提交功能"

### 5.6 导出功能

**格式**: ZIP 包含：
- `photos/` 目录：重命名后的照片
- `提交数据.xlsx`：Excel 数据表

**Excel 数据列**:
- 序号、提交时间、微信昵称、提交ID
- 所有自定义字段
- AI评分、问题、建议

**文件命名示例**:
- 模板：`{name}_{id}`
- 结果：`张三_abc123.jpg`

**下载方式**:
- 生成 7 天有效的临时链接
- 复制链接到浏览器下载
- 支持导出历史记录

---

## 6. 组件设计

### 6.1 task-card（任务卡片）

展示任务基本信息，用于列表页

**Props**:
- task: Task
- mode: 'manage' | 'view'

**显示内容**:
- 任务标题、规格、时间范围
- 状态标签（进行中/已结束）
- [管理模式] 提交统计

### 6.2 photo-spec-selector（规格选择器）

创建任务时选择证件照规格

**预设规格**:
- 一寸：295 x 413
- 小一寸：260 x 378
- 二寸：413 x 626
- 小二寸：413 x 579
- 自定义：输入宽高

### 6.3 custom-field-builder（字段构建器）

配置额外字段（文本/多选）

**功能**:
- 添加/编辑/删除字段
- 拖拽排序
- 配置必填、选项

### 6.4 camera-capture（相机组件）

**功能**:
- 实时人脸检测
- 前后摄像头切换
- 相册选择
- 拍照预览

### 6.5 photo-cropper（裁剪器）

**功能**:
- 拖拽调整位置
- 双指缩放
- 固定比例裁剪框

### 6.6 ai-evaluation-card（评估卡片）

显示 AI 评估结果

**状态**:
- pending: 加载动画 + "AI 评估中..."
- success: 分数（环形进度条）+ 问题 + 建议
- failed: 错误提示

### 6.7 submission-list（提交列表）

管理员查看提交记录

**功能**:
- 分页加载
- 显示头像、昵称、时间、评分
- 点击查看详情

---

## 7. UI 设计风格

### 7.1 设计原则

**简化设计**: 保留核心功能，用更简洁的 UI 呈现

**设计规范**:
- 使用微信 WeUI 风格
- 主色调：蓝色（#1989FA）
- 卡片式布局，圆角 12px
- 大间距，留白充足
- 图标使用 iconfont
- 避免复杂动画

### 7.2 关键页面布局

**首页（任务列表）**:
- 顶部：标题 + 创建按钮
- 列表：任务卡片（滚动）
- 底部：TabBar（我的任务/我的提交）

**创建任务页**:
- 分段表单
- 即时预览
- 底部固定"创建"按钮

**任务详情（管理视图）**:
- 顶部：任务信息卡片
- 统计卡片：提交数、过期倒计时
- 提交列表
- 底部：分享、导出按钮

**拍摄页**:
- 全屏相机
- 中央人脸识别框（半透明）
- 底部工具栏：相册、拍照、切换

---

## 8. 核心业务流程

### 8.1 创建并分享任务

```
管理员
  → 填写任务信息
  → 选择证件照规格
  → 配置自定义字段
  → 设置时间范围
  → 创建成功
  → 点击"分享"
  → 生成微信分享卡片
  → 发送到群/好友
```

### 8.2 参与者提交照片

```
参与者
  → 点击分享卡片
  → 进入任务详情页
  → 查看采集要求
  → 点击"开始采集"
  → 填写自定义字段
  → 点击"拍摄证件照"
  → 打开相机（人脸检测）
  → 拍照确认
  → 裁剪调整
  → 上传到云存储
  → 提交成功
  → AI 评估中...
  → 显示评估结果
  → 如果评分低：提示"建议重拍"
  → 确认提交 / 修改重拍
```

### 8.3 修改重传

```
参与者
  → 我的提交列表
  → 点击某个提交
  → 查看 AI 评估结果
  → 点击"修改提交"
  → 重新拍摄
  → 上传并评估
  → 更新提交
```

### 8.4 导出数据

```
管理员
  → 任务详情页
  → 点击"导出"
  → 配置文件名模板
  → 选择导出选项
  → 点击"开始导出"
  → 云函数生成 ZIP
  → 返回下载链接
  → 复制链接
  → 在浏览器下载
```

---

## 9. 成本估算

### 9.1 微信云开发成本

**免费额度**（基础版 1）:
- 数据库存储：2GB
- 数据库读操作：50,000 次/天
- 数据库写操作：30,000 次/天
- 云存储容量：5GB
- 云存储下载流量：5GB/月

### 9.2 实际使用估算

**小规模（100 个任务，5,000 张照片）**:
- 数据库存储：18MB（0.9%）
- 数据库读操作：2,100 次/天（4.2%）
- 数据库写操作：350 次/天（1.2%）
- 云存储容量：10GB
- AI 评估：5,000 次 × ¥0.004 = ¥20

**月总成本**: 约 **¥20**（主要是 AI 成本）

**中等规模（1,000 个任务，50,000 张照片）**:
- 数据库：基本免费（优化后）
- 云存储：约 ¥32.5（30天过期策略）
- AI 评估：50,000 次 × ¥0.004 = ¥200

**月总成本**: 约 **¥232.5**

**大规模（10,000 个任务，500,000 张照片）**:
- 数据库：约 ¥1/月
- 云存储：约 ¥65/月
- AI 评估：500,000 次 × ¥0.004 = ¥2,000

**月总成本**: 约 **¥2,066**

### 9.3 成本优化建议

**降低 AI 成本**:
1. 本地预检：使用微信小程序的人脸检测 API 先筛选
2. 分级评估：只对低分照片调用详细评估
3. 批量评估：争取批量折扣

**降低存储成本**:
1. 30天过期策略（节省 50%）
2. 导出后删除（节省 77%）
3. 图片压缩（JPEG 质量 85%）

---

## 10. 技术难点与解决方案

### 10.1 实时人脸检测性能

**问题**: VKSession 持续检测消耗性能

**解决方案**:
- 降低检测频率（500ms）
- 检测到人脸后才高亮拍照按钮
- 拍照成功后立即停止检测

### 10.2 大模型评估延迟

**问题**: 2-5秒等待影响体验

**解决方案**:
- 异步评估，先显示"提交成功"
- 使用实时数据库推送结果
- 评分低时提示修改，不阻塞流程

### 10.3 云函数内存限制

**问题**: 导出大量照片时内存不足

**解决方案**:
- 批量下载（每次10张）
- 流式压缩，边下载边打包
- 使用临时文件而非内存缓存

### 10.4 照片过期管理

**问题**: 定时清理可能遗漏

**解决方案**:
- 定时云函数每天执行
- 任务详情页实时显示倒计时
- 过期前 3 天推送提醒
- 导出前二次确认

---

## 11. 项目目录结构

```
photo/
├── miniprogram/
│   ├── app.ts
│   ├── app.json
│   ├── app.wxss
│   ├── pages/
│   │   ├── index/                    # 首页
│   │   ├── task-create/              # 创建任务
│   │   ├── task-detail/              # 任务详情（管理）
│   │   ├── task-view/                # 任务详情（参与）
│   │   ├── task-export/              # 导出配置
│   │   ├── submission-form/          # 信息填写
│   │   ├── camera/                   # 拍摄页面
│   │   ├── photo-crop/               # 照片裁剪
│   │   ├── submission-preview/       # 预览确认
│   │   └── submission-success/       # 提交成功
│   ├── components/
│   │   ├── task-card/
│   │   ├── photo-spec-selector/
│   │   ├── custom-field-builder/
│   │   ├── camera-capture/
│   │   ├── photo-cropper/
│   │   ├── ai-evaluation-card/
│   │   └── submission-list/
│   ├── utils/
│   ├── services/
│   ├── types/
│   └── static/
│
├── cloudfunctions/
│   ├── createTask/
│   ├── getTaskDetail/
│   ├── getTaskSubmissions/
│   ├── submitPhoto/
│   ├── evaluatePhoto/
│   ├── updateSubmission/
│   ├── exportTask/
│   ├── cleanExpiredPhotos/
│   └── common/
│
├── docs/
│   └── superpowers/
│       └── specs/
│           └── 2026-03-27-photo-collection-design.md
│
├── package.json
├── project.config.json
└── tsconfig.json
```

---

## 12. 开发计划

### 12.1 MVP 阶段（核心功能）

**目标**: 实现完整的采集流程

**功能清单**:
- [x] 用户登录（微信授权）
- [x] 创建任务（基础配置）
- [x] 任务分享（分享卡片）
- [x] 信息填写（自定义字段）
- [x] 照片拍摄（相机 + 人脸检测）
- [x] 照片上传（云存储）
- [x] AI 评估（大模型）
- [x] 查看统计（提交数量）
- [x] 导出数据（ZIP + Excel）

**预估工作量**: 11 天

| 模块 | 工作量 |
|------|--------|
| 基础框架 | 0.5天 |
| 用户系统 | 0.5天 |
| 任务管理 | 2天 |
| 拍摄采集 | 2天 |
| AI 评估 | 1天 |
| 导出功能 | 2天 |
| 组件开发 | 2天 |
| 测试优化 | 1天 |

### 12.2 增强阶段（优化体验）

**功能清单**:
- [ ] 照片裁剪调整
- [ ] 修改重传
- [ ] 过期提醒
- [ ] 导出历史
- [ ] 批量操作

**预估工作量**: 3-5 天

### 12.3 扩展阶段（高级功能）

**功能清单**:
- [ ] 多管理员协作
- [ ] 数据分析统计
- [ ] 自定义分享图
- [ ] 导出模板保存

**预估工作量**: 按需开发

---

## 13. 风险与应对

### 13.1 技术风险

**风险 1**: 大模型 API 不稳定
- **应对**: 实现重试机制，评估失败不影响提交

**风险 2**: 云函数内存/时长限制
- **应对**: 分批处理，使用流式处理

**风险 3**: 云存储成本超预期
- **应对**: 严格执行过期策略，导出后删除

### 13.2 业务风险

**风险 1**: 照片质量参差不齐
- **应对**: AI 评估 + 建议重拍

**风险 2**: 参与者忘记提交
- **应对**: 截止前推送提醒（可选）

**风险 3**: 导出文件过大无法下载
- **应对**: 分批导出，或压缩率调整

---

## 14. 后续优化方向

### 14.1 功能扩展

1. **背景处理**: 调用 AI 自动抠图换背景
2. **美颜功能**: 轻度美颜，保持真实
3. **批量打印**: 对接打印服务
4. **数据分析**: 提交趋势、质量分布

### 14.2 性能优化

1. **CDN 加速**: 照片下载使用 CDN
2. **图片懒加载**: 列表页延迟加载
3. **本地缓存**: 任务信息缓存

### 14.3 用户体验

1. **骨架屏**: 加载时显示占位
2. **操作引导**: 首次使用引导动画
3. **错误提示**: 友好的错误信息

---

## 15. 总结

### 15.1 系统特点

1. **简化设计**: 专注核心流程，UI 清晰简洁
2. **低成本运营**: 利用微信云开发，小规模使用几乎免费
3. **智能质量控制**: 大模型评估照片质量，减少无效提交
4. **灵活配置**: 自定义字段、命名规则、过期时间
5. **完整闭环**: 创建 → 分享 → 采集 → 导出

### 15.2 技术亮点

- 微信云开发全栈方案
- 大模型智能评估
- 实时人脸检测
- 灵活的导出配置
- 智能的存储管理

### 15.3 适用场景

- 学校、企业、组织批量采集
- 中小规模（< 10,000 人）
- 对成本敏感的场景
- 需要快速上线的项目

---

**文档结束**

生成时间: 2026-03-27
版本: 1.0
