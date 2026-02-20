# MTG Stats Backend

REST API для приложения учёта статистики партий Magic: The Gathering. Управление пользователями, колодами, играми и экспорт/импорт данных.

## Структура проекта

```
mtg-stats-backend/
├── main.go                 # Точка входа: роутер Gin, CORS, middleware, статика
├── go.mod / go.sum         # Зависимости Go
├── .env                    # Переменные окружения (не в git)
│
├── cmd/
│   └── hashpass/
│       └── main.go         # CLI для генерации bcrypt-хешей паролей
│
├── database/
│   └── database.go        # Подключение PostgreSQL, пул соединений, AutoMigrate
│
├── handlers/               # HTTP-обработчики API
│   ├── auth_handler.go    # POST /api/auth/login (JWT, rate limiting)
│   ├── user_handler.go    # CRUD пользователей
│   ├── deck_handler.go    # CRUD колод, загрузка/удаление изображений
│   ├── games.go           # CRUD игр, активная игра, ходы, пауза/возобновление
│   ├── stats.go           # Статистика игроков и колод
│   └── export.go          # Экспорт/импорт всех данных (gzip JSON)
│
├── middleware/
│   ├── jwt.go             # BearerOrJWTAuth, RequireUser, RequireAdmin
│   └── https.go            # RequireHTTPS в production
│
├── models/                # Доменные модели и DTO
│   ├── user.go            # User, UserRequest, UserClaims (JWT)
│   ├── deck.go            # Deck, DeckRequest
│   └── game.go            # Game, GamePlayer, GameTurn, запросы/ответы
│
└── scripts/
    └── renumber_game_ids.sql  # Скрипт перенумерации ID игр
```

## Архитектура

- **Фреймворк:** Gin
- **ORM:** GORM
- **БД:** PostgreSQL
- **Авторизация:** Bearer API_TOKEN или JWT (из `/api/auth/login`)

## API

### Аутентификация
- `POST /api/auth/login` — вход (name, password) → JWT. Rate limit: 5 попыток/мин с IP.

### Пользователи
- `GET /api/users` — список
- `POST /api/users` — создать (только админ)
- `GET /api/users/:id` — получить
- `PUT /api/users/:id` — обновить (админ — любого; пользователь — себя)
- `DELETE /api/users/:id` — удалить (только админ)

### Колоды
- `GET /api/decks`, `GET /api/decks/:id` — чтение
- `POST /api/decks`, `PUT /api/decks/:id` — создать/обновить (только админ)
- `POST /api/decks/:id/image` — загрузить image и avatar (multipart; только админ)
- `DELETE /api/decks/:id/image`, `DELETE /api/decks/:id` — удалить (только админ)

### Игры
- `GET /api/games`, `GET /api/games/:id`, `GET /api/games/active` — чтение
- `POST /api/games` — создать (только админ)
- `PUT /api/games/active` — обновить активную (только админ)
- `POST /api/games/active/pause`, `POST /api/games/active/resume` — пауза (только админ)
- `POST /api/games/active/start-turn` — начать ход (только админ)
- `POST /api/games/active/finish` — завершить (только админ)
- `DELETE /api/games` — полная очистка игр (только админ)

### Статистика
- `GET /api/stats/players`, `GET /api/stats/decks` — чтение

### Экспорт/импорт
- `GET /api/export/all` — экспорт в gzip JSON. По умолчанию без паролей; `?include_passwords=true` — с хешами паролей
- `POST /api/import/all` — полная замена данных из gzip JSON

### Health
- `GET /health` — проверка состояния (ping БД)

## Переменные окружения

| Переменная | Назначение |
|------------|------------|
| `LOCAL_DSN` | DSN PostgreSQL для локальной разработки |
| `DATABASE_URL` | DSN PostgreSQL для production |
| `API_TOKEN` | **Обязателен.** Bearer-токен для доступа к API; fallback для JWT_SECRET |
| `JWT_SECRET` | Секрет для подписи JWT |
| `PORT` | Порт сервера (по умолчанию 8080) |
| `GIN_MODE` | debug / release |
| `CORS_ALLOWED_ORIGINS` | Разрешённые CORS origins (через запятую) |
| `UPLOAD_DIR` | Директория загрузок (по умолчанию ./uploads) |

## Запуск

```bash
# Локально (LOCAL_DSN в .env)
go run .

# Production
DATABASE_URL=postgres://... PORT=8080 go run .
```

## Генерация хеша пароля

```bash
go run ./cmd/hashpass
```
