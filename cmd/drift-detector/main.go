package main

import (
	"log"

	"github.com/joho/godotenv"

	"github.com/solomon-os/go-test/internal/cli"
	"github.com/solomon-os/go-test/internal/logger"
)

func main() {
	if err := godotenv.Load(); err != nil {
		logger.Warn("failed to load .env file", "error", err)
	}

	if err := cli.Run(); err != nil {
		log.Fatal(err)
	}
}
