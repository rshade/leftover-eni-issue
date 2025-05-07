SHELL := /bin/bash
NAME          := aws-eni-cleanup
VERSION       := 0.1.0
PROVIDER      := pulumi-resource-${NAME}
PROVIDER_PATH := github.com/organization/aws-eni-cleanup-provider

WORKING_DIR     := $(shell pwd)
SCHEMA_PATH     := ${WORKING_DIR}/schema.json
PROVIDER_OUTPUT := ${WORKING_DIR}/bin/${PROVIDER}
SDK_PATH        := ${WORKING_DIR}/sdk

.PHONY: provider build install clean gen_schema gen_sdk lint format test

default: install

provider:
	(cd provider && go build -o $(PROVIDER_OUTPUT) -ldflags "-X ${PROVIDER_PATH}/pkg/schema.ProviderVersion=${VERSION}" ${PROVIDER_PATH}/cmd)

build: provider

install: build
	cp $(PROVIDER_OUTPUT) ${GOPATH}/bin

clean:
	rm -rf ${WORKING_DIR}/bin
	rm -rf ${WORKING_DIR}/sdk
	rm -rf ${SCHEMA_PATH}

gen_schema: provider
	${WORKING_DIR}/bin/${PROVIDER} schema --out=${SCHEMA_PATH}

gen_sdk: gen_schema
	cd ${WORKING_DIR} && \
	rm -rf sdk && \
	pulumi package gen-sdk --schema=${SCHEMA_PATH} --out-dir=${SDK_PATH}

lint:
	golangci-lint run

format:
	gofmt -w .

test:
	go test -v ./...

.PHONY: codegen
codegen: gen_sdk
	cd sdk && go mod tidy