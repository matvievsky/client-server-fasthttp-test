package main

import (
	"log"

	"client-server-fasthttp-test/internal/client"
)

func main() {
	if err := client.Serve(); err != nil {
		log.Fatal(err)
	}
}
