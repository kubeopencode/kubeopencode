import React, { useState, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import api, { CreateAgentTemplateRequest } from '../api/client';
import { useToast } from '../contexts/ToastContext';
import { useNamespace } from '../contexts/NamespaceContext';
import Breadcrumbs from '../components/Breadcrumbs';
import SearchableSelect from '../components/SearchableSelect';

function AgentTemplateCreatePage() {
  const navigate = useNavigate();
  const { addToast } = useToast();
  const { namespace: globalNamespace, isAllNamespaces } = useNamespace();

  const [name, setName] = useState('');
  const [selectedNamespace, setSelectedNamespace] = useState('');
  const [workspaceDir, setWorkspaceDir] = useState('');
  const [serviceAccountName, setServiceAccountName] = useState('');
  const [agentImage, setAgentImage] = useState('');
  const [executorImage, setExecutorImage] = useState('');

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
    mutationFn: (data: CreateAgentTemplateRequest) => api.createAgentTemplate(namespace, data),
    onSuccess: (tmpl) => {
      addToast(`Template "${tmpl.name}" created successfully`, 'success');
      navigate(`/templates/${tmpl.namespace}/${tmpl.name}`);
    },
    onError: (err: Error) => {
      addToast(`Failed to create template: ${err.message}`, 'error');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    const data: CreateAgentTemplateRequest = { name };
    if (workspaceDir) data.workspaceDir = workspaceDir;
    if (serviceAccountName) data.serviceAccountName = serviceAccountName;
    if (agentImage) data.agentImage = agentImage;
    if (executorImage) data.executorImage = executorImage;

    createMutation.mutate(data);
  };

  const isValid = !!name;

  const labelClass = 'block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5';
  const inputClass = 'block w-full rounded-md border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 placeholder:text-stone-300';
  const monoInputClass = `${inputClass} font-mono placeholder:font-body`;

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'Templates', to: '/templates' },
        { label: 'Create Template' },
      ]} />

      <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm max-w-3xl">
        <div className="px-6 py-5 border-b border-stone-100">
          <h2 className="font-display text-xl font-bold text-stone-900">Create Agent Template</h2>
          <p className="text-sm text-stone-400 mt-0.5">Create a reusable base configuration for Agents</p>
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
              placeholder="my-template"
              className={inputClass}
            />
          </div>

          {/* Workspace & ServiceAccount */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="workspaceDir" className={labelClass}>
                Workspace Directory
                <span className="normal-case tracking-normal text-stone-300"> (optional)</span>
              </label>
              <input
                type="text"
                id="workspaceDir"
                value={workspaceDir}
                onChange={(e) => setWorkspaceDir(e.target.value)}
                placeholder="/workspace"
                className={monoInputClass}
              />
            </div>
            <div>
              <label htmlFor="serviceAccountName" className={labelClass}>
                Service Account
                <span className="normal-case tracking-normal text-stone-300"> (optional)</span>
              </label>
              <input
                type="text"
                id="serviceAccountName"
                value={serviceAccountName}
                onChange={(e) => setServiceAccountName(e.target.value)}
                placeholder="default"
                className={monoInputClass}
              />
            </div>
          </div>

          {/* Images */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="agentImage" className={labelClass}>
                Agent Image
                <span className="normal-case tracking-normal text-stone-300"> (init container, optional)</span>
              </label>
              <input
                type="text"
                id="agentImage"
                value={agentImage}
                onChange={(e) => setAgentImage(e.target.value)}
                placeholder="ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest"
                className={monoInputClass}
              />
            </div>
            <div>
              <label htmlFor="executorImage" className={labelClass}>
                Executor Image
                <span className="normal-case tracking-normal text-stone-300"> (worker container, optional)</span>
              </label>
              <input
                type="text"
                id="executorImage"
                value={executorImage}
                onChange={(e) => setExecutorImage(e.target.value)}
                placeholder="ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest"
                className={monoInputClass}
              />
            </div>
          </div>

          <p className="text-xs text-stone-400">
            Additional configuration (contexts, credentials, skills) can be added later via YAML editing.
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
              to="/templates"
              className="px-4 py-2.5 text-sm font-medium text-stone-600 bg-stone-100 rounded-lg hover:bg-stone-200 transition-colors"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={createMutation.isPending || !isValid}
              className="px-5 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {createMutation.isPending ? 'Creating...' : 'Create Template'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default AgentTemplateCreatePage;
