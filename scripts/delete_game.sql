-- Удаление игры и всех связанных данных (game_turns, game_players).
--
-- Запуск: psql $DATABASE_URL -v game_id=42 -f delete_game.sql
-- (замените 42 на нужный ID игры)
--
-- После удаления ID новых игр продолжают sequence (например, 1,2,5,100 -> новая игра получит 101).
-- Чтобы перенумеровать игры в 1,2,3... и сбросить sequence, выполните:
--   psql $DATABASE_URL -f scripts/renumber_game_ids.sql

BEGIN;

DELETE FROM game_turns   WHERE game_id = :game_id;
DELETE FROM game_players WHERE game_id = :game_id;
DELETE FROM games       WHERE id       = :game_id;

COMMIT;
