package handlers

import (
	"net/http"
	"strconv"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
)

// GetDecks возвращает список всех колод (сортировка по id DESC).
func GetDecks(c *gin.Context) {
	db := database.GetDB()
	var decks []models.Deck

	result := db.Order("id DESC").Find(&decks)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось загрузить список колод",
		})
		return
	}

	c.JSON(http.StatusOK, decks)
}

// GetDeck возвращает одну колоду по id (404 при отсутствии).
func GetDeck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Некорректный ID колоды",
		})
		return
	}

	db := database.GetDB()
	var deck models.Deck

	result := db.First(&deck, id)

	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Колода не найдена",
		})
		return
	}

	c.JSON(http.StatusOK, deck)
}

// CreateDeck создаёт колоду по телу запроса (название 2–150 символов).
func CreateDeck(c *gin.Context) {
	var deckReq models.DeckRequest

	if err := c.ShouldBindJSON(&deckReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Некорректные данные запроса",
			"details": err.Error(),
		})
		return
	}

	// Проверяем длину названия (модель Deck допускает до 150 символов)
	if len(deckReq.Name) < 2 || len(deckReq.Name) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Название должно быть от 2 до 150 символов",
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
			"error": "Не удалось создать колоду",
		})
		return
	}

	c.JSON(http.StatusCreated, deck)
}

// UpdateDeck обновляет название колоды по id (404 при отсутствии).
func UpdateDeck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Некорректный ID колоды",
		})
		return
	}

	var deckReq models.DeckRequest

	if err := c.ShouldBindJSON(&deckReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Некорректные данные запроса",
			"details": err.Error(),
		})
		return
	}

	// Проверяем длину названия (модель Deck допускает до 150 символов)
	if len(deckReq.Name) < 2 || len(deckReq.Name) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Название должно быть от 2 до 150 символов",
		})
		return
	}

	db := database.GetDB()

	// Проверяем, существует ли колода
	var existingDeck models.Deck
	if err := db.First(&existingDeck, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Колода не найдена",
		})
		return
	}

	// Обновляем колоду
	existingDeck.Name = deckReq.Name
	result := db.Save(&existingDeck)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось обновить колоду",
		})
		return
	}

	c.JSON(http.StatusOK, existingDeck)
}

// DeleteDeck удаляет колоду по id (404 если не найдена).
func DeleteDeck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Некорректный ID колоды",
		})
		return
	}

	db := database.GetDB()

	result := db.Delete(&models.Deck{}, id)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось удалить колоду",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Колода не найдена",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Колода успешно удалена",
		"id":      id,
	})
}
