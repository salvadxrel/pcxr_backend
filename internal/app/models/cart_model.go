package models

type Cart_Model struct {
	ID      int `json:"id"`
	User_id int `json:"user_id"`
}

type Cart_Config_Model struct {
	ID                  int      `json:"id"`
	Quantity            int      `json:"quantity"`
	Product_Name        *string  `json:"product_name"`
	Product_Photo       *string  `json:"product_photo"`
	Product_Description *string  `json:"product_description"`
	Product_Price       *float64 `json:"product_price"`
	Category_Name       *string  `json:"category_name"`
	Product_Type        *string  `json:"product_type"`
	Table_ID            *int     `json:"table_id,omitempty"`
	Tabletop_ID         *int     `json:"tabletop_id,omitempty"`
	Underframe_ID       *int     `json:"underframe_id,omitempty"`
}
type ID_model struct {
	ID int `json:"id"`
}

type Add_Remove_Cart_Model struct {
	Product_id int `json:"product_id"`
}
