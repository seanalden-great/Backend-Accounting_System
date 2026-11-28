// // controllers/category_controller.go
// package controllers

// import (
// 	"net/http"

// 	"github.com/gin-gonic/gin"
// 	"gorm.io/gorm"

// 	"accounting-be/models"
// )

// type CategoryController struct {
// 	DB *gorm.DB
// }

// func (cc *CategoryController) GetCategories(c *gin.Context) {
// 	var categories []models.Category
// 	if err := cc.DB.Order("name asc").Find(&categories).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data kategori"})
// 		return
// 	}
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "data": categories})
// }

// func (cc *CategoryController) CreateCategory(c *gin.Context) {
// 	var category models.Category
// 	if err := c.ShouldBindJSON(&category); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid"})
// 		return
// 	}

// 	if err := cc.DB.Create(&category).Error; err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Kategori dengan nama tersebut mungkin sudah ada"})
// 		return
// 	}
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Kategori berhasil disimpan"})
// }

// func (cc *CategoryController) UpdateCategory(c *gin.Context) {
// 	id := c.Param("id")
// 	var category models.Category
	
// 	if err := cc.DB.First(&category, id).Error; err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Kategori tidak ditemukan"})
// 		return
// 	}

// 	if err := c.ShouldBindJSON(&category); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid"})
// 		return
// 	}

// 	if err := cc.DB.Save(&category).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate kategori"})
// 		return
// 	}
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Kategori berhasil diupdate"})
// }

// func (cc *CategoryController) DeleteCategory(c *gin.Context) {
// 	id := c.Param("id")
// 	if err := cc.DB.Delete(&models.Category{}, id).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus kategori"})
// 		return
// 	}
// 	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Kategori berhasil dihapus"})
// }

// controllers/category_controller.go
package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"accounting-be/models"
)

type CategoryController struct {
	DB *gorm.DB
}

type CategoryInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// GET /api/categories
func (cc *CategoryController) GetCategories(c *gin.Context) {
	var categories []models.Category
	if err := cc.DB.Order("name asc").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data kategori"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": categories})
}

// POST /api/categories
func (cc *CategoryController) CreateCategory(c *gin.Context) {
	var input CategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nama kategori wajib diisi"})
		return
	}

	cat := models.Category{
		Name:        input.Name,
		Description: input.Description,
	}
	if err := cc.DB.Create(&cat).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kategori dengan nama tersebut mungkin sudah ada."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Kategori berhasil disimpan.", "data": cat})
}

// PUT /api/categories/:id
func (cc *CategoryController) UpdateCategory(c *gin.Context) {
	id := c.Param("id")
	var cat models.Category
	if err := cc.DB.First(&cat, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kategori tidak ditemukan"})
		return
	}

	var input CategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid"})
		return
	}

	cat.Name = input.Name
	cat.Description = input.Description
	if err := cc.DB.Save(&cat).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal mengupdate kategori. Pastikan nama unik."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Kategori berhasil diperbarui.", "data": cat})
}

// DELETE /api/categories/:id
func (cc *CategoryController) DeleteCategory(c *gin.Context) {
	id := c.Param("id")
	if err := cc.DB.Delete(&models.Category{}, id).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal menghapus kategori. Data mungkin masih terikat dengan produk."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Kategori berhasil dihapus."})
}