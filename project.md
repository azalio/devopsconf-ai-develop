# Projects

`Project` объединяет несколько namespace'ов Kubernetes в одну логическую единицу.

Это удобно, когда одна команда работает не с одним namespace, а сразу с несколькими, например:

- `billing-dev`
- `billing-stage`
- `billing-prod`

Вместо того чтобы настраивать квоты и доступы в каждом namespace отдельно, можно управлять ими на уровне проекта.

## Что даёт Project

- объединяет namespace'ы одной команды или сервиса
- задаёт общие квоты на ресурсы
- позволяет выдавать доступ сразу на весь проект
- упрощает self-service для команд

## Основные ресурсы

В примерах ниже используются три ресурса:

- `Project` — описывает сам проект и его квоты
- `ProjectRole` — описывает роль внутри проекта
- `ProjectAccessBinding` — выдаёт доступ пользователю, группе или service account

## Быстрый старт

### 1. Создать проект

```yaml
apiVersion: platform.example.io/v1alpha1
kind: Project
metadata:
  name: billing
spec:
  displayName: "Billing"
  description: "Project for billing services"
  quotas:
    requests.cpu: "100"
    requests.memory: "200Gi"
    requests.storage: "10Ti"
```

Этот манифест создаёт проект `billing` и задаёт ему общий бюджет ресурсов.

### 2. Добавить namespace'ы в проект

Самый простой способ — пометить namespace label'ом проекта.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: billing-dev
  labels:
    platform.example.io/project-name: billing
---
apiVersion: v1
kind: Namespace
metadata:
  name: billing-stage
  labels:
    platform.example.io/project-name: billing
---
apiVersion: v1
kind: Namespace
metadata:
  name: billing-prod
  labels:
    platform.example.io/project-name: billing
```

После этого все три namespace считаются частью проекта `billing`.

### 3. Выдать доступ к проекту

Если нужно выдать пользователям или группе доступ к проекту, можно использовать `ProjectAccessBinding`.

```yaml
apiVersion: platform.example.io/v1alpha1
kind: ProjectAccessBinding
metadata:
  name: billing-developers
spec:
  projectRef:
    name: billing
  role: "project.developer"
  group:
    name: billing-developers
```

Пример для администратора проекта:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: ProjectAccessBinding
metadata:
  name: billing-admins
spec:
  projectRef:
    name: billing
  role: "project.admin"
  group:
    name: billing-admins
```

## Кастомные роли

Если встроенных ролей недостаточно, можно определить `ProjectRole`.

```yaml
apiVersion: platform.example.io/v1alpha1
kind: ProjectRole
metadata:
  name: app-maintainer
spec:
  projectRef:
    name: billing
  rules:
    - apiGroups: [""]
      resources: ["configmaps", "services"]
      verbs: ["get", "list", "watch", "create", "update", "patch"]
```

А затем выдать её через `ProjectAccessBinding`:

```yaml
apiVersion: platform.example.io/v1alpha1
kind: ProjectAccessBinding
metadata:
  name: billing-app-team
spec:
  projectRef:
    name: billing
  roleRef:
    kind: ProjectRole
    name: app-maintainer
  group:
    name: billing-app-team
```

## Практический пример

Предположим, что команда `billing` хочет:

- держать три namespace: `billing-dev`, `billing-stage`, `billing-prod`
- иметь общий лимит `100 CPU`
- дать группе `billing-admins` административный доступ к проекту
- дать группе `billing-developers` обычный доступ

Тогда минимальный сценарий выглядит так:

1. Создать `Project billing`
2. Создать namespace'ы с label `platform.example.io/project-name: billing`
3. Создать `ProjectAccessBinding` для `billing-admins`
4. Создать `ProjectAccessBinding` для `billing-developers`

## Рекомендации

- используйте единый префикс имён namespace'ов, например `billing-*`
- храните project-level quotas на уровне `Project`, а не в отдельных namespace'ах
- выдавайте доступы на проект, а не вручную в каждый namespace
- начинайте с встроенных ролей и добавляйте `ProjectRole` только когда это действительно нужно
