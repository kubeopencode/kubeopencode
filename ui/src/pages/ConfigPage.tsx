import React from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import api from '../api/client';
import Labels from '../components/Labels';
import Breadcrumbs from '../components/Breadcrumbs';
import YamlViewer from '../components/YamlViewer';
import CopyButton from '../components/CopyButton';
import { DetailSkeleton } from '../components/Skeleton';

const CONFIG_TEMPLATE = `apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster
spec:
  cleanup:
    ttlSecondsAfterFinished: 3600
    maxRetainedTasks: 100`;

function ConfigPage() {
  const queryClient = useQueryClient();
  const { data: config, isLoading, error } = useQuery({
    queryKey: ['config'],
    queryFn: () => api.getConfig(),
  });

  if (isLoading) {
    return <DetailSkeleton />;
  }

  if (error || !config) {
    const errorMessage = (error as Error)?.message || 'Not found';
    const isNotFound = errorMessage.includes('not found') || errorMessage.includes('404');
    return (
      <div className="animate-fade-in">
        <Breadcrumbs items={[{ label: 'Config' }]} />
        <div className="bg-amber-50 border border-amber-200 rounded-xl p-6">
          <h3 className="font-display text-base font-semibold text-amber-800 mb-2">
            {isNotFound ? 'No Configuration Found' : 'Error Loading Configuration'}
          </h3>
          <p className="text-sm text-amber-600">
            {isNotFound
              ? 'No KubeOpenCodeConfig resource exists yet. Create a cluster-scoped resource named "cluster" to configure system-wide settings.'
              : errorMessage}
          </p>
          {isNotFound && (
            <div className="mt-4">
              <div className="flex items-center justify-between mb-2">
                <p className="text-xs font-medium text-amber-700">Apply this YAML to get started:</p>
                <CopyButton text={CONFIG_TEMPLATE} label="Copy YAML template" />
              </div>
              <pre className="text-xs text-amber-700 bg-amber-100/50 rounded-lg p-4 font-mono overflow-x-auto">
{CONFIG_TEMPLATE}
              </pre>
              <p className="mt-3 text-xs text-amber-600">
                Run: <code className="bg-amber-100/50 px-1.5 py-0.5 rounded font-mono">kubectl apply -f config.yaml</code>
              </p>
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[{ label: 'Config' }]} />

      <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
        <div className="px-6 py-5 border-b border-stone-100">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-display text-xl font-bold text-stone-900">Cluster Configuration</h2>
              <p className="text-xs text-stone-400 mt-0.5">System-wide settings for KubeOpenCode</p>
            </div>
            <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-medium border bg-sky-50 text-sky-600 border-sky-200">
              Cluster-scoped
            </span>
          </div>
        </div>

        <div className="px-6 py-5 space-y-6">
          {/* System Image */}
          <div>
            <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-4">
              System Image
            </h3>
            {config.systemImage ? (
              <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                {config.systemImage.image && (
                  <div>
                    <dt className="text-xs text-stone-400">Image</dt>
                    <dd className="mt-1 text-xs text-stone-700 font-mono bg-stone-50 px-3 py-2 rounded-lg border border-stone-100 break-all">
                      {config.systemImage.image}
                    </dd>
                  </div>
                )}
                {config.systemImage.imagePullPolicy && (
                  <div>
                    <dt className="text-xs text-stone-400">Pull Policy</dt>
                    <dd className="mt-1 text-sm text-stone-700">{config.systemImage.imagePullPolicy}</dd>
                  </div>
                )}
              </div>
            ) : (
              <p className="text-sm text-stone-400">Using default system image</p>
            )}
          </div>

          {/* Cleanup */}
          <div>
            <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-4">
              Task Cleanup
            </h3>
            {config.cleanup ? (
              <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                <div>
                  <dt className="text-xs text-stone-400">TTL After Finished</dt>
                  <dd className="mt-1 text-sm text-stone-700">
                    {config.cleanup.ttlSecondsAfterFinished != null
                      ? formatDuration(config.cleanup.ttlSecondsAfterFinished)
                      : <span className="text-stone-400">Not set</span>}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-stone-400">Max Retained Tasks (per namespace)</dt>
                  <dd className="mt-1 text-sm text-stone-700">
                    {config.cleanup.maxRetainedTasks != null
                      ? config.cleanup.maxRetainedTasks
                      : <span className="text-stone-400">Not set</span>}
                  </dd>
                </div>
              </div>
            ) : (
              <p className="text-sm text-stone-400">No automatic cleanup configured</p>
            )}
          </div>

          {/* Proxy */}
          <div>
            <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-4">
              Proxy
            </h3>
            {config.proxy ? (
              <div className="space-y-3">
                {config.proxy.httpProxy && (
                  <div>
                    <dt className="text-xs text-stone-400">HTTP Proxy</dt>
                    <dd className="mt-1 text-xs text-stone-700 font-mono bg-stone-50 px-3 py-2 rounded-lg border border-stone-100">
                      {config.proxy.httpProxy}
                    </dd>
                  </div>
                )}
                {config.proxy.httpsProxy && (
                  <div>
                    <dt className="text-xs text-stone-400">HTTPS Proxy</dt>
                    <dd className="mt-1 text-xs text-stone-700 font-mono bg-stone-50 px-3 py-2 rounded-lg border border-stone-100">
                      {config.proxy.httpsProxy}
                    </dd>
                  </div>
                )}
                {config.proxy.noProxy && (
                  <div>
                    <dt className="text-xs text-stone-400">No Proxy</dt>
                    <dd className="mt-1 text-xs text-stone-700 font-mono bg-stone-50 px-3 py-2 rounded-lg border border-stone-100 break-all">
                      {config.proxy.noProxy}
                    </dd>
                  </div>
                )}
              </div>
            ) : (
              <p className="text-sm text-stone-400">No proxy configured</p>
            )}
          </div>

          {/* Labels */}
          {config.labels && Object.keys(config.labels).length > 0 && (
            <div>
              <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">Labels</h3>
              <Labels labels={config.labels} />
            </div>
          )}
        </div>
      </div>

      <YamlViewer
        queryKey={['config-yaml']}
        fetchYaml={() => api.getConfigYaml()}
        onSave={async (yaml) => {
          await api.updateConfigYaml(yaml);
          queryClient.invalidateQueries({ queryKey: ['config'] });
        }}
      />
    </div>
  );
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86400) {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return m > 0 ? `${h}h ${m}m` : `${h}h`;
  }
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  return h > 0 ? `${d}d ${h}h` : `${d}d`;
}

export default ConfigPage;
