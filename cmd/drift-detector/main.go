package main

import (
	"log"

	"github.com/joho/godotenv"

	"github.com/solomon-os/go-test/internal/cli"
)

func main() {
	_ = godotenv.Load() //nolint:errcheck // .env file is optional

	if err := cli.Run(); err != nil {
		log.Fatal(err)
	}
}
