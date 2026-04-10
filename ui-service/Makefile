APP_NAME=taksa-ui
export DOCKER_REGISTRY ?= taksa-registry.local
export DOCKER_LABEL ?= dev
DOCKER_IMAGE=$(DOCKER_REGISTRY)/$(APP_NAME):$(DOCKER_LABEL)

.PHONY: help build up down restart clean logs

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the Docker image
	docker-compose build

up: ## Start the container in detached mode
	docker-compose up -d

down: ## Stop and remove the container
	docker-compose down

restart: down up ## Restart the container

clean: ## Remove unused Docker resources and the image
	docker system prune -f
	docker rmi $(DOCKER_IMAGE) || true

logs: ## View container logs
	docker-compose logs -f
