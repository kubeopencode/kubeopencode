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
    refetchInterval: (query) => {
      const agents = query.state.data?.agents;
      // Poll every 5s while any agent is in a transitional state
      if (agents?.some((a) => !a.serverStatus?.suspended && !a.serverStatus?.ready)) return 5000;
      return false;
    },
  });

  const tasks = tasksData?.tasks || [];
  const agents = agentsData?.agents || [];

  const taskStats = {
    total: tasksData?.total || 0,
    running: tasks.filter((t) => t.phase === 'Running').length,
    queued: tasks.filter((t) => t.phase === 'Queued').length,
    completed: tasks.filter((t) => t.phase === 'Completed').length,
    failed: tasks.filter((t) => t.phase === 'Failed').length,
  };

  const statCards = [
    { label: 'Total', value: taskStats.total, color: 'bg-slate-50 border-slate-200 text-slate-700', accent: 'text-slate-900' },
    { label: 'Running', value: taskStats.running, color: 'bg-primary-50/80 border-primary-200 text-primary-700', accent: 'text-primary-900' },
    { label: 'Queued', value: taskStats.queued, color: 'bg-amber-50/80 border-amber-200 text-amber-700', accent: 'text-amber-900' },
    { label: 'Completed', value: taskStats.completed, color: 'bg-emerald-50/80 border-emerald-200 text-emerald-700', accent: 'text-emerald-900' },
    { label: 'Failed', value: taskStats.failed, color: 'bg-red-50/80 border-red-200 text-red-700', accent: 'text-red-900' },
  ];

  const isLoading = tasksLoading || agentsLoading;

  return (
    <div className="space-y-8 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-display text-2xl font-bold text-stone-900 tracking-tight">Dashboard</h1>
          <p className="text-sm text-stone-500 mt-1">Overview of your AI agent tasks</p>
        </div>
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

      {/* Stats */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
        {statCards.map((stat) => (
          <div
            key={stat.label}
            className={`rounded-xl p-4 border ${stat.color} transition-all hover:shadow-sm`}
          >
            <p className="text-xs font-medium uppercase tracking-wider opacity-70">{stat.label}</p>
            <p className={`text-2xl font-display font-bold mt-1 ${stat.accent}`}>
              {isLoading ? '-' : stat.value}
            </p>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Recent Tasks */}
        <div className="lg:col-span-2 bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
          <div className="px-5 py-4 border-b border-stone-100 flex items-center justify-between">
            <h2 className="font-display text-sm font-semibold text-stone-900">Recent Tasks</h2>
            <Link to="/tasks" className="text-xs text-stone-500 hover:text-primary-600 transition-colors font-medium">
              View all
            </Link>
          </div>
          {isLoading ? (
            <div className="divide-y divide-stone-100">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="px-5 py-3.5 flex items-center space-x-4">
                  <div className="animate-pulse bg-stone-100 rounded h-4 w-32" />
                  <div className="animate-pulse bg-stone-100 rounded-full h-5 w-16" />
                  <div className="animate-pulse bg-stone-100 rounded h-4 w-20 ml-auto" />
                </div>
              ))}
            </div>
          ) : tasks.length === 0 ? (
            <div className="px-5 py-12 text-center">
              <p className="text-stone-400 text-sm">No tasks yet.</p>
              <Link to="/tasks/create" className="text-sm text-primary-600 hover:text-primary-700 font-medium mt-1 inline-block">
                Create your first task
              </Link>
            </div>
          ) : (
            <ul className="divide-y divide-stone-100">
              {tasks.slice(0, 8).map((task) => (
                <li key={`${task.namespace}/${task.name}`}>
                  <Link
                    to={`/tasks/${task.namespace}/${task.name}`}
                    className="block px-5 py-3 hover:bg-stone-50/80 transition-colors"
                  >
                    <div className="flex items-center justify-between">
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium text-stone-800 truncate">
                          {task.name}
                        </p>
                        <p className="text-xs text-stone-400 mt-0.5">{task.namespace}</p>
                      </div>
                      <div className="flex items-center space-x-3 ml-4">
                        <StatusBadge phase={task.phase || 'Pending'} />
                        <span className="text-[11px] text-stone-400 whitespace-nowrap">
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
        <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm">
          <div className="px-5 py-4 border-b border-stone-100 flex items-center justify-between">
            <h2 className="font-display text-sm font-semibold text-stone-900">Agents</h2>
            <Link to="/agents" className="text-xs text-stone-500 hover:text-primary-600 transition-colors font-medium">
              View all
            </Link>
          </div>
          {agentsLoading ? (
            <div className="px-5 py-8 text-center text-stone-400 text-sm">Loading...</div>
          ) : agents.length === 0 ? (
            <div className="px-5 py-8 text-center text-stone-400 text-sm">No agents configured</div>
          ) : (
            <ul className="divide-y divide-stone-100">
              {agents.map((agent) => (
                <li key={`${agent.namespace}/${agent.name}`}>
                  <Link
                    to={`/agents/${agent.namespace}/${agent.name}`}
                    className="block px-5 py-3 hover:bg-stone-50/80 transition-colors"
                  >
                    <div className="flex items-center justify-between">
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium text-stone-800 truncate">
                          {agent.name}
                        </p>
                        <p className="text-xs text-stone-400 mt-0.5">{agent.namespace}</p>
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
