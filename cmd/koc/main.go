// Copyright Contributors to the KubeOpenCode project

// KubeOpenCode CLI for interactive agent sessions.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubeopenv1alpha1.AddToScheme(scheme))
}

var rootCmd = &cobra.Command{
	Use:   "koc",
	Short: "KubeOpenCode CLI",
	Long: `koc is the KubeOpenCode CLI for managing agents and interactive sessions.

Commands:
  get agents       List available agents across namespaces
  agent attach     Attach to a server-mode agent via OpenCode TUI

Kubeconfig resolution (in priority order):
  1. KUBEOPENCODE_KUBECONFIG environment variable
  2. KUBECONFIG environment variable
  3. Default ~/.kube/config

Examples:
  koc get agents
  koc agent attach my-agent -n test`,
}

// getKubeConfig returns a rest.Config with the following priority:
//  1. KUBEOPENCODE_KUBECONFIG env var (dedicated agent cluster config)
//  2. KUBECONFIG env var
//  3. Default ~/.kube/config
func getKubeConfig() (*rest.Config, error) {
	if path := os.Getenv("KUBEOPENCODE_KUBECONFIG"); path != "" {
		return clientcmd.BuildConfigFromFlags("", path)
	}
	// Falls back to KUBECONFIG env var, then default path
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
