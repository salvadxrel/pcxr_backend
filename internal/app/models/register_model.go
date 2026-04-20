package models

import (
	"time"
)

type Register_Model struct {
	Email      string    `json:"email"`
	Password   string    `json:"password"`
	First_Name string    `json:"first_name"`
	Last_Name  string    `json:"last_name"`
	Patronymic string    `json:"patronymic"`
	Phone      string    `json:"phone"`
	Photo      string    `json:"photo"`
	Created_at time.Time `json:"created_at"`
}
