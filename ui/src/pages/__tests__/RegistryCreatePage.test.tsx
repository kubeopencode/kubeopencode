import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../../test/utils';
import RegistryCreatePage from '../RegistryCreatePage';

// Mock TimeAgo to avoid timing issues
vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('RegistryCreatePage', () => {
  beforeEach(() => {
    // Clear cookies
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title', () => {
    renderWithProviders(<RegistryCreatePage />, { initialEntries: ['/registries/create'] });

    expect(screen.getByRole('heading', { name: 'Create Registry' })).toBeInTheDocument();
  });

  it('renders name input', async () => {
    renderWithProviders(<RegistryCreatePage />, { initialEntries: ['/registries/create'] });

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/my-registry/i)).toBeInTheDocument();
    });
  });

  it('renders cancel and submit buttons', () => {
    renderWithProviders(<RegistryCreatePage />, { initialEntries: ['/registries/create'] });

    const cancelLink = screen.getByText('Cancel');
    expect(cancelLink.closest('a')).toHaveAttribute('href', '/registries');

    const submitButton = screen.getByRole('button', { name: 'Create Registry' });
    expect(submitButton).toBeInTheDocument();
  });
});
