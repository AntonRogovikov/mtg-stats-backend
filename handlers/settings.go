package handlers

import (
	"net/http"
	"time"

	"mtg-stats-backend/database"
	"mtg-stats-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

type SettingsResponse struct {
	Timezone              string `json:"timezone"`
	TimezoneOffsetMinutes int    `json:"timezone_offset_minutes"`
}

type UpdateSettingsRequest struct {
	Timezone string `json:"timezone" binding:"required"`
}

func resolveConfiguredTimezone() (string, *time.Location, int) {
	db := database.GetDB()
	var setting models.AppSetting
	timezoneName := models.DefaultAppTimezone

	if err := db.First(&setting, "key = ?", models.AppSettingTimezoneKey).Error; err == nil {
		if setting.Value != "" {
			timezoneName = setting.Value
		}
	}

	loc, err := time.LoadLocation(timezoneName)
	if err != nil {
		timezoneName = models.DefaultAppTimezone
		loc = time.UTC
	}

	_, offsetSeconds := time.Now().In(loc).Zone()
	return timezoneName, loc, offsetSeconds / 60
}

func GetSettings(c *gin.Context) {
	timezoneName, _, offsetMinutes := resolveConfiguredTimezone()
	c.JSON(http.StatusOK, SettingsResponse{
		Timezone:              timezoneName,
		TimezoneOffsetMinutes: offsetMinutes,
	})
}

func UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	loc, err := time.LoadLocation(req.Timezone)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный часовой пояс (используйте IANA, например Europe/Moscow)"})
		return
	}

	db := database.GetDB()
	setting := models.AppSetting{
		Key:   models.AppSettingTimezoneKey,
		Value: req.Timezone,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&setting).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить настройки"})
		return
	}

	_, offsetSeconds := time.Now().In(loc).Zone()
	c.JSON(http.StatusOK, SettingsResponse{
		Timezone:              req.Timezone,
		TimezoneOffsetMinutes: offsetSeconds / 60,
	})
}
