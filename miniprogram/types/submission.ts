export interface PhotoInfo {
  url: string;
  file_size: number;
  width: number;
  height: number;
  deleted: boolean;
}

export interface AIEvaluationBreakdown {
  clarity?: number;
  lighting?: number;
  angle?: number;
  background?: number;
  expression?: number;
  composition?: number;
}

export interface AIEvaluation {
  status: '' | 'pending' | 'success' | 'failed';
  score: number;
  issues: string[];
  suggestions: string[];
  breakdown: AIEvaluationBreakdown;
  evaluated_at?: string;
  error?: string;
}

export interface UserInfo {
  nick_name: string;
  avatar_url: string;
}

export interface Submission {
  id: string;
  task_id: string;
  user_id: string;
  user_info: UserInfo;
  custom_data: Record<string, string | string[]>;
  photo: PhotoInfo;
  ai_evaluation: AIEvaluation;
  status: 'draft' | 'submitted' | 'rejected';
  created_at: string;
  updated_at: string;
}

export interface SubmissionListResponse {
  list: Submission[];
  total: number;
  has_more: boolean;
}

export interface SubmitPhotoParams {
  task_id: string;
  photo: {
    url: string;
    file_size?: number;
    width?: number;
    height?: number;
  };
  custom_data?: Record<string, any>;
}

export interface SubmissionAnalysisResult {
  model: string;
  score: number;
  breakdown: AIEvaluationBreakdown;
  issues: string[];
  suggestions: string[];
}
