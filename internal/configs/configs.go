package configs

import (
	"flag"
	"log"
	"os"

	"github.com/caarlos0/env/v6"
)

type ServerConfig struct {
	Address       string `env:"RUN_ADDRESS" envDefault:"127.0.0.1:8080"`
	DB            string `env:"DATABASE_URI" envDefault:""`
	AccrualSystem string `env:"ACCRUAL_SYSTEM_ADDRESS" envDefault:"127.0.0.1:8080"`
}

func SetServerConfig() ServerConfig {
	var config ServerConfig
	err := env.Parse(&config)
	if err != nil {
		log.Fatal(err)
	}
	addr := flag.String("a", "127.0.0.1:8080", "Service address")
	db := flag.String("d", "", "Database connection address")
	accrualSystem := flag.String("k", "", "Accrual system address")
	flag.Parse()

	if _, ok := os.LookupEnv("RUN_ADDRESS"); !ok {
		config.Address = *addr
	}
	if _, ok := os.LookupEnv("DATABASE_URI"); !ok {
		config.DB = *db
	}
	if _, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS"); !ok {
		config.AccrualSystem = *accrualSystem
	}
	return config
}
