import React from 'react';
import { Link } from 'react-router-dom';

export interface BreadcrumbItem {
  label: string;
  to?: string;
  /** If true, render as a namespace badge (monospace, muted style) */
  isNamespace?: boolean;
}

interface BreadcrumbsProps {
  items: BreadcrumbItem[];
}

function Breadcrumbs({ items }: BreadcrumbsProps) {
  return (
    <nav className="mb-5" aria-label="Breadcrumb">
      <ol className="flex items-center space-x-1.5 text-sm">
        {items.map((item, index) => (
          <li key={index} className="flex items-center">
            {index > 0 && (
              <svg className="w-3.5 h-3.5 mx-1 text-stone-300" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
                  clipRule="evenodd"
                />
              </svg>
            )}
            {item.to ? (
              <Link to={item.to} className="text-stone-400 hover:text-stone-600 transition-colors">
                {item.label}
              </Link>
            ) : item.isNamespace ? (
              <span className="text-stone-400 font-mono text-xs bg-stone-100 px-1.5 py-0.5 rounded">
                {item.label}
              </span>
            ) : (
              <span className="text-stone-700 font-medium">{item.label}</span>
            )}
          </li>
        ))}
      </ol>
    </nav>
  );
}

export default Breadcrumbs;
