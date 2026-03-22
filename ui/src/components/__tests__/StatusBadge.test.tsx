import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import StatusBadge from '../StatusBadge';

describe('StatusBadge', () => {
  it('renders the phase text', () => {
    render(<StatusBadge phase="Running" />);
    expect(screen.getByText('Running')).toBeInTheDocument();
  });

  it.each([
    ['Pending', 'bg-slate-50'],
    ['Queued', 'bg-amber-50'],
    ['Running', 'bg-primary-50'],
    ['Completed', 'bg-emerald-50'],
    ['Failed', 'bg-red-50'],
  ])('applies correct background class for %s phase', (phase, expectedClass) => {
    render(<StatusBadge phase={phase} />);
    const badge = screen.getByText(phase);
    expect(badge.className).toContain(expectedClass);
  });

  it('shows animated dot for Running phase', () => {
    const { container } = render(<StatusBadge phase="Running" />);
    const animatedDot = container.querySelector('.animate-ping');
    expect(animatedDot).toBeInTheDocument();
    expect(animatedDot?.className).toContain('bg-primary-');
  });

  it('shows animated dot for Queued phase', () => {
    const { container } = render(<StatusBadge phase="Queued" />);
    const animatedDot = container.querySelector('.animate-ping');
    expect(animatedDot).toBeInTheDocument();
    expect(animatedDot?.className).toContain('bg-amber-400');
  });

  it('does not show animated dot for Completed phase but shows static dot', () => {
    const { container } = render(<StatusBadge phase="Completed" />);
    const animatedDot = container.querySelector('.animate-ping');
    expect(animatedDot).not.toBeInTheDocument();
    // Static dot should still be present
    const staticDot = container.querySelector('.rounded-full');
    expect(staticDot).toBeInTheDocument();
  });

  it('handles case-insensitive phases', () => {
    render(<StatusBadge phase="running" />);
    const badge = screen.getByText('running');
    expect(badge.className).toContain('bg-primary-50');
  });

  it('uses default style for unknown phases', () => {
    render(<StatusBadge phase="Unknown" />);
    const badge = screen.getByText('Unknown');
    expect(badge.className).toContain('bg-slate-50');
  });
});
