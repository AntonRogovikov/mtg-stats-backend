# MTG Stats Backend

## Конфигурация

### Авторизация по токену (API_TOKEN)

Все эндпоинты `/api/*` защищены Bearer-токеном, если задана переменная **`API_TOKEN`**.

| Переменная  | Описание |
|-------------|----------|
| `API_TOKEN` | Секретный токен. При запросах к API передавайте: `Authorization: Bearer <API_TOKEN>`. Если не задан — авторизация отключена. |

**Публичные маршруты (без токена):** `GET /`, `GET /health`, `GET /uploads/*`

**Пример запроса с токеном:**
```bash
curl -H "Authorization: Bearer your-secret-token" https://your-api.example.com/api/users
```

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

#### Railway (production)

1. В **Railway Dashboard** → ваш проект → **Variables** добавьте:
   - `API_TOKEN` — сгенерируйте длинный случайный токен (например, `openssl rand -hex 32`).
   - `DATABASE_URL` — подключается автоматически при добавлении PostgreSQL.
   - `UPLOAD_DIR=/data` — если используете Volume для файлов.
   - `CORS_ALLOWED_ORIGINS` — список доменов фронтенда через запятую, например: `https://myapp.vercel.app,https://myapp.com`.

2. **Важно:** на Railway всегда задавайте `API_TOKEN`, иначе API будет доступен без авторизации.

3. На фронтенде сохраняйте токен в переменных окружения (например, `VITE_API_TOKEN` для Vite) и добавляйте заголовок ко всем запросам:
   ```js
   headers: { 'Authorization': `Bearer ${import.meta.env.VITE_API_TOKEN}` }
   ```

---

### Остальные переменные

- **База данных:** `DATABASE_URL` (PostgreSQL) или `LOCAL_DSN` для локального запуска.
- **Изображения колод** сохраняются на диск. Корень каталога задаётся переменной **`UPLOAD_DIR`**:
  - Локально по умолчанию: `./uploads`
  - **Railway с Volume:** создайте Volume в Railway, примонтируйте в `/data` и задайте `UPLOAD_DIR=/data`. 

## Основные эндпоинты

### Пользователи
- `GET /api/users` — список пользователей
- `POST /api/users` — создать пользователя
- `GET /api/users/:id` — получить пользователя
- `PUT /api/users/:id` — обновить пользователя
- `DELETE /api/users/:id` — удалить пользователя

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
