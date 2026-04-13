// MSW request handlers for API mocking (tests + browser dev mode)

import { http, HttpResponse } from 'msw';
import {
  mockNamespaces,
  mockTasks,
  mockAgents,
  mockAgentTemplates,
  mockCronTasks,
  mockCronTaskHistory,
  mockConfig,
  paginateList,
} from './data';
import type { Task, Agent, AgentTemplate, CronTask } from '../api/client';

const API_BASE = '/api/v1';

function filterByName<T extends { name: string }>(items: T[], name?: string | null): T[] {
  if (!name) return items;
  return items.filter((item) => item.name.includes(name));
}

function filterByNamespace<T extends { namespace: string }>(items: T[], namespace?: string): T[] {
  if (!namespace) return items;
  return items.filter((item) => item.namespace === namespace);
}

function filterByLabels<T extends { labels?: Record<string, string> }>(items: T[], selector?: string | null): T[] {
  if (!selector) return items;
  const parts = selector.split(',').map((s) => s.trim());
  return items.filter((item) => {
    return parts.every((part) => {
      if (part.startsWith('!')) {
        const key = part.slice(1);
        return !item.labels?.[key];
      }
      if (part.includes('=')) {
        const [key, value] = part.split('=');
        return item.labels?.[key] === value;
      }
      // Key existence check
      return item.labels?.[part] !== undefined;
    });
  });
}

function parseListParams(url: URL) {
  return {
    name: url.searchParams.get('name'),
    labelSelector: url.searchParams.get('labelSelector'),
    limit: parseInt(url.searchParams.get('limit') || '20', 10),
    offset: parseInt(url.searchParams.get('offset') || '0', 10),
    sortOrder: url.searchParams.get('sortOrder') || 'desc',
    phase: url.searchParams.get('phase'),
  };
}

function sortByCreatedAt<T extends { createdAt: string }>(items: T[], order: string): T[] {
  return [...items].sort((a, b) => {
    const diff = new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
    return order === 'asc' ? diff : -diff;
  });
}

function buildTaskListResponse(tasks: Task[], url: URL) {
  const params = parseListParams(url);
  let filtered = filterByName(tasks, params.name);
  filtered = filterByLabels(filtered, params.labelSelector);
  if (params.phase) {
    filtered = filtered.filter((t) => t.phase === params.phase);
  }
  filtered = sortByCreatedAt(filtered, params.sortOrder);
  const { items, pagination } = paginateList(filtered, params.limit, params.offset);
  return { tasks: items, total: filtered.length, pagination };
}

function buildAgentListResponse(agents: Agent[], url: URL) {
  const params = parseListParams(url);
  let filtered = filterByName(agents, params.name);
  filtered = filterByLabels(filtered, params.labelSelector);
  filtered = sortByCreatedAt(filtered, params.sortOrder);
  const { items, pagination } = paginateList(filtered, params.limit, params.offset);
  return { agents: items, total: filtered.length, pagination };
}

function buildCronTaskListResponse(cronTasks: CronTask[], url: URL) {
  const params = parseListParams(url);
  let filtered = filterByName(cronTasks, params.name);
  filtered = filterByLabels(filtered, params.labelSelector);
  filtered = sortByCreatedAt(filtered, params.sortOrder);
  const { items, pagination } = paginateList(filtered, params.limit, params.offset);
  return { cronTasks: items, total: filtered.length, pagination };
}

function buildTemplateListResponse(templates: AgentTemplate[], url: URL) {
  const params = parseListParams(url);
  let filtered = filterByName(templates, params.name);
  filtered = filterByLabels(filtered, params.labelSelector);
  filtered = sortByCreatedAt(filtered, params.sortOrder);
  const { items, pagination } = paginateList(filtered, params.limit, params.offset);
  return { templates: items, total: filtered.length, pagination };
}

export const handlers = [
  // === Server info ===
  http.get(`${API_BASE}/info`, () => {
    return HttpResponse.json({ version: '0.0.15-mock' });
  }),

  // === Namespaces ===
  http.get(`${API_BASE}/namespaces`, () => {
    return HttpResponse.json(mockNamespaces);
  }),

  // === Config ===
  http.put(`${API_BASE}/config`, () => {
    return HttpResponse.json(mockConfig);
  }),

  http.get(`${API_BASE}/config`, ({ request }) => {
    const url = new URL(request.url);
    if (url.searchParams.get('output') === 'yaml') {
      return new HttpResponse(
        `apiVersion: kubeopencode.io/v1alpha1\nkind: KubeOpenCodeConfig\nmetadata:\n  name: cluster\nspec:\n  cleanup:\n    ttlSecondsAfterFinished: 86400\n    maxRetainedTasks: 100`,
        { headers: { 'Content-Type': 'text/plain' } },
      );
    }
    return HttpResponse.json(mockConfig);
  }),

  // === Tasks ===
  http.get(`${API_BASE}/tasks`, ({ request }) => {
    const url = new URL(request.url);
    return HttpResponse.json(buildTaskListResponse(mockTasks, url));
  }),

  http.get(`${API_BASE}/namespaces/:namespace/tasks`, ({ params, request }) => {
    const url = new URL(request.url);
    const filtered = filterByNamespace(mockTasks, params.namespace as string);
    return HttpResponse.json(buildTaskListResponse(filtered, url));
  }),

  http.get(`${API_BASE}/namespaces/:namespace/tasks/:name`, ({ params, request }) => {
    const { namespace, name } = params;
    const url = new URL(request.url);
    const task = mockTasks.find((t) => t.namespace === namespace && t.name === name);
    if (!task) {
      return HttpResponse.json({ error: 'task not found' }, { status: 404 });
    }
    if (url.searchParams.get('output') === 'yaml') {
      return new HttpResponse(
        `apiVersion: kubeopencode.io/v1alpha1\nkind: Task\nmetadata:\n  name: ${name}\n  namespace: ${namespace}\nspec:\n  agentRef:\n    name: ${task.agentRef?.name || 'unknown'}\n  description: "${task.description || ''}"`,
        { headers: { 'Content-Type': 'text/plain' } },
      );
    }
    return HttpResponse.json(task);
  }),

  http.post(`${API_BASE}/namespaces/:namespace/tasks`, async ({ params, request }) => {
    const { namespace } = params;
    const body = await request.json() as Record<string, unknown>;
    const newTask: Task = {
      name: (body.name as string) || `task-${Date.now()}`,
      namespace: namespace as string,
      phase: 'Pending',
      description: body.description as string,
      agentRef: body.agentRef as { name: string },
      createdAt: new Date().toISOString(),
    };
    return HttpResponse.json(newTask, { status: 201 });
  }),

  http.delete(`${API_BASE}/namespaces/:namespace/tasks/:name`, ({ params }) => {
    const { namespace, name } = params;
    const task = mockTasks.find((t) => t.namespace === namespace && t.name === name);
    if (!task) {
      return HttpResponse.json({ error: 'task not found' }, { status: 404 });
    }
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(`${API_BASE}/namespaces/:namespace/tasks/:name/stop`, ({ params }) => {
    const { namespace, name } = params;
    const task = mockTasks.find((t) => t.namespace === namespace && t.name === name);
    if (!task) {
      return HttpResponse.json({ error: 'task not found' }, { status: 404 });
    }
    return HttpResponse.json({ ...task, phase: 'Completed' });
  }),

  // Task logs - SSE stream
  http.get(`${API_BASE}/namespaces/:namespace/tasks/:name/logs`, ({ params }) => {
    const { name } = params;
    const encoder = new TextEncoder();
    const stream = new ReadableStream({
      start(controller) {
        const lines = [
          `data: ${JSON.stringify({ type: 'status', phase: 'Running', podPhase: 'Running' })}\n\n`,
          `data: ${JSON.stringify({ type: 'log', content: `[${name}] Starting task execution...` })}\n\n`,
          `data: ${JSON.stringify({ type: 'log', content: `[${name}] Cloning repository...` })}\n\n`,
          `data: ${JSON.stringify({ type: 'log', content: `[${name}] Running opencode agent...` })}\n\n`,
          `data: ${JSON.stringify({ type: 'log', content: `[${name}] Analyzing codebase structure...` })}\n\n`,
          `data: ${JSON.stringify({ type: 'log', content: `[${name}] Generating solution...` })}\n\n`,
        ];
        let i = 0;
        const interval = setInterval(() => {
          if (i < lines.length) {
            controller.enqueue(encoder.encode(lines[i]));
            i++;
          } else {
            // Send periodic heartbeat logs
            controller.enqueue(
              encoder.encode(`data: ${JSON.stringify({ type: 'log', content: `[${name}] Working... (${i - lines.length + 1}s)` })}\n\n`),
            );
            i++;
          }
        }, 1000);
        // Clean up after 60s
        setTimeout(() => {
          clearInterval(interval);
          controller.enqueue(
            encoder.encode(`data: ${JSON.stringify({ type: 'complete', message: 'Task completed' })}\n\n`),
          );
          controller.close();
        }, 60000);
      },
    });
    return new HttpResponse(stream, {
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
    });
  }),

  // === Agents ===
  http.get(`${API_BASE}/agents`, ({ request }) => {
    const url = new URL(request.url);
    return HttpResponse.json(buildAgentListResponse(mockAgents, url));
  }),

  http.get(`${API_BASE}/namespaces/:namespace/agents`, ({ params, request }) => {
    const url = new URL(request.url);
    const filtered = filterByNamespace(mockAgents, params.namespace as string);
    return HttpResponse.json(buildAgentListResponse(filtered, url));
  }),

  http.get(`${API_BASE}/namespaces/:namespace/agents/:name`, ({ params, request }) => {
    const { namespace, name } = params;
    const url = new URL(request.url);
    const agent = mockAgents.find((a) => a.namespace === namespace && a.name === name);
    if (!agent) {
      return HttpResponse.json({ error: 'agent not found' }, { status: 404 });
    }
    if (url.searchParams.get('output') === 'yaml') {
      return new HttpResponse(
        `apiVersion: kubeopencode.io/v1alpha1\nkind: Agent\nmetadata:\n  name: ${name}\n  namespace: ${namespace}\nspec:\n  executorImage: ${agent.executorImage || ''}\n  agentImage: ${agent.agentImage || ''}\n  workspaceDir: ${agent.workspaceDir || '/workspace'}`,
        { headers: { 'Content-Type': 'text/plain' } },
      );
    }
    return HttpResponse.json(agent);
  }),

  http.post(`${API_BASE}/namespaces/:namespace/agents`, async ({ params, request }) => {
    const { namespace } = params;
    const body = await request.json() as Record<string, unknown>;
    const agentName = body.name as string;
    const newAgent: Agent = {
      name: agentName,
      namespace: namespace as string,
      profile: body.profile as string,
      workspaceDir: (body.workspaceDir as string) || '/workspace',
      contextsCount: 0,
      credentialsCount: 0,
      skillsCount: 0,
      createdAt: new Date().toISOString(),
      serverStatus: {
        deploymentName: `${agentName}-server`,
        serviceName: agentName,
        url: `http://${agentName}.${namespace}.svc.cluster.local:4096`,
        ready: false,
        port: 4096,
        suspended: false,
      },
    };
    return HttpResponse.json(newAgent, { status: 201 });
  }),

  http.post(`${API_BASE}/namespaces/:namespace/agents/:name/suspend`, ({ params }) => {
    const { namespace, name } = params;
    const agent = mockAgents.find((a) => a.namespace === namespace && a.name === name);
    if (!agent) {
      return HttpResponse.json({ error: 'agent not found' }, { status: 404 });
    }
    return HttpResponse.json({
      ...agent,
      serverStatus: { ...agent.serverStatus, suspended: true, ready: false },
    });
  }),

  http.post(`${API_BASE}/namespaces/:namespace/agents/:name/resume`, ({ params }) => {
    const { namespace, name } = params;
    const agent = mockAgents.find((a) => a.namespace === namespace && a.name === name);
    if (!agent) {
      return HttpResponse.json({ error: 'agent not found' }, { status: 404 });
    }
    return HttpResponse.json({
      ...agent,
      serverStatus: { ...agent.serverStatus, suspended: false, ready: true },
    });
  }),

  // === CronTasks ===
  http.get(`${API_BASE}/crontasks`, ({ request }) => {
    const url = new URL(request.url);
    return HttpResponse.json(buildCronTaskListResponse(mockCronTasks, url));
  }),

  http.get(`${API_BASE}/namespaces/:namespace/crontasks`, ({ params, request }) => {
    const url = new URL(request.url);
    const filtered = filterByNamespace(mockCronTasks, params.namespace as string);
    return HttpResponse.json(buildCronTaskListResponse(filtered, url));
  }),

  http.get(`${API_BASE}/namespaces/:namespace/crontasks/:name`, ({ params, request }) => {
    const { namespace, name } = params;
    const url = new URL(request.url);
    const cronTask = mockCronTasks.find((ct) => ct.namespace === namespace && ct.name === name);
    if (!cronTask) {
      return HttpResponse.json({ error: 'CronTask not found' }, { status: 404 });
    }
    if (url.searchParams.get('output') === 'yaml') {
      return new HttpResponse(
        `apiVersion: kubeopencode.io/v1alpha1\nkind: CronTask\nmetadata:\n  name: ${name}\n  namespace: ${namespace}\nspec:\n  schedule: "${cronTask.schedule}"\n  concurrencyPolicy: ${cronTask.concurrencyPolicy}\n  maxRetainedTasks: ${cronTask.maxRetainedTasks || 10}\n  taskTemplate:\n    spec:\n      agentRef:\n        name: ${cronTask.taskTemplate.agentRef?.name || 'unknown'}\n      description: "${cronTask.taskTemplate.description || ''}"`,
        { headers: { 'Content-Type': 'text/plain' } },
      );
    }
    return HttpResponse.json(cronTask);
  }),

  http.post(`${API_BASE}/namespaces/:namespace/crontasks`, async ({ params, request }) => {
    const { namespace } = params;
    const body = await request.json() as Record<string, unknown>;
    const newCronTask: CronTask = {
      name: (body.name as string) || `crontask-${Date.now()}`,
      namespace: namespace as string,
      schedule: body.schedule as string,
      timeZone: body.timeZone as string,
      concurrencyPolicy: (body.concurrencyPolicy as string) || 'Forbid',
      suspend: false,
      maxRetainedTasks: (body.maxRetainedTasks as number) || 10,
      active: 0,
      totalExecutions: 0,
      nextScheduleTime: new Date(Date.now() + 3600000).toISOString(),
      taskTemplate: {
        description: body.description as string,
        agentRef: body.agentRef as { name: string },
        templateRef: body.templateRef as { name: string },
      },
      createdAt: new Date().toISOString(),
      conditions: [
        { type: 'Ready', status: 'True', reason: 'Scheduled', message: 'Waiting for next schedule' },
      ],
    };
    return HttpResponse.json(newCronTask, { status: 201 });
  }),

  http.put(`${API_BASE}/namespaces/:namespace/crontasks/:name`, async ({ params, request }) => {
    const { namespace, name } = params;
    const cronTask = mockCronTasks.find((ct) => ct.namespace === namespace && ct.name === name);
    if (!cronTask) {
      return HttpResponse.json({ error: 'CronTask not found' }, { status: 404 });
    }
    const body = await request.json() as Record<string, unknown>;
    return HttpResponse.json({ ...cronTask, ...body });
  }),

  http.delete(`${API_BASE}/namespaces/:namespace/crontasks/:name`, ({ params }) => {
    const { namespace, name } = params;
    const cronTask = mockCronTasks.find((ct) => ct.namespace === namespace && ct.name === name);
    if (!cronTask) {
      return HttpResponse.json({ error: 'CronTask not found' }, { status: 404 });
    }
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(`${API_BASE}/namespaces/:namespace/crontasks/:name/suspend`, ({ params }) => {
    const { namespace, name } = params;
    const cronTask = mockCronTasks.find((ct) => ct.namespace === namespace && ct.name === name);
    if (!cronTask) {
      return HttpResponse.json({ error: 'CronTask not found' }, { status: 404 });
    }
    return HttpResponse.json({
      ...cronTask,
      suspend: true,
      nextScheduleTime: null,
      conditions: [{ type: 'Ready', status: 'False', reason: 'Suspended', message: 'CronTask is suspended' }],
    });
  }),

  http.post(`${API_BASE}/namespaces/:namespace/crontasks/:name/resume`, ({ params }) => {
    const { namespace, name } = params;
    const cronTask = mockCronTasks.find((ct) => ct.namespace === namespace && ct.name === name);
    if (!cronTask) {
      return HttpResponse.json({ error: 'CronTask not found' }, { status: 404 });
    }
    return HttpResponse.json({
      ...cronTask,
      suspend: false,
      nextScheduleTime: new Date(Date.now() + 3600000).toISOString(),
      conditions: [{ type: 'Ready', status: 'True', reason: 'Scheduled', message: 'Task created successfully' }],
    });
  }),

  http.post(`${API_BASE}/namespaces/:namespace/crontasks/:name/trigger`, ({ params }) => {
    const { name } = params;
    return HttpResponse.json({ message: `CronTask "${name}" triggered` });
  }),

  http.get(`${API_BASE}/namespaces/:namespace/crontasks/:name/history`, ({ params, request }) => {
    const { name } = params;
    const url = new URL(request.url);
    const historyTasks = mockCronTaskHistory.filter(
      (t) => t.labels?.['kubeopencode.io/crontask'] === name,
    );
    return HttpResponse.json(buildTaskListResponse(historyTasks.length > 0 ? historyTasks : mockCronTaskHistory, url));
  }),

  // === Agent Templates ===
  http.get(`${API_BASE}/agenttemplates`, ({ request }) => {
    const url = new URL(request.url);
    return HttpResponse.json(buildTemplateListResponse(mockAgentTemplates, url));
  }),

  http.get(`${API_BASE}/namespaces/:namespace/agenttemplates`, ({ params, request }) => {
    const url = new URL(request.url);
    const filtered = filterByNamespace(mockAgentTemplates, params.namespace as string);
    return HttpResponse.json(buildTemplateListResponse(filtered, url));
  }),

  http.post(`${API_BASE}/namespaces/:namespace/agenttemplates`, async ({ params, request }) => {
    const { namespace } = params;
    const body = await request.json() as Record<string, unknown>;
    const newTemplate: AgentTemplate = {
      name: (body.name as string) || `template-${Date.now()}`,
      namespace: namespace as string,
      workspaceDir: (body.workspaceDir as string) || '/workspace',
      serviceAccountName: body.serviceAccountName as string,
      agentImage: body.agentImage as string,
      executorImage: body.executorImage as string,
      agentCount: 0,
      contextsCount: 0,
      credentialsCount: 0,
      skillsCount: 0,
      createdAt: new Date().toISOString(),
    };
    return HttpResponse.json(newTemplate, { status: 201 });
  }),

  http.get(`${API_BASE}/namespaces/:namespace/agenttemplates/:name`, ({ params, request }) => {
    const { namespace, name } = params;
    const url = new URL(request.url);
    const template = mockAgentTemplates.find((t) => t.namespace === namespace && t.name === name);
    if (!template) {
      return HttpResponse.json({ error: 'agent template not found' }, { status: 404 });
    }
    if (url.searchParams.get('output') === 'yaml') {
      return new HttpResponse(
        `apiVersion: kubeopencode.io/v1alpha1\nkind: AgentTemplate\nmetadata:\n  name: ${name}\n  namespace: ${namespace}\nspec:\n  executorImage: ${template.executorImage || ''}\n  agentImage: ${template.agentImage || ''}\n  workspaceDir: ${template.workspaceDir || '/workspace'}`,
        { headers: { 'Content-Type': 'text/plain' } },
      );
    }
    return HttpResponse.json(template);
  }),
];
