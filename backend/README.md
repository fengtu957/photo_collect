# 批量证件照采集系统 - 后端服务

## 当前架构

- 后端: Go + `gorilla/mux`
- 数据库: MongoDB
- 文件存储: 阿里云 OSS
- AI: 通义千问-VL
- 认证: 微信登录 + JWT

当前仓库不是微信云开发方案，也不是 Kratos/MinIO 方案。接口由 `cmd/server/main.go` 启动，照片由小程序使用后端签发的精确 key 策略直传阿里云 OSS。

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
- `ALIYUN_ACCESS_KEY_ID`
- `ALIYUN_ACCESS_KEY_SECRET`
- `ALIYUN_OSS_BUCKET`
- `ALIYUN_OSS_ENDPOINT`（可选，默认 `oss-cn-shanghai.aliyuncs.com`）
- `ALIYUN_OSS_TEMP_PREFIX`（可选，默认 `photo-temp`）
- `ALIYUN_OSS_PHOTO_PREFIX`（可选，默认 `photos`）
- `ALIYUN_OSS_EXPORT_PREFIX`（可选，默认 `exports`）
- `ALIYUN_OSS_EXPORT_JOB_PREFIX`（可选，默认 `export-jobs`）
- `ALIYUN_IMAGESEG_ENDPOINT`（可选，默认 `https://imageseg.cn-shanghai.aliyuncs.com/`）
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
http://localhost:8000/paper/hinge-58241/entry
```

管理员登录使用环境变量 `ADMIN_USERNAME` / `ADMIN_PASSWORD`。

## API 清单

公开接口：

- `GET /paper/hinge-58241/entry`
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
- `POST /api/v1/upload/policy`
- `POST /api/v1/photos/finalize`
- `POST /api/v1/photos/segment`
- `POST /api/v1/tasks/{id}/export`
- `POST /api/v1/tasks/{id}/export/status`
- `POST /api/v1/tasks/{id}/export/authorize`

需要管理员 JWT 的接口：

- `GET /api/v1/admin/tasks`
- `POST /api/v1/admin/vip/grant`

## 联调说明

### 微信登录

`/api/v1/auth/login` 依赖真实的 `wx.login()` code，不能用任意 mock code 直接请求通过。

### 小程序联调地址

前端请求地址在 [request.ts](/mnt/d/code/latest/photo/miniprogram/utils/request.ts)。当前仓库使用固定测试地址；如果后续切换环境，应直接按真实联调环境修改。

### OSS 照片流程

1. 小程序调用 `POST /api/v1/upload/policy` 获取临时对象的精确 key 上传策略，并将压缩后的原图直传 OSS。
2. AI 使用同一临时对象检查照片。开启自动换背景时，AI 提示词不会校验尚未生成的背景色。
3. AI 通过后，人体分割继续使用同一临时对象；小程序下载透明图并在本地 Canvas 合成、压缩。
4. 换底结果使用最终对象策略直传 OSS；未换底时由 OSS 服务端复制临时对象。
5. `POST /api/v1/photos/finalize` 校验最终对象实际大小，并签发绑定最终 key 的提交凭证。

OSS 需要允许小程序域名执行表单上传和下载，并为临时前缀配置短生命周期清理。普通照片上传和下载的图片二进制不经过 Go 服务。

批量导出由阿里云函数计算处理。Go 只写入一个很小的 `export-jobs/` manifest 到 OSS，OSS 的 ObjectCreated 事件触发函数计算；函数计算从 OSS 流式读取照片并将 ZIP 写入 `exports/`。Go 和小程序不会下载或压缩照片。

任务详情页打开或用户主动刷新下载链接时，后端只对当前 `export_key` 做一次 HEAD 检查，并读取很小的状态文件。不会启动后台协程，也不会定时轮询函数计算。

函数计算代码位于 [aliyun/export-worker](/mnt/d/code/latest/photo/aliyun/export-worker)。上传代码包时，ZIP 根目录必须直接包含 `index.js`、`package.json` 和 `node_modules/`；函数入口为 `index.handler`。OSS 触发器只监听 `export-jobs/` 前缀下的 `.json` 文件。

## 当前缺口

- AI 评估结果尚未完整写回数据库
- 部分文档与配置仍在继续收敛
