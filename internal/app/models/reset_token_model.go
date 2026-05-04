package models

import "time"

type Reset_Token struct {
	ID        int
	UserID    int
	Token     string
	ExpiresAt time.Time
	Used      bool
}
