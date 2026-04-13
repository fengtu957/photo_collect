export interface UserEntitlementLimits {
  max_active_tasks: number;
  max_submissions_per_task: number;
  can_use_ai_analysis: boolean;
}

export interface UserEntitlementUsage {
  active_task_count: number;
}

export interface UserEntitlements {
  is_vip: boolean;
  plan_code?: string;
  expire_at?: string;
  limits: UserEntitlementLimits;
  usage: UserEntitlementUsage;
}
