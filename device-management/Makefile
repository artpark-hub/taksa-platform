GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)

ifeq ($(GOHOSTOS), windows)
	#the `find.exe` is different from `find` in bash/shell.
	#to see https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/find.
	#changed to use git-bash.exe to run find cli or other cli friendly, caused of every developer has a Git.
	#Git_Bash= $(subst cmd\,bin\bash.exe,$(dir $(shell where git)))
	Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	INTERNAL_PROTO_FILES=$(shell $(Git_Bash) -c "find internal -name *.proto")
	API_PROTO_FILES=$(shell $(Git_Bash) -c "find api -name *.proto")
else
	INTERNAL_PROTO_FILES=$(shell find internal -name *.proto)
	API_PROTO_FILES=$(shell find api -name *.proto)
endif

.PHONY: init
# init env
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest

.PHONY: config
# generate internal proto
config:
	protoc --proto_path=./internal \
	       --proto_path=./third_party \
 	       --go_out=paths=source_relative:./internal \
	       $(INTERNAL_PROTO_FILES)

.PHONY: api
# generate api proto
api:
	protoc --proto_path=./api \
	       --proto_path=./third_party \
 	       --go_out=paths=source_relative:./api \
 	       --go-http_out=paths=source_relative:./api \
 	       --go-grpc_out=paths=source_relative:./api \
	       --openapi_out=fq_schema_naming=true,default_response=false:. \
	       $(API_PROTO_FILES)

.PHONY: build
# build
build: api config generate wire
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/device-management ./cmd/device-management

.PHONY: wire
# generate wire
wire:
	cd ./cmd/device-management && wire

.PHONY: generate
# generate
generate:
	go generate ./...
	go mod tidy

.PHONY: all
# generate all and build
all: api config generate wire build

.PHONY: clean
# clean build artifacts (generated files and binaries)
clean:
	rm -rf bin/
	rm -f openapi.yaml
	rm -f cmd/device-management/wire_gen.go
	go clean

.PHONY: help
# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z_-]+:.*##/ { \
		split($$0, a, "##"); \
		target = substr(a[1], 1, index(a[1], ":")-1); \
		help = substr(a[2], 2); \
		printf "\033[36m%-22s\033[0m %s\n", target, help; \
	} /^[a-zA-Z_-]+:/ && !/##/ { \
		if (match(lastLine, /^# (.*)/)) { \
			target = substr($$1, 1, index($$1, ":")-1); \
			help = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", target, help; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.PHONY: docker-init docker-up docker-down docker-start docker-stop docker-logs docker-clean docker-fresh reset

# Docker deployment targets (aligned with taksa-deployments)
# Note: docker-fresh builds image inside container (multi-stage), so no need to run 'make all' first

docker-fresh: docker-init docker-up ## Build Docker image and start container

docker-init: ## Initialize Docker environment (create data directories and .env file)
	test -f .env || cp .env.example .env
	@DATA_DIR=$$(grep '^TAKSA_DM_DATA_DIR=' .env | cut -d= -f2); \
	mkdir -p "$$DATA_DIR"; \
	chmod 777 "$$DATA_DIR"; \
	echo "Docker environment initialized at $$DATA_DIR. Edit .env if needed."

docker-up: ## Create and start the container in detached mode
	docker-compose up -d

docker-down: ## Stop and remove the container and volumes
	docker-compose down

docker-start: ## Start existing container
	docker-compose start

docker-stop: ## Stop running container
	docker-compose stop

docker-logs: ## Follow container logs
	docker-compose logs -f

docker-clean: ## Remove container and delete data directories
	docker-compose down
	rm -rf data/
	rm -f .env

reset: docker-clean clean ## Full reset to clean git state
	@echo "Reset complete. Repository cleaned."

.DEFAULT_GOAL := help
