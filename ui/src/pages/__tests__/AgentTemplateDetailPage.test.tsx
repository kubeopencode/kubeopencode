import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import AgentTemplateDetailPage from '../AgentTemplateDetailPage';
import { Route, Routes } from 'react-router-dom';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

// Mock YamlViewer to simplify tests
vi.mock('../../components/YamlViewer', () => ({
  default: () => <div data-testid="yaml-viewer">YamlViewer</div>,
}));

function renderAgentTemplateDetailPage(namespace: string, name: string) {
  return renderWithProviders(
    <Routes>
      <Route path="/templates/:namespace/:name" element={<AgentTemplateDetailPage />} />
    </Routes>,
    { initialEntries: [`/templates/${namespace}/${name}`] }
  );
}

describe('AgentTemplateDetailPage', () => {
  it('renders template name', async () => {
    renderAgentTemplateDetailPage('default', 'standard-base');

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'standard-base' })).toBeInTheDocument();
    });
  });

  it('shows not found error', async () => {
    renderAgentTemplateDetailPage('default', 'nonexistent-template');

    await waitFor(() => {
      expect(screen.getByText(/not found/i)).toBeInTheDocument();
    });
  });

  it('renders delete button', async () => {
    renderAgentTemplateDetailPage('default', 'standard-base');

    await waitFor(() => {
      expect(screen.getByText('Delete')).toBeInTheDocument();
    });
  });
});
