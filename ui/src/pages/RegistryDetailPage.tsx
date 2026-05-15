import React, { useState } from 'react';
import { useParams, Link, useNavigate, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api, { RegistryImageInfo } from '../api/client';
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

function ImageGroups({ images }: { images: RegistryImageInfo[] }) {
  const executorImages = images.filter(
    (img) => (img.metadata?.category || '').toLowerCase() === 'executor',
  );
  const agentImages = images.filter(
    (img) => (img.metadata?.category || '').toLowerCase() === 'agent',
  );
  const uncategorized = images.filter(
    (img) => !(img.metadata?.category || '').toLowerCase(),
  );
  const groups: { label: string; items: RegistryImageInfo[] }[] = [];
  if (executorImages.length > 0) groups.push({ label: 'Executor Images', items: executorImages });
  if (agentImages.length > 0) groups.push({ label: 'Agent Images', items: agentImages });
  if (uncategorized.length > 0) groups.push({ label: 'Images', items: uncategorized });

  if (groups.length === 0) return null;

  return (
    <div>
      {groups.map((group) => (
        <div key={group.label} className="mb-6">
          <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">
            {group.label} ({group.items.length})
          </h3>
          <div className="space-y-2">
            {group.items.map((img) => (
              <div key={img.name} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm text-stone-800">{img.name}</span>
                    {img.metadata?.category && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-blue-50 text-blue-600 border border-blue-200 font-medium">
                        {img.metadata.category}
                      </span>
                    )}
                  </div>
                  <PhaseBadge phase={img.phase} />
                </div>
                <p className="mt-1 text-xs text-stone-500 font-mono break-all">{img.image}</p>
                {img.metadata?.description && (
                  <p className="mt-1.5 text-xs text-stone-400">{img.metadata.description}</p>
                )}
                {img.metadata?.tools && img.metadata.tools.length > 0 && (
                  <div className="mt-2 flex flex-wrap gap-1">
                    {img.metadata.tools.map((tool) => (
                      <span
                        key={tool}
                        className="text-[10px] px-1.5 py-0.5 rounded bg-stone-100 text-stone-500 border border-stone-200"
                      >
                        {tool}
                      </span>
                    ))}
                  </div>
                )}
                {img.message && img.phase !== 'Ready' && (
                  <p className="mt-1.5 text-[11px] text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1">
                    {img.message}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

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
              <Link
                to={`/registries/${namespace}/${name}/assemble`}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-white bg-primary-600 hover:bg-primary-700 transition-colors shadow-sm"
              >
                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M12 5v14M5 12h14" />
                </svg>
                Assemble
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
              <ImageGroups images={registry.images} />
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
                      {skill.message && skill.phase !== 'Ready' && (
                        <p className="mt-1.5 text-[11px] text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1">
                          {skill.message}
                        </p>
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
                      {plugin.message && plugin.phase !== 'Ready' && (
                        <p className="mt-1.5 text-[11px] text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1">
                          {plugin.message}
                        </p>
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
