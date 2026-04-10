package main

import (
	"log"

	"github.com/codecrafters-io/redis-starter-go/internal/server"
)

func main() {
	server := server.New("6379")

	if err := server.ListenAndAccept(); err != nil {
		log.Fatalln(err)
	}
}
