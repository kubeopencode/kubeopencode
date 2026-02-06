import React from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import Labels from '../components/Labels';
import Breadcrumbs from '../components/Breadcrumbs';
import YamlViewer from '../components/YamlViewer';
import { DetailSkeleton } from '../components/Skeleton';

function AgentDetailPage() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();

  const { data: agent, isLoading, error } = useQuery({
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
      <div className="bg-red-50 border border-red-200 rounded-lg p-6">
        <h3 className="text-lg font-medium text-red-800 mb-2">
          {isNotFound ? 'Agent Not Found' : 'Error Loading Agent'}
        </h3>
        <p className="text-red-700 mb-4">
          {isNotFound
            ? `The agent "${name}" in namespace "${namespace}" does not exist. It may have been deleted.`
            : errorMessage}
        </p>
        <Link
          to="/agents"
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-red-700 bg-red-100 rounded-md hover:bg-red-200"
        >
          &larr; Back to Agents
        </Link>
      </div>
    );
  }

  return (
    <div>
      <Breadcrumbs items={[
        { label: 'Agents', to: '/agents' },
        { label: namespace! },
        { label: name! },
      ]} />

      <div className="bg-white shadow-sm rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-xl font-bold text-gray-900">{agent.name}</h2>
              <p className="text-sm text-gray-500">{agent.namespace}</p>
            </div>
            <span className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${
              agent.mode === 'Server'
                ? 'bg-purple-100 text-purple-800'
                : 'bg-gray-100 text-gray-800'
            }`}>
              {agent.mode} Mode
            </span>
          </div>
        </div>

        <div className="px-6 py-4 space-y-6">
          {/* Basic Info */}
          <div>
            <h3 className="text-lg font-medium text-gray-900 mb-4">Configuration</h3>
            <div className="grid grid-cols-2 gap-4">
              {agent.executorImage && (
                <div>
                  <dt className="text-sm font-medium text-gray-500">Executor Image</dt>
                  <dd className="mt-1 text-sm text-gray-900 font-mono bg-gray-50 px-2 py-1 rounded">
                    {agent.executorImage}
                  </dd>
                </div>
              )}
              {agent.agentImage && (
                <div>
                  <dt className="text-sm font-medium text-gray-500">Agent Image</dt>
                  <dd className="mt-1 text-sm text-gray-900 font-mono bg-gray-50 px-2 py-1 rounded">
                    {agent.agentImage}
                  </dd>
                </div>
              )}
              {agent.workspaceDir && (
                <div>
                  <dt className="text-sm font-medium text-gray-500">Workspace Directory</dt>
                  <dd className="mt-1 text-sm text-gray-900 font-mono">
                    {agent.workspaceDir}
                  </dd>
                </div>
              )}
              {agent.maxConcurrentTasks && (
                <div>
                  <dt className="text-sm font-medium text-gray-500">Max Concurrent Tasks</dt>
                  <dd className="mt-1 text-sm text-gray-900">{agent.maxConcurrentTasks}</dd>
                </div>
              )}
            </div>
          </div>

          {/* Labels */}
          {agent.labels && Object.keys(agent.labels).length > 0 && (
            <div>
              <h3 className="text-lg font-medium text-gray-900 mb-4">Labels</h3>
              <Labels labels={agent.labels} />
            </div>
          )}

          {/* Quota */}
          {agent.quota && (
            <div>
              <h3 className="text-lg font-medium text-gray-900 mb-4">Quota</h3>
              <div className="bg-gray-50 rounded-md p-4">
                <p className="text-sm text-gray-700">
                  Maximum {agent.quota.maxTaskStarts} task starts per{' '}
                  {agent.quota.windowSeconds} seconds
                </p>
              </div>
            </div>
          )}

          {/* Server Status (Server mode only) */}
          {agent.serverStatus && (
            <div>
              <h3 className="text-lg font-medium text-gray-900 mb-4">Server Status</h3>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <dt className="text-sm font-medium text-gray-500">Deployment</dt>
                  <dd className="mt-1 text-sm text-gray-900 font-mono">{agent.serverStatus.deploymentName}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">Service</dt>
                  <dd className="mt-1 text-sm text-gray-900 font-mono">{agent.serverStatus.serviceName}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">URL</dt>
                  <dd className="mt-1 text-sm text-gray-900 font-mono break-all">{agent.serverStatus.url}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">Ready Replicas</dt>
                  <dd className="mt-1 text-sm text-gray-900">{agent.serverStatus.readyReplicas}</dd>
                </div>
              </div>
            </div>
          )}

          {/* Conditions */}
          {agent.conditions && agent.conditions.length > 0 && (
            <div>
              <h3 className="text-lg font-medium text-gray-900 mb-4">Conditions</h3>
              <div className="space-y-2">
                {agent.conditions.map((condition, idx) => (
                  <div key={idx} className="bg-gray-50 rounded-md p-3">
                    <div className="flex items-center justify-between">
                      <span className="font-medium text-gray-900">{condition.type}</span>
                      <span className={`text-xs px-2 py-1 rounded ${
                        condition.status === 'True'
                          ? 'bg-green-100 text-green-800'
                          : 'bg-gray-100 text-gray-800'
                      }`}>
                        {condition.status}
                      </span>
                    </div>
                    {condition.reason && (
                      <p className="text-sm text-gray-600 mt-1">Reason: {condition.reason}</p>
                    )}
                    {condition.message && (
                      <p className="text-sm text-gray-500 mt-1">{condition.message}</p>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Allowed Namespaces */}
          {agent.allowedNamespaces && agent.allowedNamespaces.length > 0 && (
            <div>
              <h3 className="text-lg font-medium text-gray-900 mb-4">
                Allowed Namespaces
              </h3>
              <div className="flex flex-wrap gap-2">
                {agent.allowedNamespaces.map((ns) => (
                  <span
                    key={ns}
                    className="inline-flex items-center px-3 py-1 rounded-full text-sm bg-gray-100 text-gray-800"
                  >
                    {ns}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Credentials */}
          {agent.credentials && agent.credentials.length > 0 && (
            <div>
              <h3 className="text-lg font-medium text-gray-900 mb-4">
                Credentials ({agent.credentials.length})
              </h3>
              <div className="space-y-2">
                {agent.credentials.map((cred, idx) => (
                  <div key={idx} className="bg-gray-50 rounded-md p-3">
                    <div className="flex items-center justify-between">
                      <span className="font-medium text-gray-900">{cred.name}</span>
                      <span className="text-xs text-gray-500">
                        Secret: {cred.secretRef}
                      </span>
                    </div>
                    {(cred.env || cred.mountPath) && (
                      <div className="mt-1 text-sm text-gray-600">
                        {cred.env && <span>ENV: {cred.env}</span>}
                        {cred.mountPath && <span>Mount: {cred.mountPath}</span>}
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
              <h3 className="text-lg font-medium text-gray-900 mb-4">
                Contexts ({agent.contexts.length})
              </h3>
              <div className="space-y-2">
                {agent.contexts.map((ctx, idx) => (
                  <div key={idx} className="bg-gray-50 rounded-md p-3">
                    <div className="flex items-center justify-between">
                      <span className="font-medium text-gray-900">
                        {ctx.name || `Context ${idx + 1}`}
                      </span>
                      <span className="text-xs px-2 py-1 rounded bg-blue-100 text-blue-800">
                        {ctx.type}
                      </span>
                    </div>
                    {ctx.description && (
                      <p className="mt-1 text-sm text-gray-600">{ctx.description}</p>
                    )}
                    {ctx.mountPath && (
                      <p className="mt-1 text-xs text-gray-500 font-mono">
                        Mount: {ctx.mountPath}
                      </p>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Create Task CTA */}
          <div className="pt-6 border-t border-gray-200">
            <Link
              to={`/tasks/create?agent=${agent.namespace}/${agent.name}`}
              className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-primary-600 hover:bg-primary-700"
            >
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
