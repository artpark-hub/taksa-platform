GOHOSTOS:=$(shell go env GOHOSTOS)
VERSION=$(shell git describe --tags --always)

ifeq ($(GOHOSTOS), windows)
	Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	INTERNAL_PROTO_FILES=$(shell $(Git_Bash) -c "find internal -name *.proto")
	API_PROTO_FILES=$(shell $(Git_Bash) -c "find api -name *.proto")
else
	INTERNAL_PROTO_FILES=$(shell find internal -name *.proto)
	API_PROTO_FILES=$(shell find api -name *.proto)
endif

.PHONY: init
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest

.PHONY: config
config:
	protoc --proto_path=./internal \
	       --proto_path=./third_party \
 	       --go_out=paths=source_relative:./internal \
	       $(INTERNAL_PROTO_FILES)

.PHONY: api
api:
	@if [ -z "$(API_PROTO_FILES)" ]; then \
		echo "no api proto files found, skipping api generate"; \
	else \
		protoc --proto_path=./api \
			   --proto_path=./third_party \
			   --go_out=paths=source_relative:./api \
			   --go-http_out=paths=source_relative:./api \
			   --go-grpc_out=paths=source_relative:./api \
			   --openapi_out=fq_schema_naming=true,default_response=false:. \
			   $(API_PROTO_FILES); \
	fi

.PHONY: wire
# generate wire
wire:
	cd ./cmd/nats-data-collector && wire

.PHONY: build
# build
build: api config generate wire
	mkdir -p bin/ && CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/nats-data-collector ./cmd/nats-data-collector

.PHONY: generate
generate:
	go generate ./...
	go mod tidy

.PHONY: all
all:
	make api
	make config
	make generate

help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
