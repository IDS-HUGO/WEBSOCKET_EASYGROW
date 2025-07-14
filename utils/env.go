package utils

import (
	"log"

	"github.com/joho/godotenv"
)

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("❌ Error cargando .env: %v", err)
	}
}
