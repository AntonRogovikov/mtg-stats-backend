# MTG Stats Backend

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
- `POST /api/decks` — создать колоду (только админ)
- `GET /api/decks/:id` — получить колоду
- `PUT /api/decks/:id` — обновить колоду (только админ)
- `POST /api/decks/:id/image` — загрузить изображение и аватар колоды (только админ; multipart: поля `image` и `avatar`)
- `DELETE /api/decks/:id/image` — удалить изображение и аватар колоды (только админ)
- `DELETE /api/decks/:id` — удалить колоду (только админ)

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
