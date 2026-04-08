# 批量证件照采集系统 - 后端服务

## 快速开始

### 1. 启动 MongoDB 和 MinIO

```bash
docker-compose up -d mongodb minio
```

### 2. 配置环境变量

复制 `.env.example` 为 `.env` 并填写配置：

```bash
cd backend
cp .env.example .env
```

### 3. 启动后端服务

```bash
cd backend
go run cmd/server/main.go
```

服务将在 `http://localhost:8000` 启动

## API 测试

### 创建任务

```bash
curl -X POST http://localhost:8000/api/v1/tasks \
  -H "Content-Type: application/json" \
  -H "X-User-ID: test-user" \
  -d '{
    "title": "测试任务",
    "description": "这是一个测试任务",
    "photo_spec": {
      "name": "一寸照",
      "width": 295,
      "height": 413
    },
    "start_time": "2026-04-08T00:00:00Z",
    "end_time": "2026-04-30T23:59:59Z",
    "custom_fields": []
  }'
```

### 查询任务列表

```bash
curl http://localhost:8000/api/v1/tasks \
  -H "X-User-ID: test-user"
```

### 查询任务详情

```bash
curl http://localhost:8000/api/v1/tasks/{task_id}
```

## 下一步

- [ ] 实现文件上传接口
- [ ] 实现提交照片接口
- [ ] 集成通义千问 AI 评估
- [ ] 实现微信登录
- [ ] 前端适配
