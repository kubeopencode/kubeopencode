import React, { useState, useMemo, useEffect } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import api, { CreateTaskRequest, Agent } from '../api/client';

// Check if a namespace matches a glob pattern
function matchGlob(pattern: string, namespace: string): boolean {
  // Convert glob pattern to regex
  const regexPattern = pattern
    .replace(/[.+^${}()|[\]\\]/g, '\\$&') // Escape special regex chars except * and ?
    .replace(/\*/g, '.*') // * matches any string
    .replace(/\?/g, '.'); // ? matches single char
  const regex = new RegExp(`^${regexPattern}$`);
  return regex.test(namespace);
}

// Check if an agent is available for a given namespace
function isAgentAvailableForNamespace(agent: Agent, namespace: string): boolean {
  // If no allowedNamespaces, agent is available to all namespaces
  if (!agent.allowedNamespaces || agent.allowedNamespaces.length === 0) {
    return true;
  }
  // Check if any pattern matches the namespace
  return agent.allowedNamespaces.some((pattern) => matchGlob(pattern, namespace));
}

function TaskCreatePage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [namespace, setNamespace] = useState('default');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [selectedAgent, setSelectedAgent] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState('');
  const [useTemplate, setUseTemplate] = useState(false);

  const { data: namespacesData } = useQuery({
    queryKey: ['namespaces'],
    queryFn: () => api.getNamespaces(),
  });

  const { data: agentsData } = useQuery({
    queryKey: ['agents'],
    queryFn: () => api.listAllAgents(),
  });

  const { data: templatesData } = useQuery({
    queryKey: ['tasktemplates'],
    queryFn: () => api.listAllTaskTemplates(),
  });

  // Parse query params for pre-selection
  useEffect(() => {
    const namespaceParam = searchParams.get('namespace');
    if (namespaceParam) {
      setNamespace(namespaceParam);
    }
    const templateParam = searchParams.get('template');
    if (templateParam) {
      setUseTemplate(true);
      setSelectedTemplate(templateParam);
    }
    const agentParam = searchParams.get('agent');
    if (agentParam) {
      setSelectedAgent(agentParam);
    }
  }, [searchParams]);

  // Filter agents based on allowedNamespaces
  const availableAgents = useMemo(() => {
    if (!agentsData?.agents) return [];
    return agentsData.agents.filter((agent) =>
      isAgentAvailableForNamespace(agent, namespace)
    );
  }, [agentsData?.agents, namespace]);

  // Get selected template details
  const selectedTemplateDetails = useMemo(() => {
    if (!selectedTemplate || !templatesData?.templates) return null;
    const [ns, nm] = selectedTemplate.split('/');
    return templatesData.templates.find((t) => t.namespace === ns && t.name === nm);
  }, [selectedTemplate, templatesData?.templates]);

  // Reset selected agent if it's no longer available for the new namespace
  const handleNamespaceChange = (newNamespace: string) => {
    setNamespace(newNamespace);
    if (selectedAgent) {
      const agent = agentsData?.agents.find(
        (a) => `${a.namespace}/${a.name}` === selectedAgent
      );
      if (agent && !isAgentAvailableForNamespace(agent, newNamespace)) {
        setSelectedAgent('');
      }
    }
  };

  const createMutation = useMutation({
    mutationFn: (task: CreateTaskRequest) => api.createTask(namespace, task),
    onSuccess: (task) => {
      navigate(`/tasks/${task.namespace}/${task.name}`);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    const task: CreateTaskRequest = {};

    if (name) {
      task.name = name;
    }

    // Set template reference if using a template
    if (useTemplate && selectedTemplate) {
      const [templateNs, templateName] = selectedTemplate.split('/');
      task.taskTemplateRef = {
        name: templateName,
        namespace: templateNs,
      };
    }

    // Description overrides template's description if provided
    if (description) {
      task.description = description;
    }

    // Agent overrides template's agent if provided
    if (selectedAgent) {
      const agent = agentsData?.agents.find(
        (a) => `${a.namespace}/${a.name}` === selectedAgent
      );
      if (agent) {
        task.agentRef = {
          name: agent.name,
          namespace: agent.namespace,
        };
      }
    }

    createMutation.mutate(task);
  };

  // Determine if form is valid
  const isValid = useMemo(() => {
    // If using template, we need either template OR (description AND agent)
    if (useTemplate && selectedTemplate) {
      // Template provides defaults, so we just need the template
      // But if no description and template has no description, we need one
      const hasDescription = description || selectedTemplateDetails?.description;
      const hasAgent = selectedAgent || selectedTemplateDetails?.agentRef;
      return hasDescription && hasAgent;
    }
    // If not using template, we need description and agent
    return description && selectedAgent;
  }, [useTemplate, selectedTemplate, description, selectedAgent, selectedTemplateDetails]);

  return (
    <div>
      <div className="mb-6">
        <Link to={`/tasks?namespace=${namespace}`} className="text-sm text-gray-500 hover:text-gray-700">
          &larr; Back to Tasks
        </Link>
      </div>

      <div className="bg-white shadow-sm rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-xl font-bold text-gray-900">Create Task</h2>
          <p className="text-sm text-gray-500">Create a new AI agent task</p>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-4 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label
                htmlFor="namespace"
                className="block text-sm font-medium text-gray-700"
              >
                Namespace
              </label>
              <select
                id="namespace"
                value={namespace}
                onChange={(e) => handleNamespaceChange(e.target.value)}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm"
              >
                {namespacesData?.namespaces.map((ns) => (
                  <option key={ns} value={ns}>
                    {ns}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label
                htmlFor="name"
                className="block text-sm font-medium text-gray-700"
              >
                Name (optional)
              </label>
              <input
                type="text"
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Auto-generated if empty"
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm"
              />
            </div>
          </div>

          {/* Template Toggle */}
          <div className="border border-gray-200 rounded-md p-4 bg-gray-50">
            <div className="flex items-center">
              <input
                id="use-template"
                type="checkbox"
                checked={useTemplate}
                onChange={(e) => {
                  setUseTemplate(e.target.checked);
                  if (!e.target.checked) {
                    setSelectedTemplate('');
                  }
                }}
                className="h-4 w-4 text-primary-600 focus:ring-primary-500 border-gray-300 rounded"
              />
              <label htmlFor="use-template" className="ml-2 block text-sm font-medium text-gray-700">
                Use a template
              </label>
            </div>

            {useTemplate && (
              <div className="mt-3">
                <select
                  id="template"
                  value={selectedTemplate}
                  onChange={(e) => setSelectedTemplate(e.target.value)}
                  className="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm"
                >
                  <option value="">Select a template...</option>
                  {templatesData?.templates.map((template) => (
                    <option
                      key={`${template.namespace}/${template.name}`}
                      value={`${template.namespace}/${template.name}`}
                    >
                      {template.namespace}/{template.name}
                    </option>
                  ))}
                </select>
                {selectedTemplateDetails && (
                  <div className="mt-2 text-sm text-gray-600">
                    <p>
                      <span className="font-medium">Agent:</span>{' '}
                      {selectedTemplateDetails.agentRef
                        ? `${selectedTemplateDetails.agentRef.namespace || selectedTemplateDetails.namespace}/${selectedTemplateDetails.agentRef.name}`
                        : 'None (must specify below)'}
                    </p>
                    {selectedTemplateDetails.description && (
                      <p className="mt-1">
                        <span className="font-medium">Default description:</span>{' '}
                        <span className="text-gray-500">(provided by template)</span>
                      </p>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>

          <div>
            <label
              htmlFor="agent"
              className="block text-sm font-medium text-gray-700"
            >
              Agent {useTemplate && selectedTemplateDetails?.agentRef ? '(optional - template provides default)' : ''}
            </label>
            <select
              id="agent"
              value={selectedAgent}
              onChange={(e) => setSelectedAgent(e.target.value)}
              required={!useTemplate || !selectedTemplateDetails?.agentRef}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm"
            >
              <option value="">
                {useTemplate && selectedTemplateDetails?.agentRef
                  ? 'Use template default'
                  : availableAgents.length === 0
                    ? 'No agents available'
                    : 'Select an agent...'}
              </option>
              {availableAgents.map((agent) => (
                <option
                  key={`${agent.namespace}/${agent.name}`}
                  value={`${agent.namespace}/${agent.name}`}
                >
                  {agent.namespace}/{agent.name}
                </option>
              ))}
            </select>
            <p className="mt-1 text-sm text-gray-500">
              {availableAgents.length === 0 && !useTemplate
                ? 'No agents available for this namespace. Contact your administrator.'
                : `${availableAgents.length} agent${availableAgents.length !== 1 ? 's' : ''} available for this namespace`}
            </p>
          </div>

          <div>
            <label
              htmlFor="description"
              className="block text-sm font-medium text-gray-700"
            >
              Description / Task Prompt {useTemplate && selectedTemplateDetails?.description ? '(optional - template provides default)' : ''}
            </label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={10}
              required={!useTemplate || !selectedTemplateDetails?.description}
              placeholder={
                useTemplate && selectedTemplateDetails?.description
                  ? 'Leave empty to use template default, or enter to override...'
                  : 'Describe what you want the AI agent to do...'
              }
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm font-mono"
            />
            <p className="mt-1 text-sm text-gray-500">
              {useTemplate && selectedTemplateDetails?.description
                ? 'Template provides a default description. Enter your own to override it.'
                : 'This will be the main instruction for the AI agent'}
            </p>
          </div>

          {createMutation.isError && (
            <div className="bg-red-50 border border-red-200 rounded-lg p-4">
              <p className="text-red-800">
                Error: {(createMutation.error as Error).message}
              </p>
            </div>
          )}

          <div className="flex justify-end space-x-4">
            <Link
              to={`/tasks?namespace=${namespace}`}
              className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={createMutation.isPending || !isValid}
              className="px-4 py-2 text-sm font-medium text-white bg-primary-600 rounded-md hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed"
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
