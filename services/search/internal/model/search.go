package model

// ProductDocument is the searchable representation stored in Elasticsearch.
type ProductDocument struct {
	ProductID   string   `json:"product_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       int64    `json:"price"`
	CategoryID  string   `json:"category_id"`
	SellerID    string   `json:"seller_id"`
	Images      []string `json:"images"`
	Status      string   `json:"status,omitempty"`
	Stock       int32    `json:"stock,omitempty"`
	Score       float32  `json:"-"`
}
