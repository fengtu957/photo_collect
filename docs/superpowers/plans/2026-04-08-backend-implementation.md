# 批量证件照采集系统 - 实施计划（自建后端版）

> **架构调整**: 从微信云开发改为 Go + Kratos + MongoDB + MinIO 自建后端

**目标:** 构建 B2B2C 批量证件照采集管理系统

**技术栈:**
- 前端: 微信小程序 + TypeScript
- 后端: Go + Kratos 框架
- 数据库: MongoDB
- 存储: MinIO (S3 兼容)
- AI: 通义千问-VL API
- 部署: Docker Compose

---

## 项目结构

```
photo/
├── miniprogram/              # 小程序前端
├── backend/                  # Go 后端
│   ├── cmd/server/
│   ├── internal/
│   │   ├── conf/
│   │   ├── data/
│   │   ├── biz/
│   │   ├── service/
│   │   └── server/
│   ├── api/
│   └── go.mod
├── docker-compose.yml
└── docs/
```

---

## 阶段 1: 后端项目初始化

### Task 1: 创建 Go 项目结构

- [ ] **Step 1: 初始化 Go 模块**

```bash
mkdir backend
cd backend
go mod init photo-backend
```

- [ ] **Step 2: 安装 Kratos CLI**

```bash
go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
```

- [ ] **Step 3: 创建 Kratos 项目**

```bash
kratos new . --nomod
```

- [ ] **Step 4: 安装依赖**

```bash
go mod tidy
```

---

### Task 2: 配置 Docker Compose

- [ ] **Step 1: 创建 docker-compose.yml**

在项目根目录创建 `docker-compose.yml`

- [ ] **Step 2: 配置 MongoDB**

添加 MongoDB 服务配置

- [ ] **Step 3: 配置 MinIO**

添加 MinIO 服务配置

- [ ] **Step 4: 启动服务**

```bash
docker-compose up -d
```

---

### Task 3: 配置数据库连接

- [ ] **Step 1: 安装 MongoDB 驱动**

```bash
go get go.mongodb.org/mongo-driver/mongo
```

- [ ] **Step 2: 创建数据库配置**

在 `internal/conf/conf.proto` 添加 MongoDB 配置

- [ ] **Step 3: 实现数据库连接**

在 `internal/data/data.go` 实现 MongoDB 连接

- [ ] **Step 4: 测试连接**

```bash
go run cmd/server/main.go
```

---

## 阶段 2: API 定义与数据模型

### Task 4: 定义 API 接口

- [ ] **Step 1: 创建 task.proto**

在 `api/task/v1/task.proto` 定义任务相关 API

- [ ] **Step 2: 创建 submission.proto**

在 `api/submission/v1/submission.proto` 定义提交相关 API

- [ ] **Step 3: 生成代码**

```bash
make api
```

---

### Task 5: 实现数据模型

- [ ] **Step 1: 创建 Task 模型**

在 `internal/data/task.go` 定义 Task 结构体

- [ ] **Step 2: 创建 Submission 模型**

在 `internal/data/submission.go` 定义 Submission 结构体

- [ ] **Step 3: 创建索引**

实现 MongoDB 索引创建逻辑

---

## 阶段 3: 核心业务实现

### Task 6: 实现任务管理

- [ ] **Step 1: 实现创建任务**

在 `internal/biz/task.go` 实现业务逻辑

- [ ] **Step 2: 实现查询任务**

实现任务列表和详情查询

- [ ] **Step 3: 实现 Service 层**

在 `internal/service/task.go` 实现服务接口

---

### Task 7: 实现文件上传

- [ ] **Step 1: 配置 MinIO 客户端**

安装并配置 MinIO SDK

- [ ] **Step 2: 实现上传接口**

在 `internal/service/upload.go` 实现文件上传

- [ ] **Step 3: 生成访问 URL**

实现文件访问 URL 生成

---

### Task 8: 实现提交管理

- [ ] **Step 1: 实现提交照片**

在 `internal/biz/submission.go` 实现提交逻辑

- [ ] **Step 2: 实现查询提交**

实现提交列表查询

- [ ] **Step 3: 触发 AI 评估**

异步调用 AI 评估服务

---

## 阶段 4: AI 评估集成

### Task 9: 实现 AI 评估服务

- [ ] **Step 1: 创建通义千问客户端**

在 `pkg/qwen/client.go` 封装 API 调用

- [ ] **Step 2: 实现评估逻辑**

在 `internal/biz/evaluation.go` 实现评估业务

- [ ] **Step 3: 异步处理**

使用 goroutine 异步评估

---

## 阶段 5: 前端适配

### Task 10: 修改前端 API 调用

- [ ] **Step 1: 更新 request.ts**

修改为调用自建后端 API

- [ ] **Step 2: 实现微信登录**

实现小程序登录流程

- [ ] **Step 3: 更新上传逻辑**

修改文件上传为调用后端接口

---

## 阶段 6: 部署配置

### Task 11: 配置生产环境

- [ ] **Step 1: 配置 Nginx**

配置反向代理和 HTTPS

- [ ] **Step 2: 配置域名**

在小程序后台配置服务器域名

- [ ] **Step 3: 部署后端**

使用 Docker Compose 部署

---

**预计工期**: 8-10 天

