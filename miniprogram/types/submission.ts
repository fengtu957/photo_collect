export interface PhotoInfo {
  originalUrl: string;
  originalFileId: string;
  fileSize: number;
  width: number;
  height: number;
  expiresAt: Date;
  deleted: boolean;
  deletedAt?: Date;
  deletedReason?: string;
}

export interface AIEvaluationBreakdown {
  clarity: number;
  lighting: number;
  angle: number;
  background: number;
  expression: number;
  composition: number;
}

export interface AIEvaluation {
  status: 'pending' | 'success' | 'failed';
  score: number;
  issues: string[];
  suggestions: string[];
  breakdown: AIEvaluationBreakdown;
  evaluatedAt?: Date;
  error?: string;
}

export interface UserInfo {
  nickName: string;
  avatarUrl: string;
}

export interface Submission {
  _id: string;
  _openid: string;
  taskId: string;
  userInfo: UserInfo;
  customData: Record<string, string | string[]>;
  photo: PhotoInfo;
  aiEvaluation: AIEvaluation;
  status: 'draft' | 'submitted' | 'rejected';
  createdAt: Date;
  updatedAt: Date;
}

export interface SubmitPhotoParams {
  taskId: string;
  customData: Record<string, string | string[]>;
  photo: {
    fileId: string;
    width: number;
    height: number;
    fileSize: number;
  };
}
