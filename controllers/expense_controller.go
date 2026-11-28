// // controllers/expense_controller.go
// package controllers

// import (
// 	"net/http"
// 	"time"

// 	"github.com/gin-gonic/gin"
// 	"gorm.io/gorm"

// 	"accounting-be/models"
// )

// type ExpenseController struct {
// 	DB *gorm.DB
// }

// type ExpenseInput struct {
// 	Date        string  `json:"date" binding:"required"`
// 	ReferenceNo string  `json:"reference_no"`
// 	ContactID   *uint   `json:"contact_id"`
// 	AccountCode string  `json:"account_code" binding:"required"`
// 	PaymentCode string  `json:"payment_code" binding:"required"`
// 	Amount      float64 `json:"amount" binding:"required,min=1"`
// 	Description string  `json:"description" binding:"required"`
// }

// // GET /api/expenses
// func (ec *ExpenseController) GetExpenses(c *gin.Context) {
// 	var expenses []models.Expense
// 	if err := ec.DB.Preload("Contact").Preload("Account").Preload("PaymentAcc").Order("date desc, id desc").Find(&expenses).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data biaya"})
// 		return
// 	}
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "data": expenses})
// }

// // POST /api/expenses (Buat Biaya & Jurnal Otomatis)
// func (ec *ExpenseController) CreateExpense(c *gin.Context) {
// 	var input ExpenseInput
// 	if err := c.ShouldBindJSON(&input); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tidak valid: " + err.Error()})
// 		return
// 	}

// 	parsedDate, _ := time.Parse("2006-01-02", input.Date)

// 	tx := ec.DB.Begin()

// 	// 1. Simpan Data Pengeluaran Biaya
// 	expense := models.Expense{
// 		Date:        parsedDate,
// 		ReferenceNo: input.ReferenceNo,
// 		ContactID:   input.ContactID,
// 		AccountCode: input.AccountCode,
// 		PaymentCode: input.PaymentCode,
// 		Amount:      input.Amount,
// 		Description: input.Description,
// 	}

// 	if err := tx.Create(&expense).Error; err != nil {
// 		tx.Rollback()
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan pengeluaran"})
// 		return
// 	}

// 	// 2. Buat Jurnal Otomatis (Debit: Akun Beban, Kredit: Akun Kas/Bank)
// 	j1 := models.Journal{Date: parsedDate, Description: "Biaya: " + input.Description}
// 	if input.ReferenceNo != "" {
// 		j1.Description += " (Ref: " + input.ReferenceNo + ")"
// 	}
// 	tx.Create(&j1)

// 	// Debit: Beban
// 	tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: input.AccountCode, Debit: input.Amount, Credit: 0})
// 	// Kredit: Kas/Bank
// 	tx.Create(&models.JournalEntry{JournalID: j1.ID, AccountCode: input.PaymentCode, Debit: 0, Credit: input.Amount})

// 	tx.Commit()
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Biaya berhasil dicatat beserta Jurnalnya!"})
// }

// // DELETE /api/expenses/:id
// func (ec *ExpenseController) DeleteExpense(c *gin.Context) {
// 	id := c.Param("id")
// 	if err := ec.DB.Delete(&models.Expense{}, id).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus biaya"})
// 		return
// 	}
// 	// Catatan: Di sistem ERP sungguhan, Jurnal tidak boleh dihapus, melainkan di-reverse. 
// 	// Namun untuk penyederhanaan saat ini, kita biarkan jurnal lama tetap ada atau dihapus manual.
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Biaya berhasil dihapus"})
// }

// controllers/expense_controller.go
package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"accounting-be/models"
)

type ExpenseController struct {
	DB *gorm.DB
}

type ExpenseInput struct {
	Date        string  `json:"date" binding:"required"`
	ReferenceNo string  `json:"reference_no"`
	ContactID   *uint   `json:"contact_id"`
	AccountCode string  `json:"account_code" binding:"required"`
	PaymentCode string  `json:"payment_code" binding:"required"`
	Amount      float64 `json:"amount" binding:"required,min=1"`
	PpnAmount   float64 `json:"ppn_amount"`
	PphAmount   float64 `json:"pph_amount"`
	Description string  `json:"description" binding:"required"`
}

func (ec *ExpenseController) createExpenseJournalEntries(tx *gorm.DB, journalID uint, input ExpenseInput) {
	ppn := input.PpnAmount
	pph := input.PphAmount
	netAmount := input.Amount + ppn - pph

	// 1. Jurnal Beban (Debit)
	tx.Create(&models.JournalEntry{
		JournalID:   journalID,
		AccountCode: input.AccountCode,
		Debit:       input.Amount,
		Credit:      0,
	})

	// 2. Jurnal PPN Masukan (Debit) jika ada
	if ppn > 0 {
		var ppnAcc models.Account
		tx.Where("name LIKE ?", "%PPN Masukan%").First(&ppnAcc)
		code := ppnAcc.Code
		if code == "" {
			code = input.AccountCode
		}
		tx.Create(&models.JournalEntry{
			JournalID:   journalID,
			AccountCode: code,
			Debit:       ppn,
			Credit:      0,
		})
	}

	// 3. Jurnal Hutang PPh (Kredit) jika ada
	if pph > 0 {
		var pphAcc models.Account
		tx.Where("name LIKE ? AND code LIKE ?", "%PPh%", "2%").First(&pphAcc)
		code := pphAcc.Code
		if code == "" {
			code = input.PaymentCode
		}
		tx.Create(&models.JournalEntry{
			JournalID:   journalID,
			AccountCode: code,
			Debit:       0,
			Credit:      pph,
		})
	}

	// 4. Jurnal Kas/Bank (Kredit sebesar Netto)
	tx.Create(&models.JournalEntry{
		JournalID:   journalID,
		AccountCode: input.PaymentCode,
		Debit:       0,
		Credit:      netAmount,
	})
}

// GET /api/expenses?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD
func (ec *ExpenseController) GetExpenses(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	query := ec.DB.Preload("Contact").Preload("Account").Preload("PaymentAcc")
	if startDate != "" {
		query = query.Where("DATE(date) >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("DATE(date) <= ?", endDate)
	}

	var expenses []models.Expense
	if err := query.Order("date desc, id desc").Find(&expenses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data biaya"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": expenses})
}

// POST /api/expenses
func (ec *ExpenseController) CreateExpense(c *gin.Context) {
	var input ExpenseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tidak valid: " + err.Error()})
		return
	}

	parsedDate, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid"})
		return
	}

	tx := ec.DB.Begin()

	expense := models.Expense{
		Date:        parsedDate,
		ReferenceNo: input.ReferenceNo,
		ContactID:   input.ContactID,
		AccountCode: input.AccountCode,
		PaymentCode: input.PaymentCode,
		Amount:      input.Amount,
		PpnAmount:   input.PpnAmount,
		PphAmount:   input.PphAmount,
		Description: input.Description,
	}

	if err := tx.Create(&expense).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan pengeluaran"})
		return
	}

	jDesc := "Biaya: " + input.Description
	if input.ReferenceNo != "" {
		jDesc += " (Ref: " + input.ReferenceNo + ")"
	}
	journalCode := fmt.Sprintf("EXP-%s-%d", time.Now().Format("20060102"), expense.ID)

	journal := models.Journal{
		Date:        parsedDate,
		Description: jDesc,
		ReferenceNo: journalCode,
	}
	if err := tx.Create(&journal).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat jurnal biaya"})
		return
	}

	ec.createExpenseJournalEntries(tx, journal.ID, input)

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Biaya dan Pajaknya berhasil dicatat beserta Jurnalnya!",
		"data":    expense,
	})
}

// PUT /api/expenses/:id
func (ec *ExpenseController) UpdateExpense(c *gin.Context) {
	id := c.Param("id")
	var input ExpenseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tidak valid: " + err.Error()})
		return
	}

	parsedDate, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid"})
		return
	}

	var expense models.Expense
	if err := ec.DB.First(&expense, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Catatan biaya tidak ditemukan"})
		return
	}

	oldDateStr := expense.Date.Format("20060102")
	tx := ec.DB.Begin()

	expense.Date = parsedDate
	expense.ReferenceNo = input.ReferenceNo
	expense.ContactID = input.ContactID
	expense.AccountCode = input.AccountCode
	expense.PaymentCode = input.PaymentCode
	expense.Amount = input.Amount
	expense.PpnAmount = input.PpnAmount
	expense.PphAmount = input.PphAmount
	expense.Description = input.Description

	if err := tx.Save(&expense).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui catatan biaya"})
		return
	}

	jDesc := "Biaya: " + input.Description
	if input.ReferenceNo != "" {
		jDesc += " (Ref: " + input.ReferenceNo + ")"
	}

	// Cari jurnal terkait (EXP-YYYYMMDD-{id})
	var journal models.Journal
	oldJournalCode := fmt.Sprintf("EXP-%s-%d", oldDateStr, expense.ID)
	if err := tx.Where("reference_no = ?", oldJournalCode).First(&journal).Error; err == nil {
		journal.Date = parsedDate
		journal.Description = jDesc
		tx.Save(&journal)

		tx.Where("journal_id = ?", journal.ID).Delete(&models.JournalEntry{})
		ec.createExpenseJournalEntries(tx, journal.ID, input)
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Catatan biaya dan jurnal akuntansinya berhasil diperbarui.",
		"data":    expense,
	})
}

// DELETE /api/expenses/:id
func (ec *ExpenseController) DeleteExpense(c *gin.Context) {
	id := c.Param("id")
	if err := ec.DB.Delete(&models.Expense{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus biaya"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Catatan biaya berhasil dihapus."})
}

func (ec *ExpenseController) generateKbdNumber() string {
	now := time.Now()
	month := int(now.Month())
	year := now.Year()

	var existingKbdCount int64
	ec.DB.Model(&models.Expense{}).
		Where("YEAR(date) = ? AND MONTH(date) = ? AND kbd_no IS NOT NULL AND kbd_no != ''", year, month).
		Distinct("kbd_no").
		Count(&existingKbdCount)

	nextSeq := existingKbdCount + 1
	return fmt.Sprintf("%03d/KBD-MPM/%s/%d", nextSeq, romanMonths[now.Month()], year)
}

type KbdExportInput struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}

// POST /api/expenses/kbd (Generate / Ambil Nomor KBD untuk item terpilih)
func (ec *ExpenseController) GenerateKbd(c *gin.Context) {
	var input KbdExportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih minimal 1 pengeluaran untuk dicetak."})
		return
	}

	var expenses []models.Expense
	ec.DB.Where("id IN ?", input.IDs).Order("date asc, id asc").Find(&expenses)
	if len(expenses) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data pengeluaran tidak ditemukan."})
		return
	}

	kbdNo := ""
	if expenses[0].KbdNo != nil && *expenses[0].KbdNo != "" {
		kbdNo = *expenses[0].KbdNo
	} else {
		kbdNo = ec.generateKbdNumber()
		ec.DB.Model(&models.Expense{}).Where("id IN ?", input.IDs).Update("kbd_no", kbdNo)
		ec.DB.Where("id IN ?", input.IDs).Order("date asc, id asc").Find(&expenses)
	}

	var totalNetto float64
	for _, exp := range expenses {
		totalNetto += exp.Amount + exp.PpnAmount - exp.PphAmount
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"kbd_no":   kbdNo,
		"total":    totalNetto,
		"expenses": expenses,
	})
}