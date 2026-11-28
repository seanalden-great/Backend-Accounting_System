// // controllers/product_controller.go
// package controllers

// import (
// 	"net/http"

// 	"github.com/gin-gonic/gin"
// 	"gorm.io/gorm"

// 	"accounting-be/models"
// )

// type ProductController struct {
// 	DB *gorm.DB
// }

// // GET /api/products
// func (pc *ProductController) GetProducts(c *gin.Context) {
// 	var products []models.Product
// 	if err := pc.DB.Order("id desc").Find(&products).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data produk"})
// 		return
// 	}
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "data": products})
// }

// // POST /api/products
// func (pc *ProductController) CreateProduct(c *gin.Context) {
// 	var product models.Product
// 	if err := c.ShouldBindJSON(&product); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid"})
// 		return
// 	}

// 	if err := pc.DB.Create(&product).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan produk. Pastikan SKU unik."})
// 		return
// 	}
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Produk berhasil ditambahkan", "data": product})
// }

// // PUT /api/products/:id
// func (pc *ProductController) UpdateProduct(c *gin.Context) {
// 	id := c.Param("id")
// 	var product models.Product
	
// 	if err := pc.DB.First(&product, id).Error; err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Produk tidak ditemukan"})
// 		return
// 	}

// 	if err := c.ShouldBindJSON(&product); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid"})
// 		return
// 	}

// 	if err := pc.DB.Save(&product).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate produk"})
// 		return
// 	}
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Produk berhasil diupdate"})
// }

// // DELETE /api/products/:id
// func (pc *ProductController) DeleteProduct(c *gin.Context) {
// 	id := c.Param("id")
// 	if err := pc.DB.Delete(&models.Product{}, id).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus produk"})
// 		return
// 	}
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Produk berhasil dihapus"})
// }

// controllers/product_controller.go
package controllers

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"accounting-be/models"
)

type ProductController struct {
	DB *gorm.DB
}

type ProductInput struct {
	SKU      string `json:"sku" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Category string `json:"category"`
	Unit     string `json:"unit"`
}

// GET /api/products?search=...
func (pc *ProductController) GetProducts(c *gin.Context) {
	search := c.Query("search")
	query := pc.DB.Model(&models.Product{})

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("name LIKE ? OR sku LIKE ? OR category LIKE ?", like, like, like)
	}

	var products []models.Product
	if err := query.Order("id desc").Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data produk"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": products})
}

// GET /api/products/:id (Detail Kartu Stok / Bin Card)
func (pc *ProductController) GetProductDetail(c *gin.Context) {
	id := c.Param("id")
	var product models.Product
	if err := pc.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Produk tidak ditemukan"})
		return
	}

	type TxRow struct {
		ID          uint    `json:"id"`
		Date        string  `json:"date"`
		ReferenceNo string  `json:"reference_no"`
		ContactName string  `json:"contact_name"`
		Quantity    int     `json:"quantity"`
		Price       float64 `json:"price"`
	}

	// 1. Riwayat Pembelian (Barang Masuk)
	purchaseHistory := []TxRow{}
	pc.DB.Table("purchase_items").
		Select("purchase_items.id, DATE_FORMAT(purchases.date, '%Y-%m-%d') AS date, COALESCE(purchases.reference_no, CONCAT('PO #', purchases.id)) AS reference_no, COALESCE(contacts.name, '-') AS contact_name, purchase_items.quantity, purchase_items.price").
		Joins("JOIN purchases ON purchases.id = purchase_items.purchase_id").
		Joins("LEFT JOIN contacts ON contacts.id = purchases.contact_id").
		Where("purchase_items.product_sku = ?", product.SKU).
		Order("purchases.date DESC, purchase_items.id DESC").
		Scan(&purchaseHistory)

	// 2. Riwayat Penjualan (Barang Keluar)
	saleHistory := []TxRow{}
	pc.DB.Table("sale_items").
		Select("sale_items.id, DATE_FORMAT(sales.date, '%Y-%m-%d') AS date, COALESCE(sales.reference_no, CONCAT('SO #', sales.id)) AS reference_no, COALESCE(contacts.name, '-') AS contact_name, sale_items.quantity, sale_items.price").
		Joins("JOIN sales ON sales.id = sale_items.sale_id").
		Joins("LEFT JOIN contacts ON contacts.id = sales.contact_id").
		Where("sale_items.product_sku = ?", product.SKU).
		Order("sales.date DESC, sale_items.id DESC").
		Scan(&saleHistory)

	// 3. Riwayat Penyesuaian Stok (Stock Opname)
	type AdjRow struct {
		ID       uint   `json:"id"`
		Date     string `json:"date"`
		Type     string `json:"type"`
		Quantity int    `json:"quantity"`
		Reason   string `json:"reason"`
	}
	adjustments := []AdjRow{}
	pc.DB.Table("stock_adjustments").
		Select("id, DATE_FORMAT(date, '%Y-%m-%d') AS date, type, quantity, reason").
		Where("product_sku = ?", product.SKU).
		Order("date DESC, id DESC").
		Scan(&adjustments)

	// 4. Gabungkan menjadi Kartu Stok (Bin Card) dengan Running Balance
	type LedgerRow struct {
		Date    string  `json:"date"`
		Type    string  `json:"type"`
		Ref     string  `json:"ref"`
		Contact string  `json:"contact"`
		QtyIn   int     `json:"qty_in"`
		QtyOut  int     `json:"qty_out"`
		Balance int     `json:"balance"`
		Price   float64 `json:"price"`
	}
	ledger := make([]LedgerRow, 0, len(purchaseHistory)+len(saleHistory)+len(adjustments))

	for _, p := range purchaseHistory {
		ledger = append(ledger, LedgerRow{
			Date: p.Date, Type: "Barang Masuk (PO)", Ref: p.ReferenceNo,
			Contact: p.ContactName, QtyIn: p.Quantity, QtyOut: 0, Price: p.Price,
		})
	}
	for _, s := range saleHistory {
		ledger = append(ledger, LedgerRow{
			Date: s.Date, Type: "Barang Keluar (SO)", Ref: s.ReferenceNo,
			Contact: s.ContactName, QtyIn: 0, QtyOut: s.Quantity, Price: s.Price,
		})
	}
	for _, a := range adjustments {
		isAdd := a.Type == "Penambahan"
		qtyIn, qtyOut := 0, 0
		if isAdd {
			qtyIn = a.Quantity
		} else {
			qtyOut = a.Quantity
		}
		ledger = append(ledger, LedgerRow{
			Date: a.Date, Type: "Stock Opname", Ref: fmt.Sprintf("ADJ-%04d", a.ID),
			Contact: "Gudang Utama", QtyIn: qtyIn, QtyOut: qtyOut, Price: 0,
		})
	}

	sort.Slice(ledger, func(i, j int) bool {
		return ledger[i].Date < ledger[j].Date
	})

	runningBalance := 0
	for i := range ledger {
		runningBalance += ledger[i].QtyIn - ledger[i].QtyOut
		ledger[i].Balance = runningBalance
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          "success",
		"product":         product,
		"purchaseHistory": purchaseHistory,
		"saleHistory":     saleHistory,
		"adjustments":     adjustments,
		"ledger":          ledger,
	})
}

// POST /api/products
func (pc *ProductController) CreateProduct(c *gin.Context) {
	var input ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SKU dan Nama Produk wajib diisi"})
		return
	}

	unit := input.Unit
	if unit == "" {
		unit = "Pcs"
	}

	product := models.Product{
		SKU:       input.SKU,
		Name:      input.Name,
		Category:  input.Category,
		Unit:      unit,
		BuyPrice:  0,
		SellPrice: 0,
		HPP:       0,
		Stock:     0,
	}

	if err := pc.DB.Create(&product).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal menyimpan produk. Pastikan SKU unik."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Master Produk berhasil ditambahkan.", "data": product})
}

// PUT /api/products/:id
func (pc *ProductController) UpdateProduct(c *gin.Context) {
	id := c.Param("id")
	var product models.Product
	if err := pc.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Produk tidak ditemukan"})
		return
	}

	var input ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid"})
		return
	}

	product.SKU = input.SKU
	product.Name = input.Name
	product.Category = input.Category
	product.Unit = input.Unit

	if err := pc.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal mengupdate produk. Pastikan SKU unik."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Master Produk berhasil diperbarui.", "data": product})
}

// DELETE /api/products/:id
func (pc *ProductController) DeleteProduct(c *gin.Context) {
	id := c.Param("id")
	if err := pc.DB.Delete(&models.Product{}, id).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal menghapus produk. Produk mungkin sudah dipakai di transaksi."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Produk berhasil dihapus."})
}