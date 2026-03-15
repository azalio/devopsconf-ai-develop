# Project: DevOpsConf 2026 — Kubernetes Operator with LLM

## What this is

Presentation + live demo for DevOpsConf 2026. Topic: developing production Kubernetes operators with LLM using the SPEC → PLAN → CODE → TEST → REVIEW → LEARN cycle.

The operator (Project Operator) is the running example throughout the talk.

## Key files

- `project.md` — original project idea (user-facing description)
- `devopsconf_2026_ai_operator_presentation_rewrite.md` — full presentation text (slides + speaker notes)
- `SPEC.md` — operator specification (model, invariants, edge cases, acceptance criteria)
- `PLAN.md` — implementation plan (18 tasks with PRE/POST conditions)
- `project-operator/` — Go project (kubebuilder v4.5.2 scaffold)

## Project structure

```
project-operator/
├── api/v1alpha1/           # CRD type definitions
│   ├── project_types.go
│   ├── projectrole_types.go
│   ├── projectaccessbinding_types.go
│   └── groupversion_info.go
├── internal/controller/    # Reconcilers
│   ├── project_controller.go
│   ├── projectrole_controller.go
│   └── projectaccessbinding_controller.go
├── cmd/main.go             # Entrypoint
├── config/                 # Kustomize manifests (CRD, RBAC, webhook, etc.)
├── Makefile
├── Dockerfile
└── PROJECT
```

## Conventions

- API group: `platform.example.io`
- API version: `v1alpha1`
- All CRDs are **cluster-scoped**
- Module: `github.com/example/project-operator`
- Label for managed resources: `platform.example.io/managed-by: project-operator`
- Annotation for namespace ownership: `platform.example.io/project-name: <project-name>`
- Finalizer: `platform.example.io/project-protection`

## Commands

```bash
# All Go/operator commands run via WSL Ubuntu
# Prefix: wsl -d Ubuntu -- bash -c 'export PATH="/usr/local/go/bin:$PATH" && cd "/mnt/e/code/devops26-ai-develop/.claude/worktrees/hardcore-lederberg/project-operator" && <command>'

# Go / operator (run in WSL)
make generate
make manifests
make build
make test
make install        # install CRDs into cluster
make run            # run operator locally
go vet ./...
golangci-lint run

# Kind cluster (run on Windows — kind is at /c/Users/azali/go/bin/kind.exe)
export PATH="$PATH:/c/Users/azali/go/bin"
kind create cluster --name devops26
kind load docker-image <image> --name devops26

# kubectl (run on Windows)
kubectl --context kind-devops26 <command>

# cert-manager (already installed in devops26 cluster)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
```

## Known Gotchas

- kubebuilder has no Windows binary — all Go/make commands must run via WSL Ubuntu
- WSL Go path: `/usr/local/go/bin`
- WSL project path: `/mnt/e/code/devops26-ai-develop/.claude/worktrees/hardcore-lederberg/project-operator`
- Don't use `r.Get()` after `r.Update()` without re-fetch
- `meta.SetStatusCondition()` needs `observedGeneration`
- `Status().Update()` must be separated from spec update
- Webhook must be fast — read `project.status.usedQuotas`, never list pods
- Deleting one namespace must not break full project reconciliation
- Lowering project quota below current utilization is allowed; only new violations should be blocked
- kind binary is at `/c/Users/azali/go/bin/kind.exe` — needs PATH export
- Kind cluster name: `devops26`, kubectl context: `kind-devops26`
