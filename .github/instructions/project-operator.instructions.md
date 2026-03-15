---
applyTo: "project-operator/**"
---

# Project Operator — Review Instructions

## Architecture

This is a Kubernetes operator built with kubebuilder v4.5.2.
API group: `platform.example.io`, version: `v1alpha1`.
All CRDs are **cluster-scoped**.

## Incremental implementation (PLAN.md)

The operator is implemented in **18 incremental tasks** defined in `PLAN.md`.
Each PR corresponds to one task. Tasks build on each other:

- **Task 4** (basic scaffold): adds finalizer + `status.phase`, does NOT remove the finalizer on deletion — this is intentional.
- **Task 5+**: adds namespace management, cleanup logic, finalizer removal.
- **Task 8+**: adds admission webhooks.

**When reviewing, check `PLAN.md` to understand which task the PR implements.**
Do not flag "missing" functionality that belongs to a later task.
For example, if a PR adds a finalizer but does not remove it during deletion,
check whether finalizer removal is scoped to a future task before commenting.

## TDD workflow

Starting from Task 4, each task follows TDD in two separate sessions:
1. First PR: write failing tests based on POST conditions.
2. Second PR: implement code that passes those tests.

Tests define the contract. If all tests pass, the task is complete for its scope.

## Key conventions

- Finalizer: `platform.example.io/project-protection`
- Managed-by label: `platform.example.io/managed-by: project-operator`
- Project ownership annotation: `platform.example.io/project-name: <name>`
- Module: `github.com/example/project-operator`
- Test framework: Ginkgo/Gomega with envtest (real etcd + kube-apiserver)

## Common patterns

- `client.IgnoreNotFound(err)` for idempotent reconcile on deleted objects.
- `controllerutil.ContainsFinalizer` / `AddFinalizer` / `RemoveFinalizer` for finalizer management.
- Status updates use `r.Status().Update()`, separated from spec updates.
- After `r.Update()` the in-memory object has the updated ResourceVersion; a re-fetch is only needed if a conflict is expected.

## What NOT to flag

- Missing finalizer removal in early tasks (Task 4) — by design per PLAN.md.
- Stub reconcilers for ProjectRole / ProjectAccessBinding — implemented in later tasks.
- Missing webhook validation — implemented in Tasks 8-12.
- Missing default project creation — implemented in Task 13.
