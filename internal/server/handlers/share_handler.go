// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/controller"
	servertypes "github.com/kubeopencode/kubeopencode/internal/server/types"
)

var shareLog = ctrl.Log.WithName("share")

// shareValidationInterval is how often active share sessions re-check
// whether the share link is still enabled. This ensures that disabling
// a share link disconnects existing sessions promptly.
const shareValidationInterval = 15 * time.Second

// shareUpgrader allows cross-origin WebSocket connections since the token itself is the credential.
var shareUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

// ShareHandler handles share link routes.
// These routes are outside the auth middleware and use token-based access.
type ShareHandler struct {
	k8sClient  client.Client
	clientset  kubernetes.Interface
	restConfig *rest.Config
}

// NewShareHandler creates a new ShareHandler.
func NewShareHandler(k8sClient client.Client, clientset kubernetes.Interface, restConfig *rest.Config) *ShareHandler {
	return &ShareHandler{
		k8sClient:  k8sClient,
		clientset:  clientset,
		restConfig: restConfig,
	}
}

// shareContext holds the resolved agent and share configuration for a validated token.
type shareContext struct {
	agent *kubeopenv1alpha1.Agent
}

// resolveShareToken validates a share token and returns the associated agent.
// Returns nil if the token is invalid, expired, or the agent is not ready.
func (h *ShareHandler) resolveShareToken(ctx context.Context, token string) (*shareContext, error) {
	// List all share Secrets
	var secretList corev1.SecretList
	if err := h.k8sClient.List(ctx, &secretList,
		client.MatchingLabels{controller.LabelShareToken: "true"},
	); err != nil {
		return nil, fmt.Errorf("failed to list share secrets: %w", err)
	}

	// Find matching token
	for i := range secretList.Items {
		secret := &secretList.Items[i]
		storedToken, ok := secret.Data[controller.ShareTokenKey]
		if !ok {
			continue
		}
		if subtle.ConstantTimeCompare(storedToken, []byte(token)) != 1 {
			continue
		}

		// Token matches — resolve the agent
		agentName := secret.Annotations[controller.AnnotationShareAgentName]
		agentNamespace := secret.Annotations[controller.AnnotationShareAgentNamespace]
		if agentName == "" || agentNamespace == "" {
			return nil, fmt.Errorf("share secret %q missing agent annotations", secret.Name)
		}

		var agent kubeopenv1alpha1.Agent
		if err := h.k8sClient.Get(ctx, client.ObjectKey{
			Namespace: agentNamespace,
			Name:      agentName,
		}, &agent); err != nil {
			return nil, fmt.Errorf("agent %s/%s not found: %w", agentNamespace, agentName, err)
		}

		// Validate share is enabled and active
		if agent.Spec.Share == nil || !agent.Spec.Share.Enabled {
			return nil, fmt.Errorf("share link is disabled for agent %q", agentName)
		}

		// Check expiry
		if agent.Spec.Share.ExpiresAt != nil && time.Now().After(agent.Spec.Share.ExpiresAt.Time) {
			return nil, fmt.Errorf("share link for agent %q has expired", agentName)
		}

		// Check agent readiness
		if !agent.Status.Ready {
			return nil, fmt.Errorf("agent %q is not ready", agentName)
		}
		if agent.Status.Suspended {
			return nil, fmt.Errorf("agent %q is suspended", agentName)
		}

		return &shareContext{
			agent: &agent,
		}, nil
	}

	return nil, fmt.Errorf("invalid share token")
}

// validateShareIP checks if the client IP is allowed by the agent's share IP allowlist.
func validateShareIP(r *http.Request, allowedIPs []string) error {
	if len(allowedIPs) == 0 {
		return nil // No restriction
	}

	clientIP := getClientIP(r)
	if clientIP == "" {
		return fmt.Errorf("cannot determine client IP")
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return fmt.Errorf("invalid client IP: %s", clientIP)
	}

	for _, cidr := range allowedIPs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			// Try as single IP
			if allowed := net.ParseIP(cidr); allowed != nil && allowed.Equal(ip) {
				return nil
			}
			continue
		}
		if network.Contains(ip) {
			return nil
		}
	}

	return fmt.Errorf("client IP %s is not in the allowed IP list", clientIP)
}

// getClientIP extracts the client IP from the request.
// Uses r.RemoteAddr which is already set by chi's RealIP middleware
// (which reads X-Forwarded-For / X-Real-Ip and sets RemoteAddr).
// This avoids re-reading raw headers which could be spoofed in some deployments.
func getClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ServeShareInfo returns agent information for a share token.
// GET /s/{token}/info
func (h *ShareHandler) ServeShareInfo(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	sc, err := h.resolveShareToken(r.Context(), token)
	if err != nil {
		shareLog.Info("share info: invalid token", "error", err)
		writeError(w, http.StatusNotFound, "Not found", "Invalid or expired share link")
		return
	}

	// Validate IP
	if err := validateShareIP(r, sc.agent.Spec.Share.AllowedIPs); err != nil {
		shareLog.Info("share info: IP denied", "error", err, "agent", sc.agent.Name)
		writeError(w, http.StatusForbidden, "Forbidden", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, servertypes.ShareInfoResponse{
		AgentName: sc.agent.Name,
		Namespace: sc.agent.Namespace,
		Profile:   sc.agent.Spec.Profile,
	})
}

// ServeShareTerminal handles WebSocket terminal sessions via share token.
// GET /s/{token}/terminal
func (h *ShareHandler) ServeShareTerminal(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	sc, err := h.resolveShareToken(r.Context(), token)
	if err != nil {
		shareLog.Info("share terminal: invalid token", "error", err)
		http.Error(w, "Invalid or expired share link", http.StatusNotFound)
		return
	}

	// Validate IP
	if err := validateShareIP(r, sc.agent.Spec.Share.AllowedIPs); err != nil {
		shareLog.Info("share terminal: IP denied", "error", err, "agent", sc.agent.Name)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	agent := sc.agent

	// Resolve the agent's server pod (using server's own client, no impersonation)
	podName, containerName, port, err := resolveAgentServerPod(r.Context(), h.k8sClient, agent.Namespace, agent.Name)
	if err != nil {
		shareLog.Error(err, "share terminal: failed to resolve agent server pod", "agent", agent.Name)
		http.Error(w, "Agent server not available", http.StatusServiceUnavailable)
		return
	}

	shareLog.Info("share terminal session starting",
		"agent", agent.Name, "namespace", agent.Namespace,
		"pod", podName, "clientIP", getClientIP(r))

	// Upgrade to WebSocket
	ws, err := shareUpgrader.Upgrade(w, r, nil)
	if err != nil {
		shareLog.Error(err, "share terminal: websocket upgrade failed")
		return
	}
	defer func() { _ = ws.Close() }()

	var wsMu sync.Mutex

	// Detach from chi's timeout for long-lived connection
	sessionCtx, sessionCancel := context.WithCancel(context.WithoutCancel(r.Context()))
	defer sessionCancel()

	// Start connection heartbeat (using server's own client)
	go controller.RunConnectionHeartbeat(sessionCtx, h.k8sClient, agent.Namespace, agent.Name, func(err error) {
		shareLog.Error(err, "share heartbeat: failed to patch annotation", "agent", agent.Name)
	})

	// Periodically re-validate share status so that disabling a share link
	// disconnects existing sessions promptly instead of waiting for idle timeout.
	go func() {
		ticker := time.NewTicker(shareValidationInterval)
		defer ticker.Stop()
		for {
			select {
			case <-sessionCtx.Done():
				return
			case <-ticker.C:
				if _, err := h.resolveShareToken(sessionCtx, token); err != nil {
					shareLog.Info("share terminal: share revoked, disconnecting session",
						"agent", agent.Name, "reason", err.Error())
					wsMu.Lock()
					revokedMsg := "\r\n\x1b[31mShare link has been disabled. Disconnecting...\x1b[0m\r\n"
					_ = ws.WriteMessage(websocket.BinaryMessage, []byte(revokedMsg))
					_ = ws.WriteMessage(websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "share link revoked"))
					wsMu.Unlock()
					sessionCancel()
					return
				}
			}
		}
	}()

	// Build exec request using server's own ServiceAccount (not impersonated)
	attachURL := fmt.Sprintf("http://localhost:%d", port)

	// WebSocket reader goroutine
	inputCh := make(chan []byte, 16)
	resizeCh := make(chan *remotecommand.TerminalSize, 1)

	go func() {
		defer sessionCancel()
		defer close(inputCh)
		defer close(resizeCh)
		_ = ws.SetReadDeadline(time.Now().Add(terminalIdleTimeout))
		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			_ = ws.SetReadDeadline(time.Now().Add(terminalIdleTimeout))

			if msgType == websocket.TextMessage {
				var msg resizeMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					continue
				}
				if msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
					select {
					case resizeCh <- &remotecommand.TerminalSize{
						Width:  msg.Cols,
						Height: msg.Rows,
					}:
					default:
					}
				}
			} else {
				select {
				case inputCh <- append([]byte(nil), data...):
				case <-sessionCtx.Done():
					return
				}
			}
		}
	}()

	wsWriter := &wsStdoutWriter{ws: ws, mu: &wsMu}

	// Retry loop (same as existing terminal handler)
	var lastErr error
	for attempt := 1; attempt <= maxExecRetries; attempt++ {
		if sessionCtx.Err() != nil {
			break
		}

		execReq := h.clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace(agent.Namespace).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: containerName,
				Command:   []string{"/tools/opencode", "attach", attachURL},
				Stdin:     true,
				Stdout:    true,
				TTY:       true,
			}, scheme.ParameterCodec)

		executor, err := remotecommand.NewSPDYExecutor(h.restConfig, "POST", execReq.URL())
		if err != nil {
			shareLog.Error(err, "share terminal: failed to create SPDY executor")
			lastErr = err
			break
		}

		pr, pw := io.Pipe()
		sizeQueue := &terminalSizeQueue{ch: make(chan *remotecommand.TerminalSize, 1)}
		attemptCtx, attemptCancel := context.WithCancel(sessionCtx)

		var pumpWg sync.WaitGroup
		pumpWg.Add(1)
		go func() {
			defer pumpWg.Done()
			defer func() { _ = pw.Close() }()
			defer close(sizeQueue.ch)
			for {
				select {
				case data, ok := <-inputCh:
					if !ok {
						return
					}
					if _, err := pw.Write(data); err != nil {
						return
					}
				case size, ok := <-resizeCh:
					if !ok {
						return
					}
					select {
					case sizeQueue.ch <- size:
					default:
					}
				case <-attemptCtx.Done():
					return
				}
			}
		}()

		streamOpts := remotecommand.StreamOptions{
			Stdin:             pr,
			Stdout:            wsWriter,
			Tty:               true,
			TerminalSizeQueue: sizeQueue,
		}

		lastErr = executor.StreamWithContext(attemptCtx, streamOpts)

		attemptCancel()
		_ = pr.Close()
		pumpWg.Wait()

		if lastErr == nil || !isTransientExecError(lastErr) || attempt == maxExecRetries {
			break
		}

		shareLog.Info("share terminal: transient exec failure, retrying",
			"attempt", attempt, "error", lastErr, "agent", agent.Name)
		wsMu.Lock()
		retryMsg := fmt.Sprintf("\r\n\x1b[33mConnection interrupted, retrying (%d/%d)...\x1b[0m\r\n",
			attempt, maxExecRetries)
		_ = ws.WriteMessage(websocket.BinaryMessage, []byte(retryMsg))
		wsMu.Unlock()

		select {
		case <-time.After(execRetryDelay):
		case <-sessionCtx.Done():
		}
	}

	if lastErr != nil {
		shareLog.Info("share terminal: exec session ended", "error", lastErr, "agent", agent.Name)
		errMsg := fmt.Sprintf("\r\n\x1b[31mError: %s\x1b[0m\r\n", lastErr.Error())
		wsMu.Lock()
		_ = ws.WriteMessage(websocket.BinaryMessage, []byte(errMsg))
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "exec failed"))
		wsMu.Unlock()
	} else {
		wsMu.Lock()
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		wsMu.Unlock()
	}
}
