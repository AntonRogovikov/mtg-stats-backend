// Package models — сущности и DTO API (User, Deck, Game и запросы/ответы).
package models

import (
	"time"
)

// Deck — колода; ImageURL и AvatarURL — пути вида /uploads/decks/... (файлы на диске).
type Deck struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:150;not null"`
	ImageURL  string    `json:"image_url" gorm:"size:500"`
	AvatarURL string    `json:"avatar_url" gorm:"size:500"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeckRequest — создание/обновление колоды (название).
type DeckRequest struct {
	Name string `json:"name" binding:"required,min=2,max=150"`
}
