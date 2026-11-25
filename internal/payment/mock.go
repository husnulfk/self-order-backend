package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"selforder/backend/internal/util"
)

// MockGateway is a deterministic stub for local development. The QR string it
// produces is not a valid QRIS payload; it only needs to be renderable so the
// frontend payment flow can be exercised end-to-end.
type MockGateway struct {
	secret string
}

func NewMock(secret string) *MockGateway {
	return &MockGateway{secret: secret}
}

func (m *MockGateway) CreateQRIS(req CreateQrisRequest) (*CreateQrisResponse, error) {
	pid := util.RandomID("MOCK")
	qr := fmt.Sprintf("000201010212|SELFORDER-MOCK|%s|%s|%d", pid, req.OrderCode, req.Amount)
	return &CreateQrisResponse{
		QrString:  qr,
		PaymentID: pid,
		ExpiresAt: req.ExpiresAt,
	}, nil
}

func (m *MockGateway) VerifyWebhook(rawPayload []byte, signature string) (*WebhookEvent, error) {
	if m.secret != "" {
		if signature == "" || !hmac.Equal([]byte(signature), []byte(m.sign(rawPayload))) {
			return nil, errors.New("invalid webhook signature")
		}
	}

	var ev WebhookEvent
	if err := json.Unmarshal(rawPayload, &ev); err != nil {
		return nil, err
	}
	if ev.PaymentID == "" || ev.OrderCode == "" {
		return nil, errors.New("missing payment_id or order_code")
	}
	return &ev, nil
}

// sign computes the HMAC-SHA256 hex signature for a payload.
func (m *MockGateway) sign(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(m.secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
