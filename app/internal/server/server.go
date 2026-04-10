package server

import (
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/codecrafters-io/redis-starter-go/app/internal/parser"
	"github.com/codecrafters-io/redis-starter-go/app/internal/resp"
)

type Server struct {
	addr string
	l    net.Listener
}

func New(addr string) *Server {
	slog.Info("Logs from your program will appear here!")

	return &Server{addr: addr}
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
	buf := make([]byte, 1024)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				slog.Error("read error", "error", err)
			}
			return
		}

		cmd, err := parser.Parse(string(buf[:n]))
		if err != nil {
			slog.Error("couldn't parse message", "error", err)
			_, _ = io.WriteString(conn, resp.Error("invalid command"))
			return
		}

		response := s.handleCommand(cmd)
		_, err = io.WriteString(conn, response)
		if err != nil {
			slog.Error("couldn't send response", "error", err)
			return
		}
	}
}

func (s *Server) handleCommand(cmd parser.Command) string {
	switch cmd.Name {
	case "PING":
		return resp.SimpleString("PONG")
	case "ECHO":
		if len(cmd.Args) < 1 {
			return resp.Error("wrong number of arguments for 'echo' command")
		}
		return resp.BulkString(cmd.Args[0])
	case "SET":
		return resp.SimpleString("OK")
	default:
		return resp.Error("unknown command")
	}
}
