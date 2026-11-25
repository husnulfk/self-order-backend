package payment

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
)

type MidtransGateway struct {
	client     coreapi.Client
	serverKey  string
	production bool
}

func NewMidtrans(serverKey string, production bool) *MidtransGateway {
	env := midtrans.Sandbox
	if production {
		env = midtrans.Production
	}
	var c coreapi.Client
	c.New(serverKey, env)
	midtrans.DefaultLoggerLevel = &midtrans.LoggerImplementation{LogLevel: midtrans.LogError}
	return &MidtransGateway{client: c, serverKey: serverKey, production: production}
}

func (g *MidtransGateway) CreateQRIS(req CreateQrisRequest) (*CreateQrisResponse, error) {
	res, err := g.client.ChargeTransaction(&coreapi.ChargeReq{
		PaymentType: coreapi.PaymentTypeQris,
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  req.OrderCode,
			GrossAmt: req.Amount,
		},
		CustomExpiry: &coreapi.CustomExpiry{
			ExpiryDuration: max(1, int(time.Until(req.ExpiresAt).Minutes())),
			Unit:           "minute",
		},
	})
	if err != nil {
		log.Printf("midtrans charge failed: status=%d msg=%s", err.StatusCode, err.Message)
		return nil, err
	}
	if !g.production {
		for _, a := range res.Actions {
			if strings.HasPrefix(a.Name, "generate-qr-code") {
				log.Printf("[sandbox] QR image URL (untuk simulator): %s", a.URL)
			}
		}
	}
	if res.QRString == "" {
		return nil, errors.New("midtrans: empty qr_string")
	}
	return &CreateQrisResponse{
		QrString:  res.QRString,
		PaymentID: res.TransactionID,
		ExpiresAt: req.ExpiresAt,
	}, nil
}

// Midtrans mengirim signature di dalam body, jadi parameter `signature`
// (dari header) tidak dipakai.
func (g *MidtransGateway) VerifyWebhook(raw []byte, _ string) (*WebhookEvent, error) {

	var n struct {
		OrderID string `json:"order_id"`
	}
	if err := json.Unmarshal(raw, &n); err != nil || n.OrderID == "" {
		return nil, errors.New("invalid webhook payload")
	}

	// Tangani dummy/sample order dari tombol test dashboard Midtrans
	orderLower := strings.ToLower(n.OrderID)
	if strings.Contains(orderLower, "test") || strings.Contains(orderLower, "sample") || strings.Contains(orderLower, "dummy") {
		return &WebhookEvent{
			PaymentID: "TEST-" + n.OrderID,
			OrderCode: n.OrderID,
			Status:    "paid",
			Amount:    0,
		}, nil
	}

	res, merr := g.client.CheckTransaction(n.OrderID)
	if merr != nil {
		return nil, merr
	}
	if res.StatusCode == "404" {
		return nil, errors.New("transaction not found at midtrans")
	}

	amt, _ := strconv.ParseFloat(res.GrossAmount, 64) // "55000.00"

	status := "pending"
	switch res.TransactionStatus {
	case "settlement":
		status = "paid"
	case "expire":
		status = "expired"
	case "cancel", "deny", "failure":
		status = "failed"
	}

	return &WebhookEvent{
		PaymentID: res.TransactionID,
		OrderCode: n.OrderID,
		Status:    status,
		Amount:    int64(amt),
	}, nil
}
