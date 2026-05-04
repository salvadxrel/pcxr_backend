package models

type Response_Profile struct {
	Email            string  `json:"email"`
	First_name       string  `json:"first_name"`
	Last_name        string  `json:"last_name"`
	Patronymic       *string `json:"patronymic"`
	Phone            *string `json:"phone"`
	Photo            *string `json:"photo"`
	Role             string  `json:"role"`
	Pick_up_point_ID *int    `json:"pick_up_point"`
	Cart_Items       int     `json:"cart_items"`
}
