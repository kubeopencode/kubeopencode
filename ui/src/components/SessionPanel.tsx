import React from 'react';
import type { SessionInfo } from '../api/client';

// Format token count to human-readable string
function formatTokens(count: number): string {
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}k`;
  return count.toString();
}

// Format cost string to display value
function formatCost(cost?: string): string {
  if (!cost) return '-';
  const num = parseFloat(cost);
  if (isNaN(num)) return cost;
  return `$${num.toFixed(4)}`;
}

function SessionSummaryCards({ session }: { session: SessionInfo }) {
  const summary = session.summary;
  if (!summary) return null;

  const totalTokens = (summary.tokenUsage?.input || 0) + (summary.tokenUsage?.output || 0);

  return (
    <div className="grid grid-cols-3 gap-3">
      {/* Messages */}
      <div className="bg-stone-50 rounded-lg p-3 border border-stone-100">
        <dt className="text-[10px] font-display font-medium text-stone-400 uppercase tracking-wider">Messages</dt>
        <dd className="mt-1 text-lg font-semibold text-stone-800 font-mono">{summary.messageCount || 0}</dd>
      </div>

      {/* Tokens */}
      <div className="bg-stone-50 rounded-lg p-3 border border-stone-100">
        <dt className="text-[10px] font-display font-medium text-stone-400 uppercase tracking-wider">Tokens</dt>
        <dd className="mt-1 text-lg font-semibold text-stone-800 font-mono">{formatTokens(totalTokens)}</dd>
        {summary.tokenUsage && (
          <p className="text-[10px] text-stone-400 mt-0.5">
            {formatTokens(summary.tokenUsage.input || 0)} in / {formatTokens(summary.tokenUsage.output || 0)} out
          </p>
        )}
      </div>

      {/* Cost */}
      <div className="bg-stone-50 rounded-lg p-3 border border-stone-100">
        <dt className="text-[10px] font-display font-medium text-stone-400 uppercase tracking-wider">Cost</dt>
        <dd className="mt-1 text-lg font-semibold text-stone-800 font-mono">{formatCost(summary.cost)}</dd>
      </div>
    </div>
  );
}

interface SessionPanelProps {
  session: SessionInfo;
}

function SessionPanel({ session }: SessionPanelProps) {
  if (!session.summary) return null;

  return (
    <div>
      <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
        Session Summary
      </h3>
      <SessionSummaryCards session={session} />
    </div>
  );
}

export default SessionPanel;
export { SessionSummaryCards, formatTokens, formatCost };
