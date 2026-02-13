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
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      // Use heading role to disambiguate from breadcrumb
      expect(screen.getByRole('heading', { name: 'fix-bug-123' })).toBeInTheDocument();
    });

    // Namespace shown below heading
    const heading = screen.getByRole('heading', { name: 'fix-bug-123' });
    const headerSection = heading.closest('div')!;
    expect(headerSection.textContent).toContain('default');
  });

  it('shows agent reference as link', async () => {
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      const agentLink = screen.getByText('opencode-agent');
      expect(agentLink.closest('a')).toHaveAttribute('href', '/agents/default/opencode-agent');
    });
  });

  it('shows duration', async () => {
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      expect(screen.getByText('5m')).toBeInTheDocument();
    });
  });

  it('shows pod name for running tasks', async () => {
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      expect(screen.getByText('default/fix-bug-123-pod')).toBeInTheDocument();
    });
  });

  it('shows labels when present', async () => {
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      expect(screen.getByText('myapp')).toBeInTheDocument();
      expect(screen.getByText('backend')).toBeInTheDocument();
    });
  });

  it('shows conditions when present', async () => {
    renderTaskDetailPage('default', 'add-feature-456');

    await waitFor(() => {
      expect(screen.getByText('Ready')).toBeInTheDocument();
      expect(screen.getByText('True')).toBeInTheDocument();
    });
  });

  it('shows description when present', async () => {
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      expect(screen.getByText('Fix authentication bug in login flow')).toBeInTheDocument();
    });
  });

  it('renders LogViewer for running tasks', async () => {
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      expect(screen.getByTestId('log-viewer')).toBeInTheDocument();
    });
  });

  it('renders YamlViewer', async () => {
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      expect(screen.getByTestId('yaml-viewer')).toBeInTheDocument();
    });
  });

  it('shows Stop button for running tasks', async () => {
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Stop' })).toBeInTheDocument();
    });
  });

  it('does not show Stop button for completed tasks', async () => {
    renderTaskDetailPage('default', 'add-feature-456');

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'add-feature-456' })).toBeInTheDocument();
    });

    expect(screen.queryByRole('button', { name: 'Stop' })).not.toBeInTheDocument();
  });

  it('shows Delete button', async () => {
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
    });
  });

  it('shows Rerun link', async () => {
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      const rerunLink = screen.getByText('Rerun');
      expect(rerunLink.closest('a')).toHaveAttribute(
        'href',
        '/tasks/create?namespace=default&rerun=fix-bug-123'
      );
    });
  });

  it('opens confirm dialog when Delete is clicked', async () => {
    const user = userEvent.setup();
    renderTaskDetailPage('default', 'fix-bug-123');

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
      http.delete('/api/v1/namespaces/default/tasks/fix-bug-123', () => {
        deleteCalled = true;
        return new HttpResponse(null, { status: 204 });
      })
    );

    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
    });

    // Click Delete button to open dialog
    await user.click(screen.getByRole('button', { name: 'Delete' }));

    // Find the confirm "Delete" button inside the dialog (it has role button too)
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
    renderTaskDetailPage('default', 'fix-bug-123');

    await waitFor(() => {
      // Breadcrumb has "Tasks" link
      const breadcrumbNav = screen.getByLabelText('Breadcrumb');
      expect(breadcrumbNav).toBeInTheDocument();
      expect(breadcrumbNav.textContent).toContain('Tasks');
    });
  });
});
