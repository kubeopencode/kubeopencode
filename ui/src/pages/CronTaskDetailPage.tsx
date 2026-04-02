import React, { useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import api from '../api/client';
import StatusBadge from '../components/StatusBadge';
import Labels from '../components/Labels';
import TimeAgo from '../components/TimeAgo';
import ConfirmDialog from '../components/ConfirmDialog';
import Breadcrumbs from '../components/Breadcrumbs';
import YamlViewer from '../components/YamlViewer';
import { DetailSkeleton } from '../components/Skeleton';
import { useToast } from '../contexts/ToastContext';

function CronTaskDetailPage() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  const { data: cronTask, isLoading, error } = useQuery({
    queryKey: ['crontask', namespace, name],
    queryFn: () => api.getCronTask(namespace!, name!),
    refetchInterval: 3000,
    enabled: !!namespace && !!name,
  });

  const { data: historyData } = useQuery({
    queryKey: ['crontask-history', namespace, name],
    queryFn: () => api.getCronTaskHistory(namespace!, name!, { limit: 20, sortOrder: 'desc' }),
    refetchInterval: 5000,
    enabled: !!namespace && !!name,
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteCronTask(namespace!, name!),
    onSuccess: () => {
      addToast(`CronTask "${name}" deleted successfully`, 'success');
      queryClient.invalidateQueries({ queryKey: ['crontasks'] });
      navigate('/crontasks');
    },
    onError: (err: Error) => {
      addToast(`Failed to delete CronTask: ${err.message}`, 'error');
    },
  });

  const triggerMutation = useMutation({
    mutationFn: () => api.triggerCronTask(namespace!, name!),
    onSuccess: () => {
      addToast(`CronTask "${name}" triggered successfully`, 'success');
      queryClient.invalidateQueries({ queryKey: ['crontask', namespace, name] });
      queryClient.invalidateQueries({ queryKey: ['crontask-history', namespace, name] });
    },
    onError: (err: Error) => {
      addToast(`Failed to trigger CronTask: ${err.message}`, 'error');
    },
  });

  const suspendMutation = useMutation({
    mutationFn: () => api.suspendCronTask(namespace!, name!),
    onSuccess: () => {
      addToast(`CronTask "${name}" suspended`, 'success');
      queryClient.invalidateQueries({ queryKey: ['crontask', namespace, name] });
    },
    onError: (err: Error) => {
      addToast(`Failed to suspend CronTask: ${err.message}`, 'error');
    },
  });

  const resumeMutation = useMutation({
    mutationFn: () => api.resumeCronTask(namespace!, name!),
    onSuccess: () => {
      addToast(`CronTask "${name}" resumed`, 'success');
      queryClient.invalidateQueries({ queryKey: ['crontask', namespace, name] });
    },
    onError: (err: Error) => {
      addToast(`Failed to resume CronTask: ${err.message}`, 'error');
    },
  });

  if (isLoading) {
    return <DetailSkeleton />;
  }

  if (deleteMutation.isPending || deleteMutation.isSuccess) {
    return (
      <div className="text-center py-16">
        <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-stone-200 border-t-stone-600"></div>
        <p className="mt-3 text-sm text-stone-400">Deleting CronTask...</p>
      </div>
    );
  }

  if (error || !cronTask) {
    const errorMessage = (error as Error)?.message || 'Not found';
    const isNotFound = errorMessage.includes('not found');
    return (
      <div className="bg-red-50 border border-red-200 rounded-xl p-6 animate-fade-in">
        <h3 className="font-display text-base font-semibold text-red-800 mb-2">
          {isNotFound ? 'CronTask Not Found' : 'Error Loading CronTask'}
        </h3>
        <p className="text-sm text-red-600 mb-4">
          {isNotFound
            ? `The CronTask "${name}" in namespace "${namespace}" does not exist.`
            : errorMessage}
        </p>
        <Link
          to="/crontasks"
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-red-700 bg-red-100 rounded-lg hover:bg-red-200 transition-colors"
        >
          Back to CronTasks
        </Link>
      </div>
    );
  }

  const history = historyData?.tasks || [];

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'CronTasks', to: '/crontasks' },
        { label: namespace! },
        { label: name! },
      ]} />

      <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
        <div className="px-6 py-5 border-b border-stone-100">
          <div className="flex items-center justify-between">
            <div>
              <div className="flex items-center gap-2.5">
                <h2 className="font-display text-xl font-bold text-stone-900">{cronTask.name}</h2>
                {cronTask.suspend ? (
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
              </div>
              <p className="text-sm text-stone-400 mt-0.5 font-mono text-xs">{cronTask.namespace}</p>
            </div>
            <div className="flex items-center space-x-2">
              <button
                onClick={() => triggerMutation.mutate()}
                disabled={triggerMutation.isPending}
                className="px-3 py-1.5 text-xs font-medium text-primary-700 bg-primary-50 border border-primary-200 rounded-lg hover:bg-primary-100 transition-colors"
              >
                {triggerMutation.isPending ? 'Triggering...' : 'Run Now'}
              </button>
              {cronTask.suspend ? (
                <button
                  onClick={() => resumeMutation.mutate()}
                  disabled={resumeMutation.isPending}
                  className="px-3 py-1.5 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-lg hover:bg-emerald-100 transition-colors"
                >
                  {resumeMutation.isPending ? 'Resuming...' : 'Resume'}
                </button>
              ) : (
                <button
                  onClick={() => suspendMutation.mutate()}
                  disabled={suspendMutation.isPending}
                  className="px-3 py-1.5 text-xs font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-lg hover:bg-amber-100 transition-colors"
                >
                  {suspendMutation.isPending ? 'Suspending...' : 'Suspend'}
                </button>
              )}
              <button
                onClick={() => setShowDeleteDialog(true)}
                disabled={deleteMutation.isPending}
                className="px-3 py-1.5 text-xs font-medium text-red-600 bg-red-50 border border-red-200 rounded-lg hover:bg-red-100 transition-colors"
              >
                Delete
              </button>
            </div>
          </div>
        </div>

        <div className="px-6 py-5 space-y-5">
          {/* Labels */}
          {cronTask.labels && Object.keys(cronTask.labels).length > 0 && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Labels</h3>
              <Labels labels={cronTask.labels} />
            </div>
          )}

          {/* Agent */}
          {cronTask.taskTemplate.agentRef && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Agent</h3>
              <Link
                to={`/agents/${cronTask.namespace}/${cronTask.taskTemplate.agentRef.name}`}
                className="inline-flex items-center gap-2 bg-violet-50 rounded-lg px-4 py-2.5 border border-violet-200 hover:border-violet-300 transition-colors group"
              >
                <svg className="w-4 h-4 text-violet-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="5" y="11" width="14" height="10" rx="2" />
                  <circle cx="9" cy="16" r="1" />
                  <circle cx="15" cy="16" r="1" />
                  <path d="M9 7L9 4M15 7L15 4M12 7L12 2" />
                </svg>
                <span className="text-sm font-medium text-violet-700 group-hover:text-violet-800">{cronTask.taskTemplate.agentRef.name}</span>
                <svg className="w-3.5 h-3.5 text-violet-400 group-hover:text-violet-600 transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </Link>
            </div>
          )}

          {/* Template */}
          {cronTask.taskTemplate.templateRef && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Template</h3>
              <Link
                to={`/templates/${cronTask.namespace}/${cronTask.taskTemplate.templateRef.name}`}
                className="inline-flex items-center gap-2 bg-teal-50 rounded-lg px-4 py-2.5 border border-teal-200 hover:border-teal-300 transition-colors group"
              >
                <svg className="w-4 h-4 text-teal-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="3" y="3" width="7" height="7" rx="1" />
                  <rect x="14" y="3" width="7" height="7" rx="1" />
                  <rect x="3" y="14" width="7" height="7" rx="1" />
                  <rect x="14" y="14" width="7" height="7" rx="1" />
                </svg>
                <span className="text-sm font-medium text-teal-700 group-hover:text-teal-800">{cronTask.taskTemplate.templateRef.name}</span>
                <svg className="w-3.5 h-3.5 text-teal-400 group-hover:text-teal-600 transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </Link>
            </div>
          )}

          {/* Schedule */}
          <div>
            <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-4">Schedule</h3>
            <div className="grid grid-cols-2 gap-x-6 gap-y-4">
              <div>
                <dt className="text-xs text-stone-400">Cron Expression</dt>
                <dd className="mt-1 text-sm text-stone-800 font-mono">{cronTask.schedule}</dd>
              </div>
              <div>
                <dt className="text-xs text-stone-400">Timezone</dt>
                <dd className="mt-1 text-sm text-stone-800 font-mono">{cronTask.timeZone || 'UTC'}</dd>
              </div>
              <div>
                <dt className="text-xs text-stone-400">Concurrency Policy</dt>
                <dd className="mt-1 text-sm text-stone-800">{cronTask.concurrencyPolicy}</dd>
              </div>
              {cronTask.startingDeadlineSeconds !== undefined && (
                <div>
                  <dt className="text-xs text-stone-400">Starting Deadline</dt>
                  <dd className="mt-1 text-sm text-stone-800 font-mono">{cronTask.startingDeadlineSeconds}s</dd>
                </div>
              )}
            </div>
          </div>

          {/* Execution */}
          <div>
            <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-4">Execution</h3>
            <div className="grid grid-cols-2 gap-x-6 gap-y-4">
              <div>
                <dt className="text-xs text-stone-400">Total Executions</dt>
                <dd className="mt-1 text-sm text-stone-800 font-mono">{cronTask.totalExecutions}</dd>
              </div>
              <div>
                <dt className="text-xs text-stone-400">Running Tasks</dt>
                <dd className="mt-1 text-sm text-stone-800 font-mono">{cronTask.active}</dd>
              </div>
              <div>
                <dt className="text-xs text-stone-400">Last Scheduled</dt>
                <dd className="mt-1 text-sm text-stone-800">
                  {cronTask.lastScheduleTime ? <TimeAgo date={cronTask.lastScheduleTime} /> : '-'}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-stone-400">Next Schedule</dt>
                <dd className="mt-1 text-sm text-stone-800">
                  {cronTask.nextScheduleTime ? <TimeAgo date={cronTask.nextScheduleTime} /> : '-'}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-stone-400">Created</dt>
                <dd className="mt-1 text-sm text-stone-800">
                  <TimeAgo date={cronTask.createdAt} />
                </dd>
              </div>
              {cronTask.maxRetainedTasks !== undefined && cronTask.maxRetainedTasks > 0 && (
                <div>
                  <dt className="text-xs text-stone-400">Max Retained Tasks</dt>
                  <dd className="mt-1 text-sm text-stone-800 font-mono">{cronTask.maxRetainedTasks}</dd>
                </div>
              )}
            </div>
          </div>

          {cronTask.taskTemplate.description && (
            <div>
              <dt className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-2">Description</dt>
              <dd className="bg-stone-50 rounded-lg p-4 border border-stone-100">
                <pre className="text-sm text-stone-700 whitespace-pre-wrap font-body leading-relaxed">{cronTask.taskTemplate.description}</pre>
              </dd>
            </div>
          )}

          {cronTask.conditions && cronTask.conditions.length > 0 && (
            <div>
              <dt className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-2">Conditions</dt>
              <dd className="space-y-2">
                {cronTask.conditions.map((condition, idx) => (
                  <div key={idx} className="bg-stone-50 rounded-lg p-3 border border-stone-100">
                    <div className="flex items-center justify-between">
                      <span className="font-medium text-sm text-stone-800">{condition.type}</span>
                      <span
                        className={`text-[11px] px-2 py-0.5 rounded-md border font-medium ${
                          condition.status === 'True'
                            ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                            : 'bg-stone-50 text-stone-500 border-stone-200'
                        }`}
                      >
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
              </dd>
            </div>
          )}
        </div>
      </div>

      <YamlViewer
        queryKey={['crontask-yaml', namespace!, name!]}
        fetchYaml={() => api.getCronTaskYaml(namespace!, name!)}
      />

      {/* Execution History */}
      <div className="mt-6 bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
        <div className="px-6 py-4 border-b border-stone-100">
          <h3 className="font-display text-base font-semibold text-stone-900">Execution History</h3>
          <p className="text-xs text-stone-400 mt-0.5">Tasks created by this CronTask</p>
        </div>

        {history.length === 0 ? (
          <div className="px-6 py-12 text-center text-stone-400 text-sm">
            No executions yet.
          </div>
        ) : (
          <table className="min-w-full divide-y divide-stone-100">
            <thead className="bg-stone-50/60">
              <tr>
                <th className="px-5 py-3 text-left text-xs font-display font-semibold text-stone-500 uppercase tracking-wider">
                  Name
                </th>
                <th className="px-5 py-3 text-left text-xs font-display font-semibold text-stone-500 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-5 py-3 text-left text-xs font-display font-semibold text-stone-500 uppercase tracking-wider hidden sm:table-cell">
                  Duration
                </th>
                <th className="px-5 py-3 text-left text-xs font-display font-semibold text-stone-500 uppercase tracking-wider">
                  Created
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-stone-100">
              {history.map((task) => (
                <tr key={`${task.namespace}/${task.name}`} className="hover:bg-stone-50/60 transition-colors">
                  <td className="px-5 py-3.5 whitespace-nowrap">
                    <Link
                      to={`/tasks/${task.namespace}/${task.name}`}
                      className="text-stone-800 hover:text-primary-600 font-medium text-sm transition-colors"
                    >
                      {task.name}
                    </Link>
                  </td>
                  <td className="px-5 py-3.5 whitespace-nowrap">
                    <StatusBadge phase={task.phase || 'Pending'} />
                  </td>
                  <td className="px-5 py-3.5 whitespace-nowrap text-sm text-stone-400 hidden sm:table-cell font-mono text-xs">
                    {task.duration || '-'}
                  </td>
                  <td className="px-5 py-3.5 whitespace-nowrap text-xs text-stone-400">
                    <TimeAgo date={task.createdAt} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {historyData?.pagination && historyData.pagination.totalCount > 20 && (
          <div className="px-5 py-3 border-t border-stone-100 text-xs text-stone-400">
            Showing {Math.min(20, historyData.pagination.totalCount)} of {historyData.pagination.totalCount} executions
          </div>
        )}
      </div>

      <ConfirmDialog
        open={showDeleteDialog}
        title="Delete CronTask"
        message={`Are you sure you want to delete CronTask "${name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="danger"
        onConfirm={() => {
          setShowDeleteDialog(false);
          deleteMutation.mutate();
        }}
        onCancel={() => setShowDeleteDialog(false)}
      />
    </div>
  );
}

export default CronTaskDetailPage;
