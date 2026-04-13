export function isTaskAIAnalysisEnabled(task: any): boolean {
  if (!task) {
    return true;
  }

  return task.ai_analysis_enabled !== false;
}
