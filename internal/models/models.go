package models

import "time"

// Table represents a physical cafe table identified by an unguessable token.
type Table struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Number    string    `gorm:"uniqueIndex;not null" json:"number"`
	Token     string    `gorm:"uniqueIndex;not null" json:"token"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Category groups menu items (e.g. Kopi, Non-Kopi, Makanan).
type Category struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MenuItem is a sellable item. Price is stored as integer rupiah (no decimals).
type MenuItem struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CategoryID  uint      `gorm:"index;not null" json:"category_id"`
	Category    Category  `json:"category,omitempty"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	Price       int64     `gorm:"not null" json:"price"`
	IsAvailable bool      `gorm:"default:true" json:"is_available"` // toggle "Habis"
	IsActive    bool      `gorm:"default:true" json:"is_active"`    // soft delete
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// User is a cashier or admin account.
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"not null" json:"-"`
	Role         string    `gorm:"not null" json:"role"` // "admin" | "cashier"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Order statuses per PRD:
// pending_payment -> paid -> preparing -> ready -> completed
// branches: expired / cancelled
const (
	OrderPendingPayment = "pending_payment"
	OrderPaid           = "paid"
	OrderPreparing      = "preparing"
	OrderReady          = "ready"
	OrderCompleted      = "completed"
	OrderExpired        = "expired"
	OrderCancelled      = "cancelled"
)

// Payment statuses (gateway-level).
const (
	PaymentPending = "pending"
	PaymentPaid    = "paid"
	PaymentExpired = "expired"
	PaymentFailed  = "failed"
)

// Order is a customer order tied to a table. Payment details live on the
// associated Payment record so the two can be written/updated atomically.
type Order struct {
	ID          uint        `gorm:"primaryKey" json:"id"`
	Code        string      `gorm:"uniqueIndex;not null" json:"code"`
	TableID     uint        `gorm:"index;not null" json:"table_id"`
	Table       Table       `json:"table,omitempty"`
	Status      string      `gorm:"index;not null" json:"status"`
	Total       int64       `gorm:"not null" json:"total"`
	QRString    string      `json:"qr_string,omitempty"`
	QRPaymentID string      `gorm:"column:qr_payment_id" json:"qr_payment_id,omitempty"`
	ExpiresAt   time.Time   `json:"expires_at"`
	PaidAt      *time.Time  `json:"paid_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Items       []OrderItem `json:"items,omitempty"`
	Payment     Payment     `json:"payment,omitempty"`
}

// Payment is a single QRIS charge for an order. It owns the gateway payment id
// (idempotency key), the QR payload, the expiry and the paid timestamp.
type Payment struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	OrderID          uint       `gorm:"index;not null" json:"order_id"`
	GatewayPaymentID string     `gorm:"uniqueIndex;not null" json:"gateway_payment_id"`
	Status           string     `gorm:"not null" json:"status"`
	Amount           int64      `gorm:"not null" json:"amount"`
	QRString         string     `json:"qr_string,omitempty"`
	ExpiresAt        time.Time  `json:"expires_at"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// OrderItem is a snapshot line item so order history survives menu edits.
type OrderItem struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OrderID    uint      `gorm:"index;not null" json:"order_id"`
	MenuItemID uint      `json:"menu_item_id"`
	Name       string    `gorm:"not null" json:"name"`
	Price      int64     `gorm:"not null" json:"price"`
	Qty        int       `gorm:"not null" json:"qty"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}
