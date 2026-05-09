import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/utils';
import AgentDetailPage from '../AgentDetailPage';
import { Route, Routes } from 'react-router-dom';

vi.mock('../../components/YamlViewer', () => ({
  default: () => <div data-testid="yaml-viewer">YamlViewer</div>,
}));

vi.mock('../../components/TerminalPanel', () => ({
  default: () => <div data-testid="terminal-panel">TerminalPanel</div>,
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

  it('shows live status badge for agents', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Live')).toBeInTheDocument();
    });
  });

  it('shows agent profile when available', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText(/Full-stack development agent/)).toBeInTheDocument();
    });
  });

  it('shows executor and agent images', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Executor Image')).toBeInTheDocument();
      expect(screen.getByText('ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest')).toBeInTheDocument();
    });

    expect(screen.getByText('Agent Image')).toBeInTheDocument();
    expect(screen.getByText('ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest')).toBeInTheDocument();
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

  it('shows credentials section', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Credentials (2)')).toBeInTheDocument();
      expect(screen.getByText('github-token')).toBeInTheDocument();
      expect(screen.getByText(/github-creds/)).toBeInTheDocument();
    });
  });

  it('shows contexts section', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Contexts (3)')).toBeInTheDocument();
      expect(screen.getByText('coding-standards')).toBeInTheDocument();
      expect(screen.getByText('source')).toBeInTheDocument();
    });
  });

  it('shows server status for all agents', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Server Status')).toBeInTheDocument();
      expect(screen.getByText('opencode-agent-server')).toBeInTheDocument();
      expect(screen.getByText('http://opencode-agent.default.svc.cluster.local:4096')).toBeInTheDocument();
    });
  });

  it('shows conditions for agents with conditions', async () => {
    renderAgentDetailPage('test', 'server-agent');

    await waitFor(() => {
      expect(screen.getByText('Conditions')).toBeInTheDocument();
      expect(screen.getByText('ServerReady')).toBeInTheDocument();
      expect(screen.getByText('ServerHealthy')).toBeInTheDocument();
    });
  });

  it('shows "Create Task" button', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      const link = screen.getByText('Create Task');
      expect(link.closest('a')).toHaveAttribute(
        'href',
        '/tasks/create?agent=default/opencode-agent'
      );
    });
  });

  it('renders YamlViewer', async () => {
    const user = userEvent.setup();
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'opencode-agent' })).toBeInTheDocument();
    });

    // Click the YAML tab
    await user.click(screen.getByRole('button', { name: /YAML/i }));

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

  it('shows plugins section when agent has plugins', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('Plugins (2)')).toBeInTheDocument();
      expect(screen.getByText('@kubeopencode/opencode-slack-plugin')).toBeInTheDocument();
      expect(screen.getByText('opencode-plugin-otel')).toBeInTheDocument();
    });

    // Check target badges are rendered
    const serverBadges = screen.getAllByText('server');
    expect(serverBadges.length).toBeGreaterThanOrEqual(2);
  });

  it('shows opencode config section when agent has config', async () => {
    renderAgentDetailPage('default', 'opencode-agent');

    await waitFor(() => {
      expect(screen.getByText('OpenCode Config')).toBeInTheDocument();
      expect(screen.getByText(/anthropic\/claude-sonnet/)).toBeInTheDocument();
    });
  });

  it('shows error state for nonexistent agent', async () => {
    renderAgentDetailPage('default', 'nonexistent-agent');

    await waitFor(() => {
      expect(screen.getByText(/not found/i)).toBeInTheDocument();
    });
  });
});
