import React, { useState } from 'react';
import { useParams, Link, useNavigate, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import { useToast } from '../contexts/ToastContext';
import Labels from '../components/Labels';
import ConfirmDialog from '../components/ConfirmDialog';
import Breadcrumbs from '../components/Breadcrumbs';
import YamlViewer from '../components/YamlViewer';
import { DetailSkeleton } from '../components/Skeleton';

type RegistryTabId = 'overview' | 'yaml';

const REGISTRY_TABS: { id: RegistryTabId; label: string; icon: React.ReactNode }[] = [
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

function PhaseBadge({ phase }: { phase: string }) {
  const isReady = phase === 'Ready';
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className={`w-1.5 h-1.5 rounded-full ${isReady ? 'bg-emerald-500' : 'bg-red-400'}`} />
      <span className={`text-[11px] font-medium ${isReady ? 'text-emerald-700' : 'text-red-600'}`}>
        {isReady ? 'Ready' : 'Unavailable'}
      </span>
    </span>
  );
}

function RegistryDetailPage() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const { addToast } = useToast();
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  const hashTab = location.hash.replace('#', '') as RegistryTabId;
  const initialTab: RegistryTabId = ['overview', 'yaml'].includes(hashTab) ? hashTab : 'overview';
  const [activeTab, setActiveTab] = useState<RegistryTabId>(initialTab);

  const handleTabChange = (tab: RegistryTabId) => {
    setActiveTab(tab);
    window.history.replaceState(null, '', `${location.pathname}#${tab}`);
  };

  const { data: registry, isLoading, error, refetch } = useQuery({
    queryKey: ['registry', namespace, name],
    queryFn: () => api.getRegistry(namespace!, name!),
    enabled: !!namespace && !!name,
  });

  if (isLoading) {
    return <DetailSkeleton />;
  }

  if (error || !registry) {
    const errorMessage = (error as Error)?.message || 'Not found';
    const isNotFound = errorMessage.includes('not found');
    return (
      <div className="bg-red-50 border border-red-200 rounded-xl p-6 animate-fade-in">
        <h3 className="font-display text-base font-semibold text-red-800 mb-2">
          {isNotFound ? 'Registry Not Found' : 'Error Loading Registry'}
        </h3>
        <p className="text-sm text-red-600 mb-4">
          {isNotFound
            ? `The registry "${name}" in namespace "${namespace}" does not exist.`
            : errorMessage}
        </p>
        <Link
          to="/registries"
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-red-700 bg-red-100 rounded-lg hover:bg-red-200 transition-colors"
        >
          Back to Registries
        </Link>
      </div>
    );
  }

  const { summary } = registry;

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'Registries', to: '/registries' },
        { label: namespace!, isNamespace: true },
        { label: name! },
      ]} />

      <div className="bg-white rounded-xl border-0 overflow-hidden shadow-card">
        {/* Header */}
        <div className="px-6 py-5 border-b border-stone-100">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-display text-xl font-bold text-stone-900">{registry.name}</h2>
              <p className="text-xs text-stone-400 mt-0.5 font-mono">{registry.namespace}</p>
            </div>
            <div className="flex items-center gap-2">
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
            {REGISTRY_TABS.map((tab) => {
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
                </button>
              );
            })}
          </nav>
        </div>

        {/* Overview Tab */}
        {activeTab === 'overview' && (
          <div className="px-6 py-5 space-y-6">
            {/* Labels */}
            {registry.labels && Object.keys(registry.labels).length > 0 && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Labels</h3>
                <Labels labels={registry.labels} />
              </div>
            )}

            {/* Summary */}
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-4">Summary</h3>
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                <div className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                  <dt className="text-xs text-stone-400">Images</dt>
                  <dd className="mt-1 text-lg font-semibold text-stone-800 font-mono">{summary.images}</dd>
                </div>
                <div className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                  <dt className="text-xs text-stone-400">Skills</dt>
                  <dd className="mt-1 text-lg font-semibold text-stone-800 font-mono">{summary.skills}</dd>
                </div>
                <div className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                  <dt className="text-xs text-stone-400">Plugins</dt>
                  <dd className="mt-1 text-lg font-semibold text-stone-800 font-mono">{summary.plugins}</dd>
                </div>
                <div className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                  <dt className="text-xs text-stone-400">Ready / Total</dt>
                  <dd className="mt-1 text-lg font-semibold text-stone-800 font-mono">{summary.readyCount}/{summary.totalCount}</dd>
                </div>
              </div>
            </div>

            {/* Images */}
            {registry.images && registry.images.length > 0 && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                  Images ({registry.images.length})
                </h3>
                <div className="space-y-2">
                  {registry.images.map((img, idx) => (
                    <div key={idx} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm text-stone-800">{img.name}</span>
                        <PhaseBadge phase={img.phase} />
                      </div>
                      <p className="mt-1 text-xs text-stone-500 font-mono break-all">{img.image}</p>
                      {img.metadata?.category && (
                        <div className="mt-1.5 flex items-center gap-2">
                          <span className="text-[11px] px-1.5 py-0.5 rounded bg-sky-50 text-sky-600 border border-sky-200 font-medium">
                            {img.metadata.category}
                          </span>
                          {img.metadata.tools && img.metadata.tools.length > 0 && (
                            <span className="text-[11px] text-stone-400">
                              Tools: {img.metadata.tools.join(', ')}
                            </span>
                          )}
                        </div>
                      )}
                      {img.metadata?.description && (
                        <p className="mt-1 text-xs text-stone-400">{img.metadata.description}</p>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Skills */}
            {registry.skills && registry.skills.length > 0 && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                  Skills ({registry.skills.length})
                </h3>
                <div className="space-y-2">
                  {registry.skills.map((skill, idx) => (
                    <div key={idx} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm text-stone-800">{skill.name}</span>
                        <PhaseBadge phase={skill.phase} />
                      </div>
                      {skill.repository && (
                        <p className="mt-1 text-xs text-stone-500 font-mono break-all">{skill.repository}</p>
                      )}
                      {skill.description && (
                        <p className="mt-1 text-xs text-stone-400">{skill.description}</p>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Plugins */}
            {registry.plugins && registry.plugins.length > 0 && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
                  Plugins ({registry.plugins.length})
                </h3>
                <div className="space-y-2">
                  {registry.plugins.map((plugin, idx) => (
                    <div key={idx} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm text-stone-800 font-mono">{plugin.name}</span>
                        <PhaseBadge phase={plugin.phase} />
                      </div>
                      <p className="mt-1 text-xs text-stone-500 font-mono break-all">{plugin.package}</p>
                      {plugin.description && (
                        <p className="mt-1 text-xs text-stone-400">{plugin.description}</p>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Conditions */}
            {registry.conditions && registry.conditions.length > 0 && (
              <div>
                <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Conditions</h3>
                <div className="space-y-2">
                  {registry.conditions.map((condition, idx) => (
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
          </div>
        )}

        {/* YAML Tab */}
        {activeTab === 'yaml' && (
          <div className="p-4">
            <YamlViewer
              queryKey={['registry', namespace!, name!]}
              fetchYaml={() => api.getRegistryYaml(namespace!, name!)}
              onSave={async (yaml) => {
                await api.updateRegistryYaml(namespace!, name!, yaml);
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
        title="Delete Registry"
        message={`Are you sure you want to delete registry "${name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="danger"
        onConfirm={async () => {
          setShowDeleteDialog(false);
          try {
            await api.deleteRegistry(namespace!, name!);
            addToast(`Registry "${name}" deleted`, 'success');
            navigate('/registries');
          } catch (err) {
            addToast(`Failed to delete registry: ${(err as Error).message}`, 'error');
          }
        }}
        onCancel={() => setShowDeleteDialog(false)}
      />
    </div>
  );
}

export default RegistryDetailPage;
