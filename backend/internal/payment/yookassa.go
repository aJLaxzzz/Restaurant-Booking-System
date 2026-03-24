package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ykCreateReq struct {
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Confirmation struct {
		Type      string `json:"type"`
		ReturnURL string `json:"return_url"`
	} `json:"confirmation"`
	Capture bool   `json:"capture"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ykCreateResp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Confirmation struct {
		ConfirmationURL string `json:"confirmation_url"`
	} `json:"confirmation"`
}

// CreateYooKassaPayment возвращает URL оплаты (confirmation_url).
func CreateYooKassaPayment(shopID, secretKey string, amountKopecks int, returnURL string, idempotencyKey string, metadata map[string]string) (paymentID, confirmationURL string, err error) {
	if shopID == "" || secretKey == "" {
		return "", "", fmt.Errorf("yookassa not configured")
	}
	rub := fmt.Sprintf("%.2f", float64(amountKopecks)/100.0)
	var req ykCreateReq
	req.Amount.Value = rub
	req.Amount.Currency = "RUB"
	req.Confirmation.Type = "redirect"
	req.Confirmation.ReturnURL = returnURL
	req.Capture = true
	req.Metadata = metadata
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, "https://api.yookassa.ru/v3/payments", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	httpReq.SetBasicAuth(shopID, secretKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotence-Key", idempotencyKey)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("yookassa http %d", resp.StatusCode)
	}
	var out ykCreateResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	return out.ID, out.Confirmation.ConfirmationURL, nil
}
