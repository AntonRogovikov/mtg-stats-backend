# MTG Stats Backend

## Конфигурация

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
