import React, { useState, useRef, useEffect } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import { useToast } from '../contexts/ToastContext';
import Labels from '../components/Labels';
import Breadcrumbs from '../components/Breadcrumbs';
import YamlViewer from '../components/YamlViewer';
import TerminalPanel from '../components/TerminalPanel';
import { DetailSkeleton } from '../components/Skeleton';

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
      // Keep button disabled until refetch completes
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

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };
  return (
    <button
      onClick={handleCopy}
      className="shrink-0 p-1.5 rounded-md text-stone-400 hover:text-stone-600 hover:bg-stone-200 transition-colors"
      title="Copy to clipboard"
    >
      {copied ? (
        <svg className="w-4 h-4 text-emerald-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M20 6L9 17l-5-5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      ) : (
        <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
        </svg>
      )}
    </button>
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

function DeleteAgentButton({ namespace, name }: { namespace: string; name: string }) {
  const [confirming, setConfirming] = useState(false);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const { addToast } = useToast();

  const handleDelete = async () => {
    setLoading(true);
    try {
      await api.deleteAgent(namespace, name);
      addToast(`Agent "${name}" deleted`, 'success');
      navigate('/agents');
    } catch (err) {
      addToast(`Failed to delete agent: ${(err as Error).message}`, 'error');
      setLoading(false);
      setConfirming(false);
    }
  };

  if (confirming) {
    return (
      <div className="flex items-center gap-1.5">
        <span className="text-xs text-red-600">Delete?</span>
        <button
          onClick={handleDelete}
          disabled={loading}
          className="px-2.5 py-1 rounded-md text-xs font-medium text-white bg-red-600 hover:bg-red-700 disabled:opacity-50 transition-colors"
        >
          {loading ? '...' : 'Confirm'}
        </button>
        <button
          onClick={() => setConfirming(false)}
          className="px-2.5 py-1 rounded-md text-xs font-medium text-stone-500 bg-stone-100 hover:bg-stone-200 transition-colors"
        >
          Cancel
        </button>
      </div>
    );
  }

  return (
    <button
      onClick={() => setConfirming(true)}
      className="px-3 py-1.5 rounded-lg text-xs font-medium text-red-600 bg-red-50 border border-red-200 hover:bg-red-100 transition-colors"
    >
      Delete
    </button>
  );
}

function AgentDetailPage() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();

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

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'Agents', to: '/agents' },
        { label: namespace! },
        { label: name! },
      ]} />

      <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
        <div className="px-6 py-5 border-b border-stone-100">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2.5">
                <h2 className="font-display text-xl font-bold text-stone-900">{agent.name}</h2>
                <span className={`inline-flex items-center text-xs font-medium ${
                  agent.serverStatus?.suspended
                    ? 'text-amber-600'
                    : agent.serverStatus?.ready
                      ? 'text-emerald-600'
                      : 'text-violet-600'
                }`}>
                  {agent.serverStatus?.suspended ? (
                    <span className="mr-1.5 inline-flex rounded-full h-1.5 w-1.5 bg-amber-400" />
                  ) : agent.serverStatus?.ready ? (
                    <span className="mr-1.5 inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500" />
                  ) : (
                    <span className="relative mr-1.5 flex h-1.5 w-1.5">
                      <span className="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 bg-violet-400" />
                      <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-violet-400" />
                    </span>
                  )}
                  {agent.serverStatus?.suspended ? 'Suspended' : agent.serverStatus?.ready ? 'Live' : 'Starting'}
                </span>
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
              <DeleteAgentButton namespace={agent.namespace} name={agent.name} />
            </div>
          </div>
        </div>

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
                  <dd className="mt-1 text-sm text-stone-700 font-mono break-all">{agent.serverStatus.url}</dd>
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

          {/* Terminal Panel (ready) */}
          {agent.serverStatus && agent.serverStatus.ready && (
            <TerminalPanel namespace={agent.namespace} agentName={agent.name} />
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
          {agent.credentials && agent.credentials.length > 0 && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                Credentials ({agent.credentials.length})
              </h3>
              <div className="space-y-2">
                {agent.credentials.map((cred, idx) => (
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

          {/* Contexts */}
          {agent.contexts && agent.contexts.length > 0 && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                Contexts ({agent.contexts.length})
              </h3>
              <div className="space-y-2">
                {agent.contexts.map((ctx, idx) => (
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
                  </div>
                ))}
              </div>
            </div>
          )}

        </div>
      </div>

      <YamlViewer
        queryKey={['agent', namespace!, name!]}
        fetchYaml={() => api.getAgentYaml(namespace!, name!)}
        onSave={async (yaml) => {
          await api.updateAgentYaml(namespace!, name!, yaml);
          refetch();
        }}
      />
    </div>
  );
}

export default AgentDetailPage;
