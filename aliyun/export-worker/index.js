'use strict';

const path = require('node:path');
const { PassThrough } = require('node:stream');
const OSS = require('ali-oss');
const archiver = require('archiver');

const MAX_MANIFEST_BYTES = 2 * 1024 * 1024;
const PART_SIZE = 8 * 1024 * 1024;

function text(value) {
  return String(value === undefined || value === null ? '' : value).trim();
}

function parseJSON(value) {
  if (Buffer.isBuffer(value)) {
    value = value.toString('utf8');
  }
  if (typeof value === 'string') {
    return JSON.parse(value);
  }
  if (value && typeof value === 'object') {
    return value;
  }
  throw new Error('event is empty');
}

function decodeObjectKey(value) {
  return decodeURIComponent(text(value).replace(/\+/g, ' '));
}

function getEventObjectKey(event) {
  const payload = parseJSON(event);
  const record = Array.isArray(payload.events) ? payload.events[0] : payload;
  const object = record && record.oss && record.oss.object;
  const key = object && (object.key || object.name);
  if (!key) {
    throw new Error('OSS event does not contain object key');
  }
  return decodeObjectKey(key);
}

function normalizePrefix(value, fallback) {
  return text(value).replace(/^\/+|\/+$/g, '') || fallback;
}

function isSafePathSegment(value) {
  return /^[A-Za-z0-9_-]+$/.test(value);
}

function makeOSSClient(context) {
  const credentials = (context && context.credentials) || {};
  const endpoint = text(process.env.ALIYUN_OSS_ENDPOINT);
  const endpointHost = endpoint.replace(/^https?:\/\//, '').split('/')[0];
  const endpointParts = endpointHost.split('.');
  const endpointRegion = endpointParts[0] === text(process.env.ALIYUN_OSS_BUCKET)
    ? endpointParts[1]
    : endpointParts[0];
  const region = text(process.env.ALIYUN_OSS_REGION)
    || endpointRegion
    || 'oss-cn-shanghai';
  const options = {
    region,
    bucket: text(process.env.ALIYUN_OSS_BUCKET),
    accessKeyId: credentials.accessKeyId || process.env.ALIYUN_ACCESS_KEY_ID,
    accessKeySecret: credentials.accessKeySecret || process.env.ALIYUN_ACCESS_KEY_SECRET,
    stsToken: credentials.securityToken || credentials.stsToken || process.env.ALIYUN_OSS_SECURITY_TOKEN,
    secure: true
  };
  if (endpoint) {
    options.endpoint = endpoint;
  }
  if (!options.bucket) {
    throw new Error('ALIYUN_OSS_BUCKET is required');
  }
  if (!options.accessKeyId || !options.accessKeySecret) {
    throw new Error('Function Compute RAM credentials are unavailable');
  }
  return new OSS(options);
}

function getPrefixes() {
  return {
    exportPrefix: normalizePrefix(process.env.ALIYUN_OSS_EXPORT_PREFIX, 'exports'),
    jobPrefix: normalizePrefix(process.env.ALIYUN_OSS_EXPORT_JOB_PREFIX, 'export-jobs'),
    photoPrefix: normalizePrefix(process.env.ALIYUN_OSS_PHOTO_PREFIX, 'photos')
  };
}

function validateManifest(manifest, manifestKey) {
  if (!manifest || manifest.version !== 1) {
    throw new Error('unsupported manifest version');
  }
  const taskId = text(manifest.task_id);
  const exportId = text(manifest.export_id);
  const exportKey = text(manifest.export_key);
  const statusKey = text(manifest.status_key);
  const callbackUrl = text(manifest.callback_url);
  const callbackToken = text(manifest.callback_token);
  if (!taskId || !exportId || !isSafePathSegment(taskId) || !isSafePathSegment(exportId)
    || !exportKey || !statusKey || !Array.isArray(manifest.entries) || manifest.entries.length === 0) {
    throw new Error('manifest is incomplete');
  }

  const prefixes = getPrefixes();
  const expectedManifestKey = `${prefixes.jobPrefix}/${taskId}/${exportId}.json`;
  const expectedExportKey = `${prefixes.exportPrefix}/${taskId}/${exportId}.zip`;
  const expectedStatusKey = `${prefixes.exportPrefix}/${taskId}/${exportId}.status.json`;
  if (manifestKey !== expectedManifestKey || exportKey !== expectedExportKey || statusKey !== expectedStatusKey) {
    throw new Error('manifest paths do not match task and export id');
  }
  if (callbackUrl) {
    const parsedCallbackUrl = new URL(callbackUrl);
    if ((parsedCallbackUrl.protocol !== 'https:' && parsedCallbackUrl.protocol !== 'http:') || !callbackToken) {
      throw new Error('callback configuration is invalid');
    }
  }

  const photoPrefix = `${prefixes.photoPrefix}/${taskId}/`;
  const entries = manifest.entries.map((entry) => {
    const objectKey = text(entry && entry.object_key);
    const fileName = text(entry && entry.file_name);
    if (!objectKey || objectKey !== objectKey.trim() || objectKey.includes('..')
      || path.posix.normalize(objectKey) !== objectKey || !objectKey.startsWith(photoPrefix)) {
      throw new Error(`source object is outside task photo prefix: ${objectKey}`);
    }
    if (!fileName || path.posix.basename(fileName) !== fileName || fileName === '.' || fileName === '..'
      || fileName.includes('\\') || /[\u0000-\u001f]/.test(fileName)) {
      throw new Error(`invalid archive file name: ${fileName}`);
    }
    return { objectKey, fileName };
  });
  return { taskId, exportId, exportKey, statusKey, callbackUrl, callbackToken, entries };
}

async function readManifest(client, key) {
  const result = await client.get(key);
  const content = result && result.content;
  if (!content || content.length > MAX_MANIFEST_BYTES) {
    throw new Error('manifest is empty or too large');
  }
  return JSON.parse(Buffer.from(content).toString('utf8'));
}

async function objectExists(client, key) {
  try {
    await client.head(key);
    return true;
  } catch (error) {
    if (error && (Number(error.status) === 404 || Number(error.statusCode) === 404)) {
      return false;
    }
    throw error;
  }
}

async function writeStatus(client, key, status, message) {
  const updatedAt = new Date().toISOString();
  const body = JSON.stringify({
    status,
    error_message: status === 'failed' ? text(message) : '',
    updated_at: updatedAt
  });
  await client.put(key, Buffer.from(body, 'utf8'), {
    headers: { 'Content-Type': 'application/json; charset=utf-8' }
  });
  return updatedAt;
}

async function notifyCallback(job, status, message, updatedAt) {
  if (!job || !job.callbackUrl || !job.callbackToken) {
    return;
  }
  let lastError = null;
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000);
    try {
      const response = await fetch(job.callbackUrl, {
        method: 'POST',
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
          'X-Export-Callback-Token': job.callbackToken
        },
        body: JSON.stringify({
          export_id: job.exportId,
          status,
          error_message: status === 'failed' ? text(message) : '',
          updated_at: updatedAt
        })
      });
      if (response.ok) {
        return;
      }
      lastError = new Error(`callback returned HTTP ${response.status}`);
    } catch (error) {
      lastError = error;
    } finally {
      clearTimeout(timeout);
    }
    if (attempt < 2) {
      await new Promise((resolve) => setTimeout(resolve, 300 * (attempt + 1)));
    }
  }
  console.error('failed to notify export callback', lastError && lastError.message ? lastError.message : lastError);
}

async function uploadArchiveStream(client, key, archive) {
  let pending = Buffer.alloc(0);
  let uploadId = '';
  let partNumber = 1;
  const parts = [];

  async function uploadPartBuffer(buffer) {
    const result = await client.uploadPart(
      key,
      uploadId,
      partNumber,
      buffer,
      0,
      buffer.length,
      { mime: 'application/zip' }
    );
    parts.push({ number: partNumber, etag: result.etag });
    partNumber += 1;
  }

  try {
    for await (const chunk of archive) {
      if (!chunk || chunk.length === 0) {
        continue;
      }
      const nextChunk = Buffer.from(chunk);
      pending = pending.length === 0 ? nextChunk : Buffer.concat([pending, nextChunk]);

      if (!uploadId && pending.length >= PART_SIZE) {
        const init = await client.initMultipartUpload(key, { mime: 'application/zip' });
        uploadId = init.uploadId;
      }

      while (uploadId && pending.length >= PART_SIZE) {
        const part = pending.subarray(0, PART_SIZE);
        pending = pending.subarray(PART_SIZE);
        await uploadPartBuffer(part);
      }
    }

    if (!uploadId) {
      if (pending.length === 0) {
        throw new Error('archive is empty');
      }
      await client.put(key, pending, {
        mime: 'application/zip',
        headers: { 'Content-Type': 'application/zip' }
      });
      return;
    }

    if (pending.length > 0) {
      await uploadPartBuffer(pending);
    }
    if (parts.length === 0) {
      throw new Error('archive did not produce any upload parts');
    }
    await client.completeMultipartUpload(key, uploadId, parts);
  } catch (error) {
    if (uploadId) {
      try {
        await client.abortMultipartUpload(key, uploadId);
      } catch (abortError) {
        console.error('failed to abort multipart upload', abortError);
      }
    }
    throw error;
  }
}

async function appendRemoteObject(client, archive, entry) {
  const relay = new PassThrough();
  const consumed = new Promise((resolve, reject) => {
    relay.once('end', resolve);
    relay.once('error', reject);
  });
  archive.append(relay, { name: entry.fileName });

  try {
    const source = await client.getStream(entry.objectKey);
    if (!source || !source.stream) {
      throw new Error(`source object stream unavailable: ${entry.objectKey}`);
    }
    source.stream.once('error', (error) => relay.destroy(error));
    source.stream.pipe(relay);
  } catch (error) {
    relay.destroy(error);
  }

  await consumed;
}

async function createArchive(client, job) {
  if (await objectExists(client, job.exportKey)) {
    return;
  }

  const archive = archiver('zip', { zlib: { level: 6 } });
  archive.on('warning', (warning) => {
    if (!warning || warning.code !== 'ENOENT') {
      console.warn('export archive warning', warning);
    }
  });
  let archiveError = null;
  archive.on('error', (error) => {
    archiveError = error;
  });
  const uploadPromise = uploadArchiveStream(client, job.exportKey, archive);

  try {
    for (const entry of job.entries) {
      await appendRemoteObject(client, archive, entry);
    }
    await archive.finalize();
    await uploadPromise;
    if (archiveError) {
      throw archiveError;
    }
  } catch (error) {
    archive.abort();
    try {
      await uploadPromise;
    } catch (uploadError) {
      // Preserve the original source or archive error for the status file.
    }
    throw error;
  }
}

async function handler(event, context) {
  const manifestKey = getEventObjectKey(event);
  const prefixes = getPrefixes();
  if (!manifestKey.startsWith(`${prefixes.jobPrefix}/`) || !manifestKey.endsWith('.json')) {
    return { status: 'ignored', reason: 'event is outside export job prefix' };
  }

  const client = makeOSSClient(context);
  let job;
  try {
    const manifest = await readManifest(client, manifestKey);
    job = validateManifest(manifest, manifestKey);
    const processingAt = await writeStatus(client, job.statusKey, 'processing', '');
    await notifyCallback(job, 'processing', '', processingAt);
    await createArchive(client, job);
    const successAt = await writeStatus(client, job.statusKey, 'success', '');
    await notifyCallback(job, 'success', '', successAt);
    return { status: 'success', export_key: job.exportKey };
  } catch (error) {
    const message = error && error.message ? error.message : String(error);
    if (job && job.statusKey) {
      try {
        const failedAt = await writeStatus(client, job.statusKey, 'failed', message);
        await notifyCallback(job, 'failed', message, failedAt);
      } catch (statusError) {
        console.error('failed to write export status', statusError);
      }
    }
    console.error('export worker failed', message);
    throw error;
  }
}

exports.handler = handler;
