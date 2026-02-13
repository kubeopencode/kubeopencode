import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ResourceFilter from '../ResourceFilter';

describe('ResourceFilter', () => {
  const defaultProps = {
    filters: { name: '', labelSelector: '' },
    onFilterChange: vi.fn(),
  };

  it('renders name and label input fields', () => {
    render(<ResourceFilter {...defaultProps} />);
    expect(screen.getByPlaceholderText('Filter by name...')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Label filter (e.g. app=myapp)')).toBeInTheDocument();
  });

  it('uses custom placeholder', () => {
    render(<ResourceFilter {...defaultProps} placeholder="Search tasks..." />);
    expect(screen.getByPlaceholderText('Search tasks...')).toBeInTheDocument();
  });

  it('renders Filter button', () => {
    render(<ResourceFilter {...defaultProps} />);
    expect(screen.getByText('Filter')).toBeInTheDocument();
  });

  it('does not render Clear button when no filters are active', () => {
    render(<ResourceFilter {...defaultProps} />);
    expect(screen.queryByText('Clear')).not.toBeInTheDocument();
  });

  it('renders Clear button when filters have values', () => {
    render(<ResourceFilter {...defaultProps} filters={{ name: 'test', labelSelector: '' }} />);
    expect(screen.getByText('Clear')).toBeInTheDocument();
  });

  it('calls onFilterChange when Filter button is clicked', async () => {
    const onFilterChange = vi.fn();
    const user = userEvent.setup();
    render(<ResourceFilter {...defaultProps} onFilterChange={onFilterChange} />);

    const nameInput = screen.getByPlaceholderText('Filter by name...');
    await user.type(nameInput, 'my-task');
    await user.click(screen.getByText('Filter'));

    expect(onFilterChange).toHaveBeenCalledWith({
      name: 'my-task',
      labelSelector: '',
    });
  });

  it('calls onFilterChange on Enter key press', async () => {
    const onFilterChange = vi.fn();
    const user = userEvent.setup();
    render(<ResourceFilter {...defaultProps} onFilterChange={onFilterChange} />);

    const nameInput = screen.getByPlaceholderText('Filter by name...');
    await user.type(nameInput, 'search{Enter}');

    expect(onFilterChange).toHaveBeenCalledWith({
      name: 'search',
      labelSelector: '',
    });
  });

  it('clears filters when Clear is clicked', async () => {
    const onFilterChange = vi.fn();
    const user = userEvent.setup();
    render(
      <ResourceFilter
        filters={{ name: 'existing', labelSelector: 'app=test' }}
        onFilterChange={onFilterChange}
      />
    );

    await user.click(screen.getByText('Clear'));

    expect(onFilterChange).toHaveBeenCalledWith({
      name: '',
      labelSelector: '',
    });
  });

  it('syncs local state when external filters change', () => {
    const { rerender } = render(
      <ResourceFilter {...defaultProps} filters={{ name: 'initial', labelSelector: '' }} />
    );

    const nameInput = screen.getByPlaceholderText('Filter by name...') as HTMLInputElement;
    expect(nameInput.value).toBe('initial');

    rerender(
      <ResourceFilter {...defaultProps} filters={{ name: 'updated', labelSelector: '' }} />
    );
    expect(nameInput.value).toBe('updated');
  });
});
