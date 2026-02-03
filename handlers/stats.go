package handlers

import (
	"net/http"

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
