// API Client for KubeOpenCode

const API_BASE = '/api/v1';

export interface AgentReference {
  name: string;
  namespace?: string;
}

export interface Condition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
}

export interface Task {
  name: string;
  namespace: string;
  phase: string;
  description?: string;
  agentRef?: AgentReference;
  podName?: string;
  podNamespace?: string;
  startTime?: string;
  completionTime?: string;
  duration?: string;
  createdAt: string;
  conditions?: Condition[];
  labels?: Record<string, string>;
}

export interface Pagination {
  limit: number;
  offset: number;
  totalCount: number;
  hasMore: boolean;
}

export interface TaskListResponse {
  tasks: Task[];
  total: number;
  pagination?: Pagination;
}

export interface FilterParams {
  name?: string;
  labelSelector?: string;
  limit?: number;
  offset?: number;
  sortOrder?: 'asc' | 'desc';
}

export interface ListTasksParams extends FilterParams {
  phase?: string;
}

export interface TaskTemplateReference {
  name: string;
  namespace?: string;
}

export interface CreateTaskRequest {
  name?: string;
  description?: string;
  agentRef?: AgentReference;
  taskTemplateRef?: TaskTemplateReference;
}

export interface ContextItem {
  name?: string;
  description?: string;
  type: string;
  mountPath?: string;
}

export interface CredentialInfo {
  name: string;
  secretRef: string;
  mountPath?: string;
  env?: string;
}

export interface QuotaInfo {
  maxTaskStarts?: number;
  windowSeconds?: number;
}

export interface ServerStatusInfo {
  deploymentName?: string;
  serviceName?: string;
  url?: string;
  readyReplicas: number;
}

export interface Agent {
  name: string;
  namespace: string;
  executorImage?: string;
  agentImage?: string;
  workspaceDir?: string;
  contextsCount: number;
  credentialsCount: number;
  maxConcurrentTasks?: number;
  quota?: QuotaInfo;
  allowedNamespaces?: string[];
  credentials?: CredentialInfo[];
  contexts?: ContextItem[];
  createdAt: string;
  labels?: Record<string, string>;
  mode: string;
  conditions?: Condition[];
  serverStatus?: ServerStatusInfo;
}

export interface AgentListResponse {
  agents: Agent[];
  total: number;
  pagination?: Pagination;
}

export interface TaskTemplate {
  name: string;
  namespace: string;
  description?: string;
  agentRef?: AgentReference;
  contextsCount: number;
  contexts?: ContextItem[];
  createdAt: string;
  labels?: Record<string, string>;
}

export interface TaskTemplateListResponse {
  templates: TaskTemplate[];
  total: number;
  pagination?: Pagination;
}

export interface ServerInfo {
  version: string;
}

export interface NamespaceList {
  namespaces: string[];
}

// Log streaming event types
export interface LogEvent {
  type: 'status' | 'log' | 'error' | 'complete';
  phase?: string;
  podPhase?: string;
  content?: string;
  message?: string;
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.message || error.error || `HTTP ${response.status}`);
  }

  return response.json();
}

export const api = {
  // Info
  getInfo: () => request<ServerInfo>('/info'),
  getNamespaces: () => request<NamespaceList>('/namespaces'),

  // Tasks
  listAllTasks: (params?: ListTasksParams) => {
    const searchParams = new URLSearchParams();
    if (params?.name) searchParams.set('name', params.name);
    if (params?.labelSelector) searchParams.set('labelSelector', params.labelSelector);
    if (params?.phase) searchParams.set('phase', params.phase);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<TaskListResponse>(`/tasks${queryString ? `?${queryString}` : ''}`);
  },

  listTasks: (namespace: string, params?: ListTasksParams) => {
    const searchParams = new URLSearchParams();
    if (params?.name) searchParams.set('name', params.name);
    if (params?.labelSelector) searchParams.set('labelSelector', params.labelSelector);
    if (params?.phase) searchParams.set('phase', params.phase);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<TaskListResponse>(`/namespaces/${namespace}/tasks${queryString ? `?${queryString}` : ''}`);
  },

  getTask: (namespace: string, name: string) =>
    request<Task>(`/namespaces/${namespace}/tasks/${name}`),

  getTaskYaml: async (namespace: string, name: string): Promise<string> => {
    const response = await fetch(`${API_BASE}/namespaces/${namespace}/tasks/${name}?output=yaml`);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    return response.text();
  },

  createTask: (namespace: string, task: CreateTaskRequest) =>
    request<Task>(`/namespaces/${namespace}/tasks`, {
      method: 'POST',
      body: JSON.stringify(task),
    }),

  deleteTask: (namespace: string, name: string) =>
    request<void>(`/namespaces/${namespace}/tasks/${name}`, {
      method: 'DELETE',
    }),

  stopTask: (namespace: string, name: string) =>
    request<Task>(`/namespaces/${namespace}/tasks/${name}/stop`, {
      method: 'POST',
    }),

  // Log streaming - returns an EventSource for SSE
  getTaskLogsUrl: (namespace: string, name: string, container?: string) => {
    const params = new URLSearchParams();
    if (container) params.set('container', container);
    const queryString = params.toString();
    return `${API_BASE}/namespaces/${namespace}/tasks/${name}/logs${queryString ? `?${queryString}` : ''}`;
  },

  // Agents
  listAllAgents: (params?: FilterParams) => {
    const searchParams = new URLSearchParams();
    if (params?.name) searchParams.set('name', params.name);
    if (params?.labelSelector) searchParams.set('labelSelector', params.labelSelector);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<AgentListResponse>(`/agents${queryString ? `?${queryString}` : ''}`);
  },

  listAgents: (namespace: string, params?: FilterParams) => {
    const searchParams = new URLSearchParams();
    if (params?.name) searchParams.set('name', params.name);
    if (params?.labelSelector) searchParams.set('labelSelector', params.labelSelector);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<AgentListResponse>(`/namespaces/${namespace}/agents${queryString ? `?${queryString}` : ''}`);
  },

  getAgent: (namespace: string, name: string) =>
    request<Agent>(`/namespaces/${namespace}/agents/${name}`),

  getAgentYaml: async (namespace: string, name: string): Promise<string> => {
    const response = await fetch(`${API_BASE}/namespaces/${namespace}/agents/${name}?output=yaml`);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    return response.text();
  },

  // TaskTemplates
  listAllTaskTemplates: (params?: FilterParams) => {
    const searchParams = new URLSearchParams();
    if (params?.name) searchParams.set('name', params.name);
    if (params?.labelSelector) searchParams.set('labelSelector', params.labelSelector);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<TaskTemplateListResponse>(`/tasktemplates${queryString ? `?${queryString}` : ''}`);
  },

  listTaskTemplates: (namespace: string, params?: FilterParams) => {
    const searchParams = new URLSearchParams();
    if (params?.name) searchParams.set('name', params.name);
    if (params?.labelSelector) searchParams.set('labelSelector', params.labelSelector);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<TaskTemplateListResponse>(`/namespaces/${namespace}/tasktemplates${queryString ? `?${queryString}` : ''}`);
  },

  getTaskTemplate: (namespace: string, name: string) =>
    request<TaskTemplate>(`/namespaces/${namespace}/tasktemplates/${name}`),

  getTaskTemplateYaml: async (namespace: string, name: string): Promise<string> => {
    const response = await fetch(`${API_BASE}/namespaces/${namespace}/tasktemplates/${name}?output=yaml`);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    return response.text();
  },
};

export default api;
