package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
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

func computePlayerStreaks(db *gorm.DB) map[uint]playerStreaks {
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
			WHERE g.end_time IS NOT NULL AND g.winning_team IS NOT NULL
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
	if err := db.Raw(streakQuery).Scan(&rawRows).Error; err != nil {
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

// GetPlayerStats — агрегат по игрокам по завершённым играм (победы, ходы, лучшая колода).
// Считается SQL-агрегацией без загрузки всех игр в память.
func GetPlayerStats(c *gin.Context) {
	if writeStatsCacheHit(c) {
		return
	}

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
				gp.deck_name,
				u.name AS player_name,
				g.winning_team,
				g.first_move_team,
				ROW_NUMBER() OVER (PARTITION BY gp.game_id ORDER BY gp.id) AS player_index,
				COUNT(*) OVER (PARTITION BY gp.game_id) AS players_count
			FROM game_players gp
			JOIN games g ON g.id = gp.game_id
			JOIN users u ON u.id = gp.user_id
			WHERE g.end_time IS NOT NULL
				AND g.winning_team IS NOT NULL
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
	if err := db.Raw(query).Scan(&rows).Error; err != nil {
		log.Printf("GetPlayerStats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить статистику игроков"})
		return
	}

	streaks := computePlayerStreaks(db)

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

// GetDeckStats — агрегат по колодам по завершённым играм (игры, победы, %).
// Загружает все завершённые игры в память; при большом объёме данных рассмотреть SQL-агрегацию.
func GetDeckStats(c *gin.Context) {
	if writeStatsCacheHit(c) {
		return
	}

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
				gp.deck_id,
				gp.deck_name,
				g.winning_team,
				ROW_NUMBER() OVER (PARTITION BY gp.game_id ORDER BY gp.id) AS player_index,
				COUNT(*) OVER (PARTITION BY gp.game_id) AS players_count
			FROM game_players gp
			JOIN games g ON g.id = gp.game_id
			WHERE g.end_time IS NOT NULL
				AND g.winning_team IS NOT NULL
		)
		SELECT
			deck_id,
			MAX(deck_name) AS deck_name,
			COUNT(*) AS games_count,
			SUM(
				CASE
					WHEN (CASE WHEN player_index <= (players_count / 2) THEN 1 ELSE 2 END) = winning_team
					THEN 1 ELSE 0
				END
			) AS wins_count
		FROM ranked_players
		GROUP BY deck_id
	`
	if err := db.Raw(query).Scan(&rows).Error; err != nil {
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

func periodKey(t time.Time, groupBy string) string {
	u := t.UTC()
	switch groupBy {
	case "day":
		return u.Format("2006-01-02")
	case "month":
		return u.Format("2006-01")
	default:
		year, week := u.ISOWeek()
		return strings.Join([]string{strconv.Itoa(year), "W", strconv.Itoa(week)}, "")
	}
}

// GetDeckMatchups — матрица матчапов колод по завершённым играм.
func GetDeckMatchups(c *gin.Context) {
	if writeStatsCacheHit(c) {
		return
	}

	db := database.GetDB()
	type deckMatchupRow struct {
		Deck1ID    int    `gorm:"column:deck1_id"`
		Deck1Name  string `gorm:"column:deck1_name"`
		Deck2ID    int    `gorm:"column:deck2_id"`
		Deck2Name  string `gorm:"column:deck2_name"`
		GamesCount int    `gorm:"column:games_count"`
		Deck1Wins  int    `gorm:"column:deck1_wins"`
		Deck2Wins  int    `gorm:"column:deck2_wins"`
	}
	const query = `
		WITH ranked_players AS (
			SELECT
				gp.game_id,
				gp.deck_id,
				gp.deck_name,
				g.winning_team,
				ROW_NUMBER() OVER (PARTITION BY gp.game_id ORDER BY gp.id) AS player_index,
				COUNT(*) OVER (PARTITION BY gp.game_id) AS players_count
			FROM game_players gp
			JOIN games g ON g.id = gp.game_id
			WHERE g.end_time IS NOT NULL
				AND g.winning_team IS NOT NULL
		),
		players_with_team AS (
			SELECT
				*,
				CASE WHEN player_index <= (players_count / 2) THEN 1 ELSE 2 END AS team_number
			FROM ranked_players
		),
		cross_pairs AS (
			SELECT
				p1.deck_id AS raw_deck1_id,
				p1.deck_name AS raw_deck1_name,
				p2.deck_id AS raw_deck2_id,
				p2.deck_name AS raw_deck2_name,
				p1.winning_team
			FROM players_with_team p1
			JOIN players_with_team p2
				ON p1.game_id = p2.game_id
				AND p1.team_number = 1
				AND p2.team_number = 2
		),
		normalized AS (
			SELECT
				CASE WHEN raw_deck1_id <= raw_deck2_id THEN raw_deck1_id ELSE raw_deck2_id END AS deck1_id,
				CASE WHEN raw_deck1_id <= raw_deck2_id THEN raw_deck1_name ELSE raw_deck2_name END AS deck1_name,
				CASE WHEN raw_deck1_id <= raw_deck2_id THEN raw_deck2_id ELSE raw_deck1_id END AS deck2_id,
				CASE WHEN raw_deck1_id <= raw_deck2_id THEN raw_deck2_name ELSE raw_deck1_name END AS deck2_name,
				CASE
					WHEN raw_deck1_id <= raw_deck2_id
						THEN CASE WHEN winning_team = 1 THEN 1 ELSE 0 END
					ELSE CASE WHEN winning_team = 2 THEN 1 ELSE 0 END
				END AS deck1_win
			FROM cross_pairs
		)
		SELECT
			deck1_id,
			MAX(deck1_name) AS deck1_name,
			deck2_id,
			MAX(deck2_name) AS deck2_name,
			COUNT(*) AS games_count,
			SUM(deck1_win) AS deck1_wins,
			COUNT(*) - SUM(deck1_win) AS deck2_wins
		FROM normalized
		GROUP BY deck1_id, deck2_id
	`
	var rows []deckMatchupRow
	if err := db.Raw(query).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить матрицу матчапов"})
		return
	}

	out := make([]models.DeckMatchupStats, 0, len(rows))
	for _, a := range rows {
		deck1Rate := 0.0
		deck2Rate := 0.0
		if a.GamesCount > 0 {
			deck1Rate = float64(a.Deck1Wins) / float64(a.GamesCount) * 100
			deck2Rate = float64(a.Deck2Wins) / float64(a.GamesCount) * 100
		}
		out = append(out, models.DeckMatchupStats{
			Deck1ID:      a.Deck1ID,
			Deck1Name:    a.Deck1Name,
			Deck2ID:      a.Deck2ID,
			Deck2Name:    a.Deck2Name,
			GamesCount:   a.GamesCount,
			Deck1Wins:    a.Deck1Wins,
			Deck2Wins:    a.Deck2Wins,
			Deck1WinRate: deck1Rate,
			Deck2WinRate: deck2Rate,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Deck1Name != out[j].Deck1Name {
			return out[i].Deck1Name < out[j].Deck1Name
		}
		return out[i].Deck2Name < out[j].Deck2Name
	})
	writeStatsCacheJSON(c, models.DeckMatchupsResponse{Matchups: out})
}

// GetMetaDashboard — мета-срез по времени с агрегатами колод.
func GetMetaDashboard(c *gin.Context) {
	if writeStatsCacheHit(c) {
		return
	}

	groupBy := c.DefaultQuery("group_by", "week")
	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_by должен быть day|week|month"})
		return
	}

	var fromDate *time.Time
	var toDate *time.Time
	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный from, формат YYYY-MM-DD"})
			return
		}
		u := t.UTC()
		fromDate = &u
	}
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный to, формат YYYY-MM-DD"})
			return
		}
		end := t.Add(24*time.Hour - time.Nanosecond).UTC()
		toDate = &end
	}
	if fromDate != nil && toDate != nil && fromDate.After(*toDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from должен быть <= to"})
		return
	}

	db := database.GetDB()
	whereClause, whereArgs := buildCompletedGamesWhereClause("g", fromDate, toDate)
	periodExpr := periodKeySQLExpr("g.start_time", groupBy)

	type deckAgg struct {
		id    int
		name  string
		games int
		wins  int
	}
	type periodDeckRow struct {
		Period     string `gorm:"column:period"`
		DeckID     int    `gorm:"column:deck_id"`
		DeckName   string `gorm:"column:deck_name"`
		GamesCount int    `gorm:"column:games_count"`
		WinsCount  int    `gorm:"column:wins_count"`
	}
	type periodTotalRow struct {
		Period     string `gorm:"column:period"`
		TotalGames int    `gorm:"column:total_games"`
	}

	var totalGames int64
	if err := db.Table("games AS g").Where(whereClause, whereArgs...).Count(&totalGames).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить игры для мета-дашборда"})
		return
	}

	periodTotalsQuery := fmt.Sprintf(`
		SELECT
			%s AS period,
			COUNT(*) AS total_games
		FROM games g
		WHERE %s
		GROUP BY period
	`, periodExpr, whereClause)
	var periodTotals []periodTotalRow
	if err := db.Raw(periodTotalsQuery, whereArgs...).Scan(&periodTotals).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось рассчитать периоды мета-дашборда"})
		return
	}

	periodDecksQuery := fmt.Sprintf(`
		WITH ranked_players AS (
			SELECT
				%s AS period,
				gp.deck_id,
				gp.deck_name,
				g.winning_team,
				ROW_NUMBER() OVER (PARTITION BY gp.game_id ORDER BY gp.id) AS player_index,
				COUNT(*) OVER (PARTITION BY gp.game_id) AS players_count
			FROM game_players gp
			JOIN games g ON g.id = gp.game_id
			WHERE %s
		)
		SELECT
			period,
			deck_id,
			MAX(deck_name) AS deck_name,
			COUNT(*) AS games_count,
			SUM(
				CASE
					WHEN (CASE WHEN player_index <= (players_count / 2) THEN 1 ELSE 2 END) = winning_team
					THEN 1 ELSE 0
				END
			) AS wins_count
		FROM ranked_players
		GROUP BY period, deck_id
	`, periodExpr, whereClause)
	var periodDeckRows []periodDeckRow
	if err := db.Raw(periodDecksQuery, whereArgs...).Scan(&periodDeckRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось рассчитать статистику колод для мета-дашборда"})
		return
	}

	allDecks := make(map[int]*deckAgg)
	periodDecks := make(map[string]map[int]*deckAgg, len(periodTotals))
	periodTotalsByKey := make(map[string]int, len(periodTotals))
	for _, r := range periodTotals {
		periodTotalsByKey[r.Period] = r.TotalGames
		periodDecks[r.Period] = make(map[int]*deckAgg)
	}
	for _, r := range periodDeckRows {
		if allDecks[r.DeckID] == nil {
			allDecks[r.DeckID] = &deckAgg{id: r.DeckID, name: r.DeckName}
		}
		allDecks[r.DeckID].games += r.GamesCount
		allDecks[r.DeckID].wins += r.WinsCount

		if periodDecks[r.Period] == nil {
			periodDecks[r.Period] = make(map[int]*deckAgg)
		}
		periodDecks[r.Period][r.DeckID] = &deckAgg{
			id:    r.DeckID,
			name:  r.DeckName,
			games: r.GamesCount,
			wins:  r.WinsCount,
		}
	}

	toMetaDeck := func(a *deckAgg, total int) models.MetaDeckStat {
		winRate := 0.0
		metaShare := 0.0
		if a.games > 0 {
			winRate = float64(a.wins) / float64(a.games) * 100
		}
		if total > 0 {
			metaShare = float64(a.games) / float64(total) * 100
		}
		return models.MetaDeckStat{
			DeckID:     a.id,
			DeckName:   a.name,
			GamesCount: a.games,
			WinsCount:  a.wins,
			WinRate:    winRate,
			MetaShare:  metaShare,
		}
	}

	topPlayed := make([]models.MetaDeckStat, 0, len(allDecks))
	topWinRate := make([]models.MetaDeckStat, 0, len(allDecks))
	for _, a := range allDecks {
		stat := toMetaDeck(a, int(totalGames))
		topPlayed = append(topPlayed, stat)
		if stat.GamesCount >= 3 {
			topWinRate = append(topWinRate, stat)
		}
	}
	sort.Slice(topPlayed, func(i, j int) bool {
		if topPlayed[i].GamesCount != topPlayed[j].GamesCount {
			return topPlayed[i].GamesCount > topPlayed[j].GamesCount
		}
		return topPlayed[i].DeckName < topPlayed[j].DeckName
	})
	sort.Slice(topWinRate, func(i, j int) bool {
		if topWinRate[i].WinRate != topWinRate[j].WinRate {
			return topWinRate[i].WinRate > topWinRate[j].WinRate
		}
		return topWinRate[i].DeckName < topWinRate[j].DeckName
	})
	if len(topPlayed) > 10 {
		topPlayed = topPlayed[:10]
	}
	if len(topWinRate) > 10 {
		topWinRate = topWinRate[:10]
	}

	periodKeys := make([]string, 0, len(periodTotalsByKey))
	for key := range periodTotalsByKey {
		periodKeys = append(periodKeys, key)
	}
	sort.Strings(periodKeys)

	periodOut := make([]models.MetaPeriodStats, 0, len(periodKeys))
	for _, key := range periodKeys {
		total := periodTotalsByKey[key]
		decks := make([]models.MetaDeckStat, 0, len(periodDecks[key]))
		for _, a := range periodDecks[key] {
			decks = append(decks, toMetaDeck(a, total))
		}
		sort.Slice(decks, func(i, j int) bool {
			if decks[i].GamesCount != decks[j].GamesCount {
				return decks[i].GamesCount > decks[j].GamesCount
			}
			return decks[i].DeckName < decks[j].DeckName
		})
		periodOut = append(periodOut, models.MetaPeriodStats{
			Period:     key,
			TotalGames: total,
			Decks:      decks,
		})
	}

	resp := models.MetaDashboardResponse{
		GroupBy:         groupBy,
		TotalGames:      int(totalGames),
		UniqueDecks:     len(allDecks),
		TopPlayedDecks:  topPlayed,
		TopWinRateDecks: topWinRate,
		Periods:         periodOut,
	}
	if fromDate != nil {
		resp.FromDate = fromDate.Format("2006-01-02")
	}
	if toDate != nil {
		resp.ToDate = toDate.Format("2006-01-02")
	}
	writeStatsCacheJSON(c, resp)
}

func periodKeySQLExpr(column, groupBy string) string {
	switch groupBy {
	case "day":
		return fmt.Sprintf("to_char((%s AT TIME ZONE 'UTC'), 'YYYY-MM-DD')", column)
	case "month":
		return fmt.Sprintf("to_char((%s AT TIME ZONE 'UTC'), 'YYYY-MM')", column)
	default:
		return fmt.Sprintf("to_char((%s AT TIME ZONE 'UTC'), 'IYYY') || 'W' || to_char((%s AT TIME ZONE 'UTC'), 'IW')", column, column)
	}
}

func buildCompletedGamesWhereClause(alias string, fromDate, toDate *time.Time) (string, []interface{}) {
	where := fmt.Sprintf("%s.end_time IS NOT NULL AND %s.winning_team IS NOT NULL", alias, alias)
	args := make([]interface{}, 0, 2)
	if fromDate != nil {
		where += fmt.Sprintf(" AND %s.start_time >= ?", alias)
		args = append(args, *fromDate)
	}
	if toDate != nil {
		where += fmt.Sprintf(" AND %s.start_time <= ?", alias)
		args = append(args, *toDate)
	}
	return where, args
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
