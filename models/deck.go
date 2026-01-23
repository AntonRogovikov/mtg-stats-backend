package models

import (
	"time"
)

type Deck struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:150;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserRequest для создания/обновления
type DeckRequest struct {
	Name string `json:"name" binding:"required,min=2,max=150"`
}
