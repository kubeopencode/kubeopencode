import React, { useState, useMemo, useEffect } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import api, { CreateTaskRequest } from '../api/client';
import { useToast } from '../contexts/ToastContext';
import Breadcrumbs from '../components/Breadcrumbs';

function TaskCreatePage() {
  const navigate = useNavigate();
  const { addToast } = useToast();
  const [searchParams] = useSearchParams();
  const [namespace, setNamespace] = useState('default');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [selectedAgent, setSelectedAgent] = useState('');

  const { data: namespacesData } = useQuery({
    queryKey: ['namespaces'],
    queryFn: () => api.getNamespaces(),
  });

  const { data: agentsData } = useQuery({
    queryKey: ['agents'],
    queryFn: () => api.listAllAgents(),
  });

  // Query for rerun task data
  const rerunTaskName = searchParams.get('rerun');
  const rerunNamespace = searchParams.get('namespace') || 'default';
  const { data: rerunTask } = useQuery({
    queryKey: ['task', rerunNamespace, rerunTaskName],
    queryFn: () => api.getTask(rerunNamespace, rerunTaskName!),
    enabled: !!rerunTaskName,
  });

  // Parse query params for pre-selection
  useEffect(() => {
    const namespaceParam = searchParams.get('namespace');
    if (namespaceParam) {
      setNamespace(namespaceParam);
    }
    const agentParam = searchParams.get('agent');
    if (agentParam) {
      setSelectedAgent(agentParam);
    }
  }, [searchParams]);

  // Pre-fill from rerun task
  useEffect(() => {
    if (rerunTask) {
      if (rerunTask.description) {
        setDescription(rerunTask.description);
      }
      if (rerunTask.agentRef) {
        const agentKey = `${rerunTask.namespace}/${rerunTask.agentRef.name}`;
        setSelectedAgent(agentKey);
      }
    }
  }, [rerunTask]);

  // Filter agents to same namespace
  const availableAgents = useMemo(() => {
    if (!agentsData?.agents) return [];
    return agentsData.agents.filter((agent) => agent.namespace === namespace);
  }, [agentsData?.agents, namespace]);

  // Reset selected agent if it's no longer available for the new namespace
  const handleNamespaceChange = (newNamespace: string) => {
    setNamespace(newNamespace);
    if (selectedAgent) {
      const agent = agentsData?.agents.find(
        (a) => `${a.namespace}/${a.name}` === selectedAgent
      );
      if (agent && agent.namespace !== newNamespace) {
        setSelectedAgent('');
      }
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

    if (selectedAgent) {
      const agent = agentsData?.agents.find(
        (a) => `${a.namespace}/${a.name}` === selectedAgent
      );
      if (agent) {
        task.agentRef = {
          name: agent.name,
        };
      }
    }

    createMutation.mutate(task);
  };

  // Determine if form is valid
  const isValid = description && selectedAgent;

  return (
    <div>
      <Breadcrumbs items={[
        { label: 'Tasks', to: `/tasks?namespace=${namespace}` },
        { label: 'Create Task' },
      ]} />

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

          <div>
            <label
              htmlFor="agent"
              className="block text-sm font-medium text-gray-700"
            >
              Agent
            </label>
            <select
              id="agent"
              value={selectedAgent}
              onChange={(e) => setSelectedAgent(e.target.value)}
              required
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm"
            >
              <option value="">
                {availableAgents.length === 0
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
              {availableAgents.length === 0
                ? 'No agents available for this namespace. Contact your administrator.'
                : `${availableAgents.length} agent${availableAgents.length !== 1 ? 's' : ''} available for this namespace`}
            </p>
          </div>

          <div>
            <label
              htmlFor="description"
              className="block text-sm font-medium text-gray-700"
            >
              Description / Task Prompt
            </label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={10}
              required
              placeholder="Describe what you want the AI agent to do..."
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary-500 focus:ring-primary-500 sm:text-sm font-mono"
            />
            <p className="mt-1 text-sm text-gray-500">
              This will be the main instruction for the AI agent
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
