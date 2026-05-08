package client

import (
	"net"
	"slices"

	"github.com/codecrafters-io/redis-starter-go/internal/parser"
)

type Conn struct {
	queued []parser.Command
	inTx   bool
	execTx bool
	net.Conn
}

func New(conn net.Conn) *Conn {
	return &Conn{
		queued: []parser.Command{},
		Conn:   conn,
	}
}

func (c *Conn) InTransaction() bool {
	return c.inTx
}

func (c *Conn) NewTransaction() {
	c.inTx = true
}

func (c *Conn) EnQueueCommand(cmd parser.Command) {
	c.queued = append(c.queued, cmd)
}

func (c *Conn) IsExecutingTransaction() bool {
	return c.execTx
}

func (c *Conn) ExecuteTransaction() {
	c.execTx = true
}

func (c *Conn) EndTransactionAndExection() {
	c.inTx, c.execTx = false, false
}

func (c *Conn) QueuedCommand() []parser.Command {
	return slices.Clone(c.queued)
}
