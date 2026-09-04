import { request } from '../utils/request';

export interface OSSUploadPolicy {
  upload_url: string;
  key: string;
  fields: Record<string, string>;
  expires_in: number;
}

export interface FinalizePhotoResult {
  photo_key: string;
  file_size: number;
  verification_token?: string;
}

export function getUploadPolicy(
  taskId: string,
  purpose: 'temporary' | 'final',
  sourceKey: string = '',
  verificationToken: string = ''
) {
  return request<OSSUploadPolicy>('/upload/policy', {
    method: 'POST',
    data: {
      task_id: taskId,
      purpose,
      source_key: sourceKey || undefined,
      verification_token: verificationToken || undefined
    }
  });
}

export function finalizePhoto(
  taskId: string,
  sourceKey: string,
  finalKey: string,
  verificationToken: string
) {
  return request<FinalizePhotoResult>('/photos/finalize', {
    method: 'POST',
    data: {
      task_id: taskId,
      source_key: sourceKey,
      final_key: finalKey || undefined,
      verification_token: verificationToken || undefined
    }
  });
}
