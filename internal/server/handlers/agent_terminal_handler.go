// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	authmiddleware "github.com/kubeopencode/kubeopencode/internal/server/middleware"
)

// terminalIdleTimeout is the duration after which an idle terminal session is closed.
// The timeout resets on each WebSocket read (user input).
const terminalIdleTimeout = 30 * time.Minute

var termLog = ctrl.Log.WithName("terminal")

// AgentTerminalHandler handles WebSocket terminal sessions to agent server pods.
type AgentTerminalHandler struct {
	defaultClient    client.Client
	defaultClientset kubernetes.Interface
	restConfig       *rest.Config
}

// NewAgentTerminalHandler creates a new AgentTerminalHandler.
func NewAgentTerminalHandler(c client.Client, clientset kubernetes.Interface, restConfig *rest.Config) *AgentTerminalHandler {
	return &AgentTerminalHandler{
		defaultClient:    c,
		defaultClientset: clientset,
		restConfig:       restConfig,
	}
}

// checkSameOrigin returns true if the Origin header matches the Host header (same-origin).
// Non-browser clients (no Origin header) are allowed.
func checkSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // Non-browser clients don't send Origin
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == r.Host
}

var upgrader = websocket.Upgrader{
	CheckOrigin: checkSameOrigin,
}

// resizeMessage is a terminal resize control message from the browser.
type resizeMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// terminalSizeQueue implements remotecommand.TerminalSizeQueue.
type terminalSizeQueue struct {
	ch chan *remotecommand.TerminalSize
}

func (q *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-q.ch
	if !ok {
		return nil
	}
	return size
}

// ServeTerminal upgrades the HTTP connection to WebSocket and bridges it to
// a pod exec session running "opencode attach" in the agent's server pod.
func (h *AgentTerminalHandler) ServeTerminal(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	agentName := chi.URLParam(r, "name")

	k8sClient := clientFromContext(r.Context(), h.defaultClient)

	// Resolve the agent's server pod
	podName, containerName, port, err := resolveAgentServerPod(r.Context(), k8sClient, namespace, agentName)
	if err != nil {
		termLog.Error(err, "failed to resolve agent server pod", "agent", agentName, "namespace", namespace)
		writeError(w, http.StatusBadRequest, "Cannot resolve agent server pod", err.Error())
		return
	}

	// Build impersonated rest.Config for exec RBAC enforcement.
	// This ensures the Kubernetes API server checks the user's pods/exec permission,
	// not the controller's service account.
	execConfig := rest.CopyConfig(h.restConfig)
	userInfo := authmiddleware.GetUserInfo(r.Context())
	if userInfo != nil {
		execConfig.Impersonate = rest.ImpersonationConfig{
			UserName: userInfo.Username,
			UID:      userInfo.UID,
			Groups:   userInfo.Groups,
		}
		termLog.Info("terminal session starting", "user", userInfo.Username, "agent", agentName, "namespace", namespace, "pod", podName)
	}

	// Upgrade to WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		termLog.Error(err, "websocket upgrade failed")
		return
	}
	defer ws.Close()

	// Mutex to serialize all WebSocket writes (gorilla/websocket requires this)
	var wsMu sync.Mutex

	// Detach from chi's 60s timeout for long-lived connection.
	// Use a separate cancellable context so we can stop the exec stream and heartbeat
	// when the WebSocket disconnects.
	sessionCtx, sessionCancel := context.WithCancel(context.WithoutCancel(r.Context()))
	defer sessionCancel()

	// Start connection heartbeat to prevent standby auto-suspend while terminal is active.
	// Uses the server's service account (defaultClient), not the impersonated user client.
	go controller.RunConnectionHeartbeat(sessionCtx, h.defaultClient, namespace, agentName, func(err error) {
		termLog.Error(err, "heartbeat: failed to patch annotation", "agent", agentName)
	})

	// Build the exec request using impersonated config
	execClientset, err := kubernetes.NewForConfig(execConfig)
	if err != nil {
		termLog.Error(err, "failed to create impersonated clientset")
		wsMu.Lock()
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "auth failed"))
		wsMu.Unlock()
		return
	}

	attachURL := fmt.Sprintf("http://localhost:%d", port)
	execReq := execClientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"/tools/opencode", "attach", attachURL},
			Stdin:     true,
			Stdout:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(execConfig, "POST", execReq.URL())
	if err != nil {
		termLog.Error(err, "failed to create SPDY executor")
		wsMu.Lock()
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "exec failed"))
		wsMu.Unlock()
		return
	}

	// Set up terminal size queue
	sizeQueue := &terminalSizeQueue{ch: make(chan *remotecommand.TerminalSize, 1)}

	// Create pipe for stdin: WebSocket reader writes to pw, exec reads from pr
	pr, pw := io.Pipe()

	// Read from WebSocket, write to stdin pipe + send resize events.
	// Set an idle timeout that resets on each message from the browser.
	_ = ws.SetReadDeadline(time.Now().Add(terminalIdleTimeout))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer pw.Close()
		defer close(sizeQueue.ch)
		defer sessionCancel() // Cancel session context when WebSocket disconnects

		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			// Reset idle timeout on every message from the client
			_ = ws.SetReadDeadline(time.Now().Add(terminalIdleTimeout))

			if msgType == websocket.TextMessage {
				// Control message (resize)
				var msg resizeMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					continue
				}
				if msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
					select {
					case sizeQueue.ch <- &remotecommand.TerminalSize{
						Width:  msg.Cols,
						Height: msg.Rows,
					}:
					default:
					}
				}
			} else {
				// Binary message: terminal input
				if _, err := pw.Write(data); err != nil {
					return
				}
			}
		}
	}()

	// stdout writer that forwards to WebSocket with mutex protection
	wsWriter := &wsStdoutWriter{ws: ws, mu: &wsMu}

	// Execute the command - this blocks until the exec session ends
	err = exec.StreamWithContext(sessionCtx, remotecommand.StreamOptions{
		Stdin:             pr,
		Stdout:            wsWriter,
		Tty:               true,
		TerminalSizeQueue: sizeQueue,
	})

	if err != nil {
		termLog.Info("exec session ended", "error", err, "agent", agentName)
		// Send error message to the terminal before closing
		errMsg := fmt.Sprintf("\r\n\x1b[31mError: %s\x1b[0m\r\n", err.Error())
		wsMu.Lock()
		_ = ws.WriteMessage(websocket.BinaryMessage, []byte(errMsg))
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "exec failed"))
		wsMu.Unlock()
	} else {
		// Normal close
		wsMu.Lock()
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		wsMu.Unlock()
	}

	wg.Wait()
}

// wsStdoutWriter writes exec stdout to a WebSocket connection with mutex protection.
type wsStdoutWriter struct {
	ws *websocket.Conn
	mu *sync.Mutex
}

func (w *wsStdoutWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// resolveAgentServerPod finds the running server pod for a Server-mode Agent.
func resolveAgentServerPod(ctx context.Context, k8sClient client.Client, namespace, agentName string) (podName string, containerName string, port int32, err error) {
	var agent kubeopenv1alpha1.Agent
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: agentName}, &agent); err != nil {
		return "", "", 0, fmt.Errorf("agent not found: %w", err)
	}

	if agent.Status.Suspended {
		return "", "", 0, fmt.Errorf("agent %q is suspended", agentName)
	}
	if !agent.Status.Ready {
		return "", "", 0, fmt.Errorf("agent %q is not ready (deployment may be starting up)", agentName)
	}

	// Find the server pod using the same labels as server_builder.go
	podList := &corev1.PodList{}
	if err := k8sClient.List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/name":      "kubeopencode-server",
			"app.kubernetes.io/instance":  agentName,
			"app.kubernetes.io/component": "server",
		},
	); err != nil {
		return "", "", 0, fmt.Errorf("failed to list server pods: %w", err)
	}

	// Find a Ready pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				serverPort := controller.GetServerPort(&agent)
				return pod.Name, controller.ServerContainerName, serverPort, nil
			}
		}
	}

	return "", "", 0, fmt.Errorf("no ready server pod found for agent %q", agentName)
}
