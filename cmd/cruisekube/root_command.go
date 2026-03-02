package main

import (
	"context"

	"github.com/truefoundry/cruisekube/pkg/logging"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newRootCommand(ctx context.Context) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "cruisekube",
		Short: "Kubernetes resource cruisekube",
		RunE:  runCruiseKube,
	}

	addPersistentFlags(rootCmd)
	bindPersistentFlags(ctx, rootCmd)

	return rootCmd
}

func addPersistentFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVar(&configFilePath, "config-file-path", "config.yaml", "Path to configuration file")
	rootCmd.PersistentFlags().String("execution-mode", "", "Execution mode: controller|webhook|both")
	rootCmd.PersistentFlags().String("controller-mode", "", "Controller mode: local|inCluster")
	rootCmd.PersistentFlags().String("kubeconfig-path", "", "Path to kubeconfig file (local mode)")
	rootCmd.PersistentFlags().String("prometheus-url", "", "Prometheus URL")
	rootCmd.PersistentFlags().String("server-port", "", "Server port")
	rootCmd.PersistentFlags().Bool("enable-dev-apis", false, "Enable development APIs")
	rootCmd.PersistentFlags().String("webhook-port", "", "Webhook port")
	rootCmd.PersistentFlags().String("webhook-certs-dir", "", "Webhook certificates directory")
	rootCmd.PersistentFlags().String("webhook-stats-url-host", "", "Webhook stats URL host")
	rootCmd.PersistentFlags().String("db-file-path", "", "Database file path")
	rootCmd.PersistentFlags().Bool("apply-recommendation-dry-run", false, "Apply recommendation dry run")
}

func bindPersistentFlags(ctx context.Context, rootCmd *cobra.Command) {
	bindPFlagOrFatal(ctx, "controllerMode", rootCmd.PersistentFlags().Lookup("controller-mode"))
	bindPFlagOrFatal(ctx, "executionMode", rootCmd.PersistentFlags().Lookup("execution-mode"))
	bindPFlagOrFatal(ctx, "dependencies.local.kubeconfigPath", rootCmd.PersistentFlags().Lookup("kubeconfig-path"))
	bindPFlagOrFatal(ctx, "dependencies.local.prometheusURL", rootCmd.PersistentFlags().Lookup("prometheus-url"))
	bindPFlagOrFatal(ctx, "dependencies.inCluster.prometheusURL", rootCmd.PersistentFlags().Lookup("prometheus-url"))
	bindPFlagOrFatal(ctx, "server.port", rootCmd.PersistentFlags().Lookup("server-port"))
	bindPFlagOrFatal(ctx, "server.enableDevAPIs", rootCmd.PersistentFlags().Lookup("enable-dev-apis"))
	bindPFlagOrFatal(ctx, "webhook.port", rootCmd.PersistentFlags().Lookup("webhook-port"))
	bindPFlagOrFatal(ctx, "webhook.certsDir", rootCmd.PersistentFlags().Lookup("webhook-certs-dir"))
	bindPFlagOrFatal(ctx, "webhook.statsURL.host", rootCmd.PersistentFlags().Lookup("webhook-stats-url-host"))
	bindPFlagOrFatal(ctx, "controller.tasks.applyRecommendation.dryRun", rootCmd.PersistentFlags().Lookup("apply-recommendation-dry-run"))
	bindPFlagOrFatal(ctx, "db.filePath", rootCmd.PersistentFlags().Lookup("db-file-path"))
}

func bindPFlagOrFatal(ctx context.Context, key string, flag *pflag.Flag) {
	if err := v.BindPFlag(key, flag); err != nil {
		logging.Fatalf(ctx, "Failed to bind flag: %v", err)
	}
}
