# 微信小程序开发规则

## JavaScript 兼容性

### 禁止使用的语法
- ❌ **可选链操作符 `?.`** - 微信小程序不支持
  ```typescript
  // 错误
  const value = obj?.property;

  // 正确
  const value = (obj && obj.property) || defaultValue;
  ```

### 变量命名规则
- ❌ 避免在文件顶层声明 `app` 或 `appInstance` 等全局变量
- ✅ 在函数内部声明 `const appInstance = getApp<any>()`
  ```typescript
  // 错误 - 会导致多文件冲突
  const appInstance = getApp<any>();
  Page({ ... });

  // 正确 - 在方法内部声明
  Page({
    onLoad() {
      const appInstance = getApp<any>();
      // 使用 appInstance
    }
  });
  ```

## 输入框样式规则

### Input 组件样式
```wxss
.input {
  display: block;
  width: 100%;
  font-size: 32rpx;
  color: #333;
  border: none;
  border-bottom: 1rpx solid #eee;
  padding: 0;
  margin: 0;
  box-sizing: border-box;
  height: 88rpx;
  line-height: 88rpx;
}

.input::placeholder {
  color: #999;
}
```

### Input 组件属性
```xml
<input
  class="input"
  placeholder="提示文字"
  adjust-position="{{false}}"
/>
```
- ✅ 必须添加 `adjust-position="{{false}}"` 防止聚焦时页面错位
- ✅ 使用固定 `height` 和 `line-height` 确保文字垂直居中
- ✅ 不要使用 `padding` 控制高度，用 `line-height` 代替

### Textarea 组件样式
```wxss
.textarea {
  width: 100%;
  font-size: 32rpx;
  color: #333;
  height: 160rpx;
  border: none;
  border-bottom: 1rpx solid #eee;
  padding: 20rpx 0;
  box-sizing: border-box;
}

.textarea::placeholder {
  color: #999;
}
```

## 卡片和列表样式

### 标准卡片样式
```wxss
.card {
  background: white;
  border-radius: 12rpx;
  padding: 24rpx;
  margin-bottom: 16rpx;
  border: 1rpx solid #eee;
  box-shadow: 0 2rpx 8rpx rgba(0,0,0,0.05);
}
```

### 分隔线
```wxss
.divider {
  border-bottom: 1rpx solid #f5f5f5;
  padding-bottom: 16rpx;
  margin-bottom: 16rpx;
}
```

## TypeScript 类型规则

### API 参数命名
- ✅ 使用下划线命名（snake_case）匹配后端 Go API
  ```typescript
  interface CreateTaskParams {
    title: string;
    photo_spec: PhotoSpec;  // 不是 photoSpec
    start_time: string;      // 不是 startTime
    end_time: string;        // 不是 endTime
    custom_fields: any[];    // 不是 customFields
  }
  ```

### 后端返回字段命名规则
- ✅ 后端返回的所有字段都是 snake_case
- ✅ 主要字段对照表：
  ```typescript
  // Task 对象
  id                  // 不是 _id
  user_id
  title
  description
  photo_spec          // 不是 photoSpec
  start_time          // 不是 startTime
  end_time            // 不是 endTime
  custom_fields       // 不是 customFields
  created_at          // 不是 createdAt
  updated_at          // 不是 updatedAt
  stats.total_submissions  // 不是 stats.totalSubmissions

  // Submission 对象
  id                  // 不是 _id
  task_id
  user_info.nick_name      // 不是 userInfo.nickName
  user_info.avatar_url     // 不是 userInfo.avatarUrl
  custom_data              // 不是 customData
  photo.original_url       // 不是 photo.originalUrl
  ai_evaluation            // 不是 aiEvaluation
  created_at               // 不是 createdAt
  ```

### App 全局数据类型
```typescript
// app.ts
interface IAppGlobalData {
  userInfo?: WechatMiniprogram.UserInfo;
  customFields: any[];
}

interface ICustomAppOption {
  globalData: IAppGlobalData;
  autoLogin(): void;
}

App<ICustomAppOption>({
  globalData: { customFields: [] },
  // ...
});
```

## 时间格式规则

### 前端发送给后端
- ✅ 使用 RFC3339 格式：`YYYY-MM-DDTHH:mm:ss+08:00`
  ```typescript
  function toRFC3339(date: string, time: string) {
    return `${date}T${time || '00:00'}:00+08:00`;
  }
  ```

### 前端显示
- ✅ 格式化为：`YYYY-MM-DD HH:mm`
  ```typescript
  function formatTime(iso: string) {
    if (!iso) return '';
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }
  ```

## 页面布局规则

### 容器样式
```wxss
.page {
  padding: 20rpx;
  background: #f5f5f5;
  min-height: 100vh;
  padding-bottom: 120rpx; /* 为底部按钮留空间 */
}
```

### 固定底部按钮
```wxss
.footer {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  display: flex;
  padding: 20rpx;
  background: white;
  gap: 20rpx;
  box-shadow: 0 -2rpx 8rpx rgba(0,0,0,0.1);
}
```

### 悬浮按钮（FAB）
```wxss
.fab {
  position: fixed;
  bottom: 40rpx;
  left: 50%;
  transform: translateX(-50%);
  background: #1989fa;
  color: white;
  padding: 24rpx 60rpx;
  border-radius: 60rpx;
  font-size: 32rpx;
  box-shadow: 0 8rpx 24rpx rgba(25,137,250,0.4);
}
```

## 响应式设计

### 宽度控制
```wxss
.container {
  width: 100%;
  box-sizing: border-box;
}

.card {
  width: 100%;
  box-sizing: border-box;
}
```
- ✅ 所有容器必须设置 `width: 100%` 和 `box-sizing: border-box`
- ✅ 避免内容超出屏幕边界

### 文字换行
```wxss
.text {
  word-break: break-all;
  overflow-wrap: break-word;
}
```

## 颜色规范

```wxss
/* 主色 */
--primary: #1989fa;
--success: #07c160;
--warning: #ff9500;
--danger: #ff4444;

/* 文字颜色 */
--text-primary: #333;
--text-secondary: #666;
--text-placeholder: #999;
--text-disabled: #ccc;

/* 背景色 */
--bg-page: #f5f5f5;
--bg-card: #ffffff;

/* 边框色 */
--border-light: #f5f5f5;
--border-normal: #eee;
--border-dark: #ddd;
```

## 常见错误和解决方案

### 1. 输入框文字被截断
- **原因**：高度不够或 padding 设置不当
- **解决**：使用固定 `height: 88rpx` + `line-height: 88rpx`

### 2. 聚焦时 placeholder 上移
- **原因**：使用了固定 `height` 但没有禁用自动调整
- **解决**：添加 `adjust-position="{{false}}"`

### 3. 列表不显示数据
- **原因**：使用了 `movable-view` 但没有设置高度
- **解决**：改用普通 `view` 组件

### 4. 变量名冲突
- **原因**：多个文件顶层声明了相同变量名
- **解决**：在函数内部声明变量

### 5. 可选链报错
- **原因**：微信小程序不支持 `?.` 语法
- **解决**：使用 `(obj && obj.property) || defaultValue`

## 开发流程

1. **先读取文件** - 使用 Edit/Write 前必须先 Read
2. **一次性修复** - 不要反复修改同一个问题
3. **测试兼容性** - 避免使用新语法特性
4. **保持一致性** - 遵循已有的代码风格
5. **完整测试** - 修改后在真机/模拟器测试

## 调试技巧

### 控制台日志
```typescript
console.log('数据:', JSON.stringify(data));
```

### 检查编译错误
- 查看微信开发者工具的控制台
- 注意 TypeScript 类型错误
- 检查 WXSS 语法错误

### 真机测试
- 预览前确保没有编译错误
- 注意真机和模拟器的差异
- 测试不同屏幕尺寸
