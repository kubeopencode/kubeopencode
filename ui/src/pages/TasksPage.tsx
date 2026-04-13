import React, { useState, useEffect, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import api from '../api/client';
import StatusBadge from '../components/StatusBadge';
import Labels from '../components/Labels';
import TimeAgo from '../components/TimeAgo';
import ResourceFilter from '../components/ResourceFilter';
import MultiSelect from '../components/MultiSelect';
import SortableHeader from '../components/SortableHeader';
import ConfirmDialog from '../components/ConfirmDialog';
import { TableSkeleton } from '../components/Skeleton';
import { useFilterState } from '../hooks/useFilterState';
import { useNamespace } from '../contexts/NamespaceContext';
import { useToast } from '../contexts/ToastContext';
import { LABEL_AGENT, LABEL_AGENT_TEMPLATE, LABEL_CRONTASK, appendLabelSelector } from '../utils/labels';

const PAGE_SIZE_OPTIONS = [10, 20, 50];
const PHASE_OPTIONS = [
  { value: 'Pending', label: 'Pending' },
  { value: 'Queued', label: 'Queued' },
  { value: 'Running', label: 'Running' },
  { value: 'Completed', label: 'Completed' },
  { value: 'Failed', label: 'Failed' },
];

type SourceFilter = '' | 'agent' | 'template';

function TasksPage() {
  const { namespace, isAllNamespaces } = useNamespace();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [phaseFilter, setPhaseFilter] = useState<string[]>([]);
  const [agentFilter, setAgentFilter] = useState<string[]>([]);
  const [cronTaskFilter, setCronTaskFilter] = useState<string[]>([]);
  const [sourceFilter, setSourceFilter] = useState<SourceFilter>('');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc');
  const [filters, setFilters] = useFilterState();
  const [selectedTasks, setSelectedTasks] = useState<Set<string>>(new Set());
  const [batchAction, setBatchAction] = useState<'delete' | 'stop' | null>(null);
  const [batchProcessing, setBatchProcessing] = useState(false);

  useEffect(() => {
    setCurrentPage(1);
    setSelectedTasks(new Set());
  }, [namespace, phaseFilter, agentFilter, cronTaskFilter, sourceFilter, filters.name, filters.labelSelector]);

  // Reset filters when namespace changes
  useEffect(() => {
    setAgentFilter([]);
    setCronTaskFilter([]);
    setSourceFilter('');
  }, [namespace]);

  const { data: agentsData } = useQuery({
    queryKey: ['agents-for-filter', namespace],
    queryFn: () =>
      isAllNamespaces
        ? api.listAllAgents({ limit: 100, sortOrder: 'asc' })
        : api.listAgents(namespace, { limit: 100, sortOrder: 'asc' }),
    staleTime: 60_000,
  });

  const uniqueAgentNames = useMemo(
    () => agentsData ? [...new Set(agentsData.agents.map((a) => a.name))] : [],
    [agentsData]
  );

  const { data: cronTasksData } = useQuery({
    queryKey: ['crontasks-for-filter', namespace],
    queryFn: () =>
      isAllNamespaces
        ? api.listAllCronTasks({ limit: 100, sortOrder: 'asc' })
        : api.listCronTasks(namespace, { limit: 100, sortOrder: 'asc' }),
    staleTime: 60_000,
  });

  const uniqueCronTaskNames = useMemo(
    () => cronTasksData ? [...new Set(cronTasksData.cronTasks.map((ct) => ct.name))] : [],
    [cronTasksData]
  );

  const agentOptions = useMemo(
    () => uniqueAgentNames.map((name) => ({ value: name, label: name })),
    [uniqueAgentNames]
  );

  const cronTaskOptions = useMemo(
    () => uniqueCronTaskNames.map((name) => ({ value: name, label: name })),
    [uniqueCronTaskNames]
  );

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['tasks', namespace, currentPage, pageSize, phaseFilter, agentFilter, cronTaskFilter, sourceFilter, sortOrder, filters.name, filters.labelSelector],
    queryFn: () => {
      let labelSelector = filters.labelSelector || '';
      if (agentFilter.length === 1) {
        labelSelector = appendLabelSelector(labelSelector, `${LABEL_AGENT}=${agentFilter[0]}`);
      } else if (agentFilter.length > 1) {
        labelSelector = appendLabelSelector(labelSelector, `${LABEL_AGENT} in (${agentFilter.join(',')})`);
      }
      if (cronTaskFilter.length === 1) {
        labelSelector = appendLabelSelector(labelSelector, `${LABEL_CRONTASK}=${cronTaskFilter[0]}`);
      } else if (cronTaskFilter.length > 1) {
        labelSelector = appendLabelSelector(labelSelector, `${LABEL_CRONTASK} in (${cronTaskFilter.join(',')})`);
      }
      if (sourceFilter === 'agent') {
        labelSelector = appendLabelSelector(labelSelector, LABEL_AGENT);
      } else if (sourceFilter === 'template') {
        labelSelector = appendLabelSelector(labelSelector, `!${LABEL_AGENT}`);
      }
      const params = {
        limit: pageSize,
        offset: (currentPage - 1) * pageSize,
        sortOrder,
        name: filters.name || undefined,
        labelSelector: labelSelector || undefined,
        phase: phaseFilter.length > 0 ? phaseFilter.join(',') : undefined,
      };
      return isAllNamespaces
        ? api.listAllTasks(params)
        : api.listTasks(namespace, params);
    },
    refetchInterval: (query) => {
      const tasks = query.state.data?.tasks;
      // Poll frequently while tasks are in active states, slow down otherwise
      if (tasks?.some((t) => ['Running', 'Queued', 'Pending'].includes(t.phase))) return 5000;
      return 30000;
    },
  });

  const currentTasks = data?.tasks || [];
  const allSelected = currentTasks.length > 0 && currentTasks.every(t => selectedTasks.has(`${t.namespace}/${t.name}`));

  const toggleTask = (ns: string, name: string) => {
    const key = `${ns}/${name}`;
    setSelectedTasks(prev => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    });
  };

  const toggleAll = () => {
    if (allSelected) {
      setSelectedTasks(new Set());
    } else {
      setSelectedTasks(new Set(currentTasks.map(t => `${t.namespace}/${t.name}`)));
    }
  };

  const executeBatch = async (action: 'delete' | 'stop') => {
    setBatchProcessing(true);
    const items = Array.from(selectedTasks);
    let success = 0;
    let failed = 0;
    for (const key of items) {
      const [ns, ...nameParts] = key.split('/');
      const name = nameParts.join('/');
      try {
        if (action === 'delete') {
          await api.deleteTask(ns, name);
        } else {
          await api.stopTask(ns, name);
        }
        success++;
      } catch {
        failed++;
      }
    }
    setBatchProcessing(false);
    setSelectedTasks(new Set());
    setBatchAction(null);
    queryClient.invalidateQueries({ queryKey: ['tasks'] });
    if (failed === 0) {
      addToast(`${success} task(s) ${action === 'delete' ? 'deleted' : 'stopped'} successfully`, 'success');
    } else {
      addToast(`${success} succeeded, ${failed} failed`, failed > 0 ? 'error' : 'success');
    }
  };

  return (
    <div className="animate-fade-in">
      <div className="sm:flex sm:items-center sm:justify-between mb-6">
        <div>
          <h2 className="font-display text-2xl font-bold text-stone-900 tracking-tight">Tasks</h2>
          <p className="mt-1 text-sm text-stone-500">
            Manage and monitor AI agent tasks
          </p>
        </div>
        <div className="mt-4 sm:mt-0">
          <Link
            to="/tasks/create"
            className="inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 transition-colors shadow-sm"
          >
            <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M12 5v14M5 12h14" strokeLinecap="round" />
            </svg>
            New Task
          </Link>
        </div>
      </div>

      {/* Filter bar */}
      <div className="mb-4">
        <ResourceFilter
          filters={filters}
          onFilterChange={setFilters}
          placeholder="Filter tasks by name..."
        >
          <MultiSelect
            options={PHASE_OPTIONS}
            selected={phaseFilter}
            onChange={setPhaseFilter}
            label="Status"
          />
          <div className="flex items-center gap-1.5">
            <span className="text-xs text-stone-400 font-medium">Source:</span>
            <select
              value={sourceFilter}
              onChange={(e) => setSourceFilter(e.target.value as SourceFilter)}
              className="block w-32 rounded-md border border-stone-200 bg-stone-50 focus:bg-white focus:border-primary-400 focus:ring-1 focus:ring-primary-200 text-xs text-stone-600 py-1.5 transition-colors"
            >
              <option value="">All</option>
              <option value="agent">Agent</option>
              <option value="template">Template</option>
            </select>
          </div>
          {agentOptions.length > 0 && (
            <MultiSelect
              options={agentOptions}
              selected={agentFilter}
              onChange={setAgentFilter}
              label="Agent"
            />
          )}
          {cronTaskOptions.length > 0 && (
            <MultiSelect
              options={cronTaskOptions}
              selected={cronTaskFilter}
              onChange={setCronTaskFilter}
              label="CronTask"
            />
          )}
        </ResourceFilter>
      </div>

      {isLoading ? (
        <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
          <TableSkeleton rows={5} cols={isAllNamespaces ? 7 : 6} />
        </div>
      ) : error ? (
        <div className="bg-red-50 border border-red-200 rounded-xl p-5">
          <p className="text-red-700 text-sm">Error loading tasks: {(error as Error).message}</p>
          <button
            onClick={() => refetch()}
            className="mt-2 text-sm text-red-600 hover:text-red-800 font-medium"
          >
            Retry
          </button>
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
          {/* Batch action bar */}
          {selectedTasks.size > 0 && (
            <div className="bg-primary-50 border-b border-primary-200 px-5 py-2.5 flex items-center justify-between">
              <span className="text-xs font-medium text-primary-700">
                {selectedTasks.size} task(s) selected
              </span>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setBatchAction('stop')}
                  disabled={batchProcessing}
                  className="px-3 py-1.5 text-xs font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-lg hover:bg-amber-100 transition-colors disabled:opacity-50"
                >
                  Stop Selected
                </button>
                <button
                  onClick={() => setBatchAction('delete')}
                  disabled={batchProcessing}
                  className="px-3 py-1.5 text-xs font-medium text-red-600 bg-red-50 border border-red-200 rounded-lg hover:bg-red-100 transition-colors disabled:opacity-50"
                >
                  Delete Selected
                </button>
                <button
                  onClick={() => setSelectedTasks(new Set())}
                  className="px-3 py-1.5 text-xs font-medium text-stone-500 hover:text-stone-700 transition-colors"
                >
                  Clear
                </button>
              </div>
            </div>
          )}

          <table className="min-w-full divide-y divide-stone-100">
            <thead className="bg-stone-50/60">
              <tr>
                <th className="px-3 py-3 w-8">
                  <input
                    type="checkbox"
                    checked={allSelected}
                    onChange={toggleAll}
                    className="rounded border-stone-300 text-primary-600 focus:ring-primary-500"
                  />
                </th>
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider">
                  Name
                </th>
                {isAllNamespaces && (
                  <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider">
                    Namespace
                  </th>
                )}
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider hidden lg:table-cell">
                  Labels
                </th>
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider">
                  Source
                </th>
                <th className="px-5 py-3 text-left text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider hidden sm:table-cell">
                  Duration
                </th>
                <SortableHeader
                  label="Created"
                  active={true}
                  order={sortOrder}
                  onToggle={() => { setSortOrder(o => o === 'desc' ? 'asc' : 'desc'); setCurrentPage(1); }}
                />
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-stone-100">
              {data?.tasks.length === 0 ? (
                <tr>
                  <td colSpan={isAllNamespaces ? 8 : 7} className="px-5 py-12 text-center text-stone-400 text-sm">
                    No tasks found.{' '}
                    {!isAllNamespaces && (
                      <Link to="/tasks/create" className="text-primary-600 hover:text-primary-700 font-medium">
                        Create your first task
                      </Link>
                    )}
                  </td>
                </tr>
              ) : (
                data?.tasks.map((task) => (
                  <tr key={`${task.namespace}/${task.name}`} className={`hover:bg-stone-50/60 transition-colors ${selectedTasks.has(`${task.namespace}/${task.name}`) ? 'bg-primary-50/40' : ''}`}>
                    <td className="px-3 py-3.5 w-8">
                      <input
                        type="checkbox"
                        checked={selectedTasks.has(`${task.namespace}/${task.name}`)}
                        onChange={() => toggleTask(task.namespace, task.name)}
                        className="rounded border-stone-300 text-primary-600 focus:ring-primary-500"
                      />
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap">
                      <Link
                        to={`/tasks/${task.namespace}/${task.name}`}
                        className="text-stone-800 hover:text-primary-600 font-medium text-sm transition-colors"
                      >
                        {task.name}
                      </Link>
                    </td>
                    {isAllNamespaces && (
                      <td className="px-5 py-3.5 whitespace-nowrap text-sm text-stone-400">
                        {task.namespace}
                      </td>
                    )}
                    <td className="px-5 py-3.5 whitespace-nowrap">
                      <StatusBadge phase={task.phase || 'Pending'} />
                    </td>
                    <td className="px-5 py-3.5 hidden lg:table-cell">
                      <Labels labels={task.labels} maxDisplay={2} />
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap text-xs">
                      {task.agentRef ? (
                        <Link to={`/agents/${task.namespace}/${task.agentRef.name}`} className="text-stone-500 hover:text-primary-600 font-mono transition-colors">
                          {task.agentRef.name}
                        </Link>
                      ) : task.templateRef ? (
                        <Link to={`/templates/${task.namespace}/${task.templateRef.name}`} className="text-amber-600 hover:text-amber-700 font-mono transition-colors">
                          {task.templateRef.name}
                        </Link>
                      ) : (
                        <span className="text-stone-400 font-mono">-</span>
                      )}
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap text-sm text-stone-400 hidden sm:table-cell font-mono text-xs">
                      {task.duration || '-'}
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap text-xs text-stone-400">
                      <TimeAgo date={task.createdAt} />
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
                      {Math.min(data.pagination.offset + data.tasks.length, data.pagination.totalCount)}
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

      <ConfirmDialog
        open={batchAction !== null}
        title={batchAction === 'delete' ? 'Delete Tasks' : 'Stop Tasks'}
        message={`Are you sure you want to ${batchAction} ${selectedTasks.size} task(s)? ${batchAction === 'delete' ? 'This action cannot be undone.' : 'Running tasks will be stopped.'}`}
        confirmLabel={batchAction === 'delete' ? 'Delete All' : 'Stop All'}
        variant={batchAction === 'delete' ? 'danger' : 'warning'}
        onConfirm={() => {
          if (batchAction) executeBatch(batchAction);
        }}
        onCancel={() => setBatchAction(null)}
      />
    </div>
  );
}

export default TasksPage;
