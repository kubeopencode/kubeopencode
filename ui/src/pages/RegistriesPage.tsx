import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import api from '../api/client';
import Labels from '../components/Labels';
import Skeleton from '../components/Skeleton';
import EmptyState from '../components/EmptyState';
import ResourceFilter from '../components/ResourceFilter';
import { useFilterState } from '../hooks/useFilterState';
import { useNamespace } from '../contexts/NamespaceContext';

const PAGE_SIZE = 12;

function RegistriesPage() {
  const { namespace, isAllNamespaces } = useNamespace();
  const [currentPage, setCurrentPage] = useState(1);
  const [filters, setFilters] = useFilterState();

  useEffect(() => {
    setCurrentPage(1);
  }, [namespace, filters.name, filters.labelSelector]);

  const filterParams = {
    name: filters.name || undefined,
    labelSelector: filters.labelSelector || undefined,
    limit: PAGE_SIZE,
    offset: (currentPage - 1) * PAGE_SIZE,
    sortOrder: 'desc' as const,
  };

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['registries', namespace, currentPage, filters.name, filters.labelSelector],
    queryFn: () =>
      isAllNamespaces
        ? api.listAllRegistries(filterParams)
        : api.listRegistries(namespace, filterParams),
  });

  return (
    <div className="animate-fade-in">
      <div className="sm:flex sm:items-center sm:justify-between mb-6">
        <div>
          <h2 className="font-display text-2xl font-bold text-stone-900 tracking-tight">Registries</h2>
          <p className="mt-1 text-sm text-stone-500">
            Asset catalogs for agent assembly
          </p>
        </div>
        <Link
          to="/registries/create"
          className="inline-flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 transition-colors shadow-sm"
        >
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M12 5v14M5 12h14" strokeLinecap="round" />
          </svg>
          Create Registry
        </Link>
      </div>

      <div className="mb-4">
        <ResourceFilter
          filters={filters}
          onFilterChange={setFilters}
          placeholder="Filter registries by name..."
        />
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="bg-white rounded-xl border-0 shadow-card p-5">
              <Skeleton className="h-5 w-32 mb-2" />
              <Skeleton className="h-3 w-20 mb-4" />
              <Skeleton className="h-3 w-full mb-2" />
              <Skeleton className="h-3 w-full mb-2" />
              <Skeleton className="h-3 w-3/4" />
            </div>
          ))}
        </div>
      ) : error ? (
        <div className="bg-red-50 border border-red-200 rounded-xl p-5">
          <p className="text-red-700 text-sm">Error loading registries: {(error as Error).message}</p>
          <button
            onClick={() => refetch()}
            className="mt-2 text-sm text-red-600 hover:text-red-800 font-medium"
          >
            Retry
          </button>
        </div>
      ) : (
        <>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {data?.registries.length === 0 ? (
            <div className="col-span-full bg-white rounded-xl border-0 shadow-card">
              <EmptyState
                icon={
                  <svg className="w-10 h-10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
                  </svg>
                }
                title="No registries found"
                description="Registries are asset catalogs that define images, skills, and plugins available for agent assembly."
                action={{ label: 'Create Registry', to: '/registries/create' }}
              />
            </div>
          ) : (
            data?.registries.map((reg) => {
              const { readyCount, totalCount } = reg.summary;
              let badgeColor = 'bg-stone-100 text-stone-500';
              if (totalCount > 0 && readyCount === totalCount) {
                badgeColor = 'bg-emerald-50 text-emerald-700';
              } else if (totalCount > 0 && readyCount > 0) {
                badgeColor = 'bg-amber-50 text-amber-700';
              }

              return (
                <Link
                  key={`${reg.namespace}/${reg.name}`}
                  to={`/registries/${reg.namespace}/${reg.name}`}
                  className="bg-white rounded-xl border-0 overflow-hidden shadow-card hover:shadow-card-hover transition-all group"
                >
                  <div className="p-5">
                    <div className="flex items-start justify-between">
                      <div>
                        <h3 className="text-sm font-semibold text-stone-800 group-hover:text-stone-900">
                          {reg.name}
                        </h3>
                        <p className="text-xs text-stone-400 mt-0.5 font-mono">{reg.namespace}</p>
                      </div>
                      <span className={`text-[11px] px-2 py-0.5 rounded-md font-medium ${badgeColor}`}>
                        {readyCount}/{totalCount}
                      </span>
                    </div>

                    <div className="mt-4 space-y-1.5">
                      <div className="flex justify-between text-xs">
                        <span className="text-stone-400">Images</span>
                        <span className="text-stone-600 font-mono">{reg.summary.images}</span>
                      </div>
                      <div className="flex justify-between text-xs">
                        <span className="text-stone-400">Skills</span>
                        <span className="text-stone-600 font-mono">{reg.summary.skills}</span>
                      </div>
                      <div className="flex justify-between text-xs">
                        <span className="text-stone-400">Plugins</span>
                        <span className="text-stone-600 font-mono">{reg.summary.plugins}</span>
                      </div>
                    </div>

                    {reg.labels && Object.keys(reg.labels).length > 0 && (
                      <div className="mt-4 pt-3 border-t border-stone-100">
                        <Labels labels={reg.labels} maxDisplay={3} />
                      </div>
                    )}
                  </div>
                </Link>
              );
            })
          )}
        </div>

        {/* Pagination */}
        {data?.pagination && data.pagination.totalCount > 0 && (
          <div className="mt-6 flex items-center justify-between">
            <p className="text-xs text-stone-400">
              <span className="font-medium text-stone-600">{data.pagination.offset + 1}</span>
              {' '}-{' '}
              <span className="font-medium text-stone-600">
                {Math.min(data.pagination.offset + data.registries.length, data.pagination.totalCount)}
              </span>
              {' '}of{' '}
              <span className="font-medium text-stone-600">{data.pagination.totalCount}</span>
            </p>
            <div className="flex space-x-1">
              <button
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                disabled={currentPage === 1}
                className="px-3 py-1.5 text-xs font-medium text-stone-500 bg-stone-50 border border-stone-200 rounded-lg hover:bg-stone-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Previous
              </button>
              <button
                onClick={() => setCurrentPage((p) => p + 1)}
                disabled={!data.pagination.hasMore}
                className="px-3 py-1.5 text-xs font-medium text-stone-500 bg-stone-50 border border-stone-200 rounded-lg hover:bg-stone-100 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Next
              </button>
            </div>
          </div>
        )}
        </>
      )}
    </div>
  );
}

export default RegistriesPage;
