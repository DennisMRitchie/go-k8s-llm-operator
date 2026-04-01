# ── Variables ───────────────────────────────────────────────────────────────
BINARY        := bin/llm-operator
IMAGE         := ghcr.io/dennisritchie/go-k8s-llm-operator
TAG           := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
KUBECONFIG    ?= ~/.kube/config
KIND_CLUSTER  := llm-operator-dev

.PHONY: all build test lint vet fmt tidy \
        docker-build docker-push \
        manifests install uninstall \
        up down cluster-create cluster-delete \
        deploy undeploy \
        sample-apply sample-delete \
        help

all: build ## Default target: build binary

# ── Go targets ──────────────────────────────────────────────────────────────

build: ## Compile the operator binary
	@echo "→ Building $(BINARY)..."
	CGO_ENABLED=0 go build -ldflags="-w -s" -o $(BINARY) ./main.go

tidy: ## Tidy and vendor dependencies
	go mod tidy

fmt: ## Run gofmt
	gofmt -w .

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (install separately)
	golangci-lint run ./...

test: ## Run unit tests
	go test ./... -v -count=1 -timeout 60s

# ── Docker ──────────────────────────────────────────────────────────────────

docker-build: ## Build the operator Docker image
	docker build -t $(IMAGE):$(TAG) .
	docker tag $(IMAGE):$(TAG) $(IMAGE):latest

docker-push: ## Push the Docker image to the registry
	docker push $(IMAGE):$(TAG)
	docker push $(IMAGE):latest

# ── Kind cluster ────────────────────────────────────────────────────────────

cluster-create: ## Create a local Kind cluster
	kind create cluster --name $(KIND_CLUSTER)
	kubectl cluster-info --context kind-$(KIND_CLUSTER)

cluster-delete: ## Delete the local Kind cluster
	kind delete cluster --name $(KIND_CLUSTER)

# ── CRD & RBAC ──────────────────────────────────────────────────────────────

manifests: ## Regenerate CRD manifests (requires controller-gen)
	controller-gen crd:crdVersions=v1 paths="./api/..." \
	  output:crd:artifacts:config=config/crd
	controller-gen rbac:roleName=llm-operator-manager-role paths="./..." \
	  output:rbac:artifacts:config=config/rbac

install: ## Apply CRD to current cluster
	kubectl apply -f config/crd/llmdeployments.yaml
	kubectl apply -f config/rbac/role.yaml

uninstall: ## Remove CRD from current cluster
	kubectl delete -f config/crd/llmdeployments.yaml --ignore-not-found
	kubectl delete -f config/rbac/role.yaml --ignore-not-found

# ── Docker Compose ──────────────────────────────────────────────────────────

up: ## Start full local stack (operator + Prometheus + Grafana)
	docker compose up --build -d

down: ## Stop local stack
	docker compose down

# ── Sample CR ───────────────────────────────────────────────────────────────

sample-apply: ## Apply a sample LLMDeployment CR
	kubectl apply -f - <<EOF
	apiVersion: ai.dennisritchie.dev/v1
	kind: LLMDeployment
	metadata:
	  name: qwen3-prod
	  namespace: default
	spec:
	  modelName: qwen3
	  image: ollama/ollama:latest
	  replicas: 2
	  maxReplicas: 10
	  targetQPS: 1200
	  targetLatencyMs: 500
	  scalingMetric: QPS
	  gpuRequired: false
	  guardrails:
	    enabled: true
	    blockPromptInjection: true
	    maxTokensPerRequest: 4096
	  prometheus:
	    enabled: true
	    port: 9090
	    path: /metrics
	EOF

sample-delete: ## Delete the sample LLMDeployment CR
	kubectl delete llmd qwen3-prod --ignore-not-found

# ── Help ─────────────────────────────────────────────────────────────────────

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
