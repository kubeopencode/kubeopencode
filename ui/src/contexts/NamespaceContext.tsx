import React, { createContext, useContext, useState, useCallback } from 'react';
import { getNamespaceCookie, setNamespaceCookie } from '../utils/cookies';

const ALL_NAMESPACES = '__all__';

interface NamespaceContextValue {
  namespace: string;
  setNamespace: (ns: string) => void;
  isAllNamespaces: boolean;
  ALL_NAMESPACES: string;
}

const NamespaceContext = createContext<NamespaceContextValue | null>(null);

export function NamespaceProvider({ children }: { children: React.ReactNode }) {
  const [namespace, setNamespaceState] = useState(() => {
    return getNamespaceCookie() || ALL_NAMESPACES;
  });

  const setNamespace = useCallback((ns: string) => {
    setNamespaceState(ns);
    setNamespaceCookie(ns);
  }, []);

  return (
    <NamespaceContext.Provider
      value={{
        namespace,
        setNamespace,
        isAllNamespaces: namespace === ALL_NAMESPACES,
        ALL_NAMESPACES,
      }}
    >
      {children}
    </NamespaceContext.Provider>
  );
}

export function useNamespace() {
  const ctx = useContext(NamespaceContext);
  if (!ctx) {
    throw new Error('useNamespace must be used within a NamespaceProvider');
  }
  return ctx;
}
