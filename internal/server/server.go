package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/signal"
	"sync"
	"syscall"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/internal/handler"
	"github.com/codecrafters-io/redis-starter-go/internal/resp"
	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

type Server struct {
	addr    string
	l       net.Listener
	handler *handler.Handler
	wg      sync.WaitGroup
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

	s.l = l

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	for {
		conn, err := s.l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				// graceful shutdown — wait for in-flight handlers to drain
				slog.Info("shutting down server")
				s.wg.Wait()
				return nil
			}
			slog.Error("Error accepting connection", "error", err)
			continue
		}
		c := client.New(conn)
		s.wg.Go(func() {
			s.handleConnection(ctx, c)
		})
	}
}

func (s *Server) handleConnection(ctx context.Context, c *client.Conn) {
	defer func() {
		if err := c.Close(); err != nil {
			slog.Error("failed to close connection", "error", err)
		}
	}()

	// closing the conn from a watcher goroutine interrupts a blocking Read on shutdown
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			_ = c.Close()
		case <-done:
		}
	}()

	// 1mb per request
	buf := make([]byte, 1024)

	for {
		n, err := c.Read(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) && ctx.Err() == nil {
				slog.Error("read error", "error", err)
			}
			return
		}

		response, err := s.handler.Handle(ctx, c, string(buf[:n]))
		if err != nil {
			slog.Error("handler error", "error", err)
			_, _ = io.WriteString(c, resp.Error(err.Error()))
			return
		}

		if _, err := io.WriteString(c, response); err != nil {
			slog.Error("couldn't send response", "error", err)
			return
		}
	}
}
