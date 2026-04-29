package models

type FilterModel struct {
	Categories        []string
	Lift              []string
	Panel             []string
	Type_Support      []string
	Price_min         *float64
	Price_max         *float64
	Frame_min         *int
	Frame_max         *int
	Load_capacity_min *int
	Load_capacity_max *int
	Frame_width_min   *int
	Frame_width_max   *int
	Page              int
	Order             *int
	Search            *string
}

type Response_Tables_Authorized struct {
	Product_ID             *int     `json:"product_id"`
	ID                     int      `json:"id"`
	Name                   string   `json:"name"`
	Photo                  *string  `json:"photo"`
	Description            *string  `json:"Description"`
	Price                  *float64 `json:"price"`
	Max_height             *int     `json:"max_height"`
	Min_height             *int     `json:"min_height"`
	Load_capacity          *int     `json:"load_capacity"`
	Lifting_mechanism      *string  `json:"lifting_mechanism"`
	Height_storage_console *int     `json:"height_storage_console"`
	Type_support           *string  `json:"type_support"`
	Category_id            int      `json:"category_id"`
	In_Cart                bool     `json:"in_cart"`
}

type Response_Tables_Guest struct {
	ID                     int      `json:"id"`
	Name                   string   `json:"name"`
	Photo                  *string  `json:"photo"`
	Description            *string  `json:"Description"`
	Price                  *float64 `json:"price"`
	Max_height             *int     `json:"max_height"`
	Min_height             *int     `json:"min_height"`
	Load_capacity          *int     `json:"load_capacity"`
	Lifting_mechanism      *string  `json:"lifting_mechanism"`
	Height_storage_console *int     `json:"height_storage_console"`
	Type_support           *string  `json:"type_support"`
	Category_id            int      `json:"category_id"`
}

type Response_Underframe_Authorized struct {
	Product_ID             *int     `json:"product_id"`
	ID                     int      `json:"id"`
	Name                   string   `json:"name"`
	Photo                  *string  `json:"photo"`
	Description            *string  `json:"Description"`
	Price                  *float64 `json:"price"`
	Max_height             *int     `json:"max_height"`
	Min_height             *int     `json:"min_height"`
	Load_capacity          *int     `json:"load_capacity"`
	Lifting_mechanism      *string  `json:"lifting_mechanism"`
	Type_support           *string  `json:"type_support"`
	Frame_width            *int     `json:"frame_width"`
	Category_id            int      `json:"category_id"`
	Height_storage_console *int     `json:"height_storage_console"`
	In_Cart                bool     `json:"in_cart"`
}

type Response_Underframe_Guest struct {
	ID                     int      `json:"id"`
	Name                   string   `json:"name"`
	Photo                  *string  `json:"photo"`
	Description            *string  `json:"Description"`
	Price                  *float64 `json:"price"`
	Max_height             *int     `json:"max_height"`
	Min_height             *int     `json:"min_height"`
	Load_capacity          *int     `json:"load_capacity"`
	Lifting_mechanism      *string  `json:"lifting_mechanism"`
	Type_support           *string  `json:"type_support"`
	Frame_width            *int     `json:"frame_width"`
	Category_id            int      `json:"category_id"`
	Height_storage_console *int     `json:"height_storage_console"`
}
