package client

import (
	"net"
	"slices"

	"github.com/codecrafters-io/redis-starter-go/internal/parser"
	"github.com/codecrafters-io/redis-starter-go/internal/users"
)

type Conn struct {
	queued []parser.Command
	inTx   bool
	execTx bool
	net.Conn
	user *users.User
}

func New(conn net.Conn, user *users.User) *Conn {
	return &Conn{
		queued: []parser.Command{},
		Conn:   conn,
		user:   user,
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

func (c *Conn) ClearTransaction() {
	c.inTx, c.execTx, c.queued = false, false, []parser.Command{}
}

func (c *Conn) QueuedCommand() []parser.Command {
	return slices.Clone(c.queued)
}

func (c *Conn) GetUserFlags() []string {
	return c.user.Flags()
}

func (c *Conn) GetUserName() string {
	return c.user.Name()
}
