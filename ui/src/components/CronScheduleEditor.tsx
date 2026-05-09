import React, { useMemo } from 'react';

interface CronPreset {
  label: string;
  cron: string;
  description: string;
}

const PRESETS: CronPreset[] = [
  { label: 'Every hour', cron: '0 * * * *', description: 'At minute 0 of every hour' },
  { label: 'Daily at 9am', cron: '0 9 * * *', description: 'Every day at 9:00 AM' },
  { label: 'Weekdays at 9am', cron: '0 9 * * 1-5', description: 'Monday-Friday at 9:00 AM' },
  { label: 'Daily at midnight', cron: '0 0 * * *', description: 'Every day at midnight' },
  { label: 'Weekly (Monday)', cron: '0 10 * * 1', description: 'Every Monday at 10:00 AM' },
  { label: 'Monthly (1st)', cron: '0 10 1 * *', description: 'First day of each month at 10:00 AM' },
  { label: 'Every 30 min', cron: '*/30 * * * *', description: 'Every 30 minutes' },
  { label: 'Every 6 hours', cron: '0 */6 * * *', description: 'At minute 0 every 6 hours' },
];

const CRON_FIELDS = ['Minute', 'Hour', 'Day (Month)', 'Month', 'Day (Week)'];
const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

// Parse a cron expression into a human-readable description
function describeCron(cron: string): string | null {
  if (!cron.trim()) return null;
  const parts = cron.trim().split(/\s+/);
  if (parts.length !== 5) return 'Invalid: expected 5 fields (minute hour day month weekday)';

  // Check if it matches a preset
  const preset = PRESETS.find((p) => p.cron === cron.trim());
  if (preset) return preset.description;

  const [minute, hour, dayOfMonth, month, dayOfWeek] = parts;

  const segments: string[] = [];

  // Minute
  if (minute.startsWith('*/')) {
    segments.push(`every ${minute.slice(2)} minutes`);
  } else if (minute === '*') {
    segments.push('every minute');
  }

  // Hour
  if (hour.startsWith('*/')) {
    segments.push(`every ${hour.slice(2)} hours`);
  } else if (hour !== '*') {
    const hourNum = parseInt(hour, 10);
    if (!isNaN(hourNum)) {
      const minuteNum = parseInt(minute, 10);
      const minuteStr = !isNaN(minuteNum) ? minuteNum.toString().padStart(2, '0') : '00';
      const ampm = hourNum >= 12 ? 'PM' : 'AM';
      const h12 = hourNum === 0 ? 12 : hourNum > 12 ? hourNum - 12 : hourNum;
      // Only include time if minute is not a pattern
      if (!minute.includes('*') && !minute.includes('/')) {
        segments.push(`at ${h12}:${minuteStr} ${ampm}`);
      } else {
        segments.push(`during hour ${hourNum}`);
      }
    }
  }

  // Day of month
  if (dayOfMonth !== '*' && !dayOfMonth.includes('/')) {
    segments.push(`on day ${dayOfMonth}`);
  }

  // Month
  if (month !== '*') {
    segments.push(`in month ${month}`);
  }

  // Day of week
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

  return segments.length > 0 ? segments.join(', ') : 'custom schedule';
}

// Calculate next N run times from a cron expression
function getNextRuns(cron: string, count: number, timeZone?: string): Date[] {
  const parts = cron.trim().split(/\s+/);
  if (parts.length !== 5) return [];

  const [minuteField, hourField, domField, monthField, dowField] = parts;
  const results: Date[] = [];

  // Start from the next minute
  const now = new Date();
  const current = new Date(now.getTime() + 60000);
  current.setSeconds(0, 0);

  // Simple brute force: check each minute for up to 90 days
  const maxIterations = 90 * 24 * 60;

  for (let i = 0; i < maxIterations && results.length < count; i++) {
    const d = new Date(current.getTime() + i * 60000);
    const min = d.getMinutes();
    const hr = d.getHours();
    const dom = d.getDate();
    const mon = d.getMonth() + 1;
    const dow = d.getDay();

    if (
      matchField(minuteField, min, 0, 59) &&
      matchField(hourField, hr, 0, 23) &&
      matchField(domField, dom, 1, 31) &&
      matchField(monthField, mon, 1, 12) &&
      matchField(dowField, dow, 0, 6)
    ) {
      results.push(d);
    }
  }

  return results;
}

function matchField(field: string, value: number, min: number, max: number): boolean {
  if (field === '*') return true;

  // Step values: */N or M/N
  if (field.includes('/')) {
    const [range, step] = field.split('/');
    const stepNum = parseInt(step, 10);
    if (isNaN(stepNum) || stepNum <= 0) return false;
    if (range === '*') return value % stepNum === 0;
    const start = parseInt(range, 10);
    if (isNaN(start)) return false;
    return value >= start && (value - start) % stepNum === 0;
  }

  // Ranges and lists
  const parts = field.split(',');
  for (const part of parts) {
    if (part.includes('-')) {
      const [from, to] = part.split('-').map(Number);
      if (value >= from && value <= to) return true;
    } else {
      if (parseInt(part, 10) === value) return true;
    }
  }

  return false;
}

interface CronScheduleEditorProps {
  value: string;
  onChange: (value: string) => void;
  timeZone?: string;
}

function CronScheduleEditor({ value, onChange, timeZone }: CronScheduleEditorProps) {
  const description = useMemo(() => describeCron(value), [value]);
  const nextRuns = useMemo(() => {
    if (!value.trim()) return [];
    return getNextRuns(value, 5, timeZone);
  }, [value, timeZone]);

  const isValid = value.trim().split(/\s+/).length === 5;
  const parts = value.trim().split(/\s+/);

  return (
    <div className="space-y-3">
      {/* Input */}
      <div>
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          required
          placeholder="0 9 * * 1-5"
          className="block w-full px-3 py-2 rounded-lg border border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 font-mono placeholder:text-stone-300 placeholder:font-body"
        />
        {/* Description */}
        {description && isValid && (
          <p className="mt-1.5 text-xs text-primary-600 font-medium">{description}</p>
        )}
        {value && !isValid && (
          <p className="mt-1.5 text-xs text-red-500">Invalid cron expression. Expected 5 fields: minute hour day month weekday</p>
        )}
      </div>

      {/* Field labels */}
      {isValid && (
        <div className="flex gap-1">
          {parts.map((part, idx) => (
            <div key={idx} className="flex-1 text-center">
              <div className="bg-stone-100 rounded px-2 py-1 text-xs font-mono text-stone-700">{part}</div>
              <div className="text-[9px] text-stone-400 mt-0.5 uppercase tracking-wider">{CRON_FIELDS[idx]}</div>
            </div>
          ))}
        </div>
      )}

      {/* Presets */}
      <div>
        <p className="text-[10px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5">Quick Presets</p>
        <div className="flex flex-wrap gap-1.5">
          {PRESETS.map((preset) => (
            <button
              key={preset.cron}
              type="button"
              onClick={() => onChange(preset.cron)}
              className={`px-2.5 py-1 text-[11px] font-medium rounded-md border transition-colors ${
                value.trim() === preset.cron
                  ? 'bg-primary-50 text-primary-700 border-primary-200'
                  : 'bg-white text-stone-500 border-stone-200 hover:bg-stone-50 hover:text-stone-700'
              }`}
            >
              {preset.label}
            </button>
          ))}
        </div>
      </div>

      {/* Next runs preview */}
      {isValid && nextRuns.length > 0 && (
        <div className="bg-stone-50 rounded-lg border border-stone-100 p-3">
          <p className="text-[10px] font-display font-medium text-stone-400 uppercase tracking-wider mb-2">
            Next {nextRuns.length} Executions {timeZone ? `(${timeZone})` : ''}
          </p>
          <div className="space-y-1">
            {nextRuns.map((date, idx) => (
              <div key={idx} className="flex items-center justify-between text-xs">
                <span className="text-stone-500 font-mono text-[11px]">
                  {date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })}
                </span>
                <span className="text-stone-700 font-mono text-[11px]">
                  {date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

export default CronScheduleEditor;
