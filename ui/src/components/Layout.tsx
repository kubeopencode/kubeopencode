import React, { useState } from 'react';
import { Outlet, NavLink, Link, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import { useNamespace } from '../contexts/NamespaceContext';
import NamespaceSelector from './NamespaceSelector';
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
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const location = useLocation();
  const serverVersion = useServerVersion();
  const { namespace, setNamespace, ALL_NAMESPACES } = useNamespace();

  const { data: namespacesData } = useQuery({
    queryKey: ['namespaces'],
    queryFn: () => api.getNamespaces(),
  });

  const { data: tasksData } = useQuery({
    queryKey: ['sidebar-tasks'],
    queryFn: () => api.listAllTasks({ limit: 20, sortOrder: 'desc' }),
    refetchInterval: 5000,
  });

  const tasks = tasksData?.tasks || [];
  const runningCount = tasks.filter(t => t.phase === 'Running').length;

  const navLinkClass = ({ isActive }: { isActive: boolean }) =>
    `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all ${
      isActive
        ? 'bg-primary-50 text-primary-700 font-medium'
        : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
    }`;

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
      <div className="px-3 pt-2 pb-3 flex-shrink-0 space-y-0.5">
        <NavLink to="/" end className={navLinkClass}>
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
            <path d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          Dashboard
        </NavLink>

        <NavLink
          to="/tasks"
          className={({ isActive }) =>
            `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all ${
              isActive || location.pathname.startsWith('/tasks/')
                ? 'bg-primary-50 text-primary-700 font-medium'
                : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
            }`
          }
          onClick={() => setMobileSidebarOpen(false)}
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
            <path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          <span className="flex-1">Tasks</span>
        </NavLink>

        <NavLink
          to="/crontasks"
          className={({ isActive }) =>
            `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all ${
              isActive || location.pathname.startsWith('/crontasks/')
                ? 'bg-primary-50 text-primary-700 font-medium'
                : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
            }`
          }
          onClick={() => setMobileSidebarOpen(false)}
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="10" />
            <polyline points="12 6 12 12 16 14" />
          </svg>
          <span className="flex-1">CronTasks</span>
        </NavLink>

        <NavLink
          to="/agents"
          className={({ isActive }) =>
            `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all ${
              isActive || location.pathname.startsWith('/agents/')
                ? 'bg-primary-50 text-primary-700 font-medium'
                : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
            }`
          }
          onClick={() => setMobileSidebarOpen(false)}
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <line x1="12" y1="2" x2="12" y2="6" />
            <circle cx="12" cy="2" r="1" fill="currentColor" />
            <rect x="4" y="6" width="16" height="12" rx="2" />
            <circle cx="9" cy="12" r="1.5" />
            <circle cx="15" cy="12" r="1.5" />
            <line x1="9" y1="16" x2="15" y2="16" />
            <line x1="2" y1="10" x2="4" y2="10" />
            <line x1="20" y1="10" x2="22" y2="10" />
          </svg>
          <span className="flex-1">Agents</span>
        </NavLink>

        <NavLink
          to="/templates"
          className={({ isActive }) =>
            `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all ${
              isActive || location.pathname.startsWith('/templates/')
                ? 'bg-primary-50 text-primary-700 font-medium'
                : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
            }`
          }
          onClick={() => setMobileSidebarOpen(false)}
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <rect x="3" y="3" width="7" height="7" rx="1" />
            <rect x="14" y="3" width="7" height="7" rx="1" />
            <rect x="3" y="14" width="7" height="7" rx="1" />
            <rect x="14" y="14" width="7" height="7" rx="1" />
          </svg>
          <span className="flex-1">AgentTemplates</span>
        </NavLink>
      </div>

      <div className="flex-1" />

      {/* Settings */}
      <div className="px-3 pb-3 flex-shrink-0">
        <NavLink
          to="/config"
          className={({ isActive }) =>
            `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all ${
              isActive
                ? 'bg-primary-50 text-primary-700 font-medium'
                : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
            }`
          }
          onClick={() => setMobileSidebarOpen(false)}
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="3" />
            <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" />
          </svg>
          <span className="flex-1">Config</span>
        </NavLink>
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
              to="/crontasks"
              className={({ isActive }) =>
                `w-9 h-9 rounded-lg flex items-center justify-center transition-colors ${
                  isActive ? 'text-primary-700 bg-primary-50' : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
                }`
              }
              title="CronTasks"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10" />
                <polyline points="12 6 12 12 16 14" />
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
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <line x1="12" y1="2" x2="12" y2="6" />
                <circle cx="12" cy="2" r="1" fill="currentColor" />
                <rect x="4" y="6" width="16" height="12" rx="2" />
                <circle cx="9" cy="12" r="1.5" />
                <circle cx="15" cy="12" r="1.5" />
                <line x1="9" y1="16" x2="15" y2="16" />
                <line x1="2" y1="10" x2="4" y2="10" />
                <line x1="20" y1="10" x2="22" y2="10" />
              </svg>
            </NavLink>
            <NavLink
              to="/templates"
              className={({ isActive }) =>
                `w-9 h-9 rounded-lg flex items-center justify-center transition-colors ${
                  isActive ? 'text-primary-700 bg-primary-50' : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
                }`
              }
              title="AgentTemplates"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <rect x="3" y="3" width="7" height="7" rx="1" />
                <rect x="14" y="3" width="7" height="7" rx="1" />
                <rect x="3" y="14" width="7" height="7" rx="1" />
                <rect x="14" y="14" width="7" height="7" rx="1" />
              </svg>
            </NavLink>

            <div className="flex-1" />
            <NavLink
              to="/config"
              className={({ isActive }) =>
                `w-9 h-9 rounded-lg flex items-center justify-center transition-colors ${
                  isActive ? 'text-primary-700 bg-primary-50' : 'text-sidebar-muted hover:text-sidebar-text hover:bg-sidebar-hover'
                }`
              }
              title="Config"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="3" />
                <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" />
              </svg>
            </NavLink>
            <div className="w-1.5 h-1.5 rounded-full bg-emerald-500 mb-2" title={serverVersion} />
          </div>
        )}
      </aside>

      {/* Main content area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Top bar - mobile only + breadcrumb area */}
        <header className="h-14 flex items-center justify-between px-4 lg:px-6 border-b border-slate-200/80 bg-white/60 backdrop-blur-sm flex-shrink-0 relative z-20">
          <button
            onClick={() => setMobileSidebarOpen(true)}
            className="lg:hidden flex items-center justify-center w-8 h-8 rounded-lg text-slate-500 hover:text-slate-800 hover:bg-slate-100 transition-colors"
          >
            <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M4 6h16M4 12h16M4 18h16" strokeLinecap="round" />
            </svg>
          </button>
          <div className="ml-2 lg:ml-0">
            <NamespaceSelector
              value={namespace}
              onChange={setNamespace}
              namespaces={namespacesData?.namespaces || []}
              allNamespacesValue={ALL_NAMESPACES}
            />
          </div>
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

export default Layout;
