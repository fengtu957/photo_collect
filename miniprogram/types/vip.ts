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
  is_vip: boolean;
  plan_code?: string;
  expire_at?: string;
  contact_label?: string;
  contact_value?: string;
  limits: UserEntitlementLimits;
  usage: UserEntitlementUsage;
}
