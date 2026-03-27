// Copyright Contributors to the KubeOpenCode project

package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

func init() {
	rootCmd.AddCommand(newGetCmd())
}

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Display KubeOpenCode resources",
	}
	cmd.AddCommand(newGetAgentsCmd())
	return cmd
}

func newGetAgentsCmd() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "agents",
		Short: "List available agents",
		Long: `List agents across all namespaces (or a specific namespace with -n).

Displays each agent's namespace, name, profile, mode (Server/Pod), and status.

Examples:
  kubeopencode get agents
  kubeopencode get agents -n production`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := getKubeConfig()
			if err != nil {
				return fmt.Errorf("cannot connect to cluster: %w", err)
			}

			k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			var agents kubeopenv1alpha1.AgentList
			listOpts := []client.ListOption{}
			if namespace != "" {
				listOpts = append(listOpts, client.InNamespace(namespace))
			}

			if err := k8sClient.List(cmd.Context(), &agents, listOpts...); err != nil {
				return fmt.Errorf("failed to list agents: %w", err)
			}

			if len(agents.Items) == 0 {
				if namespace != "" {
					fmt.Printf("No agents found in namespace %q\n", namespace)
				} else {
					fmt.Println("No agents found")
				}
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAMESPACE\tNAME\tPROFILE\tMODE\tSTATUS")

			for _, agent := range agents.Items {
				mode := "Pod"
				status := "-"

				if agent.Spec.ServerConfig != nil {
					mode = "Server"
					if agent.Status.ServerStatus != nil && agent.Status.ServerStatus.ReadyReplicas > 0 {
						status = "Ready"
					} else {
						status = "Not Ready"
					}
				}

				profile := agent.Spec.Profile
				if len(profile) > 60 {
					profile = profile[:57] + "..."
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					agent.Namespace, agent.Name, profile, mode, status)
			}

			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Filter by namespace (default: all namespaces)")

	return cmd
}
