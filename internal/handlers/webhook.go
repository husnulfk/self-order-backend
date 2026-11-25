package handlers

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"selforder/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errAmountMismatch = errors.New("amount mismatch")
	errUnknownStatus  = errors.New("unknown webhook status")
)

// POST /api/webhooks/payment — payment gateway callback.
// The order row is locked (SELECT ... FOR UPDATE) before the status check so
// concurrent or duplicate webhooks cannot double-process the same order.
// Idempotent: a duplicate (or already-settled) notification returns 200 with
// no double effect. Invalid signature is rejected with 401.
func (h *Handlers) PaymentWebhook(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}

	signature := c.GetHeader("X-Signature")
	if signature == "" {
		signature = c.GetHeader("X-Payment-Signature")
	}

	ev, err := h.Gateway.VerifyWebhook(raw, signature)
	if err != nil {
		log.Printf("webhook verify failed | sig_header=%q | raw=%q | err=%v", signature, string(raw), err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "signature tidak valid"})
		return
	}

	// Tangani notifikasi uji coba (test notification) dari dashboard Midtrans
	codeLower := strings.ToLower(ev.OrderCode)
	if strings.Contains(codeLower, "test") || strings.Contains(codeLower, "sample") || strings.Contains(codeLower, "dummy") {
		log.Printf("test notification received from gateway | order_code=%s", ev.OrderCode)
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "test notification received"})
		return
	}

	var status string
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		var order models.Order
		// Lock the order row to serialize concurrent webhooks for this order.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code = ?", ev.OrderCode).
			First(&order).Error; err != nil {
			return err
		}
		status = order.Status

		// Idempotency: only a pending_payment order can transition.
		if order.Status != models.OrderPendingPayment {
			if ev.Status == "paid" && order.Status != models.OrderPaid {
				log.Printf("WARNING: paid webhook untuk order %s berstatus %s", order.Code, order.Status)
			}
			return nil
		}

		switch ev.Status {
		case "paid":
			if ev.Amount > 0 && ev.Amount != order.Total {
				return errAmountMismatch
			}
			now := time.Now()
			if err := tx.Model(&order).Updates(map[string]interface{}{
				"status":  models.OrderPaid,
				"paid_at": &now,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.Payment{}).
				Where("order_id = ?", order.ID).
				Updates(map[string]interface{}{
					"status":  models.PaymentPaid,
					"paid_at": &now,
				}).Error; err != nil {
				return err
			}
			status = models.OrderPaid
			return nil

		case "pending":
			return nil

		case "expired", "failed":
			payStatus := models.PaymentExpired
			if ev.Status == "failed" {
				payStatus = models.PaymentFailed
			}
			if err := tx.Model(&order).Update("status", models.OrderExpired).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.Payment{}).
				Where("order_id = ?", order.ID).
				Update("status", payStatus).Error; err != nil {
				return err
			}
			status = models.OrderExpired
			return nil

		default:
			return errUnknownStatus
		}

	})

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "order tidak ditemukan"})
	case errors.Is(err, errAmountMismatch):
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount mismatch"})
	case errors.Is(err, errUnknownStatus):
		c.JSON(http.StatusBadRequest, gin.H{"error": "status webhook tidak dikenal"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal memperbarui pesanan"})
	default:
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": status})
	}
}
