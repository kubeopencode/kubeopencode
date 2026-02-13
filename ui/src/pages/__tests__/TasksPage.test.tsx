import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { mockTasks } from '../../mocks/data';
import { renderWithProviders } from '../../test/utils';
import TasksPage from '../TasksPage';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('TasksPage', () => {
  beforeEach(() => {
    // Clear cookies
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title and description', async () => {
    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });

    expect(screen.getByText('Tasks')).toBeInTheDocument();
    expect(screen.getByText('Manage and monitor AI agent tasks')).toBeInTheDocument();
  });

  it('renders task list from API', async () => {
    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });

    await waitFor(() => {
      expect(screen.getByText('fix-bug-123')).toBeInTheDocument();
    });

    expect(screen.getByText('add-feature-456')).toBeInTheDocument();
    expect(screen.getByText('pending-task')).toBeInTheDocument();
  });

  it('shows loading skeleton while fetching', () => {
    server.use(
      http.get('/api/v1/namespaces/:namespace/tasks', async () => {
        await new Promise((r) => setTimeout(r, 100));
        return HttpResponse.json({ tasks: [], total: 0 });
      })
    );

    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });
    expect(screen.queryByText('fix-bug-123')).not.toBeInTheDocument();
  });

  it('shows error state when API fails', async () => {
    server.use(
      http.get('/api/v1/namespaces/:namespace/tasks', () => {
        return HttpResponse.json({ message: 'Server error' }, { status: 500 });
      })
    );

    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });

    await waitFor(() => {
      expect(screen.getByText(/Error loading tasks/)).toBeInTheDocument();
    });

    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows empty state when no tasks exist', async () => {
    server.use(
      http.get('/api/v1/namespaces/:namespace/tasks', () => {
        return HttpResponse.json({
          tasks: [],
          total: 0,
          pagination: { limit: 20, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });

    await waitFor(() => {
      expect(screen.getByText(/No tasks found/)).toBeInTheDocument();
    });
  });

  it('renders "New Task" link', async () => {
    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });
    const newTaskLink = screen.getByText('New Task');
    expect(newTaskLink).toBeInTheDocument();
    expect(newTaskLink.closest('a')).toHaveAttribute('href', expect.stringContaining('/tasks/create'));
  });

  it('renders namespace selector with namespaces from API', async () => {
    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });

    await waitFor(() => {
      const select = screen.getByDisplayValue('default');
      expect(select).toBeInTheDocument();
    });

    const options = screen.getAllByRole('option');
    const optionTexts = options.map((o) => o.textContent);
    expect(optionTexts).toContain('All Namespaces');
    expect(optionTexts).toContain('default');
    expect(optionTexts).toContain('production');
    expect(optionTexts).toContain('staging');
  });

  it('renders phase filter buttons', async () => {
    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });

    expect(screen.getByRole('button', { name: 'All' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Pending' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Running' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Completed' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Failed' })).toBeInTheDocument();
  });

  it('renders status badges for tasks in the table', async () => {
    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });

    await waitFor(() => {
      expect(screen.getByText('fix-bug-123')).toBeInTheDocument();
    });

    // The table body should contain task rows with status badges
    const table = screen.getByRole('table');
    const rows = table.querySelectorAll('tbody tr');
    expect(rows.length).toBe(3);
  });

  it('renders task names as links to detail pages', async () => {
    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });

    await waitFor(() => {
      const link = screen.getByText('fix-bug-123');
      expect(link.closest('a')).toHaveAttribute('href', '/tasks/default/fix-bug-123');
    });
  });

  it('renders pagination controls when data has pagination', async () => {
    server.use(
      http.get('/api/v1/namespaces/:namespace/tasks', () => {
        return HttpResponse.json({
          tasks: mockTasks,
          total: 50,
          pagination: { limit: 20, offset: 0, totalCount: 50, hasMore: true },
        });
      })
    );

    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });

    await waitFor(() => {
      expect(screen.getByText('fix-bug-123')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText(/Showing/)).toBeInTheDocument();
    });
  });

  it('filters tasks by namespace when namespace changes', async () => {
    const user = userEvent.setup();
    let lastRequestUrl = '';

    server.use(
      http.get('/api/v1/namespaces/:namespace/tasks', ({ request }) => {
        lastRequestUrl = request.url;
        return HttpResponse.json({
          tasks: [],
          total: 0,
          pagination: { limit: 20, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<TasksPage />, { initialEntries: ['/tasks'] });

    await waitFor(() => {
      expect(screen.getByDisplayValue('default')).toBeInTheDocument();
    });

    const select = screen.getByDisplayValue('default');
    await user.selectOptions(select, 'production');

    await waitFor(() => {
      expect(lastRequestUrl).toContain('/namespaces/production/tasks');
    });
  });
});
