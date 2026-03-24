package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"restaurant-booking/internal/payment"
)

type webhookBody struct {
	PaymentID        uuid.UUID `json:"payment_id"`
	Status           string    `json:"status"`
	GatewayPaymentID string    `json:"gateway_payment_id"`
}

func (a *Handlers) finalizePaymentSuccess(ctx context.Context, paymentID uuid.UUID, gwPaymentID string, rawJSON []byte) error {
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var resID uuid.UUID
	var payStatus string
	err = tx.QueryRow(ctx, `SELECT reservation_id, status FROM payments WHERE id=$1 FOR UPDATE`, paymentID).
		Scan(&resID, &payStatus)
	if err != nil {
		return err
	}
	if payStatus == "succeeded" {
		_ = tx.Rollback(ctx)
		return nil
	}

	_, err = tx.Exec(ctx, `
		UPDATE payments SET status='succeeded', gateway_payment_id=$2, gateway_response=$3::jsonb, updated_at=NOW() WHERE id=$1`,
		paymentID, gwPaymentID, string(rawJSON))
	if err != nil {
		return err
	}

	var tableID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT table_id FROM reservations WHERE id=$1`, resID).Scan(&tableID); err != nil {
		return err
	}
	_, _ = tx.Exec(ctx, `UPDATE reservations SET status='confirmed', updated_at=NOW() WHERE id=$1`, resID)
	_, _ = tx.Exec(ctx, `UPDATE tables SET status='occupied', updated_at=NOW() WHERE id=$1`, tableID)

	var hallID uuid.UUID
	_ = tx.QueryRow(ctx, `SELECT hall_id FROM tables WHERE id=$1`, tableID).Scan(&hallID)

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	_ = a.RDB.Del(ctx, "table:"+tableID.String()+":lock").Err()
	ts := time.Now().UTC().Format(time.RFC3339)
	a.emitHallEvent(hallID, map[string]any{
		"event": "table.booked", "table_id": tableID.String(), "status": "occupied",
		"reservation_id": resID.String(), "timestamp": ts,
	})
	a.emitHallEvent(hallID, map[string]any{
		"event": "reservation.status_changed", "reservation_id": resID.String(),
		"status": "confirmed", "timestamp": ts,
	})
	_ = a.publishEvent(ctx, "payment.succeeded", map[string]any{
		"payment_id": paymentID.String(), "reservation_id": resID.String(),
	})
	return nil
}

func (a *Handlers) handleWebhook(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		a.err(w, http.StatusBadRequest, "body")
		return
	}
	sig := r.Header.Get("X-Signature")
	mac := hmac.New(sha256.New, []byte(a.Cfg.WebhookSecret))
	mac.Write(raw)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		a.err(w, http.StatusForbidden, "подпись")
		return
	}
	var wb webhookBody
	if err := json.Unmarshal(raw, &wb); err != nil {
		a.err(w, http.StatusBadRequest, "json")
		return
	}
	if wb.Status != "succeeded" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := a.finalizePaymentSuccess(r.Context(), wb.PaymentID, wb.GatewayPaymentID, raw); err != nil {
		if err == pgx.ErrNoRows {
			a.err(w, http.StatusNotFound, "платёж")
			return
		}
		a.err(w, http.StatusInternalServerError, "обработка")
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *Handlers) handleSimulatePay(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	pid, err := uuid.Parse(chi.URLParam(r, "pid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var resID, owner uuid.UUID
	var payStatus string
	err = a.Pool.QueryRow(r.Context(), `
		SELECT p.reservation_id, p.status, r.user_id FROM payments p
		JOIN reservations r ON r.id = p.reservation_id WHERE p.id=$1`, pid).
		Scan(&resID, &payStatus, &owner)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "нет платежа")
		return
	}
	if owner != u.ID && u.Role != "admin" && u.Role != "owner" {
		a.err(w, http.StatusForbidden, "чужой платёж")
		return
	}
	if payStatus != "pending" {
		a.err(w, http.StatusConflict, "уже обработан")
		return
	}
	wb := webhookBody{PaymentID: pid, Status: "succeeded", GatewayPaymentID: "sim_" + pid.String()[:8]}
	raw, _ := json.Marshal(wb)
	if err := a.finalizePaymentSuccess(r.Context(), pid, wb.GatewayPaymentID, raw); err != nil {
		a.err(w, http.StatusInternalServerError, "ошибка")
		return
	}
	a.json(w, http.StatusOK, map[string]string{"status": "succeeded"})
}

func (a *Handlers) handlePaymentGet(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	pid, err := uuid.Parse(chi.URLParam(r, "pid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var resID uuid.UUID
	var amount int
	var status string
	var idem uuid.UUID
	err = a.Pool.QueryRow(r.Context(), `
		SELECT reservation_id, amount_kopecks, status, idempotency_key FROM payments WHERE id=$1`, pid).
		Scan(&resID, &amount, &status, &idem)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "нет")
		return
	}
	var owner uuid.UUID
	_ = a.Pool.QueryRow(r.Context(), `SELECT user_id FROM reservations WHERE id=$1`, resID).Scan(&owner)
	if owner != u.ID && u.Role != "admin" && u.Role != "owner" {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	a.json(w, http.StatusOK, map[string]any{
		"id": pid.String(), "reservation_id": resID.String(), "amount_kopecks": amount,
		"status": status, "idempotency_key": idem.String(),
	})
}

func (a *Handlers) handleRefund(w http.ResponseWriter, r *http.Request) {
	pid, _ := uuid.Parse(chi.URLParam(r, "pid"))
	var amount int
	var st string
	err := a.Pool.QueryRow(r.Context(), `SELECT amount_kopecks, status FROM payments WHERE id=$1`, pid).Scan(&amount, &st)
	if err != nil {
		a.err(w, http.StatusNotFound, "нет")
		return
	}
	if st != "succeeded" {
		a.err(w, http.StatusConflict, "статус")
		return
	}
	_, _ = a.Pool.Exec(r.Context(), `UPDATE payments SET status='refunded', refund_amount_kopecks=$2, updated_at=NOW() WHERE id=$1`, pid, amount)
	w.WriteHeader(http.StatusNoContent)
}

// handlePaymentCheckout — редирект на YooKassa или Stripe; если ключей нет — только демо.
func (a *Handlers) handlePaymentCheckout(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	pid, err := uuid.Parse(chi.URLParam(r, "pid"))
	if err != nil {
		a.err(w, http.StatusBadRequest, "id")
		return
	}
	var resID, owner uuid.UUID
	var amount int
	var st string
	err = a.Pool.QueryRow(r.Context(), `
		SELECT p.reservation_id, p.amount_kopecks, p.status, r.user_id FROM payments p
		JOIN reservations r ON r.id = p.reservation_id WHERE p.id=$1`, pid).
		Scan(&resID, &amount, &st, &owner)
	if err == pgx.ErrNoRows {
		a.err(w, http.StatusNotFound, "нет платежа")
		return
	}
	if owner != u.ID && u.Role != "admin" && u.Role != "owner" {
		a.err(w, http.StatusForbidden, "нет доступа")
		return
	}
	if st != "pending" {
		a.err(w, http.StatusConflict, "уже обработан")
		return
	}
	returnURL := a.Cfg.PublicAppURL + "/pay/" + pid.String()
	meta := map[string]string{"internal_payment_id": pid.String(), "reservation_id": resID.String()}

	if a.Cfg.YooKassaShopID != "" && a.Cfg.YooKassaSecretKey != "" {
		gwID, url, err := payment.CreateYooKassaPayment(
			a.Cfg.YooKassaShopID, a.Cfg.YooKassaSecretKey, amount, returnURL, uuid.NewString(), meta)
		if err != nil {
			a.err(w, http.StatusBadGateway, err.Error())
			return
		}
		_, _ = a.Pool.Exec(r.Context(), `UPDATE payments SET gateway_payment_id=$2, updated_at=NOW() WHERE id=$1`, pid, gwID)
		a.json(w, http.StatusOK, map[string]any{"provider": "yookassa", "url": url})
		return
	}
	if a.Cfg.StripeSecretKey != "" {
		url, err := payment.CreateStripeCheckoutSession(a.Cfg.StripeSecretKey, amount, returnURL, meta)
		if err != nil {
			a.err(w, http.StatusBadGateway, err.Error())
			return
		}
		a.json(w, http.StatusOK, map[string]any{"provider": "stripe", "url": url})
		return
	}
	a.json(w, http.StatusOK, map[string]any{"provider": "demo", "url": returnURL, "hint": "задайте YOOKASSA_* или STRIPE_SECRET_KEY"})
}

func (a *Handlers) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		a.err(w, http.StatusBadRequest, "body")
		return
	}
	sig := r.Header.Get("Stripe-Signature")
	if a.Cfg.StripeWebhookSecret != "" && sig != "" {
		// Упрощённо: в проде используйте stripe.ConstructEvent
		if !strings.Contains(sig, "t=") {
			a.err(w, http.StatusForbidden, "подпись")
			return
		}
	}
	var ev struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				Metadata map[string]string `json:"metadata"`
				ID       string            `json:"id"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		a.err(w, http.StatusBadRequest, "json")
		return
	}
	if ev.Type != "checkout.session.completed" {
		w.WriteHeader(http.StatusOK)
		return
	}
	pidStr := ev.Data.Object.Metadata["internal_payment_id"]
	pid, err := uuid.Parse(pidStr)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	rawCopy := append([]byte(nil), raw...)
	if err := a.finalizePaymentSuccess(r.Context(), pid, ev.Data.Object.ID, rawCopy); err != nil {
		a.err(w, http.StatusInternalServerError, "обработка")
		return
	}
	w.WriteHeader(http.StatusOK)
}
