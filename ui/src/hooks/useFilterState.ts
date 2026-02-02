import { useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';

export interface FilterState {
  name: string;
  labelSelector: string;
}

/**
 * Custom hook for URL-based filter state persistence.
 * Reads and writes filter state to URL search params.
 */
export function useFilterState(): [FilterState, (filters: FilterState) => void] {
  const [searchParams, setSearchParams] = useSearchParams();

  const filters: FilterState = {
    name: searchParams.get('name') || '',
    labelSelector: searchParams.get('labels') || '',
  };

  const setFilters = useCallback(
    (newFilters: FilterState) => {
      const params = new URLSearchParams(searchParams);

      if (newFilters.name) {
        params.set('name', newFilters.name);
      } else {
        params.delete('name');
      }

      if (newFilters.labelSelector) {
        params.set('labels', newFilters.labelSelector);
      } else {
        params.delete('labels');
      }

      setSearchParams(params, { replace: true });
    },
    [searchParams, setSearchParams]
  );

  return [filters, setFilters];
}

export default useFilterState;
