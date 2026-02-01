// Cookie utility functions for persisting user preferences

const NAMESPACE_COOKIE_KEY = 'kubeopencode_namespace';
const COOKIE_MAX_AGE = 30 * 24 * 60 * 60; // 30 days

/**
 * Get the namespace value from cookie
 * @returns The stored namespace or null if not set
 */
export function getNamespaceCookie(): string | null {
  const match = document.cookie.match(new RegExp(`${NAMESPACE_COOKIE_KEY}=([^;]+)`));
  return match ? decodeURIComponent(match[1]) : null;
}

/**
 * Save the namespace value to cookie
 * @param namespace The namespace to save
 */
export function setNamespaceCookie(namespace: string): void {
  document.cookie = `${NAMESPACE_COOKIE_KEY}=${encodeURIComponent(namespace)};path=/;max-age=${COOKIE_MAX_AGE};SameSite=Lax`;
}
