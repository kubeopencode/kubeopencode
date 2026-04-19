import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import ConfigPage from '../ConfigPage';

// Mock YamlViewer to avoid complex rendering
vi.mock('../../components/YamlViewer', () => ({
  default: () => <div data-testid="yaml-viewer">YamlViewer</div>,
}));

describe('ConfigPage', () => {
  beforeEach(() => {
    // Clear cookies
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title', async () => {
    renderWithProviders(<ConfigPage />, { initialEntries: ['/config'] });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Cluster Configuration' })).toBeInTheDocument();
    });
  });

  it('renders config sections', async () => {
    renderWithProviders(<ConfigPage />, { initialEntries: ['/config'] });

    await waitFor(() => {
      expect(screen.getByText('System Image')).toBeInTheDocument();
    });

    expect(screen.getByText('Task Cleanup')).toBeInTheDocument();
    expect(screen.getByText('Proxy')).toBeInTheDocument();
  });

  it('shows not found state when config does not exist', async () => {
    server.use(
      http.get('/api/v1/config', () => {
        return HttpResponse.json({ error: 'not found' }, { status: 404 });
      })
    );

    renderWithProviders(<ConfigPage />, { initialEntries: ['/config'] });

    await waitFor(() => {
      expect(screen.getByText('No Configuration Found')).toBeInTheDocument();
    });
  });
});
