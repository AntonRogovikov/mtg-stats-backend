package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	mathrand "math/rand"
	"net/http"
	"strconv"
	"time"

	"mtg-stats-backend/database"
	"mtg-stats-backend/middleware"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func generateViewToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func uniqueViewToken(db *gorm.DB) (string, error) {
	for i := 0; i < 5; i++ {
		token, err := generateViewToken()
		if err != nil {
			return "", err
		}
		var count int64
		if err := db.Model(&models.Game{}).Where("view_token = ?", token).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return token, nil
		}
	}
	return "", gorm.ErrDuplicatedKey
}

func gameViewer(c *gin.Context) *middleware.UserInfo {
	me, hasUser := middleware.GetUserInfo(c)
	if !hasUser {
		return nil
	}
	return &me
}

func gameResponse(g models.Game, viewer *middleware.UserInfo) models.GameResponse {
	_, loc, _ := resolveConfiguredTimezone()
	return gameToResponse(g, viewer, loc)
}

// GetGames — список игр с игроками и ходами; is_admin в players маскируется.
func GetGames(c *gin.Context) {
	db := database.GetDB()
	var games []models.Game
	result := db.Order("updated_at DESC").Preload("Players.User").Preload("Turns").Find(&games)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список игр"})
		return
	}
	viewer := gameViewer(c)
	resp := make([]models.GameResponse, len(games))
	for i := range games {
		resp[i] = gameResponse(games[i], viewer)
	}
	c.JSON(http.StatusOK, resp)
}

// GetGame — игра по id с игроками и ходами; is_admin в players маскируется.
func GetGame(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID игры"})
		return
	}
	db := database.GetDB()
	var game models.Game
	if err := db.Preload("Players.User").Preload("Turns").First(&game, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Игра не найдена"})
		return
	}
	c.JSON(http.StatusOK, gameResponse(game, gameViewer(c)))
}

// CreateGame — создание активной игры; first_move_team 1 или 2; 409 если активная уже есть.
func CreateGame(c *gin.Context) {
	var req models.CreateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.FirstMoveTeam < 1 || req.FirstMoveTeam > 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "first_move_team должен быть 1 или 2"})
		return
	}
	if len(req.Players) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите хотя бы одного игрока"})
		return
	}

	db := database.GetDB()

	var active models.Game
	if err := db.Where("end_time IS NULL").First(&active).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Активная игра уже существует"})
		return
	}

	now := time.Now().UTC()
	game := &models.Game{
		ViewToken:            "",
		StartTime:            now,
		TurnLimitSeconds:     req.TurnLimitSeconds,
		TeamTimeLimitSeconds: req.TeamTimeLimitSeconds,
		FirstMoveTeam:        req.FirstMoveTeam,
		Team1Name:            req.Team1Name,
		Team2Name:            req.Team2Name,
		CurrentTurnTeam:      req.FirstMoveTeam,
		Players:              make([]models.GamePlayer, 0, len(req.Players)),
		Turns:                []models.GameTurn{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	token, err := uniqueViewToken(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сгенерировать публичный токен"})
		return
	}
	game.ViewToken = token
	for _, p := range req.Players {
		var userID uint
		var userName string
		if p.User != nil && p.User.ID != 0 {
			userID = p.User.ID
			userName = p.User.Name
		} else {
			userID = uint(p.UserID)
			userName = p.UserName
		}
		if userID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "У каждого игрока должен быть user_id или user.id"})
			return
		}
		game.Players = append(game.Players, models.GamePlayer{
			UserID:   userID,
			User:     models.User{ID: userID, Name: userName},
			DeckID:   p.DeckID,
			DeckName: p.DeckName,
		})
	}

	if err := db.Session(&gorm.Session{FullSaveAssociations: true}).Create(game).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать игру"})
		return
	}
	invalidateStatsCache()

	db.Preload("Players.User").Preload("Turns").First(game, game.ID)
	c.JSON(http.StatusCreated, gameResponse(*game, gameViewer(c)))
}

// GetGameByPublicToken — read-only просмотр игры без авторизации по публичному токену.
func GetGameByPublicToken(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пустой публичный токен"})
		return
	}
	db := database.GetDB()
	var game models.Game
	if err := db.Where("view_token = ?", token).Preload("Players.User").Preload("Turns").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Игра не найдена"})
		return
	}
	c.JSON(http.StatusOK, gameResponse(game, nil))
}

func shuffledCopy(src []models.GamePlayer) []models.GamePlayer {
	out := append([]models.GamePlayer(nil), src...)
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

// CreateRematch — создаёт новую игру на основе завершённой.
func CreateRematch(c *gin.Context) {
	var req models.RematchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.SourceGameID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_game_id обязателен"})
		return
	}
	mode := req.Mode
	if mode == "" {
		mode = "classic_rematch"
	}
	if mode != "classic_rematch" && mode != "swap_team_decks_random_per_player" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неподдерживаемый mode"})
		return
	}

	db := database.GetDB()

	var active models.Game
	if err := db.Where("end_time IS NULL").First(&active).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Активная игра уже существует"})
		return
	}

	var source models.Game
	if err := db.Preload("Players.User").Preload("Turns").First(&source, req.SourceGameID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Исходная игра не найдена"})
		return
	}
	if source.EndTime == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Реванш возможен только для завершённой игры"})
		return
	}
	if len(source.Players) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Недостаточно игроков для реванша"})
		return
	}

	now := time.Now().UTC()
	token, err := uniqueViewToken(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сгенерировать публичный токен"})
		return
	}

	half := len(source.Players) / 2
	if half == 0 {
		half = 1
	}
	if half >= len(source.Players) {
		half = len(source.Players) - 1
	}

	newPlayers := make([]models.GamePlayer, 0, len(source.Players))
	switch mode {
	case "classic_rematch":
		for _, p := range source.Players {
			newPlayers = append(newPlayers, models.GamePlayer{
				UserID:   p.UserID,
				User:     models.User{ID: p.User.ID, Name: p.User.Name, IsAdmin: p.User.IsAdmin},
				DeckID:   p.DeckID,
				DeckName: p.DeckName,
			})
		}
	case "swap_team_decks_random_per_player":
		team1Players := source.Players[:half]
		team2Players := source.Players[half:]

		team1DeckPool := make([]models.GamePlayer, 0, len(team1Players))
		team2DeckPool := make([]models.GamePlayer, 0, len(team2Players))
		team1DeckPool = append(team1DeckPool, team1Players...)
		team2DeckPool = append(team2DeckPool, team2Players...)
		team1DeckPool = shuffledCopy(team1DeckPool)
		team2DeckPool = shuffledCopy(team2DeckPool)

		for i, p := range team1Players {
			deckFrom := team2DeckPool[i%len(team2DeckPool)]
			newPlayers = append(newPlayers, models.GamePlayer{
				UserID:   p.UserID,
				User:     models.User{ID: p.User.ID, Name: p.User.Name, IsAdmin: p.User.IsAdmin},
				DeckID:   deckFrom.DeckID,
				DeckName: deckFrom.DeckName,
			})
		}
		for i, p := range team2Players {
			deckFrom := team1DeckPool[i%len(team1DeckPool)]
			newPlayers = append(newPlayers, models.GamePlayer{
				UserID:   p.UserID,
				User:     models.User{ID: p.User.ID, Name: p.User.Name, IsAdmin: p.User.IsAdmin},
				DeckID:   deckFrom.DeckID,
				DeckName: deckFrom.DeckName,
			})
		}
	}

	team1 := source.Team1Name
	team2 := source.Team2Name
	rematch := models.Game{
		ViewToken:            token,
		StartTime:            now,
		TurnLimitSeconds:     source.TurnLimitSeconds,
		TeamTimeLimitSeconds: source.TeamTimeLimitSeconds,
		FirstMoveTeam:        source.FirstMoveTeam,
		Team1Name:            team1,
		Team2Name:            team2,
		CurrentTurnTeam:      source.FirstMoveTeam,
		Players:              newPlayers,
		Turns:                []models.GameTurn{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := db.Session(&gorm.Session{FullSaveAssociations: true}).Create(&rematch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать реванш"})
		return
	}
	invalidateStatsCache()
	db.Preload("Players.User").Preload("Turns").First(&rematch, rematch.ID)
	c.JSON(http.StatusCreated, gameResponse(rematch, gameViewer(c)))
}

// PauseGame — поставить партию на паузу; 404 если нет активной.
func PauseGame(c *gin.Context) {
	db := database.GetDB()
	var game models.Game
	if err := db.Where("end_time IS NULL").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Нет активной игры"})
		return
	}
	if game.IsPaused {
		db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
		c.JSON(http.StatusOK, gameResponse(game, gameViewer(c)))
		return
	}
	now := time.Now().UTC()
	game.IsPaused = true
	game.PauseStartedAt = &now
	if err := db.Model(&game).Updates(map[string]interface{}{
		"is_paused":        true,
		"pause_started_at": &now,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось поставить на паузу"})
		return
	}
	db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
	c.JSON(http.StatusOK, gameResponse(game, gameViewer(c)))
}

// ResumeGame — снять паузу; время паузы не идёт в общее и в ход; 404 если нет активной.
func ResumeGame(c *gin.Context) {
	db := database.GetDB()
	var game models.Game
	if err := db.Where("end_time IS NULL").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Нет активной игры"})
		return
	}
	if !game.IsPaused || game.PauseStartedAt == nil {
		db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
		c.JSON(http.StatusOK, gameResponse(game, gameViewer(c)))
		return
	}
	now := time.Now().UTC()
	pauseDuration := now.Sub(*game.PauseStartedAt)
	game.TotalPauseDurationSeconds += int(pauseDuration.Seconds())
	if game.CurrentTurnStart != nil {
		adjusted := game.CurrentTurnStart.Add(pauseDuration)
		game.CurrentTurnStart = &adjusted
	}
	game.IsPaused = false
	game.PauseStartedAt = nil
	updates := map[string]interface{}{
		"is_paused":                    false,
		"pause_started_at":             nil,
		"total_pause_duration_seconds": game.TotalPauseDurationSeconds,
		"current_turn_start":           game.CurrentTurnStart,
	}
	if err := db.Model(&game).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось снять паузу"})
		return
	}
	db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
	c.JSON(http.StatusOK, gameResponse(game, gameViewer(c)))
}

// StartTurn — установить начало текущего хода (серверное время); 404 если нет активной игры.
func StartTurn(c *gin.Context) {
	db := database.GetDB()
	var game models.Game
	if err := db.Where("end_time IS NULL").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Нет активной игры"})
		return
	}
	now := time.Now().UTC()
	game.CurrentTurnStart = &now
	if err := db.Model(&game).UpdateColumn("current_turn_start", game.CurrentTurnStart).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить игру"})
		return
	}
	db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
	c.JSON(http.StatusOK, gameResponse(game, gameViewer(c)))
}

// GetActiveGame — текущая активная игра (end_time IS NULL); 404 если нет.
func GetActiveGame(c *gin.Context) {
	db := database.GetDB()
	var game models.Game
	if err := db.Where("end_time IS NULL").Preload("Players.User").Preload("Turns").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Нет активной игры"})
		return
	}
	c.JSON(http.StatusOK, gameResponse(game, gameViewer(c)))
}

// UpdateActiveGame — обновление текущего хода и списка ходов активной игры; 404 если нет активной.
func UpdateActiveGame(c *gin.Context) {
	var req models.UpdateActiveGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.GetDB()
	var game models.Game
	if err := db.Where("end_time IS NULL").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Нет активной игры"})
		return
	}

	game.CurrentTurnTeam = req.CurrentTurnTeam
	// Используем серверное время для начала хода — так таймер сохранится при перезагрузке страницы.
	if req.CurrentTurnStart != nil && req.CurrentTurnStart.T != nil {
		now := time.Now().UTC()
		game.CurrentTurnStart = &now
	} else {
		game.CurrentTurnStart = nil
	}
	// UpdateColumn гарантирует запись в БД (Updates с map иногда пропускает *time.Time).
	if err := db.Model(&game).UpdateColumn("current_turn_team", game.CurrentTurnTeam).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить игру"})
		return
	}
	if err := db.Model(&game).UpdateColumn("current_turn_start", game.CurrentTurnStart).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить игру"})
		return
	}

	if len(req.Turns) > 0 {
		tx := db.Begin()
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить ходы"})
			return
		}
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		if err := tx.Where("game_id = ?", game.ID).Delete(&models.GameTurn{}).Error; err != nil {
			tx.Rollback()
			log.Printf("UpdateActiveGame: delete turns: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить ходы"})
			return
		}
		if err := tx.Exec("SELECT setval(pg_get_serial_sequence('game_turns', 'id'), COALESCE((SELECT MAX(id) FROM game_turns), 0))").Error; err != nil {
			log.Printf("UpdateActiveGame: reset sequence (ignored): %v", err)
		}
		turnsToCreate := make([]models.GameTurn, len(req.Turns))
		for i := range req.Turns {
			turnsToCreate[i] = models.GameTurn{
				GameID:     game.ID,
				TeamNumber: req.Turns[i].TeamNumber,
				Duration:   req.Turns[i].Duration,
				Overtime:   req.Turns[i].Overtime,
			}
		}
		if err := tx.Omit("ID").Create(&turnsToCreate).Error; err != nil {
			tx.Rollback()
			log.Printf("UpdateActiveGame: create turns: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  "Не удалось обновить ходы",
				"detail": err.Error(),
			})
			return
		}
		if err := tx.Commit().Error; err != nil {
			log.Printf("UpdateActiveGame: commit: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить ходы"})
			return
		}
	}
	invalidateStatsCache()

	db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
	c.JSON(http.StatusOK, gameResponse(game, gameViewer(c)))
}

// FinishGame — завершение активной игры; winning_team 1 или 2; 404 если нет активной.
func FinishGame(c *gin.Context) {
	var req models.FinishGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.WinningTeam < 1 || req.WinningTeam > 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "winning_team должен быть 1 или 2"})
		return
	}

	db := database.GetDB()
	var game models.Game
	if err := db.Where("end_time IS NULL").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Нет активной игры"})
		return
	}

	now := time.Now().UTC()
	game.EndTime = &now
	game.WinningTeam = &req.WinningTeam
	game.IsTechnicalDefeat = req.IsTechnicalDefeat
	if err := db.Model(&game).Updates(map[string]interface{}{
		"end_time":            game.EndTime,
		"winning_team":        game.WinningTeam,
		"is_technical_defeat": game.IsTechnicalDefeat,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось завершить игру"})
		return
	}
	invalidateStatsCache()

	db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
	c.JSON(http.StatusOK, gameResponse(game, gameViewer(c)))
}

// ClearGamesAndTurns — полная очистка таблиц games, game_players и game_turns.
func ClearGamesAndTurns(c *gin.Context) {
	db := database.GetDB()

	tx := db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось начать транзакцию очистки"})
		return
	}

	if err := tx.Exec("DELETE FROM game_turns").Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось очистить ходы игр"})
		return
	}
	if err := tx.Exec("DELETE FROM game_players").Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось очистить игроков игр"})
		return
	}
	if err := tx.Exec("DELETE FROM games").Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось очистить игры"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось завершить транзакцию очистки"})
		return
	}
	invalidateStatsCache()

	c.JSON(http.StatusOK, gin.H{"message": "Таблицы игр и ходов успешно очищены"})
}
