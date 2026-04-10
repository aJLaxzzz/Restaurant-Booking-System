package handlers

import (
	"context"
	"log"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ensureExtraDemoRestaurants добавляет недостающие демо-рестораны (trattoria-tverskaya, la-luna, sakura-lite),
// если БД уже была создана старым сидом и полный seedBase не выполнялся.
func (a *Handlers) ensureExtraDemoRestaurants(ctx context.Context) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Password1"), a.Cfg.BcryptCost)
	if err != nil {
		log.Printf("сид: bcrypt demo owners: %v", err)
		return
	}
	hashStr := string(hash)

	o1 := a.ensureDemoOwnerID(ctx, hashStr, "owner@demo.ru", "Михаил Волков", "+79001234567")
	o2 := a.ensureDemoOwnerID(ctx, hashStr, "owner2@demo.ru", "Анна Север", "+79001234568")
	o3 := a.ensureDemoOwnerID(ctx, hashStr, "owner3@demo.ru", "Денис Океан", "+79001234569")

	a.topUpTrattoriaIfMissing(ctx, o1)
	a.topUpLaLunaIfMissing(ctx, o2)
	a.topUpSakuraIfMissing(ctx, o3)
	// Повтор без привязки к владельцу: если INSERT падал (напр. из‑за uuid.Nil в owner), добиваем карточки на главной.
	a.topUpTrattoriaIfMissing(ctx, uuid.Nil)
	a.topUpLaLunaIfMissing(ctx, uuid.Nil)
	a.topUpSakuraIfMissing(ctx, uuid.Nil)
	a.ensureDemoWaitersForRestaurants(ctx, hashStr)
	a.logDemoRestaurantCoverage(ctx)
}

// ensureDemoOwnerID гарантирует ненулевой id владельца (upsert + при ошибке — SELECT).
func (a *Handlers) ensureDemoOwnerID(ctx context.Context, hashStr, email, fullName, phone string) uuid.UUID {
	var id uuid.UUID
	err := a.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, phone, role, email_verified)
		VALUES ($1,$2,$3,$4,'owner',true)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`, email, hashStr, fullName, phone).Scan(&id)
	if err != nil || id == uuid.Nil {
		if err != nil {
			log.Printf("сид: upsert owner %s: %v", email, err)
		}
		if err2 := a.Pool.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, email).Scan(&id); err2 != nil {
			log.Printf("сид: SELECT owner %s: %v", email, err2)
			return uuid.Nil
		}
	}
	return id
}

// ownerIfNoRestaurant — владелец для INSERT: у индекса idx_restaurants_owner_one только один ресторан на owner.
// Если у владельца уже есть ресторан, возвращаем nil (демо-дозаполнение без привязки к аккаунту).
// uuid.Nil означает «вставить без владельца», не подставлять нулевой UUID в FK.
func (a *Handlers) ownerIfNoRestaurant(ctx context.Context, ownerID uuid.UUID) *uuid.UUID {
	if ownerID == uuid.Nil {
		return nil
	}
	var n int
	if err := a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM restaurants WHERE owner_user_id = $1`, ownerID).Scan(&n); err != nil {
		return nil
	}
	if n > 0 {
		return nil
	}
	return &ownerID
}

func (a *Handlers) logDemoRestaurantCoverage(ctx context.Context) {
	var n int
	_ = a.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM restaurants
		WHERE slug IN ('trattoria-tverskaya','la-luna','sakura-lite')`).Scan(&n)
	if n < 3 {
		log.Printf("сид: в каталоге демо-ресторанов %d из 3 (slug trattoria / la-luna / sakura-lite)", n)
	}
}

func (a *Handlers) topUpTrattoriaIfMissing(ctx context.Context, ownerID uuid.UUID) {
	var n int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM restaurants WHERE slug=$1`, "trattoria-tverskaya").Scan(&n)
	if n > 0 {
		var rid uuid.UUID
		if err := a.Pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE slug=$1`, "trattoria-tverskaya").Scan(&rid); err == nil {
			a.patchTrattoriaHallTablesMenu(ctx, rid)
		}
		return
	}
	rid := uuid.New()
	hid := uuid.New()
	photo := "https://images.unsplash.com/photo-1517248135467-4c7edcad34c4?w=1200&q=80"
	own := a.ownerIfNoRestaurant(ctx, ownerID)
	if _, err := a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url)
		VALUES ($1,'Траттория Тверская','Москва, ул. Тверская, 12','trattoria-tverskaya','Москва','Итальянская кухня и вино',$2,$3)`,
		rid, own, photo); err != nil {
		log.Printf("сид: trattoria-tverskaya: %v", err)
		return
	}
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Основной зал')`, hid, rid)
	wallsMain := `{"walls":[{"x1":0,"y1":0,"x2":920,"y2":0},{"x1":920,"y1":0,"x2":920,"y2":640},{"x1":920,"y1":640,"x2":0,"y2":640},{"x1":0,"y1":640,"x2":0,"y2":0}],"decorations":[{"type":"zone_label","text":"Панорамные окна","x":60,"y":40,"w":200,"h":32},{"type":"window_band","x":0,"y":0,"w":920,"h":24}]}`
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hid, wallsMain)
	mainTables := []struct {
		num        int
		cap        int
		x, y       float64
		shape      string
		w, h       float64
		rot        float64
	}{
		{1, 2, 100, 100, "round", 52, 52, 0},
		{2, 4, 260, 100, "rect", 88, 64, 0},
		{3, 4, 420, 100, "rect", 88, 64, 0},
		{4, 6, 580, 100, "ellipse", 112, 72, 0},
		{5, 4, 100, 260, "rect", 88, 64, 12},
		{6, 2, 260, 260, "round", 48, 48, 0},
	}
	for _, t := range mainTables {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, width, height, rotation_deg)
			VALUES ($1,$2,$3,$4,$5,$6,'available',$7,$8,$9)`, hid, t.num, t.cap, t.x, t.y, t.shape, t.w, t.h, t.rot)
	}
	a.seedMenuTrattoria(ctx, rid)
	log.Println("сид: дозаполнен ресторан trattoria-tverskaya")
}

// patchTrattoriaHallTablesMenu — если строка ресторана уже есть, но нет зала/столов/меню (частичный INSERT).
func (a *Handlers) patchTrattoriaHallTablesMenu(ctx context.Context, rid uuid.UUID) {
	var hc, mc, tc int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM halls WHERE restaurant_id=$1`, rid).Scan(&hc)
	_ = a.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM tables t
		JOIN halls h ON h.id = t.hall_id WHERE h.restaurant_id=$1`, rid).Scan(&tc)
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM menu_categories WHERE restaurant_id=$1`, rid).Scan(&mc)
	if hc > 0 && tc > 0 && mc > 0 {
		return
	}
	var hid uuid.UUID
	if hc == 0 {
		hid = uuid.New()
		_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Основной зал')`, hid, rid)
		wallsMain := `{"walls":[{"x1":0,"y1":0,"x2":920,"y2":0},{"x1":920,"y1":0,"x2":920,"y2":640},{"x1":920,"y1":640,"x2":0,"y2":640},{"x1":0,"y1":640,"x2":0,"y2":0}],"decorations":[{"type":"zone_label","text":"Панорамные окна","x":60,"y":40,"w":200,"h":32},{"type":"window_band","x":0,"y":0,"w":920,"h":24}]}`
		_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hid, wallsMain)
	} else if tc == 0 {
		_ = a.Pool.QueryRow(ctx, `SELECT id FROM halls WHERE restaurant_id=$1 ORDER BY created_at LIMIT 1`, rid).Scan(&hid)
	}
	if hid != uuid.Nil && tc == 0 {
		mainTables := []struct {
			num        int
			cap        int
			x, y       float64
			shape      string
			w, h       float64
			rot        float64
		}{
			{1, 2, 100, 100, "round", 52, 52, 0},
			{2, 4, 260, 100, "rect", 88, 64, 0},
			{3, 4, 420, 100, "rect", 88, 64, 0},
			{4, 6, 580, 100, "ellipse", 112, 72, 0},
			{5, 4, 100, 260, "rect", 88, 64, 12},
			{6, 2, 260, 260, "round", 48, 48, 0},
		}
		for _, t := range mainTables {
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, width, height, rotation_deg)
				VALUES ($1,$2,$3,$4,$5,$6,'available',$7,$8,$9)`, hid, t.num, t.cap, t.x, t.y, t.shape, t.w, t.h, t.rot)
		}
		log.Printf("сид: trattoria %s — добавлены столы", rid)
	}
	if mc == 0 {
		a.seedMenuTrattoria(ctx, rid)
		log.Printf("сид: trattoria %s — дозаполнено меню", rid)
	}
}

func (a *Handlers) topUpLaLunaIfMissing(ctx context.Context, ownerID uuid.UUID) {
	var n int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM restaurants WHERE slug=$1`, "la-luna").Scan(&n)
	if n > 0 {
		var rid uuid.UUID
		if err := a.Pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE slug=$1`, "la-luna").Scan(&rid); err == nil {
			a.patchLaLunaHallTablesMenu(ctx, rid)
		}
		return
	}
	rid := uuid.New()
	hid := uuid.New()
	photo := "https://images.unsplash.com/photo-1414235077428-338989a2e8c0?w=1200&q=80"
	own := a.ownerIfNoRestaurant(ctx, ownerID)
	if _, err := a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url)
		VALUES ($1,'La Luna','Санкт-Петербург, наб. реки Фонтанки, 20','la-luna','Санкт-Петербург','Европейская кухня',$2,$3)`,
		rid, own, photo); err != nil {
		log.Printf("сид: la-luna: %v", err)
		return
	}
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Главный зал')`, hid, rid)
	wallsLuna := `{"walls":[{"x1":0,"y1":0,"x2":800,"y2":0},{"x1":800,"y1":0,"x2":800,"y2":560},{"x1":800,"y1":560,"x2":0,"y2":560},{"x1":0,"y1":560,"x2":0,"y2":0}],"decorations":[{"type":"zone_label","text":"Центр зала","x":360,"y":260,"w":120,"h":28}]}`
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hid, wallsLuna)
	lunaTables := []struct {
		n   int
		cap int
		x, y float64
	}{
		{1, 2, 140, 120}, {2, 4, 300, 140}, {3, 4, 480, 160},
		{4, 4, 200, 300}, {5, 8, 420, 320},
	}
	for _, t := range lunaTables {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, width, height)
			VALUES ($1,$2,$3,$4,$5,'rect','available',80,72)`, hid, t.n, t.cap, t.x, t.y)
	}
	a.seedMenuLaLuna(ctx, rid)
	log.Println("сид: дозаполнен ресторан la-luna")
}

func (a *Handlers) patchLaLunaHallTablesMenu(ctx context.Context, rid uuid.UUID) {
	var hc, mc, tc int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM halls WHERE restaurant_id=$1`, rid).Scan(&hc)
	_ = a.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM tables t
		JOIN halls h ON h.id = t.hall_id WHERE h.restaurant_id=$1`, rid).Scan(&tc)
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM menu_categories WHERE restaurant_id=$1`, rid).Scan(&mc)
	if hc > 0 && tc > 0 && mc > 0 {
		return
	}
	var hid uuid.UUID
	if hc == 0 {
		hid = uuid.New()
		_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Главный зал')`, hid, rid)
		wallsLuna := `{"walls":[{"x1":0,"y1":0,"x2":800,"y2":0},{"x1":800,"y1":0,"x2":800,"y2":560},{"x1":800,"y1":560,"x2":0,"y2":560},{"x1":0,"y1":560,"x2":0,"y2":0}],"decorations":[{"type":"zone_label","text":"Центр зала","x":360,"y":260,"w":120,"h":28}]}`
		_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hid, wallsLuna)
	} else if tc == 0 {
		_ = a.Pool.QueryRow(ctx, `SELECT id FROM halls WHERE restaurant_id=$1 ORDER BY created_at LIMIT 1`, rid).Scan(&hid)
	}
	if hid != uuid.Nil && tc == 0 {
		lunaTables := []struct {
			n   int
			cap int
			x, y float64
		}{
			{1, 2, 140, 120}, {2, 4, 300, 140}, {3, 4, 480, 160},
			{4, 4, 200, 300}, {5, 8, 420, 320},
		}
		for _, t := range lunaTables {
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, width, height)
				VALUES ($1,$2,$3,$4,$5,'rect','available',80,72)`, hid, t.n, t.cap, t.x, t.y)
		}
		log.Printf("сид: la-luna %s — добавлены столы", rid)
	}
	if mc == 0 {
		a.seedMenuLaLuna(ctx, rid)
		log.Printf("сид: la-luna %s — дозаполнено меню", rid)
	}
}

func (a *Handlers) topUpSakuraIfMissing(ctx context.Context, ownerID uuid.UUID) {
	var n int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM restaurants WHERE slug=$1`, "sakura-lite").Scan(&n)
	if n > 0 {
		var rid uuid.UUID
		if err := a.Pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE slug=$1`, "sakura-lite").Scan(&rid); err == nil {
			a.patchSakuraHallTablesMenu(ctx, rid)
		}
		return
	}
	rid := uuid.New()
	hid := uuid.New()
	photo := "https://images.unsplash.com/photo-1579584425555-c7ce17fd4351?w=1200&q=80"
	own := a.ownerIfNoRestaurant(ctx, ownerID)
	if _, err := a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url)
		VALUES ($1,'Сакура Лайт','Москва, ул. Покровка, 3','sakura-lite','Москва','Японская кухня',$2,$3)`,
		rid, own, photo); err != nil {
		log.Printf("сид: sakura-lite: %v", err)
		return
	}
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Зал татами')`, hid, rid)
	wallsSakura := `{"walls":[{"x1":0,"y1":0,"x2":680,"y2":0},{"x1":680,"y1":0,"x2":680,"y2":520},{"x1":680,"y1":520,"x2":0,"y2":520},{"x1":0,"y1":520,"x2":0,"y2":0}],"decorations":[]}`
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hid, wallsSakura)
	sakuraTables := []struct {
		n   int
		cap int
		x, y float64
	}{
		{1, 2, 120, 100}, {2, 4, 280, 120}, {3, 4, 440, 140}, {4, 6, 200, 300},
	}
	for _, t := range sakuraTables {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, width, height)
			VALUES ($1,$2,$3,$4,$5,'rect','available',88,72)`, hid, t.n, t.cap, t.x, t.y)
	}
	a.seedMenuSakura(ctx, rid)
	log.Println("сид: дозаполнен ресторан sakura-lite")
}

func (a *Handlers) patchSakuraHallTablesMenu(ctx context.Context, rid uuid.UUID) {
	var hc, mc, tc int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM halls WHERE restaurant_id=$1`, rid).Scan(&hc)
	_ = a.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM tables t
		JOIN halls h ON h.id = t.hall_id WHERE h.restaurant_id=$1`, rid).Scan(&tc)
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM menu_categories WHERE restaurant_id=$1`, rid).Scan(&mc)
	if hc > 0 && tc > 0 && mc > 0 {
		return
	}
	var hid uuid.UUID
	if hc == 0 {
		hid = uuid.New()
		_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Зал татами')`, hid, rid)
		wallsSakura := `{"walls":[{"x1":0,"y1":0,"x2":680,"y2":0},{"x1":680,"y1":0,"x2":680,"y2":520},{"x1":680,"y1":520,"x2":0,"y2":520},{"x1":0,"y1":520,"x2":0,"y2":0}],"decorations":[]}`
		_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hid, wallsSakura)
	} else if tc == 0 {
		_ = a.Pool.QueryRow(ctx, `SELECT id FROM halls WHERE restaurant_id=$1 ORDER BY created_at LIMIT 1`, rid).Scan(&hid)
	}
	if hid != uuid.Nil && tc == 0 {
		sakuraTables := []struct {
			n   int
			cap int
			x, y float64
		}{
			{1, 2, 120, 100}, {2, 4, 280, 120}, {3, 4, 440, 140}, {4, 6, 200, 300},
		}
		for _, t := range sakuraTables {
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, width, height)
				VALUES ($1,$2,$3,$4,$5,'rect','available',88,72)`, hid, t.n, t.cap, t.x, t.y)
		}
		log.Printf("сид: sakura-lite %s — добавлены столы", rid)
	}
	if mc == 0 {
		a.seedMenuSakura(ctx, rid)
		log.Printf("сид: sakura-lite %s — дозаполнено меню", rid)
	}
}

// ensureDemoWaitersForRestaurants создаёт официантов la-luna / sakura-lite, если их ещё нет (старые БД).
func (a *Handlers) ensureDemoWaitersForRestaurants(ctx context.Context, hashStr string) {
	type row struct {
		slug string
		email, name string
		phone string
	}
	rows := []row{
		{"la-luna", "waiter3@demo.ru", "Павел Невский", "+79001234571"},
		{"sakura-lite", "waiter4@demo.ru", "Юлия Сакура", "+79001234572"},
	}
	for _, r := range rows {
		var rid uuid.UUID
		err := a.Pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE slug=$1`, r.slug).Scan(&rid)
		if err != nil {
			continue
		}
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO users (email, password_hash, full_name, phone, role, email_verified, restaurant_id)
			VALUES ($1,$2,$3,$4,'waiter',true,$5)
			ON CONFLICT (email) DO UPDATE SET restaurant_id = EXCLUDED.restaurant_id, full_name = EXCLUDED.full_name`,
			r.email, hashStr, r.name, r.phone, rid)
	}
}
