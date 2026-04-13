package handlers

import "github.com/go-chi/chi/v5"

// MountAuth — только /api/auth/* и публичная регистрация/логин.
func (a *Handlers) MountAuth(r chi.Router) {
	r.Post("/auth/register", a.handleRegister)
	r.Post("/auth/login", a.handleLogin)
	r.Post("/auth/refresh", a.handleRefresh)
	r.Post("/auth/logout", a.handleLogout)
	r.Group(func(r chi.Router) {
		r.Use(a.requireAuth)
		r.Get("/auth/me", a.handleMe)
		r.Put("/auth/me", a.handleMeUpdate)
		r.Put("/auth/me/password", a.handlePassword)
	})
}

// MountHall — залы, схема, столы, блокировки.
func (a *Handlers) MountHall(r chi.Router) {
	a.MountRestaurantsPublic(r)
	r.Get("/booking-defaults", a.handleBookingDefaultsGet)
	r.With(a.optionalAuth).Get("/halls", a.handleHallsList)
	r.Get("/halls/{id}", a.handleHallGet)
	r.With(a.optionalAuth).Get("/halls/{id}/layout", a.handleLayoutGet)
	r.With(a.optionalAuth).Get("/halls/{id}/availability", a.handleHallAvailability)
	r.Group(func(r chi.Router) {
		r.Use(a.requireAuth)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Put("/halls/{id}/layout", a.handleLayoutPut)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Post("/halls/{id}/tables", a.handleTableCreate)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Put("/halls/{id}/tables/{tid}", a.handleTableUpdate)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Delete("/halls/{id}/tables/{tid}", a.handleTableDelete)
		r.With(a.requireRoles("client", "owner")).Post("/halls/{id}/tables/{tid}/lock", a.handleTableLock)
		r.With(a.requireRoles("client", "owner")).Delete("/halls/{id}/tables/{tid}/lock", a.handleTableUnlock)
	})
}

// MountReservation — брони, владелец, официант, админ-ручная бронь.
func (a *Handlers) MountReservation(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(a.requireAuth)
		a.MountOrders(r)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Get("/reservations", a.handleReservationsList)
		r.With(a.requireRoles("client", "owner")).Post("/reservations", a.handleReservationCreate)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Post("/admin/reservations", a.handleAdminReservationCreate)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Get("/admin/clients", a.handleAdminClientsList)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Get("/admin/users/lookup", a.handleAdminClientLookup)
		// Любой авторизованный пользователь — только свои брони (WHERE user_id в хендлере)
		r.Get("/reservations/my", a.handleReservationsMy)
		r.Get("/reservations/{rid}", a.handleReservationGet)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Put("/reservations/{rid}", a.handleReservationUpdate)
		r.With(a.requireRoles("client", "owner", "admin", "superadmin")).Delete("/reservations/{rid}", a.handleReservationCancel)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Post("/reservations/{rid}/checkin", a.handleCheckin)
		r.With(a.requireRoles("admin", "owner", "superadmin")).Post("/reservations/{rid}/noshow", a.handleNoshow)
		r.With(a.requireRoles("waiter", "admin", "owner", "superadmin")).Post("/reservations/{rid}/start-service", a.handleStartService)
		r.With(a.requireRoles("waiter", "admin", "owner", "superadmin")).Post("/reservations/{rid}/complete", a.handleComplete)

		r.With(a.requireRoles("owner")).Post("/owner/restaurant", a.handleOwnerRestaurantCreate)
		r.With(a.requireRoles("owner", "admin", "superadmin")).Put("/owner/restaurant", a.handleRestaurantUpdate)
		r.With(a.requireRoles("owner", "admin", "superadmin")).Post("/upload/restaurant-photo", a.handleUploadRestaurantPhoto)
		r.With(a.requireRoles("owner", "admin", "superadmin")).Post("/upload/menu-item-photo", a.handleUploadMenuItemPhoto)
		r.With(a.requireRoles("owner")).Post("/owner/staff/assign", a.handleOwnerStaffAssign)
		r.With(a.requireRoles("admin")).Post("/admin/staff/assign", a.handleAdminStaffAssign)
		r.With(a.requireRoles("admin")).Get("/admin/waiters", a.handleAdminWaitersList)
		r.With(a.requireRoles("admin")).Get("/admin/waiters/work-dates", a.handleAdminWaitersWorkDatesBulkGet)
		r.With(a.requireRoles("admin")).Get("/admin/waiters/{id}/work-dates", a.handleAdminWaiterWorkDatesGet)
		r.With(a.requireRoles("admin")).Put("/admin/waiters/{id}/work-dates", a.handleAdminWaiterWorkDatesPut)

		r.With(a.requireRoles("owner")).Get("/owner/analytics", a.handleOwnerAnalytics)
		r.With(a.requireRoles("owner")).Get("/owner/finance", a.handleOwnerFinance)
		r.With(a.requireRoles("owner")).Get("/owner/finance/export", a.handleOwnerFinanceExport)
		r.With(a.requireRoles("owner")).Get("/owner/staff-stats", a.handleStaffStats)
		r.With(a.requireRoles("owner")).Get("/owner/users", a.handleUsersList)
		r.With(a.requireRoles("owner")).Post("/owner/users", a.handleUserCreate)
		r.With(a.requireRoles("owner")).Put("/owner/users/{uid}", a.handleUserUpdate)
		r.With(a.requireRoles("owner")).Get("/settings", a.handleSettingsGet)
		r.With(a.requireRoles("owner")).Put("/settings", a.handleSettingsPut)

		r.Route("/superadmin", func(sr chi.Router) {
			sr.Use(a.requireRoles("superadmin"))
			a.MountSuperadmin(sr)
		})

		r.With(a.requireRoles("waiter", "admin", "owner")).Get("/waiter/my-tables", a.handleWaiterTables)
		r.With(a.requireRoles("waiter", "admin", "owner")).Get("/waiter/shifts", a.handleWaiterShifts)
		r.With(a.requireRoles("waiter", "admin", "owner")).Post("/waiter/notes", a.handleWaiterNote)
	})
	r.Group(func(r chi.Router) {
		r.Use(a.requireAuth)
		r.Use(a.requireRoles("admin", "owner", "superadmin"))
		a.MountMenuAdmin(r)
	})
}

// MountPayment — платежи и вебхуки.
func (a *Handlers) MountPayment(r chi.Router) {
	r.Post("/payments/webhook", a.handleWebhook)
	r.Post("/payments/webhook/stripe", a.handleStripeWebhook)
	r.Group(func(r chi.Router) {
		r.Use(a.requireAuth)
		r.Get("/payments/{pid}", a.handlePaymentGet)
		r.Post("/payments/{pid}/checkout", a.handlePaymentCheckout)
		r.With(a.requireRoles("admin", "owner")).Post("/payments/{pid}/refund", a.handleRefund)
		r.Post("/payments/checkout/{pid}/simulate", a.handleSimulatePay)
	})
}
