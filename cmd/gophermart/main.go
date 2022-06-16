package main

import (
	"context"
	"github.com/fortuna91/ya_praktikum_final/internal/configs"
	"github.com/fortuna91/ya_praktikum_final/internal/db"
	"github.com/fortuna91/ya_praktikum_final/internal/handlers"
	"github.com/fortuna91/ya_praktikum_final/internal/middleware"
	"github.com/fortuna91/ya_praktikum_final/internal/server"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
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
			log.Fatal(err)
		}
		serverStopCtx()
		log.Println("Server was stopped correctly")
	}()

	handlers.DB = db.New(config.DB)
	handlers.DB.Create(context.Background())

	handlers.Queue.AccrualSystemAddress = config.AccrualSystem
	handlers.Queue.RetryAfter = 0

	// run accrual system
	go func() {
		handlers.Queue.UpdateOrders(handlers.DB)
	}()

	log.Printf("Start server on %s", config.Address)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
