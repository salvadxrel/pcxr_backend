package models

import "time"

type OrderRequest struct {
	Order_token string
	Status      string
	Date        time.Time
	Sum         int
}

type OrderInfoRequest struct {
	Photo    string
	Name     string
	Price    int
	Quantity int
	Sum      int
	Date     time.Time
}
