# Project: DevOpsConf 2026 — Kubernetes Operator with LLM

## What this is

Presentation + live demo for DevOpsConf 2026. Topic: developing production Kubernetes operators with LLM using the SPEC → PLAN → CODE → TEST → REVIEW → LEARN cycle.

The operator (Project Operator) is the running example throughout the talk.

## Key files

- `project.md` — original project idea (user-facing description)
- `devopsconf_2026_ai_operator_presentation_rewrite.md` — full presentation text (slides + speaker notes)
- `SPEC.md` — operator specification (model, invariants, edge cases, acceptance criteria)
- `PLAN.md` — implementation plan (18 tasks with PRE/POST conditions)

## Commands

```bash
# Go / operator
make generate
make manifests
make build
make test
make install        # install CRDs into cluster
make run            # run operator locally
go vet ./...
golangci-lint run

# Kind cluster
export PATH="$PATH:/c/Users/azali/go/bin"
kind create cluster --name devops26
kind load docker-image <image> --name devops26

# cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
```

## Known Gotchas

- Don't use `r.Get()` after `r.Update()` without re-fetch
- `meta.SetStatusCondition()` needs `observedGeneration`
- `Status().Update()` must be separated from spec update
- Webhook must be fast — read `project.status.usedQuotas`, never list pods
- Deleting one namespace must not break full project reconciliation
- Lowering project quota below current utilization is allowed; only new violations should be blocked
- kind binary is at `/c/Users/azali/go/bin/kind.exe` — needs PATH export
- Kind cluster name: `devops26`, kubectl context: `kind-devops26`
