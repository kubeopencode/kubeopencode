import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import RegistriesPage from '../RegistriesPage';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('RegistriesPage', () => {
  beforeEach(() => {
    // Clear cookies
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title', async () => {
    renderWithProviders(<RegistriesPage />, { initialEntries: ['/registries'] });

    expect(screen.getByText('Registries')).toBeInTheDocument();
    expect(screen.getByText('Asset catalogs for agent assembly')).toBeInTheDocument();
  });

  it('renders registry cards from API', async () => {
    renderWithProviders(<RegistriesPage />, { initialEntries: ['/registries'] });

    await waitFor(() => {
      expect(screen.getByText('official-catalog')).toBeInTheDocument();
    });

    expect(screen.getByText('team-alpha-registry')).toBeInTheDocument();
    expect(screen.getByText('minimal-registry')).toBeInTheDocument();
  });

  it('shows error state when API fails', async () => {
    server.use(
      http.get('/api/v1/registries', () => {
        return HttpResponse.json({ message: 'Server error' }, { status: 500 });
      })
    );

    renderWithProviders(<RegistriesPage />, { initialEntries: ['/registries'] });

    await waitFor(() => {
      expect(screen.getByText(/Error loading registries/)).toBeInTheDocument();
    });

    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows empty state', async () => {
    server.use(
      http.get('/api/v1/registries', () => {
        return HttpResponse.json({
          registries: [],
          total: 0,
          pagination: { limit: 12, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<RegistriesPage />, { initialEntries: ['/registries'] });

    await waitFor(() => {
      expect(screen.getByText(/No registries found/)).toBeInTheDocument();
    });
  });
});
