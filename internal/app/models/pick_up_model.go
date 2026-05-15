package models

type PickUpPoint_Model struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	OpeningHours string `json:"opening_hours"`
	DefaultPoint *int   `json:"default_point"`
}
