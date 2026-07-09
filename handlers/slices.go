package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// minSliceDecks — минимальный пул колод разреза: игроков четверо, каждому нужна колода.
const minSliceDecks = 4

// DefaultSliceID — id глобального разреза (создаётся при старте, неудаляемый).
const DefaultSliceID uint = 1

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// sliceIDFromQuery — разрез из query ?slice_id=N; по умолчанию глобальный.
func sliceIDFromQuery(c *gin.Context) uint {
	raw := strings.TrimSpace(c.Query("slice_id"))
	if raw == "" {
		return DefaultSliceID
	}
	v, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || v == 0 {
		return DefaultSliceID
	}
	return uint(v)
}

// buildSliceResponses собирает ответы по разрезам: пул колод, игроки, число игр.
func buildSliceResponses(db *gorm.DB, slices []models.Slice) ([]models.SliceResponse, error) {
	out := make([]models.SliceResponse, 0, len(slices))
	if len(slices) == 0 {
		return out, nil
	}

	ids := make([]uint, 0, len(slices))
	for _, s := range slices {
		ids = append(ids, s.ID)
	}

	var sliceDecks []models.SliceDeck
	if err := db.Where("slice_id IN ?", ids).Order("deck_id ASC").Find(&sliceDecks).Error; err != nil {
		return nil, err
	}
	deckIDs := make(map[uint][]uint)
	for _, sd := range sliceDecks {
		deckIDs[sd.SliceID] = append(deckIDs[sd.SliceID], sd.DeckID)
	}

	var slicePlayers []models.SlicePlayer
	if err := db.Where("slice_id IN ?", ids).Order("user_id ASC").Find(&slicePlayers).Error; err != nil {
		return nil, err
	}
	playerIDs := make(map[uint][]uint)
	for _, sp := range slicePlayers {
		playerIDs[sp.SliceID] = append(playerIDs[sp.SliceID], sp.UserID)
	}

	var gameCounts []struct {
		SliceID uint  `gorm:"column:slice_id"`
		Count   int64 `gorm:"column:count"`
	}
	if err := db.Model(&models.Game{}).Select("slice_id, COUNT(*) AS count").Where("slice_id IN ?", ids).Group("slice_id").Scan(&gameCounts).Error; err != nil {
		return nil, err
	}
	gamesBySlice := make(map[uint]int)
	for _, gc := range gameCounts {
		gamesBySlice[gc.SliceID] = int(gc.Count)
	}

	for _, s := range slices {
		dIDs := deckIDs[s.ID]
		if dIDs == nil {
			dIDs = []uint{}
		}
		pIDs := playerIDs[s.ID]
		if pIDs == nil {
			pIDs = []uint{}
		}
		out = append(out, models.SliceResponse{
			ID:         s.ID,
			Name:       s.Name,
			Color:      s.Color,
			IsDefault:  s.IsDefault,
			DeckIDs:    dIDs,
			PlayerIDs:  pIDs,
			GamesCount: gamesBySlice[s.ID],
			CreatedAt:  s.CreatedAt,
			UpdatedAt:  s.UpdatedAt,
		})
	}
	return out, nil
}

// GetSlices — список разрезов: глобальный первым, далее по имени.
func GetSlices(c *gin.Context) {
	db := database.GetDB()
	var slices []models.Slice
	if err := db.Order("is_default DESC, name ASC").Find(&slices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить список разрезов"})
		return
	}
	out, err := buildSliceResponses(db, slices)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось собрать данные разрезов"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// GetSlice — разрез по ID с пулом колод и игроками.
func GetSlice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID разреза"})
		return
	}
	db := database.GetDB()
	var slice models.Slice
	if err := db.First(&slice, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Разрез не найден"})
		return
	}
	out, err := buildSliceResponses(db, []models.Slice{slice})
	if err != nil || len(out) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось собрать данные разреза"})
		return
	}
	c.JSON(http.StatusOK, out[0])
}

// validateSliceRequest — общая валидация создания/обновления: имя, цвет, пул, игроки.
// Возвращает нормализованные deck_ids и player_ids (при пустых player_ids — все пользователи).
func validateSliceRequest(db *gorm.DB, req *models.SliceRequest, excludeSliceID uint, isDefault bool) (deckIDs []uint, playerIDs []uint, errMsg string) {
	req.Name = strings.TrimSpace(req.Name)
	if len([]rune(req.Name)) < 2 {
		return nil, nil, "Название разреза должно быть не короче 2 символов"
	}

	req.Color = strings.TrimSpace(req.Color)
	if req.Color != "" && !hexColorRe.MatchString(req.Color) {
		return nil, nil, "Цвет должен быть в формате #RRGGBB"
	}

	var nameCount int64
	q := db.Model(&models.Slice{}).Where("name = ?", req.Name)
	if excludeSliceID != 0 {
		q = q.Where("id <> ?", excludeSliceID)
	}
	if err := q.Count(&nameCount).Error; err != nil {
		return nil, nil, "Не удалось проверить уникальность названия"
	}
	if nameCount > 0 {
		return nil, nil, "Разрез с таким названием уже существует"
	}

	// Глобальный разрез: пул не ограничен и не хранится.
	if isDefault {
		return []uint{}, []uint{}, ""
	}

	deckIDs = dedupeIDs(req.DeckIDs)
	if len(deckIDs) < minSliceDecks {
		return nil, nil, "В пуле разреза должно быть минимум 4 колоды (по числу игроков)"
	}
	var deckCount int64
	if err := db.Model(&models.Deck{}).Where("id IN ?", deckIDs).Count(&deckCount).Error; err != nil {
		return nil, nil, "Не удалось проверить колоды пула"
	}
	if int(deckCount) != len(deckIDs) {
		return nil, nil, "Некоторые колоды пула не найдены"
	}

	playerIDs = dedupeIDs(req.PlayerIDs)
	if len(playerIDs) == 0 {
		// По умолчанию — все пользователи (пока играют всегда все четверо).
		var users []models.User
		if err := db.Select("id").Order("id ASC").Find(&users).Error; err != nil {
			return nil, nil, "Не удалось загрузить пользователей"
		}
		for _, u := range users {
			playerIDs = append(playerIDs, u.ID)
		}
	} else {
		var userCount int64
		if err := db.Model(&models.User{}).Where("id IN ?", playerIDs).Count(&userCount).Error; err != nil {
			return nil, nil, "Не удалось проверить игроков разреза"
		}
		if int(userCount) != len(playerIDs) {
			return nil, nil, "Некоторые игроки разреза не найдены"
		}
	}
	return deckIDs, playerIDs, ""
}

// currentSliceDeckIDs — текущий пул колод разреза.
func currentSliceDeckIDs(db *gorm.DB, sliceID uint) []uint {
	var rows []models.SliceDeck
	if err := db.Where("slice_id = ?", sliceID).Find(&rows).Error; err != nil {
		return nil
	}
	out := make([]uint, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.DeckID)
	}
	return out
}

// sameIDSet сравнивает наборы id без учёта порядка и дублей.
func sameIDSet(a, b []uint) bool {
	setA := make(map[uint]bool, len(a))
	for _, id := range a {
		if id != 0 {
			setA[id] = true
		}
	}
	setB := make(map[uint]bool, len(b))
	for _, id := range b {
		if id != 0 {
			setB[id] = true
		}
	}
	if len(setA) != len(setB) {
		return false
	}
	for id := range setA {
		if !setB[id] {
			return false
		}
	}
	return true
}

func dedupeIDs(ids []uint) []uint {
	seen := make(map[uint]bool, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// replaceSliceLinks перезаписывает пул колод и игроков разреза в рамках транзакции.
func replaceSliceLinks(tx *gorm.DB, sliceID uint, deckIDs, playerIDs []uint) error {
	if err := tx.Where("slice_id = ?", sliceID).Delete(&models.SliceDeck{}).Error; err != nil {
		return err
	}
	if err := tx.Where("slice_id = ?", sliceID).Delete(&models.SlicePlayer{}).Error; err != nil {
		return err
	}
	if len(deckIDs) > 0 {
		rows := make([]models.SliceDeck, 0, len(deckIDs))
		for _, id := range deckIDs {
			rows = append(rows, models.SliceDeck{SliceID: sliceID, DeckID: id})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	if len(playerIDs) > 0 {
		rows := make([]models.SlicePlayer, 0, len(playerIDs))
		for _, id := range playerIDs {
			rows = append(rows, models.SlicePlayer{SliceID: sliceID, UserID: id})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	return nil
}

// CreateSlice — создать разрез (только админ): имя, цвет, пул колод (мин. 4), игроки.
func CreateSlice(c *gin.Context) {
	var req models.SliceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные разреза"})
		return
	}
	db := database.GetDB()
	deckIDs, playerIDs, errMsg := validateSliceRequest(db, &req, 0, false)
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	slice := models.Slice{Name: req.Name, Color: req.Color}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&slice).Error; err != nil {
			return err
		}
		return replaceSliceLinks(tx, slice.ID, deckIDs, playerIDs)
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать разрез"})
		return
	}

	out, err := buildSliceResponses(db, []models.Slice{slice})
	if err != nil || len(out) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Разрез создан, но не удалось собрать ответ"})
		return
	}
	c.JSON(http.StatusCreated, out[0])
}

// UpdateSlice — обновить разрез (только админ). У глобального меняются только имя и цвет.
func UpdateSlice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID разреза"})
		return
	}
	db := database.GetDB()
	var slice models.Slice
	if err := db.First(&slice, uint(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Разрез не найден"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить разрез"})
		return
	}

	var req models.SliceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные разреза"})
		return
	}

	// Пул сформированного разреза заморожен: если сыграна хотя бы одна игра,
	// менять состав колод нельзя (глобального не касается — у него пул не хранится).
	poolLocked := false
	if !slice.IsDefault {
		var gamesCount int64
		if err := db.Model(&models.Game{}).Where("slice_id = ?", slice.ID).Count(&gamesCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось проверить игры разреза"})
			return
		}
		poolLocked = gamesCount > 0
	}

	deckIDs, playerIDs, errMsg := validateSliceRequest(db, &req, slice.ID, slice.IsDefault || poolLocked)
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	if poolLocked && !sameIDSet(req.DeckIDs, currentSliceDeckIDs(db, slice.ID)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пул колод нельзя менять: в разрезе уже есть игры"})
		return
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&slice).Updates(map[string]interface{}{
			"name":  req.Name,
			"color": req.Color,
		}).Error; err != nil {
			return err
		}
		if slice.IsDefault || poolLocked {
			return nil
		}
		return replaceSliceLinks(tx, slice.ID, deckIDs, playerIDs)
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить разрез"})
		return
	}

	if err := db.First(&slice, slice.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Разрез обновлён, но не удалось перечитать"})
		return
	}
	out, err := buildSliceResponses(db, []models.Slice{slice})
	if err != nil || len(out) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Разрез обновлён, но не удалось собрать ответ"})
		return
	}
	c.JSON(http.StatusOK, out[0])
}

// DeleteSlice — удалить разрез со всеми его играми (только админ; глобальный удалить нельзя).
// Каскад: ходы и игроки игр разреза, сами игры, связи пула/игроков, сам разрез.
func DeleteSlice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID разреза"})
		return
	}
	db := database.GetDB()
	var slice models.Slice
	if err := db.First(&slice, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Разрез не найден"})
		return
	}
	if slice.IsDefault {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Глобальный разрез удалить нельзя"})
		return
	}

	var deletedGames int64
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM game_turns WHERE game_id IN (SELECT id FROM games WHERE slice_id = ?)", slice.ID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM game_players WHERE game_id IN (SELECT id FROM games WHERE slice_id = ?)", slice.ID).Error; err != nil {
			return err
		}
		res := tx.Where("slice_id = ?", slice.ID).Delete(&models.Game{})
		if res.Error != nil {
			return res.Error
		}
		deletedGames = res.RowsAffected
		if err := tx.Where("slice_id = ?", slice.ID).Delete(&models.SliceDeck{}).Error; err != nil {
			return err
		}
		if err := tx.Where("slice_id = ?", slice.ID).Delete(&models.SlicePlayer{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Slice{}, slice.ID).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось удалить разрез"})
		return
	}
	invalidateStatsCache()
	c.JSON(http.StatusOK, gin.H{
		"message":       "Разрез и все его игры удалены",
		"id":            slice.ID,
		"deleted_games": deletedGames,
	})
}
