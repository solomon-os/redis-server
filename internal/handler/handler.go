package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/parser"
	"github.com/codecrafters-io/redis-starter-go/internal/resp"
	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

type Handler struct {
	store *store.Store
}

type xReadChanStruct struct {
	reply resp.ReadStreamsReply
	err   error
}

func New(store *store.Store) *Handler {
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

	case "GET":
		return h.handleGet(cmd)

	case "RPUSH":
		return h.handleRPush(cmd)

	case "LRANGE":
		return h.handleLRange(cmd)

	case "LPUSH":
		return h.handleLPush(cmd)

	case "LLEN":
		return h.handleLLen(cmd)

	case "LPOP":
		return h.handleLPop(cmd)

	case "BLPOP":
		return h.handleBLPop(cmd)

	case "TYPE":
		return h.handleType(cmd)

	case "XADD":
		return h.handleXAdd(cmd)

	case "XRANGE":
		return h.handleXRange(cmd)

	case "XREAD":
		return h.handleXRead(cmd)

	case "INCR":
		return h.handleIncr(cmd)

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
	if len(cmd.Args) < 1 {
		return resp.Error("wrong number of arguments for 'get' command")
	}
	val, exist := h.store.Get(cmd.Args[0])
	if !exist {
		return resp.NullBulkString()
	}
	return resp.BulkString(val)
}

func (h *Handler) handleSet(cmd parser.Command) string {
	args, err := parser.ParseSetArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}
	h.store.Set(args.Key, args.Value, args.TTL)
	return resp.SimpleString("OK")
}

func (h *Handler) handleRPush(cmd parser.Command) string {
	args, err := parser.ParsePushArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}
	size := h.store.RPush(args.Key, args.Value)
	return resp.Integer(size)
}

func (h *Handler) handleLRange(cmd parser.Command) string {
	args, err := parser.ParseRangeArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}
	list := h.store.LRange(args.Key, args.Start, args.End)
	return resp.BulkStringArray(list)
}

func (h *Handler) handleLPush(cmd parser.Command) string {
	args, err := parser.ParsePushArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}
	size := h.store.LPush(args.Key, args.Value)
	return resp.Integer(size)
}

func (h *Handler) handleLLen(cmd parser.Command) string {
	args, err := parser.ParseLenArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}
	len := h.store.LLen(args.Key)
	return resp.Integer(len)
}

func (h *Handler) handleLPop(cmd parser.Command) string {
	args, err := parser.ParsePopArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}
	if !args.Arguments {
		popedItem := h.store.LPop(args.Key, 1)
		if len(popedItem) == 0 {
			return resp.NullBulkString()
		}
		return resp.BulkString(popedItem[0])
	}

	if args.Arguments && args.Length < 0 {
		return resp.Error("value not an interger or out of range")
	}

	popedItems := h.store.LPop(args.Key, args.Length)

	return resp.BulkStringArray(popedItems)
}

func (h *Handler) handleBLPop(cmd parser.Command) string {
	args, err := parser.ParseBPopArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}
	if args.Timeout < 0 {
		return resp.Error("value not an interger")
	}
	items := h.store.BLPop(args.Key, args.Timeout)
	if items == nil {
		return resp.NullOrBulkStringArray([]string{})
	}

	return resp.BulkStringArray(items)
}

func (h *Handler) handleType(cmd parser.Command) string {
	args, err := parser.ParseTypeArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}
	keyType := h.store.KeyType(args.Key)

	if keyType == "" {
		return resp.SimpleString("none")
	}

	return resp.SimpleString(keyType)
}

func (h *Handler) handleXAdd(cmd parser.Command) string {
	args, err := parser.ParseStreamArgs(cmd)
	if err != nil {
		return resp.Error(fmt.Sprintf("xadd command failed: %v", err))
	}

	id, err := h.store.SetStream(args.Key, args.ID, args.Fields)
	if err != nil {
		return resp.Error(err.Error())
	}

	return resp.BulkString(id)
}

func (h *Handler) handleXRange(cmd parser.Command) string {
	args, err := parser.ParseXRangeArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
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

func (h *Handler) handleXRead(cmd parser.Command) string {
	xreadCmd, err := parser.ParseXReadArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	switch xreadCmd.Name {
	case "STREAMS":
		return h.handleXReadStreams(xreadCmd)
	case "BLOCK":
		return h.handleXReadBlock2(xreadCmd)
	}

	return resp.Error("xread command not supported")
}

func (h *Handler) handleXReadStreams(cmd parser.Command) string {
	args, err := parser.ParseXReadStreamArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	outs := make([]resp.ReadStreamsReply, 0, len(args))

	storeParams := make([]store.RangeMultiArgs, 0, len(args))

	for i := range args {
		storeParams = append(
			storeParams,
			store.RangeMultiArgs{Key: args[i].Key, Start: args[i].Start},
		)
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

func (h *Handler) handleXReadBlock2(cmd parser.Command) string {
	args, err := parser.ParseXReadBlockArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
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

	storeParams := make([]store.RangeMultiArgs, 0, len(args.Streams))

	for i := range args.Streams {
		storeParams = append(
			storeParams,
			store.RangeMultiArgs{Key: args.Streams[i].Key, Start: args.Streams[i].Start},
		)
	}

	streams, err := h.store.RangeStreamMulti(storeParams)
	if err != nil {
		return resp.Error(err.Error())
	}

	outs := h.constructXReadReply(storeParams, streams)

	if len(outs) > 0 {
		return resp.XReadReply(outs)
	}

	for {
		select {
		case <-channel:
			streams, _ = h.store.RangeStreamMulti(storeParams)
			outs = h.constructXReadReply(storeParams, streams)
			if len(outs) > 0 {
				return resp.XReadReply(outs)
			}

		case <-ctx.Done():
			// check one more time to make sure we didn't miss anything
			streams, _ = h.store.RangeStreamMulti(storeParams)
			outs = h.constructXReadReply(storeParams, streams)

			return resp.XReadReply(outs)
		}
	}
}

// func (h *Handler) handleXReadBlock(cmd parser.Command) string {
// 	var wg sync.WaitGroup
//
// 	args, err := parser.ParseXReadBlockArgs(cmd)
// 	if err != nil {
// 		return resp.Error(err.Error())
// 	}
//
// 	out := []resp.ReadStreamsReply{}
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
//
// 	ch := make(chan xReadChanStruct)
//
// 	for i := range args.Streams {
// 		wg.Add(1)
// 		go func(ctx context.Context, stream parser.XReadStreamArgs) {
// 			defer wg.Done()
// 			entries, err := h.store.RangeStreamBlock(
// 				ctx,
// 				stream.Key,
// 				stream.Start,
// 				args.Timeout,
// 			)
//
// 			var msg xReadChanStruct
//
// 			if err != nil {
// 				msg.err = err
// 				select {
// 				case ch <- msg:
// 				case <-ctx.Done():
// 				}
// 				return
// 			}
//
// 			msg.reply = resp.ReadStreamsReply{Key: stream.Key}
//
// 			for j := range entries {
// 				msg.reply.StreamReplies = append(msg.reply.StreamReplies, resp.StreamReply{
// 					ID:     entries[j].ID.String(),
// 					Fields: entries[j].FlatFields(),
// 				})
// 			}
//
// 			if len(msg.reply.StreamReplies) > 0 {
// 				select {
// 				case ch <- msg:
// 				case <-ctx.Done():
// 				}
// 			}
// 		}(ctx, args.Streams[i])
// 	}
//
// 	go func() {
// 		wg.Wait()
// 		close(ch)
// 	}()
//
// 	if args.Timeout == 0 {
// 		// first one to read
// 		msg, ok := <-ch
// 		if ok {
// 			if msg.err != nil {
// 				return resp.Error(msg.err.Error())
// 			}
// 			out = append(out, msg.reply)
// 		}
//
// 		canceled := false
// 		// check again if another is sent in before exiting
// 		for !canceled {
// 			select {
//
// 			case msg, ok := <-ch:
// 				if ok {
// 					if msg.err != nil {
// 						return resp.Error(msg.err.Error())
// 					}
// 					out = append(out, msg.reply)
// 				}
// 			case <-time.After(time.Duration(5 * time.Millisecond)):
// 				cancel()
// 				canceled = true
// 			}
// 		}
//
// 	} else {
// 		time.AfterFunc(time.Duration(args.Timeout)*time.Millisecond, func() {
// 			cancel()
// 		})
//
// 		for msg := range ch {
// 			if msg.err != nil {
// 				return resp.Error(msg.err.Error())
// 			}
// 			out = append(out, msg.reply)
// 		}
// 	}
//
// 	if len(out) == 0 {
// 		return resp.NullOrBulkStringArray(nil)
// 	}
//
// 	return resp.XReadReply(out)
// }

func (h *Handler) handleIncr(cmd parser.Command) string {
	args, err := parser.ParseIncrArgs(cmd)
	if err != nil {
		return resp.Error(err.Error())
	}

	res, err := h.store.IncrementKv(args.Key)
	if err != nil {
		return resp.Error(err.Error())
	}

	return resp.Integer(res)
}

func (h *Handler) constructXReadReply(
	storeParams []store.RangeMultiArgs,
	streams [][]store.StreamEntry,
) []resp.ReadStreamsReply {
	outs := make([]resp.ReadStreamsReply, 0, len(storeParams))

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
	return outs
}
