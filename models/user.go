package models

import (
	"time"
)

// User — игрок (участник игр, статистика).
type User struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:100;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserRequest — создание/обновление пользователя (имя 2–100 символов).
type UserRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100"`
}
