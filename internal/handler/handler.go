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
		return h.handlePing(cmd)
	case "ECHO":
		return h.handleEcho(cmd)
	case "SET":
		return h.handleSet(cmd)

	case "RPUSH":
		return h.handleRPush(cmd)

	case "LRANGE":
		return h.handleLRange(cmd)
	case "GET":
		return h.handleGet(cmd)
	default:
		return resp.Error("unknown command")
	}
}

func (h *Handler) handlePing(_ parser.Command) string {
	return resp.SimpleString("PONG")
}

func (h *Handler) handleEcho(cmd parser.Command) string {
	if len(cmd.Args) < 1 {
		return resp.Error("wrong number of arguments for 'echo' command")
	}
	return resp.BulkString(cmd.Args[0])
}

func (h *Handler) handleGet(cmd parser.Command) string {
	val, exist := h.store.Get(cmd.Args[0])
	if !exist {
		return resp.NullBulkString()
	}
	return resp.BulkString(val)
}

func (h *Handler) handleSet(cmd parser.Command) string {
	args := parser.ParseSetArgs(cmd)
	h.store.Set(cmd.Args[0], cmd.Args[1], args.TTL)
	return resp.SimpleString("OK")
}

func (h *Handler) handleRPush(cmd parser.Command) string {
	args := parser.ParseRPushArgs(cmd)
	size := h.store.SetList(args.ListKey, args.ListValue)
	return resp.Integer(size)
}

func (h *Handler) handleLRange(cmd parser.Command) string {
	args := parser.ParseLRangeArgs(cmd)
	list := h.store.GetListRange(args.Key, args.Start, args.End)
	return resp.BulkStringArray(list)
}
