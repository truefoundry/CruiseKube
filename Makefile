.PHONY: grafana-up grafana-down grafana-restart grafana-logs grafana-regenerate help

GRAFANA_CONTAINER_NAME := autopilot-grafana
GRAFANA_PORT := 3000
GRAFANA_CONFIG_DIR := $(shell pwd)/grafana

help:
	@echo "Available targets:"
	@echo "  grafana-up         - Start Grafana container"
	@echo "  grafana-down       - Stop and remove Grafana container"
	@echo "  grafana-restart    - Restart Grafana container"
	@echo "  grafana-regenerate - Regenerate datasources and restart Grafana"
	@echo "  grafana-logs       - Show Grafana container logs"
	@echo "  help              - Show this help message"

grafana-up:
	@echo "Starting Grafana container..."
	@docker run -d \
		--name $(GRAFANA_CONTAINER_NAME) \
		-p $(GRAFANA_PORT):3000 \
		-v $(GRAFANA_CONFIG_DIR)/provisioning:/etc/grafana/provisioning \
		-v $(GRAFANA_CONFIG_DIR)/dashboards:/var/lib/grafana/dashboards \
		-e GF_SECURITY_ADMIN_PASSWORD=admin \
		-e GF_USERS_ALLOW_SIGN_UP=false \
		-e GF_INSTALL_PLUGINS=grafana-clock-panel,grafana-simple-json-datasource,yesoreyeram-infinity-datasource \
		grafana/grafana:latest
	@echo "Grafana is starting up..."
	@echo "Access it at: http://localhost:$(GRAFANA_PORT)"
	@echo "Default credentials: admin/admin"
	@echo "Waiting for Grafana to be ready..."
	@sleep 10
	@echo "Grafana should now be accessible!"

grafana-down:
	@echo "Stopping Grafana container..."
	@docker stop $(GRAFANA_CONTAINER_NAME) 2>/dev/null || true
	@docker rm $(GRAFANA_CONTAINER_NAME) 2>/dev/null || true
	@echo "Grafana container stopped and removed."

grafana-restart: grafana-down grafana-up

grafana-logs:
	@docker logs -f $(GRAFANA_CONTAINER_NAME)

grafana-regenerate:
	@echo "Regenerating Grafana datasources..."
	@cd $(GRAFANA_CONFIG_DIR) && ./generate-datasources.sh
	@echo "Restarting Grafana with updated datasources..."
	@$(MAKE) grafana-restart
