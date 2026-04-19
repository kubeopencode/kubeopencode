import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '../../test/utils';
import AgentCreatePage from '../AgentCreatePage';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('AgentCreatePage', () => {
  beforeEach(() => {
    // Clear cookies
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title', () => {
    renderWithProviders(<AgentCreatePage />, { initialEntries: ['/agents/create'] });

    expect(screen.getByRole('heading', { name: 'Create Agent' })).toBeInTheDocument();
  });

  it('renders form fields', () => {
    renderWithProviders(<AgentCreatePage />, { initialEntries: ['/agents/create'] });

    expect(screen.getByText('Name')).toBeInTheDocument();
  });

  it('renders cancel and submit buttons', () => {
    renderWithProviders(<AgentCreatePage />, { initialEntries: ['/agents/create'] });

    const cancelLink = screen.getByText('Cancel');
    expect(cancelLink.closest('a')).toHaveAttribute('href', '/agents');

    const submitButton = screen.getByRole('button', { name: 'Create Agent' });
    expect(submitButton).toBeInTheDocument();
  });
});
