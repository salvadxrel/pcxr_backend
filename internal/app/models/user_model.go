package models

import "time"

type User struct {
	ID         int       `json:"id"`
	Email      string    `json:"email"`
	Password   string    `json:"password"`
	First_name string    `json:"first_name"`
	Last_name  string    `json:"last_name"`
	Patronymic string    `json:"patronymic"`
	Phone      *string   `json:"phone"`
	Photo      *string   `json:"photo"`
	Created_at time.Time `json:"created_at"`
}
