import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import CronTasksPage from '../CronTasksPage';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('CronTasksPage', () => {
  beforeEach(() => {
    // Clear cookies
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title and description', async () => {
    renderWithProviders(<CronTasksPage />, { initialEntries: ['/crontasks'] });

    expect(screen.getByText('CronTasks')).toBeInTheDocument();
    expect(screen.getByText('Scheduled AI agent tasks running on a cron schedule')).toBeInTheDocument();
  });

  it('renders crontask list from API', async () => {
    renderWithProviders(<CronTasksPage />, { initialEntries: ['/crontasks'] });

    await waitFor(() => {
      expect(screen.getByText('daily-vuln-scan')).toBeInTheDocument();
    });

    expect(screen.getByText('weekly-code-review')).toBeInTheDocument();
  });

  it('shows error state when API fails', async () => {
    server.use(
      http.get('/api/v1/crontasks', () => {
        return HttpResponse.json({ message: 'Server error' }, { status: 500 });
      })
    );

    renderWithProviders(<CronTasksPage />, { initialEntries: ['/crontasks'] });

    await waitFor(() => {
      expect(screen.getByText(/Error loading CronTasks/)).toBeInTheDocument();
    });

    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows empty state when no crontasks exist', async () => {
    server.use(
      http.get('/api/v1/crontasks', () => {
        return HttpResponse.json({
          cronTasks: [],
          total: 0,
          pagination: { limit: 20, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<CronTasksPage />, { initialEntries: ['/crontasks'] });

    await waitFor(() => {
      expect(screen.getByText(/No CronTasks found/)).toBeInTheDocument();
    });
  });
});
