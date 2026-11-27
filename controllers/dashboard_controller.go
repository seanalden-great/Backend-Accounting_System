// controllers/dashboard_controller.go
package controllers

import (
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DashboardController struct {
	DB *gorm.DB
}

type AlertRow struct {
	ID          uint    `json:"id"`
	ReferenceNo string  `json:"reference_no"`
	ContactName string  `json:"contact_name"`
	DueDate     string  `json:"due_date"`
	Total       float64 `json:"total"`
}

type BankDetail struct {
	Name    string  `json:"name"`
	Code    string  `json:"code"`
	Balance float64 `json:"balance"`
}

type LowStockRow struct {
	ID         uint    `json:"id"`
	Name       string  `json:"name"`
	SKU        string  `json:"sku"`
	TotalStock float64 `json:"total_stock"`
}

type TopProductRow struct {
	ProductSKU  string  `json:"product_sku"`
	ProductName string  `json:"product_name"`
	TotalQty    float64 `json:"total_qty"`
}

type TopCustomerRow struct {
	ContactID   uint    `json:"contact_id"`
	ContactName string  `json:"contact_name"`
	TotalSpent  float64 `json:"total_spent"`
}

type RecentActivityRow struct {
	ID          uint   `json:"id"`
	ReferenceNo string `json:"reference_no"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

func (dc *DashboardController) GetDashboard(c *gin.Context) {
	now := time.Now()
	currentMonth := int(now.Month())
	currentYear := now.Year()

	today := now.Format("2006-01-02")
	sevenDays := now.AddDate(0, 0, 7).Format("2006-01-02")

	// 1. SISTEM PERINGATAN DINI (KLASIFIKASI 3 LEVEL)
	var poAlertsRaw, soAlertsRaw []AlertRow

	dc.DB.Table("purchases").
		Select("purchases.id, purchases.reference_no, COALESCE(contacts.name, '-') AS contact_name, DATE_FORMAT(purchases.due_date, '%Y-%m-%d') AS due_date, purchases.total").
		Joins("LEFT JOIN contacts ON contacts.id = purchases.contact_id").
		Where("purchases.status IN ? AND DATE(purchases.due_date) <= ?", []string{"Pesanan", "Pengiriman"}, sevenDays).
		Order("purchases.due_date ASC").
		Scan(&poAlertsRaw)

	dc.DB.Table("sales").
		Select("sales.id, sales.reference_no, COALESCE(contacts.name, '-') AS contact_name, DATE_FORMAT(sales.due_date, '%Y-%m-%d') AS due_date, sales.total").
		Joins("LEFT JOIN contacts ON contacts.id = sales.contact_id").
		Where("sales.status IN ? AND DATE(sales.due_date) <= ?", []string{"Pesanan", "Pengiriman"}, sevenDays).
		Order("sales.due_date ASC").
		Scan(&soAlertsRaw)

	poOverdue, poToday, poUpcoming := []AlertRow{}, []AlertRow{}, []AlertRow{}
	for _, item := range poAlertsRaw {
		if item.DueDate < today {
			poOverdue = append(poOverdue, item)
		} else if item.DueDate == today {
			poToday = append(poToday, item)
		} else {
			poUpcoming = append(poUpcoming, item)
		}
	}

	soOverdue, soToday, soUpcoming := []AlertRow{}, []AlertRow{}, []AlertRow{}
	for _, item := range soAlertsRaw {
		if item.DueDate < today {
			soOverdue = append(soOverdue, item)
		} else if item.DueDate == today {
			soToday = append(soToday, item)
		} else {
			soUpcoming = append(soUpcoming, item)
		}
	}

	totalOverdue := len(poOverdue) + len(soOverdue)
	totalToday := len(poToday) + len(soToday)
	totalUpcoming := len(poUpcoming) + len(soUpcoming)
	totalAlerts := totalOverdue + totalToday + totalUpcoming

	// 2. PENDAPATAN, PENGELUARAN, LABA BERSIH
	var pendapatanBulanIni, pembelianBulanIni, biayaBulanIni float64

	dc.DB.Table("sales").
		Select("COALESCE(SUM(total), 0)").
		Where("status IN ? AND MONTH(date) = ? AND YEAR(date) = ?", []string{"Pengiriman", "Tagihan"}, currentMonth, currentYear).
		Scan(&pendapatanBulanIni)

	dc.DB.Table("purchases").
		Select("COALESCE(SUM(total), 0)").
		Where("status IN ? AND MONTH(date) = ? AND YEAR(date) = ?", []string{"Pengiriman", "Tagihan"}, currentMonth, currentYear).
		Scan(&pembelianBulanIni)

	dc.DB.Table("expenses").
		Select("COALESCE(SUM(amount), 0)").
		Where("MONTH(date) = ? AND YEAR(date) = ?", currentMonth, currentYear).
		Scan(&biayaBulanIni)

	pengeluaranBulanIni := pembelianBulanIni + biayaBulanIni
	labaBersih := pendapatanBulanIni - pengeluaranBulanIni

	// 3. SALDO KAS & BANK
	type bankAccount struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	var bankAccounts []bankAccount
	dc.DB.Table("accounts").
		Select("code, name").
		Where("(code LIKE '10%' OR code LIKE '11%') AND is_header = ?", false).
		Scan(&bankAccounts)

	var bankCodes []string
	for _, b := range bankAccounts {
		bankCodes = append(bankCodes, b.Code)
	}

	var totalKasBank float64
	rekeningDetails := []BankDetail{}

	if len(bankCodes) > 0 {
		dc.DB.Table("journal_entries").
			Select("COALESCE(SUM(debit) - SUM(credit), 0)").
			Where("account_code IN ?", bankCodes).
			Scan(&totalKasBank)

		for _, b := range bankAccounts {
			var bal float64
			dc.DB.Table("journal_entries").
				Select("COALESCE(SUM(debit) - SUM(credit), 0)").
				Where("account_code = ?", b.Code).
				Scan(&bal)
			if bal != 0 {
				rekeningDetails = append(rekeningDetails, BankDetail{
					Name:    b.Name,
					Code:    b.Code,
					Balance: bal,
				})
			}
		}
	}

	// 4. PIUTANG & HUTANG
	piutangList := []AlertRow{}
	dc.DB.Table("sales").
		Select("sales.id, sales.reference_no, COALESCE(contacts.name, '-') AS contact_name, DATE_FORMAT(sales.due_date, '%Y-%m-%d') AS due_date, sales.total").
		Joins("LEFT JOIN contacts ON contacts.id = sales.contact_id").
		Where("sales.status = ?", "Pengiriman").
		Order("sales.due_date ASC").
		Scan(&piutangList)

	var totalPiutang float64
	for _, p := range piutangList {
		totalPiutang += p.Total
	}

	hutangList := []AlertRow{}
	dc.DB.Table("purchases").
		Select("purchases.id, purchases.reference_no, COALESCE(contacts.name, '-') AS contact_name, DATE_FORMAT(purchases.due_date, '%Y-%m-%d') AS due_date, purchases.total").
		Joins("LEFT JOIN contacts ON contacts.id = purchases.contact_id").
		Where("purchases.status = ?", "Pengiriman").
		Order("purchases.due_date ASC").
		Scan(&hutangList)

	var totalHutang float64
	for _, h := range hutangList {
		totalHutang += h.Total
	}

	// 5. GRAFIK ARUS KAS (6 BULAN TERAKHIR)
	chartLabels := make([]string, 0, 6)
	chartInflows := make([]float64, 0, 6)
	chartOutflows := make([]float64, 0, 6)

	for i := 5; i >= 0; i-- {
		d := now.AddDate(0, -i, 0)
		chartLabels = append(chartLabels, d.Format("Jan 2006"))

		var inflow, outflow float64
		if len(bankCodes) > 0 {
			dc.DB.Table("journal_entries").
				Select("COALESCE(SUM(journal_entries.debit), 0)").
				Joins("JOIN journals ON journals.id = journal_entries.journal_id").
				Where("journal_entries.account_code IN ? AND MONTH(journals.date) = ? AND YEAR(journals.date) = ?", bankCodes, int(d.Month()), d.Year()).
				Scan(&inflow)

			dc.DB.Table("journal_entries").
				Select("COALESCE(SUM(journal_entries.credit), 0)").
				Joins("JOIN journals ON journals.id = journal_entries.journal_id").
				Where("journal_entries.account_code IN ? AND MONTH(journals.date) = ? AND YEAR(journals.date) = ?", bankCodes, int(d.Month()), d.Year()).
				Scan(&outflow)
		}

		chartInflows = append(chartInflows, inflow)
		chartOutflows = append(chartOutflows, outflow)
	}

	// 6. TOP 5 PRODUK TERLARIS BULAN INI
	topProducts := []TopProductRow{}
	dc.DB.Table("sale_items").
		Select("sale_items.product_sku, COALESCE(products.name, sale_items.product_sku) AS product_name, SUM(sale_items.quantity) AS total_qty").
		Joins("JOIN sales ON sales.id = sale_items.sale_id").
		Joins("LEFT JOIN products ON products.sku = sale_items.product_sku").
		Where("MONTH(sales.date) = ? AND YEAR(sales.date) = ?", currentMonth, currentYear).
		Group("sale_items.product_sku, products.name").
		Order("total_qty DESC").
		Limit(5).
		Scan(&topProducts)

	// 7. AKTIVITAS JURNAL TERAKHIR
	recentActivities := []RecentActivityRow{}
	dc.DB.Table("journals").
		Select("id, reference_no, description, DATE_FORMAT(date, '%Y-%m-%d') AS date").
		Order("id DESC").
		Limit(5).
		Scan(&recentActivities)

	// 8. KPI BISNIS: PROFIT MARGIN & TARGET PENJUALAN
	var profitMargin float64
	if pendapatanBulanIni > 0 {
		profitMargin = math.Round((labaBersih/pendapatanBulanIni)*1000) / 10
	}
	targetPenjualan := 250000000.0
	var persentaseTarget float64
	if pendapatanBulanIni > 0 {
		persentaseTarget = math.Min(100, math.Round((pendapatanBulanIni/targetPenjualan)*1000)/10)
	}

	// 9. PERINGATAN STOK MENIPIS (<= 10)
	// Sesuaikan nama tabel "stocks" jika di DB Anda bernama "product_stocks" atau "inventories"
	lowStocks := []LowStockRow{}
	dc.DB.Table("products").
		Select("products.id, products.name, products.sku, COALESCE(SUM(stocks.quantity), 0) AS total_stock").
		Joins("LEFT JOIN stocks ON stocks.product_id = products.id").
		Group("products.id, products.name, products.sku").
		Having("COALESCE(SUM(stocks.quantity), 0) <= 10").
		Order("total_stock ASC").
		Limit(4).
		Scan(&lowStocks)

	// 10. TOP 5 KLIEN (TOP SPENDER)
	topCustomers := []TopCustomerRow{}
	dc.DB.Table("sales").
		Select("sales.contact_id, COALESCE(contacts.name, 'Pelanggan Umum') AS contact_name, SUM(sales.total) AS total_spent").
		Joins("LEFT JOIN contacts ON contacts.id = sales.contact_id").
		Where("MONTH(sales.date) = ? AND YEAR(sales.date) = ? AND sales.status IN ?", currentMonth, currentYear, []string{"Pengiriman", "Tagihan"}).
		Group("sales.contact_id, contacts.name").
		Order("total_spent DESC").
		Limit(5).
		Scan(&topCustomers)

	c.JSON(http.StatusOK, gin.H{
		"pendapatanBulanIni":  pendapatanBulanIni,
		"pengeluaranBulanIni": pengeluaranBulanIni,
		"pembelianBulanIni":   pembelianBulanIni,
		"biayaBulanIni":       biayaBulanIni,
		"labaBersih":          labaBersih,
		"totalKasBank":        totalKasBank,
		"rekeningDetails":     rekeningDetails,
		"bankAccounts":        bankAccounts,
		"piutangList":         piutangList,
		"totalPiutang":        totalPiutang,
		"hutangList":          hutangList,
		"totalHutang":         totalHutang,
		"chartLabels":         chartLabels,
		"chartInflows":        chartInflows,
		"chartOutflows":       chartOutflows,
		"poOverdue":           poOverdue,
		"poToday":             poToday,
		"poUpcoming":          poUpcoming,
		"soOverdue":           soOverdue,
		"soToday":             soToday,
		"soUpcoming":          soUpcoming,
		"totalOverdue":        totalOverdue,
		"totalToday":          totalToday,
		"totalUpcoming":       totalUpcoming,
		"totalAlerts":         totalAlerts,
		"topProducts":         topProducts,
		"recentActivities":    recentActivities,
		"profitMargin":        profitMargin,
		"targetPenjualan":     targetPenjualan,
		"persentaseTarget":    persentaseTarget,
		"lowStocks":           lowStocks,
		"topCustomers":        topCustomers,
	})
}