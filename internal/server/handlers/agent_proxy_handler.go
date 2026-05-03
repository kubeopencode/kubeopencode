// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-chi/chi/v5"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var proxyLog = ctrl.Log.WithName("agent-proxy")

// AgentProxyHandler handles reverse-proxying requests to OpenCode agent servers.
// It resolves an Agent's in-cluster server URL and uses httputil.ReverseProxy
// to forward all requests, supporting both HTTP REST and SSE streaming.
type AgentProxyHandler struct {
	defaultClient client.Client
	clusterDomain string
}

// NewAgentProxyHandler creates a new AgentProxyHandler
func NewAgentProxyHandler(c client.Client, clusterDomain string) *AgentProxyHandler {
	return &AgentProxyHandler{defaultClient: c, clusterDomain: clusterDomain}
}

// ServeProxy is the catch-all handler for /api/v1/namespaces/{namespace}/agents/{name}/proxy/*
// It resolves the Agent's server URL, rewrites the request path, and proxies via httputil.ReverseProxy.
func (h *AgentProxyHandler) ServeProxy(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	agentName := chi.URLParam(r, "name")

	// Detach from chi's timeout context (60s) to support long-lived SSE streams.
	// context.WithoutCancel preserves values but does not inherit cancellation.
	// The proxy will still terminate when the client disconnects (write errors).
	ctx := context.WithoutCancel(r.Context())

	k8sClient := clientFromContext(ctx, h.defaultClient)

	serverURL, err := resolveAgentServerURL(ctx, k8sClient, namespace, agentName, h.clusterDomain)
	if err != nil {
		proxyLog.Error(err, "Failed to resolve agent server URL", "namespace", namespace, "agent", agentName)
		writeError(w, http.StatusBadGateway, "Cannot resolve agent server", err.Error())
		return
	}

	target, err := url.Parse(serverURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Invalid server URL", err.Error())
		return
	}

	proxyPath := normalizeProxyPath(r)

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = proxyPath
			req.Host = target.Host
			// Remove Authorization header — internal traffic does not need external auth
			req.Header.Del("Authorization")
		},
		// FlushInterval -1 means flush immediately after each write,
		// which is critical for SSE streaming.
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			proxyLog.Error(err, "Proxy error", "namespace", namespace, "agent", agentName, "path", proxyPath)
			writeError(w, http.StatusBadGateway, "Proxy error", err.Error())
		},
	}

	proxyLog.V(1).Info("Proxying request", "namespace", namespace, "agent", agentName, "path", proxyPath, "method", r.Method)
	proxy.ServeHTTP(w, r.WithContext(ctx))
}
