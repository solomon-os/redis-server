package handler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/internal/parser"
	"github.com/codecrafters-io/redis-starter-go/internal/resp"
	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

type Handler struct {
	store  *store.Store
	locked bool
	cond   *sync.Cond
}

func New(store *store.Store) *Handler {
	var locker sync.Mutex
	return &Handler{
		store:  store,
		locked: false,
		cond:   sync.NewCond(&locker),
	}
}

func (h *Handler) Handle(ctx context.Context, conn *client.Conn, raw string) (string, error) {
	cmd, err := parser.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("couldn't parse message: %v", err)
	}

	h.cond.L.Lock()
	for h.locked {
		h.cond.Wait()
	}
	h.cond.L.Unlock()

	response := h.handleCommand(ctx, conn, cmd)
	return response, nil
}

func (h *Handler) handleCommand(ctx context.Context, conn *client.Conn, cmd parser.Command) string {
	switch cmd.Name {
	case "PING":
		return h.handlePing(ctx, conn, cmd)

	case "ECHO":
		return h.handleEcho(ctx, conn, cmd)

	case "SET":
		return h.handleSet(ctx, conn, cmd)

	case "GET":
		return h.handleGet(ctx, conn, cmd)

	case "RPUSH":
		return h.handleRPush(ctx, conn, cmd)

	case "LRANGE":
		return h.handleLRange(ctx, conn, cmd)

	case "LPUSH":
		return h.handleLPush(ctx, conn, cmd)

	case "LLEN":
		return h.handleLLen(ctx, conn, cmd)

	case "LPOP":
		return h.handleLPop(ctx, conn, cmd)

	case "BLPOP":
		return h.handleBLPop(ctx, conn, cmd)

	case "TYPE":
		return h.handleType(ctx, conn, cmd)

	case "XADD":
		return h.handleXAdd(ctx, conn, cmd)

	case "XRANGE":
		return h.handleXRange(ctx, conn, cmd)

	case "XREAD":
		return h.handleXRead(ctx, conn, cmd)

	case "INCR":
		return h.handleIncr(ctx, conn, cmd)

	case "MULTI":
		return h.handleMulti(ctx, conn, cmd)

	case "EXEC":
		return h.handleExec(ctx, conn, cmd)

	case "DISCARD":
		return h.handleDiscard(ctx, conn, cmd)

	case "ACL":
		return h.handleAcl(ctx, conn, cmd)

	default:
		return resp.Error("unknown command")
	}
}

func (h *Handler) handlePing(_ context.Context, _ *client.Conn, _ parser.Command) string {
	return resp.SimpleString("PONG")
}

func (h *Handler) handleEcho(_ context.Context, _ *client.Conn, cmd parser.Command) string {
	if len(cmd.Args) < 1 {
		return resp.Error("wrong number of arguments for 'echo' command")
	}
	return resp.BulkString(cmd.Args[0])
}

func (h *Handler) handleGet(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	if len(cmd.Args) < 1 {
		return resp.Error("wrong number of arguments for 'get' command")
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	val, exist := h.store.Get(cmd.Args[0])
	if !exist {
		return resp.NullBulkString()
	}
	return resp.BulkString(val)
}

func (h *Handler) handleSet(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParseSetArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	h.store.Set(args.Key, args.Value, args.TTL)
	return resp.SimpleString("OK")
}

func (h *Handler) handleRPush(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParsePushArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	size := h.store.RPush(args.Key, args.Value)
	return resp.Integer(size)
}

func (h *Handler) handleLRange(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParseRangeArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	list := h.store.LRange(args.Key, args.Start, args.End)
	return resp.BulkStringArray(list)
}

func (h *Handler) handleLPush(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParsePushArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	size := h.store.LPush(args.Key, args.Value)
	return resp.Integer(size)
}

func (h *Handler) handleLLen(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParseLenArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	len := h.store.LLen(args.Key)
	return resp.Integer(len)
}

func (h *Handler) handleLPop(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParsePopArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	if args.Arguments && args.Length < 0 {
		return resp.Error("value not an interger or out of range")
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	if !args.Arguments {
		popedItem := h.store.LPop(args.Key, 1)
		if len(popedItem) == 0 {
			return resp.NullBulkString()
		}
		return resp.BulkString(popedItem[0])
	}

	popedItems := h.store.LPop(args.Key, args.Length)

	return resp.BulkStringArray(popedItems)
}

func (h *Handler) handleBLPop(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParseBPopArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}
	if args.Timeout < 0 {
		return resp.Error("value not an interger")
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	// if in trnasaction execute non-blocking operation
	if conn.IsExecutingTransaction() {
		items := h.store.LPop(args.Key, 1)
		if items == nil {
			return resp.NullOrBulkStringArray([]string{})
		}
		return resp.BulkStringArray([]string{args.Key, items[0]})
	}

	items := h.store.BLPop(args.Key, args.Timeout)
	if items == nil {
		return resp.NullOrBulkStringArray([]string{})
	}

	return resp.BulkStringArray(items)
}

func (h *Handler) handleType(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParseTypeArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	keyType := h.store.KeyType(args.Key)

	if keyType == "" {
		return resp.SimpleString("none")
	}

	return resp.SimpleString(keyType)
}

func (h *Handler) handleXAdd(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParseStreamArgs(cmd)
	if err != nil {
		return resp.Error(fmt.Sprintf("xadd command failed: %v", err))
	}

	// validate id first before queueing command
	_, _, _, _, err = store.ParseStreamID(args.ID)
	if err != nil {
		return resp.Error(err.Error())
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	id, err := h.store.SetStream(args.Key, args.ID, args.Fields)
	if err != nil {
		return resp.Error(err.Error())
	}

	return resp.BulkString(id)
}

func (h *Handler) handleXRange(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParseXRangeArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	// validate id first before queueing command
	if args.Start != "-" {
		_, _, _, _, err = store.ParseStreamID(args.Start)
		if err != nil {
			return resp.Error(err.Error())
		}
	}

	if args.End != "+" {
		_, _, _, _, err = store.ParseStreamID(args.End)
		if err != nil {
			return resp.Error(err.Error())
		}
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	entries, err := h.store.RangeStream(args.Key, args.Start, args.End)
	if err != nil {
		return resp.Error(err.Error())
	}

	out := make([]resp.StreamReply, 0, len(entries))

	for i := range entries {
		out = append(out, resp.StreamReply{
			ID:     entries[i].ID.String(),
			Fields: entries[i].FlatFields(),
		})
	}

	return resp.XRangeReply(out)
}

func (h *Handler) handleXRead(ctx context.Context, conn *client.Conn, cmd parser.Command) string {
	xreadCmd, err := parser.ParseXReadArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	switch xreadCmd.Name {
	case "STREAMS":
		return h.handleXReadStreams(ctx, conn, xreadCmd)
	case "BLOCK":
		return h.handleXReadBlock(ctx, conn, xreadCmd)
	}

	return resp.Error("xread command not supported")
}

func (h *Handler) handleXReadStreams(
	_ context.Context,
	conn *client.Conn,
	cmd parser.Command,
) string {
	args, err := parser.ParseXReadStreamArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	outs := make([]resp.ReadStreamsReply, 0, len(args))

	storeParams := make([]store.RangeMultiArgs, 0, len(args))

	for _, arg := range args {
		_, _, _, _, err = store.ParseStreamID(arg.Start)
		if err != nil {
			return resp.Error(err.Error())
		}

		storeParams = append(
			storeParams,
			store.RangeMultiArgs{Key: arg.Key, Start: arg.Start},
		)
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	streams, err := h.store.RangeStreamMulti(storeParams)
	if err != nil {
		return resp.Error(err.Error())
	}

	if len(streams) == 0 {
		return resp.XReadReply(outs)
	}

	for i, s := range storeParams {
		if len(streams[i]) == 0 {
			continue
		}

		out := resp.ReadStreamsReply{
			Key:           s.Key,
			StreamReplies: make([]resp.StreamReply, 0, len(streams[i])),
		}

		for _, k := range streams[i] {
			out.StreamReplies = append(out.StreamReplies, resp.StreamReply{
				ID:     k.ID.String(),
				Fields: k.FlatFields(),
			})
		}

		outs = append(outs, out)
	}

	return resp.XReadReply(outs)
}

func (h *Handler) handleXReadBlock(
	parentCtx context.Context,
	conn *client.Conn,
	cmd parser.Command,
) string {
	args, err := parser.ParseXReadBlockArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	storeParams := make([]store.RangeMultiArgs, 0, len(args.Streams))

	for _, argStream := range args.Streams {
		_, _, _, _, err = store.ParseStreamID(argStream.Start)
		if err != nil && argStream.Start != "$" {
			return resp.Error(err.Error())
		}

		storeParams = append(
			storeParams,
			store.RangeMultiArgs{Key: argStream.Key, Start: argStream.Start},
		)
	}

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	if args.Timeout > 0 {
		ctx, cancel = context.WithTimeout(
			ctx,
			time.Duration(args.Timeout)*time.Millisecond,
		)
		defer cancel()
	}

	channel := h.store.StreamSub()
	defer h.store.StreamUnSub(channel)

	var streams [][]store.StreamEntry
	var outs []resp.ReadStreamsReply

	if len(storeParams) >= 1 && storeParams[0].Start != "$" {
		streams, err = h.store.RangeStreamMulti(storeParams)
		if err != nil {
			return resp.Error(err.Error())
		}

		outs = h.constructXReadReply(storeParams, streams)

		if len(outs) > 0 {
			return resp.XReadReply(outs)
		}
	}

	if conn.IsExecutingTransaction() {
		return resp.XReadReply(outs)
	}

	for {
		select {
		case stream := <-channel:
			if storeParams[0].Start == "$" {
				streams = [][]store.StreamEntry{{stream}}
			} else {
				streams, _ = h.store.RangeStreamMulti(storeParams)
			}
			outs = h.constructXReadReply(storeParams, streams)
			if len(outs) > 0 {
				return resp.XReadReply(outs)
			}

		case <-ctx.Done():
			// check one more time to make sure we didn't miss anything
			if storeParams[0].Start != "$" {
				streams, _ = h.store.RangeStreamMulti(storeParams)
			}
			outs = h.constructXReadReply(storeParams, streams)

			return resp.XReadReply(outs)
		}
	}
}

func (h *Handler) handleIncr(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	args, err := parser.ParseIncrArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	// err = h.store.ValidateKeyIsInt(args.Key)
	// if err != nil {
	// 	return resp.Error(err.Error())
	// }

	enqueued := EnqueueComandIfTransactionAndNotExecuting(conn, cmd)
	if enqueued {
		return resp.SimpleString("QUEUED")
	}

	res, err := h.store.IncrementKv(args.Key)
	if err != nil {
		return resp.Error(err.Error())
	}

	return resp.Integer(res)
}

func (h *Handler) handleMulti(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	// check if client already has an active transaction
	if conn.InTransaction() {
		return resp.Error("ERR Multi calls cannot be nested")
	}

	// initialise the client transaction
	conn.NewTransaction()

	return resp.SimpleString("OK")
}

func (h *Handler) handleExec(ctx context.Context, conn *client.Conn, _ parser.Command) string {
	if !conn.InTransaction() {
		return resp.Error("EXEC without MULTI")
	}

	h.cond.L.Lock()
	h.locked = true

	conn.ExecuteTransaction()
	queuedCommands := conn.QueuedCommand()

	responses := make([]string, 0, len(queuedCommands))

	for _, cmd := range queuedCommands {
		responses = append(responses, h.handleCommand(ctx, conn, cmd))
	}

	h.locked = false
	conn.ClearTransaction()

	h.cond.Broadcast()
	h.cond.L.Unlock()

	return resp.StringArray(responses)
}

func (h *Handler) handleDiscard(_ context.Context, conn *client.Conn, _ parser.Command) string {
	if conn.InTransaction() {
		conn.ClearTransaction()
		return resp.SimpleString("OK")
	}

	return resp.Error("DISCARD without MULTI")
}

func (h *Handler) handleAcl(_ context.Context, conn *client.Conn, cmd parser.Command) string {
	arg, err := parser.ParseAclArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	switch strings.ToUpper(arg.Cmd) {
	default:
		return resp.BulkString("default")
	}
}

func (h *Handler) constructXReadReply(
	storeParams []store.RangeMultiArgs,
	streams [][]store.StreamEntry,
) []resp.ReadStreamsReply {
	outs := make([]resp.ReadStreamsReply, 0, len(storeParams))

	if len(streams) > 0 {
		for i, s := range storeParams {
			if len(streams[i]) == 0 {
				continue
			}

			out := resp.ReadStreamsReply{
				Key:           s.Key,
				StreamReplies: make([]resp.StreamReply, 0, len(streams[i])),
			}

			for _, k := range streams[i] {
				out.StreamReplies = append(out.StreamReplies, resp.StreamReply{
					ID:     k.ID.String(),
					Fields: k.FlatFields(),
				})
			}

			outs = append(outs, out)
		}
	}
	return outs
}

func EnqueueComandIfTransactionAndNotExecuting(conn *client.Conn, cmd parser.Command) bool {
	if conn.InTransaction() && !conn.IsExecutingTransaction() {
		conn.EnQueueCommand(cmd)
		return true
	}

	return false
}
