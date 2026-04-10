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
	// Всегда: дозаполнение по slug (trattoria / la-luna / sakura / bella-vista). Нужно после частичного seedBase
	// (один ресторан, остальные INSERT с тем же slug упали) — иначе ветка hallCount==0 никогда
	// не вызывала ensureExtra, и на главной оставался один ресторан.
	a.ensureExtraDemoRestaurants(ctx)
	var resCount int
	_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM reservations`).Scan(&resCount)
	if resCount == 0 {
		a.seedLifeData(ctx)
	}
	a.ensureRestaurantTodayTomorrowDemoBookings(ctx, "trattoria-tverskaya", demoTrattoriaNearMarker, "waiter@demo.ru", "waiter2@demo.ru")
	a.ensureRestaurantTodayTomorrowDemoBookings(ctx, "bella-vista", demoBellaNearMarker, "waiter5@demo.ru", "")
	a.ensureSuperadminUser(ctx)
	a.ensureDefaultSettings(ctx)
}

func (a *Handlers) ensureSuperadminUser(ctx context.Context) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Password1"), a.Cfg.BcryptCost)
	if err != nil {
		return
	}
	_, _ = a.Pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, full_name, phone, role, email_verified)
		VALUES ('superadmin@demo.ru',$1,'Системный администратор','+79000000000','superadmin',true)
		ON CONFLICT (email) DO UPDATE SET
			role = 'superadmin',
			password_hash = EXCLUDED.password_hash,
			full_name = EXCLUDED.full_name`,
		string(hash))
}

func (a *Handlers) ensureDefaultSettings(ctx context.Context) {
	_, _ = a.Pool.Exec(ctx, `
		INSERT INTO settings (key, value) VALUES ('no_show_grace_minutes', '{"minutes":20}'::jsonb)
		ON CONFLICT (key) DO NOTHING`)
}

func (a *Handlers) seedBase(ctx context.Context) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("Password1"), a.Cfg.BcryptCost)
	hashStr := string(hash)

	var owner1ID, owner2ID uuid.UUID
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, phone, role, email_verified)
		VALUES ('owner@demo.ru',$1,'Михаил Волков','+79001234567','owner',true)
		ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email
		RETURNING id`, hashStr).Scan(&owner1ID)
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, phone, role, email_verified)
		VALUES ('owner2@demo.ru',$1,'Анна Север','+79001234568','owner',true)
		ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email
		RETURNING id`, hashStr).Scan(&owner2ID)

	var owner3ID, owner4ID uuid.UUID
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, phone, role, email_verified)
		VALUES ('owner3@demo.ru',$1,'Денис Океан','+79001234569','owner',true)
		ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email
		RETURNING id`, hashStr).Scan(&owner3ID)
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, phone, role, email_verified)
		VALUES ('owner-bella@demo.ru',$1,'Рикардо Беллини','+79001234570','owner',true)
		ON CONFLICT (email) DO UPDATE SET email=EXCLUDED.email
		RETURNING id`, hashStr).Scan(&owner4ID)

	rid := uuid.New()
	rid2 := uuid.New()
	rid3 := uuid.New()
	rid4 := uuid.New()
	_, _ = a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url, phone, opens_at, closes_at, extra_json)
		VALUES ($1,'Траттория Тверская','Москва, ул. Тверская, 12','trattoria-tverskaya','Москва','Итальянская кухня и вино',$2,$3,'+7 (495) 111-20-01','10:00','23:00','{"contact_email":"hello@trattoria-demo.rest"}'::jsonb)`, rid, owner1ID, demoPhotoTrattoria)
	_, _ = a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url, phone, opens_at, closes_at, extra_json)
		VALUES ($1,'La Luna','Москва, наб. Патриарших прудов, 10','la-luna','Москва','Европейская кухня',$2,$3,'+7 (495) 222-30-02','11:00','23:30','{"contact_email":"kontakt@laluna-demo.rest"}'::jsonb)`, rid2, owner2ID, demoPhotoLaLuna)
	_, _ = a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url, phone, opens_at, closes_at, extra_json)
		VALUES ($1,'Сакура Лайт','Москва, ул. Покровка, 3','sakura-lite','Москва','Японская кухня',$2,$3,'+7 (495) 333-40-03','12:00','23:00','{"contact_email":"info@sakura-demo.rest"}'::jsonb)`, rid3, owner3ID, demoPhotoSakura)
	_, _ = a.Pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, slug, city, description, owner_user_id, photo_url, phone, opens_at, closes_at, extra_json)
		VALUES ($1,'Bella Vista','Москва, Смоленская пл., 6','bella-vista','Москва','Итальянская кухня в центре Москвы',$2,$3,'+7 (495) 444-50-04','10:00','00:00','{"contact_email":"ciao@bellavista-demo.rest"}'::jsonb)`, rid4, owner4ID, demoPhotoBella)

	hidMain := uuid.New()
	hidTerrace := uuid.New()
	hidLuna := uuid.New()
	hidSakura := uuid.New()
	hidBella := uuid.New()
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Основной зал')`, hidMain, rid)
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Летняя терраса')`, hidTerrace, rid)
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Главный зал')`, hidLuna, rid2)
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Зал татами')`, hidSakura, rid3)
	_, _ = a.Pool.Exec(ctx, `INSERT INTO halls (id, restaurant_id, name) VALUES ($1,$2,'Зал Bella Vista')`, hidBella, rid4)

	wallsMain := `{"walls":[{"x1":0,"y1":0,"x2":920,"y2":0},{"x1":920,"y1":0,"x2":920,"y2":640},{"x1":920,"y1":640,"x2":0,"y2":640},{"x1":0,"y1":640,"x2":0,"y2":0}],"decorations":[{"type":"zone_label","text":"Панорамные окна","x":60,"y":40,"w":200,"h":32},{"type":"window_band","x":0,"y":0,"w":920,"h":24}]}`
	wallsTerr := `{"walls":[{"x1":0,"y1":0,"x2":720,"y2":0},{"x1":720,"y1":0,"x2":720,"y2":480},{"x1":720,"y1":480,"x2":0,"y2":480},{"x1":0,"y1":480,"x2":0,"y2":0}],"decorations":[]}`
	wallsLuna := `{"walls":[{"x1":0,"y1":0,"x2":800,"y2":0},{"x1":800,"y1":0,"x2":800,"y2":560},{"x1":800,"y1":560,"x2":0,"y2":560},{"x1":0,"y1":560,"x2":0,"y2":0}],"decorations":[{"type":"zone_label","text":"Центр зала","x":360,"y":260,"w":120,"h":28}]}`
	wallsSakura := `{"walls":[{"x1":0,"y1":0,"x2":680,"y2":0},{"x1":680,"y1":0,"x2":680,"y2":520},{"x1":680,"y1":520,"x2":0,"y2":520},{"x1":0,"y1":520,"x2":0,"y2":0}],"decorations":[]}`
	wallsBella := `{"walls":[{"x1":0,"y1":0,"x2":880,"y2":0},{"x1":880,"y1":0,"x2":880,"y2":600},{"x1":880,"y1":600,"x2":0,"y2":600},{"x1":0,"y1":600,"x2":0,"y2":0}],"decorations":[{"type":"zone_label","text":"Панорама","x":40,"y":36,"w":160,"h":28}]}`
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hidMain, wallsMain)
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hidTerrace, wallsTerr)
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hidLuna, wallsLuna)
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hidSakura, wallsSakura)
	_, _ = a.Pool.Exec(ctx, `UPDATE halls SET layout_json=$2::jsonb WHERE id=$1`, hidBella, wallsBella)

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
		{7, 8, 420, 260, "rect", 120, 88, 0},
		{8, 4, 580, 260, "rect", 88, 64, -8},
		{9, 4, 100, 420, "rect", 88, 64, 0},
		{10, 6, 280, 420, "ellipse", 104, 80, 0},
		{11, 4, 460, 420, "rect", 88, 64, 0},
		{12, 2, 620, 420, "round", 48, 48, 0},
	}
	for _, t := range mainTables {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, width, height, rotation_deg)
			VALUES ($1,$2,$3,$4,$5,$6,'available',$7,$8,$9)`, hidMain, t.num, t.cap, t.x, t.y, t.shape, t.w, t.h, t.rot)
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
			INSERT INTO tables (hall_id, table_number, capacity, x_coordinate, y_coordinate, shape, status, block_reason, width, height)
			VALUES ($1,$2,$3,$4,$5,'rect',$6,$7,88,88)`, hidTerrace, t.num, t.cap, t.x, t.y, t.st, t.br)
	}

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
			VALUES ($1,$2,$3,$4,$5,'rect','available',80,72)`, hidLuna, t.n, t.cap, t.x, t.y)
	}

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
			VALUES ($1,$2,$3,$4,$5,'rect','available',88,72)`, hidSakura, t.n, t.cap, t.x, t.y)
	}

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
			VALUES ($1,$2,$3,$4,$5,'rect','available',88,72)`, hidBella, t.n, t.cap, t.x, t.y)
	}
	staff := []struct {
		email, name, role, phone string
		rid                      uuid.UUID
	}{
		{"admin@demo.ru", "Елена Орлова", "admin", "+79001234567", rid},
		{"admin-bella@demo.ru", "Виктория Беллини", "admin", "+79001234574", rid4},
		{"waiter@demo.ru", "Дмитрий Козлов", "waiter", "+79001234567", rid},
		{"waiter2@demo.ru", "Светлана Морозова", "waiter", "+79001234567", rid},
		{"waiter3@demo.ru", "Павел Невский", "waiter", "+79001234571", rid2},
		{"waiter4@demo.ru", "Юлия Сакура", "waiter", "+79001234572", rid3},
		{"waiter5@demo.ru", "Марко Виста", "waiter", "+79001234573", rid4},
	}
	for _, u := range staff {
		_, err := a.Pool.Exec(ctx, `
			INSERT INTO users (email, password_hash, full_name, phone, role, email_verified, restaurant_id)
			VALUES ($1,$2,$3,$4,$5,true,$6)
			ON CONFLICT (email) DO UPDATE SET
				restaurant_id = EXCLUDED.restaurant_id,
				phone = EXCLUDED.phone,
				role = CASE WHEN users.role = 'owner' THEN users.role ELSE EXCLUDED.role END,
				full_name = CASE WHEN users.role = 'owner' THEN users.full_name ELSE EXCLUDED.full_name END`,
			u.email, hashStr, u.name, u.phone, u.role, u.rid)
		if err != nil {
			log.Printf("seed staff %s: %v", u.email, err)
		}
	}

	a.seedMenuTrattoria(ctx, rid)
	a.seedMenuLaLuna(ctx, rid2)
	a.seedMenuSakura(ctx, rid3)
	a.seedMenuBellaVista(ctx, rid4)

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
		{"client16@demo.ru", "Олег Степанов", "+79161230017"},
		{"client17@demo.ru", "Вера Крылова", "+79161230018"},
		{"client18@demo.ru", "Максим Орлов", "+79161230019"},
		{"client19@demo.ru", "Софья Зайцева", "+79161230020"},
		{"client20@demo.ru", "Илья Громов", "+79161230021"},
		{"client21@demo.ru", "Алина Волкова", "+79161230022"},
		{"client22@demo.ru", "Тимур Алиев", "+79161230023"},
		{"client23@demo.ru", "Елена Сидорова", "+79161230024"},
		{"client24@demo.ru", "Артём Ким", "+79161230025"},
		{"client25@demo.ru", "Полина Чернова", "+79161230026"},
		{"client26@demo.ru", "Глеб Фролов", "+79161230027"},
		{"client27@demo.ru", "Диана Рыбакова", "+79161230028"},
		{"client28@demo.ru", "Станислав Мартынов", "+79161230029"},
		{"client29@demo.ru", "Ксения Логинова", "+79161230030"},
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

	log.Println("БД: 4 демо-ресторана, залы, столы, меню, staff + клиенты (пароль Password1)")
}

func (a *Handlers) seedMenuTrattoria(ctx context.Context, restaurantID uuid.UUID) {
	var catFood, catDrink, subPizza uuid.UUID
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Кухня',0) RETURNING id`, restaurantID).Scan(&catFood)
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Бар',1) RETURNING id`, restaurantID).Scan(&catDrink)
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, parent_id, name, sort_order) VALUES ($1,$2,'Пицца',0) RETURNING id`, restaurantID, catFood).Scan(&subPizza)

	items := []struct {
		cat uuid.UUID
		n   string
		pr  int
	}{
		{subPizza, "Маргарита", 69000},
		{subPizza, "Четыре сыра", 89000},
		{catFood, "Паста карбонара", 65000},
		{catFood, "Тирамису", 42000},
		{catDrink, "Домашний лимонад", 29000},
		{catDrink, "Эспрессо", 18000},
	}
	for i, it := range items {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO menu_items (restaurant_id, category_id, name, price_kopecks, sort_order, image_url)
			VALUES ($1,$2,$3,$4,0,$5)`, restaurantID, it.cat, it.n, it.pr, demoDishImageAt(i))
	}
}

func (a *Handlers) seedMenuLaLuna(ctx context.Context, restaurantID uuid.UUID) {
	var catMain, catBar uuid.UUID
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Европейская кухня',0) RETURNING id`, restaurantID).Scan(&catMain)
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Напитки',1) RETURNING id`, restaurantID).Scan(&catBar)
	items := []struct {
		cat uuid.UUID
		n   string
		pr  int
	}{
		{catMain, "Утиная грудка с вишнёвым соусом", 78000},
		{catMain, "Ризотто с белыми грибами", 72000},
		{catMain, "Тартар из лосося", 69000},
		{catBar, "Капучино", 22000},
		{catBar, "Домашний лимонад", 31000},
	}
	for i, it := range items {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO menu_items (restaurant_id, category_id, name, price_kopecks, sort_order, image_url)
			VALUES ($1,$2,$3,$4,0,$5)`, restaurantID, it.cat, it.n, it.pr, demoDishImageAt(i+3))
	}
}

func (a *Handlers) seedMenuSakura(ctx context.Context, restaurantID uuid.UUID) {
	var catRoll, catHot uuid.UUID
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Роллы и суши',0) RETURNING id`, restaurantID).Scan(&catRoll)
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Горячее',1) RETURNING id`, restaurantID).Scan(&catHot)
	items := []struct {
		cat uuid.UUID
		n   string
		pr  int
	}{
		{catRoll, "Филадельфия", 59000},
		{catRoll, "Калифорния", 52000},
		{catHot, "Рамен с курицей", 48000},
		{catHot, "Тяхан с морепродуктами", 55000},
		{catHot, "Мисо-суп", 29000},
	}
	for i, it := range items {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO menu_items (restaurant_id, category_id, name, price_kopecks, sort_order, image_url)
			VALUES ($1,$2,$3,$4,0,$5)`, restaurantID, it.cat, it.n, it.pr, demoDishImageAt(i+8))
	}
}

func (a *Handlers) seedMenuBellaVista(ctx context.Context, restaurantID uuid.UUID) {
	var catAntipasti, catPasta, catSecondi, catDolci, catBevande uuid.UUID
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Закуски',0) RETURNING id`, restaurantID).Scan(&catAntipasti)
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Паста и ризотто',1) RETURNING id`, restaurantID).Scan(&catPasta)
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Основные блюда',2) RETURNING id`, restaurantID).Scan(&catSecondi)
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Десерты',3) RETURNING id`, restaurantID).Scan(&catDolci)
	_ = a.Pool.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order) VALUES ($1,'Напитки',4) RETURNING id`, restaurantID).Scan(&catBevande)
	items := []struct {
		cat uuid.UUID
		n   string
		d   string
		pr  int
	}{
		{catAntipasti, "Брускетта с томатами и базиликом", "Свежий хлеб, чеснок, оливковое масло", 42000},
		{catAntipasti, "Вителло тоннато", "Телятина, соус тоннато, каперсы", 69000},
		{catAntipasti, "Капрезе", "Моцарелла буффала, томаты", 55000},
		{catPasta, "Спагетти карбонара", "Гуанчиале, яйцо, пекорино", 72000},
		{catPasta, "Тальятелле с белыми грибами", "Сливки, пармезан", 78000},
		{catPasta, "Ризотто с шафраном и морепродуктами", "", 89000},
		{catSecondi, "Оссобуко по-милански", "Тушёная голень, гремолата", 125000},
		{catSecondi, "Рыба дня на гриле", "Сезонные овощи", 98000},
		{catDolci, "Панна котта с ягодами", "", 39000},
		{catDolci, "Тирамису", "Маскарпоне, кофе, какао", 45000},
		{catBevande, "Эспрессо", "", 22000},
		{catBevande, "Апероль-спритц", "", 49000},
		{catBevande, "Минеральная вода 0,75 л", "", 35000},
	}
	for i, it := range items {
		_, _ = a.Pool.Exec(ctx, `
			INSERT INTO menu_items (restaurant_id, category_id, name, description, price_kopecks, sort_order, image_url)
			VALUES ($1,$2,$3,$4,$5,0,$6)`, restaurantID, it.cat, it.n, it.d, it.pr, demoDishImageAt(i+12))
	}
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

	// Сегодня (календарный день по Москве, как в списке броней админа)
	loc, errLoc := time.LoadLocation("Europe/Moscow")
	if errLoc != nil {
		loc = time.UTC
	}
	nowMsk := now.In(loc)
	todayLunch := time.Date(nowMsk.Year(), nowMsk.Month(), nowMsk.Day(), 12, 0, 0, 0, loc).UTC()
	todayEve := time.Date(nowMsk.Year(), nowMsk.Month(), nowMsk.Day(), 18, 30, 0, 0, loc).UTC()

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

	a.seedClosedOrdersForCompleted(ctx)

	log.Printf("Живые данные: брони и связанные записи (успешных вставок: %d)", inserted)
}

// seedClosedOrdersForCompleted добавляет закрытые счета, строки меню и платежи tab по завершённым визитам (аналитика, Excel).
func (a *Handlers) seedClosedOrdersForCompleted(ctx context.Context) {
	rows, err := a.Pool.Query(ctx, `
		SELECT r.id, COALESCE(r.completed_at, r.end_time), h.restaurant_id
		FROM reservations r
		JOIN tables t ON t.id = r.table_id
		JOIN halls h ON h.id = t.hall_id
		WHERE r.status = 'completed'
		AND r.completed_at IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM reservation_orders ro WHERE ro.reservation_id = r.id)
		ORDER BY r.completed_at DESC
		LIMIT 80`)
	if err != nil {
		log.Printf("seedClosedOrdersForCompleted: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var resID uuid.UUID
		var completed time.Time
		var restID uuid.UUID
		if err := rows.Scan(&resID, &completed, &restID); err != nil {
			continue
		}
		itemRows, err := a.Pool.Query(ctx, `
			SELECT id, price_kopecks FROM menu_items
			WHERE restaurant_id = $1
			ORDER BY random()
			LIMIT 4`, restID)
		if err != nil {
			continue
		}
		type mi struct {
			id    uuid.UUID
			price int
		}
		var items []mi
		for itemRows.Next() {
			var m mi
			_ = itemRows.Scan(&m.id, &m.price)
			items = append(items, m)
		}
		itemRows.Close()
		if len(items) == 0 {
			continue
		}
		nLines := 2
		if len(items) < 2 {
			nLines = len(items)
		}
		var oid uuid.UUID
		if err := a.Pool.QueryRow(ctx, `
			INSERT INTO reservation_orders (reservation_id, status, created_at, updated_at)
			VALUES ($1, 'closed', $2, $2)
			RETURNING id`, resID, completed).Scan(&oid); err != nil {
			continue
		}
		var tabTotal int64
		for i := 0; i < nLines; i++ {
			qty := 1 + (i % 2)
			it := items[i%len(items)]
			lineSum := int64(qty * it.price)
			tabTotal += lineSum
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO order_lines (order_id, menu_item_id, quantity, guest_label)
				VALUES ($1, $2, $3, $4)`,
				oid, it.id, qty, "гость")
		}
		if tabTotal > 0 {
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO payments (reservation_id, reservation_order_id, amount_kopecks, status, idempotency_key, purpose, gateway_payment_id)
				VALUES ($1, $2, $3, 'succeeded', $4, 'tab', $5)`,
				resID, oid, tabTotal, uuid.New(), "seed_tab_"+oid.String()[:12])
		}
	}
}

// Маркеры в comment: при каждом Seed пересоздаём эти брони (сегодня/завтра МСК).
const demoTrattoriaNearMarker = "[demo-trattoria-near]"
const demoBellaNearMarker = "[demo-bella-near]"

// ensureRestaurantTodayTomorrowDemoBookings — много демо-броней на сегодня и завтра (Europe/Moscow) для ресторана по slug.
// waiterEmail2 может быть пустым — тогда второй официант совпадает с первым (как у Bella Vista с одним официантом).
func (a *Handlers) ensureRestaurantTodayTomorrowDemoBookings(ctx context.Context, slug, marker, waiterEmail1, waiterEmail2 string) {
	var restID uuid.UUID
	if err := a.Pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE slug=$1 LIMIT 1`, slug).Scan(&restID); err != nil {
		return
	}
	if waiterEmail2 == "" {
		waiterEmail2 = waiterEmail1
	}
	var wid1, wid2 uuid.UUID
	if err := a.Pool.QueryRow(ctx, `SELECT id FROM users WHERE lower(email)=lower($1) AND restaurant_id=$2`, waiterEmail1, restID).Scan(&wid1); err != nil {
		log.Printf("ensureRestaurantTodayTomorrowDemoBookings %s: нет официанта %s: %v", slug, waiterEmail1, err)
		return
	}
	if err := a.Pool.QueryRow(ctx, `SELECT id FROM users WHERE lower(email)=lower($1) AND restaurant_id=$2`, waiterEmail2, restID).Scan(&wid2); err != nil {
		wid2 = wid1
	}

	tabRows, err := a.Pool.Query(ctx, `
		SELECT t.id FROM tables t
		JOIN halls h ON h.id = t.hall_id
		WHERE h.restaurant_id = $1
		ORDER BY t.table_number`, restID)
	if err != nil {
		return
	}
	var tableIDs []uuid.UUID
	for tabRows.Next() {
		var tid uuid.UUID
		if err := tabRows.Scan(&tid); err != nil {
			tabRows.Close()
			return
		}
		tableIDs = append(tableIDs, tid)
	}
	tabRows.Close()
	if len(tableIDs) == 0 {
		return
	}

	clientRows, err := a.Pool.Query(ctx, `SELECT id FROM users WHERE role='client' ORDER BY email LIMIT 40`)
	if err != nil {
		return
	}
	var clientIDs []uuid.UUID
	for clientRows.Next() {
		var cid uuid.UUID
		if err := clientRows.Scan(&cid); err != nil {
			clientRows.Close()
			return
		}
		clientIDs = append(clientIDs, cid)
	}
	clientRows.Close()
	if len(clientIDs) == 0 {
		return
	}

	loc, errLoc := time.LoadLocation("Europe/Moscow")
	if errLoc != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	today0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	dayAfterTomorrow0 := today0.Add(48 * time.Hour)

	_, _ = a.Pool.Exec(ctx, `
		DELETE FROM reservations r
		USING tables t, halls h
		WHERE r.table_id = t.id AND t.hall_id = h.id
		  AND h.restaurant_id = $1
		  AND r.start_time >= $2 AND r.start_time < $3
		  AND r.comment LIKE '%' || $4 || '%'`,
		restID, today0.UTC(), dayAfterTomorrow0.UTC(), marker)

	slot := 2 * time.Hour
	type nb struct {
		dayPlus int
		hh, mm  int
		status  string
		ti, ci  int
		w1      bool
		pay     string
		seated  *time.Time
		svc     *time.Time
	}
	notes := []string{
		"У окна", "Бизнес-ланч", "День рождения", "Детский стул",
		"Без орехов", "Вегетарианское меню", "Юбилей", "Встреча",
		"Командировка", "Тихий столик", "Корпоратив", "Свидание",
	}
	var books []nb
	todayH := []struct{ hh, mm int }{
		{11, 0}, {11, 30}, {12, 0}, {12, 30}, {13, 0}, {14, 0}, {15, 30},
		{16, 30}, {17, 30}, {18, 30}, {19, 30}, {20, 0}, {20, 30},
	}
	for i, hm := range todayH {
		st := "confirmed"
		pay := "succeeded"
		var seated, svc *time.Time
		w1 := i%2 == 0
		switch i {
		case 2:
			st = "seated"
			s := today0.Add(time.Duration(hm.hh)*time.Hour + time.Duration(hm.mm)*time.Minute).UTC().Add(4 * time.Minute)
			seated = &s
		case 3:
			st = "in_service"
			s0 := today0.Add(time.Duration(hm.hh)*time.Hour + time.Duration(hm.mm)*time.Minute).UTC().Add(3 * time.Minute)
			s1 := s0.Add(18 * time.Minute)
			seated, svc = &s0, &s1
		case 7:
			st = "pending_payment"
			pay = "pending"
		}
		books = append(books, nb{0, hm.hh, hm.mm, st, i, i, w1, pay, seated, svc})
	}
	tomorrowH := []struct{ hh, mm int }{
		{10, 30}, {11, 30}, {12, 30}, {14, 0}, {15, 30}, {17, 0},
		{18, 30}, {19, 0}, {20, 30}, {21, 0},
	}
	for i, hm := range tomorrowH {
		st := "confirmed"
		pay := "succeeded"
		if i == 3 || i == 7 {
			st = "pending_payment"
			pay = "pending"
		}
		books = append(books, nb{1, hm.hh, hm.mm, st, i + len(todayH), i + len(todayH), i%2 == 0, pay, nil, nil})
	}

	inserted := 0
	for _, b := range books {
		day := today0
		if b.dayPlus == 1 {
			day = today0.Add(24 * time.Hour)
		}
		start := time.Date(day.Year(), day.Month(), day.Day(), b.hh, b.mm, 0, 0, loc).UTC()
		end := start.Add(slot)
		tid := tableIDs[b.ti%len(tableIDs)]
		uid := clientIDs[b.ci%len(clientIDs)]
		var w *uuid.UUID
		if b.w1 {
			w = &wid1
		} else {
			w = &wid2
		}
		comment := notes[b.ci%len(notes)] + " " + marker
		var cap int
		if err := a.Pool.QueryRow(ctx, `SELECT capacity FROM tables WHERE id=$1`, tid).Scan(&cap); err != nil {
			continue
		}
		guests := 2 + (b.ci % 5)
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
			) VALUES ($1,$2,$3,$4,$5,$6,$7,'client',$8,$9,$10,NULL)
			RETURNING id`,
			tid, uid, start, end, guests, b.status, comment, w, b.seated, b.svc,
		).Scan(&rid)
		if err != nil {
			log.Printf("ensureRestaurantTodayTomorrowDemoBookings %s: insert: %v", slug, err)
			continue
		}
		inserted++

		switch b.pay {
		case "succeeded":
			amount := 50000 + guests*12000 + (inserted%13)*1000
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO payments (reservation_id, amount_kopecks, status, idempotency_key, gateway_payment_id)
				VALUES ($1,$2,'succeeded',$3,$4)`,
				rid, amount, uuid.New(), "demo_near_"+rid.String()[:10])
		case "pending":
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO payments (reservation_id, amount_kopecks, status, idempotency_key)
				VALUES ($1,$2,'pending',$3)`,
				rid, 95000, uuid.New())
		}

		if w != nil && (b.status == "confirmed" || b.status == "seated" || b.status == "in_service" || b.status == "pending_payment") {
			_, _ = a.Pool.Exec(ctx, `
				INSERT INTO table_assignments (reservation_id, staff_user_id, table_id)
				VALUES ($1,$2,$3)`,
				rid, *w, tid)
		}
	}

	_, _ = a.Pool.Exec(ctx, `
		UPDATE tables t SET status='occupied', updated_at=NOW()
		WHERE EXISTS (
			SELECT 1 FROM reservations r
			WHERE r.table_id = t.id
			AND r.status IN ('seated','in_service')
			AND r.start_time <= NOW() AND r.end_time >= NOW()
		)`)

	if inserted > 0 {
		log.Printf("демо: %s — %d броней на сегодня/завтра (МСК)", slug, inserted)
	}
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
