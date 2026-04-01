# 🚀 go-k8s-llm-operator

**Kubernetes Operator for LLM Workloads — Built in Go**

Automated management of LLM infrastructure in Kubernetes: autoscaling, HPA, rolling updates, and full observability driven by real-time metrics.

![Go](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat&logo=go)
![Kubernetes](https://img.shields.io/badge/Kubernetes-Operator-326CE5?style=flat&logo=kubernetes)
![controller-runtime](https://img.shields.io/badge/controller--runtime-v0.19-blue?style=flat)
![Prometheus](https://img.shields.io/badge/Prometheus-integrated-E6522C?style=flat&logo=prometheus)
![LLM](https://img.shields.io/badge/LLM-Ready-10A37F?style=flat)
![CI](https://github.com/DennisMRitchie/go-k8s-llm-operator/actions/workflows/go.yml/badge.svg) ![License](https://img.shields.io/badge/License-Apache_2.0-yellow?style=flat)

---

## ✨ Key Features

| Feature | Details |
|---|---|
| ⚙️ **Kubernetes Operator** | Full controller-runtime pattern with reconcile loop & finalizers |
| 📈 **LLM Autoscaling** | Scale on QPS, latency (p99), GPU, or CPU — configurable per CR |
| 🔄 **Managed Resources** | Automatically creates & owns `Deployment`, `Service`, `HPA` |
| 📊 **Prometheus Metrics** | `llmoperator_*` metrics for reconcile duration, replicas, QPS, latency, scale events |
| 🛡️ **Guardrails Sidecar** | Optional injection of a prompt-injection-blocking sidecar |
| 🔗 **Ecosystem Ready** | Connects to `go-llm-gateway`, `go-rag-llm-orchestrator`, `go-llm-cache` |
| 🔒 **Production-grade** | Leader election, health probes, finalizers, owner references, status conditions |

---

## 🛠 Tech Stack

- **Go 1.23** + `controller-runtime v0.19`
- **Kubernetes 1.30+** (tested on Kind)
- **Prometheus** `client_golang v1.20`
- **Docker** + **Kind** (local dev)
- **Grafana** (bundled in docker-compose)

---

## 📦 Project Structure

```
go-k8s-llm-operator/
├── api/v1/
│   └── llmdeployment_types.go   # CRD schema + DeepCopy
├── internal/
│   ├── controller/              # Main reconcile loop
│   ├── reconciler/              # Deployment / Service / HPA builders
│   └── metrics/                 # Prometheus collectors
├── config/
│   ├── crd/llmdeployments.yaml  # CRD manifest
│   └── rbac/role.yaml           # ClusterRole + ServiceAccount
├── hack/
│   ├── prometheus.yml           # Scrape config for local Prometheus
│   └── boilerplate.go.txt
├── main.go
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

---

## 🚀 Quick Start

### Prerequisites

- Go 1.23+
- Docker
- `kubectl` + `kind`
- (optional) `controller-gen` for re-generating CRDs

```bash
# Clone
git clone https://github.com/DennisMRitchie/go-k8s-llm-operator.git
cd go-k8s-llm-operator

# Create a local Kind cluster
make cluster-create

# Apply CRD + RBAC
make install

# Start operator + Prometheus + Grafana
make up

# Apply a sample LLMDeployment
make sample-apply

# Watch the operator manage your LLM workload
kubectl get llmd -w
```

Open **Grafana** at `http://localhost:3000` (admin / llmoperator) to explore `llmoperator_*` metrics.

---

## 📋 LLMDeployment CRD — Example

```yaml
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
  scalingMetric: QPS    # QPS | Latency | GPU | CPU
  gpuRequired: false
  guardrails:
    enabled: true
    blockPromptInjection: true
    maxTokensPerRequest: 4096
  prometheus:
    enabled: true
    port: 9090
    path: /metrics
```

```bash
# Check status columns
kubectl get llmd
# NAME         MODEL   REPLICAS   QPS    PHASE     AGE
# qwen3-prod   qwen3   2          1105   Running   3m
```

---

## 📊 Prometheus Metrics

| Metric | Type | Description |
|---|---|---|
| `llmoperator_controller_reconcile_total` | Counter | Reconcile calls by result |
| `llmoperator_controller_reconcile_duration_seconds` | Histogram | Reconcile loop duration |
| `llmoperator_deployment_replicas` | Gauge | Current replica count per LLMDeployment |
| `llmoperator_deployment_current_qps` | Gauge | Current QPS per LLMDeployment |
| `llmoperator_deployment_latency_p99_ms` | Gauge | p99 latency per LLMDeployment |
| `llmoperator_autoscaler_scale_events_total` | Counter | Scale events by direction (up/down) |

---

## 🔧 Makefile Targets

```
make build          Build the operator binary
make test           Run all unit tests
make lint           Run golangci-lint
make docker-build   Build Docker image
make manifests      Re-generate CRD + RBAC manifests
make install        Apply CRD + RBAC to cluster
make up             Start full local stack (docker compose)
make down           Stop local stack
make sample-apply   Deploy a sample LLMDeployment CR
make help           Show all targets
```

---

## 🔗 Related Projects

This operator is part of a cohesive **Go LLM Platform**:

| Repository | Role |
|---|---|
| [`go-llm-gateway`](https://github.com/DennisMRitchie/go-llm-gateway) | API gateway + load balancing |
| [`go-rag-llm-orchestrator`](https://github.com/DennisMRitchie/go-rag-llm-orchestrator) | RAG pipeline orchestration |
| [`go-llm-cache`](https://github.com/DennisMRitchie/go-llm-cache) | Semantic response caching |
| **`go-k8s-llm-operator`** | Kubernetes operator for all of the above |

---

## 🎯 Why this project matters (for recruiters)

- Directly matches: *"Developed custom Kubernetes operators for dynamic LLM workload orchestration"*
- Demonstrates: Kubernetes internals, CRD design, Go concurrency, Prometheus observability, and AI-infra thinking — all in one repo
- Production patterns: finalizers, leader election, owner references, status conditions, health probes

---

Built with ❤️ by **Konstantin Lychkov**  
Senior Go Developer | Go + LLM/NLP + Kubernetes  
Warsaw, Poland • Open to Remote Worldwide

⭐ Star the repo — help the community run LLM at scale!
