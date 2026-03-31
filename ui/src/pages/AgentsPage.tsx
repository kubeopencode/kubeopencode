import React, { useState, useEffect, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import Labels from '../components/Labels';
import Skeleton from '../components/Skeleton';
import ResourceFilter from '../components/ResourceFilter';
import { useFilterState } from '../hooks/useFilterState';
import { useNamespace } from '../contexts/NamespaceContext';
import { LABEL_AGENT_TEMPLATE, FILTER_HAS_TEMPLATE, FILTER_NO_TEMPLATE, appendLabelSelector } from '../utils/labels';

const PAGE_SIZE = 12;

function AgentsPage() {
  const { namespace, isAllNamespaces } = useNamespace();
  const [currentPage, setCurrentPage] = useState(1);
  const [templateFilter, setTemplateFilter] = useState('');
  const [filters, setFilters] = useFilterState();

  useEffect(() => {
    setCurrentPage(1);
  }, [namespace, templateFilter, filters.name, filters.labelSelector]);

  // Reset template filter when namespace changes
  useEffect(() => {
    setTemplateFilter('');
  }, [namespace]);

  const { data: templatesData } = useQuery({
    queryKey: ['templates-for-filter', namespace],
    queryFn: () =>
      isAllNamespaces
        ? api.listAllAgentTemplates({ limit: 100, sortOrder: 'asc' })
        : api.listAgentTemplates(namespace, { limit: 100, sortOrder: 'asc' }),
    staleTime: 60_000,
  });

  const uniqueTemplateNames = useMemo(
    () => templatesData ? [...new Set(templatesData.templates.map((t) => t.name))] : [],
    [templatesData]
  );

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['agents', namespace, currentPage, templateFilter, filters.name, filters.labelSelector],
    queryFn: () => {
      let labelSelector = filters.labelSelector || '';
      if (templateFilter === FILTER_HAS_TEMPLATE) {
        labelSelector = appendLabelSelector(labelSelector, LABEL_AGENT_TEMPLATE);
      } else if (templateFilter === FILTER_NO_TEMPLATE) {
        labelSelector = appendLabelSelector(labelSelector, `!${LABEL_AGENT_TEMPLATE}`);
      } else if (templateFilter) {
        labelSelector = appendLabelSelector(labelSelector, `${LABEL_AGENT_TEMPLATE}=${templateFilter}`);
      }
      const params = {
        name: filters.name || undefined,
        labelSelector: labelSelector || undefined,
        limit: PAGE_SIZE,
        offset: (currentPage - 1) * PAGE_SIZE,
        sortOrder: 'desc' as const,
      };
      return isAllNamespaces
        ? api.listAllAgents(params)
        : api.listAgents(namespace, params);
    },
  });

  return (
    <div className="animate-fade-in">
      <div className="sm:flex sm:items-center sm:justify-between mb-6">
        <div>
          <h2 className="font-display text-2xl font-bold text-stone-900 tracking-tight">Agents</h2>
          <p className="mt-1 text-sm text-stone-500">
            Browse available AI agents for task execution
          </p>
        </div>
      </div>

      <div className="mb-4">
        <ResourceFilter
          filters={filters}
          onFilterChange={setFilters}
          placeholder="Filter agents by name..."
        >
          {uniqueTemplateNames.length > 0 && (
            <div className="flex items-center gap-1.5">
              <span className="text-xs text-stone-400 font-medium">Template:</span>
              <select
                value={templateFilter}
                onChange={(e) => setTemplateFilter(e.target.value)}
                className="block w-40 rounded-md border border-stone-200 bg-stone-50 focus:bg-white focus:border-primary-400 focus:ring-1 focus:ring-primary-200 text-xs text-stone-600 py-1.5 transition-colors"
              >
                <option value="">All</option>
                <option value={FILTER_HAS_TEMPLATE}>Has Template</option>
                <option value={FILTER_NO_TEMPLATE}>No Template</option>
                {uniqueTemplateNames.map((name) => (
                  <option key={name} value={name}>{name}</option>
                ))}
              </select>
            </div>
          )}
        </ResourceFilter>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="bg-white rounded-xl border border-stone-200 p-5">
              <Skeleton className="h-5 w-32 mb-2" />
              <Skeleton className="h-3 w-20 mb-4" />
              <Skeleton className="h-3 w-full mb-2" />
              <Skeleton className="h-3 w-full mb-2" />
              <Skeleton className="h-3 w-3/4" />
            </div>
          ))}
        </div>
      ) : error ? (
        <div className="bg-red-50 border border-red-200 rounded-xl p-5">
          <p className="text-red-700 text-sm">Error loading agents: {(error as Error).message}</p>
          <button
            onClick={() => refetch()}
            className="mt-2 text-sm text-red-600 hover:text-red-800 font-medium"
          >
            Retry
          </button>
        </div>
      ) : (
        <>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {data?.agents.length === 0 ? (
            <div className="col-span-full text-center py-16 text-stone-400 text-sm">
              No agents found. Agents are created by platform administrators.
            </div>
          ) : (
            data?.agents.map((agent) => (
              <Link
                key={`${agent.namespace}/${agent.name}`}
                to={`/agents/${agent.namespace}/${agent.name}`}
                className="bg-white rounded-xl border border-stone-200 overflow-hidden hover:border-stone-300 hover:shadow-md transition-all group"
              >
                <div className="p-5">
                  <div className="flex items-start justify-between">
                    <div>
                      <h3 className="text-sm font-semibold text-stone-800 group-hover:text-stone-900">
                        {agent.name}
                      </h3>
                      <p className="text-xs text-stone-400 mt-0.5 font-mono">{agent.namespace}</p>
                    </div>
                    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium border ${
                      agent.serverStatus?.suspended
                        ? 'bg-amber-50 text-amber-600 border-amber-200'
                        : agent.mode === 'Server'
                          ? agent.serverStatus?.ready
                            ? 'bg-emerald-50 text-emerald-600 border-emerald-200'
                            : 'bg-violet-50 text-violet-600 border-violet-200'
                          : 'bg-stone-50 text-stone-400 border-stone-200'
                    }`}>
                      {agent.mode === 'Server' && !agent.serverStatus?.suspended && (
                        <span className={`w-1.5 h-1.5 rounded-full ${
                          agent.serverStatus?.ready ? 'bg-emerald-500' : 'bg-amber-500 animate-pulse'
                        }`} />
                      )}
                      {agent.serverStatus?.suspended ? 'Suspended' : agent.mode === 'Server' ? (agent.serverStatus?.ready ? 'Live' : 'Starting') : agent.mode}
                    </span>
                  </div>

                  {agent.profile && (
                    <p className="mt-2.5 text-xs text-stone-500 line-clamp-2 leading-relaxed">{agent.profile}</p>
                  )}

                  <div className="mt-4 space-y-1.5">
                    {agent.templateRef && (
                      <div className="flex justify-between text-xs">
                        <span className="text-stone-400">Template</span>
                        <span className="text-primary-600 font-mono text-[11px] truncate max-w-[140px]">
                          {agent.templateRef.name}
                        </span>
                      </div>
                    )}
                    {agent.maxConcurrentTasks && (
                      <div className="flex justify-between text-xs">
                        <span className="text-stone-400">Concurrency</span>
                        <span className="text-stone-600 font-mono">{agent.maxConcurrentTasks}</span>
                      </div>
                    )}
                    <div className="flex justify-between text-xs">
                      <span className="text-stone-400">Contexts</span>
                      <span className="text-stone-600 font-mono">{agent.contextsCount}</span>
                    </div>
                    <div className="flex justify-between text-xs">
                      <span className="text-stone-400">Credentials</span>
                      <span className="text-stone-600 font-mono">{agent.credentialsCount}</span>
                    </div>
                    {agent.workspaceDir && (
                      <div className="flex justify-between text-xs">
                        <span className="text-stone-400">Workspace</span>
                        <span className="text-stone-600 font-mono text-[11px] truncate max-w-[140px]">
                          {agent.workspaceDir}
                        </span>
                      </div>
                    )}
                  </div>

                  {agent.labels && Object.keys(agent.labels).length > 0 && (
                    <div className="mt-4 pt-3 border-t border-stone-100">
                      <Labels labels={agent.labels} maxDisplay={3} />
                    </div>
                  )}
                </div>
              </Link>
            ))
          )}
        </div>

        {/* Pagination */}
        {data?.pagination && data.pagination.totalCount > 0 && (
          <div className="mt-6 flex items-center justify-between">
            <p className="text-xs text-stone-400">
              <span className="font-medium text-stone-600">{data.pagination.offset + 1}</span>
              {' '}-{' '}
              <span className="font-medium text-stone-600">
                {Math.min(data.pagination.offset + data.agents.length, data.pagination.totalCount)}
              </span>
              {' '}of{' '}
              <span className="font-medium text-stone-600">{data.pagination.totalCount}</span>
            </p>
            <div className="flex space-x-1">
              <button
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                disabled={currentPage === 1}
                className="px-3 py-1.5 text-xs font-medium text-stone-500 bg-stone-50 border border-stone-200 rounded-lg hover:bg-stone-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Previous
              </button>
              <button
                onClick={() => setCurrentPage((p) => p + 1)}
                disabled={!data.pagination.hasMore}
                className="px-3 py-1.5 text-xs font-medium text-stone-500 bg-stone-50 border border-stone-200 rounded-lg hover:bg-stone-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Next
              </button>
            </div>
          </div>
        )}
        </>
      )}
    </div>
  );
}

export default AgentsPage;
