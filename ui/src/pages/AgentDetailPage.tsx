import React, { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import Labels from '../components/Labels';
import Breadcrumbs from '../components/Breadcrumbs';
import YamlViewer from '../components/YamlViewer';
import TerminalPanel from '../components/TerminalPanel';
import { DetailSkeleton } from '../components/Skeleton';

function SuspendResumeButton({ namespace, name, suspended, onSuccess }: { namespace: string; name: string; suspended: boolean; onSuccess: () => void }) {
  const [loading, setLoading] = useState(false);
  const handleClick = async () => {
    setLoading(true);
    try {
      if (suspended) {
        await api.resumeAgent(namespace, name);
      } else {
        await api.suspendAgent(namespace, name);
      }
      onSuccess();
    } catch (err) {
      console.error('Failed to update agent suspend state:', err);
    } finally {
      setLoading(false);
    }
  };
  return (
    <button
      onClick={handleClick}
      disabled={loading}
      className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
        suspended
          ? 'bg-emerald-600 text-white hover:bg-emerald-700'
          : 'bg-amber-600 text-white hover:bg-amber-700'
      } disabled:opacity-50`}
    >
      {loading ? '...' : suspended ? 'Resume' : 'Suspend'}
    </button>
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
      <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">Quick Connect</h3>
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

function AgentDetailPage() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();

  const { data: agent, isLoading, error, refetch } = useQuery({
    queryKey: ['agent', namespace, name],
    queryFn: () => api.getAgent(namespace!, name!),
    enabled: !!namespace && !!name,
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
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-display text-xl font-bold text-stone-900">{agent.name}</h2>
              <p className="text-xs text-stone-400 mt-0.5 font-mono">{agent.namespace}</p>
              {agent.profile && (
                <p className="mt-2 text-sm text-stone-500 leading-relaxed">{agent.profile}</p>
              )}
            </div>
            <span className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-medium border ${
              agent.mode === 'Server'
                ? agent.serverStatus?.ready
                  ? 'bg-violet-50 text-violet-600 border-violet-200'
                  : 'bg-amber-50 text-amber-600 border-amber-200'
                : 'bg-stone-50 text-stone-500 border-stone-200'
            }`}>
              <span className={`w-1.5 h-1.5 rounded-full ${
                agent.mode === 'Server'
                  ? agent.serverStatus?.ready ? 'bg-emerald-500' : 'bg-amber-500 animate-pulse'
                  : 'bg-stone-400'
              }`} />
              {agent.mode} Mode{agent.mode === 'Server' && !agent.serverStatus?.ready ? ' (Not Ready)' : ''}
            </span>
          </div>
        </div>

        <div className="px-6 py-5 space-y-6">
          {/* Template Reference */}
          {agent.templateRef && (
            <div>
              <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">Template</h3>
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

          {/* Configuration */}
          <div>
            <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-4">Configuration</h3>
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
              {agent.workspaceDir && (
                <div>
                  <dt className="text-xs text-stone-400">Workspace Directory</dt>
                  <dd className="mt-1 text-sm text-stone-700 font-mono">{agent.workspaceDir}</dd>
                </div>
              )}
              {agent.maxConcurrentTasks && (
                <div>
                  <dt className="text-xs text-stone-400">Max Concurrent Tasks</dt>
                  <dd className="mt-1 text-sm text-stone-700 font-mono">{agent.maxConcurrentTasks}</dd>
                </div>
              )}
            </div>
          </div>

          {/* Labels */}
          {agent.labels && Object.keys(agent.labels).length > 0 && (
            <div>
              <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">Labels</h3>
              <Labels labels={agent.labels} />
            </div>
          )}

          {/* Quota */}
          {agent.quota && (
            <div>
              <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">Quota</h3>
              <div className="bg-stone-50 rounded-lg p-4 border border-stone-100">
                <p className="text-sm text-stone-600">
                  Maximum <span className="font-mono font-medium text-stone-800">{agent.quota.maxTaskStarts}</span> task starts per{' '}
                  <span className="font-mono font-medium text-stone-800">{agent.quota.windowSeconds}</span> seconds
                </p>
              </div>
            </div>
          )}

          {/* Server Status */}
          {agent.mode === 'Server' && (
            <div>
              <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">Server Status</h3>
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
                  <div className="col-span-2">
                    <SuspendResumeButton
                      namespace={agent.namespace}
                      name={agent.name}
                      suspended={agent.serverStatus.suspended}
                      onSuccess={() => refetch()}
                    />
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
          )}

          {/* Terminal Panel (Server mode, ready) */}
          {agent.mode === 'Server' && agent.serverStatus && agent.serverStatus.ready && (
            <TerminalPanel namespace={agent.namespace} agentName={agent.name} />
          )}

          {/* Quick Connect (Server mode only) */}
          {agent.mode === 'Server' && agent.serverStatus && (
            <ServerConnectCommands
              namespace={agent.namespace}
              agentName={agent.name}
            />
          )}

          {/* Conditions */}
          {agent.conditions && agent.conditions.length > 0 && (
            <div>
              <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">Conditions</h3>
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
              <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">
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
              <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">
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

          {/* Create Task CTA */}
          <div className="pt-4 border-t border-stone-100">
            <Link
              to={`/tasks/create?agent=${agent.namespace}/${agent.name}`}
              className="inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium text-white bg-stone-900 rounded-lg hover:bg-stone-800 transition-colors shadow-sm"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M12 5v14M5 12h14" strokeLinecap="round" />
              </svg>
              Create Task with this Agent
            </Link>
          </div>
        </div>
      </div>

      <YamlViewer
        queryKey={['agent', namespace!, name!]}
        fetchYaml={() => api.getAgentYaml(namespace!, name!)}
      />
    </div>
  );
}

export default AgentDetailPage;
