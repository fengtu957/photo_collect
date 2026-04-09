export interface PhotoSpec {
  name: string;
  width: number;
  height: number;
  dpi?: number;
}

export interface CustomField {
  id: string;
  type: 'text' | 'number' | 'select' | 'multiselect';
  label: string;
  required: boolean;
  options?: string[];
  placeholder?: string;
}

export interface TaskStats {
  total_submissions: number;
  last_submit_time?: string;
}

export interface Task {
  id: string;
  user_id: string;
  title: string;
  description: string;
  photo_spec: PhotoSpec;
  start_time: string;
  end_time: string;
  enabled: boolean;
  custom_fields: CustomField[];
  stats: TaskStats;
  created_at: string;
  updated_at: string;
}

export interface CreateTaskParams {
  title: string;
  description: string;
  photo_spec: PhotoSpec;
  start_time: string;
  end_time: string;
  custom_fields: CustomField[];
}
