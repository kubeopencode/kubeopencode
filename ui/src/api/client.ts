// API Client for KubeOpenCode

const API_BASE = '/api/v1';

export interface AgentReference {
  name: string;
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
  templateRef?: AgentReference;
  podName?: string;
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

export interface CreateTaskRequest {
  name?: string;
  description?: string;
  agentRef?: AgentReference;
  templateRef?: { name: string };
}

export interface CreateVolumePersistence {
  storageClassName?: string;
  size?: string;
}

export interface CreatePersistenceConfig {
  sessions?: CreateVolumePersistence;
  workspace?: CreateVolumePersistence;
}

export interface CreateAgentRequest {
  name: string;
  profile?: string;
  templateRef?: AgentReference;
  workspaceDir?: string;
  serviceAccountName?: string;
  // P0: Images
  agentImage?: string;
  executorImage?: string;
  // P1: Common configuration
  maxConcurrentTasks?: number;
  standby?: StandbyInfo;
  persistence?: CreatePersistenceConfig;
  // P2: Advanced
  port?: number;
  proxy?: ProxyConfigInfo;
}

export interface StandbyInfo {
  idleTimeout: string;
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
  ready: boolean;
  port?: number;
  suspended: boolean;
  idleSince?: string;
}

export interface Agent {
  name: string;
  namespace: string;
  profile?: string;
  templateRef?: AgentReference;
  executorImage?: string;
  agentImage?: string;
  workspaceDir?: string;
  serviceAccountName?: string;
  contextsCount: number;
  credentialsCount: number;
  maxConcurrentTasks?: number;
  quota?: QuotaInfo;
  credentials?: CredentialInfo[];
  contexts?: ContextItem[];
  createdAt: string;
  labels?: Record<string, string>;
  standby?: StandbyInfo;
  conditions?: Condition[];
  serverStatus?: ServerStatusInfo;
}

export interface AgentTemplate {
  name: string;
  namespace: string;
  agentImage?: string;
  executorImage?: string;
  workspaceDir?: string;
  serviceAccountName?: string;
  contextsCount: number;
  credentialsCount: number;
  credentials?: CredentialInfo[];
  contexts?: ContextItem[];
  createdAt: string;
  labels?: Record<string, string>;
  conditions?: Condition[];
  agentCount: number;
}

export interface AgentTemplateListResponse {
  templates: AgentTemplate[];
  total: number;
  pagination?: Pagination;
}

export interface AgentListResponse {
  agents: Agent[];
  total: number;
  pagination?: Pagination;
}

export interface SystemImageConfig {
  image?: string;
  imagePullPolicy?: string;
}

export interface CleanupConfig {
  ttlSecondsAfterFinished?: number;
  maxRetainedTasks?: number;
}

export interface ProxyConfigInfo {
  httpProxy?: string;
  httpsProxy?: string;
  noProxy?: string;
}

export interface ConfigResponse {
  name: string;
  createdAt: string;
  systemImage?: SystemImageConfig;
  cleanup?: CleanupConfig;
  proxy?: ProxyConfigInfo;
  labels?: Record<string, string>;
}

// CronTask types
export interface CronTaskTemplateInfo {
  description?: string;
  agentRef?: AgentReference;
  templateRef?: AgentReference;
}

export interface CronTask {
  name: string;
  namespace: string;
  schedule: string;
  timeZone?: string;
  concurrencyPolicy: string;
  suspend: boolean;
  maxRetainedTasks?: number;
  startingDeadlineSeconds?: number;
  active: number;
  lastScheduleTime?: string;
  lastSuccessfulTime?: string;
  nextScheduleTime?: string;
  totalExecutions: number;
  taskTemplate: CronTaskTemplateInfo;
  createdAt: string;
  labels?: Record<string, string>;
  conditions?: Condition[];
}

export interface CronTaskListResponse {
  cronTasks: CronTask[];
  total: number;
  pagination?: Pagination;
}

export interface CreateCronTaskRequest {
  name?: string;
  schedule: string;
  timeZone?: string;
  concurrencyPolicy?: string;
  suspend?: boolean;
  startingDeadlineSeconds?: number;
  maxRetainedTasks?: number;
  description?: string;
  agentRef?: AgentReference;
  templateRef?: { name: string };
  contexts?: ContextItem[];
}

export interface ServerInfo {
  version: string;
}

export interface NamespaceList {
  namespaces: string[];
}

// Log streaming event types
export interface LogEvent {
  type: 'status' | 'log' | 'error' | 'info' | 'complete';
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

  if (response.status === 204) {
    return undefined as T;
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

  createAgent: (namespace: string, agent: CreateAgentRequest) =>
    request<Agent>(`/namespaces/${namespace}/agents`, {
      method: 'POST',
      body: JSON.stringify(agent),
    }),

  suspendAgent: (namespace: string, name: string) =>
    request<Agent>(`/namespaces/${namespace}/agents/${name}/suspend`, { method: 'POST' }),

  resumeAgent: (namespace: string, name: string) =>
    request<Agent>(`/namespaces/${namespace}/agents/${name}/resume`, { method: 'POST' }),

  updateAgentYaml: async (namespace: string, name: string, yaml: string): Promise<void> => {
    const response = await fetch(`${API_BASE}/namespaces/${namespace}/agents/${name}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/x-yaml' },
      body: yaml,
    });
    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Unknown error' }));
      throw new Error(error.message || error.error || `HTTP ${response.status}`);
    }
  },

  deleteAgent: (namespace: string, name: string) =>
    request<void>(`/namespaces/${namespace}/agents/${name}`, { method: 'DELETE' }),

  // Agent Templates
  listAllAgentTemplates: (params?: FilterParams) => {
    const searchParams = new URLSearchParams();
    if (params?.name) searchParams.set('name', params.name);
    if (params?.labelSelector) searchParams.set('labelSelector', params.labelSelector);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<AgentTemplateListResponse>(`/agenttemplates${queryString ? `?${queryString}` : ''}`);
  },

  listAgentTemplates: (namespace: string, params?: FilterParams) => {
    const searchParams = new URLSearchParams();
    if (params?.name) searchParams.set('name', params.name);
    if (params?.labelSelector) searchParams.set('labelSelector', params.labelSelector);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<AgentTemplateListResponse>(`/namespaces/${namespace}/agenttemplates${queryString ? `?${queryString}` : ''}`);
  },

  getAgentTemplate: (namespace: string, name: string) =>
    request<AgentTemplate>(`/namespaces/${namespace}/agenttemplates/${name}`),

  deleteAgentTemplate: (namespace: string, name: string) =>
    request<void>(`/namespaces/${namespace}/agenttemplates/${name}`, { method: 'DELETE' }),

  getAgentTemplateYaml: async (namespace: string, name: string): Promise<string> => {
    const response = await fetch(`${API_BASE}/namespaces/${namespace}/agenttemplates/${name}?output=yaml`);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    return response.text();
  },

  updateAgentTemplateYaml: async (namespace: string, name: string, yaml: string): Promise<void> => {
    const response = await fetch(`${API_BASE}/namespaces/${namespace}/agenttemplates/${name}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/x-yaml' },
      body: yaml,
    });
    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Unknown error' }));
      throw new Error(error.message || error.error || `HTTP ${response.status}`);
    }
  },

  // CronTasks
  listAllCronTasks: (params?: FilterParams) => {
    const searchParams = new URLSearchParams();
    if (params?.name) searchParams.set('name', params.name);
    if (params?.labelSelector) searchParams.set('labelSelector', params.labelSelector);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<CronTaskListResponse>(`/crontasks${queryString ? `?${queryString}` : ''}`);
  },

  listCronTasks: (namespace: string, params?: FilterParams) => {
    const searchParams = new URLSearchParams();
    if (params?.name) searchParams.set('name', params.name);
    if (params?.labelSelector) searchParams.set('labelSelector', params.labelSelector);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<CronTaskListResponse>(`/namespaces/${namespace}/crontasks${queryString ? `?${queryString}` : ''}`);
  },

  getCronTask: (namespace: string, name: string) =>
    request<CronTask>(`/namespaces/${namespace}/crontasks/${name}`),

  getCronTaskYaml: async (namespace: string, name: string): Promise<string> => {
    const response = await fetch(`${API_BASE}/namespaces/${namespace}/crontasks/${name}?output=yaml`);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    return response.text();
  },

  createCronTask: (namespace: string, cronTask: CreateCronTaskRequest) =>
    request<CronTask>(`/namespaces/${namespace}/crontasks`, {
      method: 'POST',
      body: JSON.stringify(cronTask),
    }),

  updateCronTask: (namespace: string, name: string, data: Partial<CreateCronTaskRequest>) =>
    request<CronTask>(`/namespaces/${namespace}/crontasks/${name}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  deleteCronTask: (namespace: string, name: string) =>
    request<void>(`/namespaces/${namespace}/crontasks/${name}`, {
      method: 'DELETE',
    }),

  suspendCronTask: (namespace: string, name: string) =>
    request<CronTask>(`/namespaces/${namespace}/crontasks/${name}/suspend`, { method: 'POST' }),

  resumeCronTask: (namespace: string, name: string) =>
    request<CronTask>(`/namespaces/${namespace}/crontasks/${name}/resume`, { method: 'POST' }),

  triggerCronTask: (namespace: string, name: string) =>
    request<{ message: string }>(`/namespaces/${namespace}/crontasks/${name}/trigger`, { method: 'POST' }),

  getCronTaskHistory: (namespace: string, name: string, params?: FilterParams) => {
    const searchParams = new URLSearchParams();
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset !== undefined) searchParams.set('offset', params.offset.toString());
    if (params?.sortOrder) searchParams.set('sortOrder', params.sortOrder);
    const queryString = searchParams.toString();
    return request<TaskListResponse>(`/namespaces/${namespace}/crontasks/${name}/history${queryString ? `?${queryString}` : ''}`);
  },

  // Config (cluster-scoped singleton)
  getConfig: () => request<ConfigResponse>('/config'),

  getConfigYaml: async (): Promise<string> => {
    const response = await fetch(`${API_BASE}/config?output=yaml`);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    return response.text();
  },

};

export default api;
