import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';

interface YamlViewerProps {
  queryKey: string[];
  fetchYaml: () => Promise<string>;
}

function YamlViewer({ queryKey, fetchYaml }: YamlViewerProps) {
  const [isOpen, setIsOpen] = useState(false);

  const { data: yaml, isLoading, error } = useQuery({
    queryKey: [...queryKey, 'yaml'],
    queryFn: fetchYaml,
    enabled: isOpen,
  });

  return (
    <div className="mt-6">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center space-x-2 text-sm font-medium text-gray-600 hover:text-gray-900"
      >
        <svg
          className={`w-4 h-4 transform transition-transform ${isOpen ? 'rotate-90' : ''}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
        </svg>
        <span>YAML</span>
      </button>
      {isOpen && (
        <div className="mt-2 bg-gray-900 rounded-lg overflow-hidden">
          <div className="px-4 py-2 bg-gray-800 flex items-center justify-between">
            <span className="text-sm text-gray-300">Resource Definition</span>
            {yaml && (
              <button
                onClick={() => navigator.clipboard.writeText(yaml)}
                className="text-xs text-gray-400 hover:text-gray-200"
              >
                Copy
              </button>
            )}
          </div>
          <div className="p-4 max-h-96 overflow-y-auto">
            {isLoading ? (
              <span className="text-gray-500 text-sm">Loading...</span>
            ) : error ? (
              <span className="text-red-400 text-sm">Error: {(error as Error).message}</span>
            ) : (
              <pre className="text-sm text-gray-100 font-mono whitespace-pre">{yaml}</pre>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export default YamlViewer;
