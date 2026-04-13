export interface PhotoSpec {
  name: string;
  width: number;
  height: number;
  dpi?: number;
  max_size_kb?: number;
  background_color?: string;
}

export interface CustomField {
  id: string;
  type: 'text' | 'number' | 'select' | 'multiselect';
  label: string;
  required: boolean;
  unique?: boolean;
  options?: string[];
  placeholder?: string;
}

export interface TaskStats {
  total_submissions: number;
  last_submit_time?: string;
}

export interface TaskExportInfo {
  status?: string;
  persistent_id?: string;
  filename_template?: string;
  export_key?: string;
  file_name?: string;
  count?: number;
  exported_at?: string;
  error_message?: string;
}

export interface Task {
  id: string;
  user_id: string;
  title: string;
  description: string;
  photo_spec: PhotoSpec;
  ai_analysis_enabled?: boolean;
  max_submissions?: number;
  start_time: string;
  end_time: string;
  enabled: boolean;
  custom_fields: CustomField[];
  stats: TaskStats;
  export_info?: TaskExportInfo;
  created_at: string;
  updated_at: string;
}

export interface CreateTaskParams {
  title: string;
  description: string;
  photo_spec: PhotoSpec;
  ai_analysis_enabled?: boolean;
  start_time: string;
  end_time: string;
  custom_fields: CustomField[];
}
