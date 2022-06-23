package server

import (
	"github.com/fortuna91/ya_praktikum_final/internal/handlers"
	"github.com/go-chi/chi/v5"
)

func NewRouter() chi.Router {
	r := chi.NewRouter()
	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", handlers.Register)
		r.Post("/login", handlers.Login)
		r.Post("/orders", handlers.UploadOrder)
		r.Get("/orders", handlers.GetOrders)

		r.Route("/balance", func(r chi.Router) {
			r.Get("/", handlers.GetBalance)
			r.Post("/withdraw", handlers.Withdraw)
			r.Get("/withdrawals", handlers.GetWithdrawals)
		})
	})
	return r
}
