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
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	cmd.AddCommand(newAgentSuspendCmd())
	cmd.AddCommand(newAgentResumeCmd())
	cmd.AddCommand(newAgentShareCmd())
	cmd.AddCommand(newAgentUnshareCmd())
	return cmd
}

func newAgentSuspendCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "suspend <agent-name>",
		Short: "Suspend a server-mode agent",
		Long: `Suspend a server-mode agent by scaling its deployment to 0 replicas.

PVCs and Service are retained, so the agent can be resumed without data loss.
Tasks targeting a suspended agent enter Queued phase until the agent is resumed.

Examples:
  kubeoc agent suspend my-agent -n test`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setSuspendState(cmd.Context(), namespace, args[0], true)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Agent namespace")
	return cmd
}

func newAgentResumeCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "resume <agent-name>",
		Short: "Resume a suspended server-mode agent",
		Long: `Resume a suspended server-mode agent by scaling its deployment back to 1 replica.

Queued tasks will automatically start running once the agent is ready.

Examples:
  kubeoc agent resume my-agent -n test`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setSuspendState(cmd.Context(), namespace, args[0], false)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Agent namespace")
	return cmd
}

func setSuspendState(ctx context.Context, namespace, agentName string, suspend bool) error {
	cfg, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("cannot connect to cluster: %w", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	var agent kubeopenv1alpha1.Agent
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      agentName,
		Namespace: namespace,
	}, &agent); err != nil {
		return fmt.Errorf("agent %q not found in namespace %q: %w", agentName, namespace, err)
	}

	if agent.Spec.Suspend == suspend {
		if suspend {
			fmt.Printf("Agent %s/%s is already suspended\n", namespace, agentName)
		} else {
			fmt.Printf("Agent %s/%s is already running\n", namespace, agentName)
		}
		return nil
	}

	agent.Spec.Suspend = suspend
	if err := k8sClient.Update(ctx, &agent); err != nil {
		action := "suspend"
		if !suspend {
			action = "resume"
		}
		return fmt.Errorf("failed to %s agent %q: %w", action, agentName, err)
	}

	if suspend {
		fmt.Printf("Agent %s/%s suspended\n", namespace, agentName)
	} else {
		fmt.Printf("Agent %s/%s resumed\n", namespace, agentName)
	}
	return nil
}

func newAgentShareCmd() *cobra.Command {
	var (
		namespace  string
		expiresIn  string
		allowedIPs []string
		readOnly   bool
		show       bool
	)

	cmd := &cobra.Command{
		Use:   "share <agent-name>",
		Short: "Enable share link for an agent",
		Long: `Enable a shareable terminal link for an agent.

When enabled, the controller generates a cryptographic token and the server
exposes a standalone terminal page at /s/{token}. Share this URL with users
who need terminal access without Kubernetes credentials.

Examples:
  kubeoc agent share my-agent -n test
  kubeoc agent share my-agent --expires-in 24h
  kubeoc agent share my-agent --allowed-ips 10.0.0.0/8,192.168.1.0/24 --read-only
  kubeoc agent share my-agent --show`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentShare(cmd.Context(), namespace, args[0], expiresIn, allowedIPs, readOnly, show)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Agent namespace")
	cmd.Flags().StringVar(&expiresIn, "expires-in", "", "Expiry duration (e.g., 1h, 24h, 168h)")
	cmd.Flags().StringSliceVar(&allowedIPs, "allowed-ips", nil, "Comma-separated CIDR ranges for IP allowlist")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Share terminal in read-only (view-only) mode")
	cmd.Flags().BoolVar(&show, "show", false, "Show existing share link info without modifying")
	return cmd
}

func newAgentUnshareCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "unshare <agent-name>",
		Short: "Disable share link for an agent",
		Long: `Disable the shareable terminal link for an agent.

The share token Secret is deleted and the link becomes invalid immediately.

Examples:
  kubeoc agent unshare my-agent -n test`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentUnshare(cmd.Context(), namespace, args[0])
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Agent namespace")
	return cmd
}

func runAgentShare(ctx context.Context, namespace, agentName, expiresIn string, allowedIPs []string, readOnly, showOnly bool) error {
	cfg, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("cannot connect to cluster: %w", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	var agent kubeopenv1alpha1.Agent
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      agentName,
		Namespace: namespace,
	}, &agent); err != nil {
		return fmt.Errorf("agent %q not found in namespace %q: %w", agentName, namespace, err)
	}

	// Show mode: display current share info and exit
	if showOnly {
		return showShareInfo(ctx, k8sClient, &agent)
	}

	// Build share config
	shareConfig := &kubeopenv1alpha1.ShareConfig{
		Enabled:    true,
		ReadOnly:   readOnly,
		AllowedIPs: allowedIPs,
	}

	if expiresIn != "" {
		d, err := time.ParseDuration(expiresIn)
		if err != nil {
			return fmt.Errorf("invalid --expires-in format (expected Go duration like 24h, 168h): %w", err)
		}
		expiresAt := metav1.NewTime(time.Now().Add(d))
		shareConfig.ExpiresAt = &expiresAt
	}

	agent.Spec.Share = shareConfig
	if err := k8sClient.Update(ctx, &agent); err != nil {
		return fmt.Errorf("failed to enable share for agent %q: %w", agentName, err)
	}

	fmt.Printf("Share link enabled for agent %s/%s\n", namespace, agentName)
	if readOnly {
		fmt.Println("Mode: read-only (view only)")
	}
	if len(allowedIPs) > 0 {
		fmt.Printf("Allowed IPs: %v\n", allowedIPs)
	}
	if expiresIn != "" {
		fmt.Printf("Expires in: %s\n", expiresIn)
	}

	// Wait briefly for controller to create the Secret, then show the token
	fmt.Println("\nWaiting for share token generation...")
	time.Sleep(3 * time.Second)
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      agentName,
		Namespace: namespace,
	}, &agent); err == nil {
		return showShareInfo(ctx, k8sClient, &agent)
	}

	fmt.Println("Share token is being generated. Run 'kubeoc agent share <name> --show' to see the URL.")
	return nil
}

func showShareInfo(ctx context.Context, k8sClient client.Client, agent *kubeopenv1alpha1.Agent) error {
	if agent.Spec.Share == nil || !agent.Spec.Share.Enabled {
		fmt.Printf("Share link is not enabled for agent %s/%s\n", agent.Namespace, agent.Name)
		return nil
	}

	if agent.Status.Share == nil || agent.Status.Share.SecretName == "" {
		fmt.Println("Share token is being generated. Please try again shortly.")
		return nil
	}

	// Read the token from the Secret
	var secret corev1.Secret
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      agent.Status.Share.SecretName,
		Namespace: agent.Namespace,
	}, &secret); err != nil {
		return fmt.Errorf("failed to read share secret: %w", err)
	}

	token := string(secret.Data[controller.ShareTokenKey])

	fmt.Printf("Agent:  %s/%s\n", agent.Namespace, agent.Name)
	fmt.Printf("Active: %v\n", agent.Status.Share.Active)
	if agent.Spec.Share.ReadOnly {
		fmt.Println("Mode:   read-only")
	}
	fmt.Printf("Token:  %s\n", token)
	fmt.Printf("Path:   /s/%s\n", token)
	if agent.Status.Share.URL != "" {
		fmt.Printf("URL:    %s\n", agent.Status.Share.URL)
	} else {
		fmt.Println("\nNote: Full URL requires server.externalURL to be configured in KubeOpenCodeConfig.")
		fmt.Printf("      Construct manually: https://<your-server>/s/%s\n", token)
	}

	return nil
}

func runAgentUnshare(ctx context.Context, namespace, agentName string) error {
	cfg, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("cannot connect to cluster: %w", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	var agent kubeopenv1alpha1.Agent
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      agentName,
		Namespace: namespace,
	}, &agent); err != nil {
		return fmt.Errorf("agent %q not found in namespace %q: %w", agentName, namespace, err)
	}

	if agent.Spec.Share == nil || !agent.Spec.Share.Enabled {
		fmt.Printf("Share link is already disabled for agent %s/%s\n", namespace, agentName)
		return nil
	}

	agent.Spec.Share = nil
	if err := k8sClient.Update(ctx, &agent); err != nil {
		return fmt.Errorf("failed to disable share for agent %q: %w", agentName, err)
	}

	fmt.Printf("Share link disabled for agent %s/%s\n", namespace, agentName)
	return nil
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

	agentPort := controller.GetServerPort(&agent)
	if localPort == 0 {
		localPort = int(agentPort)
	}

	if !agent.Status.Ready {
		deploymentName := controller.ServerDeploymentName(agentName)
		return fmt.Errorf("agent %q is not ready\n  Check: kubectl get deployment %s -n %s", agentName, deploymentName, namespace)
	}

	fmt.Println("Agent ready")

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

	localServer := &http.Server{ //nolint:gosec // ReadHeaderTimeout intentionally omitted for SSE streaming
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

	// Start connection heartbeat to prevent standby auto-suspend
	heartbeatCancel := startConnectionHeartbeat(ctx, k8sClient, namespace, agentName)
	defer heartbeatCancel()

	// Launch opencode attach
	attachCmd := exec.CommandContext(ctx, "opencode", "attach", localURL) //nolint:gosec // args are not user-controlled
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

	serverPort := controller.GetServerPort(&agent)
	deploymentName := controller.ServerDeploymentName(agentName)

	if localPort == 0 {
		localPort = int(serverPort)
	}

	if !agent.Status.Ready {
		return fmt.Errorf("agent %q is not ready\n  Check: kubectl get deployment %s -n %s", agentName, deploymentName, namespace)
	}

	fmt.Printf("Agent found: %s (port %d)\n", deploymentName, serverPort)

	if !isPortAvailable(localPort) {
		return fmt.Errorf("local port %d is already in use\n  Use --local-port to specify a different port", localPort)
	}

	fmt.Printf("Starting port-forward (localhost:%d -> %s:%d)...\n", localPort, deploymentName, serverPort)

	pfCtx, pfCancel := context.WithCancel(ctx)
	defer pfCancel()

	pfCmd := exec.CommandContext(pfCtx, "kubectl", "port-forward", //nolint:gosec // args are not user-controlled
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

	// Start connection heartbeat to prevent standby auto-suspend
	heartbeatCancel := startConnectionHeartbeat(ctx, k8sClient, namespace, agentName)
	defer heartbeatCancel()

	attachCmd := exec.CommandContext(ctx, "opencode", "attach", localURL) //nolint:gosec // args are not user-controlled
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
	_ = ln.Close()
	return true
}

// startConnectionHeartbeat starts a background goroutine that periodically updates
// the connection heartbeat annotation on the Agent to prevent standby auto-suspend.
// Returns a cancel function that stops the heartbeat.
func startConnectionHeartbeat(ctx context.Context, k8sClient client.Client, namespace, agentName string) context.CancelFunc {
	heartbeatCtx, cancel := context.WithCancel(ctx)

	// Warn once on first error so users know heartbeat is failing (e.g., missing RBAC)
	var warnOnce sync.Once

	go controller.RunConnectionHeartbeat(heartbeatCtx, k8sClient, namespace, agentName, func(err error) {
		warnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "Warning: connection heartbeat failed: %v\n", err)
		})
	})

	return cancel
}

// waitForPort waits for a TCP port to become available.
func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("port %d did not become available within %s", port, timeout)
}
