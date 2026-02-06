import React, { useState, useEffect } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import StatusBadge from '../components/StatusBadge';
import Labels from '../components/Labels';
import TimeAgo from '../components/TimeAgo';
import ResourceFilter from '../components/ResourceFilter';
import { TableSkeleton } from '../components/Skeleton';
import { useFilterState } from '../hooks/useFilterState';
import { getNamespaceCookie, setNamespaceCookie } from '../utils/cookies';

const PAGE_SIZE_OPTIONS = [10, 20, 50];
const ALL_NAMESPACES = '__all__';
const PHASE_OPTIONS = ['', 'Pending', 'Queued', 'Running', 'Completed', 'Failed'];

function TasksPage() {
  const [searchParams] = useSearchParams();
  // Initialize namespace: URL param > cookie > default
  const [namespace, setNamespace] = useState(() => {
    const urlParam = new URLSearchParams(window.location.search).get('namespace');
    if (urlParam) return urlParam;
    return getNamespaceCookie() || 'default';
  });
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [phaseFilter, setPhaseFilter] = useState('');
  const [filters, setFilters] = useFilterState();

  // Sync namespace from URL params when they change
  useEffect(() => {
    const namespaceParam = searchParams.get('namespace');
    if (namespaceParam && namespaceParam !== namespace) {
      setNamespace(namespaceParam);
      if (namespaceParam !== ALL_NAMESPACES) {
        setNamespaceCookie(namespaceParam);
      }
    }
  }, [searchParams, namespace]);

  // Handler for namespace dropdown change
  const handleNamespaceChange = (newNamespace: string) => {
    setNamespace(newNamespace);
    if (newNamespace !== ALL_NAMESPACES) {
      setNamespaceCookie(newNamespace);
    }
  };

  // Reset to page 1 when namespace or filters change
  useEffect(() => {
    setCurrentPage(1);
  }, [namespace, phaseFilter, filters.name, filters.labelSelector]);

  const { data: namespacesData } = useQuery({
    queryKey: ['namespaces'],
    queryFn: () => api.getNamespaces(),
  });

  const isAllNamespaces = namespace === ALL_NAMESPACES;

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['tasks', namespace, currentPage, pageSize, phaseFilter, filters.name, filters.labelSelector],
    queryFn: () => {
      const params = {
        limit: pageSize,
        offset: (currentPage - 1) * pageSize,
        sortOrder: 'desc' as const,
        name: filters.name || undefined,
        labelSelector: filters.labelSelector || undefined,
        phase: phaseFilter || undefined,
      };
      return isAllNamespaces
        ? api.listAllTasks(params)
        : api.listTasks(namespace, params);
    },
    refetchInterval: 5000,
  });

  return (
    <div>
      <div className="sm:flex sm:items-center sm:justify-between mb-6">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">Tasks</h2>
          <p className="mt-1 text-sm text-gray-500">
            Manage and monitor AI agent tasks
          </p>
        </div>
        <div className="mt-4 sm:mt-0 flex items-center space-x-4">
          <select
            value={namespace}
            onChange={(e) => handleNamespaceChange(e.target.value)}
            className="block w-48 rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm"
          >
            <option value={ALL_NAMESPACES}>All Namespaces</option>
            {namespacesData?.namespaces.map((ns) => (
              <option key={ns} value={ns}>
                {ns}
              </option>
            ))}
          </select>
          <Link
            to={`/tasks/create?namespace=${namespace}`}
            className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-primary-600 hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-500"
          >
            New Task
          </Link>
        </div>
      </div>

      {/* Filter bar */}
      <div className="mb-4 space-y-3">
        <ResourceFilter
          filters={filters}
          onFilterChange={setFilters}
          placeholder="Filter tasks by name..."
        />
        <div className="flex items-center space-x-2">
          <span className="text-sm text-gray-500">Phase:</span>
          {PHASE_OPTIONS.map((phase) => (
            <button
              key={phase || 'all'}
              onClick={() => setPhaseFilter(phase)}
              className={`px-3 py-1 text-xs font-medium rounded-full border ${
                phaseFilter === phase
                  ? 'bg-primary-100 text-primary-800 border-primary-300'
                  : 'bg-white text-gray-600 border-gray-300 hover:bg-gray-50'
              }`}
            >
              {phase || 'All'}
            </button>
          ))}
        </div>
      </div>

      {isLoading ? (
        <div className="bg-white shadow-sm rounded-lg overflow-hidden">
          <TableSkeleton rows={5} cols={isAllNamespaces ? 7 : 6} />
        </div>
      ) : error ? (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
          <p className="text-red-800">Error loading tasks: {(error as Error).message}</p>
          <button
            onClick={() => refetch()}
            className="mt-2 text-sm text-red-600 hover:text-red-800"
          >
            Retry
          </button>
        </div>
      ) : (
        <div className="bg-white shadow-sm rounded-lg overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Name
                </th>
                {isAllNamespaces && (
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Namespace
                  </th>
                )}
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider hidden lg:table-cell">
                  Labels
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Agent
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider hidden sm:table-cell">
                  Duration
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Created
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {data?.tasks.length === 0 ? (
                <tr>
                  <td colSpan={isAllNamespaces ? 7 : 6} className="px-6 py-12 text-center text-gray-500">
                    No tasks found.{' '}
                    {!isAllNamespaces && (
                      <Link to={`/tasks/create?namespace=${namespace}`} className="text-primary-600 hover:text-primary-800">
                        Create your first task
                      </Link>
                    )}
                  </td>
                </tr>
              ) : (
                data?.tasks.map((task) => (
                  <tr key={`${task.namespace}/${task.name}`} className="hover:bg-gray-50">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <Link
                        to={`/tasks/${task.namespace}/${task.name}`}
                        className="text-primary-600 hover:text-primary-800 font-medium"
                      >
                        {task.name}
                      </Link>
                    </td>
                    {isAllNamespaces && (
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {task.namespace}
                      </td>
                    )}
                    <td className="px-6 py-4 whitespace-nowrap">
                      <StatusBadge phase={task.phase || 'Pending'} />
                    </td>
                    <td className="px-6 py-4 hidden lg:table-cell">
                      <Labels labels={task.labels} maxDisplay={2} />
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {task.agentRef?.name || 'default'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 hidden sm:table-cell">
                      {task.duration || '-'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      <TimeAgo date={task.createdAt} />
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>

          {/* Pagination Controls */}
          {data?.pagination && data.pagination.totalCount > 0 && (
            <div className="bg-white px-4 py-3 flex items-center justify-between border-t border-gray-200 sm:px-6">
              <div className="flex-1 flex justify-between sm:hidden">
                <button
                  onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                  className="relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Previous
                </button>
                <button
                  onClick={() => setCurrentPage(p => p + 1)}
                  disabled={!data.pagination.hasMore}
                  className="ml-3 relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Next
                </button>
              </div>
              <div className="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
                <div className="flex items-center space-x-4">
                  <p className="text-sm text-gray-700">
                    Showing{' '}
                    <span className="font-medium">{data.pagination.offset + 1}</span>
                    {' '}to{' '}
                    <span className="font-medium">
                      {Math.min(data.pagination.offset + data.tasks.length, data.pagination.totalCount)}
                    </span>
                    {' '}of{' '}
                    <span className="font-medium">{data.pagination.totalCount}</span>
                    {' '}results
                  </p>
                  <select
                    value={pageSize}
                    onChange={(e) => {
                      setPageSize(Number(e.target.value));
                      setCurrentPage(1);
                    }}
                    className="block w-20 rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm"
                  >
                    {PAGE_SIZE_OPTIONS.map((size) => (
                      <option key={size} value={size}>{size}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <nav className="relative z-0 inline-flex rounded-md shadow-sm -space-x-px">
                    <button
                      onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                      disabled={currentPage === 1}
                      className="relative inline-flex items-center px-3 py-2 rounded-l-md border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      Previous
                    </button>
                    <span className="relative inline-flex items-center px-4 py-2 border border-gray-300 bg-white text-sm font-medium text-gray-700">
                      Page {currentPage} of {Math.ceil(data.pagination.totalCount / pageSize)}
                    </span>
                    <button
                      onClick={() => setCurrentPage(p => p + 1)}
                      disabled={!data.pagination.hasMore}
                      className="relative inline-flex items-center px-3 py-2 rounded-r-md border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      Next
                    </button>
                  </nav>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default TasksPage;
