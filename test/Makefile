# Makefile for Helm chart Ginkgo tests

.PHONY: all deps test test-verbose test-focus clean kind-create kind-delete lint

# Variables
GINKGO := go run github.com/onsi/ginkgo/v2/ginkgo
KIND_CLUSTER_NAME ?= postgres-test
TEST_NAMESPACE ?= postgres-test
CHART_PATH ?= ../chart
SKIP_CLEANUP ?= false

# Default target
all: deps test

# Install dependencies
deps:
	@echo "Installing test dependencies..."
	go mod download
	go install github.com/onsi/ginkgo/v2/ginkgo@latest

# Run all tests
test:
	@echo "Running Helm chart tests..."
	$(GINKGO) -v --fail-fast --trace \
		--timeout=30m \
		--output-dir=./reports \
		--json-report=report.json \
		--junit-report=report.xml

# Run tests with verbose output
test-verbose:
	@echo "Running Helm chart tests (verbose)..."
	$(GINKGO) -vv --fail-fast --trace \
		--timeout=30m \
		--output-dir=./reports \
		--json-report=report.json \
		--junit-report=report.xml

# Run specific test (use FOCUS env var)
test-focus:
	@echo "Running focused tests: $(FOCUS)"
	$(GINKGO) -v --fail-fast --trace \
		--focus="$(FOCUS)" \
		--timeout=30m \
		--output-dir=./reports \
		--json-report=report.json \
		--junit-report=report.xml

# Run tests in watch mode for development
test-watch:
	@echo "Running tests in watch mode..."
	$(GINKGO) watch -v

# Create kind cluster for local testing
kind-create:
	@echo "Creating kind cluster..."
	kind create cluster --name $(KIND_CLUSTER_NAME) --config kind-config.yaml || true
	@echo "Waiting for cluster to be ready..."
	kubectl wait --for=condition=Ready nodes --all --timeout=60s
	@echo "Kind cluster ready!"

# Delete kind cluster
kind-delete:
	@echo "Deleting kind cluster..."
	kind delete cluster --name $(KIND_CLUSTER_NAME)

# Run tests in kind cluster
test-kind: kind-create
	@echo "Running tests in kind cluster..."
	$(MAKE) test SKIP_CLEANUP=false
	@if [ "$(SKIP_CLEANUP)" = "false" ]; then \
		$(MAKE) kind-delete; \
	fi

# Lint Go code
lint:
	@echo "Linting Go code..."
	golangci-lint run ./...

# Clean test artifacts
clean:
	@echo "Cleaning test artifacts..."
	rm -rf reports/
	rm -f *.out *.log

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Install Helm chart for manual testing
helm-install:
	helm install test-release $(CHART_PATH) \
		--namespace $(TEST_NAMESPACE) \
		--create-namespace \
		--set database.password=testpass123 \
		--wait

# Uninstall Helm chart
helm-uninstall:
	helm uninstall test-release \
		--namespace $(TEST_NAMESPACE) || true
	kubectl delete namespace $(TEST_NAMESPACE) || true

# Run a single test suite
test-suite:
	@if [ -z "$(SUITE)" ]; then \
		echo "Please specify SUITE (e.g., make test-suite SUITE='New Installation')"; \
		exit 1; \
	fi
	$(GINKGO) -v --fail-fast --trace \
		--focus="$(SUITE)" \
		--timeout=30m

# Generate test report
report:
	@echo "Generating test report..."
	@if [ -f reports/report.json ]; then \
		cat reports/report.json | jq -r '.[] | select(.State == "failed") | .FullText' > reports/failures.txt; \
		echo "Failed tests written to reports/failures.txt"; \
	else \
		echo "No report.json found. Run tests first."; \
	fi

# Help
help:
	@echo "Available targets:"
	@echo "  make deps          - Install test dependencies"
	@echo "  make test          - Run all tests"
	@echo "  make test-verbose  - Run tests with verbose output"
	@echo "  make test-focus    - Run focused tests (FOCUS='pattern')"
	@echo "  make test-watch    - Run tests in watch mode"
	@echo "  make test-kind     - Run tests in kind cluster"
	@echo "  make test-coverage - Run tests with coverage"
	@echo "  make kind-create   - Create kind cluster"
	@echo "  make kind-delete   - Delete kind cluster"
	@echo "  make helm-install  - Install Helm chart for manual testing"
	@echo "  make helm-uninstall- Uninstall Helm chart"
	@echo "  make lint          - Lint Go code"
	@echo "  make clean         - Clean test artifacts"
	@echo "  make report        - Generate test report from results"
	@echo "  make help          - Show this help message"