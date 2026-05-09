import React, { useState } from 'react';
import { useParams, Link, useNavigate, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import { useToast } from '../contexts/ToastContext';
import Labels from '../components/Labels';
import AgentStatusBadge from '../components/AgentStatusBadge';
import ConfirmDialog from '../components/ConfirmDialog';
import Breadcrumbs from '../components/Breadcrumbs';
import YamlViewer from '../components/YamlViewer';
import { DetailSkeleton } from '../components/Skeleton';

type TemplateTabId = 'overview' | 'agents' | 'yaml';

const TEMPLATE_TABS: { id: TemplateTabId; label: string; icon: React.ReactNode }[] = [
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
    id: 'agents',
    label: 'Agents',
    icon: (
      <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect x="5" y="11" width="14" height="10" rx="2" />
        <circle cx="9" cy="16" r="1" />
        <circle cx="15" cy="16" r="1" />
        <path d="M9 7L9 4M15 7L15 4M12 7L12 2" />
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

function AgentTemplateDetailPage() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const { addToast } = useToast();
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  const hashTab = location.hash.replace('#', '') as TemplateTabId;
  const initialTab: TemplateTabId = ['overview', 'agents', 'yaml'].includes(hashTab) ? hashTab : 'overview';
  const [activeTab, setActiveTab] = useState<TemplateTabId>(initialTab);

  const handleTabChange = (tab: TemplateTabId) => {
    setActiveTab(tab);
    window.history.replaceState(null, '', `${location.pathname}#${tab}`);
  };

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
        { label: namespace!, isNamespace: true },
        { label: name! },
      ]} />

      <div className="bg-white rounded-xl border-0 overflow-hidden shadow-card">
        {/* Header */}
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
            {TEMPLATE_TABS.map((tab) => {
              const isActive = activeTab === tab.id;
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
                  {tab.id === 'agents' && referencingAgents.length > 0 && (
                    <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${
                      isActive ? 'bg-primary-100 text-primary-600' : 'bg-stone-200 text-stone-500'
                    }`}>
                      {referencingAgents.length}
                    </span>
                  )}
                </button>
              );
            })}
          </nav>
        </div>

        {/* Overview Tab */}
        {activeTab === 'overview' && (
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

            {/* Plugins */}
            {tmpl.plugins && tmpl.plugins.length > 0 && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                  Plugins ({tmpl.plugins.length})
                </h3>
                <div className="space-y-2">
                  {tmpl.plugins.map((plugin, idx) => (
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
            {tmpl.config && Object.keys(tmpl.config).length > 0 && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                  OpenCode Config
                </h3>
                <pre className="text-xs text-stone-600 font-mono bg-stone-50 rounded-lg p-4 border border-stone-100 overflow-x-auto">
                  {JSON.stringify(tmpl.config, null, 2)}
                </pre>
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

        {/* Agents Tab */}
        {activeTab === 'agents' && (
          <div className="px-6 py-5">
            {referencingAgents.length === 0 ? (
              <div className="text-center py-8">
                <svg className="w-10 h-10 text-stone-300 mx-auto mb-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="5" y="11" width="14" height="10" rx="2" />
                  <circle cx="9" cy="16" r="1" />
                  <circle cx="15" cy="16" r="1" />
                  <path d="M9 7L9 4M15 7L15 4M12 7L12 2" />
                </svg>
                <p className="text-sm text-stone-400">No agents are using this template yet.</p>
                <Link
                  to={`/agents/create?template=${tmpl.namespace}/${tmpl.name}`}
                  className="inline-flex items-center gap-1.5 mt-3 px-3 py-1.5 text-xs font-medium text-primary-600 bg-primary-50 rounded-lg hover:bg-primary-100 transition-colors"
                >
                  Create Agent from this Template
                </Link>
              </div>
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
                      <AgentStatusBadge
                        suspended={agent.serverStatus?.suspended}
                        ready={agent.serverStatus?.ready}
                      />
                      <svg className="w-4 h-4 text-stone-300 group-hover:text-stone-500 transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </div>
                  </Link>
                ))}
              </div>
            )}
          </div>
        )}

        {/* YAML Tab */}
        {activeTab === 'yaml' && (
          <div className="p-4">
            <YamlViewer
              queryKey={['agent-template', namespace!, name!]}
              fetchYaml={() => api.getAgentTemplateYaml(namespace!, name!)}
              onSave={async (yaml) => {
                await api.updateAgentTemplateYaml(namespace!, name!, yaml);
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
        title="Delete Template"
        message={`Are you sure you want to delete template "${name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="danger"
        onConfirm={async () => {
          setShowDeleteDialog(false);
          try {
            await api.deleteAgentTemplate(namespace!, name!);
            addToast(`Template "${name}" deleted`, 'success');
            navigate('/templates');
          } catch (err) {
            addToast(`Failed to delete template: ${(err as Error).message}`, 'error');
          }
        }}
        onCancel={() => setShowDeleteDialog(false)}
      />
    </div>
  );
}

export default AgentTemplateDetailPage;
