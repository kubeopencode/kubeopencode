import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import CronTaskDetailPage from '../CronTaskDetailPage';
import { Route, Routes } from 'react-router-dom';
import { mockCronTasks, mockCronTaskHistory } from '../../mocks/data';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

// Mock YamlViewer to simplify tests
vi.mock('../../components/YamlViewer', () => ({
  default: () => <div data-testid="yaml-viewer">YamlViewer</div>,
}));

function renderCronTaskDetailPage(namespace: string, name: string) {
  return renderWithProviders(
    <Routes>
      <Route path="/crontasks/:namespace/:name" element={<CronTaskDetailPage />} />
    </Routes>,
    { initialEntries: [`/crontasks/${namespace}/${name}`] }
  );
}

describe('CronTaskDetailPage', () => {
  it('renders crontask name and status', async () => {
    renderCronTaskDetailPage('default', 'daily-vuln-scan');

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'daily-vuln-scan' })).toBeInTheDocument();
    });

    expect(screen.getByText('Enabled')).toBeInTheDocument();
  });

  it('renders schedule information', async () => {
    renderCronTaskDetailPage('default', 'daily-vuln-scan');

    await waitFor(() => {
      expect(screen.getByText('0 9 * * 1-5')).toBeInTheDocument();
    });
  });

  it('shows not found error for invalid crontask', async () => {
    renderCronTaskDetailPage('default', 'nonexistent-crontask');

    await waitFor(() => {
      expect(screen.getByText(/not found/i)).toBeInTheDocument();
    });
  });

  it('renders action buttons', async () => {
    renderCronTaskDetailPage('default', 'daily-vuln-scan');

    await waitFor(() => {
      expect(screen.getByText('Run Now')).toBeInTheDocument();
    });

    expect(screen.getByText('Suspend')).toBeInTheDocument();
    expect(screen.getByText('Delete')).toBeInTheDocument();
  });

  it('disables Run Now button when maxRetainedTasks limit is reached', async () => {
    server.use(
      http.get('/api/v1/namespaces/:namespace/crontasks/:name', () => {
        const crontask = mockCronTasks.find(c => c.name === 'daily-vuln-scan')!;
        return HttpResponse.json({ ...crontask, maxRetainedTasks: 1 });
      }),
      http.get('/api/v1/namespaces/:namespace/crontasks/:name/history', () => {
        return HttpResponse.json({
          tasks: mockCronTaskHistory.slice(0, 2),
          total: 2,
          pagination: { limit: 20, offset: 0, totalCount: 2, hasMore: false },
        });
      }),
    );

    renderCronTaskDetailPage('default', 'daily-vuln-scan');

    await waitFor(() => {
      const runNowButton = screen.getByText('Run Now');
      expect(runNowButton).toBeDisabled();
    });
  });
});
