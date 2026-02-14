import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import StatusBadge from '../StatusBadge';

describe('StatusBadge', () => {
  it('renders the phase text', () => {
    render(<StatusBadge phase="Running" />);
    expect(screen.getByText('Running')).toBeInTheDocument();
  });

  it.each([
    ['Pending', 'bg-gray-100'],
    ['Queued', 'bg-yellow-100'],
    ['Running', 'bg-blue-100'],
    ['Completed', 'bg-green-100'],
    ['Failed', 'bg-red-100'],
  ])('applies correct background class for %s phase', (phase, expectedClass) => {
    render(<StatusBadge phase={phase} />);
    const badge = screen.getByText(phase);
    expect(badge.className).toContain(expectedClass);
  });

  it('shows animated dot for Running phase', () => {
    const { container } = render(<StatusBadge phase="Running" />);
    const animatedDot = container.querySelector('.animate-ping');
    expect(animatedDot).toBeInTheDocument();
    expect(animatedDot?.className).toContain('bg-blue-500');
  });

  it('shows animated dot for Queued phase', () => {
    const { container } = render(<StatusBadge phase="Queued" />);
    const animatedDot = container.querySelector('.animate-ping');
    expect(animatedDot).toBeInTheDocument();
    expect(animatedDot?.className).toContain('bg-yellow-500');
  });

  it('does not show animated dot for Completed phase', () => {
    const { container } = render(<StatusBadge phase="Completed" />);
    const animatedDot = container.querySelector('.animate-ping');
    expect(animatedDot).not.toBeInTheDocument();
  });

  it('handles case-insensitive phases', () => {
    render(<StatusBadge phase="running" />);
    const badge = screen.getByText('running');
    expect(badge.className).toContain('bg-blue-100');
  });

  it('uses default style for unknown phases', () => {
    render(<StatusBadge phase="Unknown" />);
    const badge = screen.getByText('Unknown');
    expect(badge.className).toContain('bg-gray-100');
  });
});
