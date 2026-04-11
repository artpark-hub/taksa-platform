SHELL := /bin/bash

export DOCKER_REGISTRY ?= taksa-registry.local
export DOCKER_LABEL ?= dev

SERVICES := device-management user-management nats-data-collector ui-service

.DEFAULT_GOAL := help

.PHONY: help build build-dm build-user build-nats-collector build-ui clean

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: build-dm build-user build-nats-collector build-ui ## Build all platform service Docker images

build-dm: ## Build device-management Docker image
	@echo "Building device-management..."
	$(MAKE) -C device-management build
	docker build -t $(DOCKER_REGISTRY)/taksa-device-management:$(DOCKER_LABEL) device-management/

build-user: ## Build user-management Docker image
	@echo "Building user-management..."
	$(MAKE) -C user-management build
	docker build -t $(DOCKER_REGISTRY)/taksa-user-management:$(DOCKER_LABEL) user-management/

build-nats-collector: ## Build nats-data-collector Docker image
	@echo "Building nats-data-collector..."
	$(MAKE) -C nats-data-collector build
	docker build -t $(DOCKER_REGISTRY)/taksa-nats-data-collector:$(DOCKER_LABEL) nats-data-collector/

build-ui: ## Build ui-service Docker image
	@echo "Building ui-service..."
	docker build -t $(DOCKER_REGISTRY)/taksa-ui-service:$(DOCKER_LABEL) ui-service/

clean: ## Clean build artifacts in all services
	@for svc in $(SERVICES); do \
		if [ -f $$svc/Makefile ]; then \
			$(MAKE) -C $$svc clean 2>/dev/null || true; \
		fi \
	done
