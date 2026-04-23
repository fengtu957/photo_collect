const DEFAULT_MAX_ACTIVE_TASKS = 3;
const DEFAULT_MAX_SUBMISSIONS_PER_TASK = 50;
const DEFAULT_MAX_OPEN_DURATION_DAYS = 7;
const DEFAULT_EXPORT_AVAILABLE_DAYS = 7;
const EXTENDED_EXPORT_AVAILABLE_DAYS = 30;
const DEFAULT_DOWNLOAD_LINK_HOURS = 2;

function toNumber(value: any): number {
  const parsed = Number(value || 0);
  if (isNaN(parsed) || parsed < 0) {
    return 0;
  }
  return parsed;
}

export function getDisplayMaxActiveTasks(entitlements: any): number {
  const limit = toNumber(entitlements && entitlements.limits && entitlements.limits.max_active_tasks);
  if (limit === 0 && entitlements && entitlements.limits) {
    return 0;
  }
  return limit > 0 ? limit : DEFAULT_MAX_ACTIVE_TASKS;
}

export function getActiveTaskCount(entitlements: any): number {
  return toNumber(entitlements && entitlements.usage && entitlements.usage.active_task_count);
}

export function getDisplayMaxSubmissionsPerTask(entitlements: any): number {
  const limit = toNumber(entitlements && entitlements.limits && entitlements.limits.max_submissions_per_task);
  if (limit === 0 && entitlements && entitlements.limits) {
    return 0;
  }
  return limit > 0 ? limit : DEFAULT_MAX_SUBMISSIONS_PER_TASK;
}

export function getDisplayMaxOpenDurationDays(entitlements: any): number {
  const limit = toNumber(entitlements && entitlements.limits && entitlements.limits.max_open_duration_days);
  if (limit === 0 && entitlements && entitlements.limits) {
    return 0;
  }
  return limit > 0 ? limit : DEFAULT_MAX_OPEN_DURATION_DAYS;
}

function hasExtendedCapability(entitlements: any): boolean {
  const limits = entitlements && entitlements.limits;
  if (!limits) {
    return false;
  }

  return !!(
    limits.can_use_ai_analysis ||
    toNumber(limits.max_active_tasks) === 0 ||
    toNumber(limits.max_submissions_per_task) === 0 ||
    toNumber(limits.max_open_duration_days) > DEFAULT_MAX_OPEN_DURATION_DAYS
  );
}

export function getDisplayExportAvailableDays(entitlements: any): number {
  return hasExtendedCapability(entitlements) ? EXTENDED_EXPORT_AVAILABLE_DAYS : DEFAULT_EXPORT_AVAILABLE_DAYS;
}

export function getDisplayDownloadLinkHours(): number {
  return DEFAULT_DOWNLOAD_LINK_HOURS;
}

export function buildActiveTaskTip(entitlements: any): string {
  const maxActiveTasks = getDisplayMaxActiveTasks(entitlements);
  if (maxActiveTasks === 0) {
    return '任务数不限';
  }
  return `最多${maxActiveTasks}个任务`;
}

export function buildSubmissionLimitTip(entitlements: any): string {
  const limit = getDisplayMaxSubmissionsPerTask(entitlements);
  if (limit === 0) {
    return '收集量不限';
  }
  return `单任务${limit}份`;
}

export function buildOpenDurationTip(entitlements: any): string {
  const limit = getDisplayMaxOpenDurationDays(entitlements);
  if (limit === 0) {
    return '开放期不限';
  }
  return `开放${limit}天`;
}

export function buildOpenDurationDetailTip(entitlements: any): string {
  const limit = getDisplayMaxOpenDurationDays(entitlements);
  if (limit === 0) {
    return '开放期不限；未设置开始时间时按当前时间计算';
  }
  return `开放${limit}天；未设置开始时间时按当前时间计算`;
}

export function buildDownloadLimitTip(entitlements: any): string {
  return `导出后可在 ${getDisplayExportAvailableDays(entitlements)} 天内重新生成下载链接，单次有效 ${getDisplayDownloadLinkHours()} 小时`;
}

export function buildRetentionTip(entitlements: any): string {
  return `保留${getDisplayExportAvailableDays(entitlements)}天`;
}

export function buildTaskSubmissionLimitText(task: any, entitlements: any): string {
  const taskLimit = toNumber(task && task.max_submissions);
  if (taskLimit > 0) {
    return `${taskLimit}人`;
  }
  if (task && task.max_submissions === 0) {
    return '不限制';
  }

  const displayLimit = getDisplayMaxSubmissionsPerTask(entitlements);
  if (displayLimit === 0) {
    return '不限制';
  }

  const currentCount = toNumber(task && task.stats && task.stats.total_submissions);
  if (currentCount > displayLimit) {
    return `${currentCount}人以上`;
  }

  return `${displayLimit}人`;
}
