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
	r.Get("/halls", a.handleHallsList)
	r.Get("/halls/{id}", a.handleHallGet)
	r.With(a.optionalAuth).Get("/halls/{id}/layout", a.handleLayoutGet)
	r.With(a.optionalAuth).Get("/halls/{id}/availability", a.handleHallAvailability)
	r.Group(func(r chi.Router) {
		r.Use(a.requireAuth)
		r.With(a.requireRoles("admin", "owner")).Put("/halls/{id}/layout", a.handleLayoutPut)
		r.With(a.requireRoles("admin", "owner")).Post("/halls/{id}/tables", a.handleTableCreate)
		r.With(a.requireRoles("admin", "owner")).Put("/halls/{id}/tables/{tid}", a.handleTableUpdate)
		r.With(a.requireRoles("admin", "owner")).Delete("/halls/{id}/tables/{tid}", a.handleTableDelete)
		r.With(a.requireRoles("client", "owner")).Post("/halls/{id}/tables/{tid}/lock", a.handleTableLock)
		r.With(a.requireRoles("client", "owner")).Delete("/halls/{id}/tables/{tid}/lock", a.handleTableUnlock)
	})
}

// MountReservation — брони, владелец, официант, админ-ручная бронь.
func (a *Handlers) MountReservation(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(a.requireAuth)
		r.With(a.requireRoles("admin", "owner")).Get("/reservations", a.handleReservationsList)
		r.With(a.requireRoles("client", "owner")).Post("/reservations", a.handleReservationCreate)
		r.With(a.requireRoles("admin", "owner")).Post("/admin/reservations", a.handleAdminReservationCreate)
		r.With(a.requireRoles("admin", "owner")).Get("/admin/clients", a.handleAdminClientsList)
		r.With(a.requireRoles("client", "owner")).Get("/reservations/my", a.handleReservationsMy)
		r.Get("/reservations/{rid}", a.handleReservationGet)
		r.With(a.requireRoles("admin", "owner")).Put("/reservations/{rid}", a.handleReservationUpdate)
		r.With(a.requireRoles("client", "owner", "admin")).Delete("/reservations/{rid}", a.handleReservationCancel)
		r.With(a.requireRoles("admin", "owner")).Post("/reservations/{rid}/checkin", a.handleCheckin)
		r.With(a.requireRoles("admin", "owner")).Post("/reservations/{rid}/noshow", a.handleNoshow)
		r.With(a.requireRoles("waiter", "admin", "owner")).Post("/reservations/{rid}/start-service", a.handleStartService)
		r.With(a.requireRoles("waiter", "admin", "owner")).Post("/reservations/{rid}/complete", a.handleComplete)

		r.With(a.requireRoles("owner")).Get("/owner/analytics", a.handleOwnerAnalytics)
		r.With(a.requireRoles("owner")).Get("/owner/finance", a.handleOwnerFinance)
		r.With(a.requireRoles("owner")).Get("/owner/finance/export", a.handleOwnerFinanceExport)
		r.With(a.requireRoles("owner")).Get("/owner/staff-stats", a.handleStaffStats)
		r.With(a.requireRoles("owner")).Get("/owner/users", a.handleUsersList)
		r.With(a.requireRoles("owner")).Post("/owner/users", a.handleUserCreate)
		r.With(a.requireRoles("owner")).Put("/owner/users/{uid}", a.handleUserUpdate)
		r.With(a.requireRoles("owner")).Get("/settings", a.handleSettingsGet)
		r.With(a.requireRoles("owner")).Put("/settings", a.handleSettingsPut)

		r.With(a.requireRoles("waiter", "admin", "owner")).Get("/waiter/my-tables", a.handleWaiterTables)
		r.With(a.requireRoles("waiter", "admin", "owner")).Get("/waiter/shifts", a.handleWaiterShifts)
		r.With(a.requireRoles("waiter", "admin", "owner")).Post("/waiter/notes", a.handleWaiterNote)
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
