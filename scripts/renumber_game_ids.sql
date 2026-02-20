-- Перенумерация id партий (games): 16, 17, 18... -> 1, 2, 3...
-- Обновляет games, game_players и game_turns для согласованности.
-- Выполнять в транзакции (откат при ошибке).

BEGIN;

-- 0. Временно снимаем все FK, ссылающиеся на games (PostgreSQL не даёт менять PK при активных ссылках)
DO $$
DECLARE r RECORD;
BEGIN
  FOR r IN SELECT c.conname, (SELECT relname FROM pg_class WHERE oid = c.conrelid) AS tbl
           FROM pg_constraint c WHERE c.confrelid = 'games'::regclass AND c.contype = 'f'
  LOOP
    EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tbl, r.conname);
  END LOOP;
END $$;

-- 1. Создаём маппинг старый_id -> новый_id (1, 2, 3... по порядку)
CREATE TEMP TABLE game_id_map AS
SELECT id AS old_id, ROW_NUMBER() OVER (ORDER BY id)::int AS new_id FROM games;

-- 2. Смещаем id в games во временный диапазон, чтобы избежать конфликтов
--    (10000 + old_id; при необходимости увеличьте, если id > 9000)
UPDATE games SET id = id + 10000;

-- 3. Обновляем ссылки в дочерних таблицах на смещённые id
UPDATE game_players gp SET game_id = gp.game_id + 10000;
UPDATE game_turns gt SET game_id = gt.game_id + 10000;

-- 4. Переназначаем id в games на 1, 2, 3...
UPDATE games g SET id = m.new_id
FROM game_id_map m WHERE g.id = m.old_id + 10000;

-- 5. Обновляем ссылки в дочерних таблицах на новые id
UPDATE game_players gp SET game_id = m.new_id
FROM game_id_map m WHERE gp.game_id = m.old_id + 10000;

UPDATE game_turns gt SET game_id = m.new_id
FROM game_id_map m WHERE gt.game_id = m.old_id + 10000;

-- 6. Сбрасываем sequence для games.id (чтобы новые записи получали id > max)
SELECT setval(
  pg_get_serial_sequence('games', 'id'),
  COALESCE((SELECT MAX(id) FROM games), 1)
);

-- 7. Восстанавливаем FK-ограничения (game_players и game_turns ссылаются на games.id)
ALTER TABLE game_players ADD CONSTRAINT fk_games_players FOREIGN KEY (game_id) REFERENCES games(id);
ALTER TABLE game_turns ADD CONSTRAINT fk_game_turns_game_id FOREIGN KEY (game_id) REFERENCES games(id);

COMMIT;
