import React from 'react';

interface StatusBadgeProps {
  phase: string;
}

function StatusBadge({ phase }: StatusBadgeProps) {
  const lowerPhase = phase.toLowerCase();

  const getStatusClass = () => {
    switch (lowerPhase) {
      case 'pending':
        return 'bg-gray-100 text-gray-800';
      case 'queued':
        return 'bg-yellow-100 text-yellow-800';
      case 'running':
        return 'bg-blue-100 text-blue-800';
      case 'completed':
        return 'bg-green-100 text-green-800';
      case 'failed':
        return 'bg-red-100 text-red-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  const getDotClass = () => {
    switch (lowerPhase) {
      case 'running':
        return 'bg-blue-500';
      case 'queued':
        return 'bg-yellow-500';
      default:
        return '';
    }
  };

  const isActive = lowerPhase === 'running' || lowerPhase === 'queued';
  const dotClass = getDotClass();

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusClass()}`}
    >
      {isActive && (
        <span className="relative mr-1.5 flex h-2 w-2">
          <span
            className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${dotClass}`}
          />
          <span
            className={`relative inline-flex rounded-full h-2 w-2 ${dotClass}`}
          />
        </span>
      )}
      {phase}
    </span>
  );
}

export default StatusBadge;
