export function isTaskAIAnalysisEnabled(task: any): boolean {
  if (!task) {
    return true;
  }

  return normalizeTaskFlag(task.ai_analysis_enabled, true);
}

export function canUseAIAnalysisFeature(entitlements: any): boolean {
  return !!(entitlements && entitlements.limits && entitlements.limits.can_use_ai_analysis);
}

export function isTaskBackgroundReplacementEnabled(task: any): boolean {
  return !!(task && normalizeTaskFlag(task.background_replacement_enabled, false));
}

export function canUseBackgroundReplacementFeature(entitlements: any): boolean {
  return !!(entitlements
    && entitlements.limits
    && entitlements.limits.can_use_background_replacement);
}

function normalizeTaskFlag(value: any, defaultValue: boolean): boolean {
  if (typeof value === 'boolean') {
    return value;
  }
  if (typeof value === 'number') {
    return value !== 0;
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    if (normalized === 'false' || normalized === '0' || normalized === 'off' || normalized === 'no' || normalized === '') {
      return false;
    }
    if (normalized === 'true' || normalized === '1' || normalized === 'on' || normalized === 'yes') {
      return true;
    }
  }
  return defaultValue;
}
