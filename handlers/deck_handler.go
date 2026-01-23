package handlers

import (
	"net/http"
	"strconv"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
)

// GetDecks возвращает все колоды
func GetDecks(c *gin.Context) {
	db := database.GetDB()
	var decks []models.Deck

	result := db.Order("id DESC").Find(&decks)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch decks",
		})
		return
	}

	c.JSON(http.StatusOK, decks)
}

// GetDeck возвращает колоду по ID
func GetDeck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid deck ID",
		})
		return
	}

	db := database.GetDB()
	var deck models.Deck

	result := db.First(&deck, id)

	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Deck not found",
		})
		return
	}

	c.JSON(http.StatusOK, deck)
}

// CreateDeck создает новую колоду
func CreateDeck(c *gin.Context) {
	var deckReq models.DeckRequest

	// Валидация входных данных
	if err := c.ShouldBindJSON(&deckReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid input data",
			"details": err.Error(),
		})
		return
	}

	// Проверяем длину названия
	if len(deckReq.Name) < 2 || len(deckReq.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Name must be between 2 and 100 characters",
		})
		return
	}

	db := database.GetDB()

	// Создаем колоду
	deck := models.Deck{
		Name: deckReq.Name,
	}

	result := db.Create(&deck)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create deck",
		})
		return
	}

	c.JSON(http.StatusCreated, deck)
}

// UpdateDeck обновляет колоду
func UpdateDeck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid deck ID",
		})
		return
	}

	var deckReq models.DeckRequest

	if err := c.ShouldBindJSON(&deckReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid input data",
			"details": err.Error(),
		})
		return
	}

	// Проверяем длину названия
	if len(deckReq.Name) < 2 || len(deckReq.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Name must be between 2 and 100 characters",
		})
		return
	}

	db := database.GetDB()

	// Проверяем, существует ли колода
	var existingDeck models.Deck
	if err := db.First(&existingDeck, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Deck not found",
		})
		return
	}

	// Обновляем колоду
	existingDeck.Name = deckReq.Name
	result := db.Save(&existingDeck)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update deck",
		})
		return
	}

	c.JSON(http.StatusOK, existingDeck)
}

// DeleteDeck удаляет колоду
func DeleteDeck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid deck ID",
		})
		return
	}

	db := database.GetDB()

	// Удаляем колоду
	result := db.Delete(&models.Deck{}, id)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete deck",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Deck not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Deck deleted successfully",
		"id":      id,
	})
}
