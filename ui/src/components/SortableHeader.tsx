import React from 'react';

interface SortableHeaderProps {
  label: string;
  active: boolean;
  order: 'asc' | 'desc';
  onToggle: () => void;
  className?: string;
}

/**
 * SortableHeader renders a clickable table column header with sort direction indicator.
 */
function SortableHeader({ label, active, order, onToggle, className }: SortableHeaderProps) {
  return (
    <th
      className={`px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider cursor-pointer select-none hover:text-stone-600 transition-colors ${className || ''}`}
      onClick={onToggle}
    >
      <span className="inline-flex items-center gap-1">
        {label}
        <svg className={`w-3 h-3 transition-transform ${active ? 'text-stone-600' : 'text-stone-300'} ${active && order === 'asc' ? 'rotate-180' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
          <path d="M6 9l6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </span>
    </th>
  );
}

export default SortableHeader;
