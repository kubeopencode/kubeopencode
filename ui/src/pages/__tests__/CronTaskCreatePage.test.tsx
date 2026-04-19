import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '../../test/utils';
import CronTaskCreatePage from '../CronTaskCreatePage';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('CronTaskCreatePage', () => {
  beforeEach(() => {
    // Clear cookies
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title and breadcrumbs', () => {
    renderWithProviders(<CronTaskCreatePage />, { initialEntries: ['/crontasks/create'] });

    expect(screen.getByRole('heading', { name: 'Create CronTask' })).toBeInTheDocument();
    const breadcrumb = screen.getByLabelText('Breadcrumb');
    expect(breadcrumb.textContent).toContain('CronTasks');
  });

  it('renders form fields', () => {
    renderWithProviders(<CronTaskCreatePage />, { initialEntries: ['/crontasks/create'] });

    expect(screen.getByText('Schedule')).toBeInTheDocument();
    expect(screen.getByText('Description')).toBeInTheDocument();
  });

  it('renders cancel and submit buttons', () => {
    renderWithProviders(<CronTaskCreatePage />, { initialEntries: ['/crontasks/create'] });

    const cancelLink = screen.getByText('Cancel');
    expect(cancelLink.closest('a')).toHaveAttribute('href', '/crontasks');

    const submitButton = screen.getByRole('button', { name: 'Create CronTask' });
    expect(submitButton).toBeInTheDocument();
  });
});
