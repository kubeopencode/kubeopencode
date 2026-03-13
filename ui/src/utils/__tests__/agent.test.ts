import { describe, it, expect } from 'vitest';
import { matchGlob } from '../agent';

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
});
