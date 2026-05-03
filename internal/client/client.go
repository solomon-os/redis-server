package client

import (
	"net"

	"github.com/codecrafters-io/redis-starter-go/internal/parser"
)

type Conn struct {
	Queued []parser.Command
	InTx   bool
	net.Conn
}

func New(conn net.Conn) *Conn {
	return &Conn{
		Queued: []parser.Command{},
		Conn:   conn,
	}
}
