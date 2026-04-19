import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import AgentTemplatesPage from '../AgentTemplatesPage';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('AgentTemplatesPage', () => {
  beforeEach(() => {
    // Clear cookies
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title', async () => {
    renderWithProviders(<AgentTemplatesPage />, { initialEntries: ['/templates'] });

    expect(screen.getByText('Agent Templates')).toBeInTheDocument();
    expect(screen.getByText('Reusable base configurations for creating Agents')).toBeInTheDocument();
  });

  it('renders template cards from API', async () => {
    renderWithProviders(<AgentTemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      expect(screen.getByText('standard-base')).toBeInTheDocument();
    });

    expect(screen.getByText('local-dev-base')).toBeInTheDocument();
    expect(screen.getByText('production-base')).toBeInTheDocument();
  });

  it('shows error state when API fails', async () => {
    server.use(
      http.get('/api/v1/agenttemplates', () => {
        return HttpResponse.json({ message: 'Server error' }, { status: 500 });
      })
    );

    renderWithProviders(<AgentTemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      expect(screen.getByText(/Error loading templates/)).toBeInTheDocument();
    });

    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows empty state', async () => {
    server.use(
      http.get('/api/v1/agenttemplates', () => {
        return HttpResponse.json({
          templates: [],
          total: 0,
          pagination: { limit: 12, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<AgentTemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      expect(screen.getByText(/No agent templates found/)).toBeInTheDocument();
    });
  });
});
