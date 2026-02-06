import React from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import StatusBadge from '../components/StatusBadge';
import { DashboardSkeleton } from '../components/Skeleton';
import TimeAgo from '../components/TimeAgo';

function DashboardPage() {
  const { data: tasksData, isLoading: tasksLoading } = useQuery({
    queryKey: ['dashboard-tasks'],
    queryFn: () => api.listAllTasks({ limit: 10 }),
    refetchInterval: 5000,
  });

  const { data: agentsData, isLoading: agentsLoading } = useQuery({
    queryKey: ['dashboard-agents'],
    queryFn: () => api.listAllAgents({ limit: 100 }),
  });

  const tasks = tasksData?.tasks || [];
  const agents = agentsData?.agents || [];

  // Compute task stats
  const taskStats = {
    total: tasksData?.total || 0,
    running: tasks.filter((t) => t.phase === 'Running').length,
    queued: tasks.filter((t) => t.phase === 'Queued').length,
    completed: tasks.filter((t) => t.phase === 'Completed').length,
    failed: tasks.filter((t) => t.phase === 'Failed').length,
  };

  const statCards = [
    { label: 'Total Tasks', value: taskStats.total, color: 'bg-gray-100 text-gray-900' },
    { label: 'Running', value: taskStats.running, color: 'bg-blue-100 text-blue-900' },
    { label: 'Queued', value: taskStats.queued, color: 'bg-yellow-100 text-yellow-900' },
    { label: 'Completed', value: taskStats.completed, color: 'bg-green-100 text-green-900' },
    { label: 'Failed', value: taskStats.failed, color: 'bg-red-100 text-red-900' },
  ];

  const isLoading = tasksLoading || agentsLoading;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <Link
          to="/tasks/create"
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-white bg-primary-600 rounded-md hover:bg-primary-700"
        >
          + New Task
        </Link>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4">
        {statCards.map((stat) => (
          <div
            key={stat.label}
            className={`rounded-lg p-4 ${stat.color}`}
          >
            <p className="text-sm font-medium opacity-75">{stat.label}</p>
            <p className="text-2xl font-bold mt-1">
              {isLoading ? '-' : stat.value}
            </p>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Recent Tasks */}
        <div className="lg:col-span-2 bg-white shadow-sm rounded-lg overflow-hidden">
          <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-gray-900">Recent Tasks</h2>
            <Link to="/tasks" className="text-sm text-primary-600 hover:text-primary-800">
              View all
            </Link>
          </div>
          {isLoading ? (
            <div className="divide-y divide-gray-200">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="px-6 py-4 flex items-center space-x-4">
                  <div className="animate-pulse bg-gray-200 rounded h-4 w-32" />
                  <div className="animate-pulse bg-gray-200 rounded h-6 w-16" />
                  <div className="animate-pulse bg-gray-200 rounded h-4 w-20 ml-auto" />
                </div>
              ))}
            </div>
          ) : tasks.length === 0 ? (
            <div className="px-6 py-8 text-center text-gray-500">
              No tasks yet.{' '}
              <Link to="/tasks/create" className="text-primary-600 hover:text-primary-800">
                Create one
              </Link>
            </div>
          ) : (
            <ul className="divide-y divide-gray-200">
              {tasks.slice(0, 8).map((task) => (
                <li key={`${task.namespace}/${task.name}`}>
                  <Link
                    to={`/tasks/${task.namespace}/${task.name}`}
                    className="block px-6 py-3 hover:bg-gray-50"
                  >
                    <div className="flex items-center justify-between">
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium text-gray-900 truncate">
                          {task.name}
                        </p>
                        <p className="text-xs text-gray-500">{task.namespace}</p>
                      </div>
                      <div className="flex items-center space-x-3 ml-4">
                        <StatusBadge phase={task.phase || 'Pending'} />
                        <span className="text-xs text-gray-400 whitespace-nowrap">
                          <TimeAgo date={task.createdAt} />
                        </span>
                      </div>
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Agents */}
        <div className="bg-white shadow-sm rounded-lg overflow-hidden">
          <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-gray-900">Agents</h2>
            <Link to="/agents" className="text-sm text-primary-600 hover:text-primary-800">
              View all
            </Link>
          </div>
          {agentsLoading ? (
            <div className="px-6 py-8 text-center text-gray-500">Loading...</div>
          ) : agents.length === 0 ? (
            <div className="px-6 py-8 text-center text-gray-500">No agents configured</div>
          ) : (
            <ul className="divide-y divide-gray-200">
              {agents.map((agent) => (
                <li key={`${agent.namespace}/${agent.name}`}>
                  <Link
                    to={`/agents/${agent.namespace}/${agent.name}`}
                    className="block px-6 py-3 hover:bg-gray-50"
                  >
                    <div className="flex items-center justify-between">
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium text-gray-900 truncate">
                          {agent.name}
                        </p>
                        <p className="text-xs text-gray-500">{agent.namespace}</p>
                      </div>
                      <span
                        className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                          agent.mode === 'Server'
                            ? 'bg-purple-100 text-purple-800'
                            : 'bg-gray-100 text-gray-800'
                        }`}
                      >
                        {agent.mode}
                      </span>
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  );
}

export default DashboardPage;
