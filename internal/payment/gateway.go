package payment

import (
	"log"
	"time"
)

// CreateQrisRequest is the input to create a QRIS charge.
type CreateQrisRequest struct {
	OrderCode string
	Amount    int64
	ExpiresAt time.Time
}

// CreateQrisResponse carries the QR payload and gateway payment id.
type CreateQrisResponse struct {
	QrString  string
	PaymentID string
	ExpiresAt time.Time
}

// WebhookEvent is the normalized payment notification from the gateway.
type WebhookEvent struct {
	PaymentID string `json:"payment_id"`
	OrderCode string `json:"order_code"`
	Status    string `json:"status"` // paid | expired | failed
	Amount    int64  `json:"amount"`
}

// Gateway abstracts the QRIS payment provider so the rest of the app is
// provider-agnostic. The mock implementation is used for development.
type Gateway interface {
	CreateQRIS(req CreateQrisRequest) (*CreateQrisResponse, error)
	// VerifyWebhook validates the raw payload against the signature and
	// returns the parsed event. It must return an error on signature mismatch.
	VerifyWebhook(rawPayload []byte, signature string) (*WebhookEvent, error)
}

// New returns the configured gateway. Currently only "mock" is implemented;
// real adapters (Midtrans/Xendit) can be added by implementing Gateway.
func New(name, webhookSecret, midtransKey string, prod bool) Gateway {
	switch name {
	case "mock", "":
		return NewMock(webhookSecret)
	case "midtrans":
		if midtransKey == "" {
			log.Fatal("MIDTRANS_SERVER_KEY wajib diisi")
		}
		return NewMidtrans(midtransKey, prod)
	default:
		log.Fatalf("PAYMENT_GATEWAY tidak dikenal: %q", name)
		return nil
	}
}
