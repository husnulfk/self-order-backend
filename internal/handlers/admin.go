package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"selforder/backend/internal/models"
	"selforder/backend/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
)

// ---- Orders (kasir + admin) ----

// GET /api/admin/orders — list orders. Defaults to active (paid/preparing/ready);
// pass ?status= to filter by a specific status (pending_payment is never shown by default).
func (h *Handlers) ListOrders(c *gin.Context) {
	status := c.Query("status")
	base := h.DB.Preload("Items").Preload("Table").Preload("Payment").Order("created_at desc")

	var orders []models.Order
	var err error
	switch status {
	case "", "active":
		err = base.Where("status IN ?", []string{models.OrderPaid, models.OrderPreparing, models.OrderReady}).Find(&orders).Error
	default:
		err = base.Where("status = ?", status).Find(&orders).Error
	}
	validStatuses := map[string]bool{
		"paid": true, "preparing": true, "ready": true,
		"completed": true, "expired": true, "pending_payment": true,
	}
	if status != "" && status != "active" && !validStatuses[status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status tidak valid"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat pesanan"})
		return
	}

	out := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		out = append(out, orderToDTO(o))
	}
	c.JSON(http.StatusOK, gin.H{"orders": out})
}

type statusReq struct {
	Status string `json:"status" binding:"required"`
}

// allowedNext enforces the PRD state machine: paid -> preparing -> ready -> completed.
var allowedNext = map[string]string{
	models.OrderPaid:      models.OrderPreparing,
	models.OrderPreparing: models.OrderReady,
	models.OrderReady:     models.OrderCompleted,
}

// PATCH /api/admin/orders/:id/status — move an order to the next valid status.
func (h *Handlers) UpdateOrderStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id tidak valid"})
		return
	}

	var req statusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
		return
	}

	var order models.Order
	if err := h.DB.Preload("Items").Preload("Table").Preload("Payment").First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pesanan tidak ditemukan"})
		return
	}

	next, ok := allowedNext[order.Status]
	if !ok || req.Status != next {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("transisi status tidak valid: %s -> %s", order.Status, req.Status)})
		return
	}

	if err := h.DB.Model(&order).Update("status", req.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui status"})
		return
	}
	order.Status = req.Status

	c.JSON(http.StatusOK, orderToDTO(order))
}

// ---- Menu items (admin only) ----

// GET /api/admin/menu-items
func (h *Handlers) ListMenuItems(c *gin.Context) {
	var items []models.MenuItem
	if err := h.DB.Preload("Category").Order("category_id asc, name asc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat menu"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type menuItemReq struct {
	CategoryID  uint   `json:"category_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Price       int64  `json:"price" binding:"required,min=1"`
	IsAvailable *bool  `json:"is_available"`
}

// POST /api/admin/menu-items
func (h *Handlers) CreateMenuItem(c *gin.Context) {
	var req menuItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
		return
	}

	var cat models.Category
	if err := h.DB.First(&cat, req.CategoryID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kategori tidak ditemukan"})
		return
	}

	item := models.MenuItem{
		CategoryID:  req.CategoryID,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		IsAvailable: req.IsAvailable == nil || *req.IsAvailable,
		IsActive:    true,
	}
	if err := h.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan item"})
		return
	}

	c.JSON(http.StatusCreated, item)
}

type menuItemUpdateReq struct {
	CategoryID  *uint   `json:"category_id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Price       *int64  `json:"price" binding:"omitempty,min=1"`
	IsAvailable *bool   `json:"is_available"`
}

// PATCH /api/admin/menu-items/:id
func (h *Handlers) UpdateMenuItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id tidak valid"})
		return
	}

	var item models.MenuItem
	if err := h.DB.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item tidak ditemukan"})
		return
	}

	var req menuItemUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
		return
	}

	if req.CategoryID != nil {
		var cat models.Category
		if err := h.DB.First(&cat, *req.CategoryID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "kategori tidak ditemukan"})
			return
		}
		item.CategoryID = *req.CategoryID
	}
	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.Description != nil {
		item.Description = *req.Description
	}
	if req.Price != nil {
		item.Price = *req.Price
	}
	if req.IsAvailable != nil {
		item.IsAvailable = *req.IsAvailable
	}

	if err := h.DB.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui item"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// DELETE /api/admin/menu-items/:id — soft delete (kept for order history).
func (h *Handlers) DeleteMenuItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id tidak valid"})
		return
	}

	var item models.MenuItem
	if err := h.DB.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item tidak ditemukan"})
		return
	}

	if err := h.DB.Model(&item).Update("is_active", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menghapus item"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type availabilityReq struct {
	IsAvailable bool `json:"is_available"`
}

// PATCH /api/admin/menu-items/:id/availability — toggle "Habis".
func (h *Handlers) ToggleAvailability(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id tidak valid"})
		return
	}

	var item models.MenuItem
	if err := h.DB.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item tidak ditemukan"})
		return
	}

	var req availabilityReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
		return
	}

	item.IsAvailable = req.IsAvailable
	if err := h.DB.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui ketersediaan"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// ---- Categories (admin only) ----

// GET /api/admin/categories
func (h *Handlers) ListCategories(c *gin.Context) {
	var cats []models.Category
	if err := h.DB.Order("sort_order asc, id asc").Find(&cats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat kategori"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": cats})
}

type categoryReq struct {
	Name      string `json:"name" binding:"required"`
	SortOrder int    `json:"sort_order"`
}

// POST /api/admin/categories
func (h *Handlers) CreateCategory(c *gin.Context) {
	var req categoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
		return
	}
	cat := models.Category{Name: req.Name, SortOrder: req.SortOrder}
	if err := h.DB.Create(&cat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan kategori"})
		return
	}
	c.JSON(http.StatusCreated, cat)
}

// ---- Tables (admin only) ----

// GET /api/admin/tables
func (h *Handlers) ListTables(c *gin.Context) {
	var tables []models.Table
	if err := h.DB.Order("number asc").Find(&tables).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat meja"})
		return
	}

	out := make([]gin.H, 0, len(tables))
	for _, t := range tables {
		out = append(out, tableToDTO(h.BaseURL, t))
	}
	c.JSON(http.StatusOK, gin.H{"tables": out})
}

type tableReq struct {
	Number string `json:"number" binding:"required"`
}

// POST /api/admin/tables — create a table with a fresh token.
func (h *Handlers) CreateTable(c *gin.Context) {
	var req tableReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
		return
	}
	t := models.Table{Number: req.Number, Token: util.TableToken(), IsActive: true}
	if err := h.DB.Create(&t).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan meja"})
		return
	}
	c.JSON(http.StatusCreated, tableToDTO(h.BaseURL, t))
}

type tableUpdateReq struct {
	Number   *string `json:"number" binding:"omitempty,min=1"`
	IsActive *bool   `json:"is_active"`
}

// PATCH /api/admin/tables/:id — rename or enable/disable a table.
func (h *Handlers) UpdateTable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id tidak valid"})
		return
	}

	var t models.Table
	if err := h.DB.First(&t, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "meja tidak ditemukan"})
		return
	}

	var req tableUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
		return
	}
	if req.Number != nil {
		t.Number = *req.Number
	}
	if req.IsActive != nil {
		t.IsActive = *req.IsActive
	}

	if err := h.DB.Save(&t).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui meja"})
		return
	}
	c.JSON(http.StatusOK, tableToDTO(h.BaseURL, t))
}

// GET /api/admin/tables/:id/qr — downloadable QR PNG encoding the table URL.
func (h *Handlers) TableQR(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id tidak valid"})
		return
	}

	var t models.Table
	if err := h.DB.First(&t, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "meja tidak ditemukan"})
		return
	}

	url := h.BaseURL + "/t/" + t.Token
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat QR"})
		return
	}
	c.Data(http.StatusOK, "image/png", png)
}

func tableToDTO(baseURL string, t models.Table) gin.H {
	return gin.H{
		"id":        t.ID,
		"number":    t.Number,
		"token":     t.Token,
		"is_active": t.IsActive,
		"qr_url":    baseURL + "/t/" + t.Token,
	}
}
