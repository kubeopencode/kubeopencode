import React, { useState, useRef, useEffect, useCallback } from 'react';
import { useParams, Link, useNavigate, useLocation } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import api, { ShareTokenResponse } from '../api/client';
import { useToast } from '../contexts/ToastContext';
import Labels from '../components/Labels';
import AgentStatusBadge from '../components/AgentStatusBadge';
import CopyButton from '../components/CopyButton';
import ConfirmDialog from '../components/ConfirmDialog';
import Breadcrumbs from '../components/Breadcrumbs';
import YamlViewer from '../components/YamlViewer';
import TerminalPanel from '../components/TerminalPanel';
import { DetailSkeleton } from '../components/Skeleton';

type TabId = 'overview' | 'terminal' | 'share' | 'yaml';

function SuspendResumeButton({ namespace, name, suspended, onSuccess }: { namespace: string; name: string; suspended: boolean; onSuccess: () => void }) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [optimisticSuspended, setOptimisticSuspended] = useState<boolean | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const displaySuspended = optimisticSuspended !== null ? optimisticSuspended : suspended;

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const handleClick = async () => {
    if (timerRef.current) clearTimeout(timerRef.current);
    setLoading(true);
    setError('');
    const newState = !displaySuspended;
    try {
      if (displaySuspended) {
        await api.resumeAgent(namespace, name);
      } else {
        await api.suspendAgent(namespace, name);
      }
      setOptimisticSuspended(newState);
      timerRef.current = setTimeout(() => {
        onSuccess();
        setOptimisticSuspended(null);
        setLoading(false);
      }, 1500);
    } catch (err: unknown) {
      setOptimisticSuspended(null);
      setLoading(false);
      const isConflict = err instanceof Error && (err.message.includes('409') || err.message.includes('running tasks'));
      setError(isConflict ? 'Cannot suspend: agent has running tasks' : (newState ? 'Failed to suspend' : 'Failed to resume'));
      setTimeout(() => setError(''), 5000);
    }
  };
  return (
    <div className="flex items-center gap-2">
      <button
        onClick={handleClick}
        disabled={loading}
        className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
          displaySuspended
            ? 'bg-emerald-600 text-white hover:bg-emerald-700'
            : 'bg-amber-600 text-white hover:bg-amber-700'
        } disabled:opacity-50`}
      >
        {loading ? '...' : displaySuspended ? 'Resume' : 'Suspend'}
      </button>
      {error && <span className="text-xs text-red-500">{error}</span>}
    </div>
  );
}

function ServerConnectCommands({ namespace, agentName }: { namespace: string; agentName: string }) {
  const kocCmd = `kubeoc agent attach ${agentName} -n ${namespace}`;
  const goInstallCmd = 'go install github.com/kubeopencode/kubeopencode/cmd/kubeoc@latest';

  return (
    <div>
      <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Quick Connect</h3>
      <div className="space-y-3">
        <div>
          <p className="text-xs text-stone-500 mb-1.5">
            One-click attach via KubeOpenCode CLI
          </p>
          <div className="flex items-center gap-2 bg-stone-900 rounded-lg px-4 py-2.5 border border-stone-700">
            <code className="text-xs text-emerald-400 font-mono flex-1">{kocCmd}</code>
            <CopyButton text={kocCmd} />
          </div>
          <div className="mt-1.5 bg-stone-50 rounded-lg px-3 py-2 border border-stone-100">
            <p className="text-[11px] text-stone-400">
              Install KubeOpenCode CLI:{' '}
              <code className="bg-stone-100 px-1.5 py-0.5 rounded text-stone-500 font-mono select-all cursor-pointer">{goInstallCmd}</code>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

// Validate a comma-separated list of CIDR/IP entries.
// Returns null if valid, or an error string if invalid.
function validateAllowedIPs(input: string): string | null {
  if (!input.trim()) return null;
  const entries = input.split(',').map(s => s.trim()).filter(Boolean);
  for (const entry of entries) {
    // Match IPv4 with optional CIDR prefix
    if (!/^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(\/\d{1,2})?$/.test(entry)) {
      return `Invalid format: "${entry}". Expected IPv4 or CIDR (e.g. 10.0.0.0/8)`;
    }
    // Validate each octet
    const ipPart = entry.split('/')[0];
    const octets = ipPart.split('.').map(Number);
    if (octets.some(o => o < 0 || o > 255)) {
      return `Invalid IP: "${entry}". Each octet must be 0-255`;
    }
    // Validate CIDR prefix length
    if (entry.includes('/')) {
      const prefix = Number(entry.split('/')[1]);
      if (prefix < 0 || prefix > 32) {
        return `Invalid CIDR prefix: "${entry}". Must be 0-32`;
      }
    }
  }
  return null;
}

function ShareLinkSection({ namespace, name, shareStatus }: {
  namespace: string;
  name: string;
  shareStatus?: { enabled: boolean; active: boolean; expiresAt?: string; allowedIPs?: string[] };
}) {
  const { addToast } = useToast();
  const queryClient = useQueryClient();
  const [loading, setLoading] = useState(false);
  const [showConfig, setShowConfig] = useState(false);
  const [expiresIn, setExpiresIn] = useState('');
  const [allowedIPs, setAllowedIPs] = useState(shareStatus?.allowedIPs?.join(', ') || '');
  const [ipError, setIpError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const enabled = shareStatus?.enabled || false;

  const { data: shareToken, refetch: refetchToken } = useQuery({
    queryKey: ['agentShare', namespace, name],
    queryFn: () => api.getAgentShare(namespace, name),
    enabled: enabled,
  });

  const invalidate = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['agent', namespace, name] });
    queryClient.invalidateQueries({ queryKey: ['agentShare', namespace, name] });
  }, [queryClient, namespace, name]);

  const handleToggle = async () => {
    if (!enabled) {
      const err = validateAllowedIPs(allowedIPs);
      if (err) { setIpError(err); return; }
    }
    setLoading(true);
    try {
      if (enabled) {
        await api.deleteAgentShare(namespace, name);
        addToast('Share link disabled', 'success');
      } else {
        await api.updateAgentShare(namespace, name, {
          enabled: true,
          expiresIn: expiresIn || undefined,
          allowedIPs: allowedIPs ? allowedIPs.split(',').map(s => s.trim()).filter(Boolean) : undefined,
        });
        addToast('Share link enabled', 'success');
      }
      setTimeout(() => { invalidate(); setLoading(false); }, 1500);
    } catch (err) {
      addToast(`Failed: ${(err as Error).message}`, 'error');
      setLoading(false);
    }
  };

  const handleUpdateConfig = async () => {
    const err = validateAllowedIPs(allowedIPs);
    if (err) { setIpError(err); return; }
    setLoading(true);
    try {
      await api.updateAgentShare(namespace, name, {
        enabled: true,
        expiresIn: expiresIn || undefined,
        allowedIPs: allowedIPs ? allowedIPs.split(',').map(s => s.trim()).filter(Boolean) : undefined,
      });
      addToast('Share config updated', 'success');
      setTimeout(() => { invalidate(); setLoading(false); setShowConfig(false); }, 1500);
    } catch (err) {
      addToast(`Failed: ${(err as Error).message}`, 'error');
      setLoading(false);
    }
  };

  const handleAllowedIPsChange = (value: string) => {
    setAllowedIPs(value);
    if (ipError) setIpError(null); // Clear error on edit
  };

  const shareUrl = shareToken?.path ? `${window.location.origin}${shareToken.path}` : '';

  const handleCopy = () => {
    if (shareUrl) {
      navigator.clipboard.writeText(shareUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider">Share Link</h3>
        <button
          onClick={handleToggle}
          disabled={loading}
          className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
            enabled ? 'bg-emerald-500' : 'bg-stone-300'
          } disabled:opacity-50`}
        >
          <span className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
            enabled ? 'translate-x-[18px]' : 'translate-x-[3px]'
          }`} />
        </button>
      </div>

      {!enabled ? (
        <div className="bg-stone-50 rounded-lg p-4 border border-stone-100">
          <p className="text-xs text-stone-500">
            Enable to generate a shareable URL. Anyone with the link can access this agent's terminal without Kubernetes credentials.
          </p>
          <div className="mt-3 space-y-2">
            <div>
              <label className="text-xs text-stone-500">Expires in</label>
              <input
                type="text"
                value={expiresIn}
                onChange={e => setExpiresIn(e.target.value)}
                placeholder="e.g., 24h, 168h (empty = no expiry)"
                className="mt-0.5 w-full px-2.5 py-1.5 text-xs border border-stone-200 rounded-md bg-white"
              />
            </div>
            <div>
              <label className="text-xs text-stone-500">Allowed IPs (CIDR, comma-separated)</label>
              <input
                type="text"
                value={allowedIPs}
                onChange={e => handleAllowedIPsChange(e.target.value)}
                onBlur={() => { if (allowedIPs.trim()) setIpError(validateAllowedIPs(allowedIPs)); }}
                placeholder="e.g., 10.0.0.0/8, 192.168.1.0/24"
                className={`mt-0.5 w-full px-2.5 py-1.5 text-xs border rounded-md bg-white ${
                  ipError ? 'border-red-300 focus:ring-red-500' : 'border-stone-200'
                }`}
              />
              {ipError && <p className="mt-1 text-[11px] text-red-500">{ipError}</p>}
            </div>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          <div className="bg-emerald-50 rounded-lg p-4 border border-emerald-200">
            <div className="flex items-center gap-2 mb-2">
              <span className={`w-2 h-2 rounded-full ${shareToken?.active ? 'bg-emerald-500' : 'bg-amber-500'}`} />
              <span className="text-xs font-medium text-emerald-800">
                {shareToken?.active ? 'Active' : 'Inactive'}
              </span>
            </div>
            <div className="flex items-center gap-3 text-[11px] text-stone-500 mb-2">
              <span>
                {shareStatus?.expiresAt
                  ? `Expires: ${new Date(shareStatus.expiresAt).toLocaleString()}`
                  : 'No expiration'}
              </span>
              <span className="text-stone-300">|</span>
              <span>
                {shareStatus?.allowedIPs && shareStatus.allowedIPs.length > 0
                  ? `IPs: ${shareStatus.allowedIPs.join(', ')}`
                  : 'All IPs allowed'}
              </span>
            </div>
            {shareUrl && (
              <div className="flex items-center gap-2 bg-stone-900 rounded-lg px-3 py-2 border border-stone-700">
                <code className="text-xs text-emerald-400 font-mono flex-1 truncate">{shareUrl}</code>
                <button
                  onClick={handleCopy}
                  className="shrink-0 p-1 rounded hover:bg-white/10 transition-colors"
                  title="Copy share URL"
                >
                  {copied ? (
                    <svg className="w-3.5 h-3.5 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="20 6 9 17 4 12" />
                    </svg>
                  ) : (
                    <svg className="w-3.5 h-3.5 text-stone-400 hover:text-stone-200" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                    </svg>
                  )}
                </button>
                <a
                  href={shareUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="shrink-0 p-1 rounded hover:bg-white/10 transition-colors"
                  title="Open in new tab"
                >
                  <svg className="w-3.5 h-3.5 text-stone-400 hover:text-stone-200" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                    <polyline points="15 3 21 3 21 9" />
                    <line x1="10" y1="14" x2="21" y2="3" />
                  </svg>
                </a>
              </div>
            )}
          </div>

          <button
            onClick={() => setShowConfig(!showConfig)}
            className="text-xs text-stone-500 hover:text-stone-700 flex items-center gap-1"
          >
            <svg className={`w-3 h-3 transition-transform ${showConfig ? 'rotate-90' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M9 5l7 7-7 7" />
            </svg>
            Configure
          </button>

          {showConfig && (
            <div className="bg-stone-50 rounded-lg p-4 border border-stone-100 space-y-3">
              <div>
                <label className="text-xs text-stone-500">New expiry (from now)</label>
                <input
                  type="text"
                  value={expiresIn}
                  onChange={e => setExpiresIn(e.target.value)}
                  placeholder="e.g., 24h, 168h (empty = keep current)"
                  className="mt-0.5 w-full px-2.5 py-1.5 text-xs border border-stone-200 rounded-md bg-white"
                />
              </div>
              <div>
                <label className="text-xs text-stone-500">Allowed IPs (CIDR, comma-separated)</label>
                <input
                  type="text"
                  value={allowedIPs}
                  onChange={e => handleAllowedIPsChange(e.target.value)}
                  onBlur={() => { if (allowedIPs.trim()) setIpError(validateAllowedIPs(allowedIPs)); }}
                  placeholder="e.g., 10.0.0.0/8 (empty = all IPs)"
                  className={`mt-0.5 w-full px-2.5 py-1.5 text-xs border rounded-md bg-white ${
                    ipError ? 'border-red-300 focus:ring-red-500' : 'border-stone-200'
                  }`}
                />
                {ipError && <p className="mt-1 text-[11px] text-red-500">{ipError}</p>}
              </div>
              <button
                onClick={handleUpdateConfig}
                disabled={loading || !!ipError}
                className="px-3 py-1.5 text-xs font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-50"
              >
                {loading ? 'Saving...' : 'Save Changes'}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

const TABS: { id: TabId; label: string; icon: React.ReactNode }[] = [
  {
    id: 'overview',
    label: 'Overview',
    icon: (
      <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="3" width="7" height="7" rx="1" />
        <rect x="14" y="3" width="7" height="7" rx="1" />
        <rect x="3" y="14" width="7" height="7" rx="1" />
        <rect x="14" y="14" width="7" height="7" rx="1" />
      </svg>
    ),
  },
  {
    id: 'terminal',
    label: 'Terminal',
    icon: (
      <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="4 17 10 11 4 5" />
        <line x1="12" y1="19" x2="20" y2="19" />
      </svg>
    ),
  },
  {
    id: 'share',
    label: 'Share',
    icon: (
      <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
        <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
      </svg>
    ),
  },
  {
    id: 'yaml',
    label: 'YAML',
    icon: (
      <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="16 18 22 12 16 6" />
        <polyline points="8 6 2 12 8 18" />
      </svg>
    ),
  },
];

function AgentDetailPage() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const { addToast } = useToast();
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // Determine initial tab from URL hash
  const hashTab = location.hash.replace('#', '') as TabId;
  const initialTab: TabId = ['overview', 'terminal', 'share', 'yaml'].includes(hashTab) ? hashTab : 'overview';
  const [activeTab, setActiveTab] = useState<TabId>(initialTab);

  const handleTabChange = (tab: TabId) => {
    setActiveTab(tab);
    // Update URL hash without creating a new history entry
    window.history.replaceState(null, '', `${location.pathname}#${tab}`);
  };

  const { data: agent, isLoading, error, refetch } = useQuery({
    queryKey: ['agent', namespace, name],
    queryFn: () => api.getAgent(namespace!, name!),
    enabled: !!namespace && !!name,
    refetchInterval: (query) => {
      const a = query.state.data;
      // Poll every 3s while agent is in a transitional state (starting/stopping)
      if (a && !a.serverStatus?.suspended && !a.serverStatus?.ready) return 3000;
      return false;
    },
  });

  if (isLoading) {
    return <DetailSkeleton />;
  }

  if (error || !agent) {
    const errorMessage = (error as Error)?.message || 'Not found';
    const isNotFound = errorMessage.includes('not found');
    return (
      <div className="bg-red-50 border border-red-200 rounded-xl p-6 animate-fade-in">
        <h3 className="font-display text-base font-semibold text-red-800 mb-2">
          {isNotFound ? 'Agent Not Found' : 'Error Loading Agent'}
        </h3>
        <p className="text-sm text-red-600 mb-4">
          {isNotFound
            ? `The agent "${name}" in namespace "${namespace}" does not exist.`
            : errorMessage}
        </p>
        <Link
          to="/agents"
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-red-700 bg-red-100 rounded-lg hover:bg-red-200 transition-colors"
        >
          Back to Agents
        </Link>
      </div>
    );
  }

  const terminalReady = !!(agent.serverStatus && agent.serverStatus.ready);
  const hasCredentials = agent.credentials && agent.credentials.length > 0;
  const hasSkills = agent.skills && agent.skills.length > 0;
  const hasPlugins = agent.plugins && agent.plugins.length > 0;
  const hasConfig = agent.config && Object.keys(agent.config).length > 0;
  const hasContexts = agent.contexts && agent.contexts.length > 0;


  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'Agents', to: '/agents' },
        { label: namespace!, isNamespace: true },
        { label: name! },
      ]} />

      <div className="bg-white rounded-xl border-0 overflow-hidden shadow-card">
        {/* Header */}
        <div className="px-6 py-5 border-b border-stone-100">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2.5">
                <h2 className="font-display text-xl font-bold text-stone-900">{agent.name}</h2>
                <AgentStatusBadge
                  suspended={agent.serverStatus?.suspended}
                  ready={agent.serverStatus?.ready}
                />
              </div>
              <p className="text-xs text-stone-400 mt-0.5 font-mono">{agent.namespace}</p>
              {agent.profile && (
                <p className="mt-2 text-sm text-stone-500 leading-relaxed">{agent.profile}</p>
              )}
            </div>
            <div className="flex items-center gap-2 shrink-0">
              {agent.serverStatus && (
                <SuspendResumeButton
                  namespace={agent.namespace}
                  name={agent.name}
                  suspended={agent.serverStatus.suspended}
                  onSuccess={() => refetch()}
                />
              )}
              <Link
                to={`/tasks/create?agent=${agent.namespace}/${agent.name}`}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 transition-colors"
              >
                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M12 5v14M5 12h14" strokeLinecap="round" />
                </svg>
                Create Task
              </Link>
              <button
                onClick={() => setShowDeleteDialog(true)}
                className="px-3 py-1.5 rounded-lg text-xs font-medium text-red-600 bg-red-50 border border-red-200 hover:bg-red-100 transition-colors"
              >
                Delete
              </button>
            </div>
          </div>
        </div>

        {/* Tab Bar */}
        <div className="px-6 border-b border-stone-100 bg-stone-50/50">
          <nav className="flex space-x-1 -mb-px" aria-label="Tabs">
            {TABS.map((tab) => {
              const isActive = activeTab === tab.id;
              // Hide terminal tab if server not ready
              if (tab.id === 'terminal' && !terminalReady) return null;
              // Hide share tab if server not available
              if (tab.id === 'share' && !agent.serverStatus) return null;
              return (
                <button
                  key={tab.id}
                  onClick={() => handleTabChange(tab.id)}
                  className={`flex items-center gap-1.5 px-3 py-2.5 text-xs font-medium border-b-2 transition-colors ${
                    isActive
                      ? 'border-primary-600 text-primary-700'
                      : 'border-transparent text-stone-500 hover:text-stone-700 hover:border-stone-300'
                  }`}
                >
                  <span className={isActive ? 'text-primary-600' : 'text-stone-400'}>{tab.icon}</span>
                  {tab.label}
                  {tab.id === 'share' && agent.share?.enabled && (
                    <span className={`w-1.5 h-1.5 rounded-full ${
                      isActive ? 'bg-emerald-500' : 'bg-emerald-400'
                    }`} />
                  )}
                </button>
              );
            })}
          </nav>
        </div>

        {/* Tab Content */}
        {activeTab === 'overview' && (
          <div className="px-6 py-5 space-y-6">
            {/* Labels */}
            {agent.labels && Object.keys(agent.labels).length > 0 && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Labels</h3>
                <Labels labels={agent.labels} />
              </div>
            )}

            {/* Template Reference */}
            {agent.templateRef && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Template</h3>
                <Link
                  to={`/templates/${agent.namespace}/${agent.templateRef.name}`}
                  className="inline-flex items-center gap-2 bg-teal-50 rounded-lg px-4 py-2.5 border border-teal-200 hover:border-teal-300 transition-colors group"
                >
                  <svg className="w-4 h-4 text-teal-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="3" y="3" width="7" height="7" rx="1" />
                    <rect x="14" y="3" width="7" height="7" rx="1" />
                    <rect x="3" y="14" width="7" height="7" rx="1" />
                    <rect x="14" y="14" width="7" height="7" rx="1" />
                  </svg>
                  <span className="text-sm font-medium text-teal-700 group-hover:text-teal-800">{agent.templateRef.name}</span>
                  <svg className="w-3.5 h-3.5 text-teal-400 group-hover:text-teal-600 transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                </Link>
              </div>
            )}

            {/* Images */}
            {(agent.executorImage || agent.agentImage) && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-4">Images</h3>
                <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                  {agent.executorImage && (
                    <div>
                      <dt className="text-xs text-stone-400">Executor Image</dt>
                      <dd className="mt-1 text-xs text-stone-700 font-mono bg-stone-50 px-3 py-2 rounded-lg border border-stone-100 break-all">
                        {agent.executorImage}
                      </dd>
                    </div>
                  )}
                  {agent.agentImage && (
                    <div>
                      <dt className="text-xs text-stone-400">Agent Image</dt>
                      <dd className="mt-1 text-xs text-stone-700 font-mono bg-stone-50 px-3 py-2 rounded-lg border border-stone-100 break-all">
                        {agent.agentImage}
                      </dd>
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Runtime */}
            {(agent.workspaceDir || agent.serviceAccountName) && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-4">Runtime</h3>
                <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                  {agent.workspaceDir && (
                    <div>
                      <dt className="text-xs text-stone-400">Workspace Directory</dt>
                      <dd className="mt-1 text-sm text-stone-700 font-mono">{agent.workspaceDir}</dd>
                    </div>
                  )}
                  {agent.serviceAccountName && (
                    <div>
                      <dt className="text-xs text-stone-400">Service Account</dt>
                      <dd className="mt-1 text-sm text-stone-700 font-mono">{agent.serviceAccountName}</dd>
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Task Management */}
            {(agent.maxConcurrentTasks || agent.quota || agent.standby) && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-4">Task Management</h3>
                <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                  {agent.maxConcurrentTasks && (
                    <div>
                      <dt className="text-xs text-stone-400">Max Concurrent Tasks</dt>
                      <dd className="mt-1 text-sm text-stone-700 font-mono">{agent.maxConcurrentTasks}</dd>
                    </div>
                  )}
                  {agent.quota && (
                    <div>
                      <dt className="text-xs text-stone-400">Quota</dt>
                      <dd className="mt-1 text-sm text-stone-700">
                        Max <span className="font-mono font-medium text-stone-800">{agent.quota.maxTaskStarts}</span> starts per{' '}
                        <span className="font-mono font-medium text-stone-800">{agent.quota.windowSeconds}</span>s
                      </dd>
                    </div>
                  )}
                  {agent.standby && (
                    <div>
                      <dt className="text-xs text-stone-400">Standby</dt>
                      <dd className="mt-1 text-sm text-stone-700">
                        Auto-suspend after <span className="font-mono font-medium text-stone-800">{agent.standby.idleTimeout}</span> idle
                      </dd>
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Server Status */}
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Server Status</h3>
              {agent.serverStatus ? (
                <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                  <div>
                    <dt className="text-xs text-stone-400">Deployment</dt>
                    <dd className="mt-1 text-sm text-stone-700 font-mono">{agent.serverStatus.deploymentName}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-stone-400">Service</dt>
                    <dd className="mt-1 text-sm text-stone-700 font-mono">{agent.serverStatus.serviceName}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-stone-400">URL</dt>
                    <dd className="mt-1 flex items-center gap-1.5">
                      <span className="text-sm text-stone-700 font-mono break-all">{agent.serverStatus.url}</span>
                      <CopyButton text={agent.serverStatus.url || ''} />
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs text-stone-400">Status</dt>
                    <dd className="mt-1 text-sm font-mono">
                      {agent.serverStatus.suspended ? (
                        <span className="text-amber-600">Suspended</span>
                      ) : agent.serverStatus.ready ? (
                        <span className="text-emerald-600">Ready</span>
                      ) : (
                        <span className="text-stone-500">Not Ready</span>
                      )}
                    </dd>
                  </div>
                </div>
              ) : (
                <div className="bg-amber-50 rounded-lg p-4 border border-amber-200">
                  <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-amber-500 animate-pulse" />
                    <p className="text-sm text-amber-700 font-medium">Server not ready</p>
                  </div>
                  <p className="text-xs text-amber-600 mt-1">
                    The server deployment has not been created yet or is still starting up. Check controller logs for errors.
                  </p>
                </div>
              )}
            </div>

            {/* Git Sync Status */}
            {agent.serverStatus?.gitSyncStatuses && agent.serverStatus.gitSyncStatuses.length > 0 && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Git Sync</h3>
                <div className="space-y-2">
                  {agent.serverStatus.gitSyncStatuses.map((gs, idx) => (
                    <div key={idx} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm text-stone-800">{gs.name}</span>
                        {gs.commitHash && (
                          <span className="text-[11px] font-mono text-stone-500 bg-stone-100 px-2 py-0.5 rounded">
                            {gs.commitHash.substring(0, 12)}
                          </span>
                        )}
                      </div>
                      {gs.lastSynced && (
                        <p className="text-xs text-stone-400 mt-1">
                          Last synced: {new Date(gs.lastSynced).toLocaleString()}
                        </p>
                      )}
                    </div>
                  ))}
                </div>
                {agent.conditions?.some(c => c.type === 'GitSyncPending' && c.status === 'True') && (
                  <div className="mt-2 bg-amber-50 rounded-lg p-3 border border-amber-200">
                    <div className="flex items-center gap-2">
                      <span className="w-2 h-2 rounded-full bg-amber-500 animate-pulse" />
                      <p className="text-sm text-amber-700 font-medium">Rollout pending</p>
                    </div>
                    <p className="text-xs text-amber-600 mt-1">
                      {agent.conditions.find(c => c.type === 'GitSyncPending')?.message}
                    </p>
                  </div>
                )}
              </div>
            )}

            {/* Quick Connect */}
            {agent.serverStatus && (
              <ServerConnectCommands
                namespace={agent.namespace}
                agentName={agent.name}
              />
            )}

            {/* Conditions */}
            {agent.conditions && agent.conditions.length > 0 && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Conditions</h3>
                <div className="space-y-2">
                  {agent.conditions.map((condition, idx) => (
                    <div key={idx} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm text-stone-800">{condition.type}</span>
                        <span className={`text-[11px] px-2 py-0.5 rounded-md border font-medium ${
                          condition.status === 'True'
                            ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                            : 'bg-stone-50 text-stone-500 border-stone-200'
                        }`}>
                          {condition.status}
                        </span>
                      </div>
                      {condition.reason && (
                        <p className="text-xs text-stone-500 mt-1">Reason: {condition.reason}</p>
                      )}
                      {condition.message && (
                        <p className="text-xs text-stone-400 mt-1">{condition.message}</p>
                      )}
                    </div>
                   ))}
                </div>
              </div>
            )}

            {/* Credentials */}
            {hasCredentials && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                  Credentials ({agent.credentials!.length})
                </h3>
                <div className="space-y-2">
                  {agent.credentials!.map((cred, idx) => (
                    <div key={idx} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm text-stone-800">{cred.name}</span>
                        <span className="text-xs text-stone-400 font-mono">{cred.secretRef}</span>
                      </div>
                      {(cred.env || cred.mountPath) && (
                        <div className="mt-1 text-xs text-stone-500 space-x-3">
                          {cred.env && <span>ENV: <span className="font-mono">{cred.env}</span></span>}
                          {cred.mountPath && <span>Mount: <span className="font-mono">{cred.mountPath}</span></span>}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Skills */}
            {hasSkills && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                  Skills ({agent.skills!.length})
                </h3>
                <div className="space-y-2">
                  {agent.skills!.map((skill, idx) => (
                    <div key={idx} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm text-stone-800">
                          {skill.name}
                        </span>
                        <span className="text-[11px] px-2 py-0.5 rounded-md bg-violet-50 text-violet-600 border border-violet-200 font-medium">
                          Git
                        </span>
                      </div>
                      {skill.git && (
                        <p className="mt-1 text-[11px] text-stone-400 font-mono truncate">
                          {skill.git.repository}
                          {skill.git.ref && skill.git.ref !== 'HEAD' ? `@${skill.git.ref}` : ''}
                          {skill.git.path ? ` / ${skill.git.path}` : ''}
                        </p>
                      )}
                      {skill.git?.names && skill.git.names.length > 0 && (
                        <div className="mt-1.5 flex flex-wrap gap-1">
                          {skill.git.names.map((sname, i) => (
                            <span key={i} className="text-[10px] px-1.5 py-0.5 rounded bg-violet-50 text-violet-500 border border-violet-100">
                              {sname}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Plugins */}
            {hasPlugins && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                  Plugins ({agent.plugins!.length})
                </h3>
                <div className="space-y-2">
                  {agent.plugins!.map((plugin, idx) => (
                    <div key={idx} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm text-stone-800 font-mono">
                          {plugin.name}
                        </span>
                        {plugin.target && (
                          <span className={`text-[11px] px-2 py-0.5 rounded-md border font-medium ${
                            plugin.target === 'tui'
                              ? 'bg-amber-50 text-amber-600 border-amber-200'
                              : 'bg-teal-50 text-teal-600 border-teal-200'
                          }`}>
                            {plugin.target}
                          </span>
                        )}
                      </div>
                      {plugin.options && Object.keys(plugin.options).length > 0 && (
                        <pre className="mt-2 text-[11px] text-stone-500 font-mono bg-stone-100 rounded p-2 overflow-x-auto">
                          {JSON.stringify(plugin.options, null, 2)}
                        </pre>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* OpenCode Config */}
            {hasConfig && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                  OpenCode Config
                </h3>
                <pre className="text-xs text-stone-600 font-mono bg-stone-50 rounded-lg p-4 border border-stone-100 overflow-x-auto">
                  {JSON.stringify(agent.config, null, 2)}
                </pre>
              </div>
            )}

            {/* Contexts */}
            {hasContexts && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                  Contexts ({agent.contexts!.length})
                </h3>
                <div className="space-y-2">
                  {agent.contexts!.map((ctx, idx) => (
                    <div key={idx} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm text-stone-800">
                          {ctx.name || `Context ${idx + 1}`}
                        </span>
                        <span className="text-[11px] px-2 py-0.5 rounded-md bg-sky-50 text-sky-600 border border-sky-200 font-medium">
                          {ctx.type}
                        </span>
                      </div>
                      {ctx.description && (
                        <p className="mt-1 text-xs text-stone-500">{ctx.description}</p>
                      )}
                      {ctx.mountPath && (
                        <p className="mt-1 text-[11px] text-stone-400 font-mono">
                          mount: {ctx.mountPath}
                        </p>
                      )}
                      {ctx.sync && ctx.sync.enabled && (
                        <div className="mt-1.5 flex items-center gap-2">
                          <span className="text-[11px] px-1.5 py-0.5 rounded bg-emerald-50 text-emerald-600 border border-emerald-200 font-medium">
                            sync: {ctx.sync.policy || 'HotReload'}
                          </span>
                          {ctx.sync.interval && (
                            <span className="text-[11px] text-stone-400">
                              every {ctx.sync.interval}
                            </span>
                          )}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {activeTab === 'terminal' && terminalReady && (
          <div className="p-4">
            <TerminalPanel namespace={agent.namespace} agentName={agent.name} defaultMode="expanded" />
          </div>
        )}

        {activeTab === 'share' && agent.serverStatus && (
          <div className="px-6 py-5">
            <ShareLinkSection
              namespace={agent.namespace}
              name={agent.name}
              shareStatus={agent.share}
            />
          </div>
        )}

        {activeTab === 'yaml' && (
          <div className="p-4">
            <YamlViewer
              queryKey={['agent', namespace!, name!]}
              fetchYaml={() => api.getAgentYaml(namespace!, name!)}
              onSave={async (yaml) => {
                await api.updateAgentYaml(namespace!, name!, yaml);
                refetch();
              }}
              defaultOpen
              hideToggle
            />
          </div>
        )}
      </div>

      <ConfirmDialog
        open={showDeleteDialog}
        title="Delete Agent"
        message={`Are you sure you want to delete Agent "${name}"? This will remove the deployment, service, and all associated resources. This action cannot be undone.`}
        confirmLabel="Delete"
        variant="danger"
        onConfirm={async () => {
          setShowDeleteDialog(false);
          setDeleting(true);
          try {
            await api.deleteAgent(namespace!, name!);
            addToast(`Agent "${name}" deleted`, 'success');
            navigate('/agents');
          } catch (err) {
            addToast(`Failed to delete agent: ${(err as Error).message}`, 'error');
            setDeleting(false);
          }
        }}
        onCancel={() => setShowDeleteDialog(false)}
      />
    </div>
  );
}

export default AgentDetailPage;
