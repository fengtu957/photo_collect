import { request, BASE_URL, getAuthHeader, ensureAuthorizedSession } from '../utils/request';
import { Task, CreateTaskParams } from '../types/task';

export interface ExportTaskParams {
  filename_template: string;
}

export interface ExportTaskResult {
  id: string;
  status: string;
  file_name: string;
  filename_template?: string;
  download_url: string;
  expires_at: string;
  available_until?: string;
  count: number;
  error_message?: string;
  exported_at?: string;
  created_at?: string;
  updated_at?: string;
}

export interface ExportHistoryResult {
  list: ExportTaskResult[];
  total: number;
  page: number;
  has_more: boolean;
}

export interface TaskInvitation {
  task_id: string;
  task_title: string;
  role: 'admin' | 'collaborator';
  role_text: string;
  token?: string;
  status: 'valid' | 'used' | 'expired' | 'accepted';
  message: string;
  valid: boolean;
  accepted?: boolean;
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

export async function getTaskByCode(taskCode: string) {
  return request<Task>(`/tasks/code/${taskCode}`, { method: 'GET' });
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

export async function createTaskInvitation(id: string, role: 'admin' | 'collaborator') {
  return request<TaskInvitation>(`/tasks/${id}/invitations`, {
    method: 'POST',
    data: { role }
  });
}

export async function getTaskInvitation(token: string) {
  return request<TaskInvitation>(`/task-invitations/${encodeURIComponent(token)}`, {
    method: 'GET'
  });
}

export async function acceptTaskInvitation(token: string) {
  return request<TaskInvitation>(`/task-invitations/${encodeURIComponent(token)}/accept`, {
    method: 'POST'
  });
}

export async function listTaskExports(id: string, page: number = 1, limit: number = 20) {
  return request<ExportHistoryResult>(`/tasks/${id}/exports?page=${page}&limit=${limit}`, {
    method: 'GET'
  });
}

export async function authorizeTaskExportLink(id: string, exportId: string) {
  return request<ExportTaskResult>(`/tasks/${id}/exports/${exportId}/authorize`, {
    method: 'POST'
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

function readDownloadedErrorMessage(filePath: string): Promise<string> {
  if (!filePath) {
    return Promise.resolve('');
  }

  return new Promise((resolve) => {
    wx.getFileSystemManager().readFile({
      filePath,
      encoding: 'utf8',
      success: (res) => {
        try {
          const parsed = JSON.parse(String(res.data || ''));
          if (parsed && parsed.message) {
            resolve(String(parsed.message));
            return;
          }
        } catch (err) {}

        resolve('');
      },
      fail: () => resolve('')
    });
  });
}

async function downloadTaskMiniProgramCodeOnce(id: string): Promise<string> {
  await ensureAuthorizedSession();

  return new Promise((resolve, reject) => {
    wx.downloadFile({
      url: `${BASE_URL}/tasks/${id}/mini-code`,
      header: getAuthHeader(),
      success: async (res) => {
        if (res.statusCode === 200 && res.tempFilePath) {
          resolve(res.tempFilePath);
          return;
        }

        const message = await readDownloadedErrorMessage(res.tempFilePath || '');
        if (res.statusCode === 401 || res.statusCode === 403 || message === 'unauthorized') {
          reject(new Error('unauthorized'));
          return;
        }

        reject(new Error(message || '涓嬭浇灏忕▼搴忕爜澶辫触'));
      },
      fail: (err) => {
        reject(new Error((err && err.errMsg) || '涓嬭浇灏忕▼搴忕爜澶辫触'));
      }
    });
  });
}

export async function downloadTaskMiniProgramCode(id: string): Promise<string> {
  try {
    return await downloadTaskMiniProgramCodeOnce(id);
  } catch (err: any) {
    if (!err || err.message !== 'unauthorized') {
      throw err;
    }

    await ensureAuthorizedSession(true);
    return downloadTaskMiniProgramCodeOnce(id);
  }
}
