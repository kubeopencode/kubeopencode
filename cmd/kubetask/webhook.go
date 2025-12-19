// Copyright Contributors to the KubeTask project

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kubetask/kubetask/internal/controller"
	"github.com/kubetask/kubetask/internal/webhook"
)

func init() {
	rootCmd.AddCommand(webhookCmd)
}

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Start the webhook server for WebhookTrigger",
	Long: `Start an HTTP server that receives webhooks and creates Tasks
based on WebhookTrigger resources.

The server watches WebhookTrigger CRDs and dynamically registers
routes at /webhooks/<namespace>/<trigger-name>.

Examples:
  # Start webhook server on port 8081
  kubetask webhook --port=8081

  # Start with custom metrics endpoint
  kubetask webhook --port=8081 --metrics-bind-address=:8082`,
	RunE: runWebhook,
}

// Webhook flags
var (
	webhookPort        int
	webhookMetricsAddr string
	webhookProbeAddr   string
)

func init() {
	webhookCmd.Flags().IntVar(&webhookPort, "port", 8080,
		"The port the webhook server listens on.")
	webhookCmd.Flags().StringVar(&webhookMetricsAddr, "metrics-bind-address", ":8081",
		"The address the metric endpoint binds to.")
	webhookCmd.Flags().StringVar(&webhookProbeAddr, "health-probe-bind-address", ":8084",
		"The address the probe endpoint binds to.")
}

func runWebhook(cmd *cobra.Command, args []string) error {
	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	logger := ctrl.Log.WithName("webhook")

	logger.Info("Starting webhook server",
		"port", webhookPort,
		"metrics", webhookMetricsAddr,
		"probes", webhookProbeAddr,
	)

	// Create manager for controller-runtime client
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: webhookMetricsAddr},
		HealthProbeBindAddress: webhookProbeAddr,
		LeaderElection:         false, // Webhook server doesn't need leader election
	})
	if err != nil {
		logger.Error(err, "Unable to start manager")
		os.Exit(1)
	}

	// Add health check handlers for Kubernetes probes
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	// Create webhook server
	webhookServer := webhook.NewServer(mgr.GetClient(), logger)

	// Setup WebhookTrigger controller to register triggers with the webhook server
	if err = (&controller.WebhookTriggerReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		WebhookServer: webhookServer,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create controller", "controller", "WebhookTrigger")
		os.Exit(1)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start manager in background (for controller)
	go func() {
		if err := mgr.Start(ctx); err != nil {
			logger.Error(err, "Problem running manager")
			os.Exit(1)
		}
	}()

	// Start webhook server
	errCh := make(chan error, 1)
	go func() {
		errCh <- webhookServer.Start(ctx, webhookPort)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigCh:
		logger.Info("Received signal, shutting down", "signal", sig)
		cancel()
	case err := <-errCh:
		if err != nil {
			logger.Error(err, "Webhook server error")
			return err
		}
	}

	logger.Info("Shutdown complete")
	return nil
}
