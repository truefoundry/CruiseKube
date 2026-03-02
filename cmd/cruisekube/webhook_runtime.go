package main

import (
	"context"

	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/handlers"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/middleware"
	"github.com/truefoundry/cruisekube/pkg/server"
)

func startWebhookRuntime(ctx context.Context, cfg *config.Config) {
	webhookPort := cfg.Webhook.Port
	certDir := cfg.Webhook.CertsDir

	handlers.InitRecommenderServiceClient(cfg)
	webhookEngine := server.SetupWebhookServerEngine(middleware.Common(nil, cfg)...)

	go func() {
		if certDir == "dev" {
			if err := webhookEngine.Run(":" + webhookPort); err != nil {
				logging.Fatalf(ctx, "HTTP server failed: %v", err)
			}
			return
		}

		if err := webhookEngine.RunTLS(":"+webhookPort, certDir+"/tls.crt", certDir+"/tls.key"); err != nil {
			logging.Fatalf(ctx, "HTTPS server failed: %v", err)
		}
	}()
}
