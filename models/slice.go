package models

import "time"

// Slice — разрез статистики: именованный контекст с пулом колод и составом игроков.
// Игра привязывается к разрезу при создании (games.slice_id) и остаётся в нём навсегда.
// Разрез id=1 (is_default) — «Глобальный»: пул колод не ограничен, удалить нельзя.
type Slice struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:100;not null;uniqueIndex"`
	Color     string    `json:"color,omitempty" gorm:"size:16"` // hex вида #RRGGBB; пусто — стандартная тема
	IsDefault bool      `json:"is_default" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SliceDeck — колода в пуле разреза (join-таблица управляется явно, без GORM-ассоциаций).
type SliceDeck struct {
	SliceID uint `gorm:"primaryKey"`
	DeckID  uint `gorm:"primaryKey"`
}

func (SliceDeck) TableName() string { return "slice_decks" }

// SlicePlayer — игрок разреза (пока справочно: в разрезе всегда все игроки).
type SlicePlayer struct {
	SliceID uint `gorm:"primaryKey"`
	UserID  uint `gorm:"primaryKey"`
}

func (SlicePlayer) TableName() string { return "slice_players" }

// SliceRequest — создание/обновление разреза.
type SliceRequest struct {
	Name      string `json:"name" binding:"required,min=2,max=100"`
	Color     string `json:"color"`
	DeckIDs   []uint `json:"deck_ids"`
	PlayerIDs []uint `json:"player_ids"`
}

// SliceResponse — разрез в ответе API: пул колод, игроки, число игр.
type SliceResponse struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Color      string    `json:"color,omitempty"`
	IsDefault  bool      `json:"is_default"`
	DeckIDs    []uint    `json:"deck_ids"`
	PlayerIDs  []uint    `json:"player_ids"`
	GamesCount int       `json:"games_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
