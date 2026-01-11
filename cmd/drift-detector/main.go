package main

import (
	"log"

	"github.com/solomon-os/go-test/internal/cli"
)

func main() {
	if err := cli.Run(); err != nil {
		log.Fatal(err)
	}
}
