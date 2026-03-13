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
