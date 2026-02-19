package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetGames — список игр с игроками и ходами, сортировка по updated_at DESC.
func GetGames(c *gin.Context) {
	db := database.GetDB()
	var games []models.Game
	result := db.Order("updated_at DESC").Preload("Players.User").Preload("Turns").Find(&games)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список игр"})
		return
	}
	c.JSON(http.StatusOK, games)
}

// GetGame — игра по id с игроками и ходами; 404 если не найдена.
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
	c.JSON(http.StatusOK, &game)
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
		StartTime:            now,
		TurnLimitSeconds:     req.TurnLimitSeconds,
		TeamTimeLimitSeconds: req.TeamTimeLimitSeconds,
		FirstMoveTeam:        req.FirstMoveTeam,
		Team1Name:         req.Team1Name,
		Team2Name:         req.Team2Name,
		CurrentTurnTeam:   req.FirstMoveTeam,
		Players:           make([]models.GamePlayer, 0, len(req.Players)),
		Turns:             []models.GameTurn{},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
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

	db.Preload("Players.User").Preload("Turns").First(game, game.ID)
	c.JSON(http.StatusCreated, game)
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
		c.JSON(http.StatusOK, &game)
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
	c.JSON(http.StatusOK, &game)
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
		c.JSON(http.StatusOK, &game)
		return
	}
	now := time.Now().UTC()
	pauseDuration := now.Sub(*game.PauseStartedAt)
	game.TotalPauseDurationSeconds += int(pauseDuration.Seconds())
	// Сдвигаем current_turn_start на длительность паузы, чтобы таймер хода считал корректно.
	if game.CurrentTurnStart != nil {
		adjusted := game.CurrentTurnStart.Add(pauseDuration)
		game.CurrentTurnStart = &adjusted
	}
	game.IsPaused = false
	game.PauseStartedAt = nil
	if err := db.Model(&game).UpdateColumn("is_paused", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось снять паузу"})
		return
	}
	if err := db.Model(&game).UpdateColumn("pause_started_at", nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось снять паузу"})
		return
	}
	if err := db.Model(&game).UpdateColumn("total_pause_duration_seconds", game.TotalPauseDurationSeconds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось снять паузу"})
		return
	}
	if game.CurrentTurnStart != nil {
		if err := db.Model(&game).UpdateColumn("current_turn_start", game.CurrentTurnStart).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось снять паузу"})
			return
		}
	}
	db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
	c.JSON(http.StatusOK, &game)
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
	c.JSON(http.StatusOK, &game)
}

// GetActiveGame — текущая активная игра (end_time IS NULL); 404 если нет.
func GetActiveGame(c *gin.Context) {
	db := database.GetDB()
	var game models.Game
	if err := db.Where("end_time IS NULL").Preload("Players.User").Preload("Turns").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Нет активной игры"})
		return
	}
	c.JSON(http.StatusOK, &game)
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
		// Сбрасываем sequence: следующий nextval() вернёт max(id)+1, избегая дубликатов.
		if err := tx.Exec("SELECT setval(pg_get_serial_sequence('game_turns', 'id'), COALESCE((SELECT MAX(id) FROM game_turns), 0))").Error; err != nil {
			log.Printf("UpdateActiveGame: reset sequence (ignored): %v", err)
		}
		// Создаём новые записи, явно исключая ID.
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
				"error":   "Не удалось обновить ходы",
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

	db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
	c.JSON(http.StatusOK, &game)
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
		"end_time":             game.EndTime,
		"winning_team":         game.WinningTeam,
		"is_technical_defeat":  game.IsTechnicalDefeat,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось завершить игру"})
		return
	}

	db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
	c.JSON(http.StatusOK, &game)
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

	c.JSON(http.StatusOK, gin.H{"message": "Таблицы игр и ходов успешно очищены"})
}
