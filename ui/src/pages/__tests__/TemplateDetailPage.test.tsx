import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../../test/utils';
import TemplateDetailPage from '../TemplateDetailPage';
import { Route, Routes } from 'react-router-dom';

vi.mock('../../components/YamlViewer', () => ({
  default: () => <div data-testid="yaml-viewer">YamlViewer</div>,
}));

function renderTemplateDetailPage(namespace: string, name: string) {
  return renderWithProviders(
    <Routes>
      <Route path="/templates/:namespace/:name" element={<TemplateDetailPage />} />
    </Routes>,
    { initialEntries: [`/templates/${namespace}/${name}`] }
  );
}

describe('TemplateDetailPage', () => {
  it('renders template name and namespace', async () => {
    renderTemplateDetailPage('default', 'pr-template');

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'pr-template' })).toBeInTheDocument();
    });
  });

  it('shows template description', async () => {
    renderTemplateDetailPage('default', 'pr-template');

    await waitFor(() => {
      expect(screen.getByText('Create a pull request following coding standards')).toBeInTheDocument();
    });
  });

  it('shows agent reference as link', async () => {
    renderTemplateDetailPage('default', 'pr-template');

    await waitFor(() => {
      const link = screen.getByText('default/opencode-agent');
      expect(link.closest('a')).toHaveAttribute('href', '/agents/default/opencode-agent');
    });
  });

  it('shows contexts section', async () => {
    renderTemplateDetailPage('default', 'pr-template');

    await waitFor(() => {
      expect(screen.getByText('Contexts (1)')).toBeInTheDocument();
      expect(screen.getByText('source')).toBeInTheDocument();
      expect(screen.getByText('Git')).toBeInTheDocument();
    });
  });

  it('shows context mount path', async () => {
    renderTemplateDetailPage('default', 'pr-template');

    await waitFor(() => {
      expect(screen.getByText('Mount: source-code')).toBeInTheDocument();
    });
  });

  it('shows "Create Task from Template" link', async () => {
    renderTemplateDetailPage('default', 'pr-template');

    await waitFor(() => {
      const link = screen.getByText('Create Task from Template');
      expect(link.closest('a')).toHaveAttribute(
        'href',
        '/tasks/create?template=default/pr-template'
      );
    });
  });

  it('renders YamlViewer', async () => {
    renderTemplateDetailPage('default', 'pr-template');

    await waitFor(() => {
      expect(screen.getByTestId('yaml-viewer')).toBeInTheDocument();
    });
  });

  it('shows breadcrumbs', async () => {
    renderTemplateDetailPage('default', 'pr-template');

    await waitFor(() => {
      const breadcrumb = screen.getByLabelText('Breadcrumb');
      expect(breadcrumb.textContent).toContain('Templates');
      expect(breadcrumb.textContent).toContain('default');
      expect(breadcrumb.textContent).toContain('pr-template');
    });
  });

  it('shows error state for nonexistent template', async () => {
    renderTemplateDetailPage('default', 'nonexistent-template');

    await waitFor(() => {
      expect(screen.getByText(/not found/i)).toBeInTheDocument();
    });
  });

  it('shows back link in error state', async () => {
    renderTemplateDetailPage('default', 'nonexistent-template');

    await waitFor(() => {
      const backLink = screen.getByText(/Back to Templates/);
      expect(backLink.closest('a')).toHaveAttribute('href', '/templates');
    });
  });
});
