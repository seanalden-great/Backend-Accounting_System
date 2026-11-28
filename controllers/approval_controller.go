// controllers/approval_controller.go
package controllers

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"accounting-be/models"
)

type ApprovalController struct {
	DB *gorm.DB
}

type PendingDocument struct {
	ID          uint    `json:"id"`
	Key         string  `json:"key"`
	Type        string  `json:"type"` // "sale" atau "purchase"
	DocType     string  `json:"doc_type"`
	Date        string  `json:"date"`
	ReferenceNo string  `json:"reference_no"`
	Person      string  `json:"person"`
	Amount      float64 `json:"amount"`
	ApproveURL  string  `json:"approve_url"`
	RejectURL   string  `json:"reject_url"`
}

// GET /api/approvals
func (ac *ApprovalController) GetPendingApprovals(c *gin.Context) {
	var sales []models.Sale
	ac.DB.Preload("Contact").
		Where("status = ?", "Menunggu Persetujuan").
		Find(&sales)

	var purchases []models.Purchase
	ac.DB.Preload("Contact").
		Where("status = ?", "Menunggu Persetujuan").
		Find(&purchases)

	pendingDocs := make([]PendingDocument, 0, len(sales)+len(purchases))

	for _, s := range sales {
		person := s.Contact.Name
		if person == "" {
			person = "Pelanggan Umum"
		}
		pendingDocs = append(pendingDocs, PendingDocument{
			ID:          s.ID,
			Key:         fmt.Sprintf("sale-%d", s.ID),
			Type:        "sale",
			DocType:     "Pesanan Penjualan (SO)",
			Date:        s.Date.Format("2006-01-02"),
			ReferenceNo: s.ReferenceNo,
			Person:      person,
			Amount:      s.Total,
			ApproveURL:  fmt.Sprintf("/sales/%d/approve", s.ID),
			RejectURL:   fmt.Sprintf("/sales/%d/reject", s.ID),
		})
	}

	for _, p := range purchases {
		person := p.Contact.Name
		if person == "" {
			person = "Pemasok Umum"
		}
		pendingDocs = append(pendingDocs, PendingDocument{
			ID:          p.ID,
			Key:         fmt.Sprintf("purchase-%d", p.ID),
			Type:        "purchase",
			DocType:     "Pesanan Pembelian (PO)",
			Date:        p.Date.Format("2006-01-02"),
			ReferenceNo: p.ReferenceNo,
			Person:      person,
			Amount:      p.Total,
			ApproveURL:  fmt.Sprintf("/purchases/%d/approve", p.ID),
			RejectURL:   fmt.Sprintf("/purchases/%d/reject", p.ID),
		})
	}

	// Urutkan dari tanggal paling lampau (ascending)
	sort.Slice(pendingDocs, func(i, j int) bool {
		return pendingDocs[i].Date < pendingDocs[j].Date
	})

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   pendingDocs,
	})
}