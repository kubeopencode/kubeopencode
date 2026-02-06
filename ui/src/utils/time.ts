// Relative time formatting utilities

const MINUTE = 60;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;
const MONTH = 30 * DAY;
const YEAR = 365 * DAY;

/**
 * Format a date as a relative time string (e.g., "3m ago", "2h ago").
 * Falls back to a short date format for dates older than 30 days.
 */
export function formatRelativeTime(date: string | Date): string {
  const d = typeof date === 'string' ? new Date(date) : date;
  const now = new Date();
  const diffSeconds = Math.floor((now.getTime() - d.getTime()) / 1000);

  if (diffSeconds < 0) {
    return 'just now';
  }

  if (diffSeconds < MINUTE) {
    return diffSeconds <= 5 ? 'just now' : `${diffSeconds}s ago`;
  }

  if (diffSeconds < HOUR) {
    const minutes = Math.floor(diffSeconds / MINUTE);
    return `${minutes}m ago`;
  }

  if (diffSeconds < DAY) {
    const hours = Math.floor(diffSeconds / HOUR);
    return `${hours}h ago`;
  }

  if (diffSeconds < WEEK) {
    const days = Math.floor(diffSeconds / DAY);
    return `${days}d ago`;
  }

  if (diffSeconds < MONTH) {
    const weeks = Math.floor(diffSeconds / WEEK);
    return `${weeks}w ago`;
  }

  if (diffSeconds < YEAR) {
    const months = Math.floor(diffSeconds / MONTH);
    return `${months}mo ago`;
  }

  const years = Math.floor(diffSeconds / YEAR);
  return `${years}y ago`;
}

/**
 * Format a date as a full locale string for tooltips.
 */
export function formatFullTime(date: string | Date): string {
  const d = typeof date === 'string' ? new Date(date) : date;
  return d.toLocaleString();
}
