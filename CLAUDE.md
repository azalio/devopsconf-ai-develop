# Project: Project Operator

## What this is

Kubernetes operator for multi-tenant project management.
Project groups namespaces into a logical unit with shared quotas and RBAC.

## Key files

- `SPEC.md` — operator specification (model, invariants, edge cases, acceptance criteria)
- `PLAN.md` — implementation plan (18 tasks with PRE/POST conditions)
- `project-operator/` — Go project (kubebuilder v4.5.2 scaffold)

## Project structure

```
project-operator/
├── api/v1alpha1/           # CRD type definitions
│   ├── project_types.go
│   ├── projectnamespace_types.go
│   ├── projectrole_types.go
│   └── projectrolebinding_types.go
├── internal/controller/    # Reconcilers
│   ├── project_controller.go
│   ├── projectnamespace_controller.go
│   ├── projectrole_controller.go
│   ├── projectrolebinding_controller.go
│   └── quota_controller.go
├── cmd/main.go
├── config/                 # Kustomize manifests (CRD, RBAC, webhook, etc.)
├── Makefile
├── Dockerfile
└── PROJECT
```

## Conventions

- API group: `platform.example.io`
- API version: `v1alpha1`
- CRDs: Project (cluster-scoped), ProjectNamespace, ProjectRole, ProjectRoleBinding (namespaced)
- Module: `github.com/example/project-operator`
- Label for managed resources: `platform.example.io/managed-by: project-operator`
- Annotation for namespace ownership: `platform.example.io/project-name: <project-name>`
- Finalizer: `platform.example.io/project-protection`

## Commands

```bash
make generate       # regenerate deepcopy, manifests
make manifests      # regenerate CRD/RBAC/webhook manifests
make build          # compile
make test           # run envtest suite
make install        # install CRDs into cluster
make run            # run operator locally
go vet ./...
golangci-lint run
```

## Testing

Tests use controller-runtime **envtest** (real etcd + kube-apiserver, no full cluster).

```bash
make test
# or directly:
go test ./internal/controller/ -v -count=1
```

- CRD paths: `config/crd/bases/` (relative, resolved by suite_test.go)
- Test framework: Ginkgo/Gomega (BDD-style)

## Known Gotchas

- Don't use `r.Get()` after `r.Update()` without re-fetch — stale cache
- `meta.SetStatusCondition()` needs `observedGeneration`
- `Status().Update()` must be separated from spec update
- Webhook must be fast — read `project.status.usedQuotas`, never list pods
- Deleting one namespace must not break full project reconciliation
- Lowering project quota below current utilization is allowed; only new violations should be blocked
- Cross-namespace ownerReference doesn't work for GC — use labels + finalizers
- ProjectRole/ProjectRoleBinding names must have prefix `stackland-projects-`
