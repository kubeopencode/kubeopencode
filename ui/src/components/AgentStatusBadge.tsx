import React from 'react';

interface AgentStatusBadgeProps {
  suspended?: boolean;
  ready?: boolean;
}

/**
 * AgentStatusBadge renders the Live / Starting / Suspended status indicator
 * for an Agent's server status. Replaces duplicated inline rendering across
 * DashboardPage, AgentsPage, AgentDetailPage, and AgentTemplateDetailPage.
 */
function AgentStatusBadge({ suspended, ready }: AgentStatusBadgeProps) {
  if (suspended) {
    return (
      <span className="inline-flex items-center text-[11px] font-medium text-amber-600">
        <span className="mr-1.5 inline-flex rounded-full h-1.5 w-1.5 bg-amber-400" />
        Suspended
      </span>
    );
  }

  if (ready) {
    return (
      <span className="inline-flex items-center text-[11px] font-medium text-emerald-600">
        <span className="mr-1.5 inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500" />
        Live
      </span>
    );
  }

  return (
    <span className="inline-flex items-center text-[11px] font-medium text-violet-600">
      <span className="relative mr-1.5 flex h-1.5 w-1.5">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 bg-violet-400" />
        <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-violet-400" />
      </span>
      Starting
    </span>
  );
}

export default AgentStatusBadge;
