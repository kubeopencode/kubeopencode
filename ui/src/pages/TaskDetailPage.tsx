import React, { useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import api from '../api/client';
import StatusBadge from '../components/StatusBadge';
import Labels from '../components/Labels';
import LogViewer from '../components/LogViewer';
import TimeAgo from '../components/TimeAgo';
import ConfirmDialog from '../components/ConfirmDialog';
import Breadcrumbs from '../components/Breadcrumbs';
import YamlViewer from '../components/YamlViewer';
import { DetailSkeleton } from '../components/Skeleton';
import { useToast } from '../contexts/ToastContext';

function TaskDetailPage() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteTask(namespace!, name!),
    onSuccess: () => {
      addToast(`Task "${name}" deleted successfully`, 'success');
      queryClient.invalidateQueries({ queryKey: ['tasks'] });
      navigate('/tasks');
    },
    onError: (err: Error) => {
      addToast(`Failed to delete task: ${err.message}`, 'error');
    },
  });

  const { data: task, isLoading, error } = useQuery({
    queryKey: ['task', namespace, name],
    queryFn: () => api.getTask(namespace!, name!),
    refetchInterval: deleteMutation.isPending ? false : 3000,
    enabled: !!namespace && !!name && !deleteMutation.isSuccess,
  });

  const stopMutation = useMutation({
    mutationFn: () => api.stopTask(namespace!, name!),
    onSuccess: () => {
      addToast(`Task "${name}" stop requested`, 'success');
      queryClient.invalidateQueries({ queryKey: ['task', namespace, name] });
    },
    onError: (err: Error) => {
      addToast(`Failed to stop task: ${err.message}`, 'error');
    },
  });

  if (isLoading) {
    return <DetailSkeleton />;
  }

  if (deleteMutation.isPending || deleteMutation.isSuccess) {
    return (
      <div className="text-center py-16">
        <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-stone-200 border-t-stone-600"></div>
        <p className="mt-3 text-sm text-stone-400">Deleting task...</p>
      </div>
    );
  }

  if (error || !task) {
    const errorMessage = (error as Error)?.message || 'Not found';
    const isNotFound = errorMessage.includes('not found');
    return (
      <div className="bg-red-50 border border-red-200 rounded-xl p-6 animate-fade-in">
        <h3 className="font-display text-base font-semibold text-red-800 mb-2">
          {isNotFound ? 'Task Not Found' : 'Error Loading Task'}
        </h3>
        <p className="text-sm text-red-600 mb-4">
          {isNotFound
            ? `The task "${name}" in namespace "${namespace}" does not exist.`
            : errorMessage}
        </p>
        <Link
          to="/tasks"
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-red-700 bg-red-100 rounded-lg hover:bg-red-200 transition-colors"
        >
          Back to Tasks
        </Link>
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'Tasks', to: '/tasks' },
        { label: namespace! },
        { label: name! },
      ]} />

      <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
        <div className="px-6 py-5 border-b border-stone-100">
          <div className="flex items-center justify-between">
            <div>
              <div className="flex items-center gap-2.5">
                <h2 className="font-display text-xl font-bold text-stone-900">{task.name}</h2>
                <StatusBadge phase={task.phase || 'Pending'} />
              </div>
              <p className="text-sm text-stone-400 mt-0.5 font-mono text-xs">{task.namespace}</p>
            </div>
            <div className="flex items-center space-x-2">
              {task.phase === 'Running' && (
                <button
                  onClick={() => stopMutation.mutate()}
                  disabled={stopMutation.isPending}
                  className="px-3 py-1.5 text-xs font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-lg hover:bg-amber-100 transition-colors"
                >
                  {stopMutation.isPending ? 'Stopping...' : 'Stop'}
                </button>
              )}
              <Link
                to={`/tasks/create?rerun=${name}&namespace=${namespace}`}
                className="px-3 py-1.5 text-xs font-medium text-stone-600 bg-stone-50 border border-stone-200 rounded-lg hover:bg-stone-100 transition-colors"
              >
                Rerun
              </Link>
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
          {task.labels && Object.keys(task.labels).length > 0 && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Labels</h3>
              <Labels labels={task.labels} />
            </div>
          )}

          {/* Agent */}
          {task.agentRef && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Agent</h3>
              <Link
                to={`/agents/${task.namespace}/${task.agentRef.name}`}
                className="inline-flex items-center gap-2 bg-violet-50 rounded-lg px-4 py-2.5 border border-violet-200 hover:border-violet-300 transition-colors group"
              >
                <svg className="w-4 h-4 text-violet-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="5" y="11" width="14" height="10" rx="2" />
                  <circle cx="9" cy="16" r="1" />
                  <circle cx="15" cy="16" r="1" />
                  <path d="M9 7L9 4M15 7L15 4M12 7L12 2" />
                </svg>
                <span className="text-sm font-medium text-violet-700 group-hover:text-violet-800">{task.agentRef.name}</span>
                <svg className="w-3.5 h-3.5 text-violet-400 group-hover:text-violet-600 transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </Link>
            </div>
          )}

          {/* Template */}
          {task.templateRef && (
            <div>
              <h3 className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-3">Template</h3>
              <Link
                to={`/templates/${task.namespace}/${task.templateRef.name}`}
                className="inline-flex items-center gap-2 bg-teal-50 rounded-lg px-4 py-2.5 border border-teal-200 hover:border-teal-300 transition-colors group"
              >
                <svg className="w-4 h-4 text-teal-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="3" y="3" width="7" height="7" rx="1" />
                  <rect x="14" y="3" width="7" height="7" rx="1" />
                  <rect x="3" y="14" width="7" height="7" rx="1" />
                  <rect x="14" y="14" width="7" height="7" rx="1" />
                </svg>
                <span className="text-sm font-medium text-teal-700 group-hover:text-teal-800">{task.templateRef.name}</span>
                <svg className="w-3.5 h-3.5 text-teal-400 group-hover:text-teal-600 transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </Link>
            </div>
          )}

          <div className="grid grid-cols-2 gap-x-6 gap-y-4">
            <div>
              <dt className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider">Duration</dt>
              <dd className="mt-1.5 text-sm text-stone-800 font-mono text-xs">{task.duration || '-'}</dd>
            </div>
            <div>
              <dt className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider">Start Time</dt>
              <dd className="mt-1.5 text-sm text-stone-800">
                {task.startTime ? <TimeAgo date={task.startTime} /> : '-'}
              </dd>
            </div>
            <div>
              <dt className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider">Completion</dt>
              <dd className="mt-1.5 text-sm text-stone-800">
                {task.completionTime ? <TimeAgo date={task.completionTime} /> : '-'}
              </dd>
            </div>
            {task.podName && (
              <div>
                <dt className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider">Pod</dt>
                <dd className="mt-1.5 text-sm text-stone-800 font-mono text-xs">
                  {task.namespace}/{task.podName}
                </dd>
              </div>
            )}
          </div>

          {task.description && (
            <div>
              <dt className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-2">Description</dt>
              <dd className="bg-stone-50 rounded-lg p-4 border border-stone-100">
                <pre className="text-sm text-stone-700 whitespace-pre-wrap font-body leading-relaxed">{task.description}</pre>
              </dd>
            </div>
          )}

          {task.conditions && task.conditions.length > 0 && (
            <div>
              <dt className="text-xs font-display font-semibold text-stone-500 uppercase tracking-wider mb-2">Conditions</dt>
              <dd className="space-y-2">
                {task.conditions.map((condition, idx) => (
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
        queryKey={['task', namespace!, name!]}
        fetchYaml={() => api.getTaskYaml(namespace!, name!)}
      />

      {(task.phase === 'Running' || task.phase === 'Completed' || task.phase === 'Failed') && (
        <div className="mt-6">
          <LogViewer
            namespace={namespace!}
            taskName={name!}
            podName={task.podName}
            isRunning={task.phase === 'Running'}
          />
        </div>
      )}

      <ConfirmDialog
        open={showDeleteDialog}
        title="Delete Task"
        message={`Are you sure you want to delete task "${name}"? This action cannot be undone.`}
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

export default TaskDetailPage;
