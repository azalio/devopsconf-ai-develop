# Project Operator — SPEC

Этот документ показывает, как выглядит этап SPEC на практике: от запроса до готовой спецификации.

---

## 1. Исходный запрос

> Мне нужен Kubernetes Project Operator.
>
> Ресурсы:
> - Project
> - ProjectRole
> - ProjectAccessBinding
>
> Project объединяет namespace'ы в одну логическую сущность.
> Namespace присоединяются к проекту через label.
> У проекта есть общая квота CPU / memory / storage.
> ProjectAccessBinding выдаёт доступ к проекту.
> ProjectRole нужен только для кастомных ролей.
> Cluster admin задаёт границы.
> Project admin работает только внутри них.

---

## 2. Вопросы, которые задал AI (раунд 1)

Прежде чем писать спецификацию, нужно дожать неочевидные решения. Вот что было спрошено:

### Модель владения
1. Namespace может принадлежать только одному Project — или допускаем shared namespaces?
2. Что происходит, если кто-то руками снимает label с namespace? Оператор возвращает label обратно или отпускает namespace?
3. Может ли Project существовать без namespace'ов (пустой проект)?

### Квоты
4. Квоты проекта — это hard limit или soft limit?
5. Что считаем: requests подов, LimitRange, ResourceQuota, или что-то другое?
6. Если project quota уменьшили ниже текущего потребления — блокируем update или разрешаем, но блокируем новые аллокации?

### Access model
7. Встроенные роли — какие? Только `project.admin` и `project.developer`, или есть ещё?
8. `ProjectAccessBinding` поддерживает subjects: user, group, serviceaccount — все три, или подмножество?
9. Что реально создаёт оператор при binding — `RoleBinding` в каждом namespace проекта? `ClusterRoleBinding`?

### Lifecycle
10. При удалении Project — что с namespace'ами? Удаляем или оставляем?
11. Нужен ли finalizer на Project для cleanup?
12. Что если namespace удалён извне, пока Project ещё жив?

### Границы (out of scope)
13. Сетевые политики между namespace'ами проекта — in scope?
14. Иерархия проектов (project внутри project) — нужна?
15. Multi-cluster — думаем об этом сейчас или нет?

---

## 3. Ответы инженера (раунд 1)

1. Только одному. Namespace'ы без label проекта попадают в дефолтный проект.
2. Оператор возвращает label обратно.
3. Да, пустой проект допустим.
4. Hard limit — webhook блокирует создание ресурсов сверх лимита.
5. Requests и limits подов.
6. Уменьшение quota разрешено, но новые аллокации сверх нового лимита блокируются.
7. Есть три встроенных роли: `project.admin`, `project.developer`, `project.viewer`.
8. Все три: user, group, serviceaccount.
9. RoleBinding в каждом namespace проекта.
10. Нужна настройка: что делать с namespace'ами при удалении проекта.
11. Да, finalizer нужен.
12. Status обновляется, reconcile продолжает работать, в лог и event пишется событие.
13. Сетевые политики внутри проекта не разрешают доступ по умолчанию (NetworkPolicy in scope).
14. Нет, иерархия не нужна.
15. Нет, multi-cluster out of scope.

---

## 4. Devil's Advocate: критика модели (раунд 2)

После первого spec AI выступил как критик и нашёл пробелы:

### Race conditions
1. **Quota check-then-admit race.** Webhook проверяет quota, видит "есть запас", пропускает pod. Но между проверкой и созданием пода другой pod тоже прошёл. Оба прошли — quota превышена. Какой механизм сериализации?
2. **Label flip race.** Пользователь снимает label, оператор возвращает. Бесконечный цикл. Нужна ли защита?
3. **Namespace migration.** Пользователь меняет label с project-a на project-b. Между снятием и добавлением — квоты обоих проектов некорректны.

### Ownership ambiguity
4. **Дефолтный проект.** Кто создаёт? Что если удалён? Может ли иметь quota?
5. **Managed ресурсы.** Оператор создаёт RoleBinding, ResourceQuota, NetworkPolicy. Что если кто-то их отредактирует или удалит вручную?
6. **ProjectRole scope.** В примере привязан к конкретному проекту. А если роль нужна для нескольких проектов?

### Quota safety
7. **Что именно считаем?** Running pods? Pending? Jobs? CronJobs? Init-контейнеры? DaemonSet?
8. **ResourceQuota per namespace vs. project-level webhook.** Оператор создаёт native ResourceQuota per namespace или полагается только на webhook?
9. **Storage quota.** Storage считается через PVC, не через pod spec. Механизм enforcement?

### Partial failures
10. **Webhook down.** Fail-open = quota не enforced. Fail-closed = namespace заблокирован. Что выбираем?
11. **Reconcile granularity.** Один ProjectAccessBinding падает — другие ждут или обрабатываются независимо?

### Delete/update edge cases
12. **Update projectRef.** Пользователь меняет projectRef в ProjectAccessBinding. Старые RoleBinding'и должны удалиться?
13. **Удаление проекта с живыми namespace'ами.** Допустимо или блокируется?
14. **namespaceDeletionPolicy изменена.** Была Retain, стала Delete. Ретроактивно?

### Authorization boundary
15. **Project admin + cluster-scoped ресурсы.** ProjectAccessBinding — cluster-scoped. Как ограничить project admin только своим проектом?
16. **ProjectRole escalation.** Project admin создаёт ProjectRole с `rules: ["*"]`. Нужна ли валидация?

---

## 5. Ответы инженера (раунд 2)

1. **Quota race.** Допускаем кратковременное превышение. Оператор подсчитает при следующем reconcile, и следующий pod уже не поместится.
2. **Label flip.** Нужна аннотация `platform.example.io/managed-by: project-operator` для защиты от бесконечного цикла.
3. **Namespace migration.** Миграций нет. Чтобы перенести namespace в другой проект: удалить namespace и создать заново в другом проекте.
4. **Дефолтный проект.** Оператор создаёт при старте.
5. **Managed ресурсы.** Оператор всегда следит за своими ресурсами и возвращает к нужному виду.
6. **ProjectRole scope.** Все роли могут быть переиспользованы между проектами. Например, `project.viewer` выдаётся user1 на проект A и user2 на проект B — user1 не видит проект B.
7. **Что считаем.** Всё: running, pending, jobs, cronjobs, init-контейнеры, daemonset.
8. **Enforcement.** Только через project-level webhook.
9. **Storage.** Убираем из scope.
10. **Webhook down.** Fail-closed.
11. **Reconcile granularity.** Каждый ProjectAccessBinding обрабатывается независимо.
12. **Update projectRef.** Да, старые RoleBinding'и должны удалиться, новые создаться.
13. **Удаление проекта.** Нельзя удалить проект, если у него есть namespace'ы. Сначала нужно убрать все namespace'ы.
14. **namespaceDeletionPolicy.** Влияет только на будущее удаление.
15. **Project admin scope.** Нужен admission webhook, который проверяет, что project admin создаёт ProjectAccessBinding/ProjectRole только для своего проекта.
16. **ProjectRole escalation.** Нужна валидация: ProjectRole не может содержать permissions шире, чем у встроенной `project.admin` роли.

---

## 6. Уточнение при реализации (раунд 3)

При обсуждении задачи namespace attachment обнаружился пробел: если кто-то **меняет** label с одного проекта на другой (не снимает, а именно меняет), оператор не знает, к какому проекту возвращать. Аннотация `managed-by: project-operator` не хранит имя проекта.

### Вопрос AI

17. Label снят — оператор восстанавливает. А если label **изменён** на другой проект? Восстанавливать по аннотации? Но в ней нет имени проекта.

### Ответ инженера

17. Label снят → восстанавливаем (по аннотации, в которой хранится имя проекта). Label изменён на другой проект → webhook отклоняет (миграции нет).

18. Webhooks требуют TLS. Как API server будет доверять сертификату webhook server'а?

### Ответ инженера

18. Используем cert-manager как обязательную зависимость. Он выпускает и ротирует сертификаты для webhook server'а, инжектит CA bundle в ValidatingWebhookConfiguration.

### Что изменилось в спецификации

- Аннотация теперь `platform.example.io/project-name: <имя-проекта>` вместо `managed-by: project-operator` — хранит имя проекта, по которому восстанавливается label
- Добавлен namespace update webhook: отклоняет изменение label `project-name` на другое значение (миграция запрещена)
- Снятие label (пустое значение или удаление) — оператор восстанавливает по аннотации
- Добавлен раздел Prerequisites: cert-manager как обязательная зависимость для TLS webhook'ов

---

## 7. Итоговая спецификация (v3)

### 7.1. Цель

Kubernetes-оператор, который объединяет namespace'ы в логические проекты с общими квотами, ролевым доступом и сетевой изоляцией.

API group: `platform.example.io/v1alpha1`

### 7.2. Prerequisites

- **cert-manager** — обязательная зависимость. Admission webhooks требуют TLS-сертификаты. cert-manager автоматически выпускает и ротирует сертификаты для webhook server'а оператора. Оператор использует аннотацию `cert-manager.io/inject-ca-from` на ValidatingWebhookConfiguration для автоматической инъекции CA bundle.

### 7.3. Модель ресурсов

#### Project

Cluster-scoped ресурс. Описывает логический проект.

```yaml
apiVersion: platform.example.io/v1alpha1
kind: Project
metadata:
  name: billing
  finalizers:
    - platform.example.io/project-protection
spec:
  displayName: "Billing"
  description: "Project for billing services"
  quotas:
    requests.cpu: "100"
    requests.memory: "200Gi"
    limits.cpu: "200"
    limits.memory: "400Gi"
status:
  phase: Active             # Active | Terminating
  namespaces:
    - name: billing-dev
      status: Active
    - name: billing-stage
      status: Active
    - name: billing-prod
      status: Active
  usedQuotas:
    requests.cpu: "42"
    requests.memory: "80Gi"
    limits.cpu: "84"
    limits.memory: "160Gi"
  conditions:
    - type: Ready
      status: "True"
      observedGeneration: 3
    - type: QuotaExceeded
      status: "False"
```

#### ProjectRole

Cluster-scoped ресурс. Определяет кастомную роль. Может быть переиспользована между проектами через разные ProjectAccessBinding.

```yaml
apiVersion: platform.example.io/v1alpha1
kind: ProjectRole
metadata:
  name: app-maintainer
spec:
  rules:
    - apiGroups: [""]
      resources: ["configmaps", "services"]
      verbs: ["get", "list", "watch", "create", "update", "patch"]
```

Валидация: rules в ProjectRole не могут превышать permissions встроенной роли `project.admin`. Webhook отклоняет попытку escalation.

#### ProjectAccessBinding

Cluster-scoped ресурс. Выдаёт доступ subject'ам к конкретному проекту.

```yaml
apiVersion: platform.example.io/v1alpha1
kind: ProjectAccessBinding
metadata:
  name: billing-developers
spec:
  projectRef:
    name: billing
  role: "project.developer"        # встроенная роль
  # ИЛИ
  roleRef:                          # кастомная роль
    kind: ProjectRole
    name: app-maintainer
  subjects:
    - kind: Group
      name: billing-developers
    - kind: User
      name: alice@example.com
    - kind: ServiceAccount
      name: ci-bot
      namespace: billing-dev
status:
  conditions:
    - type: Ready
      status: "True"
    - type: ProjectFound
      status: "True"
    - type: RoleValid
      status: "True"
```

При изменении `projectRef` оператор удаляет RoleBinding'и в namespace'ах старого проекта и создаёт в namespace'ах нового.

### 7.4. Namespace attachment

- Namespace присоединяется к проекту через label `platform.example.io/project-name: <project-name>`
- Каждый namespace принадлежит ровно одному проекту
- Namespace без label попадает в дефолтный проект `default`
- Оператор ставит аннотацию `platform.example.io/project-name: <project-name>` на managed namespace'ы — в ней хранится имя проекта-владельца
- Если кто-то **снимает** label — оператор восстанавливает его по значению из аннотации
- Если кто-то **меняет** label на другой проект (namespace уже имеет аннотацию) — webhook отклоняет (миграция запрещена)
- Namespace без label (дефолтный проект, нет аннотации) → **добавление** label существующего проекта — **разрешено** (переход из default в явный проект)
- Миграция между явными проектами не поддерживается. Чтобы перенести: удалить namespace и создать заново с label другого проекта
- Webhook отклоняет создание namespace с label несуществующего проекта

### 7.5. Дефолтный проект

- Оператор создаёт проект `default` при старте, если он не существует
- Все namespace'ы без label `platform.example.io/project-name` автоматически считаются частью дефолтного проекта
- Дефолтный проект может иметь quotas (опционально)
- Дефолтный проект нельзя удалить (защита через webhook)

### 7.6. Встроенные роли

| Роль | Что может |
|------|-----------|
| `project.admin` | Полное управление ресурсами внутри namespace'ов проекта. Может создавать ProjectAccessBinding и ProjectRole для своего проекта (через admission webhook). НЕ может менять project quota. |
| `project.developer` | CRUD на workloads, configmaps, services, secrets внутри namespace'ов проекта. Не может управлять доступами. |
| `project.viewer` | Read-only доступ ко всем ресурсам внутри namespace'ов проекта. |

Встроенные роли реализуются как предопределённые ClusterRole. Оператор создаёт RoleBinding в каждом namespace проекта.

Роли переиспользуемы: одна и та же роль может быть выдана разным пользователям на разные проекты. User1 с `project.viewer` на проект A не видит проект B.

### 7.7. Authorization boundaries

- ProjectAccessBinding и ProjectRole — cluster-scoped ресурсы
- Project admin может создавать/изменять ProjectAccessBinding и ProjectRole **только для своего проекта**
- Это enforced через admission webhook: webhook проверяет, что subject имеет роль `project.admin` в проекте, на который ссылается ресурс
- ProjectRole не может содержать permissions шире `project.admin` (escalation prevention через webhook)

### 7.8. Квоты

- Квоты задаются на уровне Project (`spec.quotas`): `requests.cpu`, `requests.memory`, `limits.cpu`, `limits.memory`
- Storage — **out of scope**
- Оператор суммирует реальное потребление всех подов (running, pending, jobs, cronjobs, init-контейнеры, daemonset) по всем namespace'ам проекта
- Enforcement — **только через project-level admission webhook** (не через native ResourceQuota per namespace)
- Webhook читает текущее потребление из `project.status.usedQuotas` и отклоняет создание/update подов, если `usedQuotas + новый pod > spec.quotas`. Webhook не ходит по namespace'ам — работает быстро
- **Допускается кратковременное превышение** в race condition (два пода прошли webhook одновременно). Оператор обнаружит при следующем reconcile и заблокирует дальнейшие аллокации
- Уменьшение quota ниже текущего потребления разрешено, но новые аллокации блокируются
- Текущее потребление отражается в `status.usedQuotas`
- Webhook — **fail-closed**: если webhook недоступен, создание подов в namespace'ах проекта блокируется

### 7.9. Managed ресурсы

Оператор создаёт и поддерживает в каждом namespace проекта:

- RoleBinding'и (для каждого ProjectAccessBinding)

Все managed ресурсы помечаются label'ом `platform.example.io/managed-by: project-operator`.
Namespace'ы дополнительно получают аннотацию `platform.example.io/project-name: <имя-проекта>` для восстановления label.

Если managed ресурс изменён или удалён вручную — оператор восстанавливает его при следующем reconcile.

### 7.11. Lifecycle

#### Старт оператора
1. Создаёт дефолтный проект `default`, если не существует
2. Полный reconcile всех проектов (идемпотентный)

#### Создание Project
1. Создаётся ресурс Project
2. Оператор ставит finalizer `platform.example.io/project-protection`
3. Оператор начинает watch namespace'ов с соответствующим label
4. Для каждого найденного namespace: создаёт RoleBinding'и, ставит аннотацию
5. Считает текущее потребление квот
6. Обновляет status

#### Удаление Project
1. Kubernetes ставит `deletionTimestamp`
2. **Webhook блокирует удаление, если у проекта есть namespace'ы.** Сначала нужно удалить или открепить все namespace'ы
3. После того как namespace'ов не осталось: оператор удаляет связанные managed ресурсы
4. Снимает finalizer
5. Project удаляется

#### Удаление namespace извне
- Оператор обнаруживает пропажу namespace через reconcile
- Удаляет namespace из `status.namespaces`
- Пересчитывает `status.usedQuotas`
- Пишет Event и log-запись
- Reconcile продолжает работать для оставшихся namespace'ов

#### Update projectRef в ProjectAccessBinding
1. Оператор обнаруживает изменение projectRef
2. Удаляет RoleBinding'и в namespace'ах старого проекта
3. Создаёт RoleBinding'и в namespace'ах нового проекта
4. Обновляет status

### 7.12. Инварианты

- Every namespace MUST belong to exactly one Project
- Namespaces without a project label MUST be assigned to the `default` project
- Operator MUST create the `default` project on startup
- The `default` project MUST NOT be deletable
- A Project MUST NOT be deletable while it has namespaces
- Operator MUST restore the project label if removed from a managed namespace (using project name stored in annotation)
- Changing the project label on a managed namespace (has annotation) MUST be rejected by webhook (no migration)
- Adding a project label to an unmanaged namespace (no annotation, default project) MUST be allowed if the target project exists
- Finalizer MUST be present on every Project
- Quota enforcement MUST be done via admission webhook only (no native ResourceQuota)
- Webhook MUST be fail-closed
- Brief quota overages due to race conditions are acceptable; operator MUST detect and block further allocations on next reconcile
- All pod types (running, pending, jobs, cronjobs, init-containers, daemonset) MUST be counted toward project quotas
- RoleBindings MUST exist in every namespace of the project for every active ProjectAccessBinding
- Managed resources MUST be restored if modified or deleted externally
- ProjectRole rules MUST NOT exceed `project.admin` permissions (escalation prevention)
- Project admin MUST only be able to create ProjectAccessBinding/ProjectRole for their own project (admission webhook)
- Each ProjectAccessBinding MUST be reconciled independently (partial failure isolation)
- Reconciler MUST be idempotent

### 7.13. Edge cases

- Namespace создан с label несуществующего проекта → webhook отклоняет
- Два ProjectAccessBinding дают одному subject разные роли в одном проекте → оба RoleBinding создаются, Kubernetes объединяет permissions
- Project quota = 0 → допустимо, блокирует все новые аллокации
- Namespace с подами удаляется, потом пересоздаётся с тем же именем → оператор создаёт managed ресурсы заново
- ProjectRole ссылается на несуществующий проект (через ProjectAccessBinding) → status condition `ProjectNotFound`, RoleBinding не создаётся
- ProjectAccessBinding ссылается на несуществующую ProjectRole → status condition `RoleNotFound`, RoleBinding не создаётся
- ProjectRole содержит permissions шире `project.admin` → webhook отклоняет
- Project admin пытается создать ProjectAccessBinding для чужого проекта → webhook отклоняет
- Два пода проходят quota webhook одновременно → кратковременное превышение допустимо, оператор заблокирует следующие аллокации
- Label снят с namespace, у которого есть аннотация project-name → оператор восстанавливает label по значению из аннотации
- Label изменён на другой проект (есть аннотация) → webhook отклоняет (миграция запрещена)
- Namespace из дефолтного проекта (нет аннотации) → добавление label существующего проекта → разрешено, оператор подхватывает namespace
- Label снят с namespace без аннотации → оператор не вмешивается (namespace не был managed)
- Дефолтный проект удаляется → webhook отклоняет

### 7.14. Failure modes

| Ситуация | Поведение |
|----------|-----------|
| API server недоступен | Reconcile retry с exponential backoff |
| Один namespace из трёх — partial failure при создании managed ресурсов | Status отражает проблемный namespace, остальные работают |
| Quota webhook недоступен | **Fail-closed**: создание подов блокируется |
| Оператор перезапущен | Полный reconcile всех проектов, идемпотентный |
| Конфликт при update status | Retry с re-fetch |
| Один ProjectAccessBinding из нескольких не reconcile'ится | Остальные обрабатываются независимо |
| Managed ресурс удалён вручную | Восстанавливается при следующем reconcile |

### 7.15. Out of scope

- Иерархия проектов
- Multi-cluster
- Billing / cost allocation
- CI/CD integration
- Storage quotas
- NetworkPolicy / сетевая изоляция
- Управление workloads внутри namespace'ов
- Миграция namespace между проектами (только удаление + пересоздание)

### 7.16. Admission webhooks (summary)

| Webhook | Тип | Что проверяет |
|---------|-----|---------------|
| Namespace create validation | Validating | Label ссылается на существующий проект |
| Namespace update validation | Validating | Смена label на другой проект отклоняется для managed NS (есть аннотация); добавление label на unmanaged NS — разрешено |
| Project deletion | Validating | Проект не имеет namespace'ов; дефолтный проект не удаляется |
| Pod quota | Validating | Суммарное потребление проекта не превышает quota |
| ProjectAccessBinding scope | Validating | Project admin создаёт binding только для своего проекта |
| ProjectRole escalation | Validating | Rules не шире `project.admin` |

### 7.17. Acceptance criteria

- [ ] Оператор создаёт дефолтный проект при старте
- [ ] Project создаётся, namespace'ы с label подхватываются
- [ ] Квоты enforced: под сверх лимита отклоняется webhook'ом
- [ ] Все типы подов (running, pending, jobs, etc.) учитываются в quota
- [ ] Кратковременное превышение quota допускается, следующие аллокации блокируются
- [ ] Webhook fail-closed: при недоступности webhook'а поды не создаются
- [ ] ProjectAccessBinding создаёт RoleBinding во всех namespace'ах проекта
- [ ] Встроенные роли (admin/developer/viewer) работают корректно
- [ ] Роли переиспользуемы между проектами (user1 видит проект A, но не B)
- [ ] ProjectRole создаёт кастомную роль, ProjectAccessBinding может на неё ссылаться
- [ ] ProjectRole с escalation отклоняется webhook'ом
- [ ] Project admin может управлять доступами только в своём проекте
- [ ] Нельзя удалить проект, если у него есть namespace'ы
- [ ] Нельзя удалить дефолтный проект
- [ ] Снятие label с managed namespace — оператор восстанавливает label по аннотации
- [ ] Изменение label на другой проект — webhook отклоняет
- [ ] Удаление namespace извне — status обновляется, event пишется
- [ ] Managed ресурсы восстанавливаются при ручном изменении/удалении
- [ ] Update projectRef в ProjectAccessBinding корректно мигрирует RoleBinding'и
- [ ] Partial failure по одному namespace не ломает весь reconcile
- [ ] Каждый ProjectAccessBinding reconcile'ится независимо
- [ ] Reconciler идемпотентен: повторный запуск не создаёт дубликатов
