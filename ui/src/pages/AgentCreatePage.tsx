import React, { useState, useMemo, useEffect } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import api, { CreateAgentRequest } from '../api/client';
import { useToast } from '../contexts/ToastContext';
import { useNamespace } from '../contexts/NamespaceContext';
import Breadcrumbs from '../components/Breadcrumbs';
import SearchableSelect from '../components/SearchableSelect';

function AgentCreatePage() {
  const navigate = useNavigate();
  const { addToast } = useToast();
  const [searchParams] = useSearchParams();
  const { namespace: globalNamespace, isAllNamespaces } = useNamespace();

  // Basic fields
  const [name, setName] = useState('');
  const [profile, setProfile] = useState('');
  const [workspaceDir, setWorkspaceDir] = useState('');
  const [serviceAccountName, setServiceAccountName] = useState('');
  // selectedTemplate stores "namespace/name" or "" for no template
  const [selectedTemplate, setSelectedTemplate] = useState('');

  // P0: Images
  const [agentImage, setAgentImage] = useState('');
  const [executorImage, setExecutorImage] = useState('');

  // P1: Common configuration
  const [maxConcurrentTasks, setMaxConcurrentTasks] = useState('');
  const [standbyIdleTimeout, setStandbyIdleTimeout] = useState('');
  const [sessionsEnabled, setSessionsEnabled] = useState(false);
  const [sessionsSize, setSessionsSize] = useState('');
  const [workspacePersistEnabled, setWorkspacePersistEnabled] = useState(false);
  const [workspacePersistSize, setWorkspacePersistSize] = useState('');

  // P2: Advanced
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [port, setPort] = useState('');
  const [httpProxy, setHttpProxy] = useState('');
  const [httpsProxy, setHttpsProxy] = useState('');
  const [noProxy, setNoProxy] = useState('');

  const { data: allTemplatesData } = useQuery({
    queryKey: ['all-templates'],
    queryFn: () => api.listAllAgentTemplates({ limit: 200, sortOrder: 'asc' }),
  });

  const allTemplates = useMemo(
    () => allTemplatesData?.templates || [],
    [allTemplatesData]
  );

  // Derive namespace from template selection, or fall back to global
  const namespace = useMemo(() => {
    if (selectedTemplate) {
      const ns = selectedTemplate.split('/')[0];
      if (ns) return ns;
    }
    return isAllNamespaces ? 'default' : globalNamespace;
  }, [selectedTemplate, globalNamespace, isAllNamespaces]);

  // Fetch template details when selected to show inherited values
  const templateParts = selectedTemplate.split('/');
  const { data: templateDetail } = useQuery({
    queryKey: ['agent-template', templateParts[0], templateParts[1]],
    queryFn: () => api.getAgentTemplate(templateParts[0], templateParts[1]),
    enabled: templateParts.length === 2 && !!templateParts[0] && !!templateParts[1],
  });

  const hasTemplate = !!selectedTemplate && !!templateDetail;

  useEffect(() => {
    const templateParam = searchParams.get('template');
    if (templateParam) {
      setSelectedTemplate(templateParam);
    }
  }, [searchParams]);

  // When template changes, clear overrides so inherited values show through
  useEffect(() => {
    setWorkspaceDir('');
    setServiceAccountName('');
    setAgentImage('');
    setExecutorImage('');
  }, [selectedTemplate]);

  const createMutation = useMutation({
    mutationFn: (agent: CreateAgentRequest) => api.createAgent(namespace, agent),
    onSuccess: (agent) => {
      addToast(`Agent "${agent.name}" created successfully`, 'success');
      navigate(`/agents/${agent.namespace}/${agent.name}`);
    },
    onError: (err: Error) => {
      addToast(`Failed to create agent: ${err.message}`, 'error');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    const agent: CreateAgentRequest = { name };

    if (profile) agent.profile = profile;

    if (selectedTemplate) {
      const templateName = selectedTemplate.split('/')[1];
      if (templateName) {
        agent.templateRef = { name: templateName };
      }
    }

    // Only send fields if user explicitly set them (override)
    if (workspaceDir) agent.workspaceDir = workspaceDir;
    if (serviceAccountName) agent.serviceAccountName = serviceAccountName;
    if (agentImage) agent.agentImage = agentImage;
    if (executorImage) agent.executorImage = executorImage;

    // P1
    if (maxConcurrentTasks) {
      const parsed = parseInt(maxConcurrentTasks, 10);
      if (!isNaN(parsed)) agent.maxConcurrentTasks = parsed;
    }
    if (standbyIdleTimeout) agent.standby = { idleTimeout: standbyIdleTimeout };

    if (sessionsEnabled || workspacePersistEnabled) {
      agent.persistence = {};
      if (sessionsEnabled) {
        agent.persistence.sessions = sessionsSize ? { size: sessionsSize } : {};
      }
      if (workspacePersistEnabled) {
        agent.persistence.workspace = workspacePersistSize ? { size: workspacePersistSize } : {};
      }
    }

    // P2
    if (port) {
      const parsed = parseInt(port, 10);
      if (!isNaN(parsed)) agent.port = parsed;
    }
    if (httpProxy || httpsProxy || noProxy) {
      agent.proxy = {
        httpProxy: httpProxy || undefined,
        httpsProxy: httpsProxy || undefined,
        noProxy: noProxy || undefined,
      };
    }

    createMutation.mutate(agent);
  };

  // Validation: name is always required; workspaceDir and serviceAccountName
  // are required only when no template provides them
  const effectiveWorkspaceDir = workspaceDir || templateDetail?.workspaceDir || '';
  const effectiveServiceAccount = serviceAccountName || templateDetail?.serviceAccountName || '';
  const isValid = name && (hasTemplate || (effectiveWorkspaceDir && effectiveServiceAccount));

  const labelClass = 'block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5';
  const inputClass = 'block w-full rounded-lg border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 placeholder:text-stone-300';
  const monoInputClass = `${inputClass} font-mono placeholder:font-body`;
  const inheritedHint = hasTemplate ? (
    <span className="normal-case tracking-normal text-stone-300"> (inherited from template)</span>
  ) : null;

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'Agents', to: '/agents' },
        { label: 'Create Agent' },
      ]} />

      <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm max-w-3xl">
        <div className="px-6 py-5 border-b border-stone-100">
          <h2 className="font-display text-xl font-bold text-stone-900">Create Agent</h2>
          <p className="text-sm text-stone-400 mt-0.5">Create a new AI agent configuration</p>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-6">
          {/* Template Selection */}
          <div>
            <label htmlFor="template" className={labelClass}>
              Template <span className="normal-case tracking-normal text-stone-300">(optional)</span>
            </label>
            <SearchableSelect
              id="template"
              value={selectedTemplate}
              onChange={setSelectedTemplate}
              options={[
                { value: '', label: 'No template' },
                ...allTemplates.map((tmpl) => ({
                  value: `${tmpl.namespace}/${tmpl.name}`,
                  label: `${tmpl.namespace}/${tmpl.name}`,
                })),
              ]}
              placeholder="No template"
            />
            <p className="mt-1.5 text-xs text-stone-400">
              {allTemplates.length === 0
                ? 'No templates available.'
                : `Agent will be created in namespace "${namespace}".`}
            </p>
          </div>

          {/* Name */}
          <div>
            <label htmlFor="name" className={labelClass}>Name</label>
            <input
              type="text"
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              placeholder="my-agent"
              className={inputClass}
            />
          </div>

          {/* Workspace & ServiceAccount */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="workspaceDir" className={labelClass}>
                Workspace Directory{inheritedHint}
              </label>
              <input
                type="text"
                id="workspaceDir"
                value={workspaceDir}
                onChange={(e) => setWorkspaceDir(e.target.value)}
                required={!hasTemplate}
                placeholder={hasTemplate && templateDetail?.workspaceDir ? templateDetail.workspaceDir : '/workspace'}
                className={monoInputClass}
              />
            </div>
            <div>
              <label htmlFor="serviceAccountName" className={labelClass}>
                Service Account{inheritedHint}
              </label>
              <input
                type="text"
                id="serviceAccountName"
                value={serviceAccountName}
                onChange={(e) => setServiceAccountName(e.target.value)}
                required={!hasTemplate}
                placeholder={hasTemplate && templateDetail?.serviceAccountName ? templateDetail.serviceAccountName : 'default'}
                className={monoInputClass}
              />
            </div>
          </div>

          {/* P0: Images */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="agentImage" className={labelClass}>
                Agent Image{inheritedHint}
                <span className="normal-case tracking-normal text-stone-300"> (init container)</span>
              </label>
              <input
                type="text"
                id="agentImage"
                value={agentImage}
                onChange={(e) => setAgentImage(e.target.value)}
                placeholder={hasTemplate && templateDetail?.agentImage ? templateDetail.agentImage : 'quay.io/kubeopencode/kubeopencode-agent-opencode:latest'}
                className={monoInputClass}
              />
            </div>
            <div>
              <label htmlFor="executorImage" className={labelClass}>
                Executor Image{inheritedHint}
                <span className="normal-case tracking-normal text-stone-300"> (worker container)</span>
              </label>
              <input
                type="text"
                id="executorImage"
                value={executorImage}
                onChange={(e) => setExecutorImage(e.target.value)}
                placeholder={hasTemplate && templateDetail?.executorImage ? templateDetail.executorImage : 'quay.io/kubeopencode/kubeopencode-agent-devbox:latest'}
                className={monoInputClass}
              />
            </div>
          </div>

          {/* Profile */}
          <div>
            <label htmlFor="profile" className={labelClass}>
              Profile <span className="normal-case tracking-normal text-stone-300">(optional)</span>
            </label>
            <textarea
              id="profile"
              value={profile}
              onChange={(e) => setProfile(e.target.value)}
              rows={3}
              placeholder="Describe this agent's purpose and capabilities..."
              className={inputClass}
            />
          </div>

          {/* P1: Common Configuration */}
          <div className="border-t border-stone-100 pt-5">
            <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-4">
              Runtime Configuration
            </h3>

            <div className="grid grid-cols-2 gap-4 mb-4">
              <div>
                <label htmlFor="maxConcurrentTasks" className={labelClass}>
                  Max Concurrent Tasks <span className="normal-case tracking-normal text-stone-300">(optional)</span>
                </label>
                <input
                  type="number"
                  id="maxConcurrentTasks"
                  value={maxConcurrentTasks}
                  onChange={(e) => setMaxConcurrentTasks(e.target.value)}
                  min="1"
                  placeholder="Unlimited"
                  className={inputClass}
                />
              </div>
              <div>
                <label htmlFor="standbyIdleTimeout" className={labelClass}>
                  Standby <span className="normal-case tracking-normal text-stone-300">(optional)</span>
                </label>
                <input
                  type="text"
                  id="standbyIdleTimeout"
                  value={standbyIdleTimeout}
                  onChange={(e) => setStandbyIdleTimeout(e.target.value)}
                  placeholder="e.g. 30m, 1h"
                  className={monoInputClass}
                />
                <p className="mt-1 text-xs text-stone-400">Auto-suspend after idle, auto-resume on new task</p>
              </div>
            </div>

            {/* Persistence */}
            <div className="space-y-3">
              <div className="flex items-center justify-between bg-stone-50 rounded-lg p-3 border border-stone-100">
                <div>
                  <label className="text-sm font-medium text-stone-700 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={sessionsEnabled}
                      onChange={(e) => setSessionsEnabled(e.target.checked)}
                      className="rounded border-stone-300 text-primary-600 focus:ring-primary-500 mr-2"
                    />
                    Session Persistence
                  </label>
                  <p className="text-xs text-stone-400 mt-0.5 ml-6">Persist conversation history across restarts</p>
                </div>
                {sessionsEnabled && (
                  <input
                    type="text"
                    value={sessionsSize}
                    onChange={(e) => setSessionsSize(e.target.value)}
                    placeholder="1Gi"
                    className="w-24 rounded-lg border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 font-mono placeholder:text-stone-300 placeholder:font-body"
                  />
                )}
              </div>

              <div className="flex items-center justify-between bg-stone-50 rounded-lg p-3 border border-stone-100">
                <div>
                  <label className="text-sm font-medium text-stone-700 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={workspacePersistEnabled}
                      onChange={(e) => setWorkspacePersistEnabled(e.target.checked)}
                      className="rounded border-stone-300 text-primary-600 focus:ring-primary-500 mr-2"
                    />
                    Workspace Persistence
                  </label>
                  <p className="text-xs text-stone-400 mt-0.5 ml-6">Persist workspace files across restarts</p>
                </div>
                {workspacePersistEnabled && (
                  <input
                    type="text"
                    value={workspacePersistSize}
                    onChange={(e) => setWorkspacePersistSize(e.target.value)}
                    placeholder="10Gi"
                    className="w-24 rounded-lg border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 font-mono placeholder:text-stone-300 placeholder:font-body"
                  />
                )}
              </div>
            </div>
          </div>

          {/* P2: Advanced Configuration (collapsible) */}
          <div className="border-t border-stone-100 pt-5">
            <button
              type="button"
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="flex items-center gap-1.5 text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider hover:text-stone-600 transition-colors"
            >
              <svg
                className={`w-3.5 h-3.5 transition-transform ${showAdvanced ? 'rotate-90' : ''}`}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              Advanced Configuration
            </button>

            {showAdvanced && (
              <div className="mt-4 space-y-4">
                {/* Port */}
                <div className="max-w-xs">
                  <label htmlFor="port" className={labelClass}>
                    Server Port <span className="normal-case tracking-normal text-stone-300">(default: 4096)</span>
                  </label>
                  <input
                    type="number"
                    id="port"
                    value={port}
                    onChange={(e) => setPort(e.target.value)}
                    min="1"
                    max="65535"
                    placeholder="4096"
                    className={inputClass}
                  />
                </div>

                {/* Proxy */}
                <div>
                  <span className={labelClass}>
                    Proxy <span className="normal-case tracking-normal text-stone-300">(optional)</span>
                  </span>
                  <div className="grid grid-cols-2 gap-4 mt-1.5">
                    <div>
                      <input
                        type="text"
                        value={httpProxy}
                        onChange={(e) => setHttpProxy(e.target.value)}
                        placeholder="HTTP Proxy (e.g. http://proxy:8080)"
                        className={monoInputClass}
                      />
                    </div>
                    <div>
                      <input
                        type="text"
                        value={httpsProxy}
                        onChange={(e) => setHttpsProxy(e.target.value)}
                        placeholder="HTTPS Proxy (e.g. http://proxy:8080)"
                        className={monoInputClass}
                      />
                    </div>
                  </div>
                  <div className="mt-2">
                    <input
                      type="text"
                      value={noProxy}
                      onChange={(e) => setNoProxy(e.target.value)}
                      placeholder="No Proxy (e.g. localhost,127.0.0.1,10.0.0.0/8)"
                      className={monoInputClass}
                    />
                  </div>
                </div>
              </div>
            )}
          </div>

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
              to="/agents"
              className="px-4 py-2.5 text-sm font-medium text-stone-600 bg-stone-100 rounded-lg hover:bg-stone-200 transition-colors"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={createMutation.isPending || !isValid}
              className="px-5 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {createMutation.isPending ? 'Creating...' : 'Create Agent'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default AgentCreatePage;
