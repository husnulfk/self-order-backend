package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"selforder/backend/internal/models"
	"selforder/backend/internal/payment"
	"selforder/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GET /api/public/tables/:token — validate table token, return table info.
func (h *Handlers) GetTableByToken(c *gin.Context) {
	token := c.Param("token")
	var table models.Table
	if err := h.DB.Where("token = ?", token).First(&table).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "meja tidak ditemukan"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat meja"})
		}
		return
	}
	if !table.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "meja tidak aktif"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":     table.ID,
		"number": table.Number,
	})
}

// GET /api/public/menu — categories + active items.
func (h *Handlers) GetMenu(c *gin.Context) {
	var categories []models.Category
	if err := h.DB.Order("sort_order asc, id asc").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat menu"})
		return
	}

	var items []models.MenuItem
	if err := h.DB.Where("is_active = ?", true).Order("category_id asc, name asc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat menu"})
		return
	}

	type itemDTO struct {
		ID          uint   `json:"id"`
		CategoryID  uint   `json:"category_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Price       int64  `json:"price"`
		IsAvailable bool   `json:"is_available"`
	}
	byCat := make(map[uint][]itemDTO, len(categories))
	for _, it := range items {
		byCat[it.CategoryID] = append(byCat[it.CategoryID], itemDTO{
			ID:          it.ID,
			CategoryID:  it.CategoryID,
			Name:        it.Name,
			Description: it.Description,
			Price:       it.Price,
			IsAvailable: it.IsAvailable,
		})
	}

	type catDTO struct {
		ID    uint      `json:"id"`
		Name  string    `json:"name"`
		Items []itemDTO `json:"items"`
	}
	out := make([]catDTO, 0, len(categories))
	for _, cat := range categories {
		its := byCat[cat.ID]
		if its == nil {
			its = []itemDTO{}
		}
		out = append(out, catDTO{ID: cat.ID, Name: cat.Name, Items: its})
	}

	c.JSON(http.StatusOK, gin.H{"categories": out})
}

type createOrderReq struct {
	TableToken string         `json:"table_token" binding:"required"`
	Items      []orderItemReq `json:"items" binding:"required,min=1,dive"`
}

type orderItemReq struct {
	MenuItemID uint   `json:"menu_item_id" binding:"required"`
	Qty        int    `json:"qty" binding:"required,min=1"`
	Note       string `json:"note"`
}

// POST /api/public/orders — create order. Total is always recomputed server-side.
func (h *Handlers) CreateOrder(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload tidak valid"})
		return
	}

	var table models.Table
	if err := h.DB.Where("token = ?", req.TableToken).First(&table).Error; err != nil || !table.IsActive {
		c.JSON(http.StatusNotFound, gin.H{"error": "meja tidak valid atau tidak aktif"})
		return
	}

	type line struct {
		menu models.MenuItem
		qty  int
		note string
	}
	lines := make([]line, 0, len(req.Items))
	var total int64

	for _, it := range req.Items {
		var m models.MenuItem
		if err := h.DB.First(&m, it.MenuItemID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "menu tidak ditemukan"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memuat menu"})
			}
			return
		}
		if !m.IsActive || !m.IsAvailable {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("item '%s' sedang tidak tersedia", m.Name)})
			return
		}
		total += m.Price * int64(it.Qty)
		lines = append(lines, line{menu: m, qty: it.Qty, note: it.Note})
	}

	if total <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "keranjang kosong"})
		return
	}

	code := util.OrderCode()
	expiresAt := time.Now().Add(h.OrderExpiry)

	qris, err := h.Gateway.CreateQRIS(payment.CreateQrisRequest{
		OrderCode: code,
		Amount:    total,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat pembayaran"})
		return
	}

	order := models.Order{
		Code:        code,
		TableID:     table.ID,
		Status:      models.OrderPendingPayment,
		Total:       total,
		QRString:    qris.QrString,
		QRPaymentID: qris.PaymentID,
		ExpiresAt:   qris.ExpiresAt,
	}

	items := make([]models.OrderItem, 0, len(lines))
	for _, ln := range lines {
		items = append(items, models.OrderItem{
			MenuItemID: ln.menu.ID,
			Name:       ln.menu.Name,
			Price:      ln.menu.Price,
			Qty:        ln.qty,
			Note:       ln.note,
		})
	}

	pay := models.Payment{
		GatewayPaymentID: qris.PaymentID,
		Status:           models.PaymentPending,
		Amount:           total,
		QRString:         qris.QrString,
		ExpiresAt:        qris.ExpiresAt,
	}

	// Write order, its line items and the payment atomically so a partial
	// failure never leaves an order without its items or payment.
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].OrderID = order.ID
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		pay.OrderID = order.ID
		return tx.Create(&pay).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal menyimpan pesanan"})
		return
	}

	qrCodeURL := fmt.Sprintf("https://api.sandbox.midtrans.com/v2/qris/%s/qr-code", pay.GatewayPaymentID)

	c.JSON(http.StatusCreated, gin.H{
		"code":        order.Code,
		"total":       order.Total,
		"qr_code_url": qrCodeURL,
		"expires_at":  pay.ExpiresAt,
	})
}

// GET /api/public/orders/:code — order status + payment info.
func (h *Handlers) GetOrderByCode(c *gin.Context) {
	code := c.Param("code")
	var order models.Order
	if err := h.DB.Preload("Items").Preload("Table").Preload("Payment").Where("code = ?", code).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pesanan tidak ditemukan"})
		return
	}

	paidAt := order.Payment.PaidAt
	if paidAt == nil {
		paidAt = order.PaidAt
	}

	resp := gin.H{
		"code":         order.Code,
		"status":       order.Status,
		"total":        order.Total,
		"table_number": order.Table.Number,
		"items":        itemsToDTO(order.Items),
		"created_at":   order.CreatedAt,
		"paid_at":      paidAt,
	}
	if order.Status == models.OrderPendingPayment {
		qrString := order.Payment.QRString
		if qrString == "" {
			qrString = order.QRString
		}
		expiresAt := order.Payment.ExpiresAt
		if expiresAt.IsZero() {
			expiresAt = order.ExpiresAt
		}
		resp["qr_string"] = qrString
		resp["expires_at"] = expiresAt
	}

	c.JSON(http.StatusOK, resp)
}
