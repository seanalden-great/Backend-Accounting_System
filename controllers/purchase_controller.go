// // controllers/purchase_controller.go
// package controllers

// import (
// 	"net/http"
// 	"time"

// 	"github.com/gin-gonic/gin"
// 	"gorm.io/gorm"

// 	"accounting-be/models"
// )

// type PurchaseController struct {
// 	DB *gorm.DB
// }

// // Format JSON yang diharapkan dari Frontend
// type PurchaseItemInput struct {
// 	ProductSKU string  `json:"product_sku" binding:"required"`
// 	Quantity   int     `json:"quantity" binding:"required,min=1"`
// 	Price      float64 `json:"price" binding:"required,min=0"`
// }

// type PurchaseInput struct {
// 	Date        string              `json:"date" binding:"required"`
// 	DueDate     string              `json:"due_date"`
// 	ReferenceNo string              `json:"reference_no"`
// 	ContactID   uint                `json:"contact_id" binding:"required"`
// 	Notes       string              `json:"notes"`
// 	Items       []PurchaseItemInput `json:"items" binding:"required,min=1"`
// }

// // POST /api/purchases
// func (pc *PurchaseController) CreatePurchase(c *gin.Context) {
// 	var input PurchaseInput
// 	if err := c.ShouldBindJSON(&input); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
// 		return
// 	}

// 	parsedDate, err := time.Parse("2006-01-02", input.Date)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid, gunakan YYYY-MM-DD"})
// 		return
// 	}

// 	var parsedDueDate time.Time
// 	if input.DueDate != "" {
// 		parsedDueDate, _ = time.Parse("2006-01-02", input.DueDate)
// 	} else {
// 		parsedDueDate = parsedDate // Default disamakan dengan tanggal nota
// 	}

// 	// Mulai Transaksi Database
// 	tx := pc.DB.Begin()

// 	// 1. Simpan Header (Nilai Total di-set 0 dulu)
// 	purchase := models.Purchase{
// 		Date:        parsedDate,
// 		DueDate:     parsedDueDate,
// 		ReferenceNo: input.ReferenceNo,
// 		ContactID:   input.ContactID,
// 		Status:      "Pesanan", // Default Fase Awal
// 		Notes:       input.Notes,
// 		Total:       0, 
// 	}

// 	if err := tx.Create(&purchase).Error; err != nil {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan header pesanan"})
// 		return
// 	}

// 	// 2. Simpan Detail Item sekaligus menghitung Grand Total
// 	var grandTotal float64
// 	for _, itemInput := range input.Items {
// 		subTotal := float64(itemInput.Quantity) * itemInput.Price
// 		grandTotal += subTotal

// 		purchaseItem := models.PurchaseItem{
// 			PurchaseID: purchase.ID,
// 			ProductSKU: itemInput.ProductSKU,
// 			Quantity:   itemInput.Quantity,
// 			Price:      itemInput.Price,
// 			SubTotal:   subTotal,
// 		}

// 		if err := tx.Create(&purchaseItem).Error; err != nil {
// 			tx.Rollback()
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan detail pesanan: " + itemInput.ProductSKU})
// 			return
// 		}
// 	}

// 	// 3. Perbarui Nilai Total di Header
// 	if err := tx.Model(&purchase).Update("Total", grandTotal).Error; err != nil {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate total pesanan"})
// 		return
// 	}

// 	// Selesaikan Transaksi
// 	tx.Commit()

// 	c.JSON(http.StatusOK, gin.H{
// 		"status":  "success",
// 		"message": "Pesanan Pembelian (PO) berhasil dibuat",
// 	})
// }

// // GET /api/purchases (Menyediakan filter ?status=Pesanan)
// func (pc *PurchaseController) GetPurchases(c *gin.Context) {
// 	var purchases []models.Purchase
	
// 	// Tarik semua relasi secara otomatis agar JSON komplit
// 	query := pc.DB.Preload("Contact").
// 		Preload("Items").
// 		Preload("Items.Product").
// 		Preload("PaymentAccount").
// 		Order("date desc, id desc")

// 	status := c.Query("status")
// 	if status != "" {
// 		query = query.Where("status = ?", status)
// 	}

// 	if err := query.Find(&purchases).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data pembelian"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"status": "success", "data": purchases})
// }

// func (pc *PurchaseController) ReceivePurchase(c *gin.Context) {
// 	id := c.Param("id")
// 	var purchase models.Purchase

// 	// Tarik data Header beserta Detail Item-nya
// 	if err := pc.DB.Preload("Items").First(&purchase, id).Error; err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Pesanan tidak ditemukan"})
// 		return
// 	}

// 	// Validasi status
// 	if purchase.Status != "Pesanan" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Pesanan ini sudah diproses atau dibatalkan"})
// 		return
// 	}

// 	// Mulai Transaksi
// 	tx := pc.DB.Begin()

// 	// 1. Ubah Status Header menjadi Pengiriman
// 	purchase.Status = "Pengiriman"
// 	if err := tx.Save(&purchase).Error; err != nil {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status pesanan"})
// 		return
// 	}

// 	// 2. Loop setiap barang, lalu Update Stok & Kalkulasi HPP (Moving Average)
// 	for _, item := range purchase.Items {
// 		var product models.Product
// 		if err := tx.Where("sku = ?", item.ProductSKU).First(&product).Error; err != nil {
// 			tx.Rollback()
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Produk tidak ditemukan: " + item.ProductSKU})
// 			return
// 		}

// 		// Rumus Moving Average HPP:
// 		// HPP Baru = ((Stok Lama * HPP Lama) + (Qty Datang * Harga Beli Datang)) / (Stok Lama + Qty Datang)
// 		oldTotalValue := float64(product.Stock) * product.HPP
// 		newIncomingValue := float64(item.Quantity) * item.Price
// 		newTotalStock := product.Stock + item.Quantity

// 		var newHPP float64
// 		if newTotalStock > 0 {
// 			newHPP = (oldTotalValue + newIncomingValue) / float64(newTotalStock)
// 		}

// 		// Update Master Produk
// 		product.Stock = newTotalStock
// 		product.HPP = newHPP
// 		product.BuyPrice = item.Price // Update referensi harga beli terakhir

// 		if err := tx.Save(&product).Error; err != nil {
// 			tx.Rollback()
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate stok produk: " + product.SKU})
// 			return
// 		}
// 	}

// 	// Commit Transaksi
// 	tx.Commit()

// 	c.JSON(http.StatusOK, gin.H{
// 		"status":  "success", 
// 		"message": "Barang berhasil diterima, Stok & HPP telah diperbarui otomatis!",
// 	})
// }

// type BillInput struct {
// 	DueDate string `json:"due_date" binding:"required"`
// 	PaymentAccount string `json:"payment_account" binding:"required"` // Tambahan: Menerima kode bank dari frontend
// }

// // PUT /api/purchases/:id/bill
// // func (pc *PurchaseController) CreateBill(c *gin.Context) {
// // 	id := c.Param("id")
// // 	var input BillInput
// // 	if err := c.ShouldBindJSON(&input); err != nil {
// // 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal jatuh tempo tidak valid"})
// // 		return
// // 	}

// // 	parsedDueDate, err := time.Parse("2006-01-02", input.DueDate)
// // 	if err != nil {
// // 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal salah, gunakan YYYY-MM-DD"})
// // 		return
// // 	}

// // 	var purchase models.Purchase
// // 	if err := pc.DB.Preload("Items").First(&purchase, id).Error; err != nil {
// // 		c.JSON(http.StatusNotFound, gin.H{"error": "Data pengiriman pembelian tidak ditemukan"})
// // 		return
// // 	}

// // 	if purchase.Status != "Pengiriman" {
// // 		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya dokumen dengan status 'Pengiriman' yang dapat dibuatkan tagihan"})
// // 		return
// // 	}

// // 	tx := pc.DB.Begin()

// // 	// 1. Update Status dan Tanggal Jatuh Tempo Header
// // 	purchase.Status = "Tagihan"
// // 	purchase.DueDate = parsedDueDate
// // 	if err := tx.Save(&purchase).Error; err != nil {
// // 		tx.Rollback()
// // 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status tagihan"})
// // 		return
// // 	}

// // 	// 2. Otomatis Buat Jurnal Akuntansi (Double-Entry)
// // 	// Cari Akun Persediaan (Biasanya kode berawalan '113' atau sesuaikan CoA Anda) dan Utang Usaha ('211')
// // 	// var inventoryAccount models.Account
// // 	// var apAccount models.Account

// // 	// tx.Where("code LIKE ?", "160%").First(&inventoryAccount) // Persediaan
// // 	// if inventoryAccount.Code == "" {
// // 	// 	tx.Where("code LIKE ?", "11%").First(&inventoryAccount) // Fallback ke aset lancar jika tidak ketemu
// // 	// }
// // 	// tx.Where("code LIKE ?", "310%").First(&apAccount)         // Utang Usaha

// // 	// if inventoryAccount.Code == "" || apAccount.Code == "" {
// // 	// 	tx.Rollback()
// // 	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Akun Persediaan atau Utang Usaha belum dikonfigurasi di CoA"})
// // 	// 	return
// // 	// }
	
// // 	// 2. Otomatis Buat Jurnal Akuntansi (Double-Entry)
// // 	var inventoryAccount models.Account
// // 	var apAccount models.Account

// // 	// PERBAIKAN: Cari berdasarkan awalan 16% DAN wajib is_header = false (Akun Detail)
// // 	tx.Where("code LIKE ? AND is_header = ?", "16%", false).First(&inventoryAccount) // Persediaan
// // 	if inventoryAccount.Code == "" {
// // 		tx.Where("code LIKE ? AND is_header = ?", "11%", false).First(&inventoryAccount) // Fallback
// // 	}
	
// // 	// PERBAIKAN: Cari berdasarkan awalan 31% DAN wajib is_header = false (Akun Detail)
// // 	tx.Where("code LIKE ? AND is_header = ?", "31%", false).First(&apAccount)         // Utang Usaha

// // 	if inventoryAccount.Code == "" || apAccount.Code == "" {
// // 		tx.Rollback()
// // 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Akun Persediaan atau Utang Usaha (Detail) belum dikonfigurasi di CoA"})
// // 		return
// // 	}

// // 	// journalCode := "BILL-" + time.Now().Format("20060102") + "-" + id
// // 	// journal := models.Journal{
// // 	// 	Date:        time.Now(),
// // 	// 	Description: "Tagihan Pembelian dari PO #" + id + " (Ref: " + purchase.ReferenceNo + ")",
// // 	// 	Reference:   journalCode,
// // 	// }
	
// // 	journalCode := "BILL-" + time.Now().Format("20060102") + "-" + id
// // 	journal := models.Journal{
// // 		Date:        time.Now(),
// // 		// Masukkan journalCode ke dalam Description
// // 		Description: "Tagihan Pembelian dari PO #" + id + " (Ref: " + purchase.ReferenceNo + ") - " + journalCode, 
// // 	}

// // 	if err := tx.Create(&journal).Error; err != nil {
// // 		tx.Rollback()
// // 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat jurnal akuntansi"})
// // 		return
// // 	}

// // 	entries := []models.JournalEntry{
// // 		{JournalID: journal.ID, AccountCode: inventoryAccount.Code, Debit: purchase.Total, Credit: 0},
// // 		{JournalID: journal.ID, AccountCode: apAccount.Code, Debit: 0, Credit: purchase.Total},
// // 	}

// // 	for _, entry := range entries {
// // 		if err := tx.Create(&entry).Error; err != nil {
// // 			tx.Rollback()
// // 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencatat rincian jurnal"})
// // 			return
// // 		}
// // 	}

// // 	tx.Commit()
// // 	c.JSON(http.StatusOK, gin.H{
// // 		"status":  "success",
// // 		"message": "Tagihan Pembelian berhasil diterbitkan & Jurnal Akuntansi otomatis tercatat!",
// // 	})
// // }

// // PUT /api/purchases/:id/bill
// // func (pc *PurchaseController) CreateBill(c *gin.Context) {
// // 	id := c.Param("id")
// // 	var input BillInput
// // 	if err := c.ShouldBindJSON(&input); err != nil {
// // 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format input tagihan tidak valid"})
// // 		return
// // 	}

// // 	parsedDueDate, err := time.Parse("2006-01-02", input.DueDate)
// // 	if err != nil {
// // 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal salah, gunakan YYYY-MM-DD"})
// // 		return
// // 	}

// // 	var purchase models.Purchase
// // 	if err := pc.DB.Preload("Items").First(&purchase, id).Error; err != nil {
// // 		c.JSON(http.StatusNotFound, gin.H{"error": "Data pengiriman pembelian tidak ditemukan"})
// // 		return
// // 	}

// // 	if purchase.Status != "Pengiriman" {
// // 		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya dokumen dengan status 'Pengiriman' yang dapat dibuatkan tagihan"})
// // 		return
// // 	}

// // 	tx := pc.DB.Begin()

// // 	// 1. Update Status dan Tanggal Jatuh Tempo Header
// // 	purchase.Status = "Tagihan"
// // 	purchase.DueDate = parsedDueDate
// // 	purchase.PaymentAccountCode = &input.PaymentAccount
// // 	if err := tx.Save(&purchase).Error; err != nil {
// // 		tx.Rollback()
// // 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status tagihan"})
// // 		return
// // 	}

// // 	// 2. Otomatis Buat Jurnal Akuntansi (Debit: Persediaan, Kredit: Bank)
// // 	var inventoryAccount models.Account
// // 	var bankAccount models.Account

// // 	// Cari Akun Persediaan
// // 	tx.Where("code LIKE ? AND is_header = ?", "16%", false).First(&inventoryAccount)
// // 	if inventoryAccount.Code == "" {
// // 		tx.Where("code LIKE ? AND is_header = ?", "11%", false).First(&inventoryAccount)
// // 	}
	
// // 	// Cari Akun Bank berdasarkan pilihan di Frontend
// // 	tx.Where("code = ? AND is_header = ?", input.PaymentAccount, false).First(&bankAccount)

// // 	if inventoryAccount.Code == "" || bankAccount.Code == "" {
// // 		tx.Rollback()
// // 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Akun Persediaan atau Akun Bank tidak ditemukan di sistem"})
// // 		return
// // 	}

// // 	journalCode := "BILL-" + time.Now().Format("20060102") + "-" + id
// // 	journal := models.Journal{
// // 		Date:        time.Now(),
// // 		Description: "Pembayaran Tagihan PO #" + id + " (Ref: " + purchase.ReferenceNo + ") - " + journalCode, 
// // 	}

// // 	if err := tx.Create(&journal).Error; err != nil {
// // 		tx.Rollback()
// // 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat jurnal akuntansi"})
// // 		return
// // 	}

// // 	// Debit: Persediaan (Barang Masuk) | Kredit: Bank (Uang Keluar)
// // 	entries := []models.JournalEntry{
// // 		{JournalID: journal.ID, AccountCode: inventoryAccount.Code, Debit: purchase.Total, Credit: 0},
// // 		{JournalID: journal.ID, AccountCode: bankAccount.Code, Debit: 0, Credit: purchase.Total},
// // 	}

// // 	for _, entry := range entries {
// // 		if err := tx.Create(&entry).Error; err != nil {
// // 			tx.Rollback()
// // 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencatat rincian jurnal"})
// // 			return
// // 		}
// // 	}

// // 	tx.Commit()
// // 	c.JSON(http.StatusOK, gin.H{
// // 		"status":  "success",
// // 		"message": "Tagihan Pembelian lunas dibayar & Jurnal Kas Keluar otomatis tercatat!",
// // 	})
// // }

// // PUT /api/purchases/:id/bill
// func (pc *PurchaseController) CreateBill(c *gin.Context) {
// 	id := c.Param("id")
// 	var input BillInput
// 	if err := c.ShouldBindJSON(&input); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format input tagihan tidak valid"})
// 		return
// 	}

// 	parsedDueDate, err := time.Parse("2006-01-02", input.DueDate)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal salah, gunakan YYYY-MM-DD"})
// 		return
// 	}

// 	var purchase models.Purchase
// 	if err := pc.DB.Preload("Items").First(&purchase, id).Error; err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Data pengiriman pembelian tidak ditemukan"})
// 		return
// 	}

// 	if purchase.Status != "Pengiriman" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya dokumen dengan status 'Pengiriman' yang dapat dibuatkan tagihan"})
// 		return
// 	}

// 	tx := pc.DB.Begin()

// 	purchase.Status = "Tagihan"
// 	purchase.DueDate = parsedDueDate
// 	purchase.PaymentAccountCode = &input.PaymentAccount
// 	if err := tx.Save(&purchase).Error; err != nil {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status tagihan"})
// 		return
// 	}

// 	// === PERBAIKAN: Cari Akun Persediaan (13%) dan Bank ===
// 	var inventoryAccount models.Account
// 	var bankAccount models.Account

// 	// Sesuai Excel CoA: 130 adalah Persediaan Induk, 131 adalah Persediaan Gula (is_header = false)
// 	tx.Where("code LIKE ? AND is_header = ?", "13%", false).First(&inventoryAccount)
	
// 	// Cari Akun Bank berdasarkan pilihan di Frontend (Kas atau Bank: Awalan 10 atau 11)
// 	tx.Where("code = ? AND is_header = ?", input.PaymentAccount, false).First(&bankAccount)

// 	if inventoryAccount.Code == "" {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Akun Persediaan Detail (Awalan 13) belum ada di Chart of Accounts!"})
// 		return
// 	}
// 	if bankAccount.Code == "" {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Akun Bank pembayar tidak valid atau merupakan akun Induk!"})
// 		return
// 	}

// 	journalCode := "BILL-" + time.Now().Format("20060102") + "-" + id
// 	journal := models.Journal{
// 		Date:        time.Now(),
// 		Description: "Pembayaran Tunai PO #" + id + " (Ref: " + purchase.ReferenceNo + ") - " + journalCode, 
// 	}

// 	if err := tx.Create(&journal).Error; err != nil {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat jurnal akuntansi"})
// 		return
// 	}

// 	// JURNAL STANDAR AKUNTANSI PEMBELIAN:
// 	// Debit: Persediaan (Bertambah) | Kredit: Kas/Bank (Berkurang)
// 	entries := []models.JournalEntry{
// 		{JournalID: journal.ID, AccountCode: inventoryAccount.Code, Debit: purchase.Total, Credit: 0},
// 		{JournalID: journal.ID, AccountCode: bankAccount.Code, Debit: 0, Credit: purchase.Total},
// 	}

// 	for _, entry := range entries {
// 		if err := tx.Create(&entry).Error; err != nil {
// 			tx.Rollback()
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencatat rincian jurnal"})
// 			return
// 		}
// 	}

// 	tx.Commit()
// 	c.JSON(http.StatusOK, gin.H{
// 		"status":  "success",
// 		"message": "Pembelian berhasil dilunasi & Jurnal (Debit Persediaan, Kredit Bank) tercatat!",
// 	})
// }

// controllers/purchase_controller.go
package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"accounting-be/models"
)

type PurchaseController struct {
	DB *gorm.DB
}

func (pc *PurchaseController) generateDocNumber(docType string, date time.Time) string {
	month := int(date.Month())
	year := date.Year()
	suffix := fmt.Sprintf("/%s/MPM/%s/%d", docType, romanMonths[date.Month()], year)

	var latestRef string
	if docType == "PO" || docType == "RFQ" {
		pc.DB.Model(&models.Purchase{}).
			Select("reference_no").
			Where("YEAR(date) = ? AND MONTH(date) = ? AND reference_no LIKE ?", year, month, "%"+suffix).
			Order("id DESC").
			Limit(1).
			Scan(&latestRef)
	} else {
		pc.DB.Model(&models.Journal{}).
			Select("reference_no").
			Where("YEAR(date) = ? AND MONTH(date) = ? AND reference_no LIKE ?", year, month, "%"+suffix).
			Order("id DESC").
			Limit(1).
			Scan(&latestRef)
	}

	nextSeq := 1
	if latestRef != "" {
		parts := strings.Split(latestRef, "/")
		if len(parts) > 0 {
			if n, err := strconv.Atoi(parts[0]); err == nil {
				nextSeq = n + 1
			}
		}
	}

	return fmt.Sprintf("%03d%s", nextSeq, suffix)
}

func (pc *PurchaseController) findDetailAccount(tx *gorm.DB, prefixes ...string) models.Account {
	var acc models.Account
	for _, p := range prefixes {
		if err := tx.Where("code LIKE ? AND is_header = ?", p, false).First(&acc).Error; err == nil && acc.Code != "" {
			return acc
		}
	}
	return acc
}

// GET /api/purchases
func (pc *PurchaseController) GetPurchases(c *gin.Context) {
	var purchases []models.Purchase
	query := pc.DB.Preload("Contact").
		Preload("Items").
		Preload("Items.Product").
		Preload("PaymentAccount").
		Order("date desc, id desc")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&purchases).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data pembelian"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          purchases,
		"nextDocNumber": pc.generateDocNumber("PO", time.Now()),
	})
}

type PurchaseItemInput struct {
	ProductSKU string  `json:"product_sku" binding:"required"`
	Quantity   int     `json:"quantity" binding:"required,min=1"`
	Price      float64 `json:"price" binding:"required,min=0"`
	TaxRate    int     `json:"tax_rate"`
}

type PurchaseInput struct {
	Date      string              `json:"date" binding:"required"`
	DueDate   string              `json:"due_date"`
	ContactID uint                `json:"contact_id" binding:"required"`
	Notes     string              `json:"notes"`
	Action    string              `json:"action"` // "quotation" (RFQ) atau "po"
	Items     []PurchaseItemInput `json:"items" binding:"required,min=1"`
}

// POST /api/purchases
func (pc *PurchaseController) CreatePurchase(c *gin.Context) {
	var input PurchaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	parsedDate, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid, gunakan YYYY-MM-DD"})
		return
	}
	parsedDueDate := parsedDate
	if input.DueDate != "" {
		if d, err := time.Parse("2006-01-02", input.DueDate); err == nil {
			parsedDueDate = d
		}
	}

	prefix := "PO"
	status := "Menunggu Persetujuan"
	if input.Action == "quotation" {
		prefix = "RFQ"
		status = "Penawaran"
	}

	refNo := pc.generateDocNumber(prefix, parsedDate)
	tx := pc.DB.Begin()

	purchase := models.Purchase{
		Date:        parsedDate,
		DueDate:     parsedDueDate,
		ReferenceNo: refNo,
		ContactID:   input.ContactID,
		Status:      status,
		Notes:       input.Notes,
	}

	if err := tx.Create(&purchase).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan dokumen pembelian"})
		return
	}

	var grandTotal, grandTax float64
	for _, item := range input.Items {
		dpp := float64(item.Quantity) * item.Price
		taxAmount := dpp * (float64(item.TaxRate) / 100.0)
		subTotal := dpp + taxAmount

		grandTax += taxAmount
		grandTotal += subTotal

		purchaseItem := models.PurchaseItem{
			PurchaseID: purchase.ID,
			ProductSKU: item.ProductSKU,
			Quantity:   item.Quantity,
			Price:      item.Price,
			TaxRate:    item.TaxRate,
			TaxAmount:  taxAmount,
			SubTotal:   subTotal,
		}
		if err := tx.Create(&purchaseItem).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan detail item pembelian"})
			return
		}
	}

	if err := tx.Model(&purchase).Updates(map[string]interface{}{
		"total":     grandTotal,
		"total_tax": grandTax,
	}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui total pembelian"})
		return
	}

	tx.Commit()
	msg := "Pesanan Pembelian (PO)"
	if input.Action == "quotation" {
		msg = "Draf Permintaan (RFQ)"
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Dokumen %s berhasil dibuat dengan nomor: %s", msg, refNo),
		"data":    purchase,
	})
}

// POST /api/purchases/:id/convert (RFQ -> Menunggu Persetujuan)
func (pc *PurchaseController) ConvertToPO(c *gin.Context) {
	id := c.Param("id")
	var purchase models.Purchase
	if err := pc.DB.First(&purchase, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dokumen tidak ditemukan"})
		return
	}
	if purchase.Status != "Penawaran" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya dokumen draf Penawaran (RFQ) yang dapat diubah ke PO"})
		return
	}

	refNo := pc.generateDocNumber("PO", time.Now())
	purchase.Status = "Menunggu Persetujuan"
	purchase.ReferenceNo = refNo
	purchase.Notes = strings.TrimSpace(purchase.Notes + "\n[Sistem] Harga disepakati Supplier. Diubah dari Draf Penawaran (RFQ) ke PO pada " + time.Now().Format("02/01/2006"))

	pc.DB.Save(&purchase)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Draf RFQ berhasil diubah menjadi Pesanan Pembelian (PO): " + refNo})
}

// POST /api/purchases/:id/approve
func (pc *PurchaseController) ApprovePurchase(c *gin.Context) {
	id := c.Param("id")
	var purchase models.Purchase
	if err := pc.DB.First(&purchase, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesanan tidak ditemukan"})
		return
	}
	if purchase.Status != "Menunggu Persetujuan" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dokumen tidak dalam status Menunggu Persetujuan"})
		return
	}

	purchase.Status = "Pesanan"
	purchase.Notes = strings.TrimSpace(purchase.Notes + "\n[DISETUJUI] (" + time.Now().Format("02/01/2006 15:04") + ")")
	pc.DB.Save(&purchase)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pesanan Pembelian (PO) berhasil disetujui dan siap diterima di gudang."})
}

// POST /api/purchases/:id/reject
func (pc *PurchaseController) RejectPurchase(c *gin.Context) {
	id := c.Param("id")
	var purchase models.Purchase
	if err := pc.DB.First(&purchase, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesanan tidak ditemukan"})
		return
	}

	purchase.Status = "Ditolak"
	purchase.Notes = strings.TrimSpace(purchase.Notes + "\n[DITOLAK] (" + time.Now().Format("02/01/2006 15:04") + ")")
	pc.DB.Save(&purchase)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pesanan Pembelian (PO) telah ditolak."})
}

// PUT /api/purchases/:id/receive (Penerimaan: Tambah Stok + Moving Average HPP + Jurnal Persediaan vs Hutang Usaha)
func (pc *PurchaseController) ReceivePurchase(c *gin.Context) {
	id := c.Param("id")
	var purchase models.Purchase
	if err := pc.DB.Preload("Items").First(&purchase, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesanan tidak ditemukan"})
		return
	}
	if purchase.Status != "Pesanan" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pesanan ini sudah diproses atau dibatalkan"})
		return
	}

	tx := pc.DB.Begin()

	for _, item := range purchase.Items {
		var product models.Product
		if err := tx.Where("sku = ?", item.ProductSKU).First(&product).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Produk tidak ditemukan: " + item.ProductSKU})
			return
		}

		oldTotalValue := float64(product.Stock) * product.HPP
		newIncomingValue := float64(item.Quantity) * item.Price
		newTotalStock := product.Stock + item.Quantity

		var newHPP float64
		if newTotalStock > 0 {
			newHPP = (oldTotalValue + newIncomingValue) / float64(newTotalStock)
		}

		product.Stock = newTotalStock
		product.HPP = newHPP
		product.BuyPrice = item.Price

		if err := tx.Save(&product).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate stok produk: " + product.SKU})
			return
		}
	}

	invAcc := pc.findDetailAccount(tx, "13%", "16%")
	apAcc := pc.findDetailAccount(tx, "31%", "3%")

	if invAcc.Code == "" || apAcc.Code == "" {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Akun Persediaan (13) atau Akun Hutang Usaha (awalan 3) belum ditemukan di CoA!"})
		return
	}

	now := time.Now()
	journalCode := fmt.Sprintf("RCV-P-%s-%d", now.Format("20060102"), purchase.ID)

	purchase.Status = "Pengiriman"
	purchase.Notes = strings.TrimSpace(purchase.Notes + "\n[Masuk Gudang] Diterima & dicatat dengan ref: " + journalCode)
	if err := tx.Save(&purchase).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status penerimaan"})
		return
	}

	// Jurnal: Debit Persediaan | Kredit Hutang Usaha
	journal := models.Journal{
		Date:        now,
		Description: fmt.Sprintf("Penerimaan Barang PO #%d (%s)", purchase.ID, journalCode),
		ReferenceNo: journalCode,
	}
	tx.Create(&journal)
	tx.Create(&models.JournalEntry{JournalID: journal.ID, AccountCode: invAcc.Code, Debit: purchase.Total, Credit: 0})
	tx.Create(&models.JournalEntry{JournalID: journal.ID, AccountCode: apAcc.Code, Debit: 0, Credit: purchase.Total})

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Barang diterima! Stok, HPP Moving Average, dan Jurnal Hutang Usaha berhasil dicatat.",
	})
}

type BillInput struct {
	DueDate        string `json:"due_date" binding:"required"`
	PaymentAccount string `json:"payment_account" binding:"required"`
}

// PUT /api/purchases/:id/bill (Pelunasan: Debit Hutang Usaha | Kredit Kas/Bank)
func (pc *PurchaseController) CreateBill(c *gin.Context) {
	id := c.Param("id")
	var input BillInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Harap pilih tanggal lunas dan rekening pembayar"})
		return
	}

	parsedDueDate, err := time.Parse("2006-01-02", input.DueDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal salah, gunakan YYYY-MM-DD"})
		return
	}

	var purchase models.Purchase
	if err := pc.DB.First(&purchase, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data penerimaan tidak ditemukan"})
		return
	}
	if purchase.Status != "Pengiriman" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya dokumen berstatus Pengiriman yang dapat dilunasi"})
		return
	}

	tx := pc.DB.Begin()
	apAcc := pc.findDetailAccount(tx, "31%", "3%")
	var bankAcc models.Account
	tx.Where("code = ? AND is_header = ?", input.PaymentAccount, false).First(&bankAcc)

	if apAcc.Code == "" || bankAcc.Code == "" {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Akun Hutang Usaha (awalan 3) atau Akun Bank tidak valid di CoA"})
		return
	}

	journalCode := pc.generateDocNumber("PI", parsedDueDate)

	purchase.Status = "Tagihan"
	purchase.DueDate = parsedDueDate
	purchase.PaymentAccountCode = &input.PaymentAccount
	purchase.Notes = strings.TrimSpace(purchase.Notes + "\n[Lunas] Tagihan dibayar dengan nomor: " + journalCode)
	if err := tx.Save(&purchase).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui status tagihan"})
		return
	}

	// Jurnal: Debit Hutang Usaha | Kredit Kas/Bank
	journal := models.Journal{
		Date:        parsedDueDate,
		Description: fmt.Sprintf("Pelunasan Hutang PO #%d (Ref: %s)", purchase.ID, purchase.ReferenceNo),
		ReferenceNo: journalCode,
	}
	tx.Create(&journal)
	tx.Create(&models.JournalEntry{JournalID: journal.ID, AccountCode: apAcc.Code, Debit: purchase.Total, Credit: 0})
	tx.Create(&models.JournalEntry{JournalID: journal.ID, AccountCode: bankAcc.Code, Debit: 0, Credit: purchase.Total})

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tagihan Pembelian lunas dengan nomor " + journalCode + ". Saldo Hutang telah ditutup dari Bank.",
	})
}

type PurchaseReturnInput struct {
	Reason string `json:"reason" binding:"required"`
}

// POST /api/purchases/:id/return (Retur Pembelian / Debit Note)
func (pc *PurchaseController) ProcessReturn(c *gin.Context) {
	id := c.Param("id")
	var input PurchaseReturnInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Alasan retur wajib diisi"})
		return
	}

	var purchase models.Purchase
	if err := pc.DB.Preload("Items").First(&purchase, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesanan tidak ditemukan"})
		return
	}
	if purchase.Status != "Pengiriman" && purchase.Status != "Tagihan" && purchase.Status != "Selesai" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya pesanan yang sudah diterima yang dapat diretur"})
		return
	}

	tx := pc.DB.Begin()
	for _, item := range purchase.Items {
		var product models.Product
		if err := tx.Where("sku = ?", item.ProductSKU).First(&product).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Produk tidak ditemukan: " + item.ProductSKU})
			return
		}
		if product.Stock < item.Quantity {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Stok tidak mencukupi untuk diretur pada produk: " + product.Name})
			return
		}
		product.Stock -= item.Quantity
		tx.Save(&product)
	}

	apAcc := pc.findDetailAccount(tx, "31%", "3%")
	invAcc := pc.findDetailAccount(tx, "13%", "16%")

	now := time.Now()
	retCode := pc.generateDocNumber("RET", now)

	if apAcc.Code != "" && invAcc.Code != "" {
		j1 := models.Journal{
			Date:        now,
			Description: fmt.Sprintf("Retur Pembelian (Debit Note) PO #%d - %s", purchase.ID, input.Reason),
			ReferenceNo: retCode,
		}
		tx.Create(&j1)
		tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: apAcc.Code, Debit: purchase.Total, Credit: 0})
		tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: invAcc.Code, Debit: 0, Credit: purchase.Total})
	}

	purchase.Status = "Diretur"
	purchase.Notes = strings.TrimSpace(purchase.Notes + fmt.Sprintf("\n[RETUR] %s (Surat Jalan Retur: %s)", input.Reason, retCode))
	tx.Save(&purchase)

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Retur Pembelian (Debit Note) berhasil diproses: " + retCode})
}

// GET /api/purchases/aging (Laporan Umur Hutang / AP Aging)
func (pc *PurchaseController) GetPayableAging(c *gin.Context) {
	var unpaidPurchases []models.Purchase
	pc.DB.Preload("Contact").
		Where("status = ?", "Pengiriman").
		Order("due_date asc").
		Find(&unpaidPurchases)

	now := time.Now().Truncate(24 * time.Hour)
	agingData := map[string]float64{
		"belum_jatuh_tempo": 0,
		"1_30_hari":         0,
		"31_60_hari":        0,
		"61_90_hari":        0,
		"lebih_90_hari":     0,
	}

	details := make([]map[string]interface{}, 0, len(unpaidPurchases))
	var totalHutang float64

	for _, po := range unpaidPurchases {
		due := po.DueDate.Truncate(24 * time.Hour)
		diffDays := int(now.Sub(due).Hours() / 24)

		category := "belum_jatuh_tempo"
		daysOverdue := 0

		if diffDays <= 0 {
			agingData["belum_jatuh_tempo"] += po.Total
		} else {
			daysOverdue = diffDays
			switch {
			case daysOverdue <= 30:
				category = "1_30_hari"
				agingData["1_30_hari"] += po.Total
			case daysOverdue <= 60:
				category = "31_60_hari"
				agingData["31_60_hari"] += po.Total
			case daysOverdue <= 90:
				category = "61_90_hari"
				agingData["61_90_hari"] += po.Total
			default:
				category = "lebih_90_hari"
				agingData["lebih_90_hari"] += po.Total
			}
		}

		totalHutang += po.Total
		ref := po.ReferenceNo
		if ref == "" {
			ref = fmt.Sprintf("PO #%d", po.ID)
		}

		details = append(details, map[string]interface{}{
			"id":           po.ID,
			"reference_no": ref,
			"supplier":     po.Contact.Name,
			"phone":        po.Contact.Phone,
			"due_date":     po.DueDate.Format("2006-01-02"),
			"total":        po.Total,
			"category":     category,
			"days_overdue": daysOverdue,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"agingData":   agingData,
		"details":     details,
		"totalHutang": totalHutang,
	})
}

// GET /api/purchases/overview
func (pc *PurchaseController) GetPurchasesOverview(c *gin.Context) {
	var purchases []models.Purchase
	pc.DB.Preload("Contact").Preload("Items.Product").Order("date desc, id desc").Find(&purchases)

	type TopProduct struct {
		ProductSKU   string  `json:"product_sku"`
		ProductName  string  `json:"product_name"`
		TotalQty     float64 `json:"total_qty"`
		TotalRevenue float64 `json:"total_revenue"`
	}
	topProducts := []TopProduct{}
	pc.DB.Table("purchase_items").
		Select("purchase_items.product_sku, COALESCE(products.name, purchase_items.product_sku) AS product_name, SUM(purchase_items.quantity) AS total_qty, SUM(purchase_items.sub_total) AS total_revenue").
		Joins("LEFT JOIN products ON products.sku = purchase_items.product_sku").
		Group("purchase_items.product_sku, products.name").
		Order("total_qty DESC").
		Limit(5).
		Scan(&topProducts)

	var predictions interface{} = nil
	if len(topProducts) > 0 {
		topSKU := topProducts[0].ProductSKU
		type HistPoint struct {
			Month string  `json:"month"`
			Qty   float64 `json:"qty"`
		}
		history := make([]HistPoint, 0, 6)
		now := time.Now()

		for i := 5; i >= 0; i-- {
			d := now.AddDate(0, -i, 0)
			var qty float64
			pc.DB.Table("purchase_items").
				Select("COALESCE(SUM(purchase_items.quantity), 0)").
				Joins("JOIN purchases ON purchases.id = purchase_items.purchase_id").
				Where("purchase_items.product_sku = ? AND MONTH(purchases.date) = ? AND YEAR(purchases.date) = ?", topSKU, int(d.Month()), d.Year()).
				Scan(&qty)
			history = append(history, HistPoint{Month: d.Format("Jan 2006"), Qty: qty})
		}

		n := float64(len(history))
		var sumX, sumY, sumXY, sumX2 float64
		for i, h := range history {
			x := float64(i + 1)
			y := h.Qty
			sumX += x
			sumY += y
			sumXY += x * y
			sumX2 += x * x
		}

		denom := (n * sumX2) - (sumX * sumX)
		var b float64
		if denom != 0 {
			b = ((n * sumXY) - (sumX * sumY)) / denom
		}
		a := (sumY - (b * sumX)) / n
		predVal := a + (b * (n + 1))
		if predVal < 0 {
			predVal = 0
		}

		trend := "Cenderung Stabil ➖"
		if b > 0 {
			trend = "Meningkat Tajam 📈"
		} else if b < 0 {
			trend = "Menurun 📉"
		}

		predictions = gin.H{
			"product":               topProducts[0].ProductName,
			"history":               history,
			"next_month_prediction": int(predVal + 0.5),
			"trend":                 trend,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"purchases":   purchases,
		"topProducts": topProducts,
		"predictions": predictions,
	})
}