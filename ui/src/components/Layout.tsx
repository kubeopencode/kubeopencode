import React, { useState } from 'react';
import { Outlet, NavLink, Link, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import StatusBadge from './StatusBadge';
import TimeAgo from './TimeAgo';
import ToastContainer from './ToastContainer';
import logoImg from '../assets/logo.png';

function useServerVersion() {
  const { data } = useQuery({
    queryKey: ['server-info'],
    queryFn: () => api.getInfo(),
    staleTime: 5 * 60 * 1000,
  });
  return data?.version || '';
}

function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [tasksExpanded, setTasksExpanded] = useState(true);
  const [agentsExpanded, setAgentsExpanded] = useState(true);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const location = useLocation();
  const serverVersion = useServerVersion();

  const { data: tasksData } = useQuery({
    queryKey: ['sidebar-tasks'],
    queryFn: () => api.listAllTasks({ limit: 20, sortOrder: 'desc' }),
    refetchInterval: 5000,
  });

  const { data: agentsData } = useQuery({
    queryKey: ['sidebar-agents'],
    queryFn: () => api.listAllAgents({ limit: 20 }),
  });

  const tasks = tasksData?.tasks || [];
  const agents = agentsData?.agents || [];
  const runningCount = tasks.filter(t => t.phase === 'Running').length;

  const isActiveRoute = (path: string) => location.pathname === path;

  const sidebarContent = (
    <div className="flex flex-col h-full">
      {/* Logo / Brand */}
      <div className="flex items-center justify-between px-4 h-14 border-b border-sidebar-border flex-shrink-0">
        <NavLink to="/" className="flex items-center gap-2.5 group">
          <img src={logoImg} alt="KubeOpenCode" className="w-7 h-7 rounded-lg" />
          <span className="font-display font-semibold text-sm text-sidebar-text tracking-tight">
            KubeOpenCode
          </span>
        </NavLink>
        <button
          onClick={() => setSidebarOpen(!sidebarOpen)}
          className="hidden lg:flex items-center justify-center w-6 h-6 rounded text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover transition-colors"
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M11 19l-7-7 7-7M17 19l-7-7 7-7" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
      </div>

      {/* New Task Button */}
      <div className="px-3 pt-3 pb-1 flex-shrink-0">
        <Link
          to="/tasks/create"
          className="flex items-center gap-2 w-full px-3 py-2.5 rounded-lg bg-primary-500 text-white hover:bg-primary-600 transition-all text-sm font-medium group shadow-sm"
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M12 5v14M5 12h14" strokeLinecap="round" />
          </svg>
          New Task
        </Link>
      </div>

      {/* Navigation */}
      <div className="px-3 pt-2 pb-1 flex-shrink-0">
        <NavLink
          to="/"
          end
          className={({ isActive }) =>
            `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all ${
              isActive
                ? 'bg-primary-50 text-primary-700 font-medium'
                : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
            }`
          }
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
            <path d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          Dashboard
        </NavLink>
      </div>

      {/* Scrollable sections */}
      <div className="flex-1 overflow-y-auto sidebar-scroll px-3 pb-3">
        {/* Tasks Section */}
        <div className="mt-1">
          <button
            onClick={() => setTasksExpanded(!tasksExpanded)}
            className="flex items-center justify-between w-full px-3 py-2 text-xs font-display font-medium text-sidebar-muted uppercase tracking-wider hover:text-sidebar-text transition-colors"
          >
            <div className="flex items-center gap-2">
              <span>Tasks</span>
              {runningCount > 0 && (
                <span className="flex items-center gap-1 px-1.5 py-0.5 rounded-full bg-primary-100 text-primary-600 text-[10px] font-semibold normal-case tracking-normal">
                  <span className="relative flex h-1.5 w-1.5">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary-400 opacity-75" />
                    <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-primary-500" />
                  </span>
                  {runningCount}
                </span>
              )}
            </div>
            <svg
              className={`w-3 h-3 transition-transform duration-200 ${tasksExpanded ? 'rotate-0' : '-rotate-90'}`}
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
            >
              <path d="M19 9l-7 7-7-7" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>

          {tasksExpanded && (
            <div className="space-y-0.5 animate-fade-in">
              {tasks.length === 0 ? (
                <p className="px-3 py-2 text-xs text-sidebar-muted/60">No tasks yet</p>
              ) : (
                tasks.map((task) => (
                  <NavLink
                    key={`${task.namespace}/${task.name}`}
                    to={`/tasks/${task.namespace}/${task.name}`}
                    onClick={() => setMobileSidebarOpen(false)}
                    className={({ isActive }) =>
                      `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all group ${
                        isActive
                          ? 'bg-primary-50 text-primary-700 font-medium'
                          : 'text-slate-600 hover:text-sidebar-text hover:bg-sidebar-hover'
                      }`
                    }
                  >
                    <TaskStatusDot phase={task.phase} />
                    <div className="flex-1 min-w-0">
                      <p className="truncate text-[13px]">{task.name}</p>
                      <p className="text-[10px] text-sidebar-muted truncate">
                        {task.namespace}
                      </p>
                    </div>
                    <span className="text-[10px] text-sidebar-muted/60 whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity">
                      <TimeAgo date={task.createdAt} />
                    </span>
                  </NavLink>
                ))
              )}
              <NavLink
                to="/tasks"
                onClick={() => setMobileSidebarOpen(false)}
                className="flex items-center gap-2 px-3 py-1.5 text-xs text-sidebar-muted hover:text-primary-600 transition-colors"
              >
                View all tasks
                <svg className="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </NavLink>
            </div>
          )}
        </div>

        {/* Agents Section */}
        <div className="mt-3">
          <button
            onClick={() => setAgentsExpanded(!agentsExpanded)}
            className="flex items-center justify-between w-full px-3 py-2 text-xs font-display font-medium text-sidebar-muted uppercase tracking-wider hover:text-sidebar-text transition-colors"
          >
            <div className="flex items-center gap-2">
              <span>Agents</span>
              <span className="px-1.5 py-0.5 rounded-full bg-slate-100 text-[10px] text-sidebar-muted font-semibold normal-case tracking-normal">
                {agents.length}
              </span>
            </div>
            <svg
              className={`w-3 h-3 transition-transform duration-200 ${agentsExpanded ? 'rotate-0' : '-rotate-90'}`}
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
            >
              <path d="M19 9l-7 7-7-7" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>

          {agentsExpanded && (
            <div className="space-y-0.5 animate-fade-in">
              {agents.length === 0 ? (
                <p className="px-3 py-2 text-xs text-sidebar-muted/60">No agents configured</p>
              ) : (
                agents.map((agent) => (
                  <NavLink
                    key={`${agent.namespace}/${agent.name}`}
                    to={`/agents/${agent.namespace}/${agent.name}`}
                    onClick={() => setMobileSidebarOpen(false)}
                    className={({ isActive }) =>
                      `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all group ${
                        isActive
                          ? 'bg-primary-50 text-primary-700 font-medium'
                          : 'text-slate-600 hover:text-sidebar-text hover:bg-sidebar-hover'
                      }`
                    }
                  >
                    <div className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${
                      agent.mode === 'Server' ? 'bg-violet-500' : 'bg-primary-400'
                    }`} />
                    <div className="flex-1 min-w-0">
                      <p className="truncate text-[13px]">{agent.name}</p>
                      <p className="text-[10px] text-sidebar-muted truncate">
                        {agent.namespace}
                      </p>
                    </div>
                    {agent.mode === 'Server' && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-violet-100 text-violet-600 font-medium opacity-0 group-hover:opacity-100 transition-opacity">
                        server
                      </span>
                    )}
                  </NavLink>
                ))
              )}
              <NavLink
                to="/agents"
                onClick={() => setMobileSidebarOpen(false)}
                className="flex items-center gap-2 px-3 py-1.5 text-xs text-sidebar-muted hover:text-primary-600 transition-colors"
              >
                View all agents
                <svg className="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </NavLink>
            </div>
          )}
        </div>
      </div>

      {/* Footer */}
      <div className="px-4 py-3 border-t border-sidebar-border flex-shrink-0">
        <div className="flex items-center gap-2 text-[11px] text-sidebar-muted">
          <div className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
          <span className="font-display">{serverVersion}</span>
        </div>
      </div>
    </div>
  );

  return (
    <div className="h-screen flex overflow-hidden bg-surface-50 font-body">
      {/* Mobile sidebar backdrop */}
      {mobileSidebarOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/40 backdrop-blur-sm lg:hidden"
          onClick={() => setMobileSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`
          fixed inset-y-0 left-0 z-40 flex flex-col bg-sidebar border-r border-sidebar-border
          transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]
          lg:static lg:z-auto
          ${mobileSidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
          ${sidebarOpen ? 'w-64' : 'w-0 lg:w-16'}
        `}
      >
        {sidebarOpen ? sidebarContent : (
          <div className="hidden lg:flex flex-col items-center py-3 gap-2 h-full border-r border-sidebar-border">
            {/* Collapsed sidebar */}
            <NavLink to="/" className="w-9 h-9 rounded-lg flex items-center justify-center mb-2">
              <img src={logoImg} alt="KubeOpenCode" className="w-7 h-7 rounded-lg" />
            </NavLink>
            <button
              onClick={() => setSidebarOpen(true)}
              className="w-9 h-9 rounded-lg flex items-center justify-center text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover transition-colors"
              title="Expand sidebar"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M13 5l7 7-7 7M5 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>
            <Link
              to="/tasks/create"
              className="w-9 h-9 rounded-lg flex items-center justify-center text-sidebar-muted hover:text-primary-600 hover:bg-sidebar-hover transition-colors"
              title="New Task"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M12 5v14M5 12h14" strokeLinecap="round" />
              </svg>
            </Link>
            <NavLink
              to="/"
              end
              className={({ isActive }) =>
                `w-9 h-9 rounded-lg flex items-center justify-center transition-colors ${
                  isActive ? 'text-primary-700 bg-primary-50' : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
                }`
              }
              title="Dashboard"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </NavLink>
            <NavLink
              to="/tasks"
              className={({ isActive }) =>
                `w-9 h-9 rounded-lg flex items-center justify-center transition-colors ${
                  isActive ? 'text-primary-700 bg-primary-50' : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
                }`
              }
              title="Tasks"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </NavLink>
            <NavLink
              to="/agents"
              className={({ isActive }) =>
                `w-9 h-9 rounded-lg flex items-center justify-center transition-colors ${
                  isActive ? 'text-primary-700 bg-primary-50' : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
                }`
              }
              title="Agents"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </NavLink>

            <div className="flex-1" />
            <div className="w-1.5 h-1.5 rounded-full bg-emerald-500 mb-2" title={serverVersion} />
          </div>
        )}
      </aside>

      {/* Main content area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Top bar - mobile only + breadcrumb area */}
        <header className="h-14 flex items-center justify-between px-4 lg:px-6 border-b border-slate-200/80 bg-white/60 backdrop-blur-sm flex-shrink-0">
          <button
            onClick={() => setMobileSidebarOpen(true)}
            className="lg:hidden flex items-center justify-center w-8 h-8 rounded-lg text-slate-500 hover:text-slate-800 hover:bg-slate-100 transition-colors"
          >
            <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M4 6h16M4 12h16M4 18h16" strokeLinecap="round" />
            </svg>
          </button>
          <div className="flex-1" />
          {runningCount > 0 && (
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-primary-50 border border-primary-100">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-primary-500" />
              </span>
              <span className="text-xs font-medium text-primary-700">
                {runningCount} running
              </span>
            </div>
          )}
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto main-scroll">
          <div className="max-w-6xl mx-auto px-4 lg:px-8 py-6 lg:py-8">
            <Outlet />
          </div>
        </main>
      </div>

      <ToastContainer />
    </div>
  );
}

function TaskStatusDot({ phase }: { phase: string }) {
  const lower = phase?.toLowerCase() || 'pending';
  const colorMap: Record<string, string> = {
    running: 'bg-primary-500',
    completed: 'bg-emerald-500',
    failed: 'bg-red-500',
    queued: 'bg-amber-500',
    pending: 'bg-slate-400',
  };
  const color = colorMap[lower] || 'bg-slate-400';
  const isAnimated = lower === 'running' || lower === 'queued';

  return (
    <span className="relative flex h-2 w-2 flex-shrink-0">
      {isAnimated && (
        <span className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${color}`} />
      )}
      <span className={`relative inline-flex rounded-full h-2 w-2 ${color}`} />
    </span>
  );
}

export default Layout;
