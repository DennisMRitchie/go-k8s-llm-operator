# ── Stage 1: Build ─────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /workspace

# Cache dependencies first for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a statically linked binary.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o bin/llm-operator ./main.go

# ── Stage 2: Runtime ────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/bin/llm-operator .

USER 65532:65532

ENTRYPOINT ["/llm-operator"]
