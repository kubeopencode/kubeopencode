import React, { useState, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import api, { CreateRegistryRequest } from '../api/client';
import { useToast } from '../contexts/ToastContext';
import { useNamespace } from '../contexts/NamespaceContext';
import Breadcrumbs from '../components/Breadcrumbs';
import SearchableSelect from '../components/SearchableSelect';

function RegistryCreatePage() {
  const navigate = useNavigate();
  const { addToast } = useToast();
  const { namespace: globalNamespace, isAllNamespaces } = useNamespace();

  const [name, setName] = useState('');
  const [selectedNamespace, setSelectedNamespace] = useState('');

  const { data: namespacesData } = useQuery({
    queryKey: ['namespaces'],
    queryFn: () => api.getNamespaces(),
  });

  const namespaces = useMemo(
    () => namespacesData?.namespaces || [],
    [namespacesData]
  );

  const namespace = selectedNamespace || (isAllNamespaces ? 'default' : globalNamespace);

  const createMutation = useMutation({
    mutationFn: (data: CreateRegistryRequest) => api.createRegistry(namespace, data),
    onSuccess: (result) => {
      addToast(`Registry "${result.name}" created successfully`, 'success');
      navigate(`/registries/${result.namespace}/${result.name}`);
    },
    onError: (err: Error) => {
      addToast(`Failed to create registry: ${err.message}`, 'error');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createMutation.mutate({ name });
  };

  const isValid = !!name;

  const labelClass = 'block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5';
  const inputClass = 'block w-full px-3 py-2 rounded-lg border border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 placeholder:text-stone-300';

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'Registries', to: '/registries' },
        { label: 'Create Registry' },
      ]} />

      <div className="bg-white rounded-xl border-0 overflow-hidden shadow-card max-w-3xl">
        <div className="px-6 py-5 border-b border-stone-100">
          <h2 className="font-display text-xl font-bold text-stone-900">Create Registry</h2>
          <p className="text-sm text-stone-400 mt-0.5">Create an asset catalog for agent assembly</p>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-6">
          {/* Namespace */}
          {isAllNamespaces && namespaces.length > 0 && (
            <div>
              <label htmlFor="namespace" className={labelClass}>Namespace</label>
              <SearchableSelect
                id="namespace"
                value={selectedNamespace}
                onChange={setSelectedNamespace}
                options={namespaces.map((ns) => ({ value: ns, label: ns }))}
                placeholder="Select namespace..."
                required
              />
            </div>
          )}

          {/* Name */}
          <div>
            <label htmlFor="name" className={labelClass}>Name</label>
            <input
              type="text"
              id="name"
              value={name}
              onChange={(e) => {
                const sanitized = e.target.value
                  .toLowerCase()
                  .replace(/\s+/g, '-')
                  .replace(/[^a-z0-9\-.]/g, '');
                setName(sanitized);
              }}
              required
              placeholder="my-registry"
              className={inputClass}
            />
          </div>

          <p className="text-xs text-stone-400">
            Additional configuration (images, skills, plugins) can be added later via YAML editing.
          </p>

          {/* Error */}
          {createMutation.isError && (
            <div className="bg-red-50 border border-red-200 rounded-lg p-4">
              <p className="text-red-700 text-sm">
                {(createMutation.error as Error).message}
              </p>
            </div>
          )}

          {/* Actions */}
          <div className="flex justify-end space-x-3 pt-2">
            <Link
              to="/registries"
              className="px-4 py-2.5 text-sm font-medium text-stone-600 bg-white shadow-ring rounded-lg hover:shadow-card transition-all"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={createMutation.isPending || !isValid}
              className="px-5 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {createMutation.isPending ? 'Creating...' : 'Create Registry'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default RegistryCreatePage;
