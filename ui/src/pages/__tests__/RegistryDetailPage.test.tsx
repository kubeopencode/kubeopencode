import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import RegistryDetailPage from '../RegistryDetailPage';
import { Route, Routes } from 'react-router-dom';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

// Mock YamlViewer to simplify tests
vi.mock('../../components/YamlViewer', () => ({
  default: () => <div data-testid="yaml-viewer">YamlViewer</div>,
}));

function renderRegistryDetailPage(namespace: string, name: string) {
  return renderWithProviders(
    <Routes>
      <Route path="/registries/:namespace/:name" element={<RegistryDetailPage />} />
    </Routes>,
    { initialEntries: [`/registries/${namespace}/${name}`] }
  );
}

describe('RegistryDetailPage', () => {
  it('renders registry name', async () => {
    renderRegistryDetailPage('default', 'official-catalog');

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'official-catalog' })).toBeInTheDocument();
    });
  });

  it('shows not found error', async () => {
    renderRegistryDetailPage('default', 'nonexistent-registry');

    await waitFor(() => {
      expect(screen.getByText(/not found/i)).toBeInTheDocument();
    });
  });

  it('renders delete button', async () => {
    renderRegistryDetailPage('default', 'official-catalog');

    await waitFor(() => {
      expect(screen.getByText('Delete')).toBeInTheDocument();
    });
  });
});
