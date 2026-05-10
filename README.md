This is my own implementation of the
["Build Your Own Redis" Challenge](https://codecrafters.io/challenges/redis).

A toy Redis clone built in Go, supporting strings, lists, streams, transactions,
TTL, blocking commands, ACL users, and password authentication. Designed to
match real Redis semantics where possible, with a focus on correct concurrency
behavior.

## Architecture

```
app/
  main.go                    - Entry point, starts the server on port 6379
internal/
  server/server.go           - TCP server, connection lifecycle, graceful shutdown
  handler/handler.go         - Command dispatch, transaction coordination, auth gate
  parser/parser.go           - RESP protocol parsing
  resp/resp.go               - RESP protocol response encoding
  store/store.go             - In-memory data store with thread-safe concurrent access
  client/client.go           - Per-connection state (transactions, queued commands, auth)
  users/users.go             - User registry, password hashing, ACL state
  helper/helper.go           - Generic concurrency helpers (e.g., OrChannel)
```

### Layer responsibilities

- **Server** owns connection lifecycle: accept loop, per-connection goroutines,
  signal-driven shutdown, WaitGroup-based drain.
- **Handler** dispatches commands to per-feature handlers. Coordinates
  cross-connection serialization for transactions via `sync.Cond`. Gates every
  non-AUTH command behind authentication when the user requires a password.
- **Parser** decodes raw RESP input into typed `Command` values, with
  bounds-checked argument extraction per command.
- **Resp** builds RESP wire-format replies (simple strings, bulk strings,
  arrays, integers, errors, null arrays).
- **Store** owns all data: kv map, list map, stream map. Provides synchronous
  read/write primitives. For blocking semantics it exposes listener channels
  the handler subscribes to.
- **Client** carries per-connection state: transaction queue, transaction
  flags, the authenticated user, and the hash of the password the connection
  authenticated with.
- **Users** owns the global user registry and the per-user state (flags,
  password hashes). Reads are wait-free via `atomic.Pointer`; writes use
  copy-on-write under a per-user mutex.

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
| `EXEC` | `EXEC` | Atomically runs the queued commands. Other connections wait until EXEC completes. |
| `DISCARD` | `DISCARD` | Cancels an in-progress transaction; clears the queue and returns to non-transactional mode. |

Notes:
- Blocking commands (`BLPOP`, `XREAD BLOCK`) inside a transaction degrade to
  their non-blocking counterparts during EXEC, matching Redis semantics.
- Argument validation that can be done at queue time (e.g., stream ID format)
  happens before the command is queued, so EXEC failures from malformed input
  surface immediately.

### Authentication & ACL

| Command | Usage | Description |
|---------|-------|-------------|
| `AUTH` | `AUTH <username> <password>` | Authenticates the connection. The hash of `<password>` is stored on the connection and re-checked against the user's current passwords on every command. |
| `ACL WHOAMI` | `ACL WHOAMI` | Returns the username the connection is authenticated as. Returns `NOAUTH` if the connection isn't authenticated. |
| `ACL GETUSER` | `ACL GETUSER <username>` | Returns the user's flags and password hashes. Returns empty arrays if the user doesn't exist. |
| `ACL SETUSER` | `ACL SETUSER <username> ><password>` | Creates the user if they don't exist, then appends `<password>` to their password set. The connection switches its effective auth to the new password. |

Auth model:
- All commands except `AUTH` are gated. If the connection's user requires a
  password (i.e., does not have the `nopass` flag) and the connection isn't
  authenticated, the command returns `NOAUTH Authentication required.`
- The default user (`default`) starts with the `nopass` flag, so connections
  are authenticated as `default` from the start unless reassigned.
- Adding a password to a user (via `ACL SETUSER` or first `AUTH`) removes the
  `nopass` flag from that user.
- Authentication is *dynamic*: the connection stores the hash of the password
  it authenticated with, and `IsAuthenticated()` re-checks that hash against
  the user's *current* passwords. If the password is removed from the user
  later, the connection loses access on the next command. This differs from
  real Redis (which freezes auth at AUTH time) and is a deliberate design
  choice.

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

### Users-level

- **Registry** (`map[string]*User`) is guarded by a `sync.RWMutex`. Reads
  (`Get`) take `RLock`; writes (`Set`, `New`) take `Lock`. The map and lock
  are package-private — callers can't reach around the accessors, so the
  invariant can't be broken from outside.
- **Per-user state is copy-on-write under `atomic.Pointer`**. Each `User`
  holds an `atomic.Pointer[snapshot]` where `snapshot` packs both the flags
  slice and the password hashes slice. Reads (`Flags`, `Passwords`,
  `CheckPassword`) do a single atomic load of the pointer — wait-free,
  scales linearly with cores, no RWMutex contention.
- **Writes** (`AddPassword`) take a per-user `sync.Mutex` (writers only),
  clone the current snapshot, mutate the copies, and atomically swap the
  pointer. Once published, a snapshot is never mutated, so readers holding
  a stale reference are always safe.
- **Why this pattern here**: passwords are read on essentially every command
  (auth gate) and written rarely (only on `ACL SETUSER`). That read/write
  ratio is the textbook case for atomic.Pointer + COW.
- **Constant-time password comparison**: `CheckHashedPassword` walks all
  stored hashes accumulating results via bitwise OR rather than short-
  circuiting on first match. Combined with `crypto/subtle.ConstantTimeCompare`
  per entry, the function takes the same total time regardless of where (or
  whether) the match occurs, defeating timing side-channels.
- **Password storage**: SHA-256 of the plaintext, stored as `[32]byte`.
  Fixed-size arrays compare faster than hex strings, allocate nothing, and
  occupy half the bytes. Hex encoding is done lazily on `Passwords()` for
  display only.

### Connection-level (server)

- **Graceful shutdown**: `signal.NotifyContext` cancels a parent context on
  SIGINT/SIGTERM. A watcher goroutine closes the listener (interrupting the
  blocked `Accept`), and per-connection watchers close their conn (interrupting
  blocked `Read`s). A `sync.WaitGroup` ensures the server only returns after
  all in-flight handlers finish.
- **Per-connection ctx**: each connection derives its own cancellable context
  from the server's, so client-disconnect propagates without affecting siblings.
- **Single-goroutine connection state**: `client.Conn`'s per-connection
  fields (transaction queue, auth state) are touched by exactly one
  goroutine — the connection's own handler loop. No mutex is needed there
  by design. The only cross-goroutine touch is the watcher's `c.Close()`,
  which is documented safe on `net.Conn`.

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
- **Auth gate is lazy**: the gate runs at every command entry, re-querying
  user state. This means ACL changes (password removal) take effect on the
  next command from any affected connection — no need to reach into other
  connections to invalidate them.

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

# Auth flow
> ACL SETUSER alice >hunter2
OK
> AUTH alice hunter2
OK
> ACL WHOAMI
"alice"
```

## Status

Implemented:
- Strings, lists, streams as described above.
- BLPOP and XREAD BLOCK with correct timeout semantics.
- Graceful shutdown.
- MULTI / EXEC / DISCARD transactions.
- Password authentication (`AUTH`) with auth-gated commands.
- ACL user management (`ACL WHOAMI`, `ACL GETUSER`, `ACL SETUSER`).

Not yet:
- WATCH / optimistic locking.
- Pub/sub (`SUBSCRIBE` / `PUBLISH`).
- Replication, persistence, clustering.
- Per-command ACL permissions (currently auth is all-or-nothing per user).
