#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────
# push-to-github.sh
# Run this script INSIDE the go-k8s-llm-operator folder.
# Prerequisites: git, gh (GitHub CLI) or a PAT token set.
# ─────────────────────────────────────────────────────────

set -euo pipefail

REPO_NAME="go-k8s-llm-operator"
GITHUB_USER="DennisMRitchie"   # ← change if needed
COMMIT_MSG="feat: production Kubernetes Operator for LLM autoscaling and management 🚀"

echo "==> Initialising git repo..."
git init
git add .
git commit -m "$COMMIT_MSG"
git branch -M main

echo "==> Creating GitHub repo (requires 'gh' CLI logged in)..."
gh repo create "$GITHUB_USER/$REPO_NAME" \
  --public \
  --description "Production-ready Kubernetes Operator (controller-runtime) for automated LLM workload management: autoscaling, HPA, guardrails sidecar, Prometheus metrics." \
  --source=. \
  --remote=origin \
  --push

echo ""
echo "✅  Done!  https://github.com/$GITHUB_USER/$REPO_NAME"
