import { describe, it, expect } from 'vitest';
import { matchGlob, isAgentAvailableForNamespace } from '../agent';
import type { Agent } from '../../api/client';

describe('agent utilities', () => {
  describe('matchGlob', () => {
    it('matches exact strings', () => {
      expect(matchGlob('default', 'default')).toBe(true);
      expect(matchGlob('default', 'production')).toBe(false);
    });

    it('matches * wildcard (any string)', () => {
      expect(matchGlob('dev-*', 'dev-team-a')).toBe(true);
      expect(matchGlob('dev-*', 'dev-')).toBe(true);
      expect(matchGlob('dev-*', 'staging')).toBe(false);
    });

    it('matches ? wildcard (single char)', () => {
      expect(matchGlob('ns-?', 'ns-a')).toBe(true);
      expect(matchGlob('ns-?', 'ns-ab')).toBe(false);
    });

    it('matches * for any namespace', () => {
      expect(matchGlob('*', 'anything')).toBe(true);
      expect(matchGlob('*', '')).toBe(true);
    });

    it('escapes special regex characters', () => {
      expect(matchGlob('ns.prod', 'ns.prod')).toBe(true);
      expect(matchGlob('ns.prod', 'ns-prod')).toBe(false);
    });

    it('matches complex patterns', () => {
      expect(matchGlob('team-*-dev', 'team-backend-dev')).toBe(true);
      expect(matchGlob('team-*-dev', 'team-backend-prod')).toBe(false);
    });
  });

  describe('isAgentAvailableForNamespace', () => {
    const makeAgent = (allowedNamespaces?: string[]): Agent => ({
      name: 'test-agent',
      namespace: 'default',
      contextsCount: 0,
      credentialsCount: 0,
      createdAt: '2026-01-01T00:00:00Z',
      mode: 'Pod',
      allowedNamespaces,
    });

    it('returns true when no allowedNamespaces is set', () => {
      expect(isAgentAvailableForNamespace(makeAgent(), 'anything')).toBe(true);
    });

    it('returns true when allowedNamespaces is empty', () => {
      expect(isAgentAvailableForNamespace(makeAgent([]), 'anything')).toBe(true);
    });

    it('returns true when namespace matches an allowed pattern', () => {
      const agent = makeAgent(['default', 'staging']);
      expect(isAgentAvailableForNamespace(agent, 'default')).toBe(true);
      expect(isAgentAvailableForNamespace(agent, 'staging')).toBe(true);
    });

    it('returns false when namespace does not match any pattern', () => {
      const agent = makeAgent(['production']);
      expect(isAgentAvailableForNamespace(agent, 'default')).toBe(false);
    });

    it('supports glob patterns in allowedNamespaces', () => {
      const agent = makeAgent(['dev-*', 'staging']);
      expect(isAgentAvailableForNamespace(agent, 'dev-team-a')).toBe(true);
      expect(isAgentAvailableForNamespace(agent, 'staging')).toBe(true);
      expect(isAgentAvailableForNamespace(agent, 'production')).toBe(false);
    });
  });
});
