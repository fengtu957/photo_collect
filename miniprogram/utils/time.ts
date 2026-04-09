function toDate(value: Date | string): Date {
  return typeof value === 'string' ? new Date(value) : value;
}

export function isEffectiveTime(value: Date | string | undefined | null): boolean {
  if (!value) return false;
  const d = toDate(value);
  if (isNaN(d.getTime())) return false;
  return d.getFullYear() > 1900;
}

export function formatTime(iso: Date | string): string {
  if (!isEffectiveTime(iso)) return '';
  const d = toDate(iso);
  if (isNaN(d.getTime())) return typeof iso === 'string' ? iso : '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function toRFC3339(date: string, time: string): string {
  if (!date) return '';
  return `${date}T${time || '00:00'}:00+08:00`;
}
