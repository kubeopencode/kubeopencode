import React, { useState, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import api, { CreateCronTaskRequest } from '../api/client';
import { useToast } from '../contexts/ToastContext';
import { useNamespace } from '../contexts/NamespaceContext';
import Breadcrumbs from '../components/Breadcrumbs';
import SearchableSelect from '../components/SearchableSelect';

type SourceType = 'agent' | 'template';

function CronTaskCreatePage() {
  const navigate = useNavigate();
  const { addToast } = useToast();
  const { namespace: globalNamespace, isAllNamespaces, setNamespace: setGlobalNamespace } = useNamespace();
  const [namespace, setNamespace] = useState(() => {
    return isAllNamespaces ? 'default' : globalNamespace;
  });
  const [name, setName] = useState('');
  const [schedule, setSchedule] = useState('');
  const [timeZone, setTimeZone] = useState('UTC');
  const [concurrencyPolicy, setConcurrencyPolicy] = useState('Forbid');
  const [description, setDescription] = useState('');
  const [sourceType, setSourceType] = useState<SourceType>('agent');
  const [selectedAgent, setSelectedAgent] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [maxRetainedTasks, setMaxRetainedTasks] = useState(10);
  const [startingDeadlineSeconds, setStartingDeadlineSeconds] = useState<number | ''>('');

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
    mutationFn: (cronTask: CreateCronTaskRequest) => api.createCronTask(namespace, cronTask),
    onSuccess: (ct) => {
      addToast(`CronTask "${ct.name}" created successfully`, 'success');
      navigate(`/crontasks/${ct.namespace}/${ct.name}`);
    },
    onError: (err: Error) => {
      addToast(`Failed to create CronTask: ${err.message}`, 'error');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    const req: CreateCronTaskRequest = {
      schedule,
      timeZone: timeZone || undefined,
      concurrencyPolicy,
    };

    if (name) {
      req.name = name;
    }

    if (description) {
      req.description = description;
    }

    if (maxRetainedTasks !== 10) {
      req.maxRetainedTasks = maxRetainedTasks;
    }

    if (startingDeadlineSeconds !== '' && startingDeadlineSeconds > 0) {
      req.startingDeadlineSeconds = startingDeadlineSeconds;
    }

    if (sourceType === 'agent' && selectedAgent) {
      const agent = agentsData?.agents.find(
        (a) => `${a.namespace}/${a.name}` === selectedAgent
      );
      if (agent) {
        req.agentRef = { name: agent.name };
      }
    }

    if (sourceType === 'template' && selectedTemplate) {
      const tmpl = templatesData?.templates.find(
        (t) => `${t.namespace}/${t.name}` === selectedTemplate
      );
      if (tmpl) {
        req.templateRef = { name: tmpl.name };
      }
    }

    createMutation.mutate(req);
  };

  const isValid = schedule && (
    (sourceType === 'agent' && selectedAgent) ||
    (sourceType === 'template' && selectedTemplate)
  );

  return (
    <div className="animate-fade-in">
      <Breadcrumbs items={[
        { label: 'CronTasks', to: '/crontasks' },
        { label: 'Create CronTask' },
      ]} />

      <div className="bg-white rounded-xl border border-stone-200 overflow-hidden shadow-sm max-w-3xl">
        <div className="px-6 py-5 border-b border-stone-100">
          <h2 className="font-display text-xl font-bold text-stone-900">Create CronTask</h2>
          <p className="text-sm text-stone-400 mt-0.5">Schedule a recurring AI agent task</p>
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

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label
                htmlFor="schedule"
                className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
              >
                Schedule
              </label>
              <input
                type="text"
                id="schedule"
                value={schedule}
                onChange={(e) => setSchedule(e.target.value)}
                required
                placeholder="0 9 * * 1-5"
                className="block w-full rounded-lg border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 font-mono placeholder:text-stone-300 placeholder:font-body"
              />
              <p className="mt-1.5 text-xs text-stone-400">
                Cron expression, e.g. <code className="bg-stone-100 px-1 py-0.5 rounded font-mono">0 9 * * 1-5</code> (weekdays at 9am)
              </p>
            </div>

            <div>
              <label
                htmlFor="timezone"
                className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
              >
                Timezone
              </label>
              <input
                type="text"
                id="timezone"
                value={timeZone}
                onChange={(e) => setTimeZone(e.target.value)}
                placeholder="UTC"
                className="block w-full rounded-lg border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 placeholder:text-stone-300"
              />
            </div>
          </div>

          <div>
            <label
              htmlFor="concurrencyPolicy"
              className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
            >
              Concurrency Policy
            </label>
            <select
              id="concurrencyPolicy"
              value={concurrencyPolicy}
              onChange={(e) => setConcurrencyPolicy(e.target.value)}
              className="block w-full rounded-lg border-stone-200 bg-white shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700"
            >
              <option value="Forbid">Forbid - Skip new if previous is still running</option>
              <option value="Allow">Allow - Allow concurrent executions</option>
              <option value="Replace">Replace - Cancel previous, start new</option>
            </select>
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
              rows={6}
              placeholder="Describe what you want the AI agent to do on each run..."
              className="block w-full rounded-lg border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 font-mono placeholder:text-stone-300 placeholder:font-body"
            />
            <p className="mt-1.5 text-xs text-stone-400">
              This will be the instruction for each scheduled task execution
            </p>
          </div>

          {/* Advanced section */}
          <div className="border border-stone-200 rounded-lg">
            <button
              type="button"
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="w-full px-4 py-3 flex items-center justify-between text-sm font-medium text-stone-600 hover:bg-stone-50 transition-colors rounded-lg"
            >
              <span>Advanced Settings</span>
              <svg
                className={`w-4 h-4 transition-transform ${showAdvanced ? 'rotate-180' : ''}`}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <path d="M6 9l6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>
            {showAdvanced && (
              <div className="px-4 pb-4 pt-1 space-y-4 border-t border-stone-100">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label
                      htmlFor="maxRetainedTasks"
                      className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
                    >
                      Max Retained Tasks
                    </label>
                    <input
                      type="number"
                      id="maxRetainedTasks"
                      value={maxRetainedTasks}
                      onChange={(e) => setMaxRetainedTasks(Number(e.target.value))}
                      min={0}
                      className="block w-full rounded-lg border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700"
                    />
                    <p className="mt-1.5 text-xs text-stone-400">
                      Number of completed Tasks to retain
                    </p>
                  </div>

                  <div>
                    <label
                      htmlFor="startingDeadlineSeconds"
                      className="block text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-1.5"
                    >
                      Starting Deadline (seconds)
                    </label>
                    <input
                      type="number"
                      id="startingDeadlineSeconds"
                      value={startingDeadlineSeconds}
                      onChange={(e) => setStartingDeadlineSeconds(e.target.value ? Number(e.target.value) : '')}
                      min={0}
                      placeholder="No deadline"
                      className="block w-full rounded-lg border-stone-200 shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 placeholder:text-stone-300"
                    />
                    <p className="mt-1.5 text-xs text-stone-400">
                      Deadline for starting a missed scheduled task
                    </p>
                  </div>
                </div>
              </div>
            )}
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
              to="/crontasks"
              className="px-4 py-2.5 text-sm font-medium text-stone-600 bg-stone-100 rounded-lg hover:bg-stone-200 transition-colors"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={createMutation.isPending || !isValid}
              className="px-5 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {createMutation.isPending ? 'Creating...' : 'Create CronTask'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default CronTaskCreatePage;
