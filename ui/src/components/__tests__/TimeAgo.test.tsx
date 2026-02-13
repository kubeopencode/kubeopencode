import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import TimeAgo from '../TimeAgo';

describe('TimeAgo', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-02-13T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders relative time', () => {
    render(<TimeAgo date="2026-02-13T11:55:00Z" />);
    expect(screen.getByText('5m ago')).toBeInTheDocument();
  });

  it('renders "just now" for recent times', () => {
    render(<TimeAgo date="2026-02-13T11:59:58Z" />);
    expect(screen.getByText('just now')).toBeInTheDocument();
  });

  it('shows full timestamp in title attribute', () => {
    render(<TimeAgo date="2026-02-13T12:00:00Z" />);
    const element = screen.getByText('just now');
    expect(element.title).toBeTruthy();
    expect(element.title.length).toBeGreaterThan(0);
  });

  it('applies custom className', () => {
    render(<TimeAgo date="2026-02-13T11:55:00Z" className="text-red-500" />);
    const element = screen.getByText('5m ago');
    expect(element.className).toContain('text-red-500');
  });

  it('accepts Date objects', () => {
    render(<TimeAgo date={new Date('2026-02-13T11:00:00Z')} />);
    expect(screen.getByText('1h ago')).toBeInTheDocument();
  });
});
