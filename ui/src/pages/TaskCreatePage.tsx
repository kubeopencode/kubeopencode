import React, { useState, useMemo, useEffect } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import api, { CreateTaskRequest } from '../api/client';
import { useToast } from '../contexts/ToastContext';
import { useNamespace } from '../contexts/NamespaceContext';
import Breadcrumbs from '../components/Breadcrumbs';
import SearchableSelect from '../components/SearchableSelect';

type SourceType = 'agent' | 'template';

function TaskCreatePage() {
  const navigate = useNavigate();
  const { addToast } = useToast();
  const [searchParams] = useSearchParams();
  const { namespace: globalNamespace, isAllNamespaces, setNamespace: setGlobalNamespace } = useNamespace();
  const [namespace, setNamespace] = useState(() => {
    const params = new URLSearchParams(window.location.search);
    const nsParam = params.get('namespace');
    if (nsParam) return nsParam;
    const agentParam = params.get('agent');
    if (agentParam) {
      const agentNs = agentParam.split('/')[0];
      if (agentNs) return agentNs;
    }
    return isAllNamespaces ? 'default' : globalNamespace;
  });
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [sourceType, setSourceType] = useState<SourceType>('agent');
  const [selectedAgent, setSelectedAgent] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState('');

  const { data: namespacesData } = useQuery({
    queryKey: ['namespaces'],
    queryFn: () => api.getNamespaces(),
  });

  const { data: agentsData } = useQuery({
    queryKey: ['agents'],
    queryFn: () => api.listAllAgents(),
  });

  const { data: templatesData } = useQuery({
    queryKey: ['agenttemplates'],
    queryFn: () => api.listAllAgentTemplates(),
  });

  const rerunTaskName = searchParams.get('rerun');
  const rerunNamespace = searchParams.get('namespace') || namespace;
  const { data: rerunTask } = useQuery({
    queryKey: ['task', rerunNamespace, rerunTaskName],
    queryFn: () => api.getTask(rerunNamespace, rerunTaskName!),
    enabled: !!rerunTaskName,
  });

  useEffect(() => {
    const agentParam = searchParams.get('agent');
    if (agentParam) {
      setSourceType('agent');
      setSelectedAgent(agentParam);
      const agentNs = agentParam.split('/')[0];
      if (agentNs) {
        setNamespace(agentNs);
      }
    }
    const templateParam = searchParams.get('template');
    if (templateParam) {
      setSourceType('template');
      setSelectedTemplate(templateParam);
      const templateNs = templateParam.split('/')[0];
      if (templateNs) {
        setNamespace(templateNs);
      }
    }
  }, [searchParams]);

  useEffect(() => {
    if (rerunTask) {
      if (rerunTask.description) {
        setDescription(rerunTask.description);
      }
      if (rerunTask.agentRef) {
        setSourceType('agent');
        const agentKey = `${rerunTask.namespace}/${rerunTask.agentRef.name}`;
        setSelectedAgent(agentKey);
        setNamespace(rerunTask.namespace);
      }
      if (rerunTask.templateRef) {
        setSourceType('template');
        const templateKey = `${rerunTask.namespace}/${rerunTask.templateRef.name}`;
        setSelectedTemplate(templateKey);
        setNamespace(rerunTask.namespace);
      }
    }
  }, [rerunTask]);

  const availableAgents = useMemo(() => {
    if (!agentsData?.agents) return [];
    return agentsData.agents.filter((agent) => agent.namespace === namespace);
  }, [agentsData?.agents, namespace]);

  const availableTemplates = useMemo(() => {
    if (!templatesData?.templates) return [];
    return templatesData.templates.filter((t) => t.namespace === namespace);
  }, [templatesData?.templates, namespace]);

  const handleNamespaceChange = (newNamespace: string) => {
    setNamespace(newNamespace);
    setGlobalNamespace(newNamespace);
    if (selectedAgent) {
      const agent = agentsData?.agents.find(
        (a) => `${a.namespace}/${a.name}` === selectedAgent
      );
      if (agent && agent.namespace !== newNamespace) {
        setSelectedAgent('');
      }
    }
    if (selectedTemplate) {
      const tmpl = templatesData?.templates.find(
        (t) => `${t.namespace}/${t.name}` === selectedTemplate
      );
      if (tmpl && tmpl.namespace !== newNamespace) {
        setSelectedTemplate('');
      }
    }
  };

  const handleSourceTypeChange = (newType: SourceType) => {
    setSourceType(newType);
    if (newType === 'agent') {
      setSelectedTemplate('');
    } else {
      setSelectedAgent('');
    }
  };

  const createMutation = useMutation({
    mutationFn: (task: CreateTaskRequest) => api.createTask(namespace, task),
    onSuccess: (task) => {
      addToast(`Task "${task.name}" created successfully`, 'success');
      navigate(`/tasks/${task.namespace}/${task.name}`);
    },
    onError: (err: Error) => {
      addToast(`Failed to create task: ${err.message}`, 'error');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    const task: CreateTaskRequest = {};

    if (name) {
      task.name = name;
    }

    if (description) {
      task.description = description;
    }

    if (sourceType === 'agent' && selectedAgent) {
      const agent = agentsData?.agents.find(
        (a) => `${a.namespace}/${a.name}` === selectedAgent
      );
      if (agent) {
        task.agentRef = { name: agent.name };
      }
    }

    if (sourceType === 'template' && selectedTemplate) {
      const tmpl = templatesData?.templates.find(
        (t) => `${t.namespace}/${t.name}` === selectedTemplate
      );
      if (tmpl) {
        task.templateRef = { name: tmpl.name };
      }
    }

    createMutation.mutate(task);
  };

  const isValid = description && (
    (sourceType === 'agent' && selectedAgent) ||
    (sourceType === 'template' && selectedTemplate)
  );

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'Tasks', to: '/tasks' },
        { label: 'Create Task' },
      ]} />

      <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm max-w-3xl">
        <div className="px-6 py-5 border-b border-stone-100">
          <h2 className="font-display text-xl font-bold text-stone-900">Create Task</h2>
          <p className="text-sm text-stone-400 mt-0.5">Create a new AI agent task</p>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-5">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label
                htmlFor="namespace"
                className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
              >
                Namespace
              </label>
              <SearchableSelect
                id="namespace"
                value={namespace}
                onChange={handleNamespaceChange}
                options={namespacesData?.namespaces.map((ns) => ({ value: ns, label: ns })) || []}
                placeholder="Select namespace..."
              />
            </div>

            <div>
              <label
                htmlFor="name"
                className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
              >
                Name <span className="normal-case tracking-normal text-stone-300">(optional)</span>
              </label>
              <input
                type="text"
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Auto-generated if empty"
                className="block w-full rounded-lg border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 placeholder:text-stone-300"
              />
            </div>
          </div>

          <div>
            <label
              className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
            >
              Source
            </label>
            <div className="flex rounded-lg border border-stone-200 overflow-hidden mb-3">
              <button
                type="button"
                onClick={() => handleSourceTypeChange('agent')}
                className={`flex-1 px-4 py-2 text-sm font-medium transition-colors ${
                  sourceType === 'agent'
                    ? 'bg-primary-50 text-primary-700 border-r border-stone-200'
                    : 'bg-white text-stone-500 hover:bg-stone-50 border-r border-stone-200'
                }`}
              >
                Agent
              </button>
              <button
                type="button"
                onClick={() => handleSourceTypeChange('template')}
                className={`flex-1 px-4 py-2 text-sm font-medium transition-colors ${
                  sourceType === 'template'
                    ? 'bg-primary-50 text-primary-700'
                    : 'bg-white text-stone-500 hover:bg-stone-50'
                }`}
              >
                Agent Template
              </button>
            </div>

            {sourceType === 'agent' ? (
              <>
                <label
                  htmlFor="agent"
                  className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
                >
                  Agent
                </label>
                <SearchableSelect
                  id="agent"
                  value={selectedAgent}
                  onChange={setSelectedAgent}
                  required
                  options={availableAgents.map((agent) => ({
                    value: `${agent.namespace}/${agent.name}`,
                    label: `${agent.namespace}/${agent.name}`,
                  }))}
                  placeholder={availableAgents.length === 0 ? 'No agents available' : 'Select an agent...'}
                />
                <p className="mt-1.5 text-xs text-stone-400">
                  {availableAgents.length === 0
                    ? 'No agents available for this namespace.'
                    : `${availableAgents.length} agent${availableAgents.length !== 1 ? 's' : ''} available`}
                </p>
                {selectedAgent && (
                  <div className="mt-2 flex items-start gap-2 bg-violet-50 border border-violet-200 rounded-lg px-3 py-2">
                    <svg className="w-4 h-4 text-violet-500 mt-0.5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <circle cx="12" cy="12" r="10" />
                      <path d="M12 16v-4M12 8h.01" strokeLinecap="round" />
                    </svg>
                    <p className="text-xs text-violet-700">
                      Task will be sent to the running Agent via <code className="bg-violet-100 px-1 py-0.5 rounded font-mono">--attach</code>. For interactive sessions, use <code className="bg-violet-100 px-1 py-0.5 rounded font-mono">opencode attach</code> instead.
                    </p>
                  </div>
                )}
              </>
            ) : (
              <>
                <label
                  htmlFor="template"
                  className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
                >
                  Agent Template
                </label>
                <SearchableSelect
                  id="template"
                  value={selectedTemplate}
                  onChange={setSelectedTemplate}
                  required
                  options={availableTemplates.map((tmpl) => ({
                    value: `${tmpl.namespace}/${tmpl.name}`,
                    label: `${tmpl.namespace}/${tmpl.name}`,
                  }))}
                  placeholder={availableTemplates.length === 0 ? 'No templates available' : 'Select a template...'}
                />
                <p className="mt-1.5 text-xs text-stone-400">
                  {availableTemplates.length === 0
                    ? 'No templates available for this namespace.'
                    : `${availableTemplates.length} template${availableTemplates.length !== 1 ? 's' : ''} available`}
                </p>
                {selectedTemplate && (
                  <div className="mt-2 flex items-start gap-2 bg-amber-50 border border-amber-200 rounded-lg px-3 py-2">
                    <svg className="w-4 h-4 text-amber-500 mt-0.5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <circle cx="12" cy="12" r="10" />
                      <path d="M12 16v-4M12 8h.01" strokeLinecap="round" />
                    </svg>
                    <p className="text-xs text-amber-700">
                      An ephemeral Pod will be created from this template. The Pod will be cleaned up after the task completes.
                    </p>
                  </div>
                )}
              </>
            )}
          </div>

          <div>
            <label
              htmlFor="description"
              className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
            >
              Description
            </label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={12}
              required
              placeholder="Describe what you want the AI agent to do..."
              className="block w-full rounded-lg border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 font-mono placeholder:text-stone-300 placeholder:font-body"
            />
            <p className="mt-1.5 text-xs text-stone-400">
              This will be the main instruction for the AI agent
            </p>
          </div>

          {createMutation.isError && (
            <div className="bg-red-50 border border-red-200 rounded-lg p-4">
              <p className="text-red-700 text-sm">
                {(createMutation.error as Error).message}
              </p>
            </div>
          )}

          <div className="flex justify-end space-x-3 pt-2">
            <Link
              to="/tasks"
              className="px-4 py-2.5 text-sm font-medium text-stone-600 bg-stone-100 rounded-lg hover:bg-stone-200 transition-colors"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={createMutation.isPending || !isValid}
              className="px-5 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {createMutation.isPending ? 'Creating...' : 'Create Task'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default TaskCreatePage;
