package configs

import (
	"flag"
	"github.com/caarlos0/env/v6"
	"log"
	"os"
	"time"
)

type ServerConfig struct {
	Address       string `env:"RUN_ADDRESS" envDefault:"127.0.0.1:8080"`
	DB            string `env:"DATABASE_URI" envDefault:""`
	AccrualSystem string `env:"ACCRUAL_SYSTEM_ADDRESS" envDefault:"127.0.0.1:8080"`

	ContextCancel      time.Duration `env:"CANCEL_INTERVAL" envDefault:"2s"`
	TokenDuration      time.Duration `env:"TOKEN_DURATION" envDefault:"1h"`
	AccrualChannelPool int           `env:"ACCRUAL_CHANNEL_POOL" envDefault:"100"`
}

func SetServerConfig() ServerConfig {
	var envConfig ServerConfig
	err := env.Parse(&envConfig)
	if err != nil {
		log.Fatal(err)
	}

	var config ServerConfig
	/*flag.StringVar(&config.Address, "a", envConfig.Address, "Address")
	flag.StringVar(&config.DB, "d", envConfig.DB, "Address")
	flag.StringVar(&config.AccrualSystem, "r", envConfig.AccrualSystem, "Address")

	flag.DurationVar(&config.ContextCancel, "c", envConfig.ContextCancel, "Context cancel interval")
	flag.DurationVar(&config.TokenDuration, "t", envConfig.TokenDuration, "Token duration")
	flag.IntVar(&config.AccrualChannelPool, "p", envConfig.AccrualChannelPool, "Accrual channel pool size")*/

	err = env.Parse(&config)
	if err != nil {
		log.Fatal(err)
	}
	addr := flag.String("a", "127.0.0.1:8080", "Service address")
	db := flag.String("d", "", "Database connection address")
	accrualSystem := flag.String("r", "", "Accrual system address")
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
