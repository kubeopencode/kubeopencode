// Mock data fixtures for tests

import type { Task, Agent, TaskTemplate, TaskListResponse, AgentListResponse, TaskTemplateListResponse } from '../api/client';

export const mockNamespaces = {
  namespaces: ['default', 'production', 'staging'],
};

export const mockTasks: Task[] = [
  {
    name: 'fix-bug-123',
    namespace: 'default',
    phase: 'Running',
    description: 'Fix authentication bug in login flow',
    agentRef: { name: 'opencode-agent', namespace: 'default' },
    podName: 'fix-bug-123-pod',
    podNamespace: 'default',
    startTime: '2026-02-13T10:00:00Z',
    createdAt: '2026-02-13T09:55:00Z',
    duration: '5m',
    labels: { app: 'myapp', team: 'backend' },
  },
  {
    name: 'add-feature-456',
    namespace: 'default',
    phase: 'Completed',
    description: 'Add user profile page',
    agentRef: { name: 'opencode-agent', namespace: 'default' },
    podName: 'add-feature-456-pod',
    podNamespace: 'default',
    startTime: '2026-02-13T08:00:00Z',
    completionTime: '2026-02-13T08:30:00Z',
    createdAt: '2026-02-13T07:55:00Z',
    duration: '30m',
    conditions: [
      { type: 'Ready', status: 'True', reason: 'TaskCompleted' },
    ],
  },
  {
    name: 'pending-task',
    namespace: 'default',
    phase: 'Pending',
    description: 'Waiting task',
    createdAt: '2026-02-13T11:00:00Z',
  },
];

export const mockTaskListResponse: TaskListResponse = {
  tasks: mockTasks,
  total: mockTasks.length,
  pagination: {
    limit: 20,
    offset: 0,
    totalCount: mockTasks.length,
    hasMore: false,
  },
};

export const mockAgents: Agent[] = [
  {
    name: 'opencode-agent',
    namespace: 'default',
    executorImage: 'quay.io/kubeopencode/kubeopencode-agent-devbox:latest',
    agentImage: 'quay.io/kubeopencode/kubeopencode-agent-opencode:latest',
    workspaceDir: '/workspace',
    contextsCount: 2,
    credentialsCount: 1,
    maxConcurrentTasks: 5,
    createdAt: '2026-02-01T00:00:00Z',
    mode: 'Pod',
    allowedNamespaces: ['default', 'staging'],
    credentials: [
      { name: 'github-token', secretRef: 'github-creds', env: 'GITHUB_TOKEN' },
    ],
    contexts: [
      { name: 'coding-standards', type: 'Text', description: 'Organization coding standards' },
      { name: 'source', type: 'Git', mountPath: 'source-code', description: 'Main repo' },
    ],
    labels: { team: 'platform', tier: 'core' },
  },
  {
    name: 'global-agent',
    namespace: 'platform',
    executorImage: 'quay.io/kubeopencode/kubeopencode-agent-devbox:latest',
    agentImage: 'quay.io/kubeopencode/kubeopencode-agent-opencode:latest',
    workspaceDir: '/workspace',
    contextsCount: 0,
    credentialsCount: 0,
    createdAt: '2026-01-15T00:00:00Z',
    mode: 'Pod',
    // No allowedNamespaces = available to all
  },
  {
    name: 'restricted-agent',
    namespace: 'production',
    executorImage: 'quay.io/kubeopencode/kubeopencode-agent-devbox:latest',
    workspaceDir: '/workspace',
    contextsCount: 0,
    credentialsCount: 0,
    createdAt: '2026-01-20T00:00:00Z',
    mode: 'Server',
    allowedNamespaces: ['production'],
    serverStatus: {
      deploymentName: 'restricted-agent-server',
      serviceName: 'restricted-agent',
      url: 'http://restricted-agent.production.svc.cluster.local:4096',
      readyReplicas: 1,
    },
    conditions: [
      { type: 'ServerReady', status: 'True' },
      { type: 'ServerHealthy', status: 'True' },
    ],
  },
];

export const mockAgentListResponse: AgentListResponse = {
  agents: mockAgents,
  total: mockAgents.length,
  pagination: {
    limit: 20,
    offset: 0,
    totalCount: mockAgents.length,
    hasMore: false,
  },
};

export const mockTemplates: TaskTemplate[] = [
  {
    name: 'pr-template',
    namespace: 'default',
    description: 'Create a pull request following coding standards',
    agentRef: { name: 'opencode-agent', namespace: 'default' },
    contextsCount: 1,
    createdAt: '2026-02-01T00:00:00Z',
    contexts: [
      { name: 'source', type: 'Git', mountPath: 'source-code', description: 'Repository source' },
    ],
  },
  {
    name: 'review-template',
    namespace: 'default',
    description: 'Review code changes',
    contextsCount: 0,
    createdAt: '2026-02-05T00:00:00Z',
  },
];

export const mockTemplateListResponse: TaskTemplateListResponse = {
  templates: mockTemplates,
  total: mockTemplates.length,
  pagination: {
    limit: 20,
    offset: 0,
    totalCount: mockTemplates.length,
    hasMore: false,
  },
};
