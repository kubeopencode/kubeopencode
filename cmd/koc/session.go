// Copyright Contributors to the KubeOpenCode project

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
)

func init() {
	rootCmd.AddCommand(newSessionCmd())
}

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Interact with agent sessions",
	}
	cmd.AddCommand(newSessionWatchCmd())
	cmd.AddCommand(newSessionAttachCmd())
	return cmd
}

func newSessionWatchCmd() *cobra.Command {
	var (
		namespace       string
		serverNamespace string
		serverService   string
		serverPort      int
	)

	cmd := &cobra.Command{
		Use:   "watch <task-name>",
		Short: "Watch agent events for a task",
		Long: `Watch SSE events from an agent session in real-time.

Connects through the kube-apiserver's service proxy to the kubeopencode server,
which proxies to the OpenCode agent server. No port-forward needed.

Examples:
  koc session watch my-task -n test
  koc session watch my-task -n production`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionProxy(cmd.Context(), namespace, args[0], false, serverNamespace, serverService, serverPort)
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Task namespace")
	cmd.Flags().StringVar(&serverNamespace, "server-namespace", defaultServerNamespace, "Namespace where kubeopencode server is deployed")
	cmd.Flags().StringVar(&serverService, "server-service", defaultServerService, "Name of kubeopencode server Service")
	cmd.Flags().IntVar(&serverPort, "server-port", defaultServerPort, "Port of kubeopencode server Service")
	return cmd
}

func newSessionAttachCmd() *cobra.Command {
	var (
		namespace       string
		serverNamespace string
		serverService   string
		serverPort      int
	)

	cmd := &cobra.Command{
		Use:   "attach <task-name>",
		Short: "Interactively attach to an agent session",
		Long: `Attach to an agent session with interactive HITL (Human-in-the-Loop) support.

Watches SSE events and allows you to respond to permission requests and
questions from the agent. Connects through the kube-apiserver's service proxy.
No port-forward needed.

Examples:
  koc session attach my-task -n test
  koc session attach my-task -n production`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionProxy(cmd.Context(), namespace, args[0], true, serverNamespace, serverService, serverPort)
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Task namespace")
	cmd.Flags().StringVar(&serverNamespace, "server-namespace", defaultServerNamespace, "Namespace where kubeopencode server is deployed")
	cmd.Flags().StringVar(&serverService, "server-service", defaultServerService, "Name of kubeopencode server Service")
	cmd.Flags().IntVar(&serverPort, "server-port", defaultServerPort, "Port of kubeopencode server Service")
	return cmd
}

// runSessionProxy connects to a task's agent session via kube-apiserver service proxy.
// Flow: koc → kube-apiserver (auth) → kubeopencode server → OpenCode agent server
func runSessionProxy(ctx context.Context, namespace, taskName string, interactive bool, svcNamespace, svcName string, svcPort int) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Println("Connecting to cluster...")

	cfg, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("cannot connect to cluster: %w", err)
	}

	// Build the service proxy base URL for HITL API
	// /api/v1/namespaces/{server-ns}/services/{server-svc}:{server-port}/proxy
	//   /api/v1/namespaces/{task-ns}/tasks/{task-name}
	serviceProxyPrefix := fmt.Sprintf(
		"/api/v1/namespaces/%s/services/%s:%d/proxy/api/v1/namespaces/%s/tasks/%s",
		svcNamespace, svcName, svcPort, namespace, taskName,
	)

	transport, err := rest.TransportFor(cfg)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	apiServerURL, err := parseAPIServerURL(cfg.Host)
	if err != nil {
		return err
	}

	// Create a helper to build full URLs for API calls
	baseURL := fmt.Sprintf("%s://%s%s", apiServerURL.Scheme, apiServerURL.Host, serviceProxyPrefix)

	if interactive {
		fmt.Printf("[info] Interactive mode — task: %s/%s. Ctrl+C to disconnect.\n\n", namespace, taskName)
	} else {
		fmt.Printf("[info] Watch mode — task: %s/%s. Ctrl+C to stop.\n\n", namespace, taskName)
	}

	// Connect to SSE events endpoint
	eventsURL := baseURL + "/events"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, eventsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   0, // No timeout for SSE
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to event stream: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Println("[connected] Event stream established")

	reader := bufio.NewReader(resp.Body)
	scanner := bufio.NewScanner(os.Stdin)

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n[info] Disconnected.\n")
			return nil
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					fmt.Println("[info] Stream ended.")
					return nil
				}
				return fmt.Errorf("read error: %w", err)
			}

			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)
			props, _ := event["properties"].(map[string]interface{})
			if props == nil {
				props = event
			}

			switch eventType {
			case "server.connected":
				fmt.Println("[connected] Session established")

			case "server.heartbeat", "stream.closed":
				if eventType == "stream.closed" {
					fmt.Println("[info] Stream closed by server.")
					return nil
				}

			case "permission.asked":
				permission, _ := props["permission"].(string)
				id, _ := props["id"].(string)
				patterns := toStringSlice(props["patterns"])
				fmt.Printf("[permission] %s on %s\n", permission, strings.Join(patterns, ", "))

				if interactive && id != "" {
					fmt.Printf("  [a]llow once / [A]lways / [r]eject: ")
					if scanner.Scan() {
						reply := parseReply(scanner.Text())
						payload, _ := json.Marshal(map[string]string{"reply": reply})
						permURL := baseURL + "/permission/" + id
						if err := postWithTransport(ctx, transport, permURL, payload); err != nil {
							fmt.Printf("  [error] %s\n", err)
						} else {
							fmt.Printf("  [replied] %s\n", reply)
						}
					}
				}

			case "question.asked":
				id, _ := props["id"].(string)
				questions, _ := props["questions"].([]interface{})
				fmt.Println("[question] Agent asks:")
				for qi, q := range questions {
					qMap, _ := q.(map[string]interface{})
					question, _ := qMap["question"].(string)
					options, _ := qMap["options"].([]interface{})
					fmt.Printf("  %d. %s\n", qi+1, question)
					for oi, opt := range options {
						optMap, _ := opt.(map[string]interface{})
						label, _ := optMap["label"].(string)
						desc, _ := optMap["description"].(string)
						fmt.Printf("     %d) %s", oi+1, label)
						if desc != "" {
							fmt.Printf(" - %s", desc)
						}
						fmt.Println()
					}
				}

				if interactive && id != "" {
					fmt.Printf("  Enter choice (number/text, 's' to skip): ")
					if scanner.Scan() {
						input := strings.TrimSpace(scanner.Text())
						if strings.ToLower(input) == "s" {
							rejectURL := baseURL + "/question/" + id + "/reject"
							_ = postWithTransport(ctx, transport, rejectURL, nil)
							fmt.Println("  [skipped]")
						} else {
							payload, _ := json.Marshal(map[string]interface{}{"answers": [][]string{{input}}})
							questionURL := baseURL + "/question/" + id
							_ = postWithTransport(ctx, transport, questionURL, payload)
							fmt.Printf("  [answered] %s\n", input)
						}
					}
				}

			case "session.status":
				if status, ok := props["status"].(map[string]interface{}); ok {
					if t, _ := status["type"].(string); t != "" {
						fmt.Printf("[status] %s\n", t)
					}
				}

			case "message.part.delta":
				if delta, _ := props["delta"].(string); delta != "" {
					fmt.Print(delta)
				}

			case "message.updated":
				fmt.Println()

			case "session.error":
				if errMsg, _ := props["error"].(string); errMsg != "" {
					fmt.Printf("[error] %s\n", errMsg)
				}
			}
		}
	}
}

// parseAPIServerURL parses the kube-apiserver URL from kubeconfig,
// handling the case where cfg.Host may be "host:port" without a scheme.
func parseAPIServerURL(host string) (*url.URL, error) {
	u, err := url.Parse(host)
	if err != nil || u.Scheme == "" || u.Host == "" {
		u, err = url.Parse("https://" + host)
		if err != nil {
			return nil, fmt.Errorf("invalid kube-apiserver URL %q: %w", host, err)
		}
	}
	return u, nil
}

// postWithTransport sends an authenticated POST request through the kube-apiserver.
func postWithTransport(ctx context.Context, transport http.RoundTripper, targetURL string, body []byte) error {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// toStringSlice converts an interface{} to []string (used for SSE event parsing).
func toStringSlice(v interface{}) []string {
	arr, _ := v.([]interface{})
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// parseReply converts user input to a permission reply value.
func parseReply(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "aa", "always":
		return "always"
	case "r", "reject":
		return "reject"
	default:
		return "once"
	}
}

