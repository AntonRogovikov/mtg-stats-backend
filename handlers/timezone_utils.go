package handlers

import (
	"time"

	"mtg-stats-backend/models"
)

func inLocation(t time.Time, loc *time.Location) time.Time {
	return t.In(loc)
}

func inLocationPtr(t *time.Time, loc *time.Location) *time.Time {
	if t == nil {
		return nil
	}
	v := t.In(loc)
	return &v
}

func userInLocation(u models.User, loc *time.Location) models.User {
	u.CreatedAt = inLocation(u.CreatedAt, loc)
	u.UpdatedAt = inLocation(u.UpdatedAt, loc)
	return u
}

func deckInLocation(d models.Deck, loc *time.Location) models.Deck {
	d.CreatedAt = inLocation(d.CreatedAt, loc)
	d.UpdatedAt = inLocation(d.UpdatedAt, loc)
	return d
}
