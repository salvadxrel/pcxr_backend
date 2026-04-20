package models

import "time"

type Session struct {
	Token      string    `json:"token"`
	User_ID    int       `json:"user_id"`
	Expires_At time.Time `json:"expires_at"`
	Created_At time.Time `json:"created_at"`
	Updated_At time.Time `json:"update_at"`
	Is_Active  bool      `json:"is_active"`
}
