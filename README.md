# MTG Stats Backend

## Конфигурация

### Авторизация

Все эндпоинты `/api/*` принимают **либо** `API_TOKEN`, **либо** JWT (от входа пользователя).

| Переменная   | Описание |
|--------------|----------|
| `API_TOKEN`  | Секретный токен для доступа к API. Передавайте: `Authorization: Bearer <API_TOKEN>`. |
| `JWT_SECRET` | Секрет для подписи JWT (если не задан — используется `API_TOKEN`). |

**Два способа авторизации:**
1. **API_TOKEN** — полный доступ к чтению; для создания/изменения/удаления пользователей требуется JWT.
2. **JWT** — получается через `POST /api/auth/login` (name, password). Используется для операций с пользователями с проверкой прав.

**Права доступа:**
- `POST /api/users`, `DELETE /api/users/:id` — только администратор (is_admin).
- `PUT /api/users/:id` — администратор может менять любого; пользователь — только себя (имя, пароль). Признак `is_admin` меняет только администратор.

**Публичные маршруты (без токена):** `GET /`, `GET /health`, `GET /uploads/*`

**Пример входа и запроса с JWT:**
```bash
# Вход (с API_TOKEN)
curl -X POST -H "Authorization: Bearer your-api-token" -H "Content-Type: application/json" \
  -d '{"name":"admin","password":"secret"}' https://your-api.example.com/api/auth/login

# Ответ: {"token":"eyJ...","user":{...}}
# Далее используйте token в запросах:
curl -H "Authorization: Bearer <JWT>" https://your-api.example.com/api/users
```

**HTTPS:** в production (без `LOCAL_DSN`) запросы по HTTP отклоняются. Учитывается заголовок `X-Forwarded-Proto` при работе за reverse proxy.

---

#### Локальный сервер

1. Создайте файл `.env` в корне проекта (или экспортируйте переменные в shell):
   ```
   LOCAL_DSN=postgres://user:password@localhost:5432/mtg_stats?sslmode=disable
   API_TOKEN=your-local-dev-token
   ```

2. Для разработки можно **оставить `API_TOKEN` пустым** — тогда все запросы к API будут проходить без проверки.

3. Запуск: `go run .` (порт по умолчанию 8080, либо `PORT=3000 go run .`).

---

#### Production (antonrogovikov.duckdns.org)

1. На сервере задайте переменные окружения:
   - `API_TOKEN` — сгенерируйте длинный случайный токен (например, `openssl rand -hex 32`).
   - `DATABASE_URL` — строка подключения PostgreSQL.
   - `UPLOAD_DIR` — путь для хранения изображений (например, `/data`).
   - `CORS_ALLOWED_ORIGINS` — список доменов фронтенда через запятую, например: `https://antonrogovikov.duckdns.org,http://localhost:5173`.

2. **Важно:** в production всегда задавайте `API_TOKEN`, иначе API будет доступен без авторизации.

3. На фронтенде сохраняйте токен в переменных окружения (например, `VITE_API_TOKEN` для Vite) и добавляйте заголовок ко всем запросам:
   ```js
   headers: { 'Authorization': `Bearer ${import.meta.env.VITE_API_TOKEN}` }
   ```

---

### Остальные переменные

- **База данных:** `DATABASE_URL` (PostgreSQL) или `LOCAL_DSN` для локального запуска.
- **Изображения колод** сохраняются на диск. Корень каталога задаётся переменной **`UPLOAD_DIR`**:
  - Локально по умолчанию: `./uploads`
  - **Production:** задайте `UPLOAD_DIR=/data` (или другой путь) для хранения изображений на диске. 

## Основные эндпоинты

### Аутентификация
- `POST /api/auth/login` — вход (name, password) → JWT. Rate limit: 5 попыток/мин с IP.

### Пользователи
- `GET /api/users` — список пользователей
- `POST /api/users` — создать пользователя (только админ)
- `GET /api/users/:id` — получить пользователя
- `PUT /api/users/:id` — обновить пользователя (админ — любого; пользователь — себя)
- `DELETE /api/users/:id` — удалить пользователя (только админ)

### Колоды
- `GET /api/decks` — список колод
- `POST /api/decks` — создать колоду
- `GET /api/decks/:id` — получить колоду
- `PUT /api/decks/:id` — обновить колоду
- `POST /api/decks/:id/image` — загрузить изображение и аватар колоды (multipart: поля `image` и `avatar`)
- `DELETE /api/decks/:id/image` — удалить изображение и аватар колоды
- `DELETE /api/decks/:id` — удалить колоду

### Игры
- `GET /api/games` — список игр
- `POST /api/games` — создать игру
- `GET /api/games/:id` — получить игру
- `GET /api/games/active` — активная игра
- `PUT /api/games/active` — обновить активную игру
- `POST /api/games/active/finish` — завершить активную игру
- `DELETE /api/games` — полная очистка таблиц игр и ходов

### Статистика
- `GET /api/stats/players` — статистика игроков
- `GET /api/stats/decks` — статистика колод

### Экспорт данных
- `GET /api/export/all` — экспорт всех данных (пользователи, колоды, игры с игроками и ходами, изображения колод в base64) в **gzip-архиве JSON**. 
  - Ответ содержит заголовки `Content-Encoding: gzip`, `Content-Type: application/json`.
- `POST /api/import/all` — полная замена всех данных БД и файлов изображений по **gzip-архиву JSON**, полученному из `GET /api/export/all`.
