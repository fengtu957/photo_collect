import { request } from '../utils/request';
import { AnalyzePhotoPreviewParams, Submission, SubmissionAnalysisResult, SubmissionListResponse, SubmitPhotoParams } from '../types/submission';

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

export async function deleteSubmission(id: string) {
  return request<{ id: string }>(`/submissions/${id}`, {
    method: 'DELETE'
  });
}

export async function authorizeSubmissionPhotoLink(id: string) {
  return request<{ download_url: string; expires_at: string }>(`/submissions/${id}/photo/authorize`, {
    method: 'POST'
  });
}

export async function getRejectionNotificationConfig() {
  return request<{ enabled: boolean; template_id: string }>('/notifications/rejection-config', {
    method: 'GET'
  });
}

export async function notifySubmissionRejected(id: string, reviewStatus: string, prompt: string) {
  return request<{ id: string }>(`/submissions/${id}/rejection-notification`, {
    method: 'POST',
    data: {
      review_status: reviewStatus,
      prompt
    }
  });
}

export async function getSubmission(id: string) {
  return request<Submission>(`/submissions/${id}`, { method: 'GET' });
}

export async function analyzePhotoPreview(params: AnalyzePhotoPreviewParams) {
  return request<SubmissionAnalysisResult>('/submissions/analyze-preview', {
    method: 'POST',
    data: params
  });
}

export async function submitPhoto(params: SubmitPhotoParams) {
  return request<{ id: string }>('/submissions', {
    method: 'POST',
    data: params
  });
}

export async function listSubmissions(taskId: string, page: number = 1, limit: number = 20, scope: 'mine' | 'all' = 'mine') {
  return request<SubmissionListResponse>(
    `/tasks/${taskId}/submissions?page=${page}&limit=${limit}&scope=${scope}`,
    { method: 'GET' }
  );
}

export async function segmentPhoto(taskId: string, photoKey: string) {
  return request<{ result_url: string; expires_in: number }>('/photos/segment', {
    method: 'POST',
    data: { task_id: taskId, oss_key: photoKey }
  });
}
