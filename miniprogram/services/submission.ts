import { request } from '../utils/request';
import { Submission, SubmitPhotoParams } from '../types/submission';

export async function submitPhoto(params: SubmitPhotoParams) {
  return request<{ id: string }>('/submissions', {
    method: 'POST',
    data: params
  });
}

export async function getSubmissions(taskId: string) {
  return request<Submission[]>(`/tasks/${taskId}/submissions`, { method: 'GET' });
}

export async function getUploadToken() {
  return request<{ token: string }>('/upload/token', { method: 'GET' });
}
