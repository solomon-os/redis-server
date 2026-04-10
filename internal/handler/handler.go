package handler

import (
	"github.com/codecrafters-io/redis-starter-go/internal/parser"
	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

type Handler struct {
	store store.Store
}

func (h *Handler) handleCommand(cmd parser.Command) {
}
