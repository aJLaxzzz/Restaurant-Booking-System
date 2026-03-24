package payment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CreateStripeCheckoutSession создаёт сессию оплаты (депозит).
func CreateStripeCheckoutSession(secretKey string, amountKopecks int, successURL string, metadata map[string]string) (sessionURL string, err error) {
	if secretKey == "" {
		return "", fmt.Errorf("stripe not configured")
	}
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", successURL+"?paid=1")
	form.Set("cancel_url", successURL)
	form.Set("line_items[0][price_data][currency]", "rub")
	form.Set("line_items[0][price_data][unit_amount]", fmt.Sprintf("%d", amountKopecks))
	form.Set("line_items[0][price_data][product_data][name]", "Депозит бронирования")
	form.Set("line_items[0][quantity]", "1")
	for k, v := range metadata {
		form.Set("metadata["+k+"]", v)
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(secretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		URL    string `json:"url"`
		Error  *struct{ Message string `json:"message"` } `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode >= 400 || out.Error != nil {
		msg := ""
		if out.Error != nil {
			msg = out.Error.Message
		}
		return "", fmt.Errorf("stripe: %s", msg)
	}
	return out.URL, nil
}
