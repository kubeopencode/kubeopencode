import React, { useState, useRef, useEffect, useMemo } from 'react';

interface NamespaceSelectorProps {
  value: string;
  onChange: (ns: string) => void;
  namespaces: string[];
  allNamespacesValue: string;
}

function NamespaceSelector({ value, onChange, namespaces, allNamespacesValue }: NamespaceSelectorProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const ref = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) {
      setSearch('');
      return;
    }
    // Focus search input when dropdown opens
    setTimeout(() => searchInputRef.current?.focus(), 0);
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    function handleEscape(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [open]);

  const filteredNamespaces = useMemo(() => {
    if (!search.trim()) return namespaces;
    const query = search.trim().toLowerCase();
    return namespaces.filter((ns) => ns.toLowerCase().includes(query));
  }, [namespaces, search]);

  const showAllNamespaces = !search.trim() || 'all namespaces'.includes(search.trim().toLowerCase());

  const displayValue = value === allNamespacesValue ? 'All Namespaces' : value;
  const isAll = value === allNamespacesValue;

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen(!open)}
        className={`
          flex items-center gap-2 pl-2.5 pr-2 py-1.5 rounded-lg border bg-white text-sm transition-all min-w-[180px]
          ${open
            ? 'border-primary-300 ring-2 ring-primary-100 shadow-sm'
            : 'border-stone-200 hover:border-stone-300 shadow-sm hover:shadow'
          }
        `}
      >
        <svg className="w-4 h-4 text-primary-500 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <polygon points="12 2 2 7 12 12 22 7 12 2" />
          <polyline points="2 17 12 22 22 17" />
          <polyline points="2 12 12 17 22 12" />
        </svg>
        <span className={`flex-1 text-left truncate ${isAll ? 'text-stone-500' : 'text-stone-800 font-medium font-mono text-xs'}`}>
          {displayValue}
        </span>
        <svg
          className={`w-3.5 h-3.5 text-stone-400 shrink-0 transition-transform duration-200 ${open ? 'rotate-180' : ''}`}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        >
          <path d="M6 9l6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>

      {open && (
        <div className="absolute top-full left-0 mt-1.5 min-w-[280px] bg-white border border-stone-200 rounded-lg shadow-lg z-[100] py-1 max-h-80 animate-fade-in">
          <div className="px-3 py-1.5 text-[10px] font-display font-medium text-stone-400 uppercase tracking-wider">
            Namespace
          </div>
          <div className="px-2 pb-1.5">
            <input
              ref={searchInputRef}
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search namespaces..."
              className="w-full px-2.5 py-1.5 text-sm border border-stone-200 rounded-md bg-stone-50 text-stone-700 placeholder-stone-400 focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-100"
            />
          </div>
          <div className="overflow-y-auto max-h-56">
            {showAllNamespaces && (
              <button
                onClick={() => { onChange(allNamespacesValue); setOpen(false); }}
                className={`w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors ${
                  isAll
                    ? 'text-primary-700 bg-primary-50/60'
                    : 'text-stone-600 hover:bg-stone-50'
                }`}
              >
                <svg className={`w-3.5 h-3.5 shrink-0 ${isAll ? 'text-primary-500' : 'text-transparent'}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="20 6 9 17 4 12" />
                </svg>
                <span>All Namespaces</span>
              </button>
            )}
            {(showAllNamespaces && filteredNamespaces.length > 0) && (
              <div className="mx-3 my-1 border-t border-stone-100" />
            )}
            {filteredNamespaces.map((ns) => {
              const selected = value === ns;
              return (
                <button
                  key={ns}
                  onClick={() => { onChange(ns); setOpen(false); }}
                  className={`w-full text-left px-3 py-2 text-sm flex items-center gap-2 transition-colors ${
                    selected
                      ? 'text-primary-700 bg-primary-50/60'
                      : 'text-stone-700 hover:bg-stone-50'
                  }`}
                >
                  <svg className={`w-3.5 h-3.5 shrink-0 ${selected ? 'text-primary-500' : 'text-transparent'}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                  <span className="font-mono text-xs">{ns}</span>
                </button>
              );
            })}
            {filteredNamespaces.length === 0 && !showAllNamespaces && (
              <div className="px-3 py-3 text-sm text-stone-400 text-center">
                No matching namespaces
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export default NamespaceSelector;
