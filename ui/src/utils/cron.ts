// Cron expression utilities

const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

const KNOWN_CRON: Record<string, string> = {
  '0 * * * *': 'Every hour',
  '0 9 * * *': 'Every day at 9:00 AM',
  '0 9 * * 1-5': 'Weekdays at 9:00 AM',
  '0 0 * * *': 'Every day at midnight',
  '0 10 * * 1': 'Every Monday at 10:00 AM',
  '0 10 1 * *': 'First day of each month at 10:00 AM',
  '*/30 * * * *': 'Every 30 minutes',
  '0 */6 * * *': 'Every 6 hours',
  '0 2 * * *': 'Every day at 2:00 AM',
};

/**
 * Returns a human-readable description of a cron expression, or null if invalid/empty.
 */
export function describeCronExpression(cron: string): string | null {
  if (!cron || !cron.trim()) return null;
  const trimmed = cron.trim();

  // Check known patterns
  if (KNOWN_CRON[trimmed]) return KNOWN_CRON[trimmed];

  const parts = trimmed.split(/\s+/);
  if (parts.length !== 5) return null;

  const [minute, hour, , , dayOfWeek] = parts;
  const segments: string[] = [];

  if (minute.startsWith('*/')) {
    segments.push(`every ${minute.slice(2)} minutes`);
  }

  if (hour.startsWith('*/')) {
    segments.push(`every ${hour.slice(2)} hours`);
  } else if (hour !== '*' && !minute.includes('*') && !minute.includes('/')) {
    const hourNum = parseInt(hour, 10);
    const minuteNum = parseInt(minute, 10);
    if (!isNaN(hourNum) && !isNaN(minuteNum)) {
      const minuteStr = minuteNum.toString().padStart(2, '0');
      const ampm = hourNum >= 12 ? 'PM' : 'AM';
      const h12 = hourNum === 0 ? 12 : hourNum > 12 ? hourNum - 12 : hourNum;
      segments.push(`at ${h12}:${minuteStr} ${ampm}`);
    }
  }

  if (dayOfWeek !== '*') {
    if (dayOfWeek === '1-5') {
      segments.push('on weekdays');
    } else if (dayOfWeek === '0,6') {
      segments.push('on weekends');
    } else {
      const days = dayOfWeek.split(',').map((d) => {
        const num = parseInt(d, 10);
        return !isNaN(num) && num >= 0 && num <= 6 ? WEEKDAYS[num] : d;
      });
      segments.push(`on ${days.join(', ')}`);
    }
  }

  return segments.length > 0 ? segments.join(', ') : null;
}
