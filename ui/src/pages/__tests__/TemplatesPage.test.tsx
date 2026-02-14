import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test/utils';
import TemplatesPage from '../TemplatesPage';

vi.mock('../../components/TimeAgo', () => ({
  default: ({ date }: { date: string }) => <span>{date}</span>,
}));

describe('TemplatesPage', () => {
  beforeEach(() => {
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  it('renders page title and description', () => {
    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });
    expect(screen.getByText('Templates')).toBeInTheDocument();
    expect(screen.getByText('Reusable task templates for common workflows')).toBeInTheDocument();
  });

  it('renders template cards from API', async () => {
    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      expect(screen.getByText('pr-template')).toBeInTheDocument();
    });

    expect(screen.getByText('review-template')).toBeInTheDocument();
  });

  it('shows template descriptions', async () => {
    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      expect(screen.getByText('Create a pull request following coding standards')).toBeInTheDocument();
      expect(screen.getByText('Review code changes')).toBeInTheDocument();
    });
  });

  it('shows contexts count badge when contexts exist', async () => {
    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      expect(screen.getByText('1 context')).toBeInTheDocument();
    });
  });

  it('shows agent reference on template cards', async () => {
    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      // pr-template has agent ref
      expect(screen.getByText('default/opencode-agent')).toBeInTheDocument();
    });
  });

  it('renders template cards as links to detail pages', async () => {
    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      const link = screen.getByText('pr-template').closest('a');
      expect(link).toHaveAttribute('href', '/templates/default/pr-template');
    });
  });

  it('renders namespace selector', async () => {
    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      const options = screen.getAllByRole('option');
      const optionTexts = options.map((o) => o.textContent);
      expect(optionTexts).toContain('All Namespaces');
    });
  });

  it('renders filter component', () => {
    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });
    expect(screen.getByPlaceholderText('Filter templates by name...')).toBeInTheDocument();
  });

  it('shows error state when API fails', async () => {
    server.use(
      http.get('/api/v1/tasktemplates', () => {
        return HttpResponse.json({ message: 'Internal error' }, { status: 500 });
      })
    );

    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      expect(screen.getByText(/Error loading templates/)).toBeInTheDocument();
    });

    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows empty state when no templates found', async () => {
    server.use(
      http.get('/api/v1/tasktemplates', () => {
        return HttpResponse.json({
          templates: [],
          total: 0,
          pagination: { limit: 12, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      expect(screen.getByText(/No templates found/)).toBeInTheDocument();
    });
  });

  it('filters by namespace when namespace is changed', async () => {
    const user = userEvent.setup();
    let lastRequestUrl = '';

    server.use(
      http.get('/api/v1/namespaces/:namespace/tasktemplates', ({ request }) => {
        lastRequestUrl = request.url;
        return HttpResponse.json({
          templates: [],
          total: 0,
          pagination: { limit: 12, offset: 0, totalCount: 0, hasMore: false },
        });
      })
    );

    renderWithProviders(<TemplatesPage />, { initialEntries: ['/templates'] });

    await waitFor(() => {
      expect(screen.getByText('pr-template')).toBeInTheDocument();
    });

    const select = screen.getAllByRole('combobox')[0];
    await user.selectOptions(select, 'default');

    await waitFor(() => {
      expect(lastRequestUrl).toContain('/namespaces/default/tasktemplates');
    });
  });
});
