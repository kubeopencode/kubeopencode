import React, { useState, useCallback, useEffect } from 'react';
import type { FilterState } from '../hooks/useFilterState';

interface ResourceFilterProps {
  onFilterChange: (filters: FilterState) => void;
  filters: FilterState;
  placeholder?: string;
  children?: React.ReactNode;
}

function ResourceFilter({ onFilterChange, filters, placeholder, children }: ResourceFilterProps) {
  const [name, setName] = useState(filters.name);
  const [labelSelector, setLabelSelector] = useState(filters.labelSelector);

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
    <div className="bg-white rounded-xl border-0 shadow-card overflow-visible">
      <div className="flex items-center gap-2 p-3">
        <div className="relative flex-1 min-w-[180px]">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-stone-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={placeholder || 'Filter by name...'}
            className="block w-full pl-9 pr-3 py-2 rounded-lg border border-stone-200 bg-stone-50 focus:bg-white focus:border-primary-400 focus:ring-1 focus:ring-primary-200 text-sm text-stone-700 placeholder:text-stone-400 transition-colors"
          />
        </div>

        <div className="relative flex-1 min-w-[180px]">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-stone-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M20.59 13.41l-7.17 7.17a2 2 0 01-2.83 0L2 12V2h10l8.59 8.59a2 2 0 010 2.82z" />
            <line x1="7" y1="7" x2="7.01" y2="7" />
          </svg>
          <input
            type="text"
            value={labelSelector}
            onChange={(e) => setLabelSelector(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Label selector (e.g. app=myapp)"
            className="block w-full pl-9 pr-3 py-2 rounded-lg border border-stone-200 bg-stone-50 focus:bg-white focus:border-primary-400 focus:ring-1 focus:ring-primary-200 text-sm text-stone-700 placeholder:text-stone-400 transition-colors"
          />
        </div>

        <button
          onClick={handleApply}
          className="px-4 py-2 text-sm font-medium text-white bg-stone-800 rounded-lg hover:bg-stone-700 active:bg-stone-900 transition-colors shadow-sm"
        >
          Apply
        </button>

        {hasFilters && (
          <button
            onClick={handleClear}
            className="flex items-center gap-1 px-3 py-2 text-sm text-stone-500 hover:text-stone-700 hover:bg-stone-50 rounded-lg transition-colors"
          >
            <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
            Clear
          </button>
        )}
      </div>

      {children && (
        <div className="flex items-center gap-2 flex-wrap px-3 pb-3 pt-2.5 border-t border-stone-100">
          {children}
        </div>
      )}
    </div>
  );
}

export default ResourceFilter;
