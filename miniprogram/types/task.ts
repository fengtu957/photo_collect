export interface PhotoSpec {
  name: string;
  width: number;
  height: number;
  dpi?: number;
}

export interface CustomField {
  id: string;
  type: 'text' | 'select';
  label: string;
  required: boolean;
  options?: string[];
  placeholder?: string;
}

export interface StorageConfig {
  retentionDays: number;
  expirationDate: Date;
  autoDeleteAfterExport: boolean;
}

export interface ExportConfig {
  nameTemplate: string;
  includeOriginal: boolean;
}

export interface TaskStats {
  totalSubmissions: number;
  lastSubmitTime?: Date;
}

export interface Task {
  _id: string;
  _openid: string;
  title: string;
  description: string;
  photoSpec: PhotoSpec;
  startTime: Date;
  endTime: Date;
  enabled: boolean;
  customFields: CustomField[];
  stats: TaskStats;
  storageConfig: StorageConfig;
  exportConfig: ExportConfig;
  createdAt: Date;
  updatedAt: Date;
}

export interface CreateTaskParams {
  title: string;
  description: string;
  photoSpec: PhotoSpec;
  startTime: Date;
  endTime: Date;
  customFields: CustomField[];
  storageConfig?: Partial<StorageConfig>;
}
