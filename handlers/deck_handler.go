package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
)

const (
	uploadsDir     = "uploads"
	decksImagesDir = "uploads/decks"
	maxImageSize   = 5 << 20 // 5 MB
)

// Разрешённые MIME-типы изображений
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

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

// UploadDeckImage принимает изображение (multipart/form-data, поле "image"),
// сохраняет в uploads/decks/{id}.{ext} и обновляет deck.ImageURL.
func UploadDeckImage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Некорректный ID колоды",
		})
		return
	}

	header, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Не указан файл изображения (ожидается поле 'image')",
		})
		return
	}

	if header.Size > maxImageSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Размер файла не должен превышать %d МБ", maxImageSize/(1<<20)),
		})
		return
	}

	file, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Не удалось прочитать файл",
		})
		return
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Не удалось прочитать файл",
		})
		return
	}
	contentType := http.DetectContentType(buf[:n])
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Допустимые форматы: JPEG, PNG, WebP",
		})
		return
	}

	db := database.GetDB()
	var deck models.Deck
	if err := db.First(&deck, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Колода не найдена",
		})
		return
	}

	fileName := strconv.Itoa(id) + ext
	dirPath := decksImagesDir
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось создать каталог для загрузок",
		})
		return
	}

	fullPath := filepath.Join(dirPath, fileName)
	dst, err := os.Create(fullPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось сохранить изображение",
		})
		return
	}
	defer dst.Close()

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		os.Remove(fullPath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось прочитать файл",
		})
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(fullPath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось сохранить изображение",
		})
		return
	}

	imageURL := "/uploads/decks/" + fileName
	deck.ImageURL = imageURL
	if err := db.Save(&deck).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Не удалось обновить колоду",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Изображение загружено",
		"image_url": deck.ImageURL,
		"deck":      deck,
	})
}

// DeleteDeck удаляет колоду по id (404 если не найдена). Удаляет и файл изображения, если он был.
func DeleteDeck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Некорректный ID колоды",
		})
		return
	}

	db := database.GetDB()

	var deck models.Deck
	if err := db.First(&deck, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Колода не найдена",
		})
		return
	}

	// Удаляем файл изображения, если есть (путь в БД: /uploads/decks/1.jpg → файл uploads/decks/1.jpg)
	if deck.ImageURL != "" {
		filePath := strings.TrimPrefix(deck.ImageURL, "/")
		if filePath != "" {
			_ = os.Remove(filePath)
		}
	}

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
