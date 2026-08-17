# Task Management System (task-service)
REST API сервис управления задачами, командной работы и аналитики продуктивности.

## Архитектура проекта
```text
.
├── cmd/
│   └── main.go               # Точка входа в приложение (Composition Root)
├── config.yaml.example       # Шаблон файла конфигурации
├── internal/
│   ├── app/                  # DI-контейнер, инициализация зависимостей
│   ├── config/               # Загрузка и валидация YAML-конфигурации
│   ├── handler/              # HTTP-слой (Delivery Layer): маппинг DTO, статусы
│   ├── middleware/           # HTTP Middleware (Auth JWT, CORS, Recovery)
│   ├── model/                # Доменные сущности (Domain Layer)
│   ├── repository/           # Интерфейсы и имплементации Data Access (MySQL/Redis)
│   ├── service/              # Бизнес-логика приложения (Use Cases)
│   └── util/                 # Вспомогательные компоненты (JWT Manager)
├── migrations/               # SQL-миграции (Goose)
├── docs/                     # Автосгенерированная документация Swagger
├── Dockerfile                # Multi-stage Dockerfile
├── docker-compose.yml        # Оркестрация контейнеров (App + MySQL + Redis)
├── README.md                 # Инструкции по запуску, миграциям, примеры
└── Taskfile.yml              # Автоматизация команд разработки
```

## Быстрый запуск
Предварительные требования
* Docker и Docker Compose
* Task (go-task) — опционально, для удобства выполнения команд

1. Клонирование и настройка окружения
```
git clone https://github.com/your-org/task-service.git
cd task-service

# Создание конфигурационного файла из шаблона
cp config.yaml.example config.yaml
```

2. Запуск через Docker Compose
Поднять все сервисы (MySQL, Redis, App):

```bash
task up
# Или напрямую через docker-compose:
# docker-compose up -d --build
```

### База данных и миграции
Для управления схемой базы данных используется инструмент Goose.

```bash
# Применить все миграции
task migrate-up

# Откатить последнюю миграцию
task migrate-down

# Проверить статус миграций
task migrate-status
```

### Swagger / OpenAPI Документация
```bash 
task swagger
```

После запуска приложения интерактивная Swagger UI схема доступна по адресу:
`http://localhost:8080/swagger/index.html`

## Примеры curl-запросов к API

### Пользователи и аутентификация
#### 1. Регистрация нового пользователя
`POST /api/v1/register`

```bash
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "strongpassword123",
    "name": "Иван Иванов"
  }'
```

#### 2. Аутентификация (Вход)
```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "strongpassword123"
  }'
```

### Управление командами
#### 3. Создание команды
```bash
curl -X POST http://localhost:8080/api/v1/teams \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <YOUR_JWT_TOKEN>" \
  -d '{
    "name": "Backend Developers"
  }'
```

#### 4. Получение списка команд пользователя
```bash
curl -X GET http://localhost:8080/api/v1/teams \
  -H "Authorization: Bearer <YOUR_JWT_TOKEN>"
```

#### 5. Приглашение участника в команду
```bash
curl -X POST http://localhost:8080/api/v1/teams/1/invite \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <YOUR_JWT_TOKEN>" \
  -d '{
    "user_id": 2,
    "role": "admin"
  }'
```

### Задачи, комментарии и история
#### 6. Создание задачи
```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <YOUR_JWT_TOKEN>" \
  -d '{
    "title": "Реализовать кеширование в Redis",
    "description": "Обернуть TaskRepository в декоратор кеширования",
    "team_id": 1,
    "assignee_id": 2,
    "due_date": "2026-09-01T18:00:00Z"
  }'
```

#### 7. Получение списка задач команды (с кешированием и пагинацией)
```bash
curl -X GET "http://localhost:8080/api/v1/tasks?team_id=1&status=in_progress&limit=10&offset=0" \
  -H "Authorization: Bearer <YOUR_JWT_TOKEN>"
```

#### 8. Обновление задачи
```bash
curl -X PUT http://localhost:8080/api/v1/tasks/1 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <YOUR_JWT_TOKEN>" \
  -d '{
    "title": "Реализовать кеширование в Redis (завершено)",
    "status": "closed",
    "assignee_id": 2
  }'
```

#### 9. Получение истории изменений задачи
```bash 
curl -X GET http://localhost:8080/api/v1/tasks/1/history \
  -H "Authorization: Bearer <YOUR_JWT_TOKEN>"
```

#### 10. Добавление комментария к задаче
```bash 
curl -X POST http://localhost:8080/api/v1/tasks/1/comments \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <YOUR_JWT_TOKEN>" \
  -d '{
    "content": "Задача выполнена, покрыта интеграционными тестами."
  }'
```

#### 11. Получение списка комментариев задачи
```bash 
curl -X GET http://localhost:8080/api/v1/tasks/1/comments \
  -H "Authorization: Bearer <YOUR_JWT_TOKEN>"
```

#### 12. Получение SQL-отчета по аналитике команды
```bash 
curl -X GET http://localhost:8080/api/v1/teams/1/stats \
  -H "Authorization: Bearer <YOUR_JWT_TOKEN>"
```

## Полезные команды (Taskfile)
|Команда|Описание|
|-------|--------|
|task up|Собрать и запустить все сервисы в Docker|
|task down|Остановить контейнеры|
|task down-v|Полная очистка с удалением Docker Volumes (БД и Redis)|
|task logs|Просмотр логов приложения в реальном времени|
|task swagger|Сгенерировать Swagger-документацию из кода|
|task swagger-fmt|Форматирование Swagger-аннотаций в коде|
|task rebuild|Пересобрать и перезапустить контейнеры|
|task migrate-up|Применить все SQL-миграции Goose|
|task migrate-down|Откатить последнюю миграцию|
|task migrate-status|Проверить статус миграций|
|task ps|Проверить статус работающих сервисов|

## Конфигурация

Приложение считывает настройки из YAML-файла (по умолчанию `config.yaml`). 
Пример содержимого `config.yaml.example`:

```yaml
environment: "development" # Варианты: development, production

server:
  host: "0.0.0.0"
  port: "8080"
  read_timeout: 10s
  write_timeout: 10s
  shutdown_timeout: 10s

database:
  dialect: "mysql"
  host: "mysql"
  port: "3306"
  user: "taskuser"
  password: "taskpass"
  name: "taskdb"
  ssl_mode: "disabled"
  max_open_connections: 25
  max_idle_connections: 10
  connection_lifetime: 15m

jwt:
  secret: "your-super-secret-jwt-key-min-8-chars" # Длина не менее 8 символов
  lifespan: 24h

logging:
  level: "info" # Варианты: debug, info, warn, error
  format: "json" # Варианты: json, console

cors:
  allowed_origins:
    - "http://localhost:3000"
    - "http://127.0.0.1:3000"

redis:
  enabled: true
  addr: "127.0.0.1:6379"
  password: ""
  db: 0

cache-services:
  user:
    enabled: true
    ttl: 15m
  task:
    enabled: true
    ttl: 5m
```