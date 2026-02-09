package models

import (
	"encoding/json"
	"strconv"
	"time"
)

// GamePlayer — участник игры (user + колода); индексы 0,1 — команда 1, 2,3 — команда 2.
type GamePlayer struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	GameID   uint   `json:"-" gorm:"not null;index"`
	UserID   uint   `json:"-" gorm:"not null"`
	User     User   `json:"user" gorm:"foreignKey:UserID"`
	DeckID   int    `json:"deck_id"`
	DeckName string `json:"deck_name"`
}

func (GamePlayer) TableName() string { return "game_players" }

// GameTurn — ход в игре: команда, длительность и овертайм (сек).
type GameTurn struct {
	ID         uint `json:"id" gorm:"primaryKey"`
	GameID     uint `json:"-" gorm:"not null;index"`
	TeamNumber int  `json:"team_number"`
	Duration   int  `json:"duration_sec"`
	Overtime   int  `json:"overtime_sec"`
}

func (GameTurn) TableName() string { return "game_turns" }

// Game — партия (игроки, ходы, лимит времени); end_time == nil — активная игра; winning_team 1 или 2.
type Game struct {
	ID               uint         `json:"id" gorm:"primaryKey"`
	StartTime        time.Time    `json:"start_time"`
	EndTime          *time.Time   `json:"end_time,omitempty"`
	TurnLimitSeconds int          `json:"turn_limit_seconds"`
	FirstMoveTeam    int          `json:"first_move_team"`
	Team1Name        string       `json:"team1_name,omitempty"`
	Team2Name        string       `json:"team2_name,omitempty"`
	Players          []GamePlayer `json:"players" gorm:"foreignKey:GameID"`
	Turns            []GameTurn   `json:"turns" gorm:"foreignKey:GameID"`
	CurrentTurnTeam  int          `json:"current_turn_team"`
	CurrentTurnStart *time.Time   `json:"current_turn_start,omitempty"`
	WinningTeam      *int         `json:"winning_team,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

// flexUint — JSON: число, строка или null (совместимость с Flutter).
type flexUint uint

func (u *flexUint) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case float64:
		*u = flexUint(x)
		return nil
	case string:
		n, err := strconv.ParseUint(x, 10, 64)
		if err != nil {
			return err
		}
		*u = flexUint(n)
		return nil
	case nil:
		*u = 0
		return nil
	default:
		*u = 0
		return nil
	}
}

// CreateGamePlayerInput — игрок в запросе: user_id/user_name или user.
type CreateGamePlayerInput struct {
	UserID   flexUint `json:"user_id"`
	UserName string   `json:"user_name"`
	User     *User    `json:"user,omitempty"`
	DeckID   int      `json:"deck_id"`
	DeckName string   `json:"deck_name"`
}

// CreateGameRequest — запрос создания игры.
type CreateGameRequest struct {
	TurnLimitSeconds int                     `json:"turn_limit_seconds"`
	FirstMoveTeam    int                     `json:"first_move_team"`
	Team1Name        string                  `json:"team1_name,omitempty"`
	Team2Name        string                  `json:"team2_name,omitempty"`
	Players          []CreateGamePlayerInput `json:"players"`
}

// FinishGameRequest — завершение игры; winning_team 1 или 2.
type FinishGameRequest struct {
	WinningTeam int `json:"winning_team"`
}

// UpdateActiveGameRequest — обновление активной игры (текущий ход, ходы).
type UpdateActiveGameRequest struct {
	CurrentTurnTeam  int        `json:"current_turn_team"`
	CurrentTurnStart *time.Time `json:"current_turn_start,omitempty"`
	Turns            []GameTurn `json:"turns"`
}

// PlayerStats — агрегат по игроку (ответ /api/stats/players).
type PlayerStats struct {
	PlayerName          string  `json:"player_name"`
	GamesCount          int     `json:"games_count"`
	WinsCount           int     `json:"wins_count"`
	WinPercent          float64 `json:"win_percent"`
	FirstMoveWins       int     `json:"first_move_wins"`
	FirstMoveGames      int     `json:"first_move_games"`
	FirstMoveWinPercent float64 `json:"first_move_win_percent"`
	AvgTurnDurationSec  int     `json:"avg_turn_duration_sec"`
	MaxTurnDurationSec  int     `json:"max_turn_duration_sec"`
	BestDeckName        string  `json:"best_deck_name"`
	BestDeckWins        int     `json:"best_deck_wins"`
	BestDeckGames       int     `json:"best_deck_games"`
}

// DeckStats — агрегат по колоде (ответ /api/stats/decks).
type DeckStats struct {
	DeckID     int     `json:"deck_id"`
	DeckName   string  `json:"deck_name"`
	GamesCount int     `json:"games_count"`
	WinsCount  int     `json:"wins_count"`
	WinPercent float64 `json:"win_percent"`
}
