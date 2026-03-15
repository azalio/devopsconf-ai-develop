# Project Operator — PLAN

Этот документ показывает, как выглядит этап PLAN на практике: от утверждённого spec до последовательности маленьких задач.

---

## 1. Исходный prompt

```text
У меня есть утверждённый spec для Project Operator.
Разбей реализацию на маленькие задачи.

Для каждой задачи укажи:
- имя
- цель
- зависимости
- PRE conditions
- POST conditions
- как проверить завершённость

Ограничения:
- одна задача = один логический шаг
- не смешивай CRD, reconciler, webhook и tests в одном большом шаге
```

---

## 2. Implementation Plan

### Общее правило: TDD для каждой задачи

Начиная с задачи 4, каждая задача выполняется в два шага:

1. **В чистой сессии** — написать тесты по POST conditions задачи. Тесты должны падать.
2. **В другой сессии** — реализовать код, который проходит эти тесты.

Почему: если код уже в контексте, AI пишет тесты под реализацию, включая её баги. Разделение сессий устраняет этот confirmation bias.

---

### Задача 1. Scaffold проекта

**Цель:** Инициализировать Go-проект с kubebuilder, настроить Kind-кластер с cert-manager.

**Зависимости:** нет

**PRE:**
- Go, Docker, Kind, kubectl установлены
- Kind-кластер создан

**POST:**
- kubebuilder проект инициализирован (`go.mod`, `Makefile`, `PROJECT`, `cmd/main.go`)
- cert-manager установлен в кластер
- `make build` проходит
- CLAUDE.md/AGENTS.md создан с командами и conventions

**Как проверить:**
```bash
make build
kubectl get pods -n cert-manager
```


---

### Задача 2. Определить CRD types

**Цель:** Описать Go-структуры для Project, ProjectRole, ProjectAccessBinding по спецификации.

**Зависимости:** Задача 1

**PRE:**
- kubebuilder проект инициализирован
- SPEC.md утверждён

**POST:**
- `api/v1alpha1/project_types.go` — Project spec, status, conditions
- `api/v1alpha1/projectrole_types.go` — ProjectRole spec
- `api/v1alpha1/projectaccessbinding_types.go` — ProjectAccessBinding spec, status
- Все типы cluster-scoped
- `make generate` проходит без ошибок
- `make manifests` генерирует CRD YAML

**Как проверить:**
```bash
make generate
make manifests
go vet ./...
```


---

### Задача 3. Установить CRD в кластер и проверить

**Цель:** Убедиться, что CRD корректно устанавливаются и принимают манифесты из spec.

**Зависимости:** Задача 2

**PRE:**
- CRD YAML сгенерированы
- Kind-кластер работает

**POST:**
- CRD установлены в кластер
- Можно создать тестовые ресурсы Project, ProjectRole, ProjectAccessBinding из примеров SPEC
- `kubectl get projects`, `kubectl get projectroles`, `kubectl get projectaccessbindings` работают

**Как проверить:**
```bash
make install
kubectl apply -f config/samples/
kubectl get projects
kubectl get projectroles
kubectl get projectaccessbindings
```


---

### Задача 4. Project reconciler — базовый каркас

**Цель:** Реализовать пустой reconciler для Project, который ставит finalizer и обновляет status.phase.

**Зависимости:** Задача 2

**PRE:**
- CRD types определены
- `make generate` проходит

**POST:**
- Project reconciler зарегистрирован в manager
- При создании Project ставится finalizer `platform.example.io/project-protection`
- `status.phase` = `Active`
- При `deletionTimestamp` — `status.phase` = `Terminating`
- Reconciler идемпотентен: повторный запуск ничего не ломает

**Как проверить:**
```bash
make run
kubectl apply -f config/samples/project.yaml
kubectl get project billing -o jsonpath='{.status.phase}'  # Active
kubectl get project billing -o jsonpath='{.metadata.finalizers}'  # [...project-protection]
```


---

### Задача 5. Namespace attachment

**Цель:** Reconciler подхватывает namespace'ы с label проекта, ставит аннотацию, обновляет status.namespaces.

**Зависимости:** Задача 4

**PRE:**
- Project reconciler работает
- Namespace'ы с label `platform.example.io/project-name` существуют

**POST:**
- Reconciler находит все namespace'ы с label проекта
- Ставит аннотацию `platform.example.io/project-name: <project-name>` на каждый namespace
- `status.namespaces` содержит список namespace'ов с их статусом
- Если namespace удалён извне — убирается из status, пишется Event
- Label восстанавливается, если снят с managed namespace (есть аннотация)

**Как проверить:**
```bash
kubectl create ns billing-dev
kubectl label ns billing-dev platform.example.io/project-name=billing
# Проверить status.namespaces
kubectl get project billing -o jsonpath='{.status.namespaces}'
# Снять label, проверить что вернулся
kubectl label ns billing-dev platform.example.io/project-name-
kubectl get ns billing-dev --show-labels
```


---

### Задача 6. Дефолтный проект

**Цель:** Оператор создаёт проект `default` при старте.

**Зависимости:** Задача 4

**PRE:**
- Project reconciler работает

**POST:**
- При старте оператора проект `default` создаётся, если не существует
- Namespace'ы без label `platform.example.io/project-name` считаются частью дефолтного проекта
- Дефолтный проект получает finalizer

**Как проверить:**
```bash
make run
kubectl get project default  # существует
```


---

### Задача 7. Quota reconciliation

**Цель:** Reconciler подсчитывает реальное потребление ресурсов по всем namespace'ам проекта и обновляет `status.usedQuotas`.

**Зависимости:** Задача 5

**PRE:**
- Namespace attachment работает
- Project.spec.quotas заданы

**POST:**
- Reconciler суммирует requests/limits всех подов (running, pending, jobs, cronjobs, init-контейнеры, daemonset) во всех namespace'ах проекта
- Результат записывается в `project.status.usedQuotas`
- Если потребление превышает quota — ставится condition `QuotaExceeded: True`
- При изменении подов (создание/удаление) reconcile перепересчитывает usedQuotas
- Reconciler идемпотентен

**Как проверить:**
```bash
kubectl get project billing -o jsonpath='{.status.usedQuotas}'
# Создать под в namespace проекта
kubectl run test --image=nginx -n billing-dev --requests='cpu=1,memory=1Gi'
# Проверить что usedQuotas обновились
kubectl get project billing -o jsonpath='{.status.usedQuotas}'
```


---

### Задача 8. Access reconciliation — встроенные роли

**Цель:** Реализовать создание RoleBinding'ов по ProjectAccessBinding для встроенных ролей.

**Зависимости:** Задача 5

**PRE:**
- Namespace attachment работает
- ClusterRole для встроенных ролей (project.admin, project.developer, project.viewer) определены согласно SPEC.md §7.6

**POST:**
- При создании ProjectAccessBinding с `role: project.developer` создаются RoleBinding'и в каждом namespace проекта
- RoleBinding ссылается на соответствующий ClusterRole
- Subjects берутся из ProjectAccessBinding.spec.subjects
- При удалении ProjectAccessBinding — RoleBinding'и удаляются
- RoleBinding'и помечены label `platform.example.io/managed-by: project-operator`
- Каждый ProjectAccessBinding reconcile'ится независимо

**Как проверить:**
```bash
kubectl apply -f config/samples/projectaccessbinding.yaml
kubectl get rolebindings -n billing-dev
kubectl get rolebindings -n billing-stage
kubectl get rolebindings -n billing-prod
```


---

### Задача 9. Access reconciliation — кастомные роли (ProjectRole)

**Цель:** Поддержать ProjectAccessBinding с roleRef на ProjectRole.

**Зависимости:** Задача 8

**PRE:**
- Access reconciliation для встроенных ролей работает

**POST:**
- ProjectRole создаёт ClusterRole с соответствующими rules
- ProjectAccessBinding с `roleRef.kind: ProjectRole` создаёт RoleBinding'и, ссылающиеся на этот ClusterRole
- Если ProjectRole не существует — status condition `RoleNotFound`, RoleBinding не создаётся
- ClusterRole помечен label `platform.example.io/managed-by: project-operator`

**Как проверить:**
```bash
kubectl apply -f config/samples/projectrole.yaml
kubectl apply -f config/samples/projectaccessbinding-custom.yaml
kubectl get clusterrole | grep app-maintainer
kubectl get rolebindings -n billing-dev
```


---

### Задача 10. Update projectRef в ProjectAccessBinding

**Цель:** При изменении projectRef корректно мигрировать RoleBinding'и.

**Зависимости:** Задача 8

**PRE:**
- Access reconciliation работает
- ProjectAccessBinding привязан к проекту A

**POST:**
- При смене projectRef с проекта A на проект B:
  - RoleBinding'и в namespace'ах проекта A удаляются
  - RoleBinding'и в namespace'ах проекта B создаются
- Status обновляется

**Как проверить:**
```bash
# Изменить projectRef в binding
kubectl patch projectaccessbinding billing-devs --type merge -p '{"spec":{"projectRef":{"name":"payments"}}}'
kubectl get rolebindings -n billing-dev      # пусто
kubectl get rolebindings -n payments-dev     # есть
```


---

### Задача 11. Namespace validation webhook

**Цель:** Webhook отклоняет создание namespace с label несуществующего проекта и изменение label на другой проект.

**Зависимости:** Задача 2, cert-manager в кластере

**PRE:**
- CRD types определены
- cert-manager установлен

**POST:**
- ValidatingWebhookConfiguration зарегистрирована
- CREATE namespace с label несуществующего проекта → отклоняется
- UPDATE managed namespace (есть аннотация): смена label на другой проект → отклоняется
- UPDATE unmanaged namespace (нет аннотации): добавление label существующего проекта → разрешено
- UPDATE namespace: снятие label → допускается (оператор восстановит через reconcile для managed NS)
- Webhook fail-closed

**Как проверить:**
```bash
kubectl create ns test-ns --dry-run=server -o yaml  # с label несуществующего проекта → ошибка
```


---

### Задача 12. Project deletion webhook

**Цель:** Webhook блокирует удаление проекта с namespace'ами и удаление дефолтного проекта.

**Зависимости:** Задача 5, Задача 6

**PRE:**
- Namespace attachment работает
- Дефолтный проект создаётся

**POST:**
- DELETE Project, у которого есть namespace'ы → отклоняется
- DELETE Project `default` → отклоняется
- DELETE Project без namespace'ов → разрешается, finalizer cleanup запускается

**Как проверить:**
```bash
kubectl delete project billing  # rejected: has namespaces
kubectl delete project default  # rejected: default project
kubectl delete project test     # удаляется, в проекте нет NS
```


---

### Задача 13. Pod quota webhook

**Цель:** Webhook отклоняет создание/update подов, если суммарное потребление проекта превысит quota.

**Зависимости:** Задача 7

**PRE:**
- Quota reconciliation работает (`status.usedQuotas` заполняется)
- Project.spec.quotas заданы

**POST:**
- Webhook берёт текущее потребление из `project.status.usedQuotas` (подсчитывается reconciler'ом)
- Если `status.usedQuotas` + requests нового пода > `spec.quotas` → отклоняется
- Webhook не ходит по namespace'ам — работает быстро, только читает status проекта
- `status.usedQuotas` обновляется reconciler'ом (все типы подов: running, pending, jobs, cronjobs, init-контейнеры, daemonset)
- Webhook fail-closed

**Как проверить:**
```bash
# Установить маленькую quota на проект
# Создать поды до превышения → последний отклоняется
```


---

### Задача 14. Authorization webhooks

**Цель:** Webhook ограничивает project admin: только свой проект для ProjectAccessBinding/ProjectRole. Webhook предотвращает escalation в ProjectRole.

**Зависимости:** Задача 8, Задача 9

**PRE:**
- Access reconciliation работает
- Встроенные роли определены

**POST:**
- Project admin создаёт ProjectAccessBinding для чужого проекта → отклоняется
- Project admin создаёт ProjectRole с rules шире `project.admin` → отклоняется
- Cluster admin может всё

**Как проверить:**
```bash
# Impersonate project admin и попробовать создать binding для чужого проекта → ошибка
```


---

### Задача 15. Status conditions

**Цель:** Добавить полноценные status conditions на все ресурсы.

**Зависимости:** Задачи 4–14

**PRE:**
- Все reconciler'ы и webhook'и работают

**POST:**
- Project: conditions `Ready`, `QuotaExceeded` с `observedGeneration`
- ProjectAccessBinding: conditions `Ready`, `ProjectFound`, `RoleValid`
- ProjectRole: condition `Ready`
- Partial failure отражается в conditions (конкретный namespace в status)

**Как проверить:**
```bash
kubectl get project billing -o jsonpath='{.status.conditions}'
kubectl get projectaccessbinding billing-devs -o jsonpath='{.status.conditions}'
```


---

### Задача 16. Проверка покрытия и интеграционные сценарии

**Цель:** Проверить, что тесты, написанные per task, покрывают все acceptance criteria из SPEC. Добить пропущенные сценарии. Добавить cross-task интеграционные тесты.

**Зависимости:** Задачи 4–15

**PRE:**
- Каждая задача (4–15) уже имеет свои тесты (написаны по TDD-правилу)
- Все тесты проходят

**POST:**
- Все acceptance criteria из SPEC.md покрыты тестами
- Добавлены интеграционные сценарии, которые затрагивают несколько компонентов:
  - Full user scenario: Project → namespace'ы → access → quota → delete
  - Cross-component: quota webhook + reconciler + namespace attachment
- Нет acceptance criteria без теста

**Как проверить:**
```bash
make test
```


---

### Задача 17. Review и cleanup

**Цель:** Code review всего кода, cleanup, lint.

**Зависимости:** Задача 16

**PRE:**
- Все тесты проходят

**POST:**
- `go vet ./...` чисто
- `golangci-lint run` чисто
- Нет TODO/FIXME без issue
- Код соответствует spec
- Все assumptions задокументированы

**Как проверить:**
```bash
go vet ./...
golangci-lint run
make test
```


---

### Задача 18. E2E тесты в Kind

**Цель:** Прогнать полный сценарий в реальном кластере.

**Зависимости:** Задача 17

**PRE:**
- Kind-кластер с cert-manager
- Оператор собран в Docker image
- Image загружен в Kind

**POST:**
- Полный user scenario из SPEC работает:
  1. Оператор стартует, дефолтный проект создан
  2. Создать Project billing
  3. Создать namespace'ы с label
  4. Создать ProjectAccessBinding
  5. Проверить RoleBinding'и, NetworkPolicy
  6. Проверить quota enforcement
  7. Удалить namespace, проверить status
  8. Попробовать удалить проект с namespace'ами → отклонено
  9. Удалить все namespace'ы, удалить проект → ок

**Как проверить:**
```bash
make docker-build
kind load docker-image <image> --name devops26
make deploy
# Прогнать сценарий
```


---

## 3. Граф зависимостей

```
1. Scaffold
└── 2. CRD types
    ├── 3. Install CRD + samples
    ├── 4. Project reconciler (базовый)
    │   ├── 5. Namespace attachment
    │   │   ├── 7. Quota reconciliation
    │   │   │   └── 13. Pod quota webhook
    │   │   ├── 8. Access — встроенные роли
    │   │   │   ├── 9. Access — кастомные роли
    │   │   │   ├── 10. Update projectRef
    │   │   │   └── 14. Authorization webhooks
    │   │   ├── 11. Namespace validation webhook
    │   │   └── 12. Project deletion webhook
    │   └── 6. Дефолтный проект
    └── 15. Status conditions (после 4–14)
        └── 16. Envtest
            └── 17. Review + cleanup
                └── 18. E2E в Kind
```

---

## 4. Уточнения при обсуждении плана (раунд 2)

### 4.1. NetworkPolicy убрана из scope

Инженер решил убрать задачу NetworkPolicy (default-deny) из scope проекта. Сетевая изоляция — отдельная задача, которая усложняет оператор без прямой связи с основной функциональностью (проекты, квоты, доступы). Задача удалена, нумерация задач обновлена (было 18, стало 17).

### 4.2. Переход namespace из дефолтного проекта

При обсуждении задачи 6 (дефолтный проект) обнаружился пробел: namespace без label неявно принадлежит дефолтному проекту, но у него нет аннотации `platform.example.io/project-name`. Что произойдёт, если на такой namespace поставить label другого проекта?

**Вопрос:**

> Namespace из дефолтного проекта (без label, без аннотации). Кто-то добавляет label `platform.example.io/project-name: billing`. Webhook это отклонит как «миграцию» или разрешит?

**Ответ инженера:**

Разрешить. Дефолтный проект — это «ничей», а не «мой». Логика webhook'а:
- Namespace **с аннотацией** (managed) → смена label на другой проект → **отклоняется** (миграция запрещена)
- Namespace **без аннотации** (unmanaged, дефолтный проект) → добавление label существующего проекта → **разрешено**

### 4.3. Quota webhook не считает — только читает status

В первой версии плана задача 12 (Pod quota webhook) описывала, что webhook суммирует requests/limits всех подов в namespace'ах проекта. Это неправильно — webhook должен работать быстро.

**Вопрос:**

> Webhook при каждом создании пода ходит по всем namespace'ам проекта и считает потребление?

**Ответ инженера:**

Нет. Webhook только читает `project.status.usedQuotas` и сравнивает с `spec.quotas`. Подсчёт текущего потребления — задача reconciler'а. Webhook должен работать очень быстро.

### 4.4. Подсчёт квот — отдельная задача

В первой версии плана не было явной задачи на подсчёт `status.usedQuotas`. Это упоминалось только вскользь в задаче Pod quota webhook. По нашему фреймворку (один логический шаг = одна задача) подсчёт квот — отдельный reconciler, не часть webhook'а и не часть namespace attachment.

Добавлена задача 7 (Quota reconciliation): reconciler считает потребление, webhook только читает результат.

### 4.5. TDD per task, а не тесты в конце

Изначально задача 16 была «написать все envtest». Это противоречит принципу TDD, который мы проповедуем в презентации: если код уже написан, AI пишет тесты под реализацию, включая её баги.

**Решение:** каждая задача (начиная с 4) выполняется в два шага — тесты в чистой сессии, реализация в другой. Задача 16 стала «проверить покрытие и добить интеграционные сценарии».

### Что изменилось

- Удалена задача NetworkPolicy из плана и SPEC
- Добавлена задача 7 (Quota reconciliation): подсчёт потребления в `status.usedQuotas`
- Обновлена задача 11 (Namespace validation webhook): webhook проверяет наличие аннотации перед блокировкой смены label
- Обновлена задача 13 (Pod quota webhook): webhook читает status, не ходит по namespace'ам; зависит от задачи 7
- Задача 16 переосмыслена: не «написать все тесты», а «проверить покрытие и добить пропуски»
- Добавлено общее правило TDD per task в начало плана
- Обновлён SPEC.md: namespace attachment, инварианты, edge cases, таблица webhooks, quota enforcement

---

## 5. Критерий качества

По слайду 23 презентации:

- [x] Каждую задачу можно дать AI отдельно — да, у каждой есть PRE/POST и scope
- [x] Каждую задачу можно проверить независимо — да, у каждой есть команды проверки
- [x] Зависимости понятны заранее — да, граф зависимостей описан
- [x] Нет больших зон «сейчас просто сделаем всё остальное» — нет, каждый шаг конкретный
