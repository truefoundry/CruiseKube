package main

import (
	"net/http"
	"time"

	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/handlers"
	"github.com/truefoundry/cruisekube/pkg/middleware"
	"github.com/truefoundry/cruisekube/pkg/server"
)

func startWebhookRuntime(runtimeManager *runtimeManager, cfg *config.Config) {
	webhookPort := cfg.Webhook.Port
	certDir := cfg.Webhook.CertsDir

	handlers.InitRecommenderServiceClient(cfg)
	webhookEngine := server.SetupWebhookServerEngine(middleware.Common(nil, cfg)...)
	webhookServer := &http.Server{
		Addr:              ":" + webhookPort,
		Handler:           webhookEngine,
		ReadHeaderTimeout: 5 * time.Second,
	}
	startHTTPServer(runtimeManager, "webhook server", "Starting webhook server on :"+webhookPort, webhookServer, func(server *http.Server) error {
		if certDir == "dev" {
			return server.ListenAndServe()
		}

		return server.ListenAndServeTLS(certDir+"/tls.crt", certDir+"/tls.key")
	})
}
