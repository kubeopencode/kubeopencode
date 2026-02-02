import { useState, useCallback, useEffect } from 'react';
import type { FilterState } from '../hooks/useFilterState';

interface ResourceFilterProps {
  onFilterChange: (filters: FilterState) => void;
  filters: FilterState;
  placeholder?: string;
}

function ResourceFilter({ onFilterChange, filters, placeholder }: ResourceFilterProps) {
  const [name, setName] = useState(filters.name);
  const [labelSelector, setLabelSelector] = useState(filters.labelSelector);

  // Sync local state with external filters
  useEffect(() => {
    setName(filters.name);
    setLabelSelector(filters.labelSelector);
  }, [filters.name, filters.labelSelector]);

  const handleApply = useCallback(() => {
    onFilterChange({ name, labelSelector });
  }, [name, labelSelector, onFilterChange]);

  const handleClear = useCallback(() => {
    setName('');
    setLabelSelector('');
    onFilterChange({ name: '', labelSelector: '' });
  }, [onFilterChange]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') {
        handleApply();
      }
    },
    [handleApply]
  );

  const hasFilters = name || labelSelector;

  return (
    <div className="flex items-center space-x-2 flex-wrap gap-y-2">
      {/* Name search input */}
      <div className="relative">
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder || 'Filter by name...'}
          className="block w-48 sm:w-64 rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm"
        />
      </div>

      {/* Label selector input */}
      <input
        type="text"
        value={labelSelector}
        onChange={(e) => setLabelSelector(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="Label filter (e.g. app=myapp)"
        className="block w-48 sm:w-56 rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm"
      />

      {/* Apply button */}
      <button
        onClick={handleApply}
        className="px-3 py-2 text-sm font-medium text-white bg-primary-600 rounded-md hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-500"
      >
        Filter
      </button>

      {/* Clear button */}
      {hasFilters && (
        <button
          onClick={handleClear}
          className="text-sm text-gray-500 hover:text-gray-700"
        >
          Clear
        </button>
      )}
    </div>
  );
}

export default ResourceFilter;
