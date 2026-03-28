import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import StatusBadge from '../StatusBadge';

describe('StatusBadge', () => {
  it('renders the phase text', () => {
    render(<StatusBadge phase="Running" />);
    expect(screen.getByText('Running')).toBeInTheDocument();
  });

  it.each([
    ['Pending', 'text-slate-600'],
    ['Queued', 'text-amber-700'],
    ['Running', 'text-primary-700'],
    ['Completed', 'text-emerald-700'],
    ['Failed', 'text-red-700'],
  ])('applies correct text class for %s phase', (phase, expectedClass) => {
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
    expect(badge.className).toContain('text-primary-700');
  });

  it('uses default style for unknown phases', () => {
    render(<StatusBadge phase="Unknown" />);
    const badge = screen.getByText('Unknown');
    expect(badge.className).toContain('text-slate-600');
  });
});
