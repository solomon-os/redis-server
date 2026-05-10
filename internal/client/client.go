package client

import (
	"net"
	"slices"

	"github.com/codecrafters-io/redis-starter-go/internal/parser"
	"github.com/codecrafters-io/redis-starter-go/internal/users"
)

type Conn struct {
	queued        []parser.Command
	inTx          bool
	execTx        bool
	authenticated bool
	net.Conn
	user     *users.User
	password [32]byte
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

func (c *Conn) CreateUserWithPassword(name, password string) {
	if c.IsAuthenticated() {
		c.user = users.New(name, password)
		c.password = users.HashPassword(password)
		c.authenticated = true
	}
}

func (c *Conn) Authenticate(name, password string) bool {
	u, ok := users.Get(name)
	if !ok {
		return false
	}

	if u.CheckPassword(password) {
		c.user = u
		c.password = users.HashPassword(password)
		c.authenticated = true
		return true
	}

	return false
}

func (c *Conn) IsAuthenticated() bool {
	if c.user == nil {
		return false
	}

	if c.authenticated {
		return true
	}

	if !c.user.PasswordRequired() {
		return true
	}

	return false
}

func (c *Conn) AddPassword(password string) {
	c.user.AddPassword(password)
	c.password = users.HashPassword(password)
}
