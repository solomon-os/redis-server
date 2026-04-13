This is my own implementation of the
["Build Your Own Redis" Challenge](https://codecrafters.io/challenges/redis).

A toy Redis clone built in Go, capable of handling core Redis commands with support for key expiration, list operations, and blocking commands.

## Architecture

```
app/
  main.go                  - Entry point, starts the server on port 6379
internal/
  server/server.go         - TCP server, connection lifecycle management
  handler/handler.go       - Command dispatch and execution
  parser/parser.go         - RESP protocol parsing
  resp/resp.go             - RESP protocol response encoding
  store/store.go           - In-memory data storage engine
```

- **Server** handles TCP connections and I/O
- **Handler** routes parsed commands to the appropriate logic
- **Parser** decodes raw RESP input into structured commands
- **Resp** encodes responses back into RESP format
- **Store** manages all data with thread-safe concurrent access

## Supported Commands

### String Commands

| Command | Usage | Description |
|---------|-------|-------------|
| `PING` | `PING` | Returns `PONG`. Health check. |
| `ECHO` | `ECHO <message>` | Returns the given message. |
| `SET` | `SET <key> <value> [EX seconds \| PX milliseconds]` | Set a key-value pair with optional TTL. |
| `GET` | `GET <key>` | Get the value of a key. Returns nil if the key does not exist. |

### List Commands

| Command | Usage | Description |
|---------|-------|-------------|
| `RPUSH` | `RPUSH <key> <value> [value ...]` | Append one or more values to the right end of a list. Returns the new list length. |
| `LPUSH` | `LPUSH <key> <value> [value ...]` | Prepend one or more values to the left end of a list. Returns the new list length. |
| `LRANGE` | `LRANGE <key> <start> <stop>` | Return elements in a list within the given range. Supports negative indices. |
| `LLEN` | `LLEN <key>` | Return the length of a list. Returns 0 if the key does not exist. |
| `LPOP` | `LPOP <key> [count]` | Remove and return elements from the left end of a list. Without count returns a single element; with count returns an array. |
| `BLPOP` | `BLPOP <key> <timeout>` | Blocking left pop. Waits for an element to be available or until timeout (in seconds). Timeout of 0 blocks indefinitely. Returns `[key, element]`. |

## Key Features

- **TTL support** - `SET` with `EX` (seconds) or `PX` (milliseconds) for automatic key expiration using Go timers
- **Blocking operations** - `BLPOP` uses a channel-based listener pattern to block until data is available
- **Negative indexing** - `LRANGE` supports negative indices (e.g., `-1` for the last element)
- **Concurrency** - Thread-safe storage using `sync.RWMutex` with read locks for reads and write locks for mutations
- **RESP protocol** - Full support for Simple Strings, Bulk Strings, Arrays, Integers, Errors, and Null Bulk Strings
- **Multiple blocking clients** - Multiple clients can `BLPOP` on the same key; they are served FIFO as elements arrive

## Running

```bash
go run app/main.go
```

Starts the server on `0.0.0.0:6379`. Connect with any Redis client or `redis-cli`.
