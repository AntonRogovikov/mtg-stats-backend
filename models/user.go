package models

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// User — игрок (участник игр, статистика).
type User struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	Name               string    `json:"name" gorm:"size:100;not null;uniqueIndex"`
	PasswordHash       string    `json:"-" gorm:"size:255"` // хеш пароля, не возвращается в API
	IsAdmin            bool      `json:"is_admin" gorm:"default:false"`
	AllowAdminDowngrade bool     `json:"-" gorm:"-"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// BeforeSave защищает чувствительные поля от случайного обнуления при косвенных Save/association-save.
func (u *User) BeforeSave(tx *gorm.DB) error {
	if u.ID == 0 {
		return nil
	}
	var current User
	if err := tx.Session(&gorm.Session{NewDB: true}).
		Select("id", "password_hash", "is_admin").
		First(&current, u.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if current.PasswordHash != "" && u.PasswordHash == "" {
		u.PasswordHash = current.PasswordHash
	}
	if current.IsAdmin && !u.IsAdmin && !u.AllowAdminDowngrade {
		u.IsAdmin = true
	}
	return nil
}

// UserRequest — создание/обновление пользователя (имя 2–100 символов).
type UserRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Password string `json:"password,omitempty"` // опционально; при обновлении пустая строка = не менять
	IsAdmin  *bool  `json:"is_admin,omitempty"` // опционально; при обновлении nil = не менять
}

// UserResponse — ответ API; is_admin показывается только админу или самому пользователю.
type UserResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	IsAdmin   *bool     `json:"is_admin,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserClaims — claims JWT (id, name, is_admin).
type UserClaims struct {
	UserID  uint   `json:"uid"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"admin"`
	jwt.RegisteredClaims
}
