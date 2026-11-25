package jobs

import (
	"log"
	"time"

	"selforder/backend/internal/models"

	"gorm.io/gorm"
)

// StartOrderExpiry runs a ticker that marks pending_payment orders as expired
// once their ExpiresAt has passed. Per PRD this runs every minute.
func StartOrderExpiry(db *gorm.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			now := time.Now()
			var expiredIDs []uint
			if err := db.Model(&models.Order{}).
				Where("status = ? AND (expires_at < ? OR id IN (SELECT order_id FROM payments WHERE expires_at < ?))",
					models.OrderPendingPayment, now, now).
				Pluck("id", &expiredIDs).Error; err != nil {
				log.Println("expire job error:", err)
				continue
			}

			if len(expiredIDs) > 0 {
				err := db.Transaction(func(tx *gorm.DB) error {
					if err := tx.Model(&models.Order{}).
						Where("id IN ?", expiredIDs).
						Update("status", models.OrderExpired).Error; err != nil {
						return err
					}
					return tx.Model(&models.Payment{}).
						Where("order_id IN ?", expiredIDs).
						Update("status", models.PaymentExpired).Error
				})
				if err != nil {
					log.Println("expire job error updating status:", err)
					continue
				}
				log.Printf("expire job: expired %d order(s)", len(expiredIDs))
			}
		}
	}()
}
