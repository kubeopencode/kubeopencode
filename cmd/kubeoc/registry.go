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

func newGetRegistriesCmd() *cobra.Command {
	var (
		namespace string
		wide      bool
		output    string
	)

	cmd := &cobra.Command{
		Use:     "registries",
		Aliases: []string{"reg", "registry"},
		Short:   "List registries (asset catalogs)",
		Long: `List registries across all namespaces (or a specific namespace with -n).

Registries are asset catalogs that index executor images, skills, and plugins
for agent assembly.

Use --wide to show additional columns (ready/total assets).
Use -o json or -o yaml to output in structured format.

Examples:
  kubeoc get registries
  kubeoc get registries -n dev
  kubeoc get registries --wide
  kubeoc get registries -o yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := getKubeConfig()
			if err != nil {
				return fmt.Errorf("cannot connect to cluster: %w", err)
			}

			k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			var registries kubeopenv1alpha1.RegistryList
			listOpts := []client.ListOption{}
			if namespace != "" {
				listOpts = append(listOpts, client.InNamespace(namespace))
			}

			if err := k8sClient.List(cmd.Context(), &registries, listOpts...); err != nil {
				return fmt.Errorf("failed to list registries: %w", err)
			}

			// Handle structured output formats
			if handled, err := outputFormat(output, registries); handled {
				return err
			}

			if len(registries.Items) == 0 {
				if namespace != "" {
					fmt.Printf("No registries found in namespace %q\n", namespace)
				} else {
					fmt.Println("No registries found")
				}
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			if wide {
				_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tIMAGES\tSKILLS\tPLUGINS\tREADY\tTOTAL")
			} else {
				_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tREADY/TOTAL")
			}

			for _, reg := range registries.Items {
				if wide {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%d\n",
						reg.Namespace,
						reg.Name,
						reg.Status.Summary.Images,
						reg.Status.Summary.Skills,
						reg.Status.Summary.Plugins,
						reg.Status.Summary.ReadyCount,
						reg.Status.Summary.TotalCount,
					)
				} else {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%d/%d\n",
						reg.Namespace,
						reg.Name,
						reg.Status.Summary.ReadyCount,
						reg.Status.Summary.TotalCount,
					)
				}
			}

			_ = w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Filter by namespace (default: all namespaces)")
	cmd.Flags().BoolVar(&wide, "wide", false, "Show additional columns (images, skills, plugins, ready, total)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format: json, yaml")

	return cmd
}
