package handlers

import (
	"net/http"
	"strconv"
	"time"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetGames возвращает все игры (сначала завершённые по дате)
func GetGames(c *gin.Context) {
	db := database.GetDB()
	var games []models.Game
	result := db.Order("updated_at DESC").Preload("Players.User").Preload("Turns").Find(&games)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch games"})
		return
	}
	c.JSON(http.StatusOK, games)
}

// GetGame возвращает игру по ID
func GetGame(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid game ID"})
		return
	}
	db := database.GetDB()
	var game models.Game
	if err := db.Preload("Players.User").Preload("Turns").First(&game, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Game not found"})
		return
	}
	c.JSON(http.StatusOK, &game)
}

func CreateGame(c *gin.Context) {
	var req models.CreateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.FirstMoveTeam < 1 || req.FirstMoveTeam > 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "first_move_team must be 1 or 2"})
		return
	}
	if len(req.Players) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "players required"})
		return
	}

	db := database.GetDB()

	var active models.Game
	if err := db.Where("end_time IS NULL").First(&active).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "active game already exists"})
		return
	}

	now := time.Now().UTC()
	game := &models.Game{
		StartTime:         now,
		TurnLimitSeconds:  req.TurnLimitSeconds,
		FirstMoveTeam:     req.FirstMoveTeam,
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "each player must have user_id or user.id"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create game"})
		return
	}

	// Подгружаем User для ответа
	db.Preload("Players.User").Preload("Turns").First(game, game.ID)
	c.JSON(http.StatusCreated, game)
}

func GetActiveGame(c *gin.Context) {
	db := database.GetDB()
	var game models.Game
	if err := db.Where("end_time IS NULL").Preload("Players.User").Preload("Turns").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active game"})
		return
	}
	c.JSON(http.StatusOK, &game)
}

func UpdateActiveGame(c *gin.Context) {
	var req models.UpdateActiveGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.GetDB()
	var game models.Game
	if err := db.Where("end_time IS NULL").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active game"})
		return
	}

	game.CurrentTurnTeam = req.CurrentTurnTeam
	game.CurrentTurnStart = req.CurrentTurnStart
	if err := db.Model(&game).Updates(map[string]interface{}{
		"current_turn_team":  game.CurrentTurnTeam,
		"current_turn_start": game.CurrentTurnStart,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update game"})
		return
	}

	if len(req.Turns) > 0 {
		db.Where("game_id = ?", game.ID).Delete(&models.GameTurn{})
		for i := range req.Turns {
			req.Turns[i].GameID = game.ID
		}
		if err := db.Create(&req.Turns).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update turns"})
			return
		}
	}

	db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
	c.JSON(http.StatusOK, &game)
}

func FinishGame(c *gin.Context) {
	var req models.FinishGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.WinningTeam < 1 || req.WinningTeam > 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "winning_team must be 1 or 2"})
		return
	}

	db := database.GetDB()
	var game models.Game
	if err := db.Where("end_time IS NULL").First(&game).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active game"})
		return
	}

	now := time.Now().UTC()
	game.EndTime = &now
	game.WinningTeam = &req.WinningTeam
	if err := db.Model(&game).Updates(map[string]interface{}{
		"end_time":     game.EndTime,
		"winning_team": game.WinningTeam,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to finish game"})
		return
	}

	db.Preload("Players.User").Preload("Turns").First(&game, game.ID)
	c.JSON(http.StatusOK, &game)
}
