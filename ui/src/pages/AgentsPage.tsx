import React, { useState, useEffect, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api, { Agent } from '../api/client';
import Labels from '../components/Labels';
import Skeleton from '../components/Skeleton';
import ResourceFilter from '../components/ResourceFilter';
import MultiSelect from '../components/MultiSelect';
import { useFilterState } from '../hooks/useFilterState';
import { useNamespace } from '../contexts/NamespaceContext';
import { LABEL_AGENT_TEMPLATE, appendLabelSelector } from '../utils/labels';

const PAGE_SIZE = 12;

const STATUS_OPTIONS = [
  { value: 'live', label: 'Live' },
  { value: 'starting', label: 'Starting' },
  { value: 'suspended', label: 'Suspended' },
];

function getAgentStatus(agent: Agent): string {
  if (agent.serverStatus?.suspended) return 'suspended';
  if (agent.serverStatus?.ready) return 'live';
  return 'starting';
}

function AgentsPage() {
  const { namespace, isAllNamespaces } = useNamespace();
  const [currentPage, setCurrentPage] = useState(1);
  const [templateFilter, setTemplateFilter] = useState<string[]>([]);
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [filters, setFilters] = useFilterState();

  useEffect(() => {
    setCurrentPage(1);
  }, [namespace, templateFilter, statusFilter, filters.name, filters.labelSelector]);

  // Reset filters when namespace changes
  useEffect(() => {
    setTemplateFilter([]);
    setStatusFilter([]);
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

  const templateOptions = useMemo(
    () => uniqueTemplateNames.map((name) => ({ value: name, label: name })),
    [uniqueTemplateNames]
  );

  const { data: rawData, isLoading, error, refetch } = useQuery({
    queryKey: ['agents', namespace, currentPage, templateFilter, statusFilter, filters.name, filters.labelSelector],
    queryFn: () => {
      let labelSelector = filters.labelSelector || '';
      if (templateFilter.length === 1) {
        labelSelector = appendLabelSelector(labelSelector, `${LABEL_AGENT_TEMPLATE}=${templateFilter[0]}`);
      } else if (templateFilter.length > 1) {
        labelSelector = appendLabelSelector(labelSelector, `${LABEL_AGENT_TEMPLATE} in (${templateFilter.join(',')})`);
      }
      const params = {
        name: filters.name || undefined,
        labelSelector: labelSelector || undefined,
        limit: 200,
        offset: 0,
        sortOrder: 'desc' as const,
      };
      return isAllNamespaces
        ? api.listAllAgents(params)
        : api.listAgents(namespace, params);
    },
    refetchInterval: (query) => {
      const agents = query.state.data?.agents;
      // Poll every 5s while any agent is in a transitional state (starting/stopping)
      if (agents?.some((a) => !a.serverStatus?.suspended && !a.serverStatus?.ready)) return 5000;
      return false;
    },
  });

  // Client-side status filtering + pagination
  const data = useMemo(() => {
    if (!rawData) return rawData;
    let agents = rawData.agents;
    if (statusFilter.length > 0) {
      agents = agents.filter((a) => statusFilter.includes(getAgentStatus(a)));
    }
    const totalCount = agents.length;
    const start = (currentPage - 1) * PAGE_SIZE;
    const end = Math.min(start + PAGE_SIZE, totalCount);
    return {
      agents: agents.slice(start, end),
      total: totalCount,
      pagination: {
        limit: PAGE_SIZE,
        offset: start,
        totalCount,
        hasMore: end < totalCount,
      },
    };
  }, [rawData, statusFilter, currentPage]);

  return (
    <div className="animate-fade-in">
      <div className="sm:flex sm:items-center sm:justify-between mb-6">
        <div>
          <h2 className="font-display text-2xl font-bold text-stone-900 tracking-tight">Agents</h2>
          <p className="mt-1 text-sm text-stone-500">
            Browse available AI agents for task execution
          </p>
        </div>
        <Link
          to="/agents/create"
          className="inline-flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 transition-colors shadow-sm"
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M12 5v14M5 12h14" strokeLinecap="round" />
          </svg>
          Create Agent
        </Link>
      </div>

      <div className="mb-4">
        <ResourceFilter
          filters={filters}
          onFilterChange={setFilters}
          placeholder="Filter agents by name..."
        >
          <MultiSelect
            options={STATUS_OPTIONS}
            selected={statusFilter}
            onChange={setStatusFilter}
            label="Status"
          />
          {templateOptions.length > 0 && (
            <MultiSelect
              options={templateOptions}
              selected={templateFilter}
              onChange={setTemplateFilter}
              label="Template"
            />
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
