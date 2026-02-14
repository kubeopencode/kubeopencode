// MSW request handlers for API mocking

import { http, HttpResponse } from 'msw';
import {
  mockNamespaces,
  mockTaskListResponse,
  mockAgentListResponse,
  mockTemplateListResponse,
  mockTasks,
  mockAgents,
  mockTemplates,
} from './data';

const API_BASE = '/api/v1';

export const handlers = [
  // Server info
  http.get(`${API_BASE}/info`, () => {
    return HttpResponse.json({ version: '0.1.0' });
  }),

  // Namespaces
  http.get(`${API_BASE}/namespaces`, () => {
    return HttpResponse.json(mockNamespaces);
  }),

  // Tasks - list all
  http.get(`${API_BASE}/tasks`, () => {
    return HttpResponse.json(mockTaskListResponse);
  }),

  // Tasks - list by namespace
  http.get(`${API_BASE}/namespaces/:namespace/tasks`, ({ params }) => {
    const { namespace } = params;
    const filtered = mockTasks.filter((t) => t.namespace === namespace);
    return HttpResponse.json({
      tasks: filtered,
      total: filtered.length,
      pagination: {
        limit: 20,
        offset: 0,
        totalCount: filtered.length,
        hasMore: false,
      },
    });
  }),

  // Tasks - get single
  http.get(`${API_BASE}/namespaces/:namespace/tasks/:name`, ({ params, request }) => {
    const { namespace, name } = params;
    const url = new URL(request.url);
    const output = url.searchParams.get('output');

    const task = mockTasks.find((t) => t.namespace === namespace && t.name === name);
    if (!task) {
      return HttpResponse.json({ error: 'task not found' }, { status: 404 });
    }

    if (output === 'yaml') {
      return new HttpResponse(`apiVersion: kubeopencode.io/v1alpha1\nkind: Task\nmetadata:\n  name: ${name}\n  namespace: ${namespace}`, {
        headers: { 'Content-Type': 'text/plain' },
      });
    }

    return HttpResponse.json(task);
  }),

  // Tasks - create
  http.post(`${API_BASE}/namespaces/:namespace/tasks`, async ({ params, request }) => {
    const { namespace } = params;
    const body = await request.json() as Record<string, unknown>;
    const newTask = {
      name: (body.name as string) || `task-${Date.now()}`,
      namespace: namespace as string,
      phase: 'Pending',
      description: body.description,
      agentRef: body.agentRef,
      createdAt: new Date().toISOString(),
    };
    return HttpResponse.json(newTask, { status: 201 });
  }),

  // Tasks - delete
  http.delete(`${API_BASE}/namespaces/:namespace/tasks/:name`, ({ params }) => {
    const { namespace, name } = params;
    const task = mockTasks.find((t) => t.namespace === namespace && t.name === name);
    if (!task) {
      return HttpResponse.json({ error: 'task not found' }, { status: 404 });
    }
    return new HttpResponse(null, { status: 204 });
  }),

  // Tasks - stop
  http.post(`${API_BASE}/namespaces/:namespace/tasks/:name/stop`, ({ params }) => {
    const { namespace, name } = params;
    const task = mockTasks.find((t) => t.namespace === namespace && t.name === name);
    if (!task) {
      return HttpResponse.json({ error: 'task not found' }, { status: 404 });
    }
    return HttpResponse.json({ ...task, phase: 'Completed' });
  }),

  // Agents - list all
  http.get(`${API_BASE}/agents`, () => {
    return HttpResponse.json(mockAgentListResponse);
  }),

  // Agents - list by namespace
  http.get(`${API_BASE}/namespaces/:namespace/agents`, ({ params }) => {
    const { namespace } = params;
    const filtered = mockAgents.filter((a) => a.namespace === namespace);
    return HttpResponse.json({
      agents: filtered,
      total: filtered.length,
      pagination: {
        limit: 20,
        offset: 0,
        totalCount: filtered.length,
        hasMore: false,
      },
    });
  }),

  // Agents - get single
  http.get(`${API_BASE}/namespaces/:namespace/agents/:name`, ({ params, request }) => {
    const { namespace, name } = params;
    const url = new URL(request.url);
    const output = url.searchParams.get('output');

    const agent = mockAgents.find((a) => a.namespace === namespace && a.name === name);
    if (!agent) {
      return HttpResponse.json({ error: 'agent not found' }, { status: 404 });
    }

    if (output === 'yaml') {
      return new HttpResponse(`apiVersion: kubeopencode.io/v1alpha1\nkind: Agent\nmetadata:\n  name: ${name}\n  namespace: ${namespace}`, {
        headers: { 'Content-Type': 'text/plain' },
      });
    }

    return HttpResponse.json(agent);
  }),

  // TaskTemplates - list all
  http.get(`${API_BASE}/tasktemplates`, () => {
    return HttpResponse.json(mockTemplateListResponse);
  }),

  // TaskTemplates - list by namespace
  http.get(`${API_BASE}/namespaces/:namespace/tasktemplates`, ({ params }) => {
    const { namespace } = params;
    const filtered = mockTemplates.filter((t) => t.namespace === namespace);
    return HttpResponse.json({
      templates: filtered,
      total: filtered.length,
      pagination: {
        limit: 20,
        offset: 0,
        totalCount: filtered.length,
        hasMore: false,
      },
    });
  }),

  // TaskTemplates - get single
  http.get(`${API_BASE}/namespaces/:namespace/tasktemplates/:name`, ({ params, request }) => {
    const { namespace, name } = params;
    const url = new URL(request.url);
    const output = url.searchParams.get('output');

    const template = mockTemplates.find((t) => t.namespace === namespace && t.name === name);
    if (!template) {
      return HttpResponse.json({ error: 'template not found' }, { status: 404 });
    }

    if (output === 'yaml') {
      return new HttpResponse(`apiVersion: kubeopencode.io/v1alpha1\nkind: TaskTemplate\nmetadata:\n  name: ${name}\n  namespace: ${namespace}`, {
        headers: { 'Content-Type': 'text/plain' },
      });
    }

    return HttpResponse.json(template);
  }),
];
