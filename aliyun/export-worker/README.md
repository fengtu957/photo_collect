# OSS 批量导出函数

该函数是阿里云函数计算（FC）事件函数。后端把导出任务清单写入
`export-jobs/<task_id>/<export_id>.json`，OSS 创建对象事件触发本函数；
函数从 `photos/` 流式读取照片，生成 ZIP 并写入 `exports/`，随后回调后端更新导出历史状态。

## 一、制作部署 ZIP

在本目录安装生产依赖：

```bash
cd aliyun/export-worker
npm install --omit=dev
```

Windows PowerShell 打包：

```powershell
Compress-Archive -Path index.js,package.json,node_modules -DestinationPath photo-export-worker.zip -Force
```

Linux/macOS 打包：

```bash
zip -r photo-export-worker.zip index.js package.json node_modules
```

生成文件位于：

```text
aliyun/export-worker/photo-export-worker.zip
```

ZIP 根目录必须直接包含以下内容，不能再套一层 `export-worker` 目录：

```text
index.js
package.json
node_modules/
```

## 二、创建函数计算函数

在阿里云函数计算控制台进入“函数管理 → 函数列表”，选择与 OSS Bucket 相同的地域，创建事件函数：

- 运行时：Node.js 18 或更高版本
- 请求处理程序：`index.handler`
- 架构：`x86_64`
- 代码上传方式：通过 ZIP 包上传代码
- 代码包：上一步生成的 `photo-export-worker.zip`
- 执行超时时间：建议 `600` 秒，根据最大导出照片量调整
- 内存：建议至少 `512 MB`
- 实例角色：选择允许该函数访问目标 OSS Bucket 的 RAM 角色

不要在函数环境变量中配置长期 AccessKey。代码会优先使用函数计算实例角色提供的临时凭证。

## 三、配置 FC 环境变量

进入目标函数详情页的“配置 → 高级配置 → 环境变量”，添加以下变量并部署。

必须配置：

```text
ALIYUN_OSS_BUCKET=实际 Bucket 名称
EXPORT_CALLBACK_URL=https://photo-collect-qa.starpix.cn/api/v1/export-callback
```

建议配置：

```text
ALIYUN_OSS_REGION=oss-cn-shanghai
ALIYUN_OSS_ENDPOINT=oss-cn-shanghai.aliyuncs.com
```

可选配置；不填写时使用右侧默认值：

```text
ALIYUN_OSS_PHOTO_PREFIX=photos
ALIYUN_OSS_EXPORT_PREFIX=exports
ALIYUN_OSS_EXPORT_JOB_PREFIX=export-jobs
```

三个前缀必须与 Go 后端中的同名配置保持一致。

回调地址配置在 FC；Go 后端只把每次导出的独立回调令牌写入私有 manifest，无需配置回调地址或共享回调密钥。

## 四、配置 OSS 触发器

在函数详情页进入“触发器”，创建“对象存储 OSS”触发器：

- Bucket：与 `ALIYUN_OSS_BUCKET` 相同
- 事件：`oss:ObjectCreated:PutObject`
- 文件前缀：`export-jobs/`
- 文件后缀：`.json`
- 触发函数版本：当前部署版本或 `LATEST`

不要把触发前缀设置为 `exports/` 或空值，否则函数写入 ZIP 和状态文件时可能再次触发自身。

## 五、RAM 权限

函数实例角色至少需要：

- 对 `photos/*`、`export-jobs/*`：`oss:GetObject`、`oss:GetObjectMeta`
- 对 `exports/*`：`oss:PutObject`
- 对 `exports/*` 的分片上传：`oss:InitiateMultipartUpload`、`oss:UploadPart`、
  `oss:CompleteMultipartUpload`、`oss:ListParts`、`oss:AbortMultipartUpload`

首次联调可以临时使用 `AliyunOSSFullAccess` 排查权限问题，生产环境应改为限定 Bucket 和前缀的自定义权限。

## 六、以后更新函数代码

1. 修改 `index.js`；如果依赖发生变化，同时修改 `package.json` 并重新执行 `npm install --omit=dev`。
2. 重新生成 `photo-export-worker.zip`。
3. 在函数详情页的代码页签选择“上传 ZIP 包”，上传新包并部署。
4. 确认请求处理程序仍为 `index.handler`，环境变量和 OSS 触发器仍绑定当前部署版本。
5. 从小程序创建一次导出，在导出历史中确认状态依次变为“排队中/处理中/已完成”。

函数会向 manifest 中的回调地址发送 `processing`、`success` 或 `failed` 状态。回调暂时失败时会重试；后端仍会通过 OSS 状态文件做兜底同步，超过 15 分钟仍无结果时由后台恢复服务标记为失败。
