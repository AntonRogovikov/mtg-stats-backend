package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
)

// ExportDeck — колода с инлайновыми данными изображений (base64), чтобы клиент мог сохранить файлы локально.
type ExportDeck struct {
	models.Deck
	ImageBase64  string `json:"image_base64,omitempty"`
	AvatarBase64 string `json:"avatar_base64,omitempty"`
}

// ExportPayload — полный дамп данных (пользователи, колоды, игры с игроками и ходами).
type ExportPayload struct {
	Users []models.User `json:"users"`
	Decks []ExportDeck  `json:"decks"`
	Games []models.Game `json:"games"`
}

func fileBase64FromImageURL(imageURL string) (string, error) {
	if imageURL == "" {
		return "", nil
	}
	path := pathFromImageURL(imageURL)
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// buildExportPayload загружает все данные из БД и формирует экспортную структуру.
func buildExportPayload(c *gin.Context) (*ExportPayload, bool) {
	db := database.GetDB()

	var users []models.User
	if err := db.Order("id ASC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список пользователей"})
		return nil, false
	}

	var decks []models.Deck
	if err := db.Order("id ASC").Find(&decks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список колод"})
		return nil, false
	}

	var games []models.Game
	if err := db.Order("updated_at DESC").Preload("Players.User").Preload("Turns").Find(&games).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список игр"})
		return nil, false
	}

	exportDecks := make([]ExportDeck, 0, len(decks))
	for _, d := range decks {
		img, err := fileBase64FromImageURL(d.ImageURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось прочитать файл изображения колоды"})
			return nil, false
		}
		avatar, err := fileBase64FromImageURL(d.AvatarURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось прочитать файл аватара колоды"})
			return nil, false
		}
		exportDecks = append(exportDecks, ExportDeck{
			Deck:         d,
			ImageBase64:  img,
			AvatarBase64: avatar,
		})
	}

	payload := &ExportPayload{
		Users: users,
		Decks: exportDecks,
		Games: games,
	}
	return payload, true
}

// ExportAllData — экспорт всех данных БД (users, decks, games) с инлайновыми картинками колод в gzip-архиве JSON.
func ExportAllData(c *gin.Context) {
	payload, ok := buildExportPayload(c)
	if !ok {
		return
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gz).Encode(payload); err != nil {
		gz.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сериализовать данные в архив"})
		return
	}
	if err := gz.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось завершить сжатие архива"})
		return
	}

	c.Header("Content-Disposition", `attachment; filename="mtg_stats_export.json.gz"`)
	c.Data(http.StatusOK, "application/gzip", buf.Bytes())
}

func writeBase64File(path string, dataB64 string) error {
	if path == "" || dataB64 == "" {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func restoreDeckImagesFromExport(decks []ExportDeck) error {
	// Полностью пересоздаём каталог с изображениями колод.
	dir := filepath.Join(getUploadDir(), decksSubdir)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, d := range decks {
		if d.ImageBase64 != "" && d.ImageURL != "" {
			if err := writeBase64File(pathFromImageURL(d.ImageURL), d.ImageBase64); err != nil {
				return err
			}
		}
		if d.AvatarBase64 != "" && d.AvatarURL != "" {
			if err := writeBase64File(pathFromImageURL(d.AvatarURL), d.AvatarBase64); err != nil {
				return err
			}
		}
	}
	return nil
}

// importAllDataFromPayload — общая логика замены всех данных и картинок по уже разобранному payload.
func importAllDataFromPayload(c *gin.Context, payload *ExportPayload) {
	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось начать транзакцию импорта"})
		return
	}

	// Очищаем данные в правильном порядке с учётом внешних ключей.
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
	if err := tx.Exec("DELETE FROM decks").Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось очистить колоды"})
		return
	}
	if err := tx.Exec("DELETE FROM users").Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось очистить пользователей"})
		return
	}

	// Восстанавливаем пользователей.
	if len(payload.Users) > 0 {
		if err := tx.Create(&payload.Users).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось восстановить пользователей"})
			return
		}
	}

	// Восстанавливаем колоды (без картинок, только записи в БД).
	if len(payload.Decks) > 0 {
		decks := make([]models.Deck, 0, len(payload.Decks))
		for _, d := range payload.Decks {
			decks = append(decks, d.Deck)
		}
		if err := tx.Create(&decks).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось восстановить колоды"})
			return
		}
	}

	// Восстанавливаем игры, игроков и ходы.
	for _, g := range payload.Games {
		players := g.Players
		turns := g.Turns
		g.Players = nil
		g.Turns = nil

		if err := tx.Create(&g).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось восстановить игру"})
			return
		}

		if len(players) > 0 {
			gps := make([]models.GamePlayer, 0, len(players))
			for _, p := range players {
				// В JSON у GamePlayer поля GameID и UserID помечены json:"-",
				// поэтому при импорте они приходят как 0. UserID берём из вложенного p.User.ID.
				userID := p.UserID
				if userID == 0 && p.User.ID != 0 {
					userID = p.User.ID
				}
				gps = append(gps, models.GamePlayer{
					ID:       p.ID,
					GameID:   g.ID,
					UserID:   userID,
					DeckID:   p.DeckID,
					DeckName: p.DeckName,
				})
			}
			if err := tx.Create(&gps).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось восстановить игроков игры"})
				return
			}
		}

		if len(turns) > 0 {
			gts := make([]models.GameTurn, 0, len(turns))
			for _, t := range turns {
				gts = append(gts, models.GameTurn{
					ID:         t.ID,
					GameID:     g.ID,
					TeamNumber: t.TeamNumber,
					Duration:   t.Duration,
					Overtime:   t.Overtime,
				})
			}
			if err := tx.Create(&gts).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось восстановить ходы игры"})
				return
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось завершить транзакцию импорта"})
		return
	}

	// Восстанавливаем файлы изображений колод на диске.
	if err := restoreDeckImagesFromExport(payload.Decks); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Данные БД восстановлены, но не удалось восстановить файлы изображений", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Все данные успешно заменены из архива"})
}

// ImportAllData — полная замена данных БД и картинок по gzip-архиву JSON.
func ImportAllData(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось прочитать тело запроса", "details": err.Error()})
		return
	}

	gr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось открыть gzip-архив", "details": err.Error()})
		return
	}
	defer gr.Close()

	var payload ExportPayload
	if err := json.NewDecoder(gr).Decode(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось разобрать JSON из архива", "details": err.Error()})
		return
	}

	importAllDataFromPayload(c, &payload)
}
