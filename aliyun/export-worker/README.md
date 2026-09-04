# OSS export worker

This Function Compute event function creates a ZIP package from the OSS
objects listed in an `export-jobs/<task_id>/<export_id>.json` manifest.
Images are streamed from OSS and the ZIP is uploaded to OSS with multipart
upload. The Go API never receives image bytes.

## Function configuration

- Runtime: Node.js managed runtime.
- Handler: `index.handler`.
- Architecture: `x86_64`.
- Execution role: a RAM role trusted by Function Compute with permission to
  read `photos/` and `export-jobs/`, and write `exports/`.
- Trigger: OSS `ObjectCreated` for prefix `export-jobs/` and suffix `.json`.
- Region: the same region as the OSS bucket.

Environment variables:

- `ALIYUN_OSS_BUCKET` (required)
- `ALIYUN_OSS_REGION` (recommended, for example `oss-cn-shanghai`)
- `ALIYUN_OSS_ENDPOINT` (optional, for example `oss-cn-shanghai.aliyuncs.com`)
- `ALIYUN_OSS_PHOTO_PREFIX` (optional, default `photos`)
- `ALIYUN_OSS_EXPORT_PREFIX` (optional, default `exports`)
- `ALIYUN_OSS_EXPORT_JOB_PREFIX` (optional, default `export-jobs`)

Do not put long-lived AccessKey values in the function. Function Compute
provides temporary credentials for the configured RAM execution role.

The execution role needs `oss:GetObject`/`oss:GetObjectMeta` for `photos/*`
and `export-jobs/*`, plus `oss:PutObject` and the multipart actions
`oss:InitiateMultipartUpload`, `oss:UploadPart`,
`oss:CompleteMultipartUpload`, `oss:ListParts`, and
`oss:AbortMultipartUpload` for `exports/*`. `AliyunOSSFullAccess` works for
initial verification but is broader than necessary; replace it with a
prefix-scoped custom policy before production use.

## Package and upload

Run `npm install --omit=dev` in this directory, then upload the directory as
a ZIP package. The ZIP root must contain `index.js`, `package.json`, and the
installed `node_modules` directory.
