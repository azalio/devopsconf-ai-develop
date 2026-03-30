# Как разрабатывать production Kubernetes-операторы с помощью LLM

Материалы к докладу на [DevOpsConf 2026](https://devopsconf.io/moscow/2026/abstracts/17475), 2 апреля 2026.

Михаил Петров, Yandex Cloud.

## О чём доклад

Прямое использование LLM как «умного генератора кода» на сложных системах быстро упирается в потерю контроля: AI молча принимает архитектурные решения, генерирует большие diff, упрощает модель там, где упрощать нельзя.

В докладе показан воспроизводимый инженерный процесс **SPEC → PLAN → TEST → CODE → REVIEW → LEARN**, который превращает LLM из генератора правдоподобного кода в управляемый инструмент разработки. Каждый этап — вход, действие, выход, критерий качества и отдельная роль для LLM.

Реальный кейс: production Kubernetes-оператор за 7 дней вместо оценки в 90 дней.

## Demo

Project Operator в реальном кластере Stackland — от пустого проекта до мультитенантной изоляции с квотами и RBAC за 2 минуты:

![Demo](demo.svg)

<details>
<summary>Локальное воспроизведение через asciinema</summary>

```bash
# Установить asciinema: brew install asciinema
asciinema play demo.cast
```
</details>

## Артефакты процесса

Это не абстрактные шаблоны, а реальные рабочие артефакты проекта:

| Файл | Этап | Что внутри |
|------|------|-----------|
| [SPEC.md](SPEC.md) | SPEC | Модель ресурсов, инварианты, edge cases, acceptance criteria |
| [PLAN.md](PLAN.md) | PLAN | 18 задач с PRE/POST conditions и критериями готовности |
| [CLAUDE.md](CLAUDE.md) | LEARN | Project context file: команды, конвенции, known gotchas |
| [project.md](project.md) | — | Исходная постановка задачи |
| [project-operator/](project-operator/) | CODE | Go-код оператора (kubebuilder v4) |

### Как читать артефакты

```
project.md          — "Что мы хотим построить"
    ↓
SPEC.md             — "Какие инварианты мы фиксируем до кода"
    ↓
PLAN.md             — "Как разбиваем на маленькие проверяемые шаги"
    ↓
project-operator/   — "Код, который прошёл тесты и review"
    ↓
CLAUDE.md           — "Что мы узнали и не хотим повторять"
```

## MAP Framework

Цикл SPEC → PLAN → TEST → CODE → REVIEW → LEARN реализован как open-source инструмент для Claude Code:

**[github.com/azalio/map-framework](https://github.com/azalio/map-framework)**

```bash
pip install mapify-cli
mapify init                    # установка в проект

/map-plan "задача"             # декомпозиция на подзадачи с контрактами
/map-tdd "задача"              # тесты до кода в чистой сессии
/map-efficient "задача"        # реализация с quality gates
/map-review                    # независимый review
/map-learn                     # извлечение уроков в project memory
```

11 специализированных агентов, state machine, branch-scoped артефакты.

## Демо-сценарий

Полный сценарий демонстрации с командами и ожидаемыми результатами: [demo-scenario.md](demo-scenario.md)

## Презентация

Полный текст презентации со спикер-нотами: [devopsconf_2026_ai_operator_presentation_rewrite-v2.md](devopsconf_2026_ai_operator_presentation_rewrite-v2.md)

## Ссылки

- [MAP Framework](https://github.com/azalio/map-framework) — реализация цикла для Claude Code
- [DevOpsConf 2026](https://devopsconf.io/moscow/2026/abstracts/17475) — страница доклада

## Оцените доклад

![QR-код для оценки](qr-code.gif)
