package models

import "time"

type GamePlayer struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	GameID   uint   `json:"-" gorm:"not null;index"`
	UserID   uint   `json:"-" gorm:"not null"`
	User     User   `json:"user" gorm:"foreignKey:UserID"`
	DeckID   int    `json:"deck_id"`
	DeckName string `json:"deck_name"`
}

func (GamePlayer) TableName() string { return "game_players" }

type GameTurn struct {
	ID         uint `json:"id" gorm:"primaryKey"`
	GameID     uint `json:"-" gorm:"not null;index"`
	TeamNumber int  `json:"team_number"`
	Duration   int  `json:"duration_sec"`
	Overtime   int  `json:"overtime_sec"`
}

func (GameTurn) TableName() string { return "game_turns" }

type Game struct {
	ID                uint         `json:"id" gorm:"primaryKey"`
	StartTime         time.Time    `json:"start_time"`
	EndTime           *time.Time   `json:"end_time,omitempty"`
	TurnLimitSeconds  int          `json:"turn_limit_seconds"`
	FirstMoveTeam     int          `json:"first_move_team"`
	Players           []GamePlayer `json:"players" gorm:"foreignKey:GameID"`
	Turns             []GameTurn   `json:"turns" gorm:"foreignKey:GameID"`
	CurrentTurnTeam   int          `json:"current_turn_team"`
	CurrentTurnStart  *time.Time   `json:"current_turn_start,omitempty"`
	WinningTeam       *int         `json:"winning_team,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

type CreateGameRequest struct {
	TurnLimitSeconds int          `json:"turn_limit_seconds"`
	FirstMoveTeam    int          `json:"first_move_team"`
	Players          []GamePlayer `json:"players"`
}

type FinishGameRequest struct {
	WinningTeam int `json:"winning_team"`
}

type UpdateActiveGameRequest struct {
	CurrentTurnTeam  int        `json:"current_turn_team"`
	CurrentTurnStart *time.Time `json:"current_turn_start,omitempty"`
	Turns            []GameTurn `json:"turns"`
}

type PlayerStats struct {
	PlayerName           string  `json:"player_name"`
	GamesCount           int     `json:"games_count"`
	WinsCount            int     `json:"wins_count"`
	WinPercent           float64 `json:"win_percent"`
	FirstMoveWins        int     `json:"first_move_wins"`
	FirstMoveGames       int     `json:"first_move_games"`
	FirstMoveWinPercent  float64 `json:"first_move_win_percent"`
	AvgTurnDurationSec   int     `json:"avg_turn_duration_sec"`
	MaxTurnDurationSec   int     `json:"max_turn_duration_sec"`
	BestDeckName         string  `json:"best_deck_name"`
	BestDeckWins         int     `json:"best_deck_wins"`
	BestDeckGames        int     `json:"best_deck_games"`
}

type DeckStats struct {
	DeckID     int     `json:"deck_id"`
	DeckName   string  `json:"deck_name"`
	GamesCount int     `json:"games_count"`
	WinsCount  int     `json:"wins_count"`
	WinPercent float64 `json:"win_percent"`
}
