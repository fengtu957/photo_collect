import { request } from '../utils/request';
import { Task, CreateTaskParams } from '../types/task';

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

export async function deleteTask(id: string) {
  return request<null>(`/tasks/${id}`, { method: 'DELETE' });
}
