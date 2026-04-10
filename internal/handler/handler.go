package handler

import (
	"fmt"

	"github.com/codecrafters-io/redis-starter-go/internal/parser"
	"github.com/codecrafters-io/redis-starter-go/internal/resp"
	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

type Handler struct {
	store store.Store
}

func New(store store.Store) *Handler {
	return &Handler{store}
}

func (h *Handler) Handle(raw string) (string, error) {
	cmd, err := parser.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("couldn't parse message: %v", err)
	}

	response := h.handleCommand(cmd)
	return response, nil
}

func (h *Handler) handleCommand(cmd parser.Command) string {
	switch cmd.Name {
	case "PING":
		return resp.SimpleString("PONG")
	case "ECHO":
		if len(cmd.Args) < 1 {
			return resp.Error("wrong number of arguments for 'echo' command")
		}
		return resp.BulkString(cmd.Args[0])
	case "SET":
		setArgs := parser.ParseSetArgs(cmd)
		if err := h.store.Set(cmd.Args[0], cmd.Args[1]); err != nil {
			return resp.Error("insertion failed")
		}
		return resp.SimpleString("OK")
	case "GET":
		val, exist := h.store.Get(cmd.Args[0])
		if !exist {
			return resp.NullBulkString()
		}
		return resp.BulkString(val)
	default:
		return resp.Error("unknown command")
	}
}
