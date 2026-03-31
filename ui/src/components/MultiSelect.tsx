import React, { useState, useRef, useEffect } from 'react';

interface MultiSelectOption {
  value: string;
  label: string;
}

interface MultiSelectProps {
  options: MultiSelectOption[];
  selected: string[];
  onChange: (selected: string[]) => void;
  label: string;
  allLabel?: string;
}

function MultiSelect({ options, selected, onChange, label, allLabel = 'All' }: MultiSelectProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
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

  const toggle = (value: string) => {
    if (selected.includes(value)) {
      onChange(selected.filter((s) => s !== value));
    } else {
      onChange([...selected, value]);
    }
  };

  const clearAll = () => {
    onChange([]);
    setOpen(false);
  };

  const displayText = selected.length === 0
    ? allLabel
    : selected.length === 1
      ? options.find((o) => o.value === selected[0])?.label || selected[0]
      : `${selected.length} selected`;

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen(!open)}
        className={`
          flex items-center gap-1.5 pl-2.5 pr-2 py-1.5 rounded-md border text-xs transition-all
          ${open
            ? 'border-primary-300 ring-1 ring-primary-100 bg-white'
            : selected.length > 0
              ? 'border-primary-200 bg-primary-50/50 text-primary-700'
              : 'border-stone-200 bg-stone-50 text-stone-600 hover:border-stone-300'
          }
        `}
      >
        <span className="text-stone-400 font-medium">{label}:</span>
        <span className={`${selected.length > 0 ? 'font-medium' : ''}`}>
          {displayText}
        </span>
        <svg
          className={`w-3 h-3 text-stone-400 shrink-0 transition-transform duration-200 ${open ? 'rotate-180' : ''}`}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        >
          <path d="M6 9l6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>

      {open && (
        <div className="absolute top-full left-0 mt-1 min-w-[160px] bg-white border border-stone-200 rounded-lg shadow-lg z-50 py-1 animate-fade-in">
          {options.map((option) => {
            const checked = selected.includes(option.value);
            return (
              <button
                key={option.value}
                onClick={() => toggle(option.value)}
                className={`w-full text-left px-3 py-1.5 text-xs flex items-center gap-2 transition-colors ${
                  checked ? 'bg-primary-50/50 text-primary-700' : 'text-stone-600 hover:bg-stone-50'
                }`}
              >
                <span className={`
                  w-3.5 h-3.5 rounded border flex items-center justify-center shrink-0 transition-colors
                  ${checked
                    ? 'bg-primary-500 border-primary-500'
                    : 'border-stone-300 bg-white'
                  }
                `}>
                  {checked && (
                    <svg className="w-2.5 h-2.5 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="20 6 9 17 4 12" />
                    </svg>
                  )}
                </span>
                {option.label}
              </button>
            );
          })}
          {selected.length > 0 && (
            <>
              <div className="mx-2 my-1 border-t border-stone-100" />
              <button
                onClick={clearAll}
                className="w-full text-left px-3 py-1.5 text-xs text-stone-400 hover:text-stone-600 transition-colors"
              >
                Clear all
              </button>
            </>
          )}
        </div>
      )}
    </div>
  );
}

export default MultiSelect;
