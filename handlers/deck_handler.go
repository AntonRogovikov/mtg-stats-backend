package handlers

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
)

const (
	decksSubdir   = "decks"
	maxImageSize  = 5 << 20
	defaultUpload = "./uploads"
)

var (
	uploadDir     string
	uploadDirOnce sync.Once
)

func getUploadDir() string {
	uploadDirOnce.Do(func() {
		uploadDir = os.Getenv("UPLOAD_DIR")
		if uploadDir == "" {
			uploadDir = defaultUpload
		}
	})
	return uploadDir
}

// GetUploadDir возвращает корневую директорию для загрузок (для Static в main). В production задайте UPLOAD_DIR.
func GetUploadDir() string {
	return getUploadDir()
}

// imageURLWithCacheBust добавляет query-параметр t=updated_at для инвалидации кэша браузера.
func imageURLWithCacheBust(url string, updatedAt time.Time) string {
	if url == "" {
		return ""
	}
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	return url + sep + "t=" + strconv.FormatInt(updatedAt.Unix(), 10)
}

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// saveDeckImageFile сохраняет файл в {UPLOAD_DIR}/decks/{id}{suffix}.{ext}; suffix "" или "_avatar". Возвращает URL /uploads/decks/...
func saveDeckImageFile(header *multipart.FileHeader, deckID int, suffix string) (string, error) {
	if header.Size > maxImageSize {
		return "", fmt.Errorf("размер файла не должен превышать %d МБ", maxImageSize/(1<<20))
	}

	file, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	contentType := http.DetectContentType(buf[:n])
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		return "", fmt.Errorf("допустимые форматы: JPEG, PNG, WebP")
	}

	base := getUploadDir()
	dir := filepath.Join(base, decksSubdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать каталог: %w", err)
	}

	fileName := strconv.Itoa(deckID) + suffix + ext
	fullPath := filepath.Join(dir, fileName)
	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("не удалось сохранить файл: %w", err)
	}
	defer dst.Close()

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		os.Remove(fullPath)
		return "", fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(fullPath)
		return "", fmt.Errorf("не удалось сохранить файл: %w", err)
	}

	return "/uploads/" + decksSubdir + "/" + fileName, nil
}

// pathFromImageURL превращает URL вида /uploads/decks/123.jpg в полный путь на диске.
func pathFromImageURL(imageURL string) string {
	if imageURL == "" || !strings.HasPrefix(imageURL, "/uploads/") {
		return ""
	}
	rel := strings.TrimPrefix(imageURL, "/uploads/")
	if rel == "" || strings.Contains(rel, "..") {
		return ""
	}
	return filepath.Join(getUploadDir(), filepath.FromSlash(rel))
}

// GetDecks — список колод, сортировка по id DESC.
func GetDecks(c *gin.Context) {
	db := database.GetDB()
	var decks []models.Deck
	if err := db.Order("id DESC").Find(&decks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список колод"})
		return
	}
	for i := range decks {
		decks[i].ImageURL = imageURLWithCacheBust(decks[i].ImageURL, decks[i].UpdatedAt)
		decks[i].AvatarURL = imageURLWithCacheBust(decks[i].AvatarURL, decks[i].UpdatedAt)
	}
	c.JSON(http.StatusOK, decks)
}

// GetDeck — колода по id; 404 если не найдена.
func GetDeck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID колоды"})
		return
	}
	db := database.GetDB()
	var deck models.Deck
	if err := db.First(&deck, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Колода не найдена"})
		return
	}
	deck.ImageURL = imageURLWithCacheBust(deck.ImageURL, deck.UpdatedAt)
	deck.AvatarURL = imageURLWithCacheBust(deck.AvatarURL, deck.UpdatedAt)
	c.JSON(http.StatusOK, deck)
}

// CreateDeck — создание колоды; название 2–150 символов.
func CreateDeck(c *gin.Context) {
	var req models.DeckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные", "details": err.Error()})
		return
	}
	if len(req.Name) < 2 || len(req.Name) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Название от 2 до 150 символов"})
		return
	}
	db := database.GetDB()
	deck := models.Deck{Name: req.Name}
	if err := db.Create(&deck).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать колоду"})
		return
	}
	c.JSON(http.StatusCreated, deck)
}

// UpdateDeck — обновление названия по id; 404 если не найдена.
func UpdateDeck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID колоды"})
		return
	}
	var req models.DeckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные", "details": err.Error()})
		return
	}
	if len(req.Name) < 2 || len(req.Name) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Название от 2 до 150 символов"})
		return
	}
	db := database.GetDB()
	var deck models.Deck
	if err := db.First(&deck, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Колода не найдена"})
		return
	}
	deck.Name = req.Name
	if err := db.Save(&deck).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить колоду"})
		return
	}
	c.JSON(http.StatusOK, deck)
}

// UploadDeckImage — multipart/form-data с полями image и avatar; оба обязательны. Сохраняет в UPLOAD_DIR/decks (на диске / Volume).
func UploadDeckImage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID колоды"})
		return
	}
	imageHeader, err := c.FormFile("image")
	if err != nil || imageHeader == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Требуется поле image"})
		return
	}
	avatarHeader, err := c.FormFile("avatar")
	if err != nil || avatarHeader == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Требуется поле avatar"})
		return
	}
	db := database.GetDB()
	var deck models.Deck
	if err := db.First(&deck, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Колода не найдена"})
		return
	}
	imageURL, err := saveDeckImageFile(imageHeader, id, "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image: " + err.Error()})
		return
	}
	deck.ImageURL = imageURL
	avatarURL, err := saveDeckImageFile(avatarHeader, id, "_avatar")
	if err != nil {
		if p := pathFromImageURL(imageURL); p != "" {
			_ = os.Remove(p)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "avatar: " + err.Error()})
		return
	}
	deck.AvatarURL = avatarURL
	if err := db.Save(&deck).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить колоду"})
		return
	}
	imgURL := imageURLWithCacheBust(deck.ImageURL, deck.UpdatedAt)
	avURL := imageURLWithCacheBust(deck.AvatarURL, deck.UpdatedAt)
	deckResp := deck
	deckResp.ImageURL = imgURL
	deckResp.AvatarURL = avURL
	c.JSON(http.StatusOK, gin.H{"message": "Изображение и аватар загружены", "image_url": imgURL, "avatar_url": avURL, "deck": deckResp})
}

// DeleteDeckImage — удаление файлов изображения и аватара с диска, обнуление ImageURL и AvatarURL.
func DeleteDeckImage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID колоды"})
		return
	}
	db := database.GetDB()
	var deck models.Deck
	if err := db.First(&deck, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Колода не найдена"})
		return
	}
	if deck.ImageURL == "" && deck.AvatarURL == "" {
		c.JSON(http.StatusOK, gin.H{"message": "У колоды нет изображения и аватара", "deck": deck})
		return
	}
	for _, u := range []string{deck.ImageURL, deck.AvatarURL} {
		if p := pathFromImageURL(u); p != "" {
			_ = os.Remove(p)
		}
	}
	deck.ImageURL = ""
	deck.AvatarURL = ""
	if err := db.Save(&deck).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить колоду"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Изображение и аватар удалены", "deck": deck})
}

// DeleteDeck — удаление колоды по id и связанных файлов изображений.
func DeleteDeck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID колоды"})
		return
	}
	db := database.GetDB()
	var deck models.Deck
	if err := db.First(&deck, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Колода не найдена"})
		return
	}
	for _, u := range []string{deck.ImageURL, deck.AvatarURL} {
		if p := pathFromImageURL(u); p != "" {
			_ = os.Remove(p)
		}
	}
	result := db.Delete(&models.Deck{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось удалить колоду"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Колода не найдена"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Колода удалена", "id": id})
}
