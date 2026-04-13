package handlers

import (
	"context"
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ensureExtraDemoRestaurants добавляет недостающие демо-рестораны (trattoria-tverskaya, la-luna, sakura-lite, bella-vista),
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
	o4 := a.ensureDemoOwnerID(ctx, hashStr, "owner-bella@demo.ru", "Рикардо Беллини", "+79001234570")

	a.dedupeBellaVistaRestaurants(ctx)
	a.topUpTrattoriaIfMissing(ctx, o1)
	a.topUpLaLunaIfMissing(ctx, o2)
	a.topUpSakuraIfMissing(ctx, o3)
	a.topUpBellaVistaIfMissing(ctx, o4)
	// Повтор без привязки к владельцу: если INSERT падал (напр. из‑за uuid.Nil в owner), добиваем карточки на главной.
	a.topUpTrattoriaIfMissing(ctx, uuid.Nil)
	a.topUpLaLunaIfMissing(ctx, uuid.Nil)
	a.topUpSakuraIfMissing(ctx, uuid.Nil)
	a.topUpBellaVistaIfMissing(ctx, uuid.Nil)
	a.ensureDemoWaitersForRestaurants(ctx, hashStr)
	a.ensureDemoBellaAdmin(ctx, hashStr)
	a.syncDemoMoscowRestaurants(ctx)
	a.syncDemoRestaurantContacts(ctx)
	a.backfillDemoMenuImages(ctx)
	a.logDemoRestaurantCoverage(ctx)
}

// dedupeBellaVistaRestaurants оставляет одну строку со slug bella-vista и удаляет лишние «Bella Vista» без канона.
func (a *Handlers) dedupeBellaVistaRestaurants(ctx context.Context) {
	rows, err := a.Pool.Query(ctx, `SELECT id FROM restaurants WHERE slug = 'bella-vista' ORDER BY created_at ASC NULLS FIRST`)
	if err != nil {
		return
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for i := 1; i < len(ids); i++ {
		_, _ = a.Pool.Exec(ctx, `
			DELETE FROM reservations res
			WHERE res.table_id IN (
				SELECT t.id FROM tables t
				INNER JOIN halls h ON h.id = t.hall_id
				WHERE h.restaurant_id = $1)`, ids[i])
		_, _ = a.Pool.Exec(ctx, `
			DELETE FROM order_lines ol
			USING menu_items mi
			WHERE ol.menu_item_id = mi.id AND mi.restaurant_id = $1`, ids[i])
		if _, err := a.Pool.Exec(ctx, `DELETE FROM restaurants WHERE id=$1`, ids[i]); err != nil {
			log.Printf("сид: дедуп bella-vista %s: %v", ids[i], err)
		} else {
			log.Printf("сид: удалён дубликат bella-vista %s", ids[i])
		}
	}
	_, _ = a.Pool.Exec(ctx, `
		DELETE FROM reservations res
		WHERE res.table_id IN (
		  SELECT t.id FROM tables t
		  INNER JOIN halls h ON h.id = t.hall_id
		  WHERE h.restaurant_id IN (
		    SELECT r.id FROM restaurants r
		    WHERE LOWER(TRIM(r.name)) = 'bella vista'
		      AND r.slug IS DISTINCT FROM 'bella-vista'
		      AND EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista')))`)
	_, _ = a.Pool.Exec(ctx, `
		DELETE FROM order_lines ol
		USING menu_items mi
		WHERE ol.menu_item_id = mi.id
		  AND mi.restaurant_id IN (
		    SELECT r.id FROM restaurants r
		    WHERE LOWER(TRIM(r.name)) = 'bella vista'
		      AND r.slug IS DISTINCT FROM 'bella-vista'
		      AND EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista'))`)
	ct, err := a.Pool.Exec(ctx, `
		DELETE FROM restaurants r
		WHERE LOWER(TRIM(r.name)) = 'bella vista'
		  AND r.slug IS DISTINCT FROM 'bella-vista'
		  AND EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista')`)
	if err == nil && ct.RowsAffected() > 0 {
		log.Printf("сид: удалены дубликаты Bella Vista по имени (%d строк)", ct.RowsAffected())
	}
	_, _ = a.Pool.Exec(ctx, `
		DELETE FROM reservations res
		WHERE res.table_id IN (
		  SELECT t.id FROM tables t
		  INNER JOIN halls h ON h.id = t.hall_id
		  WHERE h.restaurant_id IN (
		    SELECT r.id FROM restaurants r
		    WHERE r.slug IS DISTINCT FROM 'bella-vista'
		      AND LOWER(TRIM(r.slug)) LIKE 'bella-vista%'
		      AND EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista')))`)
	_, _ = a.Pool.Exec(ctx, `
		DELETE FROM order_lines ol
		USING menu_items mi
		WHERE ol.menu_item_id = mi.id
		  AND mi.restaurant_id IN (
		    SELECT r.id FROM restaurants r
		    WHERE r.slug IS DISTINCT FROM 'bella-vista'
		      AND LOWER(TRIM(r.slug)) LIKE 'bella-vista%'
		      AND EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista'))`)
	ct2, err2 := a.Pool.Exec(ctx, `
		DELETE FROM restaurants r
		WHERE r.slug IS DISTINCT FROM 'bella-vista'
		  AND LOWER(TRIM(r.slug)) LIKE 'bella-vista%'
		  AND EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista')`)
	if err2 == nil && ct2.RowsAffected() > 0 {
		log.Printf("сид: удалены строки Bella Vista с нестандартным slug (%d строк)", ct2.RowsAffected())
	}
	_, _ = a.Pool.Exec(ctx, `
		DELETE FROM reservations res
		WHERE res.table_id IN (
		  SELECT t.id FROM tables t
		  INNER JOIN halls h ON h.id = t.hall_id
		  WHERE h.restaurant_id IN (
		    SELECT r.id FROM restaurants r
		    WHERE r.slug IS DISTINCT FROM 'bella-vista'
		      AND (LOWER(TRIM(r.name)) LIKE '%bella%vista%' OR LOWER(TRIM(r.name)) = 'bella vista')
		      AND EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista')))`)
	_, _ = a.Pool.Exec(ctx, `
		DELETE FROM order_lines ol
		USING menu_items mi
		WHERE ol.menu_item_id = mi.id
		  AND mi.restaurant_id IN (
		    SELECT r.id FROM restaurants r
		    WHERE r.slug IS DISTINCT FROM 'bella-vista'
		      AND (LOWER(TRIM(r.name)) LIKE '%bella%vista%' OR LOWER(TRIM(r.name)) = 'bella vista')
		      AND EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista'))`)
	ct3, err3 := a.Pool.Exec(ctx, `
		DELETE FROM restaurants r
		WHERE r.slug IS DISTINCT FROM 'bella-vista'
		  AND (LOWER(TRIM(r.name)) LIKE '%bella%vista%' OR LOWER(TRIM(r.name)) = 'bella vista')
		  AND EXISTS (SELECT 1 FROM restaurants x WHERE x.slug = 'bella-vista')`)
	if err3 == nil && ct3.RowsAffected() > 0 {
		log.Printf("сид: удалены дубликаты Bella Vista по шаблону имени (%d строк)", ct3.RowsAffected())
	}
}

func (a *Handlers) syncDemoMoscowRestaurants(ctx context.Context) {
	const addrLuna = "Москва, наб. Патриарших прудов, 10"
	const addrTrattoria = "Москва, ул. Тверская, 12"
	const nameTrattoria = "Траттория Тверская"
	const descTrattoria = "Итальянская кухня и вино"
	const addrBella = "Москва, Смоленская пл., 6"
	const descBella = "Итальянская кухня в центре Москвы"
	const nameBella = "Bella Vista"
	_, _ = a.Pool.Exec(ctx, `
		UPDATE restaurants SET city='Москва', address=$1
		WHERE slug='la-luna'`, addrLuna)
	_, _ = a.Pool.Exec(ctx, `
		UPDATE restaurants SET city='Москва', address=$1, description=$2, name=$3
		WHERE slug='trattoria-tverskaya'`, addrTrattoria, descTrattoria, nameTrattoria)
	_, _ = a.Pool.Exec(ctx, `
		UPDATE restaurants SET city='Москва', address=$1, description=$2, name=$3
		WHERE slug='bella-vista'`, addrBella, descBella, nameBella)
}

// syncDemoRestaurantContacts — телефон, часы, email, обложка и галерея для демо-slug (старые БД).
func (a *Handlers) syncDemoRestaurantContacts(ctx context.Context) {
	type row struct {
		slug, phone, opens, closes, email string
	}
	rows := []row{
		{DemoSlugTrattoria, "+7 (495) 111-20-01", "10:00", "23:00", "hello@trattoria-demo.rest"},
		{DemoSlugLaLuna, "+7 (495) 222-30-02", "11:00", "23:30", "kontakt@laluna-demo.rest"},
		{DemoSlugSakura, "+7 (495) 333-40-03", "12:00", "23:00", "info@sakura-demo.rest"},
		{DemoSlugBella, "+7 (495) 444-50-04", "10:00", "00:00", "ciao@bellavista-demo.rest"},
	}
	for _, r := range rows {
		patch, err := json.Marshal(map[string]any{
			"contact_email": r.email,
			"photo_gallery": demoRestaurantGalleryURLs(r.slug),
		})
		if err != nil {
			continue
		}
		_, _ = a.Pool.Exec(ctx, `
			UPDATE restaurants SET
				phone = $2,
				opens_at = $3,
				closes_at = $4,
				photo_url = $6,
				extra_json = COALESCE(extra_json, '{}'::jsonb) || $5::jsonb
			WHERE slug = $1`,
			r.slug, r.phone, r.opens, r.closes, patch, demoRestaurantCoverURL(r.slug))
	}
}

func (a *Handlers) backfillDemoMenuImages(ctx context.Context) {
	for _, slug := range []string{DemoSlugTrattoria, DemoSlugLaLuna, DemoSlugSakura, DemoSlugBella} {
		urls := demoMenuImagesForSlug(slug)
		if len(urls) == 0 {
			continue
		}
		var rid uuid.UUID
		if err := a.Pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE slug=$1`, slug).Scan(&rid); err != nil {
			continue
		}
		itemRows, err := a.Pool.Query(ctx, `
			SELECT id FROM menu_items WHERE restaurant_id=$1 AND COALESCE(image_url,'') = ''
			ORDER BY sort_order, name`, rid)
		if err != nil {
			continue
		}
		i := 0
		for itemRows.Next() {
			if i >= len(urls) {
				break
			}
			var mid uuid.UUID
			if itemRows.Scan(&mid) != nil {
				continue
			}
			_, _ = a.Pool.Exec(ctx, `UPDATE menu_items SET image_url=$2 WHERE id=$1`, mid, urls[i])
			i++
		}
		itemRows.Close()
	}
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
		WHERE slug IN ('trattoria-tverskaya','la-luna','sakura-lite','bella-vista')`).Scan(&n)
	if n < 4 {
		log.Printf("сид: в каталоге демо-ресторанов %d из 4 (slug trattoria / la-luna / sakura-lite / bella-vista)", n)
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
	own := a.ownerIfNoRestaurant(ctx, ownerID)
	if _, err := a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url, phone, opens_at, closes_at, extra_json)
		VALUES ($1,'Траттория Тверская','Москва, ул. Тверская, 12','trattoria-tverskaya','Москва','Итальянская кухня и вино',$2,$3,'+7 (495) 111-20-01','10:00','23:00',$4::jsonb)`,
		rid, own, demoRestaurantCoverURL(DemoSlugTrattoria), demoRestaurantExtraJSONForSeed("hello@trattoria-demo.rest", DemoSlugTrattoria)); err != nil {
		log.Printf("сид: trattoria-tverskaya: %v", err)
		return
	}
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Основной зал')`, hid, rid)
	wallsMain := demoTrattoriaMainHallLayoutJSON()
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
		wallsMain := demoTrattoriaMainHallLayoutJSON()
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
	own := a.ownerIfNoRestaurant(ctx, ownerID)
	if _, err := a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url, phone, opens_at, closes_at, extra_json)
		VALUES ($1,'La Luna','Москва, наб. Патриарших прудов, 10','la-luna','Москва','Европейская кухня',$2,$3,'+7 (495) 222-30-02','11:00','23:30',$4::jsonb)`,
		rid, own, demoRestaurantCoverURL(DemoSlugLaLuna), demoRestaurantExtraJSONForSeed("kontakt@laluna-demo.rest", DemoSlugLaLuna)); err != nil {
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
	own := a.ownerIfNoRestaurant(ctx, ownerID)
	if _, err := a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url, phone, opens_at, closes_at, extra_json)
		VALUES ($1,'Сакура Лайт','Москва, ул. Покровка, 3','sakura-lite','Москва','Японская кухня',$2,$3,'+7 (495) 333-40-03','12:00','23:00',$4::jsonb)`,
		rid, own, demoRestaurantCoverURL(DemoSlugSakura), demoRestaurantExtraJSONForSeed("info@sakura-demo.rest", DemoSlugSakura)); err != nil {
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

func (a *Handlers) topUpBellaVistaIfMissing(ctx context.Context, ownerID uuid.UUID) {
	var n int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM restaurants WHERE slug=$1`, "bella-vista").Scan(&n)
	if n > 0 {
		var rid uuid.UUID
		if err := a.Pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE slug=$1`, "bella-vista").Scan(&rid); err == nil {
			a.patchBellaVistaHallTablesMenu(ctx, rid)
		}
		return
	}
	rid := uuid.New()
	hid := uuid.New()
	own := a.ownerIfNoRestaurant(ctx, ownerID)
	if _, err := a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url, phone, opens_at, closes_at, extra_json)
		VALUES ($1,'Bella Vista','Москва, Смоленская пл., 6','bella-vista','Москва','Итальянская кухня в центре Москвы',$2,$3,'+7 (495) 444-50-04','10:00','00:00',$4::jsonb)`,
		rid, own, demoRestaurantCoverURL(DemoSlugBella), demoRestaurantExtraJSONForSeed("ciao@bellavista-demo.rest", DemoSlugBella)); err != nil {
		log.Printf("сид: bella-vista: %v", err)
		return
	}
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Основной зал')`, hid, rid)
	wallsBella := `{"walls":[{"x1":0,"y1":0,"x2":880,"y2":0},{"x1":880,"y1":0,"x2":880,"y2":600},{"x1":880,"y1":600,"x2":0,"y2":600},{"x1":0,"y1":600,"x2":0,"y2":0}],"decorations":[{"type":"zone_label","text":"Панорама","x":40,"y":36,"w":160,"h":28}]}`
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hid, wallsBella)
	bellaTables := []struct {
		n   int
		cap int
		x, y float64
	}{
		{1, 2, 120, 110}, {2, 4, 300, 130}, {3, 4, 500, 150},
		{4, 6, 220, 320}, {5, 4, 460, 340},
	}
	for _, t := range bellaTables {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, width, height)
			VALUES ($1,$2,$3,$4,$5,'rect','available',88,72)`, hid, t.n, t.cap, t.x, t.y)
	}
	a.seedMenuBellaVista(ctx, rid)
	log.Println("сид: дозаполнен ресторан bella-vista")
}

func (a *Handlers) patchBellaVistaHallTablesMenu(ctx context.Context, rid uuid.UUID) {
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
		wallsBella := `{"walls":[{"x1":0,"y1":0,"x2":880,"y2":0},{"x1":880,"y1":0,"x2":880,"y2":600},{"x1":880,"y1":600,"x2":0,"y2":600},{"x1":0,"y1":600,"x2":0,"y2":0}],"decorations":[{"type":"zone_label","text":"Панорама","x":40,"y":36,"w":160,"h":28}]}`
		_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hid, wallsBella)
	} else if tc == 0 {
		_ = a.Pool.QueryRow(ctx, `SELECT id FROM halls WHERE restaurant_id=$1 ORDER BY created_at LIMIT 1`, rid).Scan(&hid)
	}
	if hid != uuid.Nil && tc == 0 {
		bellaTables := []struct {
			n   int
			cap int
			x, y float64
		}{
			{1, 2, 120, 110}, {2, 4, 300, 130}, {3, 4, 500, 150},
			{4, 6, 220, 320}, {5, 4, 460, 340},
		}
		for _, t := range bellaTables {
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, width, height)
				VALUES ($1,$2,$3,$4,$5,'rect','available',88,72)`, hid, t.n, t.cap, t.x, t.y)
		}
		log.Printf("сид: bella-vista %s — добавлены столы", rid)
	}
	if mc == 0 {
		a.seedMenuBellaVista(ctx, rid)
		log.Printf("сид: bella-vista %s — дозаполнено меню", rid)
	}
}

// ensureDemoBellaAdmin — администратор Bella Vista (старые БД без полного seedBase).
func (a *Handlers) ensureDemoBellaAdmin(ctx context.Context, hashStr string) {
	var rid uuid.UUID
	if err := a.Pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE slug='bella-vista' LIMIT 1`).Scan(&rid); err != nil {
		return
	}
	_, _ = a.Pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, full_name, phone, role, email_verified, restaurant_id)
		VALUES ('admin-bella@demo.ru',$1,'Виктория Беллини','+79001234574','admin',true,$2)
		ON CONFLICT (email) DO UPDATE SET
			restaurant_id = EXCLUDED.restaurant_id,
			phone = EXCLUDED.phone,
			role = CASE WHEN users.role = 'owner' THEN users.role ELSE EXCLUDED.role END,
			full_name = CASE WHEN users.role = 'owner' THEN users.full_name ELSE EXCLUDED.full_name END`,
		hashStr, rid)
}

// ensureDemoWaitersForRestaurants создаёт официантов la-luna / sakura-lite / bella-vista, если их ещё нет (старые БД).
func (a *Handlers) ensureDemoWaitersForRestaurants(ctx context.Context, hashStr string) {
	type row struct {
		slug string
		email, name string
		phone string
	}
	rows := []row{
		{"la-luna", "waiter3@demo.ru", "Павел Невский", "+79001234571"},
		{"sakura-lite", "waiter4@demo.ru", "Юлия Сакура", "+79001234572"},
		{"bella-vista", "waiter5@demo.ru", "Марко Виста", "+79001234573"},
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
			ON CONFLICT (email) DO UPDATE SET
				restaurant_id = EXCLUDED.restaurant_id,
				full_name = CASE WHEN users.role = 'owner' THEN users.full_name ELSE EXCLUDED.full_name END,
				role = CASE WHEN users.role = 'owner' THEN users.role ELSE EXCLUDED.role END`,
			r.email, hashStr, r.name, r.phone, rid)
	}
}
