import { describe, it, expect, beforeEach } from 'vitest';
import { getNamespaceCookie, setNamespaceCookie } from '../cookies';

describe('cookies', () => {
  beforeEach(() => {
    // Clear all cookies
    document.cookie.split(';').forEach((c) => {
      document.cookie = c.trim().split('=')[0] + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
    });
  });

  describe('getNamespaceCookie', () => {
    it('returns null when no cookie is set', () => {
      expect(getNamespaceCookie()).toBeNull();
    });

    it('returns the stored namespace value', () => {
      document.cookie = 'kubeopencode_namespace=production;path=/';
      expect(getNamespaceCookie()).toBe('production');
    });

    it('decodes URI-encoded values', () => {
      document.cookie = 'kubeopencode_namespace=my%20namespace;path=/';
      expect(getNamespaceCookie()).toBe('my namespace');
    });

    it('returns correct value when multiple cookies exist', () => {
      document.cookie = 'other_cookie=value;path=/';
      document.cookie = 'kubeopencode_namespace=staging;path=/';
      document.cookie = 'another_cookie=value2;path=/';
      expect(getNamespaceCookie()).toBe('staging');
    });
  });

  describe('setNamespaceCookie', () => {
    it('sets the namespace cookie', () => {
      setNamespaceCookie('default');
      expect(getNamespaceCookie()).toBe('default');
    });

    it('overwrites existing namespace cookie', () => {
      setNamespaceCookie('default');
      setNamespaceCookie('production');
      expect(getNamespaceCookie()).toBe('production');
    });

    it('handles special characters via encoding', () => {
      setNamespaceCookie('ns/with-special');
      expect(getNamespaceCookie()).toBe('ns/with-special');
    });
  });
});
