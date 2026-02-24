package handlers

import (
	"mtg-stats-backend/middleware"
	"mtg-stats-backend/models"
	"time"
)

// userToResponse возвращает UserResponse; is_admin только если viewer — админ или сам пользователь.
func userToResponse(u models.User, viewer *middleware.UserInfo, loc *time.Location) models.UserResponse {
	r := models.UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		CreatedAt: inLocation(u.CreatedAt, loc),
		UpdatedAt: inLocation(u.UpdatedAt, loc),
	}
	if viewer != nil && (viewer.IsAdmin || viewer.ID == u.ID) {
		r.IsAdmin = &u.IsAdmin
	}
	return r
}

// gameToResponse конвертирует Game в GameResponse с маскировкой is_admin в players.
func gameToResponse(g models.Game, viewer *middleware.UserInfo, loc *time.Location) models.GameResponse {
	players := make([]models.GamePlayerResponse, len(g.Players))
	for i := range g.Players {
		players[i] = models.GamePlayerResponse{
			ID:       g.Players[i].ID,
			User:     userToResponse(g.Players[i].User, viewer, loc),
			DeckID:   g.Players[i].DeckID,
			DeckName: g.Players[i].DeckName,
		}
	}
	return models.GameResponse{
		ID:                        g.ID,
		StartTime:                 inLocation(g.StartTime, loc),
		EndTime:                   inLocationPtr(g.EndTime, loc),
		TurnLimitSeconds:          g.TurnLimitSeconds,
		FirstMoveTeam:             g.FirstMoveTeam,
		Team1Name:                 g.Team1Name,
		Team2Name:                 g.Team2Name,
		Players:                   players,
		Turns:                     g.Turns,
		CurrentTurnTeam:           g.CurrentTurnTeam,
		CurrentTurnStart:          inLocationPtr(g.CurrentTurnStart, loc),
		IsPaused:                  g.IsPaused,
		PauseStartedAt:            inLocationPtr(g.PauseStartedAt, loc),
		TotalPauseDurationSeconds: g.TotalPauseDurationSeconds,
		TeamTimeLimitSeconds:      g.TeamTimeLimitSeconds,
		IsTechnicalDefeat:         g.IsTechnicalDefeat,
		WinningTeam:               g.WinningTeam,
		CreatedAt:                 inLocation(g.CreatedAt, loc),
		UpdatedAt:                 inLocation(g.UpdatedAt, loc),
	}
}
