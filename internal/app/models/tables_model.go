package models

type Tables struct {
	ID                     int     `json:"id"`
	Name                   int     `json:"name"`
	Photo                  *string `json:"photo"`
	Discription            int     `json:"discription"`
	Category               int     `json:"category"`
	Price                  float32 `json:"price"`
	Max_height             int     `json:"max_height"`
	Min_height             int     `json:"min_height"`
	Load_capacity          int     `json:"load_capacity"`
	Lifting_mechanism      *string `json:"lifting_mechanism"`
	Height_storage_console int     `json:"height_storage_console"`
	Type_support           int     `json:"type_support"`
}
