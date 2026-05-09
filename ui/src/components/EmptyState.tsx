import React from 'react';
import { Link } from 'react-router-dom';

interface EmptyStateAction {
  label: string;
  to: string;
}

interface EmptyStateProps {
  icon: React.ReactNode;
  title: string;
  description: string;
  action?: EmptyStateAction;
  secondaryAction?: EmptyStateAction;
  children?: React.ReactNode;
}

function EmptyState({ icon, title, description, action, secondaryAction, children }: EmptyStateProps) {
  return (
    <div className="text-center py-16 px-6">
      <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-stone-100 mb-4">
        {icon}
      </div>
      <h3 className="font-display text-base font-semibold text-stone-800 mb-1.5">{title}</h3>
      <p className="text-sm text-stone-500 max-w-md mx-auto leading-relaxed">{description}</p>
      {children}
      {(action || secondaryAction) && (
        <div className="mt-5 flex items-center justify-center gap-3">
          {action && (
            <Link
              to={action.to}
              className="inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 transition-colors shadow-sm"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M12 5v14M5 12h14" strokeLinecap="round" />
              </svg>
              {action.label}
            </Link>
          )}
          {secondaryAction && (
            <Link
              to={secondaryAction.to}
              className="inline-flex items-center px-4 py-2.5 text-sm font-medium text-stone-600 bg-white shadow-ring rounded-lg hover:shadow-card transition-all"
            >
              {secondaryAction.label}
            </Link>
          )}
        </div>
      )}
    </div>
  );
}

// Pre-built icons for common resource types
export const TaskIcon = () => (
  <svg className="w-7 h-7 text-stone-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2" />
    <rect x="9" y="3" width="6" height="4" rx="1" />
    <path d="M9 14l2 2 4-4" />
  </svg>
);

export const AgentIcon = () => (
  <svg className="w-7 h-7 text-stone-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <rect x="5" y="11" width="14" height="10" rx="2" />
    <circle cx="9" cy="16" r="1" />
    <circle cx="15" cy="16" r="1" />
    <path d="M9 7L9 4M15 7L15 4M12 7L12 2" />
  </svg>
);

export const TemplateIcon = () => (
  <svg className="w-7 h-7 text-stone-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="3" width="7" height="7" rx="1" />
    <rect x="14" y="3" width="7" height="7" rx="1" />
    <rect x="3" y="14" width="7" height="7" rx="1" />
    <rect x="14" y="14" width="7" height="7" rx="1" />
  </svg>
);

export const CronIcon = () => (
  <svg className="w-7 h-7 text-stone-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="10" />
    <polyline points="12 6 12 12 16 14" />
  </svg>
);

export default EmptyState;
