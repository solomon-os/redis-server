This is my own implementation of the
["Build Your Own Redis" Challenge](https://codecrafters.io/challenges/redis).

A toy Redis clone built in Go, supporting strings, lists, streams, transactions,
TTL, and blocking commands. Designed to match real Redis semantics where
possible, with a focus on correct concurrency behavior.

## Architecture

```
app/
  main.go                    - Entry point, starts the server on port 6379
internal/
  server/server.go           - TCP server, connection lifecycle, graceful shutdown
  handler/handler.go         - Command dispatch, transaction coordination
  parser/parser.go           - RESP protocol parsing
  resp/resp.go               - RESP protocol response encoding
  store/store.go             - In-memory data store with thread-safe concurrent access
  client/client.go           - Per-connection state (transactions, queued commands)
  helper/helper.go           - Generic concurrency helpers (e.g., OrChannel)
```

### Layer responsibilities

- **Server** owns connection lifecycle: accept loop, per-connection goroutines,
  signal-driven shutdown, WaitGroup-based drain.
- **Handler** dispatches commands to per-feature handlers. Coordinates
  cross-connection serialization for transactions via `sync.Cond`.
- **Parser** decodes raw RESP input into typed `Command` values, with
  bounds-checked argument extraction per command.
- **Resp** builds RESP wire-format replies (simple strings, bulk strings,
  arrays, integers, errors, null arrays).
- **Store** owns all data: kv map, list map, stream map. Provides synchronous
  read/write primitives. For blocking semantics it exposes listener channels
  the handler subscribes to.
- **Client** carries per-connection state: transaction queue, "in transaction"
  and "executing transaction" flags. Used by the handler when commands need to
  branch on transaction state.

## Supported Commands

### Strings

| Command | Usage | Description |
|---------|-------|-------------|
| `PING` | `PING` | Returns `PONG`. |
| `ECHO` | `ECHO <message>` | Echoes the message. |
| `SET` | `SET <key> <value> [EX seconds \| PX milliseconds]` | Sets a string with optional TTL. |
| `GET` | `GET <key>` | Returns the value, or nil if absent. |
| `INCR` | `INCR <key>` | Atomically increments a numeric string by 1. Initializes to 1 if absent. |
| `TYPE` | `TYPE <key>` | Returns `string`, `list`, `stream`, or `none`. |

### Lists

| Command | Usage | Description |
|---------|-------|-------------|
| `RPUSH` | `RPUSH <key> <value> [value ...]` | Appends to the right; returns the new length. Hands off directly to waiting `BLPOP` clients without storing. |
| `LPUSH` | `LPUSH <key> <value> [value ...]` | Prepends to the left; returns the new length. Same handoff behavior. |
| `LRANGE` | `LRANGE <key> <start> <stop>` | Returns elements in range; supports negative indices. |
| `LLEN` | `LLEN <key>` | Returns the list length, or 0 if absent. |
| `LPOP` | `LPOP <key> [count]` | Removes and returns from the left. Without count returns a single element; with count returns an array. |
| `BLPOP` | `BLPOP <key> <timeout>` | Blocks until an element is available or the timeout (in seconds, fractional allowed) elapses. `0` blocks indefinitely. |

### Streams

| Command | Usage | Description |
|---------|-------|-------------|
| `XADD` | `XADD <key> <id> <field> <value> [field value ...]` | Appends an entry. ID can be `*` (auto), `<ms>-*` (auto-seq), or `<ms>-<seq>` (explicit). |
| `XRANGE` | `XRANGE <key> <start> <end>` | Returns entries in range, inclusive on both ends. Supports `-` (start) and `+` (end) tokens. |
| `XREAD` | `XREAD STREAMS <key> [key ...] <id> [id ...]` | Returns entries strictly greater than each ID. Streams with no matching entries are omitted. |
| `XREAD BLOCK` | `XREAD BLOCK <ms> STREAMS <key> [key ...] <id> [id ...]` | Like XREAD, but blocks for up to `ms` milliseconds (`0` blocks indefinitely). Supports `$` for "only entries added after this call." |

### Transactions

| Command | Usage | Description |
|---------|-------|-------------|
| `MULTI` | `MULTI` | Begins a transaction. Subsequent commands return `QUEUED` until `EXEC`. |
| `EXEC` | `EXEC` | Atomically runs the queued commands. All other connections wait until EXEC completes. |

Notes:
- Blocking commands (`BLPOP`, `XREAD BLOCK`) inside a transaction degrade to
  their non-blocking counterparts during EXEC, matching Redis semantics.
- Argument validation that can be done at queue time (e.g., stream ID format)
  happens before the command is queued, so EXEC failures from malformed input
  surface immediately.

## Concurrency Design

### Store-level

- **Single `sync.RWMutex`** guards all in-memory state. Reads use `RLock`,
  writes use `Lock`.
- **List handoff for `BLPOP`**: when `RPUSH`/`LPUSH` finds waiters, items are
  delivered directly into per-waiter buffered channels under the same lock —
  no list write for handed-off items. The push's return value still reflects
  the "as if appended" length, matching Redis.
- **Stream broadcast for `XREAD BLOCK`**: `XADD` notifies all subscribed
  listeners by sending into per-subscriber buffered channels. Listeners are
  pure wake-up signals; the handler re-queries the store on each wake to get
  the actual data.
- **TTL** uses `time.AfterFunc` per key. Setting a key with an existing timer
  cancels the previous one before scheduling a new one.

### Handler-level

- **Transaction serialization** uses `sync.Cond` over a shared mutex.
  - Every command entry point checks a `locked` flag; if set, waits on the
    cond.
  - `EXEC` sets the flag, runs all queued commands, clears the flag,
    `Broadcast()`s. Other connections resume.
  - The executing connection's queued commands are dispatched directly,
    bypassing the entry-point wait.

### Connection-level (server)

- **Graceful shutdown**: `signal.NotifyContext` cancels a parent context on
  SIGINT/SIGTERM. A watcher goroutine closes the listener (interrupting the
  blocked `Accept`), and per-connection watchers close their conn (interrupting
  blocked `Read`s). A `sync.WaitGroup` ensures the server only returns after
  all in-flight handlers finish.
- **Per-connection ctx**: each connection derives its own cancellable context
  from the server's, so client-disconnect propagates without affecting siblings.

### Helpers

- **`OrChannel`** (Cox-Buday's pattern): combines N receive-only channels into
  one that closes when any input fires. Used for "wait until any of these
  conditions is met" — e.g., subscribing to multiple streams in `XREAD BLOCK`.

## Notable Implementation Details

- **RESP framing**: a fixed-size read buffer parses one command at a time. Not
  pipelining-aware (yet); each TCP read is treated as one full command.
- **`net.ErrClosed` filtering** on shutdown: avoids spurious "use of closed
  network connection" log noise when both the watcher and the deferred
  `Close()` race.
- **Subscribe-then-check ordering**: stream listeners are registered before
  the initial range read, closing the race where an XADD between read and
  subscribe could be lost.
- **Empty-string-safe BLPop**: delivery is tracked with a separate `bool` so
  that `BLPOP` correctly returns an empty string value (rather than treating
  it as a timeout sentinel).
- **`slices.Clone` for listener cleanup**: after handing off to the front of a
  listener slice, the discarded prefix is cloned away to release channel
  references for GC.

## Running

```bash
go run app/main.go
```

Starts the server on `0.0.0.0:6379`. Connect with any Redis client or `redis-cli`.

```bash
redis-cli
> SET foo bar
OK
> GET foo
"bar"
> XADD mystream '*' field1 value1
"1738152000000-0"
> XREAD BLOCK 5000 STREAMS mystream '$'
(blocks until a new entry arrives)
```

## Status

Implemented:
- Strings, lists, streams as described above.
- BLPOP and XREAD BLOCK with correct timeout semantics.
- Graceful shutdown.

In progress:
- MULTI / EXEC transactions (basic queueing + atomic execution working;
  cross-cutting cases being polished).

Not yet:
- WATCH / optimistic locking.
- Pub/sub (`SUBSCRIBE` / `PUBLISH`).
- Replication, persistence, clustering.
