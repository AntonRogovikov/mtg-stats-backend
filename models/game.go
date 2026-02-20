package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

// GamePlayerResponse — игрок в ответе API; user.is_admin маскируется для не-админов.
type GamePlayerResponse struct {
	ID       uint         `json:"id"`
	User     UserResponse `json:"user"`
	DeckID   int          `json:"deck_id"`
	DeckName string       `json:"deck_name"`
}

// GameResponse — игра в ответе API; players[].user.is_admin маскируется для не-админов.
type GameResponse struct {
	ID                        uint                  `json:"id"`
	StartTime                 time.Time             `json:"start_time"`
	EndTime                   *time.Time            `json:"end_time,omitempty"`
	TurnLimitSeconds          int                   `json:"turn_limit_seconds"`
	FirstMoveTeam             int                   `json:"first_move_team"`
	Team1Name                 string                `json:"team1_name,omitempty"`
	Team2Name                 string                `json:"team2_name,omitempty"`
	Players                   []GamePlayerResponse  `json:"players"`
	Turns                     []GameTurn            `json:"turns"`
	CurrentTurnTeam           int                   `json:"current_turn_team"`
	CurrentTurnStart          *time.Time            `json:"current_turn_start,omitempty"`
	IsPaused                  bool                  `json:"is_paused"`
	PauseStartedAt            *time.Time           `json:"pause_started_at,omitempty"`
	TotalPauseDurationSeconds int                   `json:"total_pause_duration_seconds"`
	TeamTimeLimitSeconds      int                   `json:"team_time_limit_seconds"`
	IsTechnicalDefeat         bool                  `json:"is_technical_defeat"`
	WinningTeam               *int                  `json:"winning_team,omitempty"`
	CreatedAt                 time.Time             `json:"created_at"`
	UpdatedAt                 time.Time             `json:"updated_at"`
}

// Game — партия (игроки, ходы, лимит времени); end_time == nil — активная игра; winning_team 1 или 2.
type Game struct {
	ID                        uint         `json:"id" gorm:"primaryKey"`
	StartTime                 time.Time    `json:"start_time"`
	EndTime                   *time.Time   `json:"end_time,omitempty"`
	TurnLimitSeconds          int          `json:"turn_limit_seconds"`
	FirstMoveTeam             int          `json:"first_move_team"`
	Team1Name                 string       `json:"team1_name,omitempty"`
	Team2Name                 string       `json:"team2_name,omitempty"`
	Players                   []GamePlayer `json:"players" gorm:"foreignKey:GameID"`
	Turns                     []GameTurn   `json:"turns" gorm:"foreignKey:GameID"`
	CurrentTurnTeam           int          `json:"current_turn_team"`
	CurrentTurnStart           *time.Time   `json:"current_turn_start,omitempty"`
	IsPaused                  bool         `json:"is_paused"`
	PauseStartedAt            *time.Time   `json:"pause_started_at,omitempty"`
	TotalPauseDurationSeconds int          `json:"total_pause_duration_seconds"`
	TeamTimeLimitSeconds      int          `json:"team_time_limit_seconds"`
	IsTechnicalDefeat         bool         `json:"is_technical_defeat"`
	WinningTeam               *int         `json:"winning_team,omitempty"`
	CreatedAt                 time.Time    `json:"created_at"`
	UpdatedAt                 time.Time    `json:"updated_at"`
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
	TurnLimitSeconds      int                     `json:"turn_limit_seconds"`
	TeamTimeLimitSeconds  int                     `json:"team_time_limit_seconds"`
	FirstMoveTeam         int                     `json:"first_move_team"`
	Team1Name             string                  `json:"team1_name,omitempty"`
	Team2Name             string                  `json:"team2_name,omitempty"`
	Players               []CreateGamePlayerInput `json:"players"`
}

// FinishGameRequest — завершение игры; winning_team 1 или 2.
type FinishGameRequest struct {
	WinningTeam       int  `json:"winning_team"`
	IsTechnicalDefeat bool `json:"is_technical_defeat"`
}

// flexTime — время из JSON (RFC3339, RFC3339Nano, ISO8601 от Flutter).
type flexTime struct{ T *time.Time }

func (f *flexTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		f.T = nil
		return nil
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999", // Flutter без суффикса часового пояса
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			utc := t.UTC()
			f.T = &utc
			return nil
		}
	}
	var t time.Time
	if err := json.Unmarshal(b, &t); err != nil {
		return fmt.Errorf("invalid time format %q: %w", s, err)
	}
	utc := t.UTC()
	f.T = &utc
	return nil
}

// UpdateActiveGameRequest — обновление активной игры (текущий ход, ходы).
type UpdateActiveGameRequest struct {
	CurrentTurnTeam  int      `json:"current_turn_team"`
	CurrentTurnStart *flexTime `json:"current_turn_start,omitempty"`
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
