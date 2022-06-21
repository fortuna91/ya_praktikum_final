package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/fortuna91/ya_praktikum_final/internal/accrual"
	"github.com/fortuna91/ya_praktikum_final/internal/auth"
	"github.com/fortuna91/ya_praktikum_final/internal/configs"
	"github.com/fortuna91/ya_praktikum_final/internal/handlers"
	"github.com/fortuna91/ya_praktikum_final/internal/middleware"
	"github.com/fortuna91/ya_praktikum_final/internal/server"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	config := configs.SetServerConfig()

	r := server.NewRouter()
	server := &http.Server{Addr: config.Address, Handler: middleware.Authorization(r)}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-sigChan

		ctx, serverStopCtx := context.WithTimeout(context.Background(), 10*time.Second)
		err := server.Shutdown(ctx)
		if err != nil {
			log.Err(err)
			panic(err)
		}
		serverStopCtx()
		log.Info().Msg("Server was stopped correctly")
	}()

	handlers.HashKey = config.HashKey
	handlers.ContextCancelTimeout = config.ContextCancel
	accrual.ContextCancelTimeout = config.ContextCancel
	accrual.AccrualSystemAddress = config.AccrualSystem
	auth.TokenDuration = config.TokenDuration

	if err := handlers.PrepareDB(config.DB); err != nil {
		log.Err(err)
		panic(err)
	}

	// run accrual system
	go func() {
		accrual.UpdateOrders(handlers.GetDB())
	}()

	log.Info().Msgf("Start server on %s", config.Address)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Err(err)
	}
}
