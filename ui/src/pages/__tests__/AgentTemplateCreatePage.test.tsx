import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '../../test/utils';
import AgentTemplateCreatePage from '../AgentTemplateCreatePage';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('AgentTemplateCreatePage', () => {
  beforeEach(() => {
    // Clear cookies
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title', () => {
    renderWithProviders(<AgentTemplateCreatePage />, { initialEntries: ['/templates/create'] });

    expect(screen.getByRole('heading', { name: 'Create Agent Template' })).toBeInTheDocument();
  });

  it('renders form fields', () => {
    renderWithProviders(<AgentTemplateCreatePage />, { initialEntries: ['/templates/create'] });

    expect(screen.getByText('Name')).toBeInTheDocument();
    expect(screen.getByText(/Workspace Directory/)).toBeInTheDocument();
  });

  it('renders cancel and submit buttons', () => {
    renderWithProviders(<AgentTemplateCreatePage />, { initialEntries: ['/templates/create'] });

    const cancelLink = screen.getByText('Cancel');
    expect(cancelLink.closest('a')).toHaveAttribute('href', '/templates');

    const submitButton = screen.getByRole('button', { name: 'Create Template' });
    expect(submitButton).toBeInTheDocument();
  });
});
