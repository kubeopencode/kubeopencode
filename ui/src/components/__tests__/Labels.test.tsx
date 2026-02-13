import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import Labels from '../Labels';

describe('Labels', () => {
  it('renders nothing when labels is undefined', () => {
    const { container } = render(<Labels />);
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing when labels is empty', () => {
    const { container } = render(<Labels labels={{}} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders all labels when no maxDisplay is set', () => {
    const labels = { app: 'myapp', env: 'prod', team: 'backend' };
    render(<Labels labels={labels} />);

    expect(screen.getByText('myapp')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();
    expect(screen.getByText('backend')).toBeInTheDocument();
  });

  it('renders key=value format with key in separate span', () => {
    render(<Labels labels={{ app: 'myapp' }} />);
    expect(screen.getByText('app=')).toBeInTheDocument();
    expect(screen.getByText('myapp')).toBeInTheDocument();
  });

  it('sets title attribute with key=value', () => {
    render(<Labels labels={{ app: 'myapp' }} />);
    const label = screen.getByTitle('app=myapp');
    expect(label).toBeInTheDocument();
  });

  it('limits displayed labels with maxDisplay', () => {
    const labels = { a: '1', b: '2', c: '3', d: '4' };
    render(<Labels labels={labels} maxDisplay={2} />);

    expect(screen.getByText('1')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.queryByText('3')).not.toBeInTheDocument();
    expect(screen.queryByText('4')).not.toBeInTheDocument();
  });

  it('shows "+N more" text when labels exceed maxDisplay', () => {
    const labels = { a: '1', b: '2', c: '3', d: '4' };
    render(<Labels labels={labels} maxDisplay={2} />);
    expect(screen.getByText('+2 more')).toBeInTheDocument();
  });

  it('does not show "+N more" when all labels fit', () => {
    const labels = { a: '1', b: '2' };
    render(<Labels labels={labels} maxDisplay={5} />);
    expect(screen.queryByText(/more/)).not.toBeInTheDocument();
  });
});
