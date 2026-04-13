import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import TimeAgo from '../components/TimeAgo';
import ResourceFilter from '../components/ResourceFilter';
import SortableHeader from '../components/SortableHeader';
import { TableSkeleton } from '../components/Skeleton';
import { useFilterState } from '../hooks/useFilterState';
import { useNamespace } from '../contexts/NamespaceContext';

const PAGE_SIZE_OPTIONS = [10, 20, 50];

function CronTasksPage() {
  const { namespace, isAllNamespaces } = useNamespace();
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc');
  const [filters, setFilters] = useFilterState();

  useEffect(() => {
    setCurrentPage(1);
  }, [namespace, filters.name, filters.labelSelector]);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['crontasks', namespace, currentPage, pageSize, sortOrder, filters.name, filters.labelSelector],
    queryFn: () => {
      const params = {
        limit: pageSize,
        offset: (currentPage - 1) * pageSize,
        sortOrder,
        name: filters.name || undefined,
        labelSelector: filters.labelSelector || undefined,
      };
      return isAllNamespaces
        ? api.listAllCronTasks(params)
        : api.listCronTasks(namespace, params);
    },
    refetchInterval: 30000,
  });

  return (
    <div className="animate-fade-in">
      <div className="sm:flex sm:items-center sm:justify-between mb-6">
        <div>
          <h2 className="font-display text-2xl font-bold text-stone-900 tracking-tight">CronTasks</h2>
          <p className="mt-1 text-sm text-stone-500">
            Scheduled AI agent tasks running on a cron schedule
          </p>
        </div>
        <div className="mt-4 sm:mt-0">
          <Link
            to="/crontasks/create"
            className="inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 transition-colors shadow-sm"
          >
            <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M12 5v14M5 12h14" strokeLinecap="round" />
            </svg>
            New CronTask
          </Link>
        </div>
      </div>

      {/* Filter bar */}
      <div className="mb-4">
        <ResourceFilter
          filters={filters}
          onFilterChange={setFilters}
          placeholder="Filter CronTasks by name..."
        />
      </div>

      {isLoading ? (
        <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
          <TableSkeleton rows={5} cols={isAllNamespaces ? 7 : 6} />
        </div>
      ) : error ? (
        <div className="bg-red-50 border border-red-200 rounded-xl p-5">
          <p className="text-red-700 text-sm">Error loading CronTasks: {(error as Error).message}</p>
          <button
            onClick={() => refetch()}
            className="mt-2 text-sm text-red-600 hover:text-red-800 font-medium"
          >
            Retry
          </button>
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
          <table className="min-w-full divide-y divide-stone-100">
            <thead className="bg-stone-50/60">
              <tr>
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider">
                  Name
                </th>
                {isAllNamespaces && (
                  <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider">
                    Namespace
                  </th>
                )}
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider">
                  Schedule
                </th>
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider">
                  Source
                </th>
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider hidden lg:table-cell">
                  Last Run
                </th>
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider hidden lg:table-cell">
                  Next Run
                </th>
                <SortableHeader
                  label="Age"
                  active={true}
                  order={sortOrder}
                  onToggle={() => { setSortOrder(o => o === 'desc' ? 'asc' : 'desc'); setCurrentPage(1); }}
                />
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-stone-100">
              {data?.cronTasks.length === 0 ? (
                <tr>
                  <td colSpan={isAllNamespaces ? 7 : 6} className="px-5 py-12 text-center text-stone-400 text-sm">
                    No CronTasks found.{' '}
                    {!isAllNamespaces && (
                      <Link to="/crontasks/create" className="text-primary-600 hover:text-primary-700 font-medium">
                        Create your first CronTask
                      </Link>
                    )}
                  </td>
                </tr>
              ) : (
                data?.cronTasks.map((ct) => (
                  <tr key={`${ct.namespace}/${ct.name}`} className="hover:bg-stone-50/60 transition-colors">
                    <td className="px-5 py-3.5 whitespace-nowrap">
                      <Link
                        to={`/crontasks/${ct.namespace}/${ct.name}`}
                        className="text-stone-800 hover:text-primary-600 font-medium text-sm transition-colors"
                      >
                        {ct.name}
                      </Link>
                    </td>
                    {isAllNamespaces && (
                      <td className="px-5 py-3.5 whitespace-nowrap text-sm text-stone-400">
                        {ct.namespace}
                      </td>
                    )}
                    <td className="px-5 py-3.5 whitespace-nowrap text-xs font-mono text-stone-600">
                      {ct.schedule}
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap text-xs">
                      {ct.taskTemplate.agentRef ? (
                        <Link to={`/agents/${ct.namespace}/${ct.taskTemplate.agentRef.name}`} className="text-stone-500 hover:text-primary-600 font-mono transition-colors">
                          {ct.taskTemplate.agentRef.name}
                        </Link>
                      ) : ct.taskTemplate.templateRef ? (
                        <Link to={`/templates/${ct.namespace}/${ct.taskTemplate.templateRef.name}`} className="text-amber-600 hover:text-amber-700 font-mono transition-colors">
                          {ct.taskTemplate.templateRef.name}
                        </Link>
                      ) : (
                        <span className="text-stone-400 font-mono">-</span>
                      )}
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap">
                      {ct.suspend ? (
                        <span className="inline-flex items-center text-xs font-medium text-stone-500">
                          <span className="mr-1.5 inline-flex rounded-full h-1.5 w-1.5 bg-stone-400" />
                          Suspended
                        </span>
                      ) : (
                        <span className="inline-flex items-center text-xs font-medium text-emerald-700">
                          <span className="mr-1.5 inline-flex rounded-full h-1.5 w-1.5 bg-emerald-400" />
                          Enabled
                        </span>
                      )}
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap text-xs text-stone-400 hidden lg:table-cell">
                      {ct.lastScheduleTime ? <TimeAgo date={ct.lastScheduleTime} /> : '-'}
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap text-xs text-stone-400 hidden lg:table-cell">
                      {ct.nextScheduleTime ? <TimeAgo date={ct.nextScheduleTime} /> : '-'}
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap text-xs text-stone-400">
                      <TimeAgo date={ct.createdAt} />
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>

          {/* Pagination */}
          {data?.pagination && data.pagination.totalCount > 0 && (
            <div className="bg-white px-5 py-3 flex items-center justify-between border-t border-stone-100">
              <div className="flex-1 flex justify-between sm:hidden">
                <button
                  onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                  className="px-3 py-1.5 text-sm font-medium text-stone-600 bg-stone-100 rounded-lg hover:bg-stone-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Previous
                </button>
                <button
                  onClick={() => setCurrentPage(p => p + 1)}
                  disabled={!data.pagination.hasMore}
                  className="ml-3 px-3 py-1.5 text-sm font-medium text-stone-600 bg-stone-100 rounded-lg hover:bg-stone-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Next
                </button>
              </div>
              <div className="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
                <div className="flex items-center space-x-4">
                  <p className="text-xs text-stone-400">
                    <span className="font-medium text-stone-600">{data.pagination.offset + 1}</span>
                    {' '}-{' '}
                    <span className="font-medium text-stone-600">
                      {Math.min(data.pagination.offset + data.cronTasks.length, data.pagination.totalCount)}
                    </span>
                    {' '}of{' '}
                    <span className="font-medium text-stone-600">{data.pagination.totalCount}</span>
                  </p>
                  <select
                    value={pageSize}
                    onChange={(e) => {
                      setPageSize(Number(e.target.value));
                      setCurrentPage(1);
                    }}
                    className="block w-16 rounded-lg border-stone-200 text-xs text-stone-600 focus:border-primary-500 focus:ring-primary-500"
                  >
                    {PAGE_SIZE_OPTIONS.map((size) => (
                      <option key={size} value={size}>{size}</option>
                    ))}
                  </select>
                </div>
                <div className="flex items-center space-x-1">
                  <button
                    onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                    disabled={currentPage === 1}
                    className="px-3 py-1.5 text-xs font-medium text-stone-500 bg-stone-50 border border-stone-200 rounded-lg hover:bg-stone-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    Prev
                  </button>
                  <span className="px-3 py-1.5 text-xs font-mono text-stone-500">
                    {currentPage}/{Math.ceil(data.pagination.totalCount / pageSize)}
                  </span>
                  <button
                    onClick={() => setCurrentPage(p => p + 1)}
                    disabled={!data.pagination.hasMore}
                    className="px-3 py-1.5 text-xs font-medium text-stone-500 bg-stone-50 border border-stone-200 rounded-lg hover:bg-stone-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  >
                    Next
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

    </div>
  );
}

export default CronTasksPage;
