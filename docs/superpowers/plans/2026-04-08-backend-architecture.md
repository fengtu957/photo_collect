# 批量证件照采集系统 - 后端架构设计

## 技术栈调整

### 原架构
- 微信云开发（云函数 + 云数据库 + 云存储）
- 成本：每月 ¥19 起

### 新架构
- **后端**: Go + `gorilla/mux`
- **数据库**: MongoDB
- **文件存储**: 七牛云
- **部署**: 连接现有服务直接联调与部署
- **成本**: 仅服务器成本

---

## 项目结构

```
photo/
├── miniprogram/              # 小程序前端
│   ├── pages/
│   ├── components/
│   ├── utils/
│   └── types/
│
├── backend/                  # Go 后端服务
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── data/            # 数据层
│   │   ├── biz/             # 业务逻辑
│   │   └── service/         # 服务层
│   ├── pkg/                 # 公共包
│   └── go.mod
│
└── docs/
```

---

## 后端服务设计

### API 端点

```
POST   /api/v1/tasks                    # 创建任务
GET    /api/v1/tasks                    # 获取任务列表
GET    /api/v1/tasks/:id                # 获取任务详情
DELETE /api/v1/tasks/:id                # 删除任务

POST   /api/v1/submissions              # 提交照片
GET    /api/v1/submissions/:id          # 获取提交详情
PUT    /api/v1/submissions/:id          # 修改提交

GET    /api/v1/tasks/:taskId/submissions # 获取提交列表
GET    /api/v1/upload/token              # 获取七牛上传凭证
GET    /api/v1/export/:taskId           # 导出任务数据

POST   /api/v1/auth/login               # 微信登录
```

### 数据库设计

**tasks 集合**
```json
{
  "_id": "ObjectId",
  "user_id": "string",
  "title": "string",
  "description": "string",
  "photo_spec": {
    "name": "string",
    "width": 295,
    "height": 413
  },
  "start_time": "ISODate",
  "end_time": "ISODate",
  "enabled": true,
  "custom_fields": [],
  "stats": {
    "total_submissions": 0
  },
  "created_at": "ISODate",
  "updated_at": "ISODate"
}
```

**submissions 集合**
```json
{
  "_id": "ObjectId",
  "task_id": "ObjectId",
  "user_id": "string",
  "user_info": {
    "nick_name": "string",
    "avatar_url": "string"
  },
  "custom_data": {},
  "photo": {
    "url": "string",
    "file_size": 0,
    "width": 0,
    "height": 0
  },
  "ai_evaluation": {
    "status": "pending",
    "score": 0,
    "issues": [],
    "suggestions": []
  },
  "created_at": "ISODate"
}
```

**users 集合**
```json
{
  "_id": "ObjectId",
  "openid": "string",
  "session_key": "string",
  "nick_name": "string",
  "avatar_url": "string",
  "created_at": "ISODate"
}
```

---

## 当前联调方式

- MongoDB 使用现有服务
- 小程序通过固定测试地址请求后端
- 图片上传由后端签发七牛上传凭证，小程序直传七牛

## 微信小程序配置

### 服务器域名配置

在微信公众平台 → 开发 → 开发管理 → 服务器域名：

- **request 合法域名**: `https://your-domain.com`
- **uploadFile 合法域名**: `https://your-domain.com`
- **downloadFile 合法域名**: `https://your-domain.com`

### 前端 API 调用

```typescript
// miniprogram/utils/request.ts
const BASE_URL = 'https://your-domain.com/api/v1';

export async function request<T>(
  url: string,
  options: WechatMiniprogram.RequestOption = {}
): Promise<T> {
  const token = wx.getStorageSync('token');

  const res = await wx.request({
    url: BASE_URL + url,
    header: {
      'Authorization': `Bearer ${token}`,
      ...options.header
    },
    ...options
  });

  if (res.statusCode !== 200) {
    throw new Error(res.data.message || '请求失败');
  }

  return res.data;
}
```

---

## 认证流程

### 微信登录

1. 小程序调用 `wx.login()` 获取 code
2. 发送 code 到后端 `/api/v1/auth/login`
3. 后端调用微信 API 换取 openid 和 session_key
4. 后端生成 JWT token 返回
5. 小程序存储 token，后续请求携带

### 后端实现

```go
// internal/service/auth.go
func (s *AuthService) Login(ctx context.Context, code string) (string, error) {
    // 调用微信 API
    resp, err := s.wechat.Code2Session(code)
    if err != nil {
        return "", err
    }

    // 查找或创建用户
    user, err := s.data.FindOrCreateUser(ctx, resp.OpenID)
    if err != nil {
        return "", err
    }

    // 生成 JWT token
    token, err := s.jwt.GenerateToken(user.ID, user.OpenID)
    return token, err
}
```

---
