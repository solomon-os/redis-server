package handler

import (
	"github.com/codecrafters-io/redis-starter-go/app/internal/parser"
	"github.com/codecrafters-io/redis-starter-go/app/internal/store"
)

type Handler struct {
	store store.Store
}

func (h *Handler) handleCommand(cmd parser.Command) {
}
