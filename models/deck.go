package models

import (
	"time"
)

// Deck — колода (набор карт). Привязывается к игрокам в играх.
type Deck struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:150;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeckRequest — тело запроса для создания/обновления колоды
type DeckRequest struct {
	Name string `json:"name" binding:"required,min=2,max=150"`
}
