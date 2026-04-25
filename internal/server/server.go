package server

import (
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/internal/handler"
	"github.com/codecrafters-io/redis-starter-go/internal/resp"
	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

type Server struct {
	addr    string
	l       net.Listener
	handler *handler.Handler
}

func New(addr string) *Server {
	slog.Info("Logs from your program will appear here!")

	store := store.New()
	handler := handler.New(store)

	return &Server{addr: addr, handler: handler}
}

func (s *Server) ListenAndAccept() error {
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", s.addr))
	if err != nil {
		return fmt.Errorf("failed to bind to port 6379: %w", err)
	}

	defer l.Close()

	s.l = l

	for {
		conn, err := s.l.Accept()
		if err != nil {
			slog.Error("Error accepting connection: ", "error", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Error("failed to close listener", "error", err)
		}
	}()

	client := client.New(conn)
	buf := make([]byte, 1024)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				slog.Error("read error", "error", err)
			}
			return
		}

		response, err := s.handler.Handle(client, string(buf[:n]))
		if err != nil {
			slog.Error("", "error", err)
			_, _ = io.WriteString(conn, resp.Error(err.Error()))
			return
		}

		_, err = io.WriteString(conn, response)
		if err != nil {
			slog.Error("couldn't send response", "error", err)
			return
		}
	}
}
