# 批量证件照采集系统 - 实施计划（当前仓库版）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or an equivalent task-by-task execution workflow. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标:** 在当前仓库基础上，完成一个可上线的批量证件照采集系统，覆盖任务创建、参与者上传、AI 质量评估、管理端查看与导出。

**当前架构:** 微信小程序原生 + TypeScript + Go HTTP API + MongoDB + 七牛云 + 通义千问-VL

**当前日期:** 2026-04-09

---

## 当前仓库现状

### 已有能力

- [x] 小程序已具备任务列表、任务创建、任务详情、照片上传、自定义字段编辑等基础页面
- [x] 后端已具备微信登录、JWT 鉴权、任务创建/列表/详情/删除、提交创建/编辑/列表/详情、上传 token 获取
- [x] MongoDB 数据模型、业务层、服务层已搭建完成
- [x] 七牛上传链路已接入，前端可以直接上传到七牛后再提交后端
- [x] 前后端字段命名已开始统一为 snake_case

### 当前主要缺口

- [ ] 文档和部署配置仍有历史漂移，部分内容仍指向云开发、MinIO、Kratos 等旧方案
- [ ] AI 评估逻辑尚未闭环，评估结果没有写回 submission
- [ ] 管理端缺少分享、导出、参与视图等完整业务闭环
- [ ] 前端页面和类型仍有少量 `any` 与旧逻辑遗留
- [ ] 缺少系统化的联调、回归和上线前检查

---

## 目标架构

### 前端

```
miniprogram/
├── pages/
│   ├── task-list/                 # 任务列表
│   ├── task-create/               # 创建任务
│   ├── task-detail/               # 管理视图
│   ├── photo-upload/              # 上传/编辑提交
│   ├── custom-fields/             # 字段列表编辑
│   └── field-edit/                # 字段项编辑
├── services/
├── utils/
└── types/
```

### 后端

```
backend/
├── cmd/server/main.go             # HTTP 服务入口
├── internal/
│   ├── data/                      # MongoDB 模型与仓储
│   ├── biz/                       # 核心业务
│   └── service/                   # HTTP 处理与鉴权
└── pkg/qwen.go                    # 通义千问客户端
```

### 外部依赖

- MongoDB: 任务与提交数据
- 七牛云: 原始照片存储与私有下载链接
- 微信登录接口: `code -> openid`
- 通义千问-VL: 证件照质量评估

---

## 阶段 1: 基线收敛与文档修正

### Task 1: 对齐仓库文档与真实架构

**目标:** 去掉“云开发/云函数/MinIO/Kratos”的误导信息，统一为当前 Go 后端方案

- [ ] **Step 1: 清理实施计划、README、架构文档中的历史描述**
  - 更新 `docs/superpowers/plans/2026-04-08-photo-collection-implementation.md`
  - 更新 `backend/README.md`
  - 复核 `docs/superpowers/plans/2026-04-08-backend-architecture.md`

- [ ] **Step 2: 明确当前 API 清单**
  - 登录: `POST /api/v1/auth/login`
  - 任务: `POST/GET/GET by id/DELETE /api/v1/tasks`
  - 提交: `POST/GET by id/PUT /api/v1/submissions`
  - 列表: `GET /api/v1/tasks/{taskId}/submissions`
  - 上传: `GET /api/v1/upload/token`

- [ ] **Step 3: 统一环境变量说明**
  - MongoDB
  - JWT
  - 微信小程序 `WECHAT_APPID` / `WECHAT_SECRET`
  - 七牛 `QINIU_ACCESS_KEY` / `QINIU_SECRET_KEY` / `QINIU_BUCKET` / `QINIU_DOMAIN`
  - 通义千问 `QWEN_API_KEY`

- [ ] **Step 4: 提交文档修正**

```bash
git add docs/ backend/README.md
git commit -m "docs: 对齐当前 Go 后端方案文档"
```

### Task 2: 收敛配置与联调基线

**目标:** 让本地开发和联调方式可复制、可验证

- [ ] **Step 1: 明确当前 MongoDB 与后端联调方式**
  - 使用现有 MongoDB 服务
  - 删除无效部署说明
  - 避免继续引入 Docker Compose 依赖

- [ ] **Step 2: 复核 `backend/.env.example`**
  - 去掉不应提交的真实敏感信息
  - 保留占位值和说明

- [ ] **Step 3: 统一前端后端联调地址配置**
  - 收敛 `miniprogram/utils/request.ts` 的 `BASE_URL`
  - 预留开发、测试、生产环境切换方案

- [ ] **Step 4: 提交配置基线**

```bash
git add backend/.env.example miniprogram/utils/request.ts docs/
git commit -m "chore: 收敛联调配置基线"
```

---

## 阶段 2: 前后端契约稳定化

### Task 3: 统一前端类型与后端返回结构

**目标:** 所有核心类型统一使用当前后端真实字段

- [x] **Step 1: 统一 `Task` 与 `Submission` 的 snake_case 字段**
- [x] **Step 2: 修复上传页编辑模式读取提交详情**
- [ ] **Step 3: 清理仍残留的旧字段命名和 `any`**
  - `miniprogram/pages/task-list/task-list.ts`
  - `miniprogram/pages/task-detail/task-detail.ts`
  - `miniprogram/pages/photo-upload/photo-upload.ts`
  - `miniprogram/services/task.ts`

- [ ] **Step 4: 补充分页和列表响应类型**
  - 任务列表
  - 提交列表
  - 通用接口响应

- [ ] **Step 5: 提交契约稳定化**

```bash
git add miniprogram/types/ miniprogram/services/ miniprogram/pages/
git commit -m "refactor: 收敛前后端类型契约"
```

### Task 4: 补齐后端接口健壮性

**目标:** 所有已开放 API 都有明确的权限、空值和错误处理

- [ ] **Step 1: 复核任务接口**
  - 创建时参数校验
  - 查询不存在任务时返回明确错误
  - 删除任务时处理关联提交

- [ ] **Step 2: 复核提交接口**
  - 创建时任务状态校验
  - 更新时权限校验
  - 列表与详情都处理任务不存在场景

- [ ] **Step 3: 统一 HTTP 错误语义**
  - 鉴权失败
  - 参数错误
  - 资源不存在
  - 权限不足

- [ ] **Step 4: 提交接口健壮性改造**

```bash
git add backend/internal/
git commit -m "fix: 完善任务与提交接口健壮性"
```

---

## 阶段 3: 完善管理端流程

### Task 5: 打磨任务列表页

**目标:** 让管理员能清晰看到“我创建的任务”和“我参与的任务”

- [ ] **Step 1: 任务卡片信息收敛**
  - 标题
  - 描述/规格
  - 创建时间
  - 截止时间
  - 提交数

- [ ] **Step 2: 增加任务状态展示**
  - 进行中
  - 未开始
  - 已截止

- [ ] **Step 3: 处理空状态和加载失败**

- [ ] **Step 4: 评估是否需要拆 `task-card` 组件**

- [ ] **Step 5: 提交任务列表页优化**

```bash
git add miniprogram/pages/task-list/
git commit -m "feat: 优化任务列表页"
```

### Task 6: 打磨任务创建页与字段编辑

**目标:** 稳定任务创建体验，避免产生脏数据

- [ ] **Step 1: 增加任务表单校验**
  - 标题必填
  - 截止时间必填
  - 开始时间不能晚于截止时间
  - 规格宽高必须为正数

- [ ] **Step 2: 收敛自定义字段编辑**
  - 字段类型限制
  - 必填项校验
  - 选项为空校验
  - 最多字段数说明

- [ ] **Step 3: 复核页面样式符合小程序规则**
  - input 高度
  - `adjust-position="{{false}}"`
  - 底部按钮与滚动区关系

- [ ] **Step 4: 提交创建页与字段页优化**

```bash
git add miniprogram/pages/task-create/ miniprogram/pages/custom-fields/ miniprogram/pages/field-edit/
git commit -m "feat: 完善任务创建与字段编辑流程"
```

### Task 7: 完善任务详情管理视图

**目标:** 管理员能够完整查看任务和提交情况

- [ ] **Step 1: 完善任务信息展示**
  - 规格
  - 时间范围
  - 自定义字段
  - 提交统计

- [ ] **Step 2: 完善提交列表**
  - 头像昵称
  - 提交时间
  - 自定义信息
  - AI 评估状态与分数

- [ ] **Step 3: 增加分享能力**
  - 自定义分享文案
  - 分享路径带上任务 ID

- [ ] **Step 4: 预留导出入口**

- [ ] **Step 5: 提交任务详情页优化**

```bash
git add miniprogram/pages/task-detail/
git commit -m "feat: 完善任务详情管理视图"
```

---

## 阶段 4: 完善参与者流程

### Task 8: 明确参与者入口

**目标:** 让非创建者通过分享链接进入任务并提交资料

- [ ] **Step 1: 决定参与者入口形态**
  - 方案 A: 复用 `task-detail`，按用户身份区分视图
  - 方案 B: 新增 `task-view` 页面做轻量参与视图

- [ ] **Step 2: 实现路径参数与分享回流**
  - `taskId`
  - 可选 `fromShare`

- [ ] **Step 3: 明确参与者可见信息**
  - 标题
  - 规格要求
  - 截止时间
  - 自定义字段说明

- [ ] **Step 4: 提交参与者入口**

```bash
git add miniprogram/pages/
git commit -m "feat: 添加参与者任务入口"
```

### Task 9: 打磨上传与编辑提交流程

**目标:** 让参与者可以稳定提交和修改照片

- [x] **Step 1: 修复编辑已有提交时错误读取分页结果的 bug**
- [x] **Step 2: 支持编辑时不重新上传照片直接提交**
- [x] **Step 3: 修复底部固定按钮与滚动内容重叠问题**
- [ ] **Step 4: 增加任务状态校验提示**
  - 已截止
  - 任务不存在
  - 无权限

- [ ] **Step 5: 完善图片元信息**
  - 宽高
  - 大小
  - 可选文件格式

- [ ] **Step 6: 评估是否拆成“表单页 + 预览页”**
  - 当前可保留单页上传
  - 若交互复杂再拆页

- [ ] **Step 7: 提交上传流程优化**

```bash
git add miniprogram/pages/photo-upload/ backend/internal/
git commit -m "feat: 完善上传与编辑提交流程"
```

---

## 阶段 5: AI 评估闭环

### Task 10: 完成通义千问评估结果写回

**目标:** 提交后能异步得到可展示的评估结果

- [ ] **Step 1: 完善 `backend/pkg/qwen.go`**
  - 请求错误处理
  - 响应解析容错
  - 非 JSON 输出兜底

- [ ] **Step 2: 完成 `backend/internal/biz/evaluation.go`**
  - 解析评估结果
  - 写回 submission 的 `ai_evaluation`
  - 设置 `evaluated_at`

- [ ] **Step 3: 设计触发方式**
  - 创建提交后异步触发
  - 更新提交后重新触发

- [ ] **Step 4: 增加失败重试与错误记录**
  - 失败状态
  - 错误消息
  - 可控重试次数

- [ ] **Step 5: 提交 AI 评估闭环**

```bash
git add backend/pkg/qwen.go backend/internal/biz/evaluation.go backend/internal/
git commit -m "feat: 完成 AI 评估闭环"
```

### Task 11: 前端展示 AI 评估结果

**目标:** 管理员和提交者都能看懂评估结果

- [ ] **Step 1: 在任务详情页展示评估状态**
  - pending
  - success
  - failed

- [ ] **Step 2: 展示分项得分、问题和建议**

- [ ] **Step 3: 低分场景给出重拍引导**

- [ ] **Step 4: 评估是否需要轮询刷新**
  - 页面返回刷新
  - 定时轮询
  - 手动刷新

- [ ] **Step 5: 提交评估结果展示**

```bash
git add miniprogram/pages/task-detail/
git commit -m "feat: 展示 AI 评估结果"
```

---

## 阶段 6: 导出功能

### Task 12: 设计导出接口与文件结构

**目标:** 管理员可以导出照片和结构化数据

- [ ] **Step 1: 明确导出范围**
  - 仅管理员可导出
  - 导出所有提交
  - 打包照片 + Excel/CSV

- [ ] **Step 2: 定义后端接口**
  - 同步导出还是异步导出
  - 下载链接有效期
  - 文件命名模板

- [ ] **Step 3: 设计导出目录结构**
  - `photos/`
  - `metadata.xlsx` 或 `metadata.csv`

- [ ] **Step 4: 提交导出设计**

```bash
git add docs/ backend/
git commit -m "docs: 设计导出功能接口与文件结构"
```

### Task 13: 实现导出能力

**目标:** 后端可生成下载包，前端可触发导出

- [ ] **Step 1: 后端实现导出服务**
  - 查询任务与提交
  - 下载七牛私有文件
  - 重命名
  - 打包 ZIP
  - 上传导出结果

- [ ] **Step 2: 前端增加导出入口**
  - 任务详情页按钮
  - 导出中提示
  - 导出成功提示

- [ ] **Step 3: 处理大任务导出策略**
  - 批量下载
  - 超时处理
  - 重试

- [ ] **Step 4: 提交导出能力**

```bash
git add backend/ miniprogram/pages/task-detail/
git commit -m "feat: 实现任务导出能力"
```

---

## 阶段 7: 联调、测试与上线准备

### Task 14: 端到端联调

**目标:** 用真实小程序和真实后端走通主流程

- [ ] **Step 1: 管理员流程**
  - 登录
  - 创建任务
  - 配置字段
  - 查看任务详情
  - 分享任务

- [ ] **Step 2: 参与者流程**
  - 进入分享任务
  - 填写信息
  - 上传照片
  - 编辑提交

- [ ] **Step 3: AI 流程**
  - 提交后进入 pending
  - 成功后刷新为 score / issues / suggestions

- [ ] **Step 4: 导出流程**
  - 发起导出
  - 获取下载链接
  - 校验文件内容

### Task 15: 回归与兼容性测试

**目标:** 降低小程序环境下的回归风险

- [ ] **Step 1: 真机验证关键页面**
  - 任务列表
  - 创建任务
  - 任务详情
  - 上传页

- [ ] **Step 2: 验证小程序兼容性约束**
  - 不使用可选链
  - 输入框 `adjust-position="{{false}}"`
  - 固定底栏不遮挡内容

- [ ] **Step 3: 覆盖边界情况**
  - 任务不存在
  - 已截止任务
  - 重复提交
  - 七牛上传失败
  - 微信登录失败
  - AI 评估失败

### Task 16: 上线准备

**目标:** 让部署和域名配置具备上线条件

- [ ] **Step 1: 配置 HTTPS 域名**
  - request 合法域名
  - uploadFile 合法域名
  - downloadFile 合法域名

- [ ] **Step 2: 生产环境部署**
  - 后端
  - MongoDB
  - 七牛配置
  - 环境变量

- [ ] **Step 3: 最终检查**
  - 日志
  - 错误监控
  - 数据备份

- [ ] **Step 4: 提交上线准备**

```bash
git add .
git commit -m "chore: 完成上线前联调与部署准备"
```

---

## 里程碑

1. **M1: 契约稳定**
   - 文档与配置不再漂移
   - 前后端字段统一
   - 上传编辑流程稳定

2. **M2: 核心业务闭环**
   - 管理员可创建任务并查看提交
   - 参与者可上传并编辑照片

3. **M3: AI 闭环**
   - 提交后能看到评估结果
   - 失败场景可追踪

4. **M4: 交付能力**
   - 管理员可导出数据
   - 完成真机测试和部署准备

---

## 风险与注意事项

1. **当前计划以仓库真实代码为准**
   - 不再引入云函数、云数据库、云存储方案

2. **当前后端不是 Kratos 项目**
   - 采用 `gorilla/mux + service/biz/data` 的现有实现继续推进

3. **七牛链路是当前正式上传路径**
   - 文档和配置不得再混入 MinIO 方案，除非后续明确切换

4. **字段命名必须保持 snake_case**
   - 与 Go API、MongoDB 文档结构保持一致

5. **小程序兼容性必须优先**
   - 不使用 `?.`
   - 输入框样式和底部固定栏遵守现有规则

---

**计划创建时间:** 2026-04-08
**计划修订时间:** 2026-04-09
**计划版本:** 2.0
