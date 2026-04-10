import { request } from '../utils/request';
import { Task, CreateTaskParams } from '../types/task';

export interface ExportTaskParams {
  filename_template: string;
}

export interface ExportTaskResult {
  status: string;
  file_name: string;
  download_url: string;
  expires_at: string;
  count: number;
  error_message?: string;
}

export async function createTask(params: CreateTaskParams) {
  return request<{ id: string }>('/tasks', {
    method: 'POST',
    data: params
  });
}

export async function listTasks() {
  return request<Task[]>('/tasks', { method: 'GET' });
}

export async function getTask(id: string) {
  return request<Task>(`/tasks/${id}`, { method: 'GET' });
}

export async function updateTask(id: string, params: CreateTaskParams) {
  return request<{ id: string }>(`/tasks/${id}`, {
    method: 'PUT',
    data: params
  });
}

export async function deleteTask(id: string) {
  return request<null>(`/tasks/${id}`, { method: 'DELETE' });
}

export async function exportTask(id: string, params: ExportTaskParams) {
  return request<ExportTaskResult>(`/tasks/${id}/export`, {
    method: 'POST',
    data: params
  });
}

export async function authorizeExportLink(id: string) {
  return request<ExportTaskResult>(`/tasks/${id}/export/authorize`, {
    method: 'POST'
  });
}

export async function syncExportStatus(id: string) {
  return request<ExportTaskResult>(`/tasks/${id}/export/status`, {
    method: 'POST'
  });
}
