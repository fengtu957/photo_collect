# 批量证件照采集系统 - 后端服务

## 当前架构

- 后端: Go + `gorilla/mux`
- 数据库: MongoDB
- 文件存储: 七牛云
- AI: 通义千问-VL
- 认证: 微信登录 + JWT

当前仓库不是微信云开发方案，也不是 Kratos/MinIO 方案。接口由 [main.go](/mnt/d/code/latest/photo/backend/cmd/server/main.go) 启动，文件上传走七牛上传凭证接口。

## 环境变量

复制示例文件并填写实际配置：

```bash
cd backend
cp .env.example .env
```

必须配置的环境变量：

- `MONGODB_URI`
- `JWT_SECRET`
- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`
- `WECHAT_APPID`
- `WECHAT_SECRET`
- `QINIU_ACCESS_KEY`
- `QINIU_SECRET_KEY`
- `QINIU_BUCKET`
- `QINIU_DOMAIN`
- `QWEN_API_KEY`

## 本地开发

### 1. 准备 MongoDB

当前项目直接连接现有 MongoDB 服务，不依赖 Docker Compose。

### 2. 启动后端

```bash
cd backend
go run cmd/server/main.go
```

服务默认监听 `http://localhost:8000`。

### 3. 打开后台管理页

启动后可直接在浏览器访问：

```text
http://localhost:8000/admin
```

管理员登录使用环境变量 `ADMIN_USERNAME` / `ADMIN_PASSWORD`。

## API 清单

公开接口：

- `GET /admin`
- `POST /api/v1/auth/login`
- `POST /api/v1/admin/login`

需要 JWT 的接口：

- `POST /api/v1/tasks`
- `GET /api/v1/tasks`
- `GET /api/v1/tasks/{id}`
- `DELETE /api/v1/tasks/{id}`
- `POST /api/v1/submissions`
- `GET /api/v1/submissions/{id}`
- `PUT /api/v1/submissions/{id}`
- `GET /api/v1/tasks/{taskId}/submissions`
- `GET /api/v1/upload/token`

需要管理员 JWT 的接口：

- `GET /api/v1/admin/tasks`
- `POST /api/v1/admin/vip/grant`

## 联调说明

### 微信登录

`/api/v1/auth/login` 依赖真实的 `wx.login()` code，不能用任意 mock code 直接请求通过。

### 小程序联调地址

前端请求地址在 [request.ts](/mnt/d/code/latest/photo/miniprogram/utils/request.ts)。当前仓库使用固定测试地址；如果后续切换环境，应直接按真实联调环境修改。

### 七牛上传

上传流程是：

1. 小程序调用 `GET /api/v1/upload/token`
2. 小程序直接上传图片到七牛
3. 小程序再调用提交接口，把七牛 key 写入后端

## 当前缺口

- AI 评估结果尚未完整写回数据库
- 导出能力尚未实现
- 部分文档与配置仍在继续收敛
