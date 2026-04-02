import React, { useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import { useToast } from '../contexts/ToastContext';
import Labels from '../components/Labels';
import Breadcrumbs from '../components/Breadcrumbs';
import YamlViewer from '../components/YamlViewer';
import { DetailSkeleton } from '../components/Skeleton';

function DeleteTemplateButton({ namespace, name }: { namespace: string; name: string }) {
  const [confirming, setConfirming] = useState(false);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const { addToast } = useToast();

  const handleDelete = async () => {
    setLoading(true);
    try {
      await api.deleteAgentTemplate(namespace, name);
      addToast(`Template "${name}" deleted`, 'success');
      navigate('/templates');
    } catch (err) {
      addToast(`Failed to delete template: ${(err as Error).message}`, 'error');
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

function AgentTemplateDetailPage() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();

  const { data: tmpl, isLoading, error, refetch } = useQuery({
    queryKey: ['agent-template', namespace, name],
    queryFn: () => api.getAgentTemplate(namespace!, name!),
    enabled: !!namespace && !!name,
  });

  // Fetch referencing agents via label selector
  const { data: agentsData } = useQuery({
    queryKey: ['template-agents', namespace, name],
    queryFn: () => api.listAgents(namespace!, { labelSelector: `kubeopencode.io/agent-template=${name}` }),
    enabled: !!namespace && !!name,
    refetchInterval: (query) => {
      const agents = query.state.data?.agents;
      // Poll every 5s while any referencing agent is in a transitional state
      if (agents?.some((a) => !a.serverStatus?.suspended && !a.serverStatus?.ready)) return 5000;
      return false;
    },
  });

  const referencingAgents = agentsData?.agents || [];

  if (isLoading) {
    return <DetailSkeleton />;
  }

  if (error || !tmpl) {
    const errorMessage = (error as Error)?.message || 'Not found';
    const isNotFound = errorMessage.includes('not found');
    return (
      <div className="bg-red-50 border border-red-200 rounded-xl p-6 animate-fade-in">
        <h3 className="font-display text-base font-semibold text-red-800 mb-2">
          {isNotFound ? 'Agent Template Not Found' : 'Error Loading Template'}
        </h3>
        <p className="text-sm text-red-600 mb-4">
          {isNotFound
            ? `The template "${name}" in namespace "${namespace}" does not exist.`
            : errorMessage}
        </p>
        <Link
          to="/templates"
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-red-700 bg-red-100 rounded-lg hover:bg-red-200 transition-colors"
        >
          Back to Templates
        </Link>
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'Templates', to: '/templates' },
        { label: namespace! },
        { label: name! },
      ]} />

      <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
        <div className="px-6 py-5 border-b border-stone-100">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-display text-xl font-bold text-stone-900">{tmpl.name}</h2>
              <p className="text-xs text-stone-400 mt-0.5 font-mono">{tmpl.namespace}</p>
            </div>
            <div className="flex items-center gap-2">
              <Link
                to={`/agents/create?template=${tmpl.namespace}/${tmpl.name}`}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 transition-colors"
              >
                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M12 5v14M5 12h14" strokeLinecap="round" />
                </svg>
                Create Agent
              </Link>
              <DeleteTemplateButton namespace={tmpl.namespace} name={tmpl.name} />
            </div>
          </div>
        </div>

        <div className="px-6 py-5 space-y-6">
          {/* Labels */}
          {tmpl.labels && Object.keys(tmpl.labels).length > 0 && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Labels</h3>
              <Labels labels={tmpl.labels} />
            </div>
          )}

          {/* Configuration */}
          <div>
            <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-4">Configuration</h3>
            <div className="grid grid-cols-2 gap-x-6 gap-y-4">
              {tmpl.executorImage && (
                <div>
                  <dt className="text-xs text-stone-400">Executor Image</dt>
                  <dd className="mt-1 text-xs text-stone-700 font-mono bg-stone-50 px-3 py-2 rounded-lg border border-stone-100 break-all">
                    {tmpl.executorImage}
                  </dd>
                </div>
              )}
              {tmpl.agentImage && (
                <div>
                  <dt className="text-xs text-stone-400">Agent Image</dt>
                  <dd className="mt-1 text-xs text-stone-700 font-mono bg-stone-50 px-3 py-2 rounded-lg border border-stone-100 break-all">
                    {tmpl.agentImage}
                  </dd>
                </div>
              )}
              {tmpl.workspaceDir && (
                <div>
                  <dt className="text-xs text-stone-400">Workspace Directory</dt>
                  <dd className="mt-1 text-sm text-stone-700 font-mono">{tmpl.workspaceDir}</dd>
                </div>
              )}
              {tmpl.serviceAccountName && (
                <div>
                  <dt className="text-xs text-stone-400">Service Account</dt>
                  <dd className="mt-1 text-sm text-stone-700 font-mono">{tmpl.serviceAccountName}</dd>
                </div>
              )}
            </div>
          </div>

          {/* Referencing Agents */}
          <div>
            <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
              Referencing Agents ({referencingAgents.length})
            </h3>
            {referencingAgents.length === 0 ? (
              <p className="text-sm text-stone-400">No agents are using this template yet.</p>
            ) : (
              <div className="space-y-2">
                {referencingAgents.map((agent) => (
                  <Link
                    key={`${agent.namespace}/${agent.name}`}
                    to={`/agents/${agent.namespace}/${agent.name}`}
                    className="flex items-center justify-between bg-stone-50 rounded-lg p-3 border border-stone-100 hover:border-stone-300 transition-colors group"
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      <div className="min-w-0">
                        <div>
                          <span className="text-sm font-medium text-stone-800 group-hover:text-stone-900">{agent.name}</span>
                          <span className="text-xs text-stone-400 ml-2">{agent.namespace}</span>
                        </div>
                        {agent.profile && (
                          <p className="text-xs text-stone-400 truncate mt-0.5">{agent.profile}</p>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-2 flex-shrink-0">
                      <span className={`inline-flex items-center text-[11px] font-medium ${
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
                      <svg className="w-4 h-4 text-stone-300 group-hover:text-stone-500 transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </div>
                  </Link>
                ))}
              </div>
            )}
          </div>

          {/* Conditions */}
          {tmpl.conditions && tmpl.conditions.length > 0 && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Conditions</h3>
              <div className="space-y-2">
                {tmpl.conditions.map((condition, idx) => (
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
          {tmpl.credentials && tmpl.credentials.length > 0 && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                Credentials ({tmpl.credentials.length})
              </h3>
              <div className="space-y-2">
                {tmpl.credentials.map((cred, idx) => (
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
          {tmpl.contexts && tmpl.contexts.length > 0 && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                Contexts ({tmpl.contexts.length})
              </h3>
              <div className="space-y-2">
                {tmpl.contexts.map((ctx, idx) => (
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
        queryKey={['agent-template', namespace!, name!]}
        fetchYaml={() => api.getAgentTemplateYaml(namespace!, name!)}
        onSave={async (yaml) => {
          await api.updateAgentTemplateYaml(namespace!, name!, yaml);
          refetch();
        }}
      />
    </div>
  );
}

export default AgentTemplateDetailPage;
