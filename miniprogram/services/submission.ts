import { request } from '../utils/request';
import { Submission, SubmissionAnalysisResult, SubmissionListResponse, SubmitPhotoParams } from '../types/submission';

export async function createSubmission(params: SubmitPhotoParams) {
  return request<{ id: string }>('/submissions', {
    method: 'POST',
    data: params
  });
}

export async function updateSubmission(id: string, params: SubmitPhotoParams) {
  return request<{ id: string }>(`/submissions/${id}`, {
    method: 'PUT',
    data: params
  });
}

export async function getSubmission(id: string) {
  return request<Submission>(`/submissions/${id}`, { method: 'GET' });
}

export async function analyzeSubmission(id: string) {
  return request<SubmissionAnalysisResult>(`/submissions/${id}/analyze`, {
    method: 'POST'
  });
}

export async function submitPhoto(params: SubmitPhotoParams) {
  return request<{ id: string }>('/submissions', {
    method: 'POST',
    data: params
  });
}

export async function listSubmissions(taskId: string, page: number = 1, limit: number = 20) {
  return request<SubmissionListResponse>(
    `/tasks/${taskId}/submissions?page=${page}&limit=${limit}`,
    { method: 'GET' }
  );
}

export async function getUploadToken() {
  return request<{ token: string }>('/upload/token', { method: 'GET' });
}
