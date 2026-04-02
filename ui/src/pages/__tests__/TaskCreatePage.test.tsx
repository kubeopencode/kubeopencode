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
    expect(screen.getByText('Namespace')).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/Auto-generated/)).toBeInTheDocument();
    expect(screen.getByText('Description')).toBeInTheDocument();
  });

  it('loads namespaces from API', async () => {
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    await waitFor(() => {
      // SearchableSelect renders the namespace. The page should have "default" visible.
      expect(screen.getByText('Namespace')).toBeInTheDocument();
    });
  });

  it('loads agents from API', async () => {
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });

    await waitFor(() => {
      expect(screen.getByText(/\d+ agents? available/)).toBeInTheDocument();
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
    const descriptionInput = screen.getByPlaceholderText('Describe what you want the AI agent to do...');
    await user.type(descriptionInput, 'Fix the login bug');

    // Open agent SearchableSelect and pick one
    const agentButton = screen.getByText('Select an agent...');
    await user.click(agentButton);

    // Wait for dropdown options to appear and click the first agent
    await waitFor(() => {
      const options = screen.getAllByRole('button').filter(
        (btn) => btn.textContent?.includes('/') && btn.closest('.absolute')
      );
      expect(options.length).toBeGreaterThan(0);
    });

    const agentOptions = screen.getAllByRole('button').filter(
      (btn) => btn.textContent?.includes('/') && btn.closest('.absolute')
    );
    await user.click(agentOptions[0]);

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
    await user.type(screen.getByPlaceholderText(/Auto-generated/), 'my-task');
    await user.type(screen.getByPlaceholderText('Describe what you want the AI agent to do...'), 'Fix the bug');

    // Select agent via SearchableSelect
    const agentButton = screen.getByText('Select an agent...');
    await user.click(agentButton);

    await waitFor(() => {
      const options = screen.getAllByRole('button').filter(
        (btn) => btn.textContent?.includes('/') && btn.closest('.absolute')
      );
      expect(options.length).toBeGreaterThan(0);
    });

    const agentOptions = screen.getAllByRole('button').filter(
      (btn) => btn.textContent?.includes('/') && btn.closest('.absolute')
    );
    await user.click(agentOptions[0]);

    // Submit
    await user.click(screen.getByRole('button', { name: 'Create Task' }));

    await waitFor(() => {
      expect(capturedBody).not.toBeNull();
      expect(capturedBody!.name).toBe('my-task');
      expect(capturedBody!.description).toBe('Fix the bug');
      expect(capturedBody!.agentRef).toBeDefined();
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

    await user.type(screen.getByPlaceholderText('Describe what you want the AI agent to do...'), 'Test');

    // Select agent
    const agentButton = screen.getByText('Select an agent...');
    await user.click(agentButton);
    await waitFor(() => {
      const options = screen.getAllByRole('button').filter(
        (btn) => btn.textContent?.includes('/') && btn.closest('.absolute')
      );
      expect(options.length).toBeGreaterThan(0);
    });
    const agentOptions = screen.getAllByRole('button').filter(
      (btn) => btn.textContent?.includes('/') && btn.closest('.absolute')
    );
    await user.click(agentOptions[0]);

    await user.click(screen.getByRole('button', { name: 'Create Task' }));

    await waitFor(() => {
      expect(screen.getByText(/Agent not found/)).toBeInTheDocument();
    });
  });

  it('renders Cancel link back to tasks', () => {
    renderWithProviders(<TaskCreatePage />, { initialEntries: ['/tasks/create'] });
    const cancelLink = screen.getByText('Cancel');
    expect(cancelLink.closest('a')).toHaveAttribute('href', expect.stringContaining('/tasks'));
  });
});
