import React from 'react';
import type { SessionInfo } from '../api/client';

// Format token count to human-readable string
function formatTokens(count: number): string {
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}k`;
  return count.toString();
}

// Format cost string to display value. Returns '-' for missing/zero cost.
function formatCost(cost?: string): string {
  if (!cost) return '-';
  const num = parseFloat(cost);
  if (isNaN(num) || num === 0) return '-';
  return `$${num.toFixed(4)}`;
}

// Check if cost has a meaningful (non-zero) value
function hasCost(cost?: string): boolean {
  if (!cost) return false;
  const num = parseFloat(cost);
  return !isNaN(num) && num > 0;
}

function SessionSummaryCards({ session }: { session: SessionInfo }) {
  const summary = session.summary;
  if (!summary) return null;

  const totalTokens = (summary.tokenUsage?.input || 0) + (summary.tokenUsage?.output || 0);
  const showCost = hasCost(summary.cost);

  return (
    <div className={`grid ${showCost ? 'grid-cols-3' : 'grid-cols-2'} gap-3`}>
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

      {/* Cost - only shown when > 0 */}
      {showCost && (
        <div className="bg-stone-50 rounded-lg p-3 border border-stone-100">
          <dt className="text-[10px] font-display font-medium text-stone-400 uppercase tracking-wider">Cost</dt>
          <dd className="mt-1 text-lg font-semibold text-stone-800 font-mono">{formatCost(summary.cost)}</dd>
        </div>
      )}
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
export { SessionSummaryCards, formatTokens, formatCost, hasCost };
