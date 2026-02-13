import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { formatRelativeTime, formatFullTime } from '../time';

describe('time utilities', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-02-13T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe('formatRelativeTime', () => {
    it('returns "just now" for times within 5 seconds', () => {
      const date = new Date('2026-02-13T11:59:57Z');
      expect(formatRelativeTime(date)).toBe('just now');
    });

    it('returns "just now" for future times', () => {
      const date = new Date('2026-02-13T12:00:10Z');
      expect(formatRelativeTime(date)).toBe('just now');
    });

    it('returns seconds ago for times between 6-59 seconds', () => {
      const date = new Date('2026-02-13T11:59:30Z');
      expect(formatRelativeTime(date)).toBe('30s ago');
    });

    it('returns minutes ago', () => {
      const date = new Date('2026-02-13T11:55:00Z');
      expect(formatRelativeTime(date)).toBe('5m ago');
    });

    it('returns hours ago', () => {
      const date = new Date('2026-02-13T09:00:00Z');
      expect(formatRelativeTime(date)).toBe('3h ago');
    });

    it('returns days ago', () => {
      const date = new Date('2026-02-11T12:00:00Z');
      expect(formatRelativeTime(date)).toBe('2d ago');
    });

    it('returns weeks ago', () => {
      const date = new Date('2026-01-30T12:00:00Z');
      expect(formatRelativeTime(date)).toBe('2w ago');
    });

    it('returns months ago', () => {
      const date = new Date('2025-11-13T12:00:00Z');
      expect(formatRelativeTime(date)).toBe('3mo ago');
    });

    it('returns years ago', () => {
      const date = new Date('2025-02-13T12:00:00Z');
      expect(formatRelativeTime(date)).toBe('1y ago');
    });

    it('accepts string dates', () => {
      expect(formatRelativeTime('2026-02-13T11:55:00Z')).toBe('5m ago');
    });
  });

  describe('formatFullTime', () => {
    it('returns a locale string for a Date object', () => {
      const date = new Date('2026-02-13T12:00:00Z');
      const result = formatFullTime(date);
      expect(typeof result).toBe('string');
      expect(result.length).toBeGreaterThan(0);
    });

    it('accepts string dates', () => {
      const result = formatFullTime('2026-02-13T12:00:00Z');
      expect(typeof result).toBe('string');
      expect(result.length).toBeGreaterThan(0);
    });
  });
});
