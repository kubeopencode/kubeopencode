import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test/utils';
import YamlViewer from '../YamlViewer';

const sampleYaml = `apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: test-task
  namespace: default`;

describe('YamlViewer', () => {
  it('renders collapsed by default', () => {
    renderWithProviders(
      <YamlViewer queryKey={['test']} fetchYaml={() => Promise.resolve(sampleYaml)} />
    );

    expect(screen.getByText('YAML')).toBeInTheDocument();
    expect(screen.queryByText('Resource Definition')).not.toBeInTheDocument();
  });

  it('expands when YAML button is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <YamlViewer queryKey={['test']} fetchYaml={() => Promise.resolve(sampleYaml)} />
    );

    await user.click(screen.getByText('YAML'));

    await waitFor(() => {
      expect(screen.getByText('Resource Definition')).toBeInTheDocument();
    });
  });

  it('shows YAML content after expanding', async () => {
    const user = userEvent.setup();
    const { container } = renderWithProviders(
      <YamlViewer queryKey={['test']} fetchYaml={() => Promise.resolve(sampleYaml)} />
    );

    await user.click(screen.getByText('YAML'));

    await waitFor(() => {
      const pre = container.querySelector('pre');
      expect(pre).toBeInTheDocument();
      expect(pre!.textContent).toContain('apiVersion: kubeopencode.io/v1alpha1');
      expect(pre!.textContent).toContain('kind: Task');
    });
  });

  it('shows Copy button when YAML is loaded', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <YamlViewer queryKey={['test']} fetchYaml={() => Promise.resolve(sampleYaml)} />
    );

    await user.click(screen.getByText('YAML'));

    await waitFor(() => {
      expect(screen.getByText('Copy')).toBeInTheDocument();
    });
  });

  it('shows loading state while fetching', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <YamlViewer
        queryKey={['test']}
        fetchYaml={() => new Promise((resolve) => setTimeout(() => resolve(sampleYaml), 100))}
      />
    );

    await user.click(screen.getByText('YAML'));

    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('shows error state when fetch fails', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <YamlViewer
        queryKey={['test-error']}
        fetchYaml={() => Promise.reject(new Error('Network error'))}
      />
    );

    await user.click(screen.getByText('YAML'));

    await waitFor(() => {
      expect(screen.getByText(/Error: Network error/)).toBeInTheDocument();
    });
  });

  it('collapses when YAML button is clicked again', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <YamlViewer queryKey={['test']} fetchYaml={() => Promise.resolve(sampleYaml)} />
    );

    // Expand
    await user.click(screen.getByText('YAML'));
    await waitFor(() => {
      expect(screen.getByText('Resource Definition')).toBeInTheDocument();
    });

    // Collapse
    await user.click(screen.getByText('YAML'));
    expect(screen.queryByText('Resource Definition')).not.toBeInTheDocument();
  });
});
