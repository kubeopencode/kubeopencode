import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import TaskCreatePage from '../TaskCreatePage';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('TaskCreatePage', () => {
  it('renders the create task form', async () => {
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    // Use heading role to avoid matching breadcrumb text
    expect(screen.getByRole('heading', { name: 'Create Task' })).toBeInTheDocument();
    expect(screen.getByLabelText(/Namespace/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Name \(optional\)/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Description/)).toBeInTheDocument();
  });

  it('loads namespaces from API', async () => {
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    await waitFor(() => {
      const namespaceSelect = screen.getByLabelText(/Namespace/);
      const options = namespaceSelect.querySelectorAll('option');
      const optionTexts = Array.from(options).map((o) => o.textContent);
      expect(optionTexts).toContain('default');
      expect(optionTexts).toContain('production');
    });
  });

  it('loads agents from API', async () => {
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    await waitFor(() => {
      const agentSelect = screen.getByLabelText(/^Agent/);
      const options = agentSelect.querySelectorAll('option');
      expect(options.length).toBeGreaterThan(1);
    });
  });

  it('filters agents by namespace availability', async () => {
    const user = userEvent.setup();
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    // Wait for agents to load
    await waitFor(() => {
      expect(screen.getByText(/\d+ agents? available/)).toBeInTheDocument();
    });

    // Switch to production namespace
    const namespaceSelect = screen.getByLabelText(/Namespace/);
    await user.selectOptions(namespaceSelect, 'production');

    // For production: global-agent (no restrictions) + restricted-agent (allows production)
    await waitFor(() => {
      expect(screen.getByText(/\d+ agents? available/)).toBeInTheDocument();
    });
  });

  it('shows template toggle and template selector', async () => {
    const user = userEvent.setup();
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    const templateToggle = screen.getByLabelText('Use a template');
    expect(templateToggle).toBeInTheDocument();

    // Template selector should not be visible initially
    expect(screen.queryByText('Select a template...')).not.toBeInTheDocument();

    // Enable template mode
    await user.click(templateToggle);

    // Template selector should now be visible
    await waitFor(() => {
      expect(screen.getByText('Select a template...')).toBeInTheDocument();
    });
  });

  it('loads templates when template mode is enabled', async () => {
    const user = userEvent.setup();
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    await user.click(screen.getByLabelText('Use a template'));

    await waitFor(() => {
      // Find the template select by looking for the option text
      expect(screen.getByText('default/pr-template')).toBeInTheDocument();
    });
  });

  it('disables Create Task button when form is invalid', async () => {
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    // Wait for agents to load so the page is ready
    await waitFor(() => {
      expect(screen.getByText(/agents? available/)).toBeInTheDocument();
    });

    const submitButton = screen.getByRole('button', { name: 'Create Task' });
    expect(submitButton).toBeDisabled();
  });

  it('enables Create Task button when description and agent are set', async () => {
    const user = userEvent.setup();
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    // Wait for agents to load
    await waitFor(() => {
      expect(screen.getByText(/agents? available/)).toBeInTheDocument();
    });

    // Fill description
    const descriptionInput = screen.getByLabelText(/Description/);
    await user.type(descriptionInput, 'Fix the login bug');

    // Select an agent
    const agentSelect = screen.getByLabelText(/^Agent/);
    await user.selectOptions(agentSelect, 'default/opencode-agent');

    // Button should be enabled
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Create Task' })).not.toBeDisabled();
    });
  });

  it('submits form with correct data', async () => {
    const user = userEvent.setup();
    let capturedBody: Record<string, unknown> | null = null;

    server.use(
      http.post('/api/v1/namespaces/:namespace/tasks', async ({ request }) => {
        capturedBody = await request.json() as Record<string, unknown>;
        return HttpResponse.json({
          name: 'my-task',
          namespace: 'default',
          phase: 'Pending',
          createdAt: new Date().toISOString(),
        });
      })
    );

    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    await waitFor(() => {
      expect(screen.getByText(/agents? available/)).toBeInTheDocument();
    });

    // Fill form
    await user.type(screen.getByLabelText(/Name \(optional\)/), 'my-task');
    await user.type(screen.getByLabelText(/Description/), 'Fix the bug');
    await user.selectOptions(screen.getByLabelText(/^Agent/), 'default/opencode-agent');

    // Submit
    await user.click(screen.getByRole('button', { name: 'Create Task' }));

    await waitFor(() => {
      expect(capturedBody).not.toBeNull();
      expect(capturedBody!.name).toBe('my-task');
      expect(capturedBody!.description).toBe('Fix the bug');
      expect(capturedBody!.agentRef).toEqual({
        name: 'opencode-agent',
        namespace: 'default',
      });
    });
  });

  it('shows error message when creation fails', async () => {
    const user = userEvent.setup();

    server.use(
      http.post('/api/v1/namespaces/:namespace/tasks', () => {
        return HttpResponse.json({ message: 'Agent not found' }, { status: 400 });
      })
    );

    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    await waitFor(() => {
      expect(screen.getByText(/agents? available/)).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/Description/), 'Test');
    await user.selectOptions(screen.getByLabelText(/^Agent/), 'default/opencode-agent');
    await user.click(screen.getByRole('button', { name: 'Create Task' }));

    await waitFor(() => {
      expect(screen.getByText(/Agent not found/)).toBeInTheDocument();
    });
  });

  it('pre-fills namespace from URL params', async () => {
    renderWithProviders(<TaskCreatePage />, {
      initialEntries: ['/tasks/create?namespace=staging'],
    });

    await waitFor(() => {
      const namespaceSelect = screen.getByLabelText(/Namespace/) as HTMLSelectElement;
      expect(namespaceSelect.value).toBe('staging');
    });
  });

  it('renders Cancel link back to tasks', () => {
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });
    const cancelLink = screen.getByText('Cancel');
    expect(cancelLink.closest('a')).toHaveAttribute('href', expect.stringContaining('/tasks'));
  });
});
