package client

import "net"

type Client struct {
	conn net.Conn
	addr *net.Conn // serves as unique identifier of the connection
}

func New(conn net.Conn) *Client {
	return &Client{
		conn: conn,
		addr: &conn,
	}
}
