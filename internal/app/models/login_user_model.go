package models

type Login_User_Model struct {
	User_ID  int    `json:"user_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Login_Model struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
