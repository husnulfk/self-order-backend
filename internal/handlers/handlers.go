package handlers

import (
	"time"

	"selforder/backend/internal/models"
	"selforder/backend/internal/payment"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handlers bundles shared dependencies for all HTTP handlers.
type Handlers struct {
	DB          *gorm.DB
	Gateway     payment.Gateway
	BaseURL     string
	OrderExpiry time.Duration
	JWTSecret   string
	JWTExpiry   time.Duration
}

func New(db *gorm.DB, gw payment.Gateway, baseURL string, orderExpiry, jwtExpiry time.Duration, jwtSecret string) *Handlers {
	return &Handlers{
		DB:          db,
		Gateway:     gw,
		BaseURL:     baseURL,
		OrderExpiry: orderExpiry,
		JWTSecret:   jwtSecret,
		JWTExpiry:   jwtExpiry,
	}
}

func itemsToDTO(items []models.OrderItem) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, gin.H{
			"id":           it.ID,
			"menu_item_id": it.MenuItemID,
			"name":         it.Name,
			"price":        it.Price,
			"qty":          it.Qty,
			"note":         it.Note,
		})
	}
	return out
}

func orderToDTO(o models.Order) gin.H {
	tableNumber := ""
	if o.Table.ID != 0 {
		tableNumber = o.Table.Number
	}
	expiresAt := o.Payment.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = o.ExpiresAt
	}
	paidAt := o.Payment.PaidAt
	if paidAt == nil {
		paidAt = o.PaidAt
	}
	return gin.H{
		"id":           o.ID,
		"code":         o.Code,
		"status":       o.Status,
		"total":        o.Total,
		"table_id":     o.TableID,
		"table_number": tableNumber,
		"items":        itemsToDTO(o.Items),
		"created_at":   o.CreatedAt,
		"expires_at":   expiresAt,
		"paid_at":      paidAt,
	}
}
