// Agent utility functions

import type { Agent } from '../api/client';

/**
 * Check if a namespace matches a glob pattern.
 * Supports * (any string) and ? (single char) wildcards.
 */
export function matchGlob(pattern: string, namespace: string): boolean {
  const regexPattern = pattern
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*/g, '.*')
    .replace(/\?/g, '.');
  const regex = new RegExp(`^${regexPattern}$`);
  return regex.test(namespace);
}

/**
 * Check if an agent is available for a given namespace
 * based on its allowedNamespaces configuration.
 */
export function isAgentAvailableForNamespace(agent: Agent, namespace: string): boolean {
  if (!agent.allowedNamespaces || agent.allowedNamespaces.length === 0) {
    return true;
  }
  return agent.allowedNamespaces.some((pattern) => matchGlob(pattern, namespace));
}
