import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import AgentsPage from '../AgentsPage';

describe('AgentsPage', () => {
  beforeEach(() => {
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title and description', () => {
    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });
    expect(screen.getByText('Agents')).toBeInTheDocument();
    expect(screen.getByText('Browse available AI agents for task execution')).toBeInTheDocument();
  });

  it('renders agent cards from API', async () => {
    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });

    await waitFor(() => {
      expect(screen.getByText('opencode-agent')).toBeInTheDocument();
    });

    expect(screen.getByText('global-agent')).toBeInTheDocument();
    expect(screen.getByText('restricted-agent')).toBeInTheDocument();
  });

  it('shows agent configuration details on cards', async () => {
    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });

    await waitFor(() => {
      expect(screen.getByText('opencode-agent')).toBeInTheDocument();
    });

    // Should show contexts/credentials counts
    const contextLabels = screen.getAllByText('Contexts');
    expect(contextLabels.length).toBeGreaterThan(0);

    const credentialLabels = screen.getAllByText('Credentials');
    expect(credentialLabels.length).toBeGreaterThan(0);
  });

  it('shows maxConcurrentTasks badge when set', async () => {
    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });

    await waitFor(() => {
      expect(screen.getByText('Max 5')).toBeInTheDocument();
    });
  });

  it('shows allowed namespaces on agent cards', async () => {
    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });

    await waitFor(() => {
      // Multiple agents may have allowed namespaces
      const labels = screen.getAllByText('Allowed namespaces:');
      expect(labels.length).toBeGreaterThan(0);
    });
  });

  it('renders agent cards as links to detail pages', async () => {
    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });

    await waitFor(() => {
      const link = screen.getByText('opencode-agent').closest('a');
      expect(link).toHaveAttribute('href', '/agents/default/opencode-agent');
    });
  });

  it('renders namespace selector', async () => {
    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });

    await waitFor(() => {
      const options = screen.getAllByRole('option');
      const optionTexts = options.map((o) => o.textContent);
      expect(optionTexts).toContain('All Namespaces');
    });
  });

  it('renders filter component', () => {
    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });
    expect(screen.getByPlaceholderText('Filter agents by name...')).toBeInTheDocument();
  });

  it('shows error state when API fails', async () => {
    server.use(
      http.get('/api/v1/agents', () => {
        return HttpResponse.json({ message: 'Internal error' }, { status: 500 });
      })
    );

    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });

    await waitFor(() => {
      expect(screen.getByText(/Error loading agents/)).toBeInTheDocument();
    });

    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows empty state when no agents found', async () => {
    server.use(
      http.get('/api/v1/agents', () => {
        return HttpResponse.json({
          agents: [],
          total: 0,
          pagination: { limit: 12, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });

    await waitFor(() => {
      expect(screen.getByText(/No agents found/)).toBeInTheDocument();
    });
  });

  it('filters by namespace when namespace is changed', async () => {
    const user = userEvent.setup();
    let lastRequestUrl = '';

    server.use(
      http.get('/api/v1/namespaces/:namespace/agents', ({ request }) => {
        lastRequestUrl = request.url;
        return HttpResponse.json({
          agents: [],
          total: 0,
          pagination: { limit: 12, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<AgentsPage />, { initialEntries: ['/agents'] });

    await waitFor(() => {
      expect(screen.getByText('opencode-agent')).toBeInTheDocument();
    });

    // Select a specific namespace
    const select = screen.getAllByRole('combobox')[0];
    await user.selectOptions(select, 'production');

    await waitFor(() => {
      expect(lastRequestUrl).toContain('/namespaces/production/agents');
    });
  });
});
