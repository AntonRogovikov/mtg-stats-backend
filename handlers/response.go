package handlers

import (
	"mtg-stats-backend/middleware"
	"mtg-stats-backend/models"
)

// userToResponse возвращает UserResponse; is_admin только если viewer — админ или сам пользователь.
func userToResponse(u models.User, viewer *middleware.UserInfo) models.UserResponse {
	r := models.UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
	if viewer != nil && (viewer.IsAdmin || viewer.ID == u.ID) {
		r.IsAdmin = &u.IsAdmin
	}
	return r
}

// gameToResponse конвертирует Game в GameResponse с маскировкой is_admin в players.
func gameToResponse(g models.Game, viewer *middleware.UserInfo) models.GameResponse {
	players := make([]models.GamePlayerResponse, len(g.Players))
	for i := range g.Players {
		players[i] = models.GamePlayerResponse{
			ID:       g.Players[i].ID,
			User:     userToResponse(g.Players[i].User, viewer),
			DeckID:   g.Players[i].DeckID,
			DeckName: g.Players[i].DeckName,
		}
	}
	return models.GameResponse{
		ID:                        g.ID,
		StartTime:                 g.StartTime,
		EndTime:                   g.EndTime,
		TurnLimitSeconds:          g.TurnLimitSeconds,
		FirstMoveTeam:             g.FirstMoveTeam,
		Team1Name:                 g.Team1Name,
		Team2Name:                 g.Team2Name,
		Players:                   players,
		Turns:                     g.Turns,
		CurrentTurnTeam:           g.CurrentTurnTeam,
		CurrentTurnStart:          g.CurrentTurnStart,
		IsPaused:                  g.IsPaused,
		PauseStartedAt:            g.PauseStartedAt,
		TotalPauseDurationSeconds: g.TotalPauseDurationSeconds,
		TeamTimeLimitSeconds:      g.TeamTimeLimitSeconds,
		IsTechnicalDefeat:         g.IsTechnicalDefeat,
		WinningTeam:               g.WinningTeam,
		CreatedAt:                 g.CreatedAt,
		UpdatedAt:                 g.UpdatedAt,
	}
}
