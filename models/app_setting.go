package models

import "time"

// AppSetting — глобальные настройки приложения (key-value).
type AppSetting struct {
	Key       string    `json:"key" gorm:"primaryKey;size:100"`
	Value     string    `json:"value" gorm:"size:255;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const AppSettingTimezoneKey = "app_timezone"
const DefaultAppTimezone = "UTC"
