export interface UserEntitlementLimits {
  max_active_tasks: number;
  max_submissions_per_task: number;
  max_open_duration_days: number;
  can_use_ai_analysis: boolean;
}

export interface UserEntitlementUsage {
  active_task_count: number;
}

export interface UserEntitlements {
  limits: UserEntitlementLimits;
  usage: UserEntitlementUsage;
  [key: string]: any;
}
