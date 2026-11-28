// models/product.go
package models

type Product struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	SKU       string  `gorm:"type:varchar(50);uniqueIndex;not null" json:"sku"`
	Name      string  `gorm:"type:varchar(150);not null" json:"name"`
	Category  string  `gorm:"type:varchar(50)" json:"category"`
	Unit      string  `gorm:"type:varchar(20);default:'Pcs'" json:"unit"`
	BuyPrice  float64 `gorm:"type:decimal(15,2);default:0" json:"buy_price"`
	SellPrice float64 `gorm:"type:decimal(15,2);default:0" json:"sell_price"`
	HPP       float64 `gorm:"type:decimal(15,2);default:0" json:"hpp"`
	Stock     int     `gorm:"type:int;default:0" json:"stock"`
}