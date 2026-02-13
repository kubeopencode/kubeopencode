import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import DashboardPage from '../DashboardPage';

vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('DashboardPage', () => {
  it('renders the dashboard heading and new task link', () => {
    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
    expect(screen.getByText('+ New Task')).toBeInTheDocument();
  });

  it('renders stat cards with data from API', async () => {
    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    await waitFor(() => {
      expect(screen.getByText('Total Tasks')).toBeInTheDocument();
    });

    expect(screen.getByText('Running')).toBeInTheDocument();
    expect(screen.getByText('Queued')).toBeInTheDocument();
    expect(screen.getByText('Completed')).toBeInTheDocument();
    expect(screen.getByText('Failed')).toBeInTheDocument();
  });

  it('shows stat values once loaded', async () => {
    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    // Total tasks count should appear (3 from mock data)
    await waitFor(() => {
      expect(screen.getByText('3')).toBeInTheDocument();
    });
  });

  it('renders recent tasks section with task names', async () => {
    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    expect(screen.getByText('Recent Tasks')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('fix-bug-123')).toBeInTheDocument();
    });

    expect(screen.getByText('add-feature-456')).toBeInTheDocument();
    expect(screen.getByText('pending-task')).toBeInTheDocument();
  });

  it('renders task names as links to detail pages', async () => {
    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    await waitFor(() => {
      const link = screen.getByText('fix-bug-123').closest('a');
      expect(link).toHaveAttribute('href', '/tasks/default/fix-bug-123');
    });
  });

  it('renders agents sidebar', async () => {
    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    expect(screen.getByText('Agents')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('opencode-agent')).toBeInTheDocument();
    });

    expect(screen.getByText('global-agent')).toBeInTheDocument();
    expect(screen.getByText('restricted-agent')).toBeInTheDocument();
  });

  it('renders agent names as links to detail pages', async () => {
    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    await waitFor(() => {
      const link = screen.getByText('opencode-agent').closest('a');
      expect(link).toHaveAttribute('href', '/agents/default/opencode-agent');
    });
  });

  it('shows agent mode badges', async () => {
    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    await waitFor(() => {
      // Pod mode agents and Server mode agent
      const podBadges = screen.getAllByText('Pod');
      expect(podBadges.length).toBe(2);
      expect(screen.getByText('Server')).toBeInTheDocument();
    });
  });

  it('shows "View all" links', async () => {
    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    const viewAllLinks = screen.getAllByText('View all');
    expect(viewAllLinks.length).toBe(2); // tasks + agents
  });

  it('shows empty state when no tasks exist', async () => {
    server.use(
      http.get('/api/v1/tasks', () => {
        return HttpResponse.json({
          tasks: [],
          total: 0,
          pagination: { limit: 10, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    await waitFor(() => {
      expect(screen.getByText(/No tasks yet/)).toBeInTheDocument();
    });

    expect(screen.getByText('Create one')).toBeInTheDocument();
  });

  it('shows empty state when no agents exist', async () => {
    server.use(
      http.get('/api/v1/agents', () => {
        return HttpResponse.json({
          agents: [],
          total: 0,
          pagination: { limit: 100, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    await waitFor(() => {
      expect(screen.getByText('No agents configured')).toBeInTheDocument();
    });
  });

  it('shows loading dashes while data is fetching', () => {
    server.use(
      http.get('/api/v1/tasks', async () => {
        await new Promise((r) => setTimeout(r, 200));
        return HttpResponse.json({ tasks: [], total: 0 });
      }),
      http.get('/api/v1/agents', async () => {
        await new Promise((r) => setTimeout(r, 200));
        return HttpResponse.json({ agents: [], total: 0 });
      })
    );

    renderWithProviders(<DashboardPage />, { initialEntries: ['/'] });

    // Stat cards show '-' when loading
    const dashes = screen.getAllByText('-');
    expect(dashes.length).toBe(5); // 5 stat cards
  });
});
