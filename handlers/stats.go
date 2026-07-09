package handlers

import (
	"log"
	"net/http"
	"time"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type playerStreaks struct {
	CurrentWinStreak  *int
	CurrentLossStreak *int
	MaxWinStreak      *int
	MaxLossStreak     *int
}

func computePlayerStreaks(db *gorm.DB, sliceID uint) map[uint]playerStreaks {
	const streakQuery = `
		WITH ranked_players AS (
			SELECT
				gp.user_id,
				g.end_time,
				g.winning_team,
				ROW_NUMBER() OVER (PARTITION BY gp.game_id ORDER BY gp.id) AS player_index,
				COUNT(*) OVER (PARTITION BY gp.game_id) AS players_count
			FROM game_players gp
			JOIN games g ON g.id = gp.game_id
			WHERE g.end_time IS NOT NULL AND g.winning_team IS NOT NULL AND g.slice_id = ?
		)
		SELECT
			user_id,
			end_time,
			(CASE WHEN player_index <= (players_count / 2) THEN 1 ELSE 2 END) = winning_team AS won
		FROM ranked_players
		ORDER BY user_id, end_time
	`
	var rawRows []struct {
		UserID  uint      `gorm:"column:user_id"`
		EndTime time.Time `gorm:"column:end_time"`
		Won     bool      `gorm:"column:won"`
	}
	if err := db.Raw(streakQuery, sliceID).Scan(&rawRows).Error; err != nil {
		return nil
	}
	byUser := make(map[uint][]bool)
	for _, row := range rawRows {
		byUser[row.UserID] = append(byUser[row.UserID], row.Won)
	}
	result := make(map[uint]playerStreaks)
	for userID, outcomes := range byUser {
		if len(outcomes) == 0 {
			continue
		}
		var curWin, curLoss, maxWin, maxLoss int
		for _, won := range outcomes {
			if won {
				curWin++
				curLoss = 0
				if curWin > maxWin {
					maxWin = curWin
				}
			} else {
				curLoss++
				curWin = 0
				if curLoss > maxLoss {
					maxLoss = curLoss
				}
			}
		}
		s := playerStreaks{}
		if maxWin > 0 {
			s.MaxWinStreak = &maxWin
		}
		if maxLoss > 0 {
			s.MaxLossStreak = &maxLoss
		}
		if curWin > 0 {
			s.CurrentWinStreak = &curWin
		}
		if curLoss > 0 {
			s.CurrentLossStreak = &curLoss
		}
		result[userID] = s
	}
	return result
}

// teamForPlayerIndex — индекс игрока 0,1 → команда 1; 2,3 → команда 2.
func teamForPlayerIndex(i int) int {
	if i < 2 {
		return 1
	}
	return 2
}

// GetPlayerStats — агрегат по игрокам по завершённым играм разреза (?slice_id=N, по умолчанию глобальный).
// Считается SQL-агрегацией без загрузки всех игр в память.
func GetPlayerStats(c *gin.Context) {
	if writeStatsCacheHit(c) {
		return
	}

	sliceID := sliceIDFromQuery(c)
	db := database.GetDB()
	type playerStatsRow struct {
		UserID             uint   `gorm:"column:user_id"`
		PlayerName         string `gorm:"column:player_name"`
		GamesCount         int    `gorm:"column:games_count"`
		WinsCount          int    `gorm:"column:wins_count"`
		FirstMoveWins      int    `gorm:"column:first_move_wins"`
		FirstMoveGames     int    `gorm:"column:first_move_games"`
		AvgTurnDurationSec int    `gorm:"column:avg_turn_duration_sec"`
		MaxTurnDurationSec int    `gorm:"column:max_turn_duration_sec"`
		BestDeckName       string `gorm:"column:best_deck_name"`
		BestDeckWins       int    `gorm:"column:best_deck_wins"`
		BestDeckGames      int    `gorm:"column:best_deck_games"`
	}

	const query = `
		WITH ranked_players AS (
			SELECT
				gp.game_id,
				gp.user_id,
				gp.deck_id,
				-- Актуальное имя колоды из справочника по deck_id; снимок gp.deck_name — только для удалённых колод.
				COALESCE(d.name, gp.deck_name) AS deck_name,
				u.name AS player_name,
				g.winning_team,
				g.first_move_team,
				ROW_NUMBER() OVER (PARTITION BY gp.game_id ORDER BY gp.id) AS player_index,
				COUNT(*) OVER (PARTITION BY gp.game_id) AS players_count
			FROM game_players gp
			JOIN games g ON g.id = gp.game_id
			JOIN users u ON u.id = gp.user_id
			LEFT JOIN decks d ON d.id = gp.deck_id
			WHERE g.end_time IS NOT NULL
				AND g.winning_team IS NOT NULL
				AND g.slice_id = ?
		),
		players_with_team AS (
			SELECT
				*,
				CASE WHEN player_index <= (players_count / 2) THEN 1 ELSE 2 END AS player_team
			FROM ranked_players
		),
		player_games AS (
			SELECT
				user_id,
				MAX(player_name) AS player_name,
				COUNT(*) AS games_count,
				SUM(CASE WHEN player_team = winning_team THEN 1 ELSE 0 END) AS wins_count,
				SUM(CASE WHEN player_team = first_move_team THEN 1 ELSE 0 END) AS first_move_games,
				SUM(CASE WHEN player_team = first_move_team AND player_team = winning_team THEN 1 ELSE 0 END) AS first_move_wins
			FROM players_with_team
			GROUP BY user_id
		),
		player_turns AS (
			SELECT
				pwt.user_id,
				COALESCE(AVG(gt.duration)::int, 0) AS avg_turn_duration_sec,
				COALESCE(MAX(gt.duration), 0) AS max_turn_duration_sec
			FROM players_with_team pwt
			JOIN game_turns gt ON gt.game_id = pwt.game_id
			WHERE gt.team_number = pwt.player_team
			GROUP BY pwt.user_id
		),
		deck_rates AS (
			SELECT
				user_id,
				deck_id,
				MAX(deck_name) AS deck_name,
				COUNT(*) AS games_count,
				SUM(CASE WHEN player_team = winning_team THEN 1 ELSE 0 END) AS wins_count,
				CASE
					WHEN COUNT(*) > 0
					THEN (SUM(CASE WHEN player_team = winning_team THEN 1 ELSE 0 END)::float / COUNT(*))
					ELSE 0
				END AS win_ratio
			FROM players_with_team
			GROUP BY user_id, deck_id
		),
		best_deck AS (
			SELECT DISTINCT ON (user_id)
				user_id,
				deck_name AS best_deck_name,
				wins_count AS best_deck_wins,
				games_count AS best_deck_games
			FROM deck_rates
			ORDER BY user_id, win_ratio DESC, wins_count DESC, games_count DESC, deck_name ASC
		)
		SELECT
			pg.user_id,
			pg.player_name,
			pg.games_count,
			pg.wins_count,
			pg.first_move_wins,
			pg.first_move_games,
			COALESCE(pt.avg_turn_duration_sec, 0) AS avg_turn_duration_sec,
			COALESCE(pt.max_turn_duration_sec, 0) AS max_turn_duration_sec,
			COALESCE(bd.best_deck_name, '') AS best_deck_name,
			COALESCE(bd.best_deck_wins, 0) AS best_deck_wins,
			COALESCE(bd.best_deck_games, 0) AS best_deck_games
		FROM player_games pg
		LEFT JOIN player_turns pt ON pt.user_id = pg.user_id
		LEFT JOIN best_deck bd ON bd.user_id = pg.user_id
		ORDER BY pg.player_name ASC
	`
	var rows []playerStatsRow
	if err := db.Raw(query, sliceID).Scan(&rows).Error; err != nil {
		log.Printf("GetPlayerStats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить статистику игроков"})
		return
	}

	streaks := computePlayerStreaks(db, sliceID)

	out := make([]models.PlayerStats, 0, len(rows))
	for _, r := range rows {
		winPct := 0.0
		if r.GamesCount > 0 {
			winPct = float64(r.WinsCount) / float64(r.GamesCount) * 100
		}
		firstMovePct := 0.0
		if r.FirstMoveGames > 0 {
			firstMovePct = float64(r.FirstMoveWins) / float64(r.FirstMoveGames) * 100
		}

		stat := models.PlayerStats{
			PlayerName:          r.PlayerName,
			GamesCount:          r.GamesCount,
			WinsCount:           r.WinsCount,
			WinPercent:          winPct,
			FirstMoveWins:       r.FirstMoveWins,
			FirstMoveGames:      r.FirstMoveGames,
			FirstMoveWinPercent: firstMovePct,
			AvgTurnDurationSec:  r.AvgTurnDurationSec,
			MaxTurnDurationSec:  r.MaxTurnDurationSec,
			BestDeckName:        r.BestDeckName,
			BestDeckWins:        r.BestDeckWins,
			BestDeckGames:       r.BestDeckGames,
		}
		if s, ok := streaks[r.UserID]; ok {
			stat.CurrentWinStreak = s.CurrentWinStreak
			stat.CurrentLossStreak = s.CurrentLossStreak
			stat.MaxWinStreak = s.MaxWinStreak
			stat.MaxLossStreak = s.MaxLossStreak
		}
		out = append(out, stat)
	}
	writeStatsCacheJSON(c, out)
}

// GetDeckStats — агрегат по колодам по завершённым играм разреза (?slice_id=N, по умолчанию глобальный).
// games_count — число уникальных партий с колодой (COUNT DISTINCT game_id).
func GetDeckStats(c *gin.Context) {
	if writeStatsCacheHit(c) {
		return
	}

	sliceID := sliceIDFromQuery(c)
	db := database.GetDB()
	type deckStatsRow struct {
		DeckID     int    `gorm:"column:deck_id"`
		DeckName   string `gorm:"column:deck_name"`
		GamesCount int    `gorm:"column:games_count"`
		WinsCount  int    `gorm:"column:wins_count"`
	}
	var rows []deckStatsRow
	query := `
		WITH ranked_players AS (
			SELECT
				gp.game_id,
				gp.deck_id,
				-- Актуальное имя колоды из справочника по deck_id; снимок gp.deck_name — только для удалённых колод.
				COALESCE(d.name, gp.deck_name) AS deck_name,
				g.winning_team,
				ROW_NUMBER() OVER (PARTITION BY gp.game_id ORDER BY gp.id) AS player_index,
				COUNT(*) OVER (PARTITION BY gp.game_id) AS players_count
			FROM game_players gp
			JOIN games g ON g.id = gp.game_id
			LEFT JOIN decks d ON d.id = gp.deck_id
			WHERE g.end_time IS NOT NULL
				AND g.winning_team IS NOT NULL
				AND g.slice_id = ?
		),
		players_with_team AS (
			SELECT
				*,
				CASE WHEN player_index <= (players_count / 2) THEN 1 ELSE 2 END AS player_team
			FROM ranked_players
		),
		deck_per_game AS (
			SELECT
				deck_id,
				MAX(deck_name) AS deck_name,
				game_id,
				MAX(CASE WHEN player_team = winning_team THEN 1 ELSE 0 END) AS won
			FROM players_with_team
			GROUP BY deck_id, game_id
		)
		SELECT
			deck_id,
			MAX(deck_name) AS deck_name,
			COUNT(*) AS games_count,
			SUM(won) AS wins_count
		FROM deck_per_game
		GROUP BY deck_id
		ORDER BY deck_name ASC
	`
	if err := db.Raw(query, sliceID).Scan(&rows).Error; err != nil {
		log.Printf("GetDeckStats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить статистику колод"})
		return
	}

	out := make([]models.DeckStats, 0, len(rows))
	for _, r := range rows {
		pct := 0.0
		if r.GamesCount > 0 {
			pct = float64(r.WinsCount) / float64(r.GamesCount) * 100
		}
		out = append(out, models.DeckStats{
			DeckID:     r.DeckID,
			DeckName:   r.DeckName,
			GamesCount: r.GamesCount,
			WinsCount:  r.WinsCount,
			WinPercent: pct,
		})
	}
	writeStatsCacheJSON(c, out)
}

func writeStatsCacheHit(c *gin.Context) bool {
	if payload, ok := statsResponseCache.Get(c.Request.URL.RequestURI()); ok {
		c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
		return true
	}
	return false
}

func writeStatsCacheJSON(c *gin.Context, payload interface{}) {
	key := c.Request.URL.RequestURI()
	if err := statsResponseCache.SetJSON(key, payload); err != nil {
		c.JSON(http.StatusOK, payload)
		return
	}

	bytesPayload, ok := statsResponseCache.Get(key)
	if !ok {
		c.JSON(http.StatusOK, payload)
		return
	}
	// Отдаём тот же сериализованный байтовый массив, который хранится в кэше.
	c.Data(http.StatusOK, "application/json; charset=utf-8", bytesPayload)
}
