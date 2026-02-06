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
      navigate(`/tasks?namespace=${namespace}`);
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

  // If delete is in progress or succeeded, don't show error - navigation will happen
  if (deleteMutation.isPending || deleteMutation.isSuccess) {
    return (
      <div className="text-center py-12">
        <div className="inline-block animate-spin rounded-full h-8 w-8 border-4 border-gray-300 border-t-primary-600"></div>
        <p className="mt-4 text-gray-500">Deleting task...</p>
      </div>
    );
  }

  if (error || !task) {
    const errorMessage = (error as Error)?.message || 'Not found';
    const isNotFound = errorMessage.includes('not found');
    return (
      <div className="bg-red-50 border border-red-200 rounded-lg p-6">
        <h3 className="text-lg font-medium text-red-800 mb-2">
          {isNotFound ? 'Task Not Found' : 'Error Loading Task'}
        </h3>
        <p className="text-red-700 mb-4">
          {isNotFound
            ? `The task "${name}" in namespace "${namespace}" does not exist. It may have been deleted.`
            : errorMessage}
        </p>
        <Link
          to={`/tasks?namespace=${namespace}`}
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-red-700 bg-red-100 rounded-md hover:bg-red-200"
        >
          Back to Tasks
        </Link>
      </div>
    );
  }

  return (
    <div>
      <Breadcrumbs items={[
        { label: 'Tasks', to: `/tasks?namespace=${namespace}` },
        { label: namespace!, to: `/tasks?namespace=${namespace}` },
        { label: name! },
      ]} />

      <div className="bg-white shadow-sm rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-xl font-bold text-gray-900">{task.name}</h2>
              <p className="text-sm text-gray-500">{task.namespace}</p>
            </div>
            <div className="flex items-center space-x-4">
              <StatusBadge phase={task.phase || 'Pending'} />
              {task.phase === 'Running' && (
                <button
                  onClick={() => stopMutation.mutate()}
                  disabled={stopMutation.isPending}
                  className="px-3 py-1 text-sm font-medium text-yellow-700 bg-yellow-100 rounded-md hover:bg-yellow-200"
                >
                  {stopMutation.isPending ? 'Stopping...' : 'Stop'}
                </button>
              )}
              <Link
                to={`/tasks/create?namespace=${namespace}&rerun=${name}`}
                className="px-3 py-1 text-sm font-medium text-primary-700 bg-primary-100 rounded-md hover:bg-primary-200"
              >
                Rerun
              </Link>
              <button
                onClick={() => setShowDeleteDialog(true)}
                disabled={deleteMutation.isPending}
                className="px-3 py-1 text-sm font-medium text-red-700 bg-red-100 rounded-md hover:bg-red-200"
              >
                {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>

        <div className="px-6 py-4 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <dt className="text-sm font-medium text-gray-500">Agent</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {task.agentRef ? (
                  <Link
                    to={`/agents/${task.agentRef.namespace || task.namespace}/${task.agentRef.name}`}
                    className="text-primary-600 hover:text-primary-800"
                  >
                    {task.agentRef.name}
                  </Link>
                ) : (
                  'default'
                )}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Duration</dt>
              <dd className="mt-1 text-sm text-gray-900">{task.duration || '-'}</dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Start Time</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {task.startTime ? <TimeAgo date={task.startTime} /> : '-'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Completion Time</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {task.completionTime ? <TimeAgo date={task.completionTime} /> : '-'}
              </dd>
            </div>
            {task.podName && (
              <div>
                <dt className="text-sm font-medium text-gray-500">Pod</dt>
                <dd className="mt-1 text-sm text-gray-900">
                  {task.podNamespace}/{task.podName}
                </dd>
              </div>
            )}
            {task.labels && Object.keys(task.labels).length > 0 && (
              <div className="col-span-2">
                <dt className="text-sm font-medium text-gray-500">Labels</dt>
                <dd className="mt-1">
                  <Labels labels={task.labels} />
                </dd>
              </div>
            )}
          </div>

          {task.description && (
            <div>
              <dt className="text-sm font-medium text-gray-500 mb-2">Description</dt>
              <dd className="bg-gray-50 rounded-md p-4">
                <pre className="text-sm text-gray-900 whitespace-pre-wrap">{task.description}</pre>
              </dd>
            </div>
          )}

          {task.conditions && task.conditions.length > 0 && (
            <div>
              <dt className="text-sm font-medium text-gray-500 mb-2">Conditions</dt>
              <dd className="space-y-2">
                {task.conditions.map((condition, idx) => (
                  <div key={idx} className="bg-gray-50 rounded-md p-3">
                    <div className="flex items-center justify-between">
                      <span className="font-medium text-gray-900">{condition.type}</span>
                      <span
                        className={`text-xs px-2 py-1 rounded ${
                          condition.status === 'True'
                            ? 'bg-green-100 text-green-800'
                            : 'bg-gray-100 text-gray-800'
                        }`}
                      >
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
              </dd>
            </div>
          )}
        </div>
      </div>

      <YamlViewer
        queryKey={['task', namespace!, name!]}
        fetchYaml={() => api.getTaskYaml(namespace!, name!)}
      />

      {/* Log Viewer - show when task has a pod */}
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
