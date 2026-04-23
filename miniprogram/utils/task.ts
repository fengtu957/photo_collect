export function isTaskAIAnalysisEnabled(task: any): boolean {
  if (!task) {
    return true;
  }

  return task.ai_analysis_enabled !== false;
}

export function canUseAIAnalysisFeature(entitlements: any): boolean {
  return !!(entitlements && entitlements.limits && entitlements.limits.can_use_ai_analysis);
}
