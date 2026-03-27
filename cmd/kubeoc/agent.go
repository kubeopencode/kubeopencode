// Copyright Contributors to the KubeOpenCode project

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/controller"
)

const (
	// Default kubeopencode server deployment settings
	defaultServerNamespace = "kubeopencode-system"
	defaultServerService   = "kubeopencode-server"
	defaultServerPort      = 2746
)

func init() {
	rootCmd.AddCommand(newAgentCmd())
}

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Interact with KubeOpenCode agents",
	}
	cmd.AddCommand(newAgentAttachCmd())
	return cmd
}

func newAgentAttachCmd() *cobra.Command {
	var (
		namespace       string
		localPort       int
		serverNamespace string
		serverService   string
		serverPort      int
		usePortForward  bool
	)

	cmd := &cobra.Command{
		Use:   "attach <agent-name>",
		Short: "Attach to a server-mode agent via OpenCode TUI",
		Long: `Attach to a server-mode agent with a single command.

By default, connects through the kube-apiserver's built-in service proxy,
using your kubeconfig credentials for authentication. No port-forward needed.

Use --use-port-forward to fall back to kubectl port-forward (legacy mode).

Kubeconfig resolution (in priority order):
  1. KUBEOPENCODE_KUBECONFIG environment variable
  2. KUBECONFIG environment variable
  3. Default ~/.kube/config

Examples:
  kubeoc agent attach server-agent -n test
  kubeoc agent attach my-agent -n production --local-port 5000
  kubeoc agent attach my-agent -n test --use-port-forward`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if usePortForward {
				return runAgentAttachPortForward(cmd.Context(), namespace, args[0], localPort)
			}
			return runAgentAttachServiceProxy(cmd.Context(), namespace, args[0], localPort, serverNamespace, serverService, serverPort)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Agent namespace")
	cmd.Flags().IntVar(&localPort, "local-port", 0, "Local port (default: same as agent server port)")
	cmd.Flags().StringVar(&serverNamespace, "server-namespace", defaultServerNamespace,
		"Namespace where kubeopencode server is deployed")
	cmd.Flags().StringVar(&serverService, "server-service", defaultServerService,
		"Name of kubeopencode server Service")
	cmd.Flags().IntVar(&serverPort, "server-port", defaultServerPort,
		"Port of kubeopencode server Service")
	cmd.Flags().BoolVar(&usePortForward, "use-port-forward", false,
		"Use kubectl port-forward instead of service proxy (legacy mode)")

	return cmd
}

// runAgentAttachServiceProxy connects to an agent via kube-apiserver's built-in service proxy.
// Flow: local proxy → kube-apiserver (auth) → kubeopencode server → agent OpenCode server
func runAgentAttachServiceProxy(ctx context.Context, namespace, agentName string, localPort int, svcNamespace, svcName string, svcPort int) error {
	if err := checkBinary("opencode"); err != nil {
		return fmt.Errorf("opencode not found: %w\n  Install: https://opencode.ai", err)
	}

	fmt.Println("Connecting to cluster...")

	cfg, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("cannot connect to cluster: %w", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Look up the Agent to verify it exists and is ready
	fmt.Printf("Looking up agent %s/%s...\n", namespace, agentName)

	var agent kubeopenv1alpha1.Agent
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: agentName, Namespace: namespace}, &agent); err != nil {
		return fmt.Errorf("agent %q not found in namespace %q: %w", agentName, namespace, err)
	}

	if !controller.IsServerMode(&agent) {
		return fmt.Errorf("agent %q is not in Server mode (no serverConfig)\n  Only server-mode agents support interactive attach", agentName)
	}

	agentPort := controller.GetServerPort(&agent)
	if localPort == 0 {
		localPort = int(agentPort)
	}

	if agent.Status.ServerStatus == nil || agent.Status.ServerStatus.ReadyReplicas == 0 {
		deploymentName := controller.ServerDeploymentName(agentName)
		return fmt.Errorf("agent %q server is not ready (0 ready replicas)\n  Check: kubectl get deployment %s -n %s", agentName, deploymentName, namespace)
	}

	fmt.Printf("Agent ready (%d replicas)\n", agent.Status.ServerStatus.ReadyReplicas)

	// Create authenticated transport for kube-apiserver
	transport, err := rest.TransportFor(cfg)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	// Parse the kube-apiserver URL from kubeconfig.
	// cfg.Host may be "host:port" without scheme, which url.Parse misparses
	// (treats "host" as scheme). Ensure scheme is present.
	apiServerHost := cfg.Host
	apiServerURL, err := url.Parse(apiServerHost)
	if err != nil || apiServerURL.Scheme == "" || apiServerURL.Host == "" {
		apiServerURL, err = url.Parse("https://" + apiServerHost)
		if err != nil {
			return fmt.Errorf("invalid kube-apiserver URL %q: %w", apiServerHost, err)
		}
	}

	if !isPortAvailable(localPort) {
		return fmt.Errorf("local port %d is already in use\n  Use --local-port to specify a different port", localPort)
	}

	// Build the service proxy path prefix:
	// /api/v1/namespaces/{server-ns}/services/{server-svc}:{server-port}/proxy
	//   /api/v1/namespaces/{agent-ns}/agents/{agent-name}/proxy
	serviceProxyPrefix := fmt.Sprintf(
		"/api/v1/namespaces/%s/services/%s:%d/proxy/api/v1/namespaces/%s/agents/%s/proxy",
		svcNamespace, svcName, svcPort, namespace, agentName,
	)

	// Create the local reverse proxy
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = apiServerURL.Scheme
			req.URL.Host = apiServerURL.Host
			// /event → /.../proxy/.../proxy/event
			req.URL.Path = serviceProxyPrefix + req.URL.Path
			req.Host = apiServerURL.Host
		},
		Transport:     transport,
		FlushInterval: -1, // Immediate flush for SSE streaming
	}

	localServer := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", localPort),
		Handler:      proxy,
		WriteTimeout: 0, // Disabled for SSE long-lived connections
	}

	// Start the local proxy server
	errChan := make(chan error, 1)
	go func() {
		if err := localServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	localURL := fmt.Sprintf("http://localhost:%d", localPort)

	// Wait for the local proxy to be ready, checking for startup errors
	if err := waitForPort(localPort, 5*time.Second); err != nil {
		// Check if there's a more specific startup error (e.g., bind permission denied)
		select {
		case startErr := <-errChan:
			return fmt.Errorf("local proxy failed to start: %w", startErr)
		default:
			return fmt.Errorf("local proxy failed to start: %w", err)
		}
	}

	fmt.Printf("Local proxy ready: %s\n", localURL)
	fmt.Printf("Launching opencode attach...\n\n")

	// Launch opencode attach
	attachCmd := exec.CommandContext(ctx, "opencode", "attach", localURL)
	attachCmd.Stdin = os.Stdin
	attachCmd.Stdout = os.Stdout
	attachCmd.Stderr = os.Stderr

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		_ = localServer.Close()
	}()

	err = attachCmd.Run()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = localServer.Shutdown(shutdownCtx)

	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("opencode attach exited with error: %w", err)
	}

	fmt.Println("\nSession ended.")
	return nil
}

// runAgentAttachPortForward is the legacy mode using kubectl port-forward.
func runAgentAttachPortForward(ctx context.Context, namespace, agentName string, localPort int) error {
	if err := checkBinary("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found: %w\n  Install: https://kubernetes.io/docs/tasks/tools/", err)
	}
	if err := checkBinary("opencode"); err != nil {
		return fmt.Errorf("opencode not found: %w\n  Install: https://opencode.ai", err)
	}

	fmt.Println("Connecting to cluster...")

	cfg, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("cannot connect to cluster: %w", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	fmt.Printf("Looking up agent %s/%s...\n", namespace, agentName)

	var agent kubeopenv1alpha1.Agent
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: agentName, Namespace: namespace}, &agent); err != nil {
		return fmt.Errorf("agent %q not found in namespace %q: %w", agentName, namespace, err)
	}

	if !controller.IsServerMode(&agent) {
		return fmt.Errorf("agent %q is not in Server mode (no serverConfig)\n  Only server-mode agents support interactive attach", agentName)
	}

	serverPort := controller.GetServerPort(&agent)
	deploymentName := controller.ServerDeploymentName(agentName)

	if localPort == 0 {
		localPort = int(serverPort)
	}

	if agent.Status.ServerStatus == nil || agent.Status.ServerStatus.ReadyReplicas == 0 {
		return fmt.Errorf("agent %q server is not ready (0 ready replicas)\n  Check: kubectl get deployment %s -n %s", agentName, deploymentName, namespace)
	}

	fmt.Printf("Agent found: %s (port %d, %d ready replicas)\n", deploymentName, serverPort, agent.Status.ServerStatus.ReadyReplicas)

	if !isPortAvailable(localPort) {
		return fmt.Errorf("local port %d is already in use\n  Use --local-port to specify a different port", localPort)
	}

	fmt.Printf("Starting port-forward (localhost:%d -> %s:%d)...\n", localPort, deploymentName, serverPort)

	pfCtx, pfCancel := context.WithCancel(ctx)
	defer pfCancel()

	pfCmd := exec.CommandContext(pfCtx, "kubectl", "port-forward",
		"-n", namespace,
		fmt.Sprintf("deployment/%s", deploymentName),
		fmt.Sprintf("%d:%d", localPort, serverPort),
	)
	pfCmd.Stderr = os.Stderr

	if err := pfCmd.Start(); err != nil {
		return fmt.Errorf("failed to start port-forward: %w", err)
	}

	localURL := fmt.Sprintf("http://localhost:%d", localPort)
	if err := waitForPort(localPort, 15*time.Second); err != nil {
		pfCancel()
		_ = pfCmd.Wait()
		return fmt.Errorf("port-forward failed to start: %w", err)
	}

	fmt.Printf("Port-forward ready: %s\n", localURL)
	fmt.Printf("Launching opencode attach...\n\n")

	attachCmd := exec.CommandContext(ctx, "opencode", "attach", localURL)
	attachCmd.Stdin = os.Stdin
	attachCmd.Stdout = os.Stdout
	attachCmd.Stderr = os.Stderr

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		pfCancel()
	}()

	err = attachCmd.Run()

	pfCancel()
	_ = pfCmd.Wait()

	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("opencode attach exited with error: %w", err)
	}

	fmt.Println("\nSession ended. Port-forward cleaned up.")
	return nil
}

// checkBinary verifies a binary is available in PATH.
func checkBinary(name string) error {
	_, err := exec.LookPath(name)
	return err
}

// isPortAvailable checks if a TCP port is available locally.
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// waitForPort waits for a TCP port to become available.
func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("port %d did not become available within %s", port, timeout)
}
