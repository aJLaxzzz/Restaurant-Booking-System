package handlers

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (a *Handlers) Seed(ctx context.Context) {
	var hallCount int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM halls`).Scan(&hallCount)
	if hallCount == 0 {
		a.seedBase(ctx)
	}
	var resCount int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM reservations`).Scan(&resCount)
	if resCount == 0 {
		a.seedLifeData(ctx)
	}
}

func (a *Handlers) seedBase(ctx context.Context) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("Password1"), a.Cfg.BcryptCost)
	hashStr := string(hash)

	rid := uuid.New()
	_, _ = a.Pool.Exec(ctx, `INSERT INTO restaurants (id, name, address) VALUES ($1,'Bella Vista','Москва, ул. Тверская, 12')`, rid)

	hidMain := uuid.New()
	hidTerrace := uuid.New()
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Основной зал')`, hidMain, rid)
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Летняя терраса')`, hidTerrace, rid)

	wallsMain := `[{"x1":0,"y1":0,"x2":920,"y2":0},{"x1":920,"y1":0,"x2":920,"y2":640},{"x1":920,"y1":640,"x2":0,"y2":640},{"x1":0,"y1":640,"x2":0,"y2":0}]`
	wallsTerr := `[{"x1":0,"y1":0,"x2":720,"y2":0},{"x1":720,"y1":0,"x2":720,"y2":480},{"x1":720,"y1":480,"x2":0,"y2":480},{"x1":0,"y1":480,"x2":0,"y2":0}]`
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hidMain, wallsMain)
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hidTerrace, wallsTerr)

	// Основной зал: сетка 4×3
	mainTables := []struct {
		num  int
		cap  int
		x, y float64
	}{
		{1, 2, 100, 100}, {2, 4, 260, 100}, {3, 4, 420, 100}, {4, 6, 580, 100},
		{5, 4, 100, 260}, {6, 2, 260, 260}, {7, 8, 420, 260}, {8, 4, 580, 260},
		{9, 4, 100, 420}, {10, 6, 280, 420}, {11, 4, 460, 420}, {12, 2, 620, 420},
	}
	for _, t := range mainTables {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status)
			VALUES ($1,$2,$3,$4,$5,'circle','available')`, hidMain, t.num, t.cap, t.x, t.y)
	}

	// Терраса: 6 столов, один заблокирован
	terrTables := []struct {
		num  int
		cap  int
		x, y float64
		st   string
		br   *string
	}{
		{1, 4, 120, 120, "available", nil},
		{2, 2, 300, 120, "available", nil},
		{3, 6, 480, 120, "blocked", strPtr("Ремонт навеса")},
		{4, 4, 120, 300, "available", nil},
		{5, 4, 300, 300, "available", nil},
		{6, 8, 480, 300, "available", nil},
	}
	for _, t := range terrTables {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, block_reason)
			VALUES ($1,$2,$3,$4,$5,'rect',$6,$7,$8)`, hidTerrace, t.num, t.cap, t.x, t.y, t.st, t.br)
	}

	staff := []struct {
		email, name, role string
	}{
		{"owner@demo.ru", "Михаил Волков", "owner"},
		{"admin@demo.ru", "Елена Орлова", "admin"},
		{"waiter@demo.ru", "Дмитрий Козлов", "waiter"},
		{"waiter2@demo.ru", "Светлана Морозова", "waiter"},
	}
	for _, u := range staff {
		_, err := a.Pool.Exec(ctx, `
			INSERT INTO users (email, password_hash, full_name, phone, role, email_verified)
			VALUES ($1,$2,$3,'+79001234567',$4,true)
			ON CONFLICT (email) DO NOTHING`,
			u.email, hashStr, u.name, u.role)
		if err != nil {
			log.Printf("seed staff %s: %v", u.email, err)
		}
	}

	clients := []struct {
		email, name, phone string
	}{
		{"client@demo.ru", "Алексей Петров", "+79161230001"},
		{"client1@demo.ru", "Анна Иванова", "+79161230002"},
		{"client2@demo.ru", "Пётр Смирнов", "+79161230003"},
		{"client3@demo.ru", "Ольга Кузнецова", "+79161230004"},
		{"client4@demo.ru", "Игорь Новиков", "+79161230005"},
		{"client5@demo.ru", "Мария Попова", "+79161230006"},
		{"client6@demo.ru", "Сергей Васильев", "+79161230007"},
		{"client7@demo.ru", "Екатерина Соколова", "+79161230008"},
		{"client8@demo.ru", "Андрей Михайлов", "+79161230009"},
		{"client9@demo.ru", "Наталья Фёдорова", "+79161230010"},
		{"client10@demo.ru", "Виктор Семёнов", "+79161230011"},
		{"client11@demo.ru", "Татьяна Егорова", "+79161230012"},
		{"client12@demo.ru", "Роман Павлов", "+79161230013"},
		{"client13@demo.ru", "Дарья Лебедева", "+79161230014"},
		{"client14@demo.ru", "Константин Захаров", "+79161230015"},
		{"client15@demo.ru", "Юлия Белова", "+79161230016"},
	}
	for _, u := range clients {
		_, err := a.Pool.Exec(ctx, `
			INSERT INTO users (email, password_hash, full_name, phone, role, email_verified)
			VALUES ($1,$2,$3,$4,'client',true)
			ON CONFLICT (email) DO NOTHING`,
			u.email, hashStr, u.name, u.phone)
		if err != nil {
			log.Printf("seed client %s: %v", u.email, err)
		}
	}

	log.Println("БД: ресторан, 2 зала, 18 столов, staff + 16 клиентов (пароль Password1)")
}

func strPtr(s string) *string { return &s }

func (a *Handlers) seedLifeData(ctx context.Context) {
	var wid1, wid2 uuid.UUID
	if err := a.Pool.QueryRow(ctx, `SELECT id FROM users WHERE email='waiter@demo.ru'`).Scan(&wid1); err != nil {
		log.Printf("seedLifeData: нет официанта waiter@demo.ru: %v", err)
		return
	}
	if err := a.Pool.QueryRow(ctx, `SELECT id FROM users WHERE email='waiter2@demo.ru'`).Scan(&wid2); err != nil {
		wid2 = wid1
	}

	rows, err := a.Pool.Query(ctx, `SELECT t.id, t.hall_id, t.table_number FROM tables t ORDER BY t.hall_id, t.table_number`)
	if err != nil {
		log.Printf("seedLifeData tables: %v", err)
		return
	}
	type trow struct {
		id       uuid.UUID
		hallID   uuid.UUID
		tableNum int
	}
	var tables []trow
	for rows.Next() {
		var r trow
		_ = rows.Scan(&r.id, &r.hallID, &r.tableNum)
		tables = append(tables, r)
	}
	rows.Close()
	if len(tables) < 5 {
		log.Println("seedLifeData: мало столов, пропуск")
		return
	}

	clientRows, err := a.Pool.Query(ctx, `SELECT id, email FROM users WHERE role='client' ORDER BY email`)
	if err != nil {
		return
	}
	var clients []struct {
		id    uuid.UUID
		email string
	}
	for clientRows.Next() {
		var c struct {
			id    uuid.UUID
			email string
		}
		_ = clientRows.Scan(&c.id, &c.email)
		clients = append(clients, c)
	}
	clientRows.Close()
	if len(clients) == 0 {
		return
	}

	now := time.Now().UTC()
	slot := 2 * time.Hour
	day := 24 * time.Hour

	// Хелпер: взять стол по индексу (по модулю)
	tAt := func(i int) uuid.UUID { return tables[i%len(tables)].id }
	clientAt := func(i int) uuid.UUID { return clients[i%len(clients)].id }

	type spec struct {
		ti         int // индекс стола в tables
		ci         int // индекс клиента
		start, end time.Time
		status     string
		guests     int
		comment    string
		waiter     *uuid.UUID
		createdBy  string
		seated     *time.Time
		svcStart   *time.Time
		completed  *time.Time
		payStatus  string // "", "succeeded", "pending", "succeeded_refund"
	}

	var specs []spec

	// Прошлое: завершённые визиты (~18)
	for d := 1; d <= 18; d++ {
		start := now.Add(-day * time.Duration(d+3)).Add(11 * time.Hour)
		if d%3 == 0 {
			start = start.Add(6 * time.Hour) // ужин
		}
		end := start.Add(slot)
		st := "completed"
		completed := end.Add(45 * time.Minute)
		seated := start.Add(5 * time.Minute)
		svc := seated.Add(10 * time.Minute)
		w := &wid1
		if d%2 == 0 {
			w = &wid2
		}
		specs = append(specs, spec{
			ti: d, ci: d, start: start, end: end, status: st, guests: 2 + (d % 5),
			comment:   pickComment(d),
			waiter:    w,
			createdBy: "client",
			seated:    &seated, svcStart: &svc, completed: &completed,
			payStatus: "succeeded",
		})
		if d%7 == 0 {
			specs[len(specs)-1].payStatus = "succeeded_refund"
		}
	}

	// Отмены и no-show
	for i, st := range []string{"cancelled_by_client", "cancelled_by_admin", "no_show", "cancelled_by_client", "no_show"} {
		start := now.Add(-day * time.Duration(5+i)).Add(13 * time.Hour)
		specs = append(specs, spec{
			ti: 3 + i, ci: 4 + i, start: start, end: start.Add(slot), status: st, guests: 3,
			comment: "перенос планов", createdBy: "client", payStatus: "",
		})
	}

	// Сегодня и ближайшие дни: активные статусы
	todayLunch := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
	todayEve := time.Date(now.Year(), now.Month(), now.Day(), 18, 30, 0, 0, time.UTC)

	specs = append(specs, spec{
		ti: 0, ci: 1, start: todayLunch, end: todayLunch.Add(slot), status: "seated", guests: 3,
		comment: "детское кресло", waiter: &wid1, createdBy: "client",
		seated: ptrTime(todayLunch.Add(3 * time.Minute)), payStatus: "succeeded",
	})
	specs = append(specs, spec{
		ti: 1, ci: 2, start: todayEve, end: todayEve.Add(slot), status: "in_service", guests: 4,
		comment: "день рождения", waiter: &wid1, createdBy: "client",
		seated: ptrTime(todayEve.Add(2 * time.Minute)), svcStart: ptrTime(todayEve.Add(20 * time.Minute)),
		payStatus: "succeeded",
	})
	specs = append(specs, spec{
		ti: 2, ci: 3, start: todayLunch.Add(3 * time.Hour), end: todayLunch.Add(3*time.Hour + slot), status: "confirmed", guests: 2,
		comment: "", waiter: &wid2, createdBy: "client", payStatus: "",
	})

	// Будущие брони
	for i := 1; i <= 12; i++ {
		start := now.Add(day * time.Duration(i+1)).Add(time.Duration(10+i%8) * time.Hour)
		end := start.Add(slot)
		st := "confirmed"
		ps := ""
		if i%5 == 0 {
			st = "pending_payment"
			ps = "pending"
		}
		w := &wid2
		if i%2 == 0 {
			w = &wid1
		}
		specs = append(specs, spec{
			ti: 4 + i, ci: 5 + i, start: start, end: end, status: st, guests: 2 + (i % 6),
			comment: futureNote(i), waiter: w, createdBy: "client", payStatus: ps,
		})
	}

	// Админская бронь на VIP-стол
	specs = append(specs, spec{
		ti: 6, ci: 8, start: now.Add(day * 2).Add(19 * time.Hour), end: now.Add(day * 2).Add(21 * time.Hour),
		status: "confirmed", guests: 6, comment: "Корпоратив (админ)", waiter: &wid1,
		createdBy: "admin", payStatus: "succeeded",
	})

	inserted := 0
	for _, s := range specs {
		tid := tAt(s.ti)
		uid := clientAt(s.ci)
		var cap int
		if err := a.Pool.QueryRow(ctx, `SELECT capacity FROM tables WHERE id=$1`, tid).Scan(&cap); err != nil {
			log.Printf("seed capacity: %v", err)
			continue
		}
		guests := s.guests
		if guests > cap {
			guests = cap
		}
		if guests < 1 {
			guests = 1
		}
		var rid uuid.UUID
		err := a.Pool.QueryRow(ctx, `
			INSERT INTO reservations (
				table_id, user_id, start_time, end_time, guest_count, status, comment, created_by,
				assigned_waiter_id, seated_at, service_started_at, completed_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			RETURNING id`,
			tid, uid, s.start, s.end, guests, s.status, nullStr(s.comment), s.createdBy,
			s.waiter, s.seated, s.svcStart, s.completed,
		).Scan(&rid)
		if err != nil {
			log.Printf("seed reservation: %v", err)
			continue
		}
		inserted++

		switch s.payStatus {
		case "succeeded":
			amount := 45000 + (guests * 15000) + (inserted%17)*1000
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO payments (reservation_id, amount_kopecks, status, idempotency_key, gateway_payment_id)
				VALUES ($1,$2,'succeeded',$3,$4)`,
				rid, amount, uuid.New(), "seed_"+rid.String()[:8])
		case "succeeded_refund":
			amount := 120000
			refund := 36000
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO payments (reservation_id, amount_kopecks, status, idempotency_key, refund_amount_kopecks, gateway_payment_id)
				VALUES ($1,$2,'succeeded',$3,$4,$5)`,
				rid, amount, uuid.New(), refund, "seed_ref_"+rid.String()[:8])
		case "pending":
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO payments (reservation_id, amount_kopecks, status, idempotency_key)
				VALUES ($1,$2,'pending',$3)`,
				rid, 90000, uuid.New())
		}

		if s.waiter != nil && (s.status == "confirmed" || s.status == "seated" || s.status == "in_service") {
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO table_assignments (reservation_id, staff_user_id, table_id)
				VALUES ($1,$2,$3)`,
				rid, *s.waiter, tid)
		}
	}

	// Уведомления
	for i := 0; i < 12; i++ {
		uid := clientAt(i)
		sentAt := time.Now().UTC().Add(-day * time.Duration(i+1))
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO notifications (user_id, type, template, status, sent_at)
			VALUES ($1,'booking','reminder_24h','sent',$2)`,
			uid, sentAt)
	}

	// Заметки официантов (последние брони с официантом)
	noteRows, errNotes := a.Pool.Query(ctx, `
		SELECT r.id FROM reservations r
		WHERE r.assigned_waiter_id IS NOT NULL AND r.status IN ('completed','seated','in_service')
		ORDER BY r.start_time DESC LIMIT 8`)
	if errNotes != nil {
		log.Printf("seedLifeData notes query: %v", errNotes)
	} else {
		defer noteRows.Close()
		var nids []uuid.UUID
		for noteRows.Next() {
			var id uuid.UUID
			_ = noteRows.Scan(&id)
			nids = append(nids, id)
		}
		notes := []string{
			"Просят воду без газа на стол.",
			"Аллергия на орехи — передать на кухню.",
			"Гости опоздают на 15 мин.",
			"Праздничный торт принесут сами в 20:00.",
			"Предпочитают стол у окна — выполнено.",
		}
		for i, resID := range nids {
			if i >= len(notes) {
				break
			}
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO waiter_notes (reservation_id, user_id, note)
				VALUES ($1,$2,$3)`, resID, wid1, notes[i])
		}
	}

	// Статусы столов: занятость для активных броней
	_, _ = a.Pool.Exec(ctx, `
		UPDATE tables t SET status='occupied', updated_at=NOW()
		WHERE EXISTS (
			SELECT 1 FROM reservations r
			WHERE r.table_id = t.id
			AND r.status IN ('seated','in_service')
			AND r.start_time <= NOW() AND r.end_time >= NOW()
		)`)

	log.Printf("Живые данные: брони и связанные записи (успешных вставок: %d)", inserted)
}

func ptrTime(t time.Time) *time.Time { return &t }

func pickComment(i int) string {
	cm := []string{
		"Без лука", "Вегетарианское меню", "Детский стул",
		"У окна", "Бизнес-ланч", "Свидание", "Юбилей",
		"Тихий столик", "Командировка", "Встреча с партнёрами",
	}
	return cm[i%len(cm)]
}

func futureNote(i int) string {
	cm := []string{
		"", "поздравить с годовщиной", "пробка — могут опоздать",
		"алкоголь заранее не заказывать", "2 детских меню",
	}
	return cm[i%len(cm)]
}
