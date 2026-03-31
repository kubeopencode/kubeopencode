import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import Labels from '../components/Labels';
import Skeleton from '../components/Skeleton';
import ResourceFilter from '../components/ResourceFilter';
import { useFilterState } from '../hooks/useFilterState';
import { getNamespaceCookie, setNamespaceCookie } from '../utils/cookies';

const PAGE_SIZE = 12;

function AgentTemplatesPage() {
  const [selectedNamespace, setSelectedNamespace] = useState<string>(() => {
    return getNamespaceCookie() || '';
  });
  const [currentPage, setCurrentPage] = useState(1);
  const [filters, setFilters] = useFilterState();

  const handleNamespaceChange = (newNamespace: string) => {
    setSelectedNamespace(newNamespace);
    if (newNamespace) {
      setNamespaceCookie(newNamespace);
    }
  };

  useEffect(() => {
    setCurrentPage(1);
  }, [selectedNamespace, filters.name, filters.labelSelector]);

  const { data: namespacesData } = useQuery({
    queryKey: ['namespaces'],
    queryFn: () => api.getNamespaces(),
  });

  const filterParams = {
    name: filters.name || undefined,
    labelSelector: filters.labelSelector || undefined,
    limit: PAGE_SIZE,
    offset: (currentPage - 1) * PAGE_SIZE,
    sortOrder: 'desc' as const,
  };

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['agent-templates', selectedNamespace, currentPage, filters.name, filters.labelSelector],
    queryFn: () =>
      selectedNamespace
        ? api.listAgentTemplates(selectedNamespace, filterParams)
        : api.listAllAgentTemplates(filterParams),
  });

  return (
    <div className="animate-fade-in">
      <div className="sm:flex sm:items-center sm:justify-between mb-6">
        <div>
          <h2 className="font-display text-2xl font-bold text-stone-900 tracking-tight">Agent Templates</h2>
          <p className="mt-1 text-sm text-stone-500">
            Reusable base configurations for creating Agents
          </p>
        </div>
        <div className="mt-4 sm:mt-0">
          <select
            value={selectedNamespace}
            onChange={(e) => handleNamespaceChange(e.target.value)}
            className="block w-full sm:w-48 rounded-lg border-stone-200 bg-white shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700"
          >
            <option value="">All Namespaces</option>
            {namespacesData?.namespaces.map((ns) => (
              <option key={ns} value={ns}>
                {ns}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="mb-4">
        <ResourceFilter
          filters={filters}
          onFilterChange={setFilters}
          placeholder="Filter templates by name..."
        />
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
          <p className="text-red-700 text-sm">Error loading templates: {(error as Error).message}</p>
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
          {data?.templates.length === 0 ? (
            <div className="col-span-full text-center py-16 text-stone-400 text-sm">
              No agent templates found. Templates are created by platform administrators.
            </div>
          ) : (
            data?.templates.map((tmpl) => (
              <Link
                key={`${tmpl.namespace}/${tmpl.name}`}
                to={`/templates/${tmpl.namespace}/${tmpl.name}`}
                className="bg-white rounded-xl border border-stone-200 overflow-hidden hover:border-stone-300 hover:shadow-md transition-all group"
              >
                <div className="p-5">
                  <div className="flex items-start justify-between">
                    <div>
                      <h3 className="text-sm font-semibold text-stone-800 group-hover:text-stone-900">
                        {tmpl.name}
                      </h3>
                      <p className="text-xs text-stone-400 mt-0.5 font-mono">{tmpl.namespace}</p>
                    </div>
                    <span className={`inline-flex items-center px-2 py-0.5 rounded-md text-[10px] font-medium border ${
                      tmpl.hasServerConfig
                        ? 'bg-violet-50 text-violet-600 border-violet-200'
                        : 'bg-stone-50 text-stone-400 border-stone-200'
                    }`}>
                      {tmpl.hasServerConfig ? 'Server' : 'Pod'}
                    </span>
                  </div>

                  <div className="mt-4 space-y-1.5">
                    <div className="flex justify-between text-xs">
                      <span className="text-stone-400">Agents</span>
                      <span className="text-stone-600 font-mono">{tmpl.agentCount}</span>
                    </div>
                    <div className="flex justify-between text-xs">
                      <span className="text-stone-400">Contexts</span>
                      <span className="text-stone-600 font-mono">{tmpl.contextsCount}</span>
                    </div>
                    <div className="flex justify-between text-xs">
                      <span className="text-stone-400">Credentials</span>
                      <span className="text-stone-600 font-mono">{tmpl.credentialsCount}</span>
                    </div>
                    {tmpl.workspaceDir && (
                      <div className="flex justify-between text-xs">
                        <span className="text-stone-400">Workspace</span>
                        <span className="text-stone-600 font-mono text-[11px] truncate max-w-[140px]">
                          {tmpl.workspaceDir}
                        </span>
                      </div>
                    )}
                  </div>

                  {tmpl.labels && Object.keys(tmpl.labels).length > 0 && (
                    <div className="mt-4 pt-3 border-t border-stone-100">
                      <Labels labels={tmpl.labels} maxDisplay={3} />
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
                {Math.min(data.pagination.offset + data.templates.length, data.pagination.totalCount)}
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

export default AgentTemplatesPage;
