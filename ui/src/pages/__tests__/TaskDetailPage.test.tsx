import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import TaskDetailPage from '../TaskDetailPage';
import { Route, Routes } from 'react-router-dom';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

// Mock LogViewer to avoid SSE complexity in tests
vi.mock('../../components/LogViewer', () => ({
  default: ({ taskName }: { taskName: string }) => (
    <div data-testid="log-viewer">LogViewer for {taskName}</div>
  ),
}));

// Mock YamlViewer to simplify tests
vi.mock('../../components/YamlViewer', () => ({
  default: () => <div data-testid="yaml-viewer">YamlViewer</div>,
}));

function renderTaskDetailPage(namespace: string, name: string) {
  return renderWithProviders(
    <Routes>
      <Route path="/tasks/:namespace/:name" element={<TaskDetailPage />} />
    </Routes>,
    { initialEntries: [`/tasks/${namespace}/${name}`] }
  );
}

describe('TaskDetailPage', () => {
  it('renders task details from API', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'fix-auth-bug' })).toBeInTheDocument();
    });

    // Namespace is displayed on the page
    const namespaceElements = screen.getAllByText('default');
    expect(namespaceElements.length).toBeGreaterThan(0);
  });

  it('shows agent reference as link', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      const agentLink = screen.getByText('opencode-agent');
      expect(agentLink.closest('a')).toHaveAttribute('href', '/agents/default/opencode-agent');
    });
  });

  it('shows duration', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByText('5m 30s')).toBeInTheDocument();
    });
  });

  it('shows pod name for running tasks', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByText('default/fix-auth-bug-pod')).toBeInTheDocument();
    });
  });

  it('shows labels when present', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByText('auth-service')).toBeInTheDocument();
      expect(screen.getByText('backend')).toBeInTheDocument();
    });
  });

  it('shows conditions when present', async () => {
    renderTaskDetailPage('default', 'add-user-profile');

    await waitFor(() => {
      expect(screen.getByText('Ready')).toBeInTheDocument();
      expect(screen.getByText('True')).toBeInTheDocument();
    });
  });

  it('shows description when present', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByText('Fix authentication bug in login flow causing 401 errors for OAuth users')).toBeInTheDocument();
    });
  });

  it('renders LogViewer for running tasks', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByTestId('log-viewer')).toBeInTheDocument();
    });
  });

  it('renders YamlViewer', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByTestId('yaml-viewer')).toBeInTheDocument();
    });
  });

  it('shows Stop button for running tasks', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Stop' })).toBeInTheDocument();
    });
  });

  it('does not show Stop button for completed tasks', async () => {
    renderTaskDetailPage('default', 'add-user-profile');

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'add-user-profile' })).toBeInTheDocument();
    });

    expect(screen.queryByRole('button', { name: 'Stop' })).not.toBeInTheDocument();
  });

  it('shows Delete button', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
    });
  });

  it('shows Rerun link', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      const rerunLink = screen.getByText('Rerun');
      expect(rerunLink.closest('a')).toHaveAttribute(
        'href',
        '/tasks/create?rerun=fix-auth-bug&namespace=default'
      );
    });
  });

  it('opens confirm dialog when Delete is clicked', async () => {
    const user = userEvent.setup();
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Delete' }));

    // Confirm dialog should appear
    expect(screen.getByText('Delete Task')).toBeInTheDocument();
    expect(screen.getByText(/Are you sure you want to delete task/)).toBeInTheDocument();
  });

  it('calls delete API when confirmed', async () => {
    const user = userEvent.setup();
    let deleteCalled = false;

    server.use(
      http.delete('/api/v1/namespaces/default/tasks/fix-auth-bug', () => {
        deleteCalled = true;
        return new HttpResponse(null, { status: 204 });
      })
    );

    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
    });

    // Click Delete button to open dialog
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    // Find the confirm "Delete" button inside the dialog
    const dialog = screen.getByText('Delete Task').closest('.relative.bg-white')!;
    const confirmButton = dialog.querySelector('button.bg-red-600') ||
      Array.from(dialog.querySelectorAll('button')).find(
        (btn) => btn.className.includes('bg-red')
      );

    if (confirmButton) {
      await user.click(confirmButton);
    }

    await waitFor(() => {
      expect(deleteCalled).toBe(true);
    });
  });

  it('shows error state when task is not found', async () => {
    renderTaskDetailPage('default', 'nonexistent-task');

    await waitFor(() => {
      expect(screen.getByText(/not found/i)).toBeInTheDocument();
    });
  });

  it('shows breadcrumbs navigation', async () => {
    renderTaskDetailPage('default', 'fix-auth-bug');

    await waitFor(() => {
      const breadcrumbNav = screen.getByLabelText('Breadcrumb');
      expect(breadcrumbNav).toBeInTheDocument();
      expect(breadcrumbNav.textContent).toContain('Tasks');
    });
  });
});
