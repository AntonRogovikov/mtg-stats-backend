package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// User — игрок (участник игр, статистика).
type User struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Name         string    `json:"name" gorm:"size:100;not null;uniqueIndex"`
	PasswordHash string    `json:"-" gorm:"size:255"` // хеш пароля, не возвращается в API
	IsAdmin      bool      `json:"is_admin" gorm:"default:false"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserRequest — создание/обновление пользователя (имя 2–100 символов).
type UserRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Password string `json:"password,omitempty"` // опционально; при обновлении пустая строка = не менять
	IsAdmin  *bool  `json:"is_admin,omitempty"` // опционально; при обновлении nil = не менять
}

// UserClaims — claims JWT (id, name, is_admin).
type UserClaims struct {
	UserID  uint   `json:"uid"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"admin"`
	jwt.RegisteredClaims
}
