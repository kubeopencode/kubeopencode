import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '../../test/utils';
import Breadcrumbs from '../Breadcrumbs';

describe('Breadcrumbs', () => {
  it('renders all breadcrumb items', () => {
    renderWithProviders(
      <Breadcrumbs items={[
        { label: 'Home', to: '/' },
        { label: 'Tasks', to: '/tasks' },
        { label: 'my-task' },
      ]} />
    );

    expect(screen.getByText('Home')).toBeInTheDocument();
    expect(screen.getByText('Tasks')).toBeInTheDocument();
    expect(screen.getByText('my-task')).toBeInTheDocument();
  });

  it('renders links for items with "to" property', () => {
    renderWithProviders(
      <Breadcrumbs items={[
        { label: 'Tasks', to: '/tasks' },
        { label: 'current' },
      ]} />
    );

    const link = screen.getByText('Tasks');
    expect(link.tagName).toBe('A');
    expect(link).toHaveAttribute('href', '/tasks');
  });

  it('renders plain text for the last item (no "to")', () => {
    renderWithProviders(
      <Breadcrumbs items={[
        { label: 'Tasks', to: '/tasks' },
        { label: 'my-task' },
      ]} />
    );

    const lastItem = screen.getByText('my-task');
    expect(lastItem.tagName).toBe('SPAN');
    expect(lastItem.className).toContain('font-medium');
  });

  it('has proper aria label', () => {
    renderWithProviders(
      <Breadcrumbs items={[{ label: 'Home' }]} />
    );

    expect(screen.getByLabelText('Breadcrumb')).toBeInTheDocument();
  });

  it('renders separator between items', () => {
    const { container } = renderWithProviders(
      <Breadcrumbs items={[
        { label: 'A', to: '/' },
        { label: 'B', to: '/b' },
        { label: 'C' },
      ]} />
    );

    // SVG separators between items (2 separators for 3 items)
    const svgs = container.querySelectorAll('svg');
    expect(svgs.length).toBe(2);
  });

  it('does not render separator before first item', () => {
    const { container } = renderWithProviders(
      <Breadcrumbs items={[{ label: 'Only' }]} />
    );

    const svgs = container.querySelectorAll('svg');
    expect(svgs.length).toBe(0);
  });
});
