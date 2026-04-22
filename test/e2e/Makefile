# Variables
CLUSTER_NAME := cruisekube-e2e
NAMESPACE := cruisekube-system
CONTROLLER_IMAGE := cruisekube:latest
FRONTEND_IMAGE := cruisekube-frontend:latest
T ?= ./tests
TIMEOUT := 400s

.PHONY: all setup clean test

all:
	@echo "Run 'make build-images' to build images"
	@echo "Run 'make setup' to setup environment"
	@echo "Run 'make test' to run tests"
	@echo "Run 'make clean' to clean environment"

setup: build-images kind-up apply-deps
clean: kind-down

reset: uninstall-ck
	@echo "Deleting namespace..."
	-@kubectl delete namespace test-webhook --timeout=60s
	-@kubectl delete namespace test-workloads --timeout=60s
	-@kubectl delete namespace test-oom --timeout=60s
	-@kubectl delete namespace test-incomplete-stats --timeout=60s
	-@kubectl delete namespace $(NAMESPACE) --timeout=60s

test:
	@echo "Running KUTTL tests..."
	@echo "Current context: $$(kubectl config current-context)"
	@kubectl kuttl test --start-kind=false --test $(T)
	@echo "E2E Tests completed!"

.PHONY: build-images
build-images: ## Build controller image
	@echo "Building controller image..."
	@cd ../.. && docker build -t $(CONTROLLER_IMAGE) .
	@echo "Controller image built successfully"
	@echo "Building frontend image..."
	@cd ../../cruiseKube-frontend && docker build -t $(FRONTEND_IMAGE) .
	@echo "Frontend image built successfully"

.PHONY: kind-up
kind-up: ## Create Kind cluster and load images
	@echo "Creating Kind cluster '$(CLUSTER_NAME)'..."
	@if ! kind get clusters | grep -q $(CLUSTER_NAME); then \
		kind create cluster --config ./kind-config.yaml; \
		echo "Kind cluster '$(CLUSTER_NAME)' created successfully."; \
	else \
		echo "Kind cluster '$(CLUSTER_NAME)' already exists."; \
	fi
	@echo "Switching to kind cluster context..."
	@kubectl config use-context kind-$(CLUSTER_NAME)
	@echo "Waiting for cluster to be ready..."
	@kubectl wait --for=condition=Ready nodes --all --timeout=120s
	kubectl scale --replicas=1 deployment/coredns -n kube-system
	@echo "Loading controller image into kind cluster..."
	@kind load docker-image $(CONTROLLER_IMAGE) --name $(CLUSTER_NAME)
	@kind load docker-image $(FRONTEND_IMAGE) --name $(CLUSTER_NAME)
	@echo "Image loaded successfully"

.PHONY: kind-down
kind-down: ## Delete Kind cluster
	@echo "Deleting kind cluster '$(CLUSTER_NAME)'..."
	@if kind get clusters | grep -q $(CLUSTER_NAME); then \
		kind delete cluster --name $(CLUSTER_NAME); \
		echo "Kind cluster '$(CLUSTER_NAME)' deleted successfully."; \
	else \
		echo "Kind cluster '$(CLUSTER_NAME)' does not exist."; \
	fi

.PHONY: apply-deps
apply-deps: ## Install all dependencies
	@echo "Creating namespace..."
	@printf 'apiVersion: v1\nkind: Namespace\nmetadata:\n  name: %s\n' "$(NAMESPACE)" | kubectl apply -f -
	@echo "Applying controller admin credentials secret for e2e..."
	@kubectl apply -f ./manifest/global/controller-admin-secret.yaml
	@echo "Building Helm dependencies..."
	@cd ../../charts/cruisekube && helm dependency build
	@echo "Installing CruiseKube via Helm..."
	@helm upgrade --wait --timeout $(TIMEOUT) --install --create-namespace cruisekube ../../charts/cruisekube -n $(NAMESPACE) -f ./manifest/global/values-cruisekube.yaml
	@echo "All dependencies installed successfully."

.PHONY: reload-images
reload-images: ## Reload images into kind and restart deployments
	@echo "Loading images into kind cluster..."
	@kind load docker-image $(CONTROLLER_IMAGE) --name $(CLUSTER_NAME)
	@kind load docker-image $(FRONTEND_IMAGE) --name $(CLUSTER_NAME)
	@echo "Restarting cruisekube controller deployment..."
	@kubectl rollout restart deployment cruisekube-controller -n $(NAMESPACE)
	@kubectl rollout restart deployment cruisekube-webhook -n $(NAMESPACE)
	@kubectl rollout restart deployment cruisekube-frontend -n $(NAMESPACE)
	@echo "Images reloaded and deployment restarted."

ick: install-ck

install-ck: ## Update helm chart
	@printf 'apiVersion: v1\nkind: Namespace\nmetadata:\n  name: %s\n' "$(NAMESPACE)" | kubectl apply -f -
	@kubectl apply -f ./manifest/global/controller-admin-secret.yaml
	@cd ../../charts/cruisekube && helm dependency build
	@helm upgrade --wait --timeout $(TIMEOUT) --install --create-namespace cruisekube ../../charts/cruisekube -n $(NAMESPACE) -f ./manifest/global/values-cruisekube.yaml

uick: uninstall-ck
uninstall-ck: ## Delete cruisekube
	@echo "Deleting cruisekube..."
	
	-@helm delete cruisekube -n $(NAMESPACE)
