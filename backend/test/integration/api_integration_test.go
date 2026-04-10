// Интеграционные тесты HTTP API против запущенного монолита (go run ./cmd/api).
// Запуск: из каталога backend — go test ./test/integration/... -v -count=1
// Переменная API_TEST_BASE_URL (по умолчанию http://127.0.0.1:8080).
package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func apiBase(t *testing.T) string {
	t.Helper()
	b := os.Getenv("API_TEST_BASE_URL")
	if b == "" {
		return "http://127.0.0.1:8080"
	}
	return strings.TrimRight(b, "/")
}

func httpClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{
		Timeout: 60 * time.Second,
		Jar:     jar,
	}
}

func mustOK(t *testing.T, resp *http.Response, allowed ...int) []byte {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	ok := false
	for _, c := range allowed {
		if resp.StatusCode == c {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatalf("HTTP %d, body: %s", resp.StatusCode, truncate(string(body), 800))
	}
	return body
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func doJSON(t *testing.T, c *http.Client, method, path, token string, payload any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, apiBase(t)+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func bootstrapTestMenuItem(t *testing.T, c *http.Client, adm string) string {
	t.Helper()
	suf := uuid.New().String()[:8]
	cb := mustOK(t, doJSON(t, c, "POST", "/api/admin/menu/categories", adm, map[string]any{
		"name": "Интеграция " + suf, "sort_order": 0,
	}), http.StatusCreated)
	var cat struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cb, &cat); err != nil {
		t.Fatal(err)
	}
	ib := mustOK(t, doJSON(t, c, "POST", "/api/admin/menu/items", adm, map[string]any{
		"category_id":   cat.ID,
		"name":          "Позиция " + suf,
		"price_kopecks": 50000,
		"sort_order":    0,
	}), http.StatusCreated)
	var it struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(ib, &it); err != nil {
		t.Fatal(err)
	}
	return it.ID
}

func login(t *testing.T, c *http.Client, email, password string) string {
	t.Helper()
	resp := doJSON(t, c, "POST", "/api/auth/login", "", map[string]string{
		"email": email, "password": password,
	})
	body := mustOK(t, resp, http.StatusOK)
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.AccessToken == "" {
		t.Fatal("empty access_token")
	}
	return out.AccessToken
}

func TestFullAPIIntegration(t *testing.T) {
	c := httpClient(t)
	base := apiBase(t)
	respPing, err := c.Get(base + "/health")
	if err != nil {
		t.Skipf("нет соединения с %s: %v", base, err)
	}
	_ = respPing.Body.Close()
	if respPing.StatusCode != http.StatusOK {
		t.Skipf("GET /health: %d — сервер не монолит или не запущен", respPing.StatusCode)
	}

	rsCat := doJSON(t, c, "GET", "/api/restaurants", "", nil)
	_, _ = io.ReadAll(rsCat.Body)
	_ = rsCat.Body.Close()
	if rsCat.StatusCode == http.StatusNotFound {
		t.Skip("нужен актуальный монолит из этого репозитория: перезапустите API (см. scripts/dev-restart.sh или: cd backend && go run ./cmd/api). На шлюзе без /api/restaurants тесты не применимы.")
	}
	if rsCat.StatusCode != http.StatusOK {
		t.Skipf("GET /api/restaurants: %d", rsCat.StatusCode)
	}

	t.Run("public_catalog", func(t *testing.T) {
		hallsBody := mustOK(t, doJSON(t, c, "GET", "/api/halls", "", nil), http.StatusOK)
		var halls []map[string]any
		if err := json.Unmarshal(hallsBody, &halls); err != nil {
			t.Fatal(err)
		}
		if len(halls) < 1 {
			t.Fatal("ожидался хотя бы один зал (сид)")
		}
		hid := halls[0]["id"].(string)
		mustOK(t, doJSON(t, c, "GET", "/api/halls/"+url.PathEscape(hid), "", nil), http.StatusOK)

		// Каталог ресторанов (есть в актуальном монолите; старые сборки могут отвечать 404)
		rs := doJSON(t, c, "GET", "/api/restaurants", "", nil)
		rb, _ := io.ReadAll(rs.Body)
		_ = rs.Body.Close()
		if rs.StatusCode == http.StatusOK {
			var list []map[string]any
			if err := json.Unmarshal(rb, &list); err != nil {
				t.Fatal(err)
			}
			if len(list) > 0 {
				rid := list[0]["id"].(string)
				mustOK(t, doJSON(t, c, "GET", "/api/restaurants/"+url.PathEscape(rid)+"/menu", "", nil), http.StatusOK)
			}
		} else {
			t.Logf("пропуск /api/restaurants*: HTTP %d (обновите монолит до текущей ветки)", rs.StatusCode)
		}
	})

	t.Run("auth_me_unauthorized", func(t *testing.T) {
		resp := doJSON(t, c, "GET", "/api/auth/me", "", nil)
		mustOK(t, resp, http.StatusUnauthorized)
	})

	var restaurantID, hallID, tableA, tableB, menuItemID string
	t.Run("discover_hall_tables_menu", func(t *testing.T) {
		hallsBody := mustOK(t, doJSON(t, c, "GET", "/api/halls", "", nil), http.StatusOK)
		var halls []map[string]any
		if err := json.Unmarshal(hallsBody, &halls); err != nil {
			t.Fatal(err)
		}
		if len(halls) < 1 {
			t.Fatal("нет залов")
		}
		for _, h0 := range halls {
			cand, _ := h0["id"].(string)
			if cand == "" {
				continue
			}
			lay := mustOK(t, doJSON(t, c, "GET", "/api/halls/"+url.PathEscape(cand)+"/layout", "", nil), http.StatusOK)
			var layout struct {
				Tables []struct {
					ID       string `json:"id"`
					Capacity int    `json:"capacity"`
				} `json:"tables"`
			}
			if err := json.Unmarshal(lay, &layout); err != nil {
				continue
			}
			if layout.Tables == nil || len(layout.Tables) < 2 {
				continue
			}
			hallID = cand
			tableA, tableB = layout.Tables[0].ID, layout.Tables[1].ID
			break
		}
		if hallID == "" || tableA == "" {
			t.Fatal("нет зала с минимум двумя столами в layout (проверьте сид / редактор схемы)")
		}
		for _, h0 := range halls {
			if id, _ := h0["id"].(string); id == hallID {
				restaurantID, _ = h0["restaurant_id"].(string)
				break
			}
		}
		if restaurantID == "" {
			detail := mustOK(t, doJSON(t, c, "GET", "/api/halls/"+url.PathEscape(hallID), "", nil), http.StatusOK)
			var hallOne map[string]any
			if err := json.Unmarshal(detail, &hallOne); err != nil {
				t.Fatal(err)
			}
			restaurantID, _ = hallOne["restaurant_id"].(string)
		}
		if restaurantID == "" {
			t.Fatal("не удалось определить restaurant_id (GET /api/halls/{id})")
		}

		// Публичное меню или админ-список (официант добавляет строку заказа)
		pub := doJSON(t, c, "GET", "/api/restaurants/"+url.PathEscape(restaurantID)+"/menu", "", nil)
		mb, _ := io.ReadAll(pub.Body)
		_ = pub.Body.Close()
		if pub.StatusCode == http.StatusOK {
			var menu struct {
				Items []struct {
					ID string `json:"id"`
				} `json:"items"`
			}
			if err := json.Unmarshal(mb, &menu); err != nil {
				t.Fatal(err)
			}
			if len(menu.Items) >= 1 {
				menuItemID = menu.Items[0].ID
				return
			}
			t.Log("публичное меню без позиций, пробуем /api/admin/menu/items")
		}
		adm := login(t, c, "admin@demo.ru", "Password1")
		admResp := doJSON(t, c, "GET", "/api/admin/menu/items", adm, nil)
		itemsBody, _ := io.ReadAll(admResp.Body)
		_ = admResp.Body.Close()
		if admResp.StatusCode == http.StatusOK {
			var items []map[string]any
			if err := json.Unmarshal(itemsBody, &items); err != nil || items == nil {
				items = []map[string]any{}
			}
			if len(items) >= 1 {
				menuItemID, _ = items[0]["id"].(string)
				return
			}
		} else {
			t.Logf("GET /api/admin/menu/items → %d, создаём тестовую позицию", admResp.StatusCode)
		}
		menuItemID = bootstrapTestMenuItem(t, c, adm)
	})

	// Уникальное время в будущем (дни + случайные секунды), чтобы не пересекаться с бронями в БД
	offSec := time.Now().UnixNano() % 200000
	slotOverlap := time.Now().UTC().Add(35*24*time.Hour + time.Duration(offSec)*time.Second).Truncate(time.Minute)
	slotHappy := slotOverlap.Add(96 * time.Hour)

	t.Run("availability", func(t *testing.T) {
		start := slotOverlap.Format(time.RFC3339)
		end := slotOverlap.Add(2 * time.Hour).Format(time.RFC3339)
		q := fmt.Sprintf("/api/halls/%s/availability?start=%s&end=%s&guests=2",
			url.PathEscape(hallID), url.QueryEscape(start), url.QueryEscape(end))
		mustOK(t, doJSON(t, c, "GET", q, "", nil), http.StatusOK)
	})

	t.Run("availability_past_returns_400", func(t *testing.T) {
		start := time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339)
		end := time.Now().UTC().Add(-70 * time.Hour).Format(time.RFC3339)
		q := fmt.Sprintf("/api/halls/%s/availability?start=%s&end=%s&guests=2",
			url.PathEscape(hallID), url.QueryEscape(start), url.QueryEscape(end))
		rs := doJSON(t, c, "GET", q, "", nil)
		b, _ := io.ReadAll(rs.Body)
		_ = rs.Body.Close()
		if rs.StatusCode != http.StatusBadRequest {
			t.Fatalf("ожидали 400 для прошлого слота, получили %d: %s", rs.StatusCode, string(b))
		}
	})

	t.Run("overlap_second_booking_same_user", func(t *testing.T) {
		tok := login(t, c, "client3@demo.ru", "Password1")
		body := map[string]any{
			"table_id":         tableA,
			"start_time":       slotOverlap.Format(time.RFC3339),
			"guest_count":      2,
			"comment":          "api test overlap A",
			"idempotency_key":  uuid.New().String(),
		}
		r1 := doJSON(t, c, "POST", "/api/reservations", tok, body)
		mustOK(t, r1, http.StatusCreated)

		body["table_id"] = tableB
		body["idempotency_key"] = uuid.New().String()
		body["comment"] = "api test overlap B"
		r2 := doJSON(t, c, "POST", "/api/reservations", tok, body)
		b2, _ := io.ReadAll(r2.Body)
		_ = r2.Body.Close()
		if r2.StatusCode != http.StatusConflict {
			t.Fatalf("ожидали 409 пересечения по времени, получили %d: %s", r2.StatusCode, string(b2))
		}
		var errObj map[string]string
		_ = json.Unmarshal(b2, &errObj)
		if errObj["error"] == "" {
			t.Fatal("ожидалось поле error в JSON")
		}

		// отмена первой брони (pending_payment)
		my := mustOK(t, doJSON(t, c, "GET", "/api/reservations/my", tok, nil), http.StatusOK)
		var myList []map[string]any
		_ = json.Unmarshal(my, &myList)
		var rid string
		for _, row := range myList {
			if com, ok := row["comment"].(string); ok && strings.Contains(com, "overlap A") {
				rid = row["id"].(string)
				break
			}
		}
		if rid == "" {
			t.Fatal("не найдена бронь overlap A для отмены")
		}
		del := doJSON(t, c, "DELETE", "/api/reservations/"+url.PathEscape(rid), tok, nil)
		mustOK(t, del, http.StatusOK)
	})

	t.Run("auth_me_roles_refresh", func(t *testing.T) {
		jarClient := httpClient(t)
		_ = login(t, jarClient, "owner@demo.ru", "Password1")
		u, _ := url.Parse(apiBase(t) + "/api/auth/refresh")
		req, _ := http.NewRequest("POST", u.String(), nil)
		refResp, err := jarClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		refBody := mustOK(t, refResp, http.StatusOK)
		var refOut struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(refBody, &refOut); err != nil {
			t.Fatal(err)
		}
		if refOut.AccessToken == "" {
			t.Fatal("refresh: пустой access_token")
		}
		me := mustOK(t, doJSON(t, jarClient, "GET", "/api/auth/me", refOut.AccessToken, nil), http.StatusOK)
		var meMap map[string]any
		_ = json.Unmarshal(me, &meMap)
		if meMap["role"] != "owner" {
			t.Fatalf("role: %v", meMap["role"])
		}
	})

	t.Run("admin_owner_waiter_endpoints", func(t *testing.T) {
		adm := login(t, c, "admin@demo.ru", "Password1")
		lu := mustOK(t, doJSON(t, c, "GET", "/api/admin/users/lookup?phone="+url.QueryEscape("+79161230001"), adm, nil), http.StatusOK)
		var lookupOut struct {
			ID       string `json:"id"`
			FullName string `json:"full_name"`
		}
		if err := json.Unmarshal(lu, &lookupOut); err != nil || lookupOut.ID == "" {
			t.Fatalf("lookup client: %v, body=%s", err, string(lu))
		}

		mustOK(t, doJSON(t, c, "GET", "/api/reservations", adm, nil), http.StatusOK)
		mustOK(t, doJSON(t, c, "GET", "/api/admin/clients", adm, nil), http.StatusOK)
		for _, p := range []string{"/api/admin/menu/categories", "/api/admin/menu/items"} {
			rs := doJSON(t, c, "GET", p, adm, nil)
			b, _ := io.ReadAll(rs.Body)
			_ = rs.Body.Close()
			if rs.StatusCode != http.StatusOK {
				t.Logf("меню админки недоступно (%s HTTP %d) — обновите монолит", p, rs.StatusCode)
			} else if len(b) < 2 {
				t.Logf("%s: пустой ответ", p)
			}
		}

		ownerTok := login(t, c, "owner@demo.ru", "Password1")
		mustOK(t, doJSON(t, c, "GET", "/api/owner/analytics", ownerTok, nil), http.StatusOK)
		mustOK(t, doJSON(t, c, "GET", "/api/owner/finance", ownerTok, nil), http.StatusOK)
		exp := doJSON(t, c, "GET", "/api/owner/finance/export", ownerTok, nil)
		expBody := mustOK(t, exp, http.StatusOK)
		if len(expBody) < 64 {
			t.Fatal("ожидался XLSX (байты)")
		}
		mustOK(t, doJSON(t, c, "GET", "/api/owner/staff-stats", ownerTok, nil), http.StatusOK)
		mustOK(t, doJSON(t, c, "GET", "/api/owner/users", ownerTok, nil), http.StatusOK)
		mustOK(t, doJSON(t, c, "GET", "/api/settings", ownerTok, nil), http.StatusOK)

		wt := login(t, c, "waiter@demo.ru", "Password1")
		mustOK(t, doJSON(t, c, "GET", "/api/waiter/my-tables", wt, nil), http.StatusOK)
		mustOK(t, doJSON(t, c, "GET", "/api/waiter/shifts", wt, nil), http.StatusOK)
	})

	var happyResID, happyPayID string
	t.Run("happy_path_reservation_order_tab", func(t *testing.T) {
		if menuItemID == "" {
			t.Skip("нет id блюда: нужны GET /api/restaurants/{id}/menu или GET /api/admin/menu/items в актуальной сборке API")
		}
		tok := login(t, c, "client2@demo.ru", "Password1")
		start := slotHappy.Format(time.RFC3339)
		end := slotHappy.Add(2 * time.Hour).Format(time.RFC3339)
		q := fmt.Sprintf("/api/halls/%s/availability?start=%s&end=%s&guests=2",
			url.PathEscape(hallID), url.QueryEscape(start), url.QueryEscape(end))
		mustOK(t, doJSON(t, c, "GET", q, "", nil), http.StatusOK)

		cr := doJSON(t, c, "POST", "/api/reservations", tok, map[string]any{
			"table_id":         tableA,
			"start_time":       start,
			"guest_count":      2,
			"comment":          "api integration happy",
			"idempotency_key":  uuid.New().String(),
		})
		cb := mustOK(t, cr, http.StatusCreated)
		var created map[string]any
		if err := json.Unmarshal(cb, &created); err != nil {
			t.Fatal(err)
		}
		happyResID = created["reservation_id"].(string)
		payAny, ok := created["payment_id"].(string)
		if !ok || payAny == "" {
			t.Fatal("ожидался payment_id (или включите deposit_percent > 0 в настройках)")
		}
		happyPayID = payAny

		sim := doJSON(t, c, "POST", "/api/payments/checkout/"+url.PathEscape(happyPayID)+"/simulate", tok, nil)
		mustOK(t, sim, http.StatusOK)

		mustOK(t, doJSON(t, c, "GET", "/api/payments/"+url.PathEscape(happyPayID), tok, nil), http.StatusOK)

		waiterTok := login(t, c, "waiter@demo.ru", "Password1")
		var wid string
		wme := mustOK(t, doJSON(t, c, "GET", "/api/auth/me", waiterTok, nil), http.StatusOK)
		var wm map[string]any
		_ = json.Unmarshal(wme, &wm)
		wid = wm["id"].(string)

		chk := doJSON(t, c, "POST", "/api/reservations/"+url.PathEscape(happyResID)+"/checkin", admToken(t, c), map[string]any{
			"waiter_id": wid,
		})
		mustOK(t, chk, http.StatusNoContent)

		ss := doJSON(t, c, "POST", "/api/reservations/"+url.PathEscape(happyResID)+"/start-service", waiterTok, nil)
		mustOK(t, ss, http.StatusNoContent)

		line := doJSON(t, c, "POST", "/api/reservations/"+url.PathEscape(happyResID)+"/order/lines", waiterTok, map[string]any{
			"menu_item_id": menuItemID,
			"quantity":     1,
			"guest_label":  "Тест",
		})
		mustOK(t, line, http.StatusCreated)

		tab := doJSON(t, c, "POST", "/api/reservations/"+url.PathEscape(happyResID)+"/order/checkout", tok, nil)
		tabRaw, _ := io.ReadAll(tab.Body)
		_ = tab.Body.Close()
		if tab.StatusCode != http.StatusCreated && tab.StatusCode != http.StatusOK {
			t.Fatalf("checkout счёта: %d %s", tab.StatusCode, string(tabRaw))
		}
		var tabPay map[string]any
		_ = json.Unmarshal(tabRaw, &tabPay)
		if closed, _ := tabPay["closed_without_payment"].(bool); closed {
			// сумма заказа полностью покрыта учтённым депозитом
		} else {
			tabPID, _ := tabPay["payment_id"].(string)
			if tabPID == "" {
				t.Fatal("нет payment_id для счёта")
			}
			tabSim := doJSON(t, c, "POST", "/api/payments/checkout/"+url.PathEscape(tabPID)+"/simulate", tok, nil)
			mustOK(t, tabSim, http.StatusOK)
		}

		cmp := doJSON(t, c, "POST", "/api/reservations/"+url.PathEscape(happyResID)+"/complete", waiterTok, nil)
		mustOK(t, cmp, http.StatusNoContent)

		note := doJSON(t, c, "POST", "/api/waiter/notes", waiterTok, map[string]any{
			"reservation_id": happyResID,
			"note":           "интеграционный тест",
		})
		// после complete бронь уже completed — заметка может быть запрещена; допускаем 403
		_ = note.Body.Close()
		if note.StatusCode != http.StatusCreated && note.StatusCode != http.StatusForbidden {
			t.Fatalf("waiter note: %d", note.StatusCode)
		}
	})

	t.Run("admin_refund_deposit", func(t *testing.T) {
		if happyPayID == "" {
			t.Skip("нет happyPayID")
		}
		adm := login(t, c, "admin@demo.ru", "Password1")
		ref := doJSON(t, c, "POST", "/api/payments/"+url.PathEscape(happyPayID)+"/refund", adm, nil)
		b, _ := io.ReadAll(ref.Body)
		_ = ref.Body.Close()
		if ref.StatusCode != http.StatusNoContent && ref.StatusCode != http.StatusConflict {
			t.Fatalf("refund: %d %s", ref.StatusCode, string(b))
		}
	})
}

func admToken(t *testing.T, c *http.Client) string {
	t.Helper()
	return login(t, c, "admin@demo.ru", "Password1")
}
