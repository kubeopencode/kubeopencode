import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { useFilterState } from '../useFilterState';

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>;
}

function wrapperWithParams(search: string) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <MemoryRouter initialEntries={[`/?${search}`]}>{children}</MemoryRouter>;
  };
}

describe('useFilterState', () => {
  it('returns empty filters by default', () => {
    const { result } = renderHook(() => useFilterState(), { wrapper });
    const [filters] = result.current;
    expect(filters.name).toBe('');
    expect(filters.labelSelector).toBe('');
  });

  it('reads initial state from URL search params', () => {
    const { result } = renderHook(() => useFilterState(), {
      wrapper: wrapperWithParams('name=my-task&labels=app%3Dmyapp'),
    });
    const [filters] = result.current;
    expect(filters.name).toBe('my-task');
    expect(filters.labelSelector).toBe('app=myapp');
  });

  it('updates URL when setFilters is called with values', () => {
    const { result } = renderHook(() => useFilterState(), { wrapper });

    act(() => {
      const [, setFilters] = result.current;
      setFilters({ name: 'test-task', labelSelector: 'env=prod' });
    });

    const [filters] = result.current;
    expect(filters.name).toBe('test-task');
    expect(filters.labelSelector).toBe('env=prod');
  });

  it('removes params when values are empty', () => {
    const { result } = renderHook(() => useFilterState(), {
      wrapper: wrapperWithParams('name=old-value&labels=old-label'),
    });

    act(() => {
      const [, setFilters] = result.current;
      setFilters({ name: '', labelSelector: '' });
    });

    const [filters] = result.current;
    expect(filters.name).toBe('');
    expect(filters.labelSelector).toBe('');
  });
});
