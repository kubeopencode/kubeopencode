import React from 'react';
import { Link, useLocation } from 'react-router-dom';

function NotFoundPage() {
  const location = useLocation();

  return (
    <div className="animate-fade-in flex items-center justify-center min-h-[60vh]">
      <div className="text-center max-w-md">
        <p className="text-6xl font-display font-bold text-stone-200">404</p>
        <h1 className="mt-4 font-display text-xl font-semibold text-stone-800">Page Not Found</h1>
        <p className="mt-2 text-sm text-stone-500">
          The path <code className="bg-stone-100 px-2 py-0.5 rounded text-stone-600 font-mono text-xs">{location.pathname}</code> does not exist.
        </p>
        <Link
          to="/"
          className="mt-6 inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 transition-colors"
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M19 12H5M12 19l-7-7 7-7" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          Back to Dashboard
        </Link>
      </div>
    </div>
  );
}

export default NotFoundPage;
