package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
)

// teamForPlayerIndex — индекс игрока 0,1 → команда 1; 2,3 → команда 2.
func teamForPlayerIndex(i int) int {
	if i < 2 {
		return 1
	}
	return 2
}

// GetPlayerStats — агрегат по игрокам по завершённым играм (победы, ходы, лучшая колода).
// Загружает все завершённые игры в память; при большом объёме данных рассмотреть SQL-агрегацию.
func GetPlayerStats(c *gin.Context) {
	db := database.GetDB()
	var games []models.Game
	if err := db.Where("end_time IS NOT NULL").Preload("Players.User").Preload("Turns").Find(&games).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список игр"})
		return
	}

	type agg struct {
		playerName     string
		games          int
		wins           int
		firstMoveGames int
		firstMoveWins  int
		turnDurations  []int
		deckWins       map[int]int
		deckGames      map[int]int
		deckNames      map[int]string
	}
	byUser := make(map[uint]*agg)

	for _, g := range games {
		if g.WinningTeam == nil {
			continue
		}
		winTeam := *g.WinningTeam
		firstMove := g.FirstMoveTeam

		for i, p := range g.Players {
			uid := p.User.ID
			if byUser[uid] == nil {
				byUser[uid] = &agg{
					playerName: p.User.Name,
					deckWins:   make(map[int]int),
					deckGames:  make(map[int]int),
					deckNames:  make(map[int]string),
				}
			}
			a := byUser[uid]
			a.games++
			myTeam := teamForPlayerIndex(i)
			if myTeam == winTeam {
				a.wins++
			}
			if myTeam == firstMove {
				a.firstMoveGames++
				if myTeam == winTeam {
					a.firstMoveWins++
				}
			}
			a.deckGames[p.DeckID]++
			a.deckNames[p.DeckID] = p.DeckName
			if myTeam == winTeam {
				a.deckWins[p.DeckID]++
			}
		}

		for _, t := range g.Turns {
			team := t.TeamNumber
			for i, p := range g.Players {
				if teamForPlayerIndex(i) == team {
					uid := p.User.ID
					if byUser[uid] != nil {
						byUser[uid].turnDurations = append(byUser[uid].turnDurations, t.Duration)
					}
				}
			}
		}
	}

	out := make([]models.PlayerStats, 0, len(byUser))
	for _, a := range byUser {
		winPct := 0.0
		if a.games > 0 {
			winPct = float64(a.wins) / float64(a.games) * 100
		}
		firstMovePct := 0.0
		if a.firstMoveGames > 0 {
			firstMovePct = float64(a.firstMoveWins) / float64(a.firstMoveGames) * 100
		}
		avgTurn := 0
		maxTurn := 0
		if len(a.turnDurations) > 0 {
			sum := 0
			for _, d := range a.turnDurations {
				sum += d
				if d > maxTurn {
					maxTurn = d
				}
			}
			avgTurn = sum / len(a.turnDurations)
		}
		bestDeck := ""
		bestWins := 0
		bestGames := 0
		for deckID, wins := range a.deckWins {
			gamesWithDeck := a.deckGames[deckID]
			name := a.deckNames[deckID]
			if gamesWithDeck > 0 && (bestDeck == "" || float64(wins)/float64(gamesWithDeck) > float64(bestWins)/float64(bestGames)) {
				bestDeck = name
				bestWins = wins
				bestGames = gamesWithDeck
			}
		}

		out = append(out, models.PlayerStats{
			PlayerName:          a.playerName,
			GamesCount:          a.games,
			WinsCount:           a.wins,
			WinPercent:          winPct,
			FirstMoveWins:       a.firstMoveWins,
			FirstMoveGames:      a.firstMoveGames,
			FirstMoveWinPercent: firstMovePct,
			AvgTurnDurationSec:  avgTurn,
			MaxTurnDurationSec:  maxTurn,
			BestDeckName:        bestDeck,
			BestDeckWins:        bestWins,
			BestDeckGames:       bestGames,
		})
	}
	c.JSON(http.StatusOK, out)
}

// GetDeckStats — агрегат по колодам по завершённым играм (игры, победы, %).
// Загружает все завершённые игры в память; при большом объёме данных рассмотреть SQL-агрегацию.
func GetDeckStats(c *gin.Context) {
	db := database.GetDB()
	var games []models.Game
	if err := db.Where("end_time IS NOT NULL").Preload("Players.User").Preload("Turns").Find(&games).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список игр"})
		return
	}

	type agg struct {
		name  string
		games int
		wins  int
	}
	byDeck := make(map[int]*agg)

	for _, g := range games {
		if g.WinningTeam == nil {
			continue
		}
		winTeam := *g.WinningTeam
		for i, p := range g.Players {
			team := teamForPlayerIndex(i)
			if byDeck[p.DeckID] == nil {
				byDeck[p.DeckID] = &agg{name: p.DeckName}
			}
			byDeck[p.DeckID].games++
			if team == winTeam {
				byDeck[p.DeckID].wins++
			}
		}
	}

	out := make([]models.DeckStats, 0, len(byDeck))
	for id, a := range byDeck {
		pct := 0.0
		if a.games > 0 {
			pct = float64(a.wins) / float64(a.games) * 100
		}
		out = append(out, models.DeckStats{
			DeckID:     id,
			DeckName:   a.name,
			GamesCount: a.games,
			WinsCount:  a.wins,
			WinPercent: pct,
		})
	}
	c.JSON(http.StatusOK, out)
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
	db := database.GetDB()
	var games []models.Game
	if err := db.Where("end_time IS NOT NULL").Preload("Players.User").Find(&games).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список игр"})
		return
	}

	type matchupAgg struct {
		deck1ID   int
		deck1Name string
		deck2ID   int
		deck2Name string
		games     int
		deck1Wins int
		deck2Wins int
	}
	aggByPair := make(map[string]*matchupAgg)

	for _, g := range games {
		if g.WinningTeam == nil || len(g.Players) < 2 {
			continue
		}
		half := len(g.Players) / 2
		if half == 0 || half >= len(g.Players) {
			continue
		}
		team1 := g.Players[:half]
		team2 := g.Players[half:]
		for _, p1 := range team1 {
			for _, p2 := range team2 {
				d1ID, d1Name := p1.DeckID, p1.DeckName
				d2ID, d2Name := p2.DeckID, p2.DeckName
				deck1Won := *g.WinningTeam == 1
				if d1ID > d2ID {
					d1ID, d2ID = d2ID, d1ID
					d1Name, d2Name = d2Name, d1Name
					deck1Won = !deck1Won
				}
				key := strings.Join([]string{strconv.Itoa(d1ID), ":", strconv.Itoa(d2ID)}, "")
				if aggByPair[key] == nil {
					aggByPair[key] = &matchupAgg{
						deck1ID:   d1ID,
						deck1Name: d1Name,
						deck2ID:   d2ID,
						deck2Name: d2Name,
					}
				}
				agg := aggByPair[key]
				agg.games++
				if deck1Won {
					agg.deck1Wins++
				} else {
					agg.deck2Wins++
				}
			}
		}
	}

	out := make([]models.DeckMatchupStats, 0, len(aggByPair))
	for _, a := range aggByPair {
		deck1Rate := 0.0
		deck2Rate := 0.0
		if a.games > 0 {
			deck1Rate = float64(a.deck1Wins) / float64(a.games) * 100
			deck2Rate = float64(a.deck2Wins) / float64(a.games) * 100
		}
		out = append(out, models.DeckMatchupStats{
			Deck1ID:      a.deck1ID,
			Deck1Name:    a.deck1Name,
			Deck2ID:      a.deck2ID,
			Deck2Name:    a.deck2Name,
			GamesCount:   a.games,
			Deck1Wins:    a.deck1Wins,
			Deck2Wins:    a.deck2Wins,
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
	c.JSON(http.StatusOK, models.DeckMatchupsResponse{Matchups: out})
}

// GetMetaDashboard — мета-срез по времени с агрегатами колод.
func GetMetaDashboard(c *gin.Context) {
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
	query := db.Where("end_time IS NOT NULL").Preload("Players.User")
	if fromDate != nil {
		query = query.Where("start_time >= ?", *fromDate)
	}
	if toDate != nil {
		query = query.Where("start_time <= ?", *toDate)
	}
	var games []models.Game
	if err := query.Find(&games).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить игры для мета-дашборда"})
		return
	}

	type deckAgg struct {
		id    int
		name  string
		games int
		wins  int
	}
	type periodAgg struct {
		totalGames int
		decks      map[int]*deckAgg
	}

	allDecks := make(map[int]*deckAgg)
	periods := make(map[string]*periodAgg)
	totalGames := 0

	for _, g := range games {
		if g.WinningTeam == nil {
			continue
		}
		totalGames++
		pKey := periodKey(g.StartTime, groupBy)
		if periods[pKey] == nil {
			periods[pKey] = &periodAgg{decks: make(map[int]*deckAgg)}
		}
		pAgg := periods[pKey]
		pAgg.totalGames++

		for i, p := range g.Players {
			team := teamForPlayerIndex(i)
			if allDecks[p.DeckID] == nil {
				allDecks[p.DeckID] = &deckAgg{id: p.DeckID, name: p.DeckName}
			}
			if pAgg.decks[p.DeckID] == nil {
				pAgg.decks[p.DeckID] = &deckAgg{id: p.DeckID, name: p.DeckName}
			}

			allDecks[p.DeckID].games++
			pAgg.decks[p.DeckID].games++
			if team == *g.WinningTeam {
				allDecks[p.DeckID].wins++
				pAgg.decks[p.DeckID].wins++
			}
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
		stat := toMetaDeck(a, totalGames)
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

	periodKeys := make([]string, 0, len(periods))
	for key := range periods {
		periodKeys = append(periodKeys, key)
	}
	sort.Strings(periodKeys)

	periodOut := make([]models.MetaPeriodStats, 0, len(periodKeys))
	for _, key := range periodKeys {
		pAgg := periods[key]
		decks := make([]models.MetaDeckStat, 0, len(pAgg.decks))
		for _, a := range pAgg.decks {
			decks = append(decks, toMetaDeck(a, pAgg.totalGames))
		}
		sort.Slice(decks, func(i, j int) bool {
			if decks[i].GamesCount != decks[j].GamesCount {
				return decks[i].GamesCount > decks[j].GamesCount
			}
			return decks[i].DeckName < decks[j].DeckName
		})
		periodOut = append(periodOut, models.MetaPeriodStats{
			Period:     key,
			TotalGames: pAgg.totalGames,
			Decks:      decks,
		})
	}

	resp := models.MetaDashboardResponse{
		GroupBy:         groupBy,
		TotalGames:      totalGames,
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
	c.JSON(http.StatusOK, resp)
}
