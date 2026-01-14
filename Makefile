.PHONY: build test lint clean install run-local build-release

# Binary name and paths
BINARY_NAME=gitops-kustomzchk
BIN_DIR=bin
MAIN_PATH=./src/cmd/gitops-kustomzchk

# Build variables
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

build:
	@mkdir -p ${BIN_DIR}
	go build ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME} ${MAIN_PATH}

install: build
	cp ${BIN_DIR}/${BINARY_NAME} $(GOPATH)/bin/

test:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

test-coverage: test
	go tool cover -html=coverage.txt -o coverage.html

lint:
	golangci-lint run ./...

clean:
	rm -rf ${BIN_DIR}
	rm -f coverage.txt coverage.html
	rm -rf dist/

# Build release binaries for multiple platforms
build-release:
	@echo "Building release binaries..."
	@mkdir -p dist
	@echo "Building Linux AMD64..."
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-amd64 ${MAIN_PATH}
	# @echo "Building Linux ARM64..."
	# GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-arm64 ${MAIN_PATH}
	# @echo "Building macOS AMD64..."
	# GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-amd64 ${MAIN_PATH}
	@echo "Building macOS ARM64..."
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-arm64 ${MAIN_PATH}
	@echo "Generating checksums..."
	cd dist && sha256sum ${BINARY_NAME}-* > checksums.txt
	@echo "✅ Release binaries built successfully!"
	@ls -lh dist/

run: build
	${BIN_DIR}/${BINARY_NAME}

# Run in local mode with test data
# Usage: make run-local (default DEBUG=0, LOGLEVEL=info)
# Usage: DEBUG=1 make run-local (enable debug logging)
# Usage: LOGLEVEL=debug make run-local (set log level)
# Usage: DEBUG=1 LOGLEVEL=trace make run-local (both debug and trace logging)
run-local-old: build
	DEBUG=1 ${BIN_DIR}/${BINARY_NAME} --run-mode local \
		--service my-app \
		--environments stg,prod \
		--fail-on-overlay-not-found=false \
		--lc-before-manifests-path test/local_pre_v0_4/before/services \
		--lc-after-manifests-path test/local_pre_v0_4/after/services \
		--policies-path test/local_pre_v0_4/policies \
		--templates-path test/local_pre_v0_4/templates \
		--output-dir test/local_pre_v0_4/output \
		--enable-export-report true \
		--enable-export-performance-report true \
		--debug true;

run-local: build
	DEBUG=1 ${BIN_DIR}/${BINARY_NAME} --run-mode local \
		--fail-on-overlay-not-found=false \
		--lc-before-kustomize-build-path "test/local/before/services/my-app/clusters/[CLUSTER]/[ENV]" \
		--lc-after-kustomize-build-path "test/local/after/services/my-app/clusters/[CLUSTER]/[ENV]" \
		--kustomize-build-values "CLUSTER=alpha,beta;ENV=stg,prod" \
		--policies-path test/local/policies \
		--templates-path test/local/templates \
		--output-dir test/local/output \
		--enable-export-report true \
		--enable-export-performance-report true \
		--debug true;

# Run in local mode with dynamic paths (new v0.5+ feature)
run-local-dynamic: build
	DEBUG=1 ${BIN_DIR}/${BINARY_NAME} --run-mode local \
		--fail-on-overlay-not-found=false \
		--lc-before-kustomize-build-path "test/ut_local_dynamic/before/services/my-app/clusters/[CLUSTER]/[ENV]" \
		--lc-after-kustomize-build-path "test/ut_local_dynamic/after/services/my-app/clusters/[CLUSTER]/[ENV]" \
		--kustomize-build-values "CLUSTER=alpha,beta;ENV=stg,prod" \
		--policies-path test/ut_local_dynamic/policies \
		--templates-path test/ut_local_dynamic/templates \
		--output-dir test/output \
		--enable-export-report true \
		--enable-export-performance-report true \
		--debug true;
	@echo ""
	@echo "📄 Reports generated:"
	@ls -lh test/output/*.md

run-github: build
	# if [[ "${GH_TOKEN}" == "" ]]; then
	# 	echo "GH_TOKEN is not set"
	# 	exit 1
	# fi

	${BIN_DIR}/${BINARY_NAME} --run-mode github \
		--environments stg,prod \
		--service app-bootstrap-touya \
		--gh-repo moneyforward/tmp-clone_k8s-service-manifests \
		--gh-pr-number 1 \
		--enable-export-report true \
		--enable-export-performance-report true \
		--output-dir test/output \
		--manifests-path services \
		--templates-path test/local/templates \
		--policies-path test/local/policies \
		--debug true;
	@echo ""
	@echo "📄 Reports generated:"
	@ls -lh test/output/*.md

# OPA policy tests
test-policies:
	opa test sample/policies/*.opa

# System Integration Test (SIT) - Local mode
sit-test-local:
	@echo "Running System Integration Test (Local Mode)..."
	@test/ut_local/run_test.sh

# Run all SIT tests, GitHub mode to be added
sit-test: test-sit-local

# Format code
fmt:
	go fmt ./...
	gofumpt -w .

# Check for security issues
security:
	gosec ./...

# Run all checks (lint + test + security)
check: lint test security test-policies

help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  install        - Install the binary to GOPATH/bin"
	@echo "  test           - Run tests with coverage"
	@echo "  test-coverage   - Generate HTML coverage report"
	@echo "  lint           - Run linter"
	@echo "  clean          - Clean build artifacts"
	@echo "  run-local      - Run in local mode with test data"
	@echo "                   Usage: make run-local (DEBUG=0, LOGLEVEL=info)"
	@echo "                   Usage: DEBUG=1 make run-local (debug mode)"
	@echo "                   Usage: LOGLEVEL=debug make run-local (set log level)"
	@echo "                   Usage: DEBUG=1 LOGLEVEL=trace make run-local (both)"
	@echo "  test-policies  - Test OPA policies"
	@echo "  fmt            - Format code"
	@echo "  security       - Check for security issues"
	@echo "  check          - Run all checks"
	@echo "  build-release  - Build release binaries for all platforms"


