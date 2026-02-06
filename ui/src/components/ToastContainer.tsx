import React from 'react';
import { useToast } from '../contexts/ToastContext';

function ToastContainer() {
  const { toasts, removeToast } = useToast();

  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col space-y-2">
      {toasts.map((toast) => {
        const bgClass =
          toast.type === 'success'
            ? 'bg-green-600'
            : toast.type === 'error'
              ? 'bg-red-600'
              : 'bg-gray-800';

        return (
          <div
            key={toast.id}
            className={`${bgClass} text-white px-4 py-3 rounded-lg shadow-lg flex items-center space-x-3 min-w-[280px] max-w-md animate-slide-in`}
          >
            <span className="text-sm flex-1">{toast.message}</span>
            <button
              onClick={() => removeToast(toast.id)}
              className="text-white/70 hover:text-white flex-shrink-0"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        );
      })}
    </div>
  );
}

export default ToastContainer;
