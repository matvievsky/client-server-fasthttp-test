package main

import (
	"log"

	"client-server-fasthttp-test/internal/server"
)

func main() {
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}
