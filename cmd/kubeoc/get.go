// Copyright Contributors to the KubeOpenCode project

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

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
	cmd.AddCommand(newGetAgentTemplatesCmd())
	cmd.AddCommand(newGetTasksCmd())
	cmd.AddCommand(newGetCronTasksCmd())
	cmd.AddCommand(newGetRegistriesCmd())
	return cmd
}

// outputFormat prints items as JSON or YAML if the output flag is set.
// Returns true if output was handled (caller should return), false if table output should be used.
func outputFormat(output string, items interface{}) (bool, error) {
	switch output {
	case "json":
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return true, fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return true, nil
	case "yaml":
		data, err := yaml.Marshal(items)
		if err != nil {
			return true, fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Print(string(data))
		return true, nil
	case "":
		return false, nil
	default:
		return true, fmt.Errorf("unknown output format %q (supported: json, yaml)", output)
	}
}

func newGetAgentsCmd() *cobra.Command {
	var (
		namespace string
		wide      bool
		output    string
	)

	cmd := &cobra.Command{
		Use:   "agents",
		Short: "List available agents",
		Long: `List agents across all namespaces (or a specific namespace with -n).

Use --wide to show additional columns (profile, template).
Use -o json or -o yaml to output in structured format.

Examples:
  kubeoc get agents
  kubeoc get agents -n production
  kubeoc get agents --wide
  kubeoc get agents -o json
  kubeoc get agents -o yaml`,
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

			// Handle structured output formats
			if handled, err := outputFormat(output, agents); handled {
				return err
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
			if wide {
				_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tSTATUS\tPROFILE\tTEMPLATE")
			} else {
				_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tSTATUS")
			}

			for _, agent := range agents.Items {
				var status string
				switch {
				case agent.Status.Suspended:
					status = "Suspended"
				case agent.Status.Ready:
					status = "Ready"
				default:
					status = "Not Ready"
				}

				if wide {
					profile := agent.Spec.Profile
					if len(profile) > 50 {
						profile = profile[:47] + "..."
					}

					template := "-"
					if agent.Spec.TemplateRef != nil {
						template = agent.Spec.TemplateRef.Name
					}

					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						agent.Namespace, agent.Name, status, profile, template)
				} else {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
						agent.Namespace, agent.Name, status)
				}
			}

			_ = w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Filter by namespace (default: all namespaces)")
	cmd.Flags().BoolVar(&wide, "wide", false, "Show additional columns (profile, template)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format: json, yaml")

	return cmd
}
