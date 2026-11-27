// // controllers/sale_controller.go
// package controllers

// import (
// 	"net/http"
// 	"time"

// 	"github.com/gin-gonic/gin"
// 	"gorm.io/gorm"

// 	"accounting-be/models"
// )

// type SaleController struct {
// 	DB *gorm.DB
// }

// type SaleItemInput struct {
// 	ProductSKU string  `json:"product_sku" binding:"required"`
// 	Quantity   int     `json:"quantity" binding:"required,min=1"`
// 	Price      float64 `json:"price" binding:"required,min=0"`
// }

// type SaleInput struct {
// 	Date        string          `json:"date" binding:"required"`
// 	DueDate     string          `json:"due_date"`
// 	ReferenceNo string          `json:"reference_no"`
// 	ContactID   uint            `json:"contact_id" binding:"required"`
// 	Notes       string          `json:"notes"`
// 	Items       []SaleItemInput `json:"items" binding:"required,min=1"`
// }

// // GET /api/sales
// func (sc *SaleController) GetSales(c *gin.Context) {
// 	var sales []models.Sale
// 	query := sc.DB.Preload("Contact").
// 		Preload("Items").
// 		Preload("Items.Product").
// 		Order("date desc, id desc")

// 	status := c.Query("status")
// 	if status != "" {
// 		query = query.Where("status = ?", status)
// 	}

// 	if err := query.Find(&sales).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data penjualan"})
// 		return
// 	}
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "data": sales})
// }

// // POST /api/sales (Buat Pesanan Penjualan / SO)
// func (sc *SaleController) CreateSale(c *gin.Context) {
// 	var input SaleInput
// 	if err := c.ShouldBindJSON(&input); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
// 		return
// 	}

// 	parsedDate, _ := time.Parse("2006-01-02", input.Date)
// 	parsedDueDate, _ := time.Parse("2006-01-02", input.DueDate)
// 	if input.DueDate == "" { parsedDueDate = parsedDate }

// 	tx := sc.DB.Begin()

// 	sale := models.Sale{
// 		Date:        parsedDate,
// 		DueDate:     parsedDueDate,
// 		ReferenceNo: input.ReferenceNo,
// 		ContactID:   input.ContactID,
// 		Status:      "Pesanan",
// 		Notes:       input.Notes,
// 		Total:       0,
// 	}

// 	if err := tx.Create(&sale).Error; err != nil {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan pesanan penjualan"})
// 		return
// 	}

// 	var grandTotal float64
// 	for _, itemInput := range input.Items {
// 		subTotal := float64(itemInput.Quantity) * itemInput.Price
// 		grandTotal += subTotal

// 		saleItem := models.SaleItem{
// 			SaleID:     sale.ID,
// 			ProductSKU: itemInput.ProductSKU,
// 			Quantity:   itemInput.Quantity,
// 			Price:      itemInput.Price,
// 			SubTotal:   subTotal,
// 		}
// 		if err := tx.Create(&saleItem).Error; err != nil {
// 			tx.Rollback()
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan detail item penjualan"})
// 			return
// 		}
// 	}

// 	if err := tx.Model(&sale).Update("Total", grandTotal).Error; err != nil {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui total penjualan"})
// 		return
// 	}

// 	tx.Commit()
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pesanan Penjualan berhasil dibuat"})
// }

// // PUT /api/sales/:id/deliver (Pengiriman: Otomatis Kurangi Stok Gudang)
// func (sc *SaleController) DeliverSale(c *gin.Context) {
// 	id := c.Param("id")
// 	var sale models.Sale
// 	if err := sc.DB.Preload("Items").First(&sale, id).Error; err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Pesanan tidak ditemukan"})
// 		return
// 	}

// 	if sale.Status != "Pesanan" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Status pesanan tidak valid untuk dikirim"})
// 		return
// 	}

// 	tx := sc.DB.Begin()
// 	sale.Status = "Pengiriman"
// 	if err := tx.Save(&sale).Error; err != nil {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status pengiriman"})
// 		return
// 	}

// 	// Kurangi Stok Produk
// 	for _, item := range sale.Items {
// 		var product models.Product
// 		if err := tx.Where("sku = ?", item.ProductSKU).First(&product).Error; err != nil {
// 			tx.Rollback()
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Produk tidak ditemukan: " + item.ProductSKU})
// 			return
// 		}

// 		if product.Stock < item.Quantity {
// 			tx.Rollback()
// 			c.JSON(http.StatusBadRequest, gin.H{"error": "Stok tidak mencukupi untuk produk: " + product.Name})
// 			return
// 		}

// 		product.Stock -= item.Quantity
// 		if err := tx.Save(&product).Error; err != nil {
// 			tx.Rollback()
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengurangi stok produk"})
// 			return
// 		}
// 	}

// 	tx.Commit()
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Barang berhasil dikirim dan stok gudang dikurangi!"})
// }

// // PUT /api/sales/:id/invoice (Tagihan: Otomatis Buat Jurnal Piutang, Pendapatan, dan HPP)
// func (sc *SaleController) CreateInvoice(c *gin.Context) {
// 	id := c.Param("id")
// 	var sale models.Sale
// 	if err := sc.DB.Preload("Items").First(&sale, id).Error; err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Data pengiriman tidak ditemukan"})
// 		return
// 	}

// 	if sale.Status != "Pengiriman" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya dokumen dengan status 'Pengiriman' yang dapat ditagih"})
// 		return
// 	}

// 	tx := sc.DB.Begin()
// 	sale.Status = "Tagihan"
// 	if err := tx.Save(&sale).Error; err != nil {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status tagihan"})
// 		return
// 	}

// 	// Hitung Total HPP dari item yang terjual
// 	var totalHPP float64
// 	for _, item := range sale.Items {
// 		var prod models.Product
// 		tx.Where("sku = ?", item.ProductSKU).First(&prod)
// 		totalHPP += float64(item.Quantity) * prod.HPP
// 	}

// 	// Cari Akun Akuntansi (Piutang Usaha '112', Pendapatan '411', HPP '511', Persediaan '113')
// 	var arAcc, revAcc, cogsAcc, invAcc models.Account
// 	tx.Where("code LIKE ?", "112%").First(&arAcc)   // Piutang Usaha
// 	tx.Where("code LIKE ?", "411%").First(&revAcc)   // Pendapatan Penjualan
// 	tx.Where("code LIKE ?", "511%").First(&cogsAcc)  // Beban Pokok Penjualan (HPP)
// 	tx.Where("code LIKE ?", "113%").First(&invAcc)   // Persediaan

// 	if arAcc.Code == "" || revAcc.Code == "" {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Konfigurasi Akun Piutang atau Pendapatan di CoA belum lengkap"})
// 		return
// 	}

// 	// // 1. Buat Jurnal Piutang & Pendapatan
// 	// j1 := models.Journal{Date: time.Now(), Description: "Tagihan Penjualan SO #" + id, Reference: "INV-S-" + id}
// 	// tx.Create(&j1)
// 	// tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: arAcc.Code, Debit: sale.Total, Credit: 0})
// 	// tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: revAcc.Code, Debit: 0, Credit: sale.Total})

// 	// // 2. Buat Jurnal HPP & Persediaan (Jika akun HPP & Persediaan tersedia)
// 	// if cogsAcc.Code != "" && invAcc.Code != "" && totalHPP > 0 {
// 	// 	j2 := models.Journal{Date: time.Now(), Description: "HPP Penjualan SO #" + id, Reference: "COGS-S-" + id}
// 	// 	tx.Create(&j2)
// 	// 	tx.Create(&models.JournalEntry{JournalID: j2.ID, AccountCode: cogsAcc.Code, Debit: totalHPP, Credit: 0})
// 	// 	tx.Create(&models.JournalEntry{JournalID: j2.ID, AccountCode: invAcc.Code, Debit: 0, Credit: totalHPP})
// 	// }
	
// 	// 1. Buat Jurnal Piutang & Pendapatan
// 	j1 := models.Journal{Date: time.Now(), Description: "Tagihan Penjualan SO #" + id + " (INV-S-" + id + ")"}
// 	tx.Create(&j1)
// 	tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: arAcc.Code, Debit: sale.Total, Credit: 0})
// 	tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: revAcc.Code, Debit: 0, Credit: sale.Total})

// 	// 2. Buat Jurnal HPP & Persediaan (Jika akun HPP & Persediaan tersedia)
// 	if cogsAcc.Code != "" && invAcc.Code != "" && totalHPP > 0 {
// 		j2 := models.Journal{Date: time.Now(), Description: "HPP Penjualan SO #" + id + " (COGS-S-" + id + ")"}
// 		tx.Create(&j2)
// 		tx.Create(&models.JournalEntry{JournalID: j2.ID, AccountCode: cogsAcc.Code, Debit: totalHPP, Credit: 0})
// 		tx.Create(&models.JournalEntry{JournalID: j2.ID, AccountCode: invAcc.Code, Debit: 0, Credit: totalHPP})
// 	}

// 	tx.Commit()
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Tagihan Penjualan & Jurnal Akuntansi berhasil diterbitkan!"})
// }

// controllers/sale_controller.go
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

type SaleController struct {
	DB *gorm.DB
}

var romanMonths = map[time.Month]string{
	time.January: "I", time.February: "II", time.March: "III", time.April: "IV",
	time.May: "V", time.June: "VI", time.July: "VII", time.August: "VIII",
	time.September: "IX", time.October: "X", time.November: "XI", time.December: "XII",
}

func (sc *SaleController) generateDocNumber(docType string, date time.Time) string {
	month := int(date.Month())
	year := date.Year()
	suffix := fmt.Sprintf("/%s/MPM/%s/%d", docType, romanMonths[date.Month()], year)

	var latestRef string
	if docType == "SO" || docType == "QT" {
		sc.DB.Model(&models.Sale{}).
			Select("reference_no").
			Where("YEAR(date) = ? AND MONTH(date) = ? AND reference_no LIKE ?", year, month, "%"+suffix).
			Order("id DESC").
			Limit(1).
			Scan(&latestRef)
	} else {
		sc.DB.Model(&models.Journal{}).
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

func (sc *SaleController) findDetailAccount(tx *gorm.DB, prefixes ...string) models.Account {
	var acc models.Account
	for _, p := range prefixes {
		if err := tx.Where("code LIKE ? AND is_header = ?", p, false).First(&acc).Error; err == nil && acc.Code != "" {
			return acc
		}
	}
	return acc
}

// GET /api/sales
func (sc *SaleController) GetSales(c *gin.Context) {
	var sales []models.Sale
	query := sc.DB.Preload("Contact").
		Preload("Items").
		Preload("Items.Product").
		Preload("PaymentAccount").
		Order("date desc, id desc")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&sales).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data penjualan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          sales,
		"nextDocNumber": sc.generateDocNumber("SO", time.Now()),
	})
}

type SaleItemInput struct {
	ProductSKU string  `json:"product_sku" binding:"required"`
	Quantity   int     `json:"quantity" binding:"required,min=1"`
	Price      float64 `json:"price" binding:"required,min=0"`
	TaxRate    int     `json:"tax_rate"`
}

type SaleInput struct {
	Date      string          `json:"date" binding:"required"`
	DueDate   string          `json:"due_date"`
	ContactID uint            `json:"contact_id" binding:"required"`
	Notes     string          `json:"notes"`
	Action    string          `json:"action"` // "quotation" atau "so"
	Items     []SaleItemInput `json:"items" binding:"required,min=1"`
}

// POST /api/sales
func (sc *SaleController) CreateSale(c *gin.Context) {
	var input SaleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid: " + err.Error()})
		return
	}

	parsedDate, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid"})
		return
	}
	parsedDueDate := parsedDate
	if input.DueDate != "" {
		if d, err := time.Parse("2006-01-02", input.DueDate); err == nil {
			parsedDueDate = d
		}
	}

	prefix := "SO"
	status := "Menunggu Persetujuan"
	if input.Action == "quotation" {
		prefix = "QT"
		status = "Penawaran"
	}

	refNo := sc.generateDocNumber(prefix, parsedDate)
	tx := sc.DB.Begin()

	sale := models.Sale{
		Date:        parsedDate,
		DueDate:     parsedDueDate,
		ReferenceNo: refNo,
		ContactID:   input.ContactID,
		Status:      status,
		Notes:       input.Notes,
	}

	if err := tx.Create(&sale).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan dokumen penjualan"})
		return
	}

	var grandTotal, grandTax float64
	for _, item := range input.Items {
		dpp := float64(item.Quantity) * item.Price
		taxAmount := dpp * (float64(item.TaxRate) / 100.0)
		subTotal := dpp + taxAmount

		grandTax += taxAmount
		grandTotal += subTotal

		saleItem := models.SaleItem{
			SaleID:     sale.ID,
			ProductSKU: item.ProductSKU,
			Quantity:   item.Quantity,
			Price:      item.Price,
			TaxRate:    item.TaxRate,
			TaxAmount:  taxAmount,
			SubTotal:   subTotal,
		}
		if err := tx.Create(&saleItem).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan item penjualan"})
			return
		}

		// Update harga jual produk terbaru
		tx.Model(&models.Product{}).Where("sku = ?", item.ProductSKU).Update("sell_price", item.Price)
	}

	if err := tx.Model(&sale).Updates(map[string]interface{}{
		"total":     grandTotal,
		"total_tax": grandTax,
	}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui total penjualan"})
		return
	}

	tx.Commit()
	msg := "Pesanan Penjualan (SO)"
	if input.Action == "quotation" {
		msg = "Draf Penawaran (Quotation)"
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Dokumen %s berhasil dibuat dengan nomor: %s", msg, refNo),
		"data":    sale,
	})
}

// POST /api/sales/:id/convert (Penawaran -> Menunggu Persetujuan)
func (sc *SaleController) ConvertToSO(c *gin.Context) {
	id := c.Param("id")
	var sale models.Sale
	if err := sc.DB.First(&sale, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dokumen tidak ditemukan"})
		return
	}
	if sale.Status != "Penawaran" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya dokumen draf Penawaran yang dapat diubah ke SO"})
		return
	}

	refNo := sc.generateDocNumber("SO", time.Now())
	sale.Status = "Menunggu Persetujuan"
	sale.ReferenceNo = refNo
	sale.Notes = strings.TrimSpace(sale.Notes + "\n[Sistem] Klien Setuju. Diubah dari Draf Penawaran ke SO pada " + time.Now().Format("02/01/2006"))

	sc.DB.Save(&sale)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Penawaran berhasil diubah menjadi Pesanan Penjualan (SO): " + refNo})
}

// POST /api/sales/:id/approve
func (sc *SaleController) ApproveSale(c *gin.Context) {
	id := c.Param("id")
	var sale models.Sale
	if err := sc.DB.First(&sale, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesanan tidak ditemukan"})
		return
	}
	if sale.Status != "Menunggu Persetujuan" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dokumen tidak dalam status Menunggu Persetujuan"})
		return
	}

	sale.Status = "Pesanan"
	sale.Notes = strings.TrimSpace(sale.Notes + "\n[DISETUJUI] (" + time.Now().Format("02/01/2006 15:04") + ")")
	sc.DB.Save(&sale)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pesanan Penjualan (SO) berhasil disetujui dan siap dikirim."})
}

// POST /api/sales/:id/reject
func (sc *SaleController) RejectSale(c *gin.Context) {
	id := c.Param("id")
	var sale models.Sale
	if err := sc.DB.First(&sale, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesanan tidak ditemukan"})
		return
	}

	sale.Status = "Ditolak"
	sale.Notes = strings.TrimSpace(sale.Notes + "\n[DITOLAK] (" + time.Now().Format("02/01/2006 15:04") + ")")
	sc.DB.Save(&sale)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pesanan Penjualan (SO) telah ditolak."})
}

// PUT /api/sales/:id/deliver (Pengiriman: Kurangi Stok + Jurnal Piutang/Pendapatan & HPP/Persediaan)
func (sc *SaleController) DeliverSale(c *gin.Context) {
	id := c.Param("id")
	var sale models.Sale
	if err := sc.DB.Preload("Items").First(&sale, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesanan tidak ditemukan"})
		return
	}
	if sale.Status != "Pesanan" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status pesanan tidak valid untuk dikirim"})
		return
	}

	tx := sc.DB.Begin()
	var totalHPP float64
	var deliveryLogs []string

	for _, item := range sale.Items {
		var product models.Product
		if err := tx.Where("sku = ?", item.ProductSKU).First(&product).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Produk tidak ditemukan: " + item.ProductSKU})
			return
		}
		if product.Stock < item.Quantity {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Stok tidak mencukupi untuk produk: " + product.Name})
			return
		}

		product.Stock -= item.Quantity
		if err := tx.Save(&product).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengurangi stok produk"})
			return
		}

		totalHPP += float64(item.Quantity) * product.HPP
		deliveryLogs = append(deliveryLogs, fmt.Sprintf("%s (%d unit)", product.Name, item.Quantity))
	}

	arAcc := sc.findDetailAccount(tx, "12%")
	revAcc := sc.findDetailAccount(tx, "50%", "4%")
	cogsAcc := sc.findDetailAccount(tx, "60%", "5%")
	invAcc := sc.findDetailAccount(tx, "13%", "16%")

	if arAcc.Code == "" || revAcc.Code == "" || cogsAcc.Code == "" || invAcc.Code == "" {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Konfigurasi Akun (Piutang 12%, Persediaan 13%, Pendapatan 50%, HPP 60%) belum lengkap di CoA!"})
		return
	}

	now := time.Now()
	journalCode := sc.generateDocNumber("SJ", now)

	sale.Status = "Pengiriman"
	sale.Notes = strings.TrimSpace(sale.Notes + fmt.Sprintf("\n[No. SJ: %s] %s", journalCode, strings.Join(deliveryLogs, " | ")))
	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui status pengiriman"})
		return
	}

	// Jurnal 1: Piutang (D) vs Pendapatan (K)
	j1 := models.Journal{
		Date:        now,
		Description: fmt.Sprintf("Pengiriman Penjualan SO #%d - Surat Jalan: %s", sale.ID, journalCode),
		ReferenceNo: journalCode,
	}
	tx.Create(&j1)
	tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: arAcc.Code, Debit: sale.Total, Credit: 0})
	tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: revAcc.Code, Debit: 0, Credit: sale.Total})

	// Jurnal 2: HPP (D) vs Persediaan (K)
	if totalHPP > 0 {
		j2 := models.Journal{
			Date:        now,
			Description: fmt.Sprintf("HPP Pengiriman SO #%d (Surat Jalan: %s)", sale.ID, journalCode),
			ReferenceNo: journalCode,
		}
		tx.Create(&j2)
		tx.Create(&models.JournalEntry{JournalID: j2.ID, AccountCode: cogsAcc.Code, Debit: totalHPP, Credit: 0})
		tx.Create(&models.JournalEntry{JournalID: j2.ID, AccountCode: invAcc.Code, Debit: 0, Credit: totalHPP})
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Barang berhasil dikirim dengan Surat Jalan: " + journalCode})
}

type InvoiceInput struct {
	PaymentAccount string `json:"payment_account" binding:"required"`
}

// PUT /api/sales/:id/invoice (Pelunasan Piutang: Kas/Bank (D) vs Piutang (K))
func (sc *SaleController) CreateInvoice(c *gin.Context) {
	id := c.Param("id")
	var input InvoiceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Harap pilih rekening Kas/Bank penerima dana"})
		return
	}

	var sale models.Sale
	if err := sc.DB.Preload("Items").First(&sale, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data pengiriman tidak ditemukan"})
		return
	}
	if sale.Status != "Pengiriman" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya dokumen berstatus Pengiriman yang dapat dilunasi"})
		return
	}

	tx := sc.DB.Begin()
	var bankAcc models.Account
	tx.Where("code = ? AND is_header = ?", input.PaymentAccount, false).First(&bankAcc)
	arAcc := sc.findDetailAccount(tx, "12%")

	if bankAcc.Code == "" || arAcc.Code == "" {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Akun Kas/Bank atau Piutang Usaha (12%) tidak ditemukan di CoA"})
		return
	}

	now := time.Now()
	journalCode := sc.generateDocNumber("GKP", now)

	sale.Status = "Tagihan"
	sale.PaymentAccountCode = &input.PaymentAccount
	sale.Notes = strings.TrimSpace(sale.Notes + "\n[Lunas] Diterbitkan Invoice Nomor: " + journalCode)
	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui status tagihan"})
		return
	}

	j1 := models.Journal{
		Date:        now,
		Description: fmt.Sprintf("Pelunasan Piutang SO #%d - Invoice: %s", sale.ID, journalCode),
		ReferenceNo: journalCode,
	}
	tx.Create(&j1)
	tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: bankAcc.Code, Debit: sale.Total, Credit: 0})
	tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: arAcc.Code, Debit: 0, Credit: sale.Total})

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pembayaran berhasil! Invoice bernomor " + journalCode + " telah diterbitkan."})
}

type ReturnInput struct {
	Reason string `json:"reason" binding:"required"`
}

// POST /api/sales/:id/return (Retur Penjualan / Credit Note)
func (sc *SaleController) ProcessReturn(c *gin.Context) {
	id := c.Param("id")
	var input ReturnInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Alasan retur wajib diisi"})
		return
	}

	var sale models.Sale
	if err := sc.DB.Preload("Items").First(&sale, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesanan tidak ditemukan"})
		return
	}
	if sale.Status != "Pengiriman" && sale.Status != "Tagihan" && sale.Status != "Selesai" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya pesanan yang sudah terkirim yang dapat diproses retur"})
		return
	}

	tx := sc.DB.Begin()
	var totalHPP float64
	for _, item := range sale.Items {
		var product models.Product
		if err := tx.Where("sku = ?", item.ProductSKU).First(&product).Error; err == nil {
			product.Stock += item.Quantity
			tx.Save(&product)
			totalHPP += float64(item.Quantity) * product.HPP
		}
	}

	arAcc := sc.findDetailAccount(tx, "12%")
	revAcc := sc.findDetailAccount(tx, "50%", "4%")
	cogsAcc := sc.findDetailAccount(tx, "60%", "5%")
	invAcc := sc.findDetailAccount(tx, "13%", "16%")

	now := time.Now()
	retCode := sc.generateDocNumber("RET", now)

	if revAcc.Code != "" && arAcc.Code != "" {
		j1 := models.Journal{
			Date:        now,
			Description: fmt.Sprintf("Retur Penjualan (Credit Note) SO #%d - %s", sale.ID, input.Reason),
			ReferenceNo: retCode,
		}
		tx.Create(&j1)
		tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: revAcc.Code, Debit: sale.Total, Credit: 0})
		tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: arAcc.Code, Debit: 0, Credit: sale.Total})
	}

	if totalHPP > 0 && invAcc.Code != "" && cogsAcc.Code != "" {
		j2 := models.Journal{
			Date:        now,
			Description: fmt.Sprintf("Pembalikan HPP Retur SO #%d", sale.ID),
			ReferenceNo: retCode,
		}
		tx.Create(&j2)
		tx.Create(&models.JournalEntry{JournalID: j2.ID, AccountCode: invAcc.Code, Debit: totalHPP, Credit: 0})
		tx.Create(&models.JournalEntry{JournalID: j2.ID, AccountCode: cogsAcc.Code, Debit: 0, Credit: totalHPP})
	}

	sale.Status = "Diretur"
	sale.Notes = strings.TrimSpace(sale.Notes + fmt.Sprintf("\n[RETUR] %s (No. Dokumen: %s)", input.Reason, retCode))
	tx.Save(&sale)

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Retur Penjualan (Credit Note) berhasil diproses: " + retCode})
}

// GET /api/sales/aging (Laporan Umur Piutang / AR Aging)
func (sc *SaleController) GetReceivableAging(c *gin.Context) {
	var unpaidSales []models.Sale
	sc.DB.Preload("Contact").
		Where("status = ?", "Pengiriman").
		Order("due_date asc").
		Find(&unpaidSales)

	now := time.Now().Truncate(24 * time.Hour)
	agingData := map[string]float64{
		"belum_jatuh_tempo": 0,
		"1_30_hari":         0,
		"31_60_hari":        0,
		"61_90_hari":        0,
		"lebih_90_hari":     0,
	}

	details := make([]map[string]interface{}, 0, len(unpaidSales))
	var totalPiutang float64

	for _, sale := range unpaidSales {
		due := sale.DueDate.Truncate(24 * time.Hour)
		diffDays := int(now.Sub(due).Hours() / 24)

		category := "belum_jatuh_tempo"
		daysOverdue := 0

		if diffDays <= 0 {
			agingData["belum_jatuh_tempo"] += sale.Total
		} else {
			daysOverdue = diffDays
			switch {
			case daysOverdue <= 30:
				category = "1_30_hari"
				agingData["1_30_hari"] += sale.Total
			case daysOverdue <= 60:
				category = "31_60_hari"
				agingData["31_60_hari"] += sale.Total
			case daysOverdue <= 90:
				category = "61_90_hari"
				agingData["61_90_hari"] += sale.Total
			default:
				category = "lebih_90_hari"
				agingData["lebih_90_hari"] += sale.Total
			}
		}

		totalPiutang += sale.Total
		ref := sale.ReferenceNo
		if ref == "" {
			ref = fmt.Sprintf("SO #%d", sale.ID)
		}

		details = append(details, map[string]interface{}{
			"id":           sale.ID,
			"reference_no": ref,
			"customer":     sale.Contact.Name,
			"phone":        sale.Contact.Phone,
			"due_date":     sale.DueDate.Format("2006-01-02"),
			"total":        sale.Total,
			"category":     category,
			"days_overdue": daysOverdue,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"agingData":    agingData,
		"details":      details,
		"totalPiutang": totalPiutang,
	})
}