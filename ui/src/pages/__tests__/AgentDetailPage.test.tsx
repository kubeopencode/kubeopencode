import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../../test/utils';
import AgentDetailPage from '../AgentDetailPage';
import { Route, Routes } from 'react-router-dom';

vi.mock('../../components/YamlViewer', () => ({
  default: () => <div data-testid="yaml-viewer">YamlViewer</div>,
}));

function renderAgentDetailPage(namespace: string, name: string) {
  return renderWithProviders(
    <Routes>
      <Route path="/agents/:namespace/:name" element={<AgentDetailPage />} />
    </Routes>,
    { initialEntries: [`/agents/${namespace}/${name}`] }
  );
}

describe('AgentDetailPage', () => {
  it('renders agent name and namespace', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'opencode-agent' })).toBeInTheDocument();
    });
  });

  it('shows Pod mode badge', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Pod Mode')).toBeInTheDocument();
    });
  });

  it('shows Server mode badge for server agents', async () => {
    renderAgentDetailPage('production', 'restricted-agent');

    await waitFor(() => {
      expect(screen.getByText('Server Mode')).toBeInTheDocument();
    });
  });

  it('shows executor and agent images', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Executor Image')).toBeInTheDocument();
      expect(screen.getByText('quay.io/kubeopencode/kubeopencode-agent-devbox:latest')).toBeInTheDocument();
    });

    expect(screen.getByText('Agent Image')).toBeInTheDocument();
    expect(screen.getByText('quay.io/kubeopencode/kubeopencode-agent-opencode:latest')).toBeInTheDocument();
  });

  it('shows workspace directory', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Workspace Directory')).toBeInTheDocument();
      expect(screen.getByText('/workspace')).toBeInTheDocument();
    });
  });

  it('shows max concurrent tasks', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Max Concurrent Tasks')).toBeInTheDocument();
      expect(screen.getByText('5')).toBeInTheDocument();
    });
  });

  it('shows labels section', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Labels')).toBeInTheDocument();
      expect(screen.getByText('platform')).toBeInTheDocument();
      expect(screen.getByText('core')).toBeInTheDocument();
    });
  });

  it('shows allowed namespaces', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Allowed Namespaces')).toBeInTheDocument();
    });
  });

  it('shows credentials section', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Credentials (1)')).toBeInTheDocument();
      expect(screen.getByText('github-token')).toBeInTheDocument();
      expect(screen.getByText(/github-creds/)).toBeInTheDocument();
    });
  });

  it('shows contexts section', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Contexts (2)')).toBeInTheDocument();
      expect(screen.getByText('coding-standards')).toBeInTheDocument();
      expect(screen.getByText('source')).toBeInTheDocument();
    });
  });

  it('shows server status for server mode agents', async () => {
    renderAgentDetailPage('production', 'restricted-agent');

    await waitFor(() => {
      expect(screen.getByText('Server Status')).toBeInTheDocument();
      expect(screen.getByText('restricted-agent-server')).toBeInTheDocument();
      expect(screen.getByText('http://restricted-agent.production.svc.cluster.local:4096')).toBeInTheDocument();
    });
  });

  it('shows conditions for agents with conditions', async () => {
    renderAgentDetailPage('production', 'restricted-agent');

    await waitFor(() => {
      expect(screen.getByText('Conditions')).toBeInTheDocument();
      expect(screen.getByText('ServerReady')).toBeInTheDocument();
      expect(screen.getByText('ServerHealthy')).toBeInTheDocument();
    });
  });

  it('shows "Create Task with this Agent" link', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      const link = screen.getByText('Create Task with this Agent');
      expect(link.closest('a')).toHaveAttribute(
        'href',
        '/tasks/create?agent=default/opencode-agent'
      );
    });
  });

  it('renders YamlViewer', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByTestId('yaml-viewer')).toBeInTheDocument();
    });
  });

  it('shows breadcrumbs', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      const breadcrumb = screen.getByLabelText('Breadcrumb');
      expect(breadcrumb.textContent).toContain('Agents');
      expect(breadcrumb.textContent).toContain('default');
      expect(breadcrumb.textContent).toContain('opencode-agent');
    });
  });

  it('shows error state for nonexistent agent', async () => {
    renderAgentDetailPage('default', 'nonexistent-agent');

    await waitFor(() => {
      expect(screen.getByText(/not found/i)).toBeInTheDocument();
    });
  });
});
